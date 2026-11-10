package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Quality reporting (Stage 10b, brief 8): nightly agreement snapshots, the
// daily batch of gold-miss feedback, and the project quality report.

// Job kinds.
const (
	JobAgreementSnapshot = "agreement_snapshot"
	JobGoldFeedback      = "gold_feedback"
)

// AgreementWindow is how far back a nightly snapshot looks.
const AgreementWindow = 30 * 24 * time.Hour

// DailyJobs are the quality jobs and their local run times (after midnight
// in Nairobi, when the previous day is complete).
var DailyJobs = []jobs.Daily{{Kind: JobAgreementSnapshot, At: time.Hour}, {Kind: JobGoldFeedback, At: 90 * time.Minute}}

// Handlers returns the job handlers for the runner.
func (s *Service) Handlers() map[string]jobs.Handler {
	day := func(payload json.RawMessage) (time.Time, error) {
		var p struct{ Day string }
		if err := json.Unmarshal(payload, &p); err != nil {
			return time.Time{}, err
		}
		return time.ParseInLocation(time.DateOnly, p.Day, jobs.Nairobi)
	}
	return map[string]jobs.Handler{
		JobAgreementSnapshot: func(ctx context.Context, payload json.RawMessage) error {
			d, err := day(payload)
			if err != nil {
				return err
			}
			end := d.AddDate(0, 0, 1)
			_, err = s.SnapshotAgreement(ctx, end.Add(-AgreementWindow), end, nil)
			return err
		},
		JobGoldFeedback: func(ctx context.Context, payload json.RawMessage) error {
			d, err := day(payload)
			if err != nil {
				return err
			}
			_, err = s.GoldFeedback(ctx, d, d.AddDate(0, 0, 1))
			return err
		},
	}
}

// SnapshotAgreement computes brief 8.2's agreement figures for every project
// with items completed in [from, to) (or just one project) and stores them.
// It runs in one transaction, so a retried job never leaves half a snapshot.
func (s *Service) SnapshotAgreement(ctx context.Context, from, to time.Time, only *uuid.UUID) (int, error) {
	n := 0
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		ids := []uuid.UUID{}
		if only != nil {
			ids = append(ids, *only)
		} else {
			var err error
			if ids, err = q.ProjectsWithConsensusIn(ctx, store.ProjectsWithConsensusInParams{FromAt: from, ToAt: to}); err != nil {
				return err
			}
		}
		for _, id := range ids {
			pr, err := q.GetProjectForWork(ctx, id)
			if err != nil {
				return err
			}
			cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
			if err != nil {
				return err
			}
			rows, err := q.CompletedItemAnswers(ctx, store.CompletedItemAnswersParams{ProjectID: id, FromAt: from, ToAt: to})
			if err != nil {
				return err
			}
			k := int(pr.OverlapK)
			items := [][]quality.Answer{}
			var cur uuid.UUID
			for _, r := range rows {
				var a quality.Answer
				if err := json.Unmarshal(r.Answer, &a); err != nil {
					return err
				}
				if r.ItemID != cur || len(items) == 0 {
					items, cur = append(items, nil), r.ItemID
				}
				items[len(items)-1] = append(items[len(items)-1], a)
			}
			complete := items[:0]
			for _, it := range items {
				if len(it) == k { // the same number of raters on every item
					complete = append(complete, it)
				}
			}
			for _, st := range quality.ProjectAgreement(pr.TaskType, cfg.Answers(), complete, k) {
				var label *string
				if st.Label != "" {
					label = &st.Label
				}
				if err := q.AddAgreementSnapshot(ctx, store.AddAgreementSnapshotParams{OrgID: pr.OrgID, ProjectID: id, Label: label,
					Statistic: st.Statistic, Value: math.Round(st.Value*10000) / 10000, NItems: int32(st.Items), NRaters: int32(st.Raters),
					WindowStart: from, WindowEnd: to}); err != nil {
					return err
				}
				n++
			}
		}
		return nil
	})
	return n, err
}

// GoldFeedback writes the daily batch of gold-miss feedback (brief 8.1):
// one note per contributor and project with misses in [from, to), never
// naming the items, so it cannot reveal which items were gold.
func (s *Service) GoldFeedback(ctx context.Context, from, to time.Time) (int, error) {
	n := 0
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.GoldResultsForDay(ctx, store.GoldResultsForDayParams{FromAt: from, ToAt: to})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.Missed == 0 {
				continue
			}
			body := fmt.Sprintf("On %s, %d of your %d answers on %s to items with a known answer did not match it. "+
				"Before your next tasks, re-read the project instructions, especially the parts about the cases you found hard. "+
				"This note is automatic and does not say which items they were.",
				r.ProjectName, r.Missed, r.Total, from.In(jobs.Nairobi).Format("2 January"))
			if err := q.AddFeedback(ctx, store.AddFeedbackParams{OrgID: r.OrgID, ProjectID: r.ProjectID, ContributorID: r.ContributorID, Body: body}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// warmupRate raises the gold rate for a contributor's first tasks on a
// project (D-100 section 3.6; paid as usual, feedback as brief 8.1).
func warmupRate(ctx context.Context, q *store.Queries, contributor, project uuid.UUID) (float64, error) {
	tasks, rate := 5.0, 0.5
	for key, dst := range map[string]*float64{"warmup.tasks": &tasks, "warmup.gold_rate": &rate} {
		v, err := q.GetSetting(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if err := json.Unmarshal(v, dst); err != nil {
			return 0, fmt.Errorf("setting %s: %w", key, err)
		}
	}
	done, err := q.CountMyWorkInProject(ctx, store.CountMyWorkInProjectParams{ContributorID: contributor, ProjectID: project})
	if err != nil {
		return 0, err
	}
	if float64(done) < tasks {
		return rate, nil
	}
	return 0, nil
}

// AgreementFigure is one agreement statistic with its interpretation band
// (kappa only; percent and mean absolute difference have none).
type AgreementFigure struct {
	Label     *string   `json:"label"` // null: whole project
	Statistic string    `json:"statistic"`
	Value     float64   `json:"value"`
	Band      *string   `json:"band"`
	Items     int32     `json:"items"`
	Raters    int32     `json:"raters"`
	WindowEnd time.Time `json:"window_end"`
}

// ProjectQuality is the project's quality picture for Ops and its client.
// It holds aggregates only: no contributor appears in it (brief 8.4).
type ProjectQuality struct {
	Items          int32             `json:"items"`
	Completed      int32             `json:"completed"`
	RoutedToReview int32             `json:"routed_to_review"`
	ReviewRate     *float64          `json:"review_rate"`
	RouteReasons   map[string]int32  `json:"route_reasons"`
	GoldAccuracy   *float64          `json:"gold_accuracy"` // last 30 days, all contributors
	GoldResults    int32             `json:"gold_results"`
	Contributors   int32             `json:"contributors"`
	Agreement      []AgreementFigure `json:"agreement"`
}

// GetProjectQuality returns the quality picture. The caller must be able to
// see the project (row-level security: staff, or its client); figures are
// then read in system scope because gold rows are staff-only.
func (s *Service) GetProjectQuality(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (ProjectQuality, error) {
	if err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		_, err := store.New(tx).GetProjectForWork(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}); err != nil {
		return ProjectQuality{}, err
	}
	var out ProjectQuality
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		sm, err := q.ProjectQualitySummary(ctx, projectID)
		if err != nil {
			return err
		}
		out = ProjectQuality{Items: sm.Items, Completed: sm.Completed, RoutedToReview: sm.Routed, GoldResults: sm.GoldTotal,
			Contributors: sm.Contributors, Agreement: []AgreementFigure{},
			RouteReasons: map[string]int32{"no_majority": sm.NoMajority, "spread": sm.Spread, "flagged": sm.Flagged, "audit_sample": sm.AuditSample}}
		if sm.Completed > 0 {
			v := math.Round(float64(sm.Routed)/float64(sm.Completed)*10000) / 10000
			out.ReviewRate = &v
		}
		if sm.GoldTotal > 0 {
			v := math.Round(float64(sm.GoldCorrect)/float64(sm.GoldTotal)*10000) / 10000
			out.GoldAccuracy = &v
		}
		snaps, err := q.LatestSnapshots(ctx, projectID)
		if err != nil {
			return err
		}
		for _, sn := range snaps {
			f := AgreementFigure{Label: sn.Label, Statistic: sn.Statistic, Value: sn.Value, Items: sn.NItems, Raters: sn.NRaters, WindowEnd: sn.WindowEnd}
			if sn.Statistic == "cohen_kappa" || sn.Statistic == "fleiss_kappa" {
				b := quality.Band(sn.Value)
				f.Band = &b
			}
			out.Agreement = append(out.Agreement, f)
		}
		return nil
	})
	return out, err
}

// SnapshotProjectNow recomputes one project's agreement over the last 30 days
// (Ops, so figures are fresh before a client call).
func (s *Service) SnapshotProjectNow(ctx context.Context, p *auth.Principal, projectID uuid.UUID, m auth.Meta) (ProjectQuality, error) {
	if _, err := s.GetProjectQuality(ctx, p, projectID); err != nil {
		return ProjectQuality{}, err
	}
	now := time.Now()
	if _, err := s.SnapshotAgreement(ctx, now.Add(-AgreementWindow), now, &projectID); err != nil {
		return ProjectQuality{}, err
	}
	return s.GetProjectQuality(ctx, p, projectID)
}

// Feedback is a note to a contributor: from a reviewer (Stage 11) or the
// automatic gold-miss batch.
type Feedback struct {
	ID          uuid.UUID  `json:"id"`
	Body        string     `json:"body"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ProjectName string     `json:"project_name"`
	Automatic   bool       `json:"automatic"`
	ReadAt      *time.Time `json:"read_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// MyFeedback lists the caller's latest feedback.
func (s *Service) MyFeedback(ctx context.Context, p *auth.Principal) ([]Feedback, error) {
	id, err := contributorOf(p)
	if err != nil {
		return nil, err
	}
	out := []Feedback{}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListMyFeedback(ctx, id)
		for _, r := range rows {
			out = append(out, Feedback{ID: r.ID, Body: r.Body, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
				Automatic: r.Automatic, ReadAt: r.ReadAt, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

// MarkFeedbackRead marks one of the caller's notes as read.
func (s *Service) MarkFeedbackRead(ctx context.Context, p *auth.Principal, feedbackID uuid.UUID) error {
	id, err := contributorOf(p)
	if err != nil {
		return err
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		_, err := store.New(tx).MarkFeedbackRead(ctx, store.MarkFeedbackReadParams{ID: feedbackID, ContributorID: id})
		return err
	})
}

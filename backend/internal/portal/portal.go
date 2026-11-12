// Package portal serves the client (enterprise) view of the workspace (Stage
// 13a, brief 6.3): project progress and pace, the workforce summary, the
// organisation's activity log, its team and its invoices. Ops reads the same
// figures for any organisation.
//
// Access is decided twice: the caller must see the row in their own
// database scope (row-level security, AC-10), and only then are aggregates
// read in system scope, because gold rows and the audit log are staff-only.
// Nothing that identifies a contributor leaves this package except a
// per-organisation code (D-123).
package portal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

var (
	ErrNotFound     = errors.New("portal: not found")
	ErrInvalidInput = errors.New("portal: invalid input")
	ErrConflict     = errors.New("portal: conflict")
	ErrStorage      = errors.New("portal: file storage unavailable")
	ErrFileChanged  = errors.New("portal: stored file does not match its checksum")
)

// Service serves the client portal.
type Service struct {
	pool    *db.Pool
	blobs   storage.Blobs
	codeKey []byte
	now     func() time.Time
	log     *slog.Logger
}

// NewService derives the contributor-code key from secret
// (config.IdentityKey), so no new secret is needed.
func NewService(pool *db.Pool, blobs storage.Blobs, secret []byte, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("mezai/client-contributor-codes/v1"))
	return &Service{pool: pool, blobs: blobs, codeKey: m.Sum(nil), now: time.Now, log: log}
}

// visibleProject loads a project in the caller's scope: staff see any, a
// client only its own organisation's (row-level security).
func (s *Service) visibleProject(ctx context.Context, p *auth.Principal, id uuid.UUID) (store.Project, error) {
	var pr store.Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		pr, err = store.New(tx).GetProject(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return pr, err
}

// PaceWindow is how far back the finishing pace looks.
const PaceWindow = 14 * 24 * time.Hour

// ThroughputWeeks is how many weeks of throughput the chart shows.
const ThroughputWeeks = 8

// Week is one Nairobi week (starting Monday) of finished items.
type Week struct {
	Week  string `json:"week"` // YYYY-MM-DD, the Monday
	Final int32  `json:"final"`
}

// Pace projects when the work will finish from the last 14 days (A-074).
type Pace struct {
	// Goal is the target count when set, otherwise every item imported.
	Goal      int32   `json:"goal"`
	Remaining int32   `json:"remaining"`
	PerDay    float64 `json:"per_day"`
	// ProjectedFinish is null when there is no pace yet or nothing remains.
	ProjectedFinish *string `json:"projected_finish"`
	// State: done, on_track, at_risk (projected after the deadline, or no
	// pace and the deadline is near), no_pace, no_deadline.
	State string `json:"state"`
}

// Progress is a project's items by stage, without gold checks.
type Progress struct {
	ProjectID  uuid.UUID `json:"project_id"`
	Total      int32     `json:"total"`
	NotStarted int32     `json:"not_started"`
	InProgress int32     `json:"in_progress"`
	InReview   int32     `json:"in_review"`
	Final      int32     `json:"final"`
	Rejected   int32     `json:"rejected"`
	Target     *int32    `json:"target"`
	Deadline   *string   `json:"deadline"`
	Throughput []Week    `json:"throughput"`
	Pace       Pace      `json:"pace"`
}

func day(t time.Time) string { return t.Format(time.DateOnly) }

// GetProgress returns a project's progress, throughput and pace.
func (s *Service) GetProgress(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (Progress, error) {
	pr, err := s.visibleProject(ctx, p, projectID)
	if err != nil {
		return Progress{}, err
	}
	now := s.now()
	today := now.In(jobs.Nairobi)
	monday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, jobs.Nairobi).
		AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	first := monday.AddDate(0, 0, -7*(ThroughputWeeks-1))

	out := Progress{ProjectID: pr.ID, Target: pr.TargetCount, Throughput: []Week{}}
	if pr.Deadline != nil {
		d := day(*pr.Deadline)
		out.Deadline = &d
	}
	var recent int32
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		c, err := q.ProjectProgress(ctx, projectID)
		if err != nil {
			return err
		}
		out.Total, out.NotStarted, out.InProgress, out.InReview, out.Final, out.Rejected =
			c.Total, c.NotStarted, c.InProgress, c.InReview, c.Final, c.Rejected
		weeks, err := q.ProjectThroughput(ctx, store.ProjectThroughputParams{ProjectID: projectID, Since: &first})
		if err != nil {
			return err
		}
		got := map[string]int32{}
		for _, w := range weeks {
			got[day(w.Week)] = w.Final
		}
		for i := range ThroughputWeeks {
			k := day(first.AddDate(0, 0, 7*i))
			out.Throughput = append(out.Throughput, Week{Week: k, Final: got[k]})
		}
		recent, err = q.CountFinalSince(ctx, store.CountFinalSinceParams{ProjectID: projectID, Since: ptr(now.Add(-PaceWindow))})
		return err
	})
	if err != nil {
		return Progress{}, err
	}
	out.Pace = pace(out, recent, today)
	return out, nil
}

// atRiskWithin: with no pace yet, a deadline this close is at risk.
const atRiskWithin = 7

// pace projects a finishing date from recent finished items per day.
func pace(pr Progress, recent int32, today time.Time) Pace {
	goal := pr.Total
	if pr.Target != nil && *pr.Target > 0 {
		goal = *pr.Target
	}
	pc := Pace{Goal: goal, Remaining: max(goal-pr.Final, 0)}
	pc.PerDay = math.Round(float64(recent)/PaceWindow.Hours()*24*100) / 100
	var deadline *time.Time
	if pr.Deadline != nil {
		if d, err := time.ParseInLocation(time.DateOnly, *pr.Deadline, jobs.Nairobi); err == nil {
			deadline = &d
		}
	}
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, jobs.Nairobi)
	switch {
	case goal > 0 && pc.Remaining == 0:
		pc.State = "done"
		return pc
	case recent > 0:
		days := int(math.Ceil(float64(pc.Remaining) / (float64(recent) / PaceWindow.Hours() * 24)))
		f := day(start.AddDate(0, 0, days))
		pc.ProjectedFinish = &f
	}
	switch {
	case deadline == nil:
		pc.State = "no_deadline"
	case pc.ProjectedFinish != nil && *pc.ProjectedFinish > day(*deadline):
		pc.State = "at_risk"
	case pc.ProjectedFinish != nil:
		pc.State = "on_track"
	case deadline.Sub(start) <= atRiskWithin*24*time.Hour:
		pc.State = "at_risk"
	default:
		pc.State = "no_pace"
	}
	return pc
}

// Code returns a contributor's code for one organisation: stable within
// the organisation, unlinkable across organisations without the key (D-123).
// It always starts with a letter, so it never looks like a staff-side
// public code (C-0007).
func (s *Service) Code(orgID, contributorID uuid.UUID) string {
	m := hmac.New(sha256.New, s.codeKey)
	m.Write(orgID[:])
	m.Write(contributorID[:])
	sum := m.Sum(nil)
	const letters = "ABCDEFGHJKLMNPQRSTVWXYZ" // Crockford letters, no I, O or U
	enc := base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)
	return fmt.Sprintf("C-%c%s", letters[int(sum[0])%len(letters)], enc.EncodeToString(sum[1:])[:5])
}

// Worker is one contributor on a project, by code only.
type Worker struct {
	Code            string    `json:"code"`
	Tasks           int32     `json:"tasks"`
	LastSubmittedAt time.Time `json:"last_submitted_at"`
}

// SkillCount is how many of the project's contributors hold a skill.
type SkillCount struct {
	Kind         string `json:"kind"` // language or domain
	Value        string `json:"value"`
	Contributors int32  `json:"contributors"`
}

// Workforce summarises who works on a project (brief 6.3: counts, skills,
// contributor IDs only).
type Workforce struct {
	Contributors int32        `json:"contributors"`
	ActiveRecent int32        `json:"active_last_7_days"`
	Workers      []Worker     `json:"workers"`
	Skills       []SkillCount `json:"skills"`
}

// GetWorkforce returns the workforce summary for a project.
func (s *Service) GetWorkforce(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (Workforce, error) {
	pr, err := s.visibleProject(ctx, p, projectID)
	if err != nil {
		return Workforce{}, err
	}
	out := Workforce{Workers: []Worker{}, Skills: []SkillCount{}}
	week := s.now().Add(-7 * 24 * time.Hour)
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.ProjectWorkforce(ctx, projectID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out.Workers = append(out.Workers, Worker{Code: s.Code(pr.OrgID, r.ContributorID), Tasks: r.Tasks, LastSubmittedAt: r.LastSubmittedAt})
			if r.LastSubmittedAt.After(week) {
				out.ActiveRecent++
			}
		}
		out.Contributors = int32(len(rows))
		skills, err := q.ProjectWorkforceSkills(ctx, projectID)
		for _, k := range skills {
			out.Skills = append(out.Skills, SkillCount{Kind: k.Kind, Value: k.Value, Contributors: k.Contributors})
		}
		return err
	})
	return out, err
}

func ptr[T any](v T) *T { return &v }

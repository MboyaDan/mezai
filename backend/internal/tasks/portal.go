package tasks

import (
	"context"
	"errors"
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

// Contributor dashboard reads (Stage 12a): the quality record, the size of
// the work on offer and the training material. None of it says which items
// were gold (brief 8.1).

// TrustView is the trust score as the contributor sees it (brief 8.4),
// fixed at the start of the Nairobi day (D-120).
type TrustView struct {
	Score            *float64  `json:"score"`
	GoldAccuracy     *float64  `json:"gold_accuracy"`
	Agreement        *float64  `json:"agreement"`
	ReviewAcceptance *float64  `json:"review_acceptance"`
	Building         bool      `json:"building"` // fewer gold results than min_gold_results
	CalculatedAt     time.Time `json:"calculated_at"`
}

// TrendPoint is the last score of one Nairobi day.
type TrendPoint struct {
	Day          string   `json:"day"`
	Score        *float64 `json:"score"`
	GoldAccuracy *float64 `json:"gold_accuracy"`
}

// Quality is the contributor's quality record and the rules behind it.
type Quality struct {
	AsOf           time.Time            `json:"as_of"` // start of today in Nairobi
	Trust          *TrustView           `json:"trust"` // null before the first score
	Trend          []TrendPoint         `json:"trend"` // last 30 days, days with a score only
	Weights        quality.TrustWeights `json:"weights"`
	Floor          float64              `json:"floor"`
	WindowDays     int32                `json:"window_days"`
	MinGoldResults int32                `json:"min_gold_results"`
}

const trendDays = 30

// startOfDay is midnight in Nairobi on t's Nairobi date.
func startOfDay(t time.Time) time.Time {
	n := t.In(jobs.Nairobi)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, jobs.Nairobi)
}

// MyQuality returns the caller's trust score and 30-day trend as they stood
// at the start of today. Today's tasks show tomorrow, the same rhythm as
// gold feedback and gold earnings (D-110), so a score that moves right after
// a task never marks that task as gold. Runs in system scope because gold
// results are staff-only under row-level security; every query filters by
// the caller.
func (s *Service) MyQuality(ctx context.Context, p *auth.Principal) (Quality, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Quality{}, err
	}
	asOf := startOfDay(time.Now())
	out := Quality{AsOf: asOf, Trend: []TrendPoint{}}
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		ts, err := loadTrustSettings(ctx, q)
		if err != nil {
			return err
		}
		out.Weights, out.Floor, out.WindowDays, out.MinGoldResults = ts.weights, ts.floor, ts.windowDays, ts.minGold
		row, err := q.TrustScoreBefore(ctx, store.TrustScoreBeforeParams{ContributorID: id, Before: asOf})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		default:
			gold, err := q.GoldCountBefore(ctx, store.GoldCountBeforeParams{ContributorID: id, Before: asOf})
			if err != nil {
				return err
			}
			out.Trust = &TrustView{Score: row.Score, GoldAccuracy: row.GoldAccuracy, Agreement: row.AgreementRate,
				ReviewAcceptance: row.ReviewAcceptance, Building: gold < ts.minGold, CalculatedAt: row.CreatedAt}
		}
		trend, err := q.TrustTrend(ctx, store.TrustTrendParams{ContributorID: id, FromAt: asOf.AddDate(0, 0, -trendDays), Before: asOf})
		for _, t := range trend {
			out.Trend = append(out.Trend, TrendPoint{Day: t.Day.Format(time.DateOnly), Score: t.Score, GoldAccuracy: t.GoldAccuracy})
		}
		return err
	})
	return out, err
}

// minTimedAnswers is how many recent answers a project needs before its
// typical time per task is shown.
const minTimedAnswers = 10

// addWorkSizes fills TasksLeft and MinutesPerTask. System scope: counting
// open items needs rows the contributor cannot read; the result is two
// numbers per project.
func (s *Service) addWorkSizes(ctx context.Context, contributor uuid.UUID, out []Project) error {
	if len(out) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(out))
	for i, p := range out {
		ids[i] = p.ID
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		open, err := q.OpenItemCounts(ctx, store.OpenItemCountsParams{Ids: ids, ContributorID: contributor})
		if err != nil {
			return err
		}
		times, err := q.TypicalTaskTimes(ctx, ids)
		if err != nil {
			return err
		}
		left := map[uuid.UUID]int32{}
		for _, o := range open {
			left[o.ProjectID] = o.Open
		}
		mins := map[uuid.UUID]float64{}
		for _, t := range times {
			if t.Answers >= minTimedAnswers {
				// Rounded up to the half minute; never below half a minute.
				mins[t.ProjectID] = math.Max(0.5, math.Ceil(t.MedianMs/30000)/2)
			}
		}
		for i := range out {
			n := left[out[i].ID]
			out[i].TasksLeft = &n
			if m, ok := mins[out[i].ID]; ok {
				out[i].MinutesPerTask = &m
			}
		}
		return nil
	})
}

// Guideline is one project guideline.
type Guideline struct {
	Title  string `json:"title"`
	BodyMd string `json:"body_md"`
}

// TrainingProject is a project's instructions and guidelines.
type TrainingProject struct {
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	TaskType       string      `json:"task_type"`
	InstructionsMd string      `json:"instructions_md"`
	Guidelines     []Guideline `json:"guidelines"`
}

// MyTraining returns the instructions and guidelines of every live project
// the caller may work on. Someone not yet active gets an empty list: the
// training page then shows only the general material.
func (s *Service) MyTraining(ctx context.Context, p *auth.Principal) ([]TrainingProject, error) {
	projects, err := s.eligibleProjects(ctx, p)
	if errors.Is(err, ErrNotEligible) {
		return []TrainingProject{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]TrainingProject, len(projects))
	ids := make([]uuid.UUID, len(projects))
	at := map[uuid.UUID]int{}
	for i, pr := range projects {
		out[i] = TrainingProject{ID: pr.ID, Name: pr.Name, TaskType: pr.TaskType, InstructionsMd: pr.InstructionsMd, Guidelines: []Guideline{}}
		ids[i] = pr.ID
		at[pr.ID] = i
	}
	if len(ids) == 0 {
		return out, nil
	}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).GuidelinesForProjects(ctx, ids)
		for _, r := range rows {
			i := at[r.ProjectID]
			out[i].Guidelines = append(out[i].Guidelines, Guideline{Title: r.Title, BodyMd: r.BodyMd})
		}
		return err
	})
	return out, err
}

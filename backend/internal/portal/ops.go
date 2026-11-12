package portal

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Ops overview (Stage 13b, brief 6.3): every open project with its pace,
// the queues waiting for Ops, and contributor health. Staff only.

// RecentWindow is what "active" and "worked recently" mean on the overview.
const RecentWindow = 7 * 24 * time.Hour

// AutoPauseWindow is how far back automatic pauses are counted.
const AutoPauseWindow = 30 * 24 * time.Hour

// GoldWindow matches the project quality panel (30 days).
const GoldWindow = 30 * 24 * time.Hour

// WatchListSize is how many low trust scores the overview lists.
const WatchListSize = 8

// Queues are the counts waiting for someone at ELS. Each matches the list
// it links to.
type Queues struct {
	Review          int32      `json:"review"`
	OldestReviewAt  *time.Time `json:"oldest_review_at"`
	Reports         int32      `json:"reports"`
	Attempts        int32      `json:"attempts"`
	Applications    int32      `json:"applications"`
	Interviews      int32      `json:"interviews"`
	IdentityChecks  int32      `json:"identity_checks"`
	ProjectRequests int32      `json:"project_requests"`
}

// OpsProject is one open project on the overview.
type OpsProject struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Status             string    `json:"status"`
	TaskType           string    `json:"task_type"`
	OrgID              uuid.UUID `json:"org_id"`
	OrgName            string    `json:"org_name"`
	Deadline           *string   `json:"deadline"`
	Total              int32     `json:"total"`
	Final              int32     `json:"final"`
	InReview           int32     `json:"in_review"`
	ActiveContributors int32     `json:"active_contributors"`
	Reserved           int32     `json:"reserved"`
	GoldResults        int32     `json:"gold_results"`
	GoldAccuracy       *float64  `json:"gold_accuracy"`
	Pace               Pace      `json:"pace"`
}

// TrustWatch is a contributor with a low settled trust score.
type TrustWatch struct {
	ID           uuid.UUID `json:"id"`
	PublicCode   string    `json:"public_code"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Score        float64   `json:"score"`
	GoldAccuracy *float64  `json:"gold_accuracy"`
	ScoredAt     time.Time `json:"scored_at"`
	// AutoPaused: paused, and the last status change was the system's.
	AutoPaused bool `json:"auto_paused"`
}

// ContributorHealth counts contributors by status and trust.
type ContributorHealth struct {
	Applied          int32        `json:"applied"`
	Qualifying       int32        `json:"qualifying"`
	Active           int32        `json:"active"`
	Paused           int32        `json:"paused"`
	Removed          int32        `json:"removed"`
	WorkedRecently   int32        `json:"worked_last_7_days"`
	AutoPaused       int32        `json:"auto_paused_last_30_days"`
	ActiveBelowFloor int32        `json:"active_below_floor"`
	Building         int32        `json:"building"`
	Floor            float64      `json:"floor"`
	MinGoldResults   int32        `json:"min_gold_results"`
	Watch            []TrustWatch `json:"watch"`
}

// OpsOverview is the Ops home page.
type OpsOverview struct {
	GeneratedAt  time.Time         `json:"generated_at"`
	Queues       Queues            `json:"queues"`
	Projects     []OpsProject      `json:"projects"`
	Contributors ContributorHealth `json:"contributors"`
}

// stateOrder puts the projects that need Ops first.
var stateOrder = map[string]int{"at_risk": 0, "no_pace": 1, "on_track": 2, "no_deadline": 3, "done": 4}

// GetOpsOverview reads the overview in the caller's staff scope.
func (s *Service) GetOpsOverview(ctx context.Context, p *auth.Principal) (OpsOverview, error) {
	if p.Kind != auth.KindStaff {
		return OpsOverview{}, ErrNotFound
	}
	now := s.now()
	today := now.In(jobs.Nairobi)
	out := OpsOverview{GeneratedAt: now.UTC(), Projects: []OpsProject{}}
	out.Contributors.Watch = []TrustWatch{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		qs, err := q.OpsQueues(ctx)
		if err != nil {
			return err
		}
		out.Queues = Queues{Review: qs.Review, Reports: qs.Reports, Attempts: qs.Attempts,
			Applications: qs.Applications, Interviews: qs.Interviews, IdentityChecks: qs.IdentityChecks,
			ProjectRequests: qs.ProjectRequests}
		oldest, err := q.OpsOldestReview(ctx)
		if err != nil {
			return err
		}
		if len(oldest) > 0 {
			out.Queues.OldestReviewAt = &oldest[0]
		}

		rows, err := q.OpsProjects(ctx, store.OpsProjectsParams{
			ActiveSince: ptr(now.Add(-RecentWindow)),
			GoldSince:   now.Add(-GoldWindow),
			PaceSince:   ptr(now.Add(-PaceWindow)),
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			op := OpsProject{ID: r.ID, Name: r.Name, Status: r.Status, TaskType: r.TaskType, OrgID: r.OrgID,
				OrgName: r.OrgName, Total: r.Total, Final: r.Final, InReview: r.InReview,
				ActiveContributors: r.ActiveContributors, Reserved: r.Reserved, GoldResults: r.GoldTotal}
			if r.Deadline != nil {
				op.Deadline = ptr(day(*r.Deadline))
			}
			if r.GoldTotal > 0 {
				op.GoldAccuracy = ptr(float64(r.GoldCorrect) / float64(r.GoldTotal))
			}
			op.Pace = pace(Progress{Total: r.Total, Final: r.Final, Target: r.TargetCount, Deadline: op.Deadline},
				r.RecentFinal, today)
			out.Projects = append(out.Projects, op)
		}
		// Stable: within a state the query's deadline order stays.
		slices.SortStableFunc(out.Projects, func(a, b OpsProject) int {
			return stateOrder[a.Pace.State] - stateOrder[b.Pace.State]
		})

		floor, err := settingNumber(ctx, q, "trust.floor", 0.70)
		if err != nil {
			return err
		}
		minGold, err := settingNumber(ctx, q, "trust.min_gold_results", 20)
		if err != nil {
			return err
		}
		h, err := q.OpsContributorHealth(ctx, store.OpsContributorHealthParams{
			ActiveSince: ptr(now.Add(-RecentWindow)),
			PausedSince: now.Add(-AutoPauseWindow),
			MinGold:     int32(minGold),
			Floor:       floor,
		})
		if err != nil {
			return err
		}
		c := &out.Contributors
		c.Applied, c.Qualifying, c.Active, c.Paused, c.Removed = h.Applied, h.Qualifying, h.Active, h.Paused, h.Removed
		c.WorkedRecently, c.AutoPaused, c.ActiveBelowFloor, c.Building = h.WorkedRecently, h.AutoPaused, h.ActiveBelowFloor, h.Building
		c.Floor, c.MinGoldResults = floor, int32(minGold)

		watch, err := q.OpsTrustWatch(ctx, store.OpsTrustWatchParams{MinGold: int32(minGold), Lim: WatchListSize})
		if err != nil {
			return err
		}
		for _, w := range watch {
			if w.Score == nil {
				continue
			}
			c.Watch = append(c.Watch, TrustWatch{ID: w.ID, PublicCode: w.PublicCode, Name: w.DisplayName,
				Status: w.Status, Score: *w.Score, GoldAccuracy: w.GoldAccuracy, ScoredAt: w.ScoredAt,
				AutoPaused: w.Status == "paused" && w.LastChangeAutomatic})
		}
		return nil
	})
	if err != nil {
		return OpsOverview{}, err
	}
	return out, nil
}

// settingNumber reads a numeric setting, keeping the brief's default when
// the key is missing (the same rule as the quality engine).
func settingNumber(ctx context.Context, q *store.Queries, key string, def float64) (float64, error) {
	v, err := q.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(string(v)), `"`), 64)
	if err != nil {
		return 0, fmt.Errorf("setting %s: %w", key, err)
	}
	return f, nil
}

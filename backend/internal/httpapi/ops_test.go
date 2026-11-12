package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

type opsOverviewResp struct {
	Queues struct {
		Review          int     `json:"review"`
		OldestReviewAt  *string `json:"oldest_review_at"`
		Reports         int     `json:"reports"`
		Attempts        int     `json:"attempts"`
		Applications    int     `json:"applications"`
		Interviews      int     `json:"interviews"`
		IdentityChecks  int     `json:"identity_checks"`
		ProjectRequests int     `json:"project_requests"`
	} `json:"queues"`
	Projects []struct {
		ID                 string   `json:"id"`
		Status             string   `json:"status"`
		Total              int      `json:"total"`
		Final              int      `json:"final"`
		InReview           int      `json:"in_review"`
		ActiveContributors int      `json:"active_contributors"`
		GoldAccuracy       *float64 `json:"gold_accuracy"`
		Pace               struct {
			State string `json:"state"`
		} `json:"pace"`
	} `json:"projects"`
	Contributors struct {
		Active           int     `json:"active"`
		WorkedRecently   int     `json:"worked_last_7_days"`
		AutoPaused       int     `json:"auto_paused_last_30_days"`
		ActiveBelowFloor int     `json:"active_below_floor"`
		Floor            float64 `json:"floor"`
		Watch            []struct {
			PublicCode string  `json:"public_code"`
			Status     string  `json:"status"`
			Score      float64 `json:"score"`
			AutoPaused bool    `json:"auto_paused"`
		} `json:"watch"`
	} `json:"contributors"`
}

// TestOpsOverview (Stage 13b): only Ops reads it; every queue count equals
// the list it links to; projects carry the 13a pace with at-risk first;
// trust health counts settled scores against the floor.
func TestOpsOverview(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")
	for _, who := range []string{"client.admin@sample.invalid", "reviewer@sample.invalid", "contributor001@sample.invalid"} {
		if w := e.do(http.MethodGet, "/v1/ops/overview", nil, e.signIn(who), nil); w.Code != http.StatusForbidden {
			t.Fatalf("%s reads the Ops overview: %d", who, w.Code)
		}
	}

	// Two contributors disagree on every item: four items go to review.
	project := e.reviewProject(4)
	e.workAll(e.signIn("contributor001@sample.invalid"), project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(e.signIn("contributor002@sample.invalid"), project, map[string]any{"labels": []string{"No"}})
	// A second live project, due tomorrow with nothing finished: at risk.
	late := e.reviewProject(2)
	c1 := e.contributorID("contributor001@sample.invalid")
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		if _, err := tx.Exec(ctx, `UPDATE projects SET deadline = (now() AT TIME ZONE 'Africa/Nairobi')::date + 1 WHERE id = $1`, late); err != nil {
			return err
		}
		// A settled score under the floor, then an automatic pause.
		if _, err := tx.Exec(ctx, `INSERT INTO trust_scores (contributor_id, score, gold_accuracy, gold_count, window_days, weights)
			VALUES ($1, 0.55, 0.60, 40, 30, '{}')`, c1); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE contributor_profiles SET status = 'paused' WHERE id = $1`, c1); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO contributor_status_history (contributor_id, from_status, to_status, reason)
			VALUES ($1, 'active', 'paused', 'Trust score 0.55 below the floor 0.70')`, c1)
		return err
	})

	var o opsOverviewResp
	e.get(ops, "/v1/ops/overview", &o)

	// Each queue equals its list.
	var review struct{ Items []struct{ ID string } }
	e.get(ops, "/v1/review/items", &review)
	var reports struct{ Reports []struct{ ID string } }
	e.get(ops, "/v1/content-reports?status=open", &reports)
	var attempts struct{ Attempts []struct{ ID string } }
	e.get(ops, "/v1/qualification/attempts", &attempts)
	var requests struct{ Requests []struct{ ID string } }
	e.get(ops, "/v1/project-requests?status=submitted", &requests)
	var apps struct{ Contributors []struct{ ID string } }
	e.get(ops, "/v1/contributors?status=qualifying&screening=pending", &apps)
	if o.Queues.Review != len(review.Items) || o.Queues.Review < 4 || o.Queues.OldestReviewAt == nil {
		t.Errorf("review queue %d (oldest %v), list %d", o.Queues.Review, o.Queues.OldestReviewAt, len(review.Items))
	}
	if o.Queues.Reports != len(reports.Reports) || o.Queues.Attempts != len(attempts.Attempts) ||
		o.Queues.ProjectRequests != len(requests.Requests) || o.Queues.Applications != len(apps.Contributors) {
		t.Errorf("queues %+v; lists: reports %d attempts %d requests %d applications %d", o.Queues,
			len(reports.Reports), len(attempts.Attempts), len(requests.Requests), len(apps.Contributors))
	}

	// Projects: the at-risk one first; the review one with its counts.
	if len(o.Projects) == 0 || o.Projects[0].ID != late || o.Projects[0].Pace.State != "at_risk" {
		t.Fatalf("first project: %+v", o.Projects)
	}
	found := false
	for _, p := range o.Projects {
		if p.ID == project {
			found = true
			if p.Total != 4 || p.InReview != 4 || p.Final != 0 || p.ActiveContributors != 2 || p.GoldAccuracy != nil {
				t.Errorf("review project: %+v", p)
			}
		}
		if p.Status == "closed" {
			t.Errorf("closed project listed: %+v", p)
		}
	}
	if !found {
		t.Fatal("review project missing")
	}

	// Contributor health: the automatic pause is counted and listed.
	c := o.Contributors
	if c.Floor != 0.70 || c.AutoPaused != 1 || c.WorkedRecently < 2 {
		t.Errorf("health: %+v", c)
	}
	if len(c.Watch) == 0 || c.Watch[0].Score != 0.55 || c.Watch[0].Status != "paused" || !c.Watch[0].AutoPaused {
		t.Errorf("watch: %+v", c.Watch)
	}
}

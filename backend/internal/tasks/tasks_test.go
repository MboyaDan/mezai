package tasks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
	"github.com/MboyaDan/mezai/backend/seed"
)

// bigProject creates a live classification project with no skill
// requirements, overlap k, the gold rate, and regular and gold items.
func bigProject(t *testing.T, pool *db.Pool, k, regular, gold int, goldRate float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO projects (org_id, template_id, task_type, name, status, instructions_md, config, overlap_k, gold_rate, published_at)
			SELECT o.id, t.id, 'classification', 'Gold rate check (Sample)', 'live', 'Sample.',
			       '{"labels":["Yes","No"],"multi_label":false}', $1, $2, now()
			FROM organisations o, task_templates t
			WHERE o.slug = 'sample-client-ltd' AND t.task_type = 'classification'
			ORDER BY t.version DESC LIMIT 1
			RETURNING id`, k, goldRate).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO items (org_id, project_id, external_id, content)
			SELECT p.org_id, p.id, 'r' || n, jsonb_build_object('text', 'Sample sentence ' || n)
			FROM projects p, generate_series(1, $2) n WHERE p.id = $1`, id, regular); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			WITH g AS (
			  INSERT INTO items (org_id, project_id, external_id, content)
			  SELECT p.org_id, p.id, 'g' || n, jsonb_build_object('text', 'Sample gold sentence ' || n)
			  FROM projects p, generate_series(1, $2) n WHERE p.id = $1
			  RETURNING id, org_id, project_id)
			INSERT INTO gold_items (org_id, project_id, item_id, expected)
			SELECT org_id, project_id, id, '{"labels":["Yes"]}' FROM g`, id, gold)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// workers are the active, verified sample contributors.
func workers(t *testing.T, pool *db.Pool) []*auth.Principal {
	t.Helper()
	var out []*auth.Principal
	err := pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT id, user_id FROM contributor_profiles
			WHERE status = 'active' AND identity_status = 'verified' ORDER BY public_code`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p := &auth.Principal{Kind: auth.KindContributor}
			if err := rows.Scan(&p.ContributorID, &p.UserID); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil || len(out) < 5 {
		t.Fatalf("workers: %d, %v", len(out), err)
	}
	return out
}

func keys(t *testing.T, v any) []string {
	b, _ := json.Marshal(v)
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestGoldRateAndOverlap is AC-06: over 1,000 assignments gold appears at
// the configured rate, no contributor gets an item twice, no regular item
// passes k contributors, and gold tasks look exactly like regular ones.
func TestGoldRateAndOverlap(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	if _, err := seed.Run(ctx, pool, seed.Options{Password: "sample-password-only"}); err != nil {
		t.Fatal(err)
	}
	const k, rate, total = 5, 0.10, 1000
	project := bigProject(t, pool, k, 250, 40, rate)
	ws := workers(t, pool)
	svc := tasks.NewService(pool).WithRand(rand.New(rand.NewPCG(7, 11)).Float64)

	var goldTask, regularTask *tasks.Task
	isGold := func(taskID uuid.UUID) bool {
		var g bool
		_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gold_items g JOIN assignments a ON a.item_id = g.item_id WHERE a.id = $1)`, taskID).Scan(&g)
		})
		return g
	}
	goldCount := 0
	for i := range total {
		w := ws[i%len(ws)]
		task, err := svc.Next(ctx, w, project)
		if err != nil {
			t.Fatalf("assignment %d: %v", i, err)
		}
		if isGold(task.ID) {
			goldCount++
			if goldTask == nil {
				goldTask = &task
			}
		} else if regularTask == nil {
			regularTask = &task
		}
		res, err := svc.Submit(ctx, w, task.ID, fmt.Sprintf("key-%06d", i),
			tasks.SubmitInput{Answer: json.RawMessage(`{"labels":["Yes"]}`)}, auth.Meta{})
		if err != nil || res.Status != "submitted" {
			t.Fatalf("submit %d: %+v %v", i, res, err)
		}
	}

	// Rate: binomial(1000, 0.10) has a standard deviation of about 9.5, so
	// ±30 is over 3 deviations. The seeded source makes the run repeatable.
	if goldCount < 70 || goldCount > 130 {
		t.Fatalf("gold served %d times in %d assignments, want about %d", goldCount, total, int(rate*total))
	}
	t.Logf("gold share: %d / %d", goldCount, total)

	// Overlap and uniqueness, counted in the database.
	var dupes, overK, notK int
	_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT item_id, contributor_id FROM assignments
			WHERE project_id = $1 GROUP BY 1, 2 HAVING count(*) > 1) d`, project).Scan(&dupes); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM items i WHERE i.project_id = $1
			AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
			AND (SELECT count(*) FROM submissions s WHERE s.item_id = i.id AND s.status = 'submitted') > $2`, project, k).Scan(&overK); err != nil {
			return err
		}
		// Every regular item that reached awaiting_consensus has exactly k distinct contributors.
		return tx.QueryRow(ctx, `SELECT count(*) FROM items i WHERE i.project_id = $1 AND i.status = 'awaiting_consensus'
			AND (SELECT count(DISTINCT s.contributor_id) FROM submissions s WHERE s.item_id = i.id) <> $2`, project, k).Scan(&notK)
	})
	if dupes != 0 || overK != 0 || notK != 0 {
		t.Fatalf("duplicates %d, items over k %d, consensus items without exactly k contributors %d", dupes, overK, notK)
	}

	// Indistinguishable: the same fields, content of the same shape, and an
	// ID that is the assignment's, never the item's.
	if goldTask == nil || regularTask == nil {
		t.Fatal("expected both kinds of task")
	}
	if !slices.Equal(keys(t, *goldTask), keys(t, *regularTask)) {
		t.Fatalf("task fields differ: %v vs %v", keys(t, *goldTask), keys(t, *regularTask))
	}
	var gc, rc map[string]any
	_ = json.Unmarshal(goldTask.Content, &gc)
	_ = json.Unmarshal(regularTask.Content, &rc)
	if !slices.Equal(keys(t, gc), keys(t, rc)) {
		t.Fatalf("content fields differ: %v vs %v", gc, rc)
	}
}

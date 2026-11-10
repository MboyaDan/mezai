package tasks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

// qProject creates a live project with regular items (content) and gold
// items (content, expected), no audit sampling and no QA warnings unless
// the test sets them.
func qProject(t *testing.T, pool *db.Pool, taskType string, config any, k int, goldRate float64,
	regular []map[string]string, gold [][2]any) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	cfg, _ := json.Marshal(config)
	var id uuid.UUID
	err := pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO projects (org_id, template_id, task_type, name, status, instructions_md, config, overlap_k, gold_rate,
			  audit_sample_rate, qa_checks, published_at)
			SELECT o.id, t.id, $1, 'Quality check (Sample)', 'live', 'Sample.', $2, $3, $4, 0, '{"warn_fast_seconds":0}', now()
			FROM organisations o, task_templates t
			WHERE o.slug = 'sample-client-ltd' AND t.task_type = $1 ORDER BY t.version DESC LIMIT 1
			RETURNING id`, taskType, cfg, k, goldRate).Scan(&id); err != nil {
			return err
		}
		for i, c := range regular {
			b, _ := json.Marshal(c)
			if _, err := tx.Exec(ctx, `INSERT INTO items (org_id, project_id, external_id, content)
				SELECT org_id, id, $2, $3 FROM projects WHERE id = $1`, id, fmt.Sprintf("r%d", i), b); err != nil {
				return err
			}
		}
		for i, g := range gold {
			c, _ := json.Marshal(g[0])
			e, _ := json.Marshal(g[1])
			if _, err := tx.Exec(ctx, `WITH it AS (INSERT INTO items (org_id, project_id, external_id, content)
				SELECT org_id, id, $2, $3 FROM projects WHERE id = $1 RETURNING id, org_id, project_id)
				INSERT INTO gold_items (org_id, project_id, item_id, expected) SELECT org_id, project_id, id, $4 FROM it`,
				id, fmt.Sprintf("g%d", i), c, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func staffQuery[T any](t *testing.T, pool *db.Pool, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), sql, args...).Scan(&v)
	}); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func setSetting(t *testing.T, pool *db.Pool, key, value string) {
	t.Helper()
	if err := pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE settings SET value = $2 WHERE key = $1`, key, value)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// work takes the next task and submits answer for it.
func work(t *testing.T, svc *tasks.Service, p *auth.Principal, project uuid.UUID, answer string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	task, err := svc.Next(ctx, p, project)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if _, err := svc.Submit(ctx, p, task.ID, "q-"+task.ID.String(), tasks.SubmitInput{Answer: json.RawMessage(answer),
		Signals: quality.Signals{TimeOnTaskMs: 60_000}}, auth.Meta{}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	return task.ID
}

var labels3 = map[string]any{"labels": []string{"Yes", "No", "Maybe"}, "multi_label": false}

// TestConsensusClassification: majority with agreement per rater, and no
// majority going to review (brief 7 and 8.3).
func TestConsensusClassification(t *testing.T) {
	pool, _ := setup(t)
	project := qProject(t, pool, quality.Classification, labels3, 3, 0,
		[]map[string]string{{"text": "Sample one."}}, nil)
	people := crowd(t, pool, 3)
	svc := tasks.NewService(pool)
	for i, ans := range []string{`{"labels":["Yes"]}`, `{"labels":["Yes"]}`, `{"labels":["No"]}`} {
		work(t, svc, people[i], project, ans)
	}
	if s := staffQuery[string](t, pool, `SELECT status FROM items WHERE project_id = $1`, project); s != "consensus" {
		t.Fatalf("item status %s", s)
	}
	if r := staffQuery[string](t, pool, `SELECT result::text FROM consensus_results WHERE project_id = $1`, project); r != `{"labels": ["Yes"]}` {
		t.Fatalf("result %s", r)
	}
	if a := staffQuery[float64](t, pool, `SELECT agreement FROM consensus_results WHERE project_id = $1`, project); a < 0.66 || a > 0.67 {
		t.Fatalf("agreement %v", a)
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM submissions WHERE project_id = $1 AND agrees_with_final = (contributor_id <> $2)`, project, people[2].ContributorID); n != 3 {
		t.Fatalf("agreement flags: %d of 3 right", n)
	}
	// Each rater's trust now has an agreement input.
	if n := staffQuery[int](t, pool, `SELECT count(DISTINCT contributor_id) FROM trust_scores WHERE agreement_rate IS NOT NULL
		AND contributor_id = ANY($1)`, []uuid.UUID{people[0].ContributorID, people[1].ContributorID, people[2].ContributorID}); n != 3 {
		t.Fatalf("trust recomputed for %d raters", n)
	}

	// Three different labels: no majority, so review; agreement not recorded.
	p2 := qProject(t, pool, quality.Classification, labels3, 3, 0, []map[string]string{{"text": "Sample two."}}, nil)
	for i, ans := range []string{`{"labels":["Yes"]}`, `{"labels":["No"]}`, `{"labels":["Maybe"]}`} {
		work(t, svc, people[i], p2, ans)
	}
	if s := staffQuery[string](t, pool, `SELECT status || ':' || route_reason FROM items i JOIN consensus_results c ON c.item_id = i.id WHERE i.project_id = $1`, p2); s != "in_review:no_majority" {
		t.Fatalf("no majority: %s", s)
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM submissions WHERE project_id = $1 AND agrees_with_final IS NOT NULL`, p2); n != 0 {
		t.Fatal("agreement recorded before a final answer exists")
	}
}

// TestConsensusRouting: score spread, differing corrections, QA flags and
// the audit sample all send an item to review (brief 8.3).
func TestConsensusRouting(t *testing.T) {
	pool, ctx := setup(t)
	people := crowd(t, pool, 2)
	svc := tasks.NewService(pool)
	route := func(project uuid.UUID) string {
		return staffQuery[string](t, pool, `SELECT coalesce(route_reason, 'none') FROM consensus_results WHERE project_id = $1`, project)
	}

	rubric := map[string]any{"criteria": []string{"accuracy"}, "scale": []int{1, 5}}
	pw := qProject(t, pool, quality.Pairwise, rubric, 2, 0,
		[]map[string]string{{"prompt": "Sample?", "response_a": "Sample A.", "response_b": "Sample B."}}, nil)
	work(t, svc, people[0], pw, `{"preference":"a","scores_a":{"accuracy":5},"scores_b":{"accuracy":2},"rationale":"A is clearer."}`)
	work(t, svc, people[1], pw, `{"preference":"a","scores_a":{"accuracy":1},"scores_b":{"accuracy":2},"rationale":"A is better."}`)
	if r := route(pw); r != "spread" {
		t.Fatalf("rubric spread 4 > 2: %s", r)
	}

	tr := qProject(t, pool, quality.TranslationReview, map[string]any{"criteria": []string{"adequacy", "fluency"}, "scale": []int{1, 5}}, 2, 0,
		[]map[string]string{{"source": "Good morning", "translation": "Habari ya jioni", "source_lang": "en", "target_lang": "sw"}}, nil)
	work(t, svc, people[0], tr, `{"adequacy":3,"fluency":4,"corrected":"Habari ya asubuhi"}`)
	work(t, svc, people[1], tr, `{"adequacy":3,"fluency":4,"corrected":"Habari za asubuhi"}`)
	if r := route(tr); r != "no_majority" {
		t.Fatalf("different corrections: %s", r)
	}

	same := []map[string]string{{"text": "Sample flag."}}
	fl := qProject(t, pool, quality.Classification, labels3, 1, 0, same, nil)
	_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE projects SET qa_checks = '{"warn_fast_seconds":120}' WHERE id = $1`, fl)
		return err
	})
	work(t, svc, people[0], fl, `{"labels":["Yes"]}`) // 60 s < 120 s: a too_fast warning
	if r := route(fl); r != "flagged" {
		t.Fatalf("QA warning: %s", r)
	}

	au := qProject(t, pool, quality.Classification, labels3, 1, 0, []map[string]string{{"text": "Sample audit."}}, nil)
	_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE projects SET audit_sample_rate = 1 WHERE id = $1`, au)
		return err
	})
	work(t, svc, people[0], au, `{"labels":["Yes"]}`)
	if r := route(au); r != "audit_sample" {
		t.Fatalf("audit sample: %s", r)
	}
	// k = 1 with no reason: consensus, and no agreement input (one rater).
	ok := qProject(t, pool, quality.Classification, labels3, 1, 0, []map[string]string{{"text": "Sample solo."}}, nil)
	work(t, svc, people[1], ok, `{"labels":["No"]}`)
	if r := route(ok); r != "none" {
		t.Fatalf("k=1: %s", r)
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM submissions WHERE project_id = $1 AND agrees_with_final IS NOT NULL`, ok); n != 0 {
		t.Fatal("a single rater counted as agreeing")
	}
}

// TestGoldTrustAndAutoPause: gold is scored, the result is never in the
// submit answer, the score builds until the minimum, and a contributor
// below the floor is paused automatically with Ops notified (AC-08).
func TestGoldTrustAndAutoPause(t *testing.T) {
	pool, ctx := setup(t)
	setSetting(t, pool, "trust.min_gold_results", "3")
	gold := [][2]any{}
	for i := range 5 {
		gold = append(gold, [2]any{map[string]string{"text": fmt.Sprintf("Sample gold %d.", i)}, map[string]any{"labels": []string{"Yes"}}})
	}
	regular := []map[string]string{}
	for i := range 5 {
		regular = append(regular, map[string]string{"text": fmt.Sprintf("Sample %d.", i)})
	}
	project := qProject(t, pool, quality.Classification, labels3, 1, 0.5, regular, gold)
	p := crowd(t, pool, 1)[0]
	svc := tasks.NewService(pool, tasks.Options{NotifyTo: "ops@mezai.invalid", AppOrigin: "http://localhost:3000"}).
		WithRand(func() float64 { return 0 }) // always gold, never audit

	for i := range 3 {
		work(t, svc, p, project, `{"labels":["No"]}`) // wrong on gold
		status := staffQuery[string](t, pool, `SELECT status FROM contributor_profiles WHERE id = $1`, p.ContributorID)
		if i < 2 && status != "active" {
			t.Fatalf("paused while still building (gold %d): %s", i+1, status)
		}
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM gold_results WHERE contributor_id = $1 AND NOT correct`, p.ContributorID); n != 3 {
		t.Fatalf("gold results: %d", n)
	}
	if s := staffQuery[float64](t, pool, `SELECT score FROM trust_scores WHERE contributor_id = $1 ORDER BY created_at DESC LIMIT 1`, p.ContributorID); s != 0 {
		t.Fatalf("score %v", s)
	}
	if s := staffQuery[string](t, pool, `SELECT status FROM contributor_profiles WHERE id = $1`, p.ContributorID); s != "paused" {
		t.Fatalf("not paused: %s", s)
	}
	if r := staffQuery[string](t, pool, `SELECT reason FROM contributor_status_history WHERE contributor_id = $1 AND to_status = 'paused'`, p.ContributorID); r != "Trust score 0.00 below the floor 0.70" {
		t.Fatalf("history reason %q", r)
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM email_outbox WHERE template = 'trust_pause' AND to_address = 'ops@mezai.invalid'`); n != 1 {
		t.Fatalf("ops notices: %d", n)
	}
	if _, err := svc.Next(ctx, p, project); err == nil {
		t.Fatal("a paused contributor got a task")
	}

	// A right gold answer counts as correct; the submit answer looks the same.
	q := crowd(t, pool, 1)[0]
	task, _ := svc.Next(ctx, q, project)
	res, err := svc.Submit(ctx, q, task.ID, "right-"+task.ID.String(), tasks.SubmitInput{Answer: json.RawMessage(`{"labels":["Yes"]}`),
		Signals: quality.Signals{TimeOnTaskMs: 60_000}}, auth.Meta{})
	if err != nil || res.Status != "submitted" {
		t.Fatalf("submit: %+v %v", res, err)
	}
	b, _ := json.Marshal(res)
	if string(b) != fmt.Sprintf(`{"submission_id":"%s","status":"submitted","warnings":[]}`, res.SubmissionID) {
		t.Fatalf("submit answer reveals more than it should: %s", b)
	}
	if n := staffQuery[int](t, pool, `SELECT count(*) FROM gold_results WHERE contributor_id = $1 AND correct`, q.ContributorID); n != 1 {
		t.Fatal("right gold answer not scored correct")
	}
}

// TestProbation: below the minimum gold results the gold rate is doubled
// (capped); after it, the project's rate applies (brief 8.4).
func TestProbation(t *testing.T) {
	pool, ctx := setup(t)
	project := qProject(t, pool, quality.Classification, labels3, 1, 0.1,
		[]map[string]string{{"text": "Sample a."}, {"text": "Sample b."}},
		[][2]any{{map[string]string{"text": "Sample gold."}, map[string]any{"labels": []string{"Yes"}}}})
	p := crowd(t, pool, 1)[0]
	isGold := func(task uuid.UUID) bool {
		return staffQuery[bool](t, pool, `SELECT EXISTS (SELECT 1 FROM gold_items g JOIN assignments a ON a.item_id = g.item_id WHERE a.id = $1)`, task)
	}
	// A draw of 0.15: above the project's 0.10, below the probation 0.20.
	svc := tasks.NewService(pool).WithRand(func() float64 { return 0.15 })
	task, err := svc.Next(ctx, p, project)
	if err != nil || !isGold(task.ID) {
		t.Fatalf("probation should serve gold at 0.15: %v", err)
	}
	if err := svc.Skip(ctx, p, task.ID, "unclear", "", auth.Meta{}); err != nil {
		t.Fatal(err)
	}
	setSetting(t, pool, "trust.min_gold_results", "0") // probation over
	task, err = svc.Next(ctx, p, project)
	if err != nil || isGold(task.ID) {
		t.Fatalf("after probation 0.15 should be regular: %v", err)
	}
}

package tasks_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

// TestJobRunner: daily jobs are queued once per date, each runs once even
// with several runners at the same time, and a failure is retried later.
func TestJobRunner(t *testing.T) {
	pool, ctx := setup(t)
	now := time.Date(2026, 11, 10, 3, 0, 0, 0, jobs.Nairobi)
	var ran atomic.Int64
	var days sync.Map
	ok := func(_ context.Context, p json.RawMessage) error {
		ran.Add(1)
		var v struct{ Day string }
		_ = json.Unmarshal(p, &v)
		days.Store(v.Day, true)
		return nil
	}
	fail := func(context.Context, json.RawMessage) error { return errors.New("sample failure") }
	runner := func() *jobs.Runner {
		return &jobs.Runner{Pool: pool, Now: func() time.Time { return now }, Daily: []jobs.Daily{{Kind: "test_ok", At: time.Hour}, {Kind: "test_fail", At: time.Hour}},
			Handlers: map[string]jobs.Handler{"test_ok": ok, "test_fail": fail}}
	}
	for range 3 {
		if err := runner().EnqueueDaily(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for range 4 { // four "instances" at once
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = runner().RunDue(ctx) }()
	}
	wg.Wait()
	if ran.Load() != 1 {
		t.Fatalf("daily job ran %d times", ran.Load())
	}
	if _, ok := days.Load("2026-11-09"); !ok {
		t.Fatal("a nightly job reports on the previous day")
	}
	status := staffQuery[string](t, pool, `SELECT status || ':' || attempts || ':' || coalesce(last_error, '') || ':' || (run_at > now())::text
		FROM jobs WHERE kind = 'test_fail'`)
	if status != "queued:1:sample failure:true" {
		t.Fatalf("failed job: %s", status)
	}
}

// TestAgreementSnapshotAndReport: the snapshot matches the maths on real
// submissions, and the report shows bands and review figures.
func TestAgreementSnapshotAndReport(t *testing.T) {
	pool, ctx := setup(t)
	texts := []map[string]string{{"text": "Sample 1."}, {"text": "Sample 2."}, {"text": "Sample 3."}, {"text": "Sample 4."}}
	project := qProject(t, pool, quality.Classification, map[string]any{"labels": []string{"a", "b"}}, 2, 0, texts, nil)
	people := crowd(t, pool, 2)
	svc := tasks.NewService(pool)
	// Rater 1: a, b, a, b. Rater 2: a, b, b, b (by item text). Three of four agree.
	plan := map[string][2]string{"Sample 1.": {"a", "a"}, "Sample 2.": {"b", "b"}, "Sample 3.": {"a", "b"}, "Sample 4.": {"b", "b"}}
	for r, p := range people {
		for range 4 {
			task, err := svc.Next(ctx, p, project)
			if err != nil {
				t.Fatal(err)
			}
			var content map[string]string
			_ = json.Unmarshal(task.Content, &content)
			label := plan[content["text"]][r]
			if _, err := svc.Submit(ctx, p, task.ID, "s-"+task.ID.String(), tasks.SubmitInput{Answer: json.RawMessage(`{"labels":["` + label + `"]}`),
				Signals: quality.Signals{TimeOnTaskMs: 60_000}}, auth.Meta{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	now := time.Now()
	n, err := svc.SnapshotAgreement(ctx, now.Add(-time.Hour), now.Add(time.Minute), nil)
	if err != nil || n == 0 {
		t.Fatalf("snapshot: %d %v", n, err)
	}
	get := func(stat string) float64 {
		return staffQuery[float64](t, pool, `SELECT value FROM agreement_snapshots WHERE project_id = $1 AND label IS NULL AND statistic = $2`, project, stat)
	}
	if get("percent") != 0.75 || get("cohen_kappa") != 0.5 {
		t.Fatalf("percent %v cohen %v", get("percent"), get("cohen_kappa"))
	}
	if c := staffQuery[int](t, pool, `SELECT count(*) FROM agreement_snapshots WHERE project_id = $1 AND label IS NOT NULL`, project); c != 4 {
		t.Fatalf("per-label rows: %d (percent and kappa for a and b)", c)
	}
	// Running it again for the same window adds a new snapshot (history), and the report uses the latest.
	staff := staffPrincipal(t, pool)
	rep, err := svc.GetProjectQuality(ctx, staff, project)
	if err != nil {
		t.Fatal(err)
	}
	var kappa *tasks.AgreementFigure
	for i, f := range rep.Agreement {
		if f.Label == nil && f.Statistic == "cohen_kappa" {
			kappa = &rep.Agreement[i]
		}
	}
	if kappa == nil || kappa.Band == nil || *kappa.Band != "moderate" || rep.Items != 4 || rep.Completed != 4 || rep.Contributors != 2 {
		t.Fatalf("report: %+v", rep)
	}
	// Item 3 split a/b between two raters: a tie, so it went to review.
	if rep.RoutedToReview != 1 || rep.ReviewRate == nil || *rep.ReviewRate != 0.25 || rep.RouteReasons["no_majority"] != 1 {
		t.Fatalf("review figures: %+v", rep)
	}
}

// TestGoldFeedbackBatch: one note per contributor and project with misses,
// never naming an item; nothing for a day without misses.
func TestGoldFeedbackBatch(t *testing.T) {
	pool, ctx := setup(t)
	gold := [][2]any{}
	for _, s := range []string{"Sample gold one.", "Sample gold two."} {
		gold = append(gold, [2]any{map[string]string{"text": s}, map[string]any{"labels": []string{"Yes"}}})
	}
	project := qProject(t, pool, quality.Classification, labels3, 1, 0.5,
		[]map[string]string{{"text": "Sample x."}, {"text": "Sample y."}}, gold)
	p := crowd(t, pool, 1)[0]
	svc := tasks.NewService(pool).WithRand(func() float64 { return 0 })
	work(t, svc, p, project, `{"labels":["No"]}`)  // a miss
	work(t, svc, p, project, `{"labels":["Yes"]}`) // a match
	now := time.Now()
	n, err := svc.GoldFeedback(ctx, now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("feedback notes: %d %v", n, err)
	}
	fb, err := svc.MyFeedback(ctx, p)
	if err != nil || len(fb) != 1 || !fb[0].Automatic || fb[0].ReadAt != nil {
		t.Fatalf("my feedback: %+v %v", fb, err)
	}
	if !strings.Contains(fb[0].Body, "1 of your 2 answers") || strings.Contains(fb[0].Body, "Sample gold") || strings.Contains(fb[0].Body, "gold") {
		t.Fatalf("body names more than it should: %q", fb[0].Body)
	}
	if err := svc.MarkFeedbackRead(ctx, p, fb[0].ID); err != nil {
		t.Fatal(err)
	}
	if fb, _ = svc.MyFeedback(ctx, p); fb[0].ReadAt == nil {
		t.Fatal("not marked read")
	}
	other := crowd(t, pool, 1)[0]
	if fb, _ := svc.MyFeedback(ctx, other); len(fb) != 0 {
		t.Fatal("someone else's feedback visible")
	}
	if n, _ := svc.GoldFeedback(ctx, now.Add(time.Hour), now.Add(2*time.Hour)); n != 0 {
		t.Fatal("feedback written for a day without results")
	}
}

// TestWarmup: a contributor's first tasks on a project come at the warm-up
// gold rate; after them the project's rate applies (D-100).
func TestWarmup(t *testing.T) {
	pool, _ := setup(t)
	setSetting(t, pool, "trust.probation_gold_multiplier", "1")
	setSetting(t, pool, "warmup.tasks", "2")
	regular := []map[string]string{}
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		regular = append(regular, map[string]string{"text": "Sample " + s + "."})
	}
	gold := [][2]any{}
	for _, s := range []string{"a", "b", "c", "d"} {
		gold = append(gold, [2]any{map[string]string{"text": "Sample gold " + s + "."}, map[string]any{"labels": []string{"Yes"}}})
	}
	project := qProject(t, pool, quality.Classification, labels3, 1, 0.1, regular, gold)
	p := crowd(t, pool, 1)[0]
	svc := tasks.NewService(pool).WithRand(func() float64 { return 0.3 }) // above 0.1, below the warm-up 0.5
	isGold := func(task uuid.UUID) bool {
		return staffQuery[bool](t, pool, `SELECT EXISTS (SELECT 1 FROM gold_items g JOIN assignments a ON a.item_id = g.item_id WHERE a.id = $1)`, task)
	}
	for i := range 3 {
		task := work(t, svc, p, project, `{"labels":["Yes"]}`)
		if want := i < 2; isGold(task) != want {
			t.Fatalf("task %d: gold %v, want %v", i+1, isGold(task), want)
		}
	}
}

// staffPrincipal is a staff caller for service-level reads.
func staffPrincipal(t *testing.T, pool *db.Pool) *auth.Principal {
	t.Helper()
	p := &auth.Principal{Kind: auth.KindStaff}
	if err := pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'ops@sample.invalid'`).Scan(&p.UserID)
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

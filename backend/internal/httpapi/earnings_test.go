package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type earningsResp struct {
	Totals []struct {
		Currency      string `json:"currency"`
		PendingCents  int64  `json:"pending_cents"`
		PendingTasks  int    `json:"pending_tasks"`
		ApprovedCents int64  `json:"approved_cents"`
		PaidCents     int64  `json:"paid_cents"`
		EarnedCents   int64  `json:"earned_cents"`
	} `json:"totals"`
	Tasks struct {
		Submitted   int `json:"submitted"`
		Waiting     int `json:"waiting"`
		Accepted    int `json:"accepted"`
		NotAccepted int `json:"not_accepted"`
	} `json:"tasks"`
	NextPayoutDate *string `json:"next_payout_date"`
	MinPayoutCents int64   `json:"min_payout_cents"`
}

type statementResp struct {
	Lines []struct {
		Day          string  `json:"day"`
		Type         string  `json:"type"`
		Tasks        int     `json:"tasks"`
		AmountCents  int64   `json:"amount_cents"`
		ProjectName  *string `json:"project_name"`
		Reason       *string `json:"reason"`
		MpesaReceipt *string `json:"mpesa_receipt"`
	} `json:"lines"`
	Before *string `json:"before"`
}

type payoutsResp struct {
	Payouts []struct {
		Status        string  `json:"status"`
		AmountCents   int64   `json:"amount_cents"`
		MpesaReceipt  *string `json:"mpesa_receipt"`
		FailureReason *string `json:"failure_reason"`
		PhoneMasked   string  `json:"phone_masked"`
	} `json:"payouts"`
}

func (e *env) get(s *session, path string, v any) {
	e.t.Helper()
	w := e.do(http.MethodGet, path, nil, s, nil)
	if w.Code != http.StatusOK {
		e.t.Fatalf("GET %s: %d %s", path, w.Code, w.Body)
	}
	decode(e.t, w, v)
}

// payContributor records a settled payout of amount for the contributor, as
// Stage 14 will: a batch created by Ops and approved by someone else, a paid
// line with a receipt and the negative ledger entry. A second, failed line
// in another batch moves no money.
func (e *env) payContributor(contributor string, amount int64, receipt string) {
	e.t.Helper()
	ops, admin := e.userID("ops@sample.invalid"), e.userID("superadmin@sample.invalid")
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		batch := func(key string) (string, error) {
			var id string
			err := tx.QueryRow(ctx, `INSERT INTO payout_batches (period_start, period_end, status, idempotency_key, created_by, approved_by, approved_at)
				VALUES (current_date - 7, current_date - 1, 'settled', $1, $2, $3, now()) RETURNING id`, key, ops, admin).Scan(&id)
			return id, err
		}
		paid, err := batch("test-paid-" + receipt)
		if err != nil {
			return err
		}
		var line string
		if err := tx.QueryRow(ctx, `INSERT INTO payout_lines (batch_id, contributor_id, amount_cents, currency, mpesa_phone, status, mpesa_receipt, recorded_by, recorded_at)
			SELECT $1, id, $2, 'KES', mpesa_phone, 'paid', $3, $4, now() FROM contributor_profiles WHERE id = $5 RETURNING id`,
			paid, amount, receipt, ops, contributor).Scan(&line); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, payout_line_id)
			VALUES ($1, 'payout', $2, 'KES', $3)`, contributor, -amount, line); err != nil {
			return err
		}
		failed, err := batch("test-failed-" + receipt)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO payout_lines (batch_id, contributor_id, amount_cents, currency, mpesa_phone, status, failure_reason, recorded_by, recorded_at)
			SELECT $1, id, 999, 'KES', mpesa_phone, 'failed', 'Number not registered (Sample)', $2, now() FROM contributor_profiles WHERE id = $3`,
			failed, ops, contributor)
		return err
	})
}

// TestEarningsReconcileWithLedger is Stage 12's done-when: every number a
// contributor sees equals the ledger, and they see only their own.
func TestEarningsReconcileWithLedger(t *testing.T) {
	e := newEnv(t)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)
	id1 := e.contributorID("contributor001@sample.invalid")
	id2 := e.contributorID("contributor002@sample.invalid")

	// Both answer four items alike: consensus, all eight answers accepted at 25.00.
	done := e.reviewProject(4)
	e.workAll(c1, done, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, done, map[string]any{"labels": []string{"Yes"}})
	// Only contributor 1 answers a second project: three answers pending (k = 2).
	open := e.reviewProject(3)
	e.workAll(c1, open, map[string]any{"labels": []string{"No"}})

	// A payout of 60.00, and 35 days of small adjustments (one a day) so the
	// statement needs two pages.
	e.payContributor(id1, 6000, "SAMPLE0001")
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, reason, created_by, created_at)
			SELECT $1, 'adjustment', CASE WHEN d % 5 = 0 THEN -50 ELSE 100 END, 'KES', 'Sample correction ' || d,
			  (SELECT id FROM users WHERE email = 'ops@sample.invalid'), now() - make_interval(days => d)
			FROM generate_series(1, 35) d`, id1)
		return err
	})

	ledger := func(id, filter string) int64 {
		var n int64
		e.staff(func(tx pgx.Tx) error {
			return tx.QueryRow(context.Background(),
				`SELECT coalesce(sum(amount_cents), 0) FROM earnings_ledger WHERE contributor_id = $1 AND `+filter, id).Scan(&n)
		})
		return n
	}

	var s earningsResp
	e.get(c1, "/v1/me/earnings", &s)
	if len(s.Totals) != 1 || s.Totals[0].Currency != "KES" {
		t.Fatalf("totals: %+v", s.Totals)
	}
	tot := s.Totals[0]
	checks := []struct {
		name      string
		got, want int64
	}{
		{"approved = ledger balance", tot.ApprovedCents, ledger(id1, "true")},
		{"earned = accruals + adjustments", tot.EarnedCents, ledger(id1, "entry_type IN ('accrual', 'adjustment')")},
		{"paid = -(payouts + reversals)", tot.PaidCents, -ledger(id1, "entry_type IN ('payout', 'payout_reversal')")},
		{"earned - paid = approved", tot.EarnedCents - tot.PaidCents, tot.ApprovedCents},
		{"accrued four answers", ledger(id1, "entry_type = 'accrual'"), 4 * 2500},
		{"paid 60.00", tot.PaidCents, 6000},
		{"pending three answers at 25.00", tot.PendingCents, 3 * 2500},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
	if tot.PendingTasks != 3 || s.Tasks.Submitted != 7 || s.Tasks.Accepted != 4 || s.Tasks.Waiting != 3 || s.Tasks.NotAccepted != 0 {
		t.Errorf("counts: pending %d, tasks %+v", tot.PendingTasks, s.Tasks)
	}
	if s.NextPayoutDate != nil || s.MinPayoutCents != 50000 {
		t.Errorf("payout settings: next %v, min %d", s.NextPayoutDate, s.MinPayoutCents)
	}

	// Every statement line, across pages, sums to the balance; accepted work
	// is one line per day and project, never per task.
	var sum int64
	pages, accrualLines := 0, 0
	path := "/v1/me/earnings/statement"
	for {
		var st statementResp
		e.get(c1, path, &st)
		pages++
		for _, l := range st.Lines {
			sum += l.AmountCents
			if l.Type == "accrual" {
				accrualLines++
				if l.Tasks != 4 || l.AmountCents != 10000 {
					t.Errorf("accrual line: %+v", l)
				}
			}
			if l.Type == "payout" && (l.MpesaReceipt == nil || *l.MpesaReceipt != "SAMPLE0001") {
				t.Errorf("payout line without its receipt: %+v", l)
			}
			if l.Type == "adjustment" && l.Reason == nil {
				t.Errorf("adjustment without its reason: %+v", l)
			}
		}
		if st.Before == nil {
			break
		}
		path = "/v1/me/earnings/statement?before=" + *st.Before
		if pages > 5 {
			t.Fatal("statement does not end")
		}
	}
	if pages != 2 || accrualLines != 1 || sum != tot.ApprovedCents {
		t.Errorf("statement: %d pages, %d accrual lines, sum %d, balance %d", pages, accrualLines, sum, tot.ApprovedCents)
	}
	if w := e.do(http.MethodGet, "/v1/me/earnings/statement?before=yesterday", nil, c1, nil); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad cursor: %d", w.Code)
	}

	var p payoutsResp
	e.get(c1, "/v1/me/payouts", &p)
	if len(p.Payouts) != 2 {
		t.Fatalf("payouts: %+v", p.Payouts)
	}
	for _, x := range p.Payouts {
		if !strings.Contains(x.PhoneMasked, "••") {
			t.Errorf("phone not masked as expected: %q", x.PhoneMasked)
		}
		switch x.Status {
		case "paid":
			if x.AmountCents != 6000 || x.MpesaReceipt == nil {
				t.Errorf("paid line: %+v", x)
			}
		case "failed":
			if x.FailureReason == nil {
				t.Errorf("failed line: %+v", x)
			}
		default:
			t.Errorf("status %q", x.Status)
		}
	}

	// Contributor 2 sees only their own: four accruals, nothing else.
	var s2 earningsResp
	e.get(c2, "/v1/me/earnings", &s2)
	if len(s2.Totals) != 1 || s2.Totals[0].ApprovedCents != ledger(id2, "true") || s2.Totals[0].ApprovedCents != 10000 ||
		s2.Totals[0].PaidCents != 0 || s2.Totals[0].PendingCents != 0 {
		t.Errorf("contributor 2 totals: %+v", s2.Totals)
	}
	var st2 statementResp
	e.get(c2, "/v1/me/earnings/statement", &st2)
	if len(st2.Lines) != 1 || st2.Lines[0].Type != "accrual" || st2.Before != nil {
		t.Errorf("contributor 2 statement: %+v", st2)
	}
	var p2 payoutsResp
	e.get(c2, "/v1/me/payouts", &p2)
	if len(p2.Payouts) != 0 {
		t.Errorf("contributor 2 payouts: %+v", p2.Payouts)
	}

	// Staff without a contributor profile, and people without the permission.
	_, admin := e.login("superadmin@sample.invalid", pw)
	if w := e.do(http.MethodGet, "/v1/me/earnings", nil, admin, nil); w.Code != http.StatusForbidden || errorCode(w) != "not_a_contributor" {
		t.Errorf("super admin: %d %s", w.Code, w.Body)
	}
	_, ops := e.login("ops@sample.invalid", pw)
	if w := e.do(http.MethodGet, "/v1/me/payouts", nil, ops, nil); w.Code != http.StatusForbidden {
		t.Errorf("ops: %d", w.Code)
	}

	// A next payout date shows once Ops sets one; a malformed one hides itself.
	for value, want := range map[string]string{`"2026-12-04"`: "2026-12-04", `"soon"`: ""} {
		e.staff(func(tx pgx.Tx) error {
			_, err := tx.Exec(context.Background(), `UPDATE settings SET value = $1 WHERE key = 'payout.next_date'`, value)
			return err
		})
		var s earningsResp
		e.get(c1, "/v1/me/earnings", &s)
		got := ""
		if s.NextPayoutDate != nil {
			got = *s.NextPayoutDate
		}
		if got != want {
			t.Errorf("next payout date from %s: %q, want %q", value, got, want)
		}
	}
}

type qualityResp struct {
	AsOf  string `json:"as_of"`
	Trust *struct {
		Score        *float64 `json:"score"`
		GoldAccuracy *float64 `json:"gold_accuracy"`
		Building     bool     `json:"building"`
	} `json:"trust"`
	Trend []struct {
		Day   string   `json:"day"`
		Score *float64 `json:"score"`
	} `json:"trend"`
	Floor          float64 `json:"floor"`
	MinGoldResults int     `json:"min_gold_results"`
}

// TestQualityShowsStartOfDay: the contributor's score is the last one before
// today (Nairobi), so a task done today never moves it (D-120); Ops still
// sees the live score.
func TestQualityShowsStartOfDay(t *testing.T) {
	e := newEnv(t)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	id := e.contributorID("contributor001@sample.invalid")

	var q qualityResp
	e.get(c1, "/v1/me/quality", &q)
	if q.Trust != nil || len(q.Trend) != 0 || q.Floor != 0.70 || q.MinGoldResults != 20 {
		t.Fatalf("new contributor: %+v", q)
	}

	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO trust_scores (contributor_id, score, gold_accuracy, gold_count, window_days, weights, created_at) VALUES
			  ($1, 0.80, 0.80, 5, 30, '{}', (date_trunc('day', now() AT TIME ZONE 'Africa/Nairobi') - interval '3 days 2 hours') AT TIME ZONE 'Africa/Nairobi'),
			  ($1, 0.85, 0.85, 6, 30, '{}', (date_trunc('day', now() AT TIME ZONE 'Africa/Nairobi') - interval '1 day 5 hours') AT TIME ZONE 'Africa/Nairobi'),
			  ($1, 0.90, 0.90, 7, 30, '{}', (date_trunc('day', now() AT TIME ZONE 'Africa/Nairobi') - interval '1 hour') AT TIME ZONE 'Africa/Nairobi'),
			  ($1, 0.40, 0.40, 8, 30, '{}', now()),
			  ($1, 0.50, 0.50, 1, 30, '{}', now() - interval '45 days')`, id)
		return err
	})
	e.get(c1, "/v1/me/quality", &q)
	if q.Trust == nil || q.Trust.Score == nil || *q.Trust.Score != 0.90 || !q.Trust.Building {
		t.Fatalf("trust: %+v", q.Trust)
	}
	// Three days with a score in the window: not today, not 45 days ago.
	if len(q.Trend) != 3 {
		t.Fatalf("trend: %+v", q.Trend)
	}
	last := q.Trend[len(q.Trend)-1]
	if *last.Score != 0.90 {
		t.Errorf("last trend point %+v, want yesterday's 0.90", last)
	}
	for _, p := range q.Trend {
		if *p.Score == 0.40 || *p.Score == 0.50 {
			t.Errorf("trend includes today or a day outside 30 days: %+v", q.Trend)
		}
	}

	// The contributor's profile carries no live score; Ops sees the live one.
	var prof struct {
		Trust *struct{} `json:"trust"`
	}
	e.get(c1, "/v1/me/contributor", &prof)
	if prof.Trust != nil {
		t.Error("contributor profile shows the live trust score")
	}
	_, ops := e.login("ops@sample.invalid", pw)
	var detail struct {
		Profile struct {
			Trust *struct {
				Score *float64 `json:"score"`
			} `json:"trust"`
		} `json:"profile"`
	}
	e.get(ops, "/v1/contributors/"+id, &detail)
	if detail.Profile.Trust == nil || detail.Profile.Trust.Score == nil || *detail.Profile.Trust.Score != 0.40 {
		t.Errorf("ops trust: %+v", detail.Profile.Trust)
	}
}

// TestWorkSizesAndTraining: tasks left per contributor, and guidelines.
func TestWorkSizesAndTraining(t *testing.T) {
	e := newEnv(t)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)
	project := e.reviewProject(3)
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO project_guidelines (org_id, project_id, title, body_md, position)
			SELECT org_id, id, 'Sample guideline', 'Read the whole sentence first.', 1 FROM projects WHERE id = $1`, project)
		return err
	})

	left := func(s *session) (int, bool) {
		var r struct {
			Projects []struct {
				ID             string   `json:"id"`
				TasksLeft      *int     `json:"tasks_left"`
				MinutesPerTask *float64 `json:"minutes_per_task"`
			} `json:"projects"`
		}
		e.get(s, "/v1/me/projects", &r)
		for _, p := range r.Projects {
			if p.ID == project {
				if p.TasksLeft == nil {
					t.Fatal("tasks_left missing")
				}
				return *p.TasksLeft, p.MinutesPerTask != nil
			}
		}
		t.Fatal("project not listed")
		return 0, false
	}
	if n, timed := left(c1); n != 0 || timed {
		t.Errorf("contributor 1: %d left, timed %v", n, timed)
	}
	if n, _ := left(c2); n != 3 {
		t.Errorf("contributor 2: %d left, want 3", n)
	}

	var tr struct {
		Projects []struct {
			ID         string `json:"id"`
			Guidelines []struct {
				Title string `json:"title"`
			} `json:"guidelines"`
		} `json:"projects"`
	}
	e.get(c1, "/v1/me/training", &tr)
	found := false
	for _, p := range tr.Projects {
		if p.ID == project {
			found = len(p.Guidelines) == 1 && p.Guidelines[0].Title == "Sample guideline"
		}
	}
	if !found {
		t.Errorf("training: %+v", tr.Projects)
	}
}

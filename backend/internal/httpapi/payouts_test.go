package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
)

type payoutLineResp struct {
	ID            string  `json:"id"`
	PublicCode    string  `json:"public_code"`
	PhoneMasked   string  `json:"phone_masked"`
	AmountCents   int64   `json:"amount_cents"`
	Status        string  `json:"status"`
	FailureReason *string `json:"failure_reason"`
}

type payoutBatchResp struct {
	ID              string           `json:"id"`
	Status          string           `json:"status"`
	PeriodStart     string           `json:"period_start"`
	PeriodEnd       string           `json:"period_end"`
	TotalCents      int64            `json:"total_cents"`
	LineCount       int              `json:"line_count"`
	PaidCents       int64            `json:"paid_cents"`
	FormatConfirmed *bool            `json:"format_confirmed"`
	Lines           []payoutLineResp `json:"lines"`
}

func (b payoutBatchResp) line(code string) payoutLineResp {
	for _, l := range b.Lines {
		if l.PublicCode == code {
			return l
		}
	}
	return payoutLineResp{}
}

// TestPayoutsManualPath is AC-11: the creator cannot approve their own
// batch; the exported file's total equals the approved lines and each line
// is what the contributor was owed; after results are recorded every
// contributor's balance equals the sum of their ledger entries, a paid line
// left the balance once and a failed line moved no money.
func TestPayoutsManualPath(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")
	approver := e.signIn("approver@sample.invalid")
	opsID := e.userID("ops@sample.invalid")
	c := map[string]string{}
	for _, n := range []string{"001", "002", "003", "004"} {
		c[n] = e.contributorID("contributor" + n + "@sample.invalid")
	}
	pc := func(n string) string { return "C-0" + n }

	// Balances: 001 earned KES 1,200.50 before the period ended and 300 after;
	// 002 earned 800; 003 is owed 700 but identity is not verified; 004 is
	// owed only 200 (below the KES 500 minimum).
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		credit := func(id string, cents int64, at string) error {
			_, err := tx.Exec(ctx, `INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, reason, created_by, created_at)
				VALUES ($1, 'adjustment', $2, 'KES', 'Sample credit for tests', $3, `+at+`)`, id, cents, opsID)
			return err
		}
		for _, x := range []struct {
			id    string
			cents int64
			at    string
		}{{c["001"], 120050, "now() - interval '3 days'"}, {c["001"], 30000, "now()"}, {c["002"], 80000, "now() - interval '3 days'"},
			{c["003"], 70000, "now() - interval '3 days'"}, {c["004"], 20000, "now() - interval '3 days'"}} {
			if err := credit(x.id, x.cents, x.at); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE contributor_profiles SET identity_status = 'verified', identity_decided_by = $2, identity_decided_at = now(),
			mpesa_name_checked_at = now(), mpesa_name_checked_by = $2 WHERE id = ANY($1::uuid[])`,
			[]string{c["001"], c["002"], c["004"]}, opsID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE contributor_profiles SET identity_status = 'pending', identity_decided_by = NULL, identity_decided_at = NULL,
			mpesa_name_checked_at = now(), mpesa_name_checked_by = $2 WHERE id = $1`, c["003"], opsID)
		return err
	})

	yesterday := time.Now().In(time.FixedZone("EAT", 3*3600)).AddDate(0, 0, -1).Format(time.DateOnly)

	// Preview: who is paid and who is held, and why.
	var pv struct {
		TotalCents int64            `json:"total_cents"`
		Lines      []payoutLineResp `json:"lines"`
		Held       []struct {
			PublicCode string `json:"public_code"`
			Reason     string `json:"reason"`
		} `json:"held"`
	}
	e.get(ops, "/v1/payouts/preview?period_end="+yesterday, &pv)
	held := map[string]string{}
	for _, h := range pv.Held {
		held[h.PublicCode] = h.Reason
	}
	if held[pc("003")] != "identity_unverified" || held[pc("004")] != "below_minimum" {
		t.Fatalf("held: %+v", pv.Held)
	}
	if w := e.do(http.MethodGet, "/v1/payouts/preview?period_end="+time.Now().AddDate(0, 0, 2).Format(time.DateOnly), nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("future period: %d", w.Code)
	}

	// Create, idempotently.
	create := func(key string) (int, payoutBatchResp) {
		w := e.do(http.MethodPost, "/v1/payouts/batches", map[string]string{"period_end": yesterday}, ops, map[string]string{"Idempotency-Key": key})
		var b payoutBatchResp
		if w.Code < 300 {
			decode(t, w, &b)
		}
		return w.Code, b
	}
	status, b := create("batch-one")
	if status != http.StatusCreated || b.Status != "draft" {
		t.Fatalf("create: %d %+v", status, b)
	}
	if again, b2 := create("batch-one"); again != http.StatusOK || b2.ID != b.ID {
		t.Fatalf("replay: %d %s", again, b2.ID)
	}
	if other, _ := create("batch-two"); other != http.StatusConflict {
		t.Fatalf("second open batch: %d", other)
	}
	l1, l2 := b.line(pc("001")), b.line(pc("002"))
	if l1.AmountCents != 120000 || l2.AmountCents != 80000 || b.line(pc("003")).ID != "" || b.line(pc("004")).ID != "" {
		t.Fatalf("lines: %+v", b.Lines)
	}
	if strings.Contains(l1.PhoneMasked, "700000001") || l1.PhoneMasked == "" {
		t.Errorf("phone not masked: %q", l1.PhoneMasked)
	}
	var sum int64
	for _, l := range b.Lines {
		sum += l.AmountCents
	}
	if sum != b.TotalCents || len(b.Lines) != b.LineCount || b.TotalCents != pv.TotalCents {
		t.Fatalf("batch total %d (%d lines) vs lines %d (%d), preview %d", b.TotalCents, b.LineCount, sum, len(b.Lines), pv.TotalCents)
	}

	post := func(s *session, path string, body any) (int, payoutBatchResp) {
		w := e.do(http.MethodPost, path, body, s, nil)
		var out payoutBatchResp
		if w.Code == http.StatusOK {
			decode(t, w, &out)
		}
		return w.Code, out
	}
	base := "/v1/payouts/batches/" + b.ID

	// Nothing can be exported or approved before it is submitted.
	if code, _ := post(approver, base+"/approve", nil); code != http.StatusConflict {
		t.Fatalf("approve a draft: %d", code)
	}
	if code, _ := post(ops, base+"/submit", nil); code != http.StatusOK {
		t.Fatalf("submit: %d", code)
	}
	// Two-person rule: the API refuses the creator, and so does the table.
	// ops@ has no payout.approve, so the creator check is tested through a
	// Super Admin's own batch below and directly in the database here.
	if code, _ := post(ops, base+"/approve", nil); code != http.StatusForbidden {
		t.Fatalf("ops approves: %d", code)
	}
	err := e.pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE payout_batches SET status = 'approved', approved_by = created_by, approved_at = now() WHERE id = $1`, b.ID)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "payout_batches_check1") {
		t.Fatalf("database allowed the creator to approve: %v", err)
	}

	// Sent back, resubmitted, approved by the second person.
	if code, _ := post(approver, base+"/return", map[string]string{"reason": ""}); code != http.StatusUnprocessableEntity {
		t.Fatalf("return without a note: %d", code)
	}
	if code, out := post(approver, base+"/return", map[string]string{"reason": "Check the period (Sample)"}); code != http.StatusOK || out.Status != "draft" {
		t.Fatalf("return: %d %+v", code, out)
	}
	post(ops, base+"/submit", nil)
	if code, out := post(approver, base+"/approve", nil); code != http.StatusOK || out.Status != "approved" {
		t.Fatalf("approve: %d %+v", code, out)
	}
	if code, _ := post(ops, base+"/cancel", map[string]string{"reason": "Too late (Sample)"}); code != http.StatusConflict {
		t.Fatalf("cancel after approval: %d", code)
	}
	if code, _ := post(ops, "/v1/payouts/lines/"+l1.ID+"/result", map[string]string{"status": "paid", "receipt": "SAMPLE0A01"}); code != http.StatusConflict {
		t.Fatalf("result before export: %d", code)
	}

	// Export: the file total equals the approved lines.
	code, out := post(ops, base+"/export", nil)
	if code != http.StatusOK || out.Status != "exported" || out.FormatConfirmed == nil || *out.FormatConfirmed {
		t.Fatalf("export: %d %+v", code, out)
	}
	f := e.do(http.MethodGet, base+"/file", nil, approver, nil)
	if f.Code != http.StatusOK || !strings.HasPrefix(f.Header().Get("Content-Type"), "text/csv") || f.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("file: %d %v", f.Code, f.Header())
	}
	rows := strings.Split(strings.TrimSpace(f.Body.String()), "\r\n")
	if rows[0] != "Phone Number,Amount,Name,Reference" || len(rows) != 1+len(b.Lines) {
		t.Fatalf("file rows: %q", rows)
	}
	var fileTotal int64
	for _, r := range rows[1:] {
		cols := strings.Split(r, ",")
		var kes int64
		for _, ch := range cols[1] {
			kes = kes*10 + int64(ch-'0')
		}
		fileTotal += kes * 100
	}
	if fileTotal != b.TotalCents {
		t.Fatalf("file total %d, batch %d", fileTotal, b.TotalCents)
	}
	if !strings.Contains(f.Body.String(), "254700000001,1200,Sample Contributor 001,MEZAI C-0001 "+yesterday) {
		t.Fatalf("file line for 001: %s", f.Body.String())
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'payout_batch.downloaded' AND entity_id = $1`, b.ID); n != 1 {
		t.Fatalf("download audits: %d", n)
	}

	// Results: 001 paid (twice, idempotent), 002 failed; a receipt cannot
	// be used twice.
	result := func(line string, body map[string]string) (int, payoutBatchResp) {
		return post(ops, "/v1/payouts/lines/"+line+"/result", body)
	}
	if code, _ := result(l1.ID, map[string]string{"status": "paid", "receipt": "bad"}); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad receipt: %d", code)
	}
	if code, _ := result(l1.ID, map[string]string{"status": "paid", "receipt": "sample0a01"}); code != http.StatusOK {
		t.Fatalf("paid: %d", code)
	}
	if code, _ := result(l1.ID, map[string]string{"status": "paid", "receipt": "SAMPLE0A01"}); code != http.StatusOK {
		t.Fatalf("paid replay: %d", code)
	}
	if code, _ := result(l1.ID, map[string]string{"status": "failed", "reason": "x"}); code != http.StatusConflict {
		t.Fatalf("paid then failed: %d", code)
	}
	if code, _ := result(l2.ID, map[string]string{"status": "paid", "receipt": "SAMPLE0A01"}); code != http.StatusConflict {
		t.Fatalf("receipt reused: %d", code)
	}
	// The seed's sample balances (006, 008) are in the batch too; pay them.
	paid := int64(120000)
	for i, l := range b.Lines {
		if l.ID == l1.ID || l.ID == l2.ID {
			continue
		}
		if code, _ := result(l.ID, map[string]string{"status": "paid", "receipt": fmt.Sprintf("SAMPLE0B%02d", i)}); code != http.StatusOK {
			t.Fatalf("paid %s: %d", l.PublicCode, code)
		}
		paid += l.AmountCents
	}
	code, out = result(l2.ID, map[string]string{"status": "failed", "reason": "Number not registered for M-Pesa (Sample)"})
	if code != http.StatusOK || out.Status != "settled" || out.PaidCents != paid {
		t.Fatalf("failed and settled: %d %+v", code, out)
	}

	// Reconciliation: one payout entry, for the paid line only; every
	// balance equals its ledger sum and what the contributor sees.
	if n := e.count(`SELECT count(*) FROM earnings_ledger WHERE entry_type = 'payout' AND payout_line_id = $1 AND amount_cents = -120000`, l1.ID); n != 1 {
		t.Fatalf("payout entries for the paid line: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM earnings_ledger WHERE payout_line_id = $1`, l2.ID); n != 0 {
		t.Fatalf("ledger entries for the failed line: %d", n)
	}
	want := map[string]int64{"001": 150050 - 120000, "002": 80000, "003": 70000, "004": 20000}
	for n, cents := range want {
		if got := e.count(`SELECT coalesce(sum(amount_cents), 0)::int FROM earnings_ledger WHERE contributor_id = $1`, c[n]); int64(got) != cents {
			t.Errorf("contributor %s ledger %d, want %d", n, got, cents)
		}
	}
	var mine struct {
		Totals []struct {
			ApprovedCents int64 `json:"approved_cents"`
			PaidCents     int64 `json:"paid_cents"`
		} `json:"totals"`
	}
	e.get(e.signIn("contributor001@sample.invalid"), "/v1/me/earnings", &mine)
	if len(mine.Totals) != 1 || mine.Totals[0].ApprovedCents != want["001"] || mine.Totals[0].PaidCents != 120000 {
		t.Fatalf("contributor 001 sees %+v", mine.Totals)
	}
	if n := e.count(`SELECT count(*) FROM payout_lines WHERE batch_id = $1 AND status = 'pending'`, b.ID); n != 0 {
		t.Fatalf("pending lines in a settled batch: %d", n)
	}

	// A recorded line can never change, even straight in the database.
	err = e.pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE payout_lines SET status = 'failed', failure_reason = 'x' WHERE id = $1`, l1.ID)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "already has a result") {
		t.Fatalf("recorded line changed: %v", err)
	}

	// The next batch starts after this one; Super Admin cannot approve a
	// batch they created; a cancelled batch frees the slot.
	admin := e.signIn("superadmin@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/payouts/batches", map[string]string{"period_end": yesterday}, admin, map[string]string{"Idempotency-Key": "batch-three"}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlapping period: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE payout_batches SET period_end = period_end - 2, period_start = LEAST(period_start, period_end - 2) WHERE id = $1`, b.ID)
		return err
	})
	w := e.do(http.MethodPost, "/v1/payouts/batches", map[string]string{"period_end": yesterday}, admin, map[string]string{"Idempotency-Key": "batch-four"})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin batch: %d %s", w.Code, w.Body)
	}
	var mineB payoutBatchResp
	decode(t, w, &mineB)
	if l := mineB.line(pc("002")); l.AmountCents != 80000 {
		t.Fatalf("failed amount not owed again: %+v", mineB.Lines)
	}
	post(admin, "/v1/payouts/batches/"+mineB.ID+"/submit", nil)
	if code, _ := post(admin, "/v1/payouts/batches/"+mineB.ID+"/approve", nil); code != http.StatusForbidden {
		t.Fatalf("super admin approves own batch: %d", code)
	}
	if code, out := post(admin, "/v1/payouts/batches/"+mineB.ID+"/cancel", map[string]string{"reason": "Wrong period (Sample)"}); code != http.StatusOK || out.Status != "cancelled" {
		t.Fatalf("cancel: %d %+v", code, out)
	}
	if n := e.count(`SELECT count(*) FROM payout_batches WHERE status NOT IN ('settled', 'cancelled')`); n != 0 {
		t.Fatalf("open batches: %d", n)
	}

	// Next payout date for contributors.
	if code := e.do(http.MethodPut, "/v1/payouts/next-date", map[string]string{"date": "2020-01-01"}, ops, nil).Code; code != http.StatusUnprocessableEntity {
		t.Fatalf("past date: %d", code)
	}
	next := time.Now().AddDate(0, 0, 7).Format(time.DateOnly)
	if code := e.do(http.MethodPut, "/v1/payouts/next-date", map[string]string{"date": next}, ops, nil).Code; code != http.StatusOK {
		t.Fatalf("next date: %d", code)
	}
	var list struct {
		NextDate *string `json:"next_date"`
	}
	w = e.do(http.MethodGet, "/v1/payouts/batches", nil, ops, nil)
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.NextDate == nil || *list.NextDate != next {
		t.Fatalf("list next_date = %v (%v), want %s", list.NextDate, err, next)
	}
}

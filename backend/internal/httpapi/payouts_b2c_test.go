package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/internal/identity"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/mpesa"
	"github.com/MboyaDan/mezai/backend/internal/payouts"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/seed"
)

const cbToken = "test-callback-token-0123456789abcdef"

// gatewayRefuses wraps the fake: payments to phones ending in 2 are refused
// by the gateway before acceptance (like a 429), every time.
type gatewayRefuses struct {
	mpesa.Client
	mu    sync.Mutex
	tries map[string]int
}

func (g *gatewayRefuses) PayB2C(ctx context.Context, p mpesa.Payment) (mpesa.Ack, error) {
	if strings.HasSuffix(p.Phone, "2") {
		g.mu.Lock()
		g.tries[p.OriginatorConversationID]++
		g.mu.Unlock()
		return mpesa.Ack{}, mpesa.ErrNotAccepted
	}
	return g.Client.PayB2C(ctx, p)
}

type b2cEnv struct {
	*env
	fake   *mpesa.Fake
	client *gatewayRefuses
	runner *jobs.Runner
}

func newB2CEnv(t *testing.T) *b2cEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	if _, err := seed.Run(context.Background(), pool, seed.Options{Password: pw}); err != nil {
		t.Fatal(err)
	}
	fake := mpesa.NewFake()
	fake.Manual = true
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := &gatewayRefuses{tries: map[string]int{}, Client: mpesa.NewDaraja(mpesa.Config{BaseURL: srv.URL, ConsumerKey: "k",
		ConsumerSecret: "s", ShortCode: "600000", InitiatorName: "i", SecurityCredential: "c",
		CallbackBase: "https://api.example.invalid/v1/mpesa/callbacks/" + cbToken}, nil)}
	files := storage.NewMemory()
	opts := auth.DefaultOptions(time.Hour, 30*time.Minute)
	opts.AppOrigin = origin
	svc, err := auth.NewService(pool, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, rules := httpapi.NewRouterWithRules(httpapi.Deps{Auth: &httpapi.AuthDeps{
		Service: svc, Pool: pool, AppOrigin: origin, CookieSecure: true,
		Files: files, Enquiry: httpapi.EnquiryDeps{NotifyTo: "ops@mezai.invalid"}, IdentityKey: identity.DevKey,
		Mpesa: &httpapi.MpesaDeps{Client: client, Environment: "fake", CallbackToken: cbToken,
			CallbackIPs: []netip.Prefix{netip.MustParsePrefix("196.201.214.0/24")}},
	}})
	e := &b2cEnv{env: &env{t: t, pool: pool, router: r, rules: rules, files: files}, fake: fake, client: client}
	// Callbacks go straight into the router from Safaricom's (pretend) address.
	fake.Deliver = func(u string, body []byte) error {
		e.callback(u, body, "196.201.214.200")
		return nil
	}
	jobSvc := payouts.NewService(pool, files, slog.New(slog.DiscardHandler)).WithB2C(&payouts.B2C{Client: client, Environment: "fake"})
	e.runner = &jobs.Runner{Pool: pool, Handlers: jobSvc.Handlers()}
	return e
}

func (e *b2cEnv) callback(rawURL string, body []byte, ip string) int {
	e.t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		e.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, u.Path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":40000"
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w.Code
}

// runJobs runs every due job; with makeDue, scheduled status queries first.
func (e *b2cEnv) runJobs(makeDue bool) {
	e.t.Helper()
	if makeDue {
		e.staff(func(tx pgx.Tx) error {
			_, err := tx.Exec(context.Background(), `UPDATE jobs SET run_at = now() WHERE kind = 'mpesa.status_query' AND status = 'queued'`)
			return err
		})
	}
	if _, err := e.runner.RunDue(context.Background()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *b2cEnv) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	e.staff(func(tx pgx.Tx) error { return tx.QueryRow(context.Background(), sql, args...).Scan(&n) })
	return n
}

// TestPayoutsB2C runs an approved batch through the fake Daraja with every
// outcome and proves the brief 13 rules: one request per line at most, a
// crash or timeout is settled by status queries (never a resend), callbacks
// are checked and stored before processing, duplicates change nothing, and
// paid lines = succeeded transactions = payout ledger entries.
func TestPayoutsB2C(t *testing.T) {
	e := newB2CEnv(t)
	ops := e.signIn("ops@sample.invalid")
	approver := e.signIn("approver@sample.invalid")
	opsID := e.userID("ops@sample.invalid")
	ctx := context.Background()

	// Owed KES 1,000 each, by phone ending: 1 paid, 2 gateway refuses, 3
	// worker crashed after marking, 6 refused, 7 500 then paid, 8 timeout
	// then paid, 9 failed.
	codes := []string{"001", "002", "003", "005", "006", "007", "008", "009"}
	ids := map[string]string{}
	for _, n := range codes {
		ids[n] = e.contributorID("contributor" + n + "@sample.invalid")
	}
	e.staff(func(tx pgx.Tx) error {
		for _, n := range codes {
			if _, err := tx.Exec(ctx, `INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, reason, created_by, created_at)
				VALUES ($1, 'adjustment', 100000, 'KES', 'Sample credit for tests', $2, now() - interval '3 days')`, ids[n], opsID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE contributor_profiles SET identity_status = 'verified', identity_decided_by = $2, identity_decided_at = now(),
			mpesa_name_checked_at = now(), mpesa_name_checked_by = $2 WHERE id = ANY($1::uuid[])`,
			[]string{ids["001"], ids["002"], ids["003"], ids["005"], ids["006"], ids["007"], ids["008"], ids["009"]}, opsID)
		return err
	})

	yesterday := time.Now().In(jobs.Nairobi).AddDate(0, 0, -1).Format(time.DateOnly)
	w := e.do(http.MethodPost, "/v1/payouts/batches", map[string]string{"period_end": yesterday}, ops, map[string]string{"Idempotency-Key": "b2c-one"})
	var b payoutBatchResp
	decode(t, w, &b)
	batch := b.ID
	for _, step := range []struct {
		s    *session
		path string
	}{{ops, "submit"}, {approver, "approve"}} {
		if w := e.do(http.MethodPost, "/v1/payouts/batches/"+batch+"/"+step.path, nil, step.s, nil); w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", step.path, w.Code, w.Body)
		}
	}
	var list struct {
		B2C payouts.B2CStatus `json:"b2c"`
	}
	e.get(ops, "/v1/payouts/batches", &list)
	if !list.B2C.Enabled || list.B2C.Environment != "fake" {
		t.Fatalf("b2c status: %+v", list.B2C)
	}

	// Callback checks happen before anything is stored.
	cb := "https://api.example.invalid/v1/mpesa/callbacks/"
	stray := []byte(`{"Result":{"ResultType":0,"ResultCode":0,"OriginatorConversationID":"mz-unknown","ConversationID":"AG_X"}}`)
	if code := e.callback(cb+"wrong-token-wrong-token-wrong-token/b2c/result", stray, "196.201.214.200"); code != http.StatusNotFound {
		t.Fatalf("wrong token: %d", code)
	}
	if code := e.callback(cb+cbToken+"/b2c/result", stray, "203.0.113.9"); code != http.StatusForbidden {
		t.Fatalf("address not on the list: %d", code)
	}
	if n := e.count(`SELECT count(*) FROM mpesa_callbacks`); n != 0 {
		t.Fatalf("refused callbacks were stored: %d", n)
	}
	if code := e.callback(cb+cbToken+"/b2c/result", stray, "196.201.214.200"); code != http.StatusOK {
		t.Fatalf("unmatched callback: %d", code)
	}
	if n := e.count(`SELECT count(*) FROM mpesa_callbacks WHERE verdict = 'unmatched'`); n != 1 {
		t.Fatalf("unmatched callback stored: %d", n)
	}

	// Disburse once; the batch can then be neither exported nor recorded by hand.
	if w := e.do(http.MethodPost, "/v1/payouts/batches/"+batch+"/disburse", nil, approver, nil); w.Code != http.StatusForbidden {
		t.Fatalf("disburse needs payout.create: %d", w.Code)
	}
	w = e.do(http.MethodPost, "/v1/payouts/batches/"+batch+"/disburse", nil, ops, nil)
	decode(t, w, &b)
	if w.Code != http.StatusOK || b.Status != "disbursing" {
		t.Fatalf("disburse: %d %+v", w.Code, b.Status)
	}
	line := func(n string) payoutLineResp { return b.line("C-0" + n) }
	for _, path := range []string{"disburse", "export"} {
		if w := e.do(http.MethodPost, "/v1/payouts/batches/"+batch+"/"+path, nil, ops, nil); w.Code != http.StatusConflict {
			t.Fatalf("%s after disburse: %d", path, w.Code)
		}
	}
	if w := e.do(http.MethodPost, "/v1/payouts/lines/"+line("001").ID+"/result", map[string]string{"status": "paid", "receipt": "ABC1234567"}, ops, nil); w.Code != http.StatusConflict {
		t.Fatalf("manual result on a B2C line: %d", w.Code)
	}

	// A worker marked 003 as sending two hours ago and died: never resent.
	ocid := func(n string) string { return "mz-" + strings.ReplaceAll(line(n).ID, "-", "") }
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE mpesa_transactions SET status = 'sending', sent_at = now() - interval '2 hours', send_attempts = 1
			WHERE originator_conversation_id = $1`, ocid("003"))
		return err
	})

	e.runJobs(false)
	txStatus := func(n string) string {
		var s string
		e.staff(func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT status FROM mpesa_transactions WHERE originator_conversation_id = $1`, ocid(n)).Scan(&s)
		})
		return s
	}
	for n, want := range map[string]string{"001": "accepted", "005": "accepted", "002": "queued", "003": "unknown", "006": "failed",
		"007": "unknown", "008": "accepted", "009": "accepted"} {
		if got := txStatus(n); got != want {
			t.Fatalf("after sending, %s: %s, want %s", n, got, want)
		}
	}

	// Results: 001 paid, 008 timed out (status query due now), 009 failed.
	e.fake.Flush()
	// The same success again (Safaricom retries) changes nothing.
	var dup []byte
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT body FROM mpesa_callbacks c JOIN mpesa_transactions t ON t.id = c.transaction_id
			WHERE t.originator_conversation_id = $1 AND c.kind = 'b2c_result'`, ocid("001")).Scan(&dup)
	})
	if code := e.callback(cb+cbToken+"/b2c/result", dup, "196.201.214.201"); code != http.StatusOK {
		t.Fatalf("duplicate: %d", code)
	}

	// Status queries settle 007 and 008 (paid) and start on 003 (not found).
	e.runJobs(true)
	e.fake.Flush()
	for i := 0; i < 3; i++ { // 003 needs three "not found" answers, an hour after sending
		e.runJobs(true)
		e.fake.Flush()
	}
	for n, want := range map[string]string{"001": "succeeded", "005": "succeeded", "002": "queued", "003": "failed", "006": "failed",
		"007": "succeeded", "008": "succeeded", "009": "failed"} {
		if got := txStatus(n); got != want {
			t.Fatalf("after results, %s: %s, want %s", n, got, want)
		}
	}

	// 002 was never accepted: Ops stops it. A sent one cannot be stopped.
	if w := e.do(http.MethodPost, "/v1/payouts/lines/"+line("001").ID+"/stop", map[string]string{"reason": "test"}, ops, nil); w.Code != http.StatusConflict {
		t.Fatalf("stop a paid line: %d", w.Code)
	}
	w = e.do(http.MethodPost, "/v1/payouts/lines/"+line("002").ID+"/stop", map[string]string{"reason": "Gateway kept refusing"}, ops, nil)
	decode(t, w, &b)
	if w.Code != http.StatusOK || b.Status != "settled" {
		t.Fatalf("stop: %d %s", w.Code, b.Status)
	}
	e.runJobs(false) // 002's retry, if due, must not send now
	for n, want := range map[string]string{"001": "paid", "005": "paid", "002": "failed", "003": "failed", "006": "failed",
		"007": "paid", "008": "paid", "009": "failed"} {
		if got := line(n).Status; got != want {
			t.Fatalf("line %s: %s, want %s (%v)", n, got, want, line(n).FailureReason)
		}
	}

	// Never twice: each payment reached Daraja at most once; 003 never.
	for _, n := range codes {
		if r := e.fake.Requests(ocid(n)); r > 1 || (n == "003" && r != 0) {
			t.Fatalf("%s sent %d times", n, r)
		}
	}
	if e.client.tries[ocid("002")] < 1 || e.fake.Requests(ocid("002")) != 0 {
		t.Fatal("002 should have been tried and refused by the gateway only")
	}

	// A late success for a failed line is flagged; no money is recorded.
	var conv string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT conversation_id FROM mpesa_transactions WHERE originator_conversation_id = $1`, ocid("009")).Scan(&conv)
	})
	late := []byte(`{"Result":{"ResultType":0,"ResultCode":0,"ResultDesc":"ok","OriginatorConversationID":"` + ocid("009") +
		`","ConversationID":"` + conv + `","TransactionID":"LATE123456","ResultParameters":{"ResultParameter":[{"Key":"TransactionAmount","Value":1000}]}}}`)
	if code := e.callback(cb+cbToken+"/b2c/result", late, "196.201.214.200"); code != http.StatusOK {
		t.Fatalf("late: %d", code)
	}

	// Ledger: one payout entry per paid line, none for failed ones.
	if n := e.count(`SELECT count(*) FROM earnings_ledger e JOIN payout_lines l ON l.id = e.payout_line_id
		WHERE l.batch_id = $1 AND e.entry_type = 'payout'`, batch); n != 4+len(paidSeedLines(b, codes)) {
		t.Fatalf("payout entries: %d", n)
	}
	// Stage 16: one "payout sent" email per paid line, including the one
	// whose duplicate callback arrived, and none for failed or late lines.
	if n := e.count(`SELECT count(*) FROM email_outbox o JOIN payout_lines l ON o.dedupe_key = 'payout_sent:' || l.id
		WHERE l.batch_id = $1`, batch); n != 4+len(paidSeedLines(b, codes)) {
		t.Fatalf("payout emails: %d", n)
	}
	for _, n := range []string{"002", "003", "006", "009"} {
		if bal := e.count(`SELECT coalesce(sum(amount_cents), 0) FROM earnings_ledger WHERE contributor_id = $1`, ids[n]); bal < 100000 {
			t.Fatalf("%s failed but balance is %d", n, bal)
		}
	}

	var rec payouts.Reconciliation
	e.get(approver, "/v1/payouts/reconciliation", &rec)
	if len(rec.Batches) != 1 || !rec.Batches[0].Balanced || rec.Batches[0].PaidCents != rec.Batches[0].LedgerCents ||
		rec.Batches[0].PendingLines != 0 {
		t.Fatalf("reconciliation: %+v", rec.Batches)
	}
	kinds := map[string]int{}
	for _, x := range rec.Exceptions {
		kinds[x.Kind]++
	}
	if kinds["late_success"] != 1 || kinds["callback"] != 1 || kinds["name_mismatch"] != 1 {
		t.Fatalf("exceptions: %+v", rec.Exceptions)
	}

	// Registered names (D-139): 005's number is registered to somebody else.
	// The money went to that number, so the line stays paid, but the name
	// check is withdrawn and the next batch holds 005. Others matched.
	nameCheck := func(n string) string {
		var v *string
		e.staff(func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT name_check FROM mpesa_transactions WHERE originator_conversation_id = $1`, ocid(n)).Scan(&v)
		})
		if v == nil {
			return ""
		}
		return *v
	}
	for n, want := range map[string]string{"001": "match", "005": "mismatch", "007": "match", "008": "match", "006": "", "009": ""} {
		if got := nameCheck(n); got != want {
			t.Fatalf("name check %s: %q, want %q", n, got, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND mpesa_name_mismatch_at IS NOT NULL AND mpesa_name_checked_at IS NULL`, ids["005"]); n != 1 {
		t.Fatal("005's name check must be withdrawn")
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND mpesa_name_mismatch_at IS NULL AND mpesa_name_checked_at IS NOT NULL`, ids["001"]); n != 1 {
		t.Fatal("001's name check must stand")
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'contributor.mpesa_name_mismatch' AND entity_id = $1`, ids["005"]); n != 1 {
		t.Fatal("mismatch not audited")
	}
	// Neither the registered names nor the phone numbers beside them are stored.
	if n := e.count(`SELECT count(*) FROM mpesa_callbacks WHERE body::text ~ '(SAMPLE|2547[0-9]{8}|callbacks/)'`); n != 0 {
		t.Fatalf("%d stored callbacks still hold a name, a phone number or our callback URL", n)
	}
	// With the check withdrawn the next batch holds 005 as name_unchecked
	// (payouts.Decide, TestDecide) until Ops checks the name again.
	if w := e.do(http.MethodPost, "/v1/contributors/"+ids["005"]+"/mpesa-name-check", nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("Ops checks the name again: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND mpesa_name_mismatch_at IS NULL AND mpesa_name_checked_at IS NOT NULL`, ids["005"]); n != 1 {
		t.Fatal("a fresh check must clear the mismatch")
	}
	e.get(approver, "/v1/payouts/reconciliation", &rec)
	for _, x := range rec.Exceptions {
		if x.Kind == "name_mismatch" {
			t.Fatal("a cleared mismatch is still an exception")
		}
	}

	var txs struct {
		Transactions []payouts.Transaction `json:"transactions"`
	}
	e.get(ops, "/v1/payouts/batches/"+batch+"/transactions", &txs)
	if len(txs.Transactions) != len(b.Lines) {
		t.Fatalf("transactions: %d lines %d", len(txs.Transactions), len(b.Lines))
	}
	for _, x := range txs.Transactions {
		if strings.Contains(x.OriginatorID, "2547") {
			t.Fatal("phone in transaction list")
		}
	}
	// Contributors and clients see none of this.
	c1 := e.signIn("contributor001@sample.invalid")
	for _, p := range []string{"/v1/payouts/reconciliation", "/v1/payouts/batches/" + batch + "/transactions"} {
		if w := e.do(http.MethodGet, p, nil, c1, nil); w.Code != http.StatusForbidden {
			t.Fatalf("contributor %s: %d", p, w.Code)
		}
	}
}

// paidSeedLines are lines of sample contributors outside codes (seed
// balances, D-131), paid by the fake when their phone does not end in 2-9.
func paidSeedLines(b payoutBatchResp, codes []string) []string {
	mine := map[string]bool{}
	for _, c := range codes {
		mine["C-0"+c] = true
	}
	var out []string
	for _, l := range b.Lines {
		if !mine[l.PublicCode] && l.Status == "paid" {
			out = append(out, l.PublicCode)
		}
	}
	return out
}

// TestPayoutsB2COff: without MPESA_ENV the batch can only be exported, and
// the callback routes answer 404.
func TestPayoutsB2COff(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")
	var list struct {
		B2C payouts.B2CStatus `json:"b2c"`
	}
	e.get(ops, "/v1/payouts/batches", &list)
	if list.B2C.Enabled || list.B2C.Environment != "off" {
		t.Fatalf("b2c: %+v", list.B2C)
	}
	if w := e.do(http.MethodPost, "/v1/payouts/batches/00000000-0000-7000-8000-000000000000/disburse", nil, ops, nil); w.Code != http.StatusConflict {
		t.Fatalf("disburse with B2C off: %d", w.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/mpesa/callbacks/"+cbToken+"/b2c/result", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("callback with B2C off: %d", w.Code)
	}
}

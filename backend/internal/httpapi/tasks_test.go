package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type taskResp struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	ExpiresAt string         `json:"expires_at"`
	Content   map[string]any `json:"content"`
	Draft     map[string]any `json:"draft"`
}

type resultResp struct {
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"`
	Warnings     []struct {
		Code string `json:"code"`
	} `json:"warnings"`
}

const (
	routing  = "Customer message routing (Sample)"
	pairwise = "Helpful answers comparison (Sample)"
)

func (e *env) next(s *session, project string) (*httptest.ResponseRecorder, taskResp) {
	e.t.Helper()
	w := e.do(http.MethodPost, "/v1/me/projects/"+project+"/next", map[string]any{}, s, nil)
	var t taskResp
	if w.Code == http.StatusOK {
		decode(e.t, w, &t)
	}
	return w, t
}

func (e *env) submitTask(s *session, task, key string, answer any) *httptest.ResponseRecorder {
	e.t.Helper()
	h := map[string]string{}
	if key != "" {
		h["Idempotency-Key"] = key
	}
	return e.do(http.MethodPost, "/v1/me/tasks/"+task+"/submit", map[string]any{"answer": answer, "signals": map[string]any{"time_on_task_ms": 60000}}, s, h)
}

var routingAnswer = map[string]any{"labels": []string{"Billing"}}

// TestTaskFlow covers eligibility, one task at a time, task privacy, drafts
// and AC-07: the same idempotency key creates one submission.
func TestTaskFlow(t *testing.T) {
	e := newEnv(t)
	project := e.projectID(routing)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)

	// Eligibility: skills held (Swahili at fluent or better) and active + verified.
	w := e.do(http.MethodGet, "/v1/me/projects", nil, c1, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), routing) || strings.Contains(w.Body.String(), "English to Swahili") {
		t.Fatalf("c1 projects: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "overlap") || strings.Contains(w.Body.String(), "gold") {
		t.Fatalf("project view leaks quality settings: %s", w.Body)
	}
	_, c3 := e.login("contributor003@sample.invalid", pw) // Swahili at "working" only
	if w := e.do(http.MethodGet, "/v1/me/projects", nil, c3, nil); strings.Contains(w.Body.String(), routing) {
		t.Fatalf("c3 sees a project needing fluent Swahili: %s", w.Body)
	}
	if w, _ := e.next(c3, project); errorCode(w) != "not_eligible" {
		t.Fatalf("c3 next: %d %s", w.Code, w.Body)
	}
	_, c9 := e.login("contributor009@sample.invalid", pw) // qualifying, identity pending
	if w := e.do(http.MethodGet, "/v1/me/projects", nil, c9, nil); errorCode(w) != "not_eligible" {
		t.Fatalf("qualifying contributor: %d %s", w.Code, w.Body)
	}

	// One task at a time; the response names nothing but the task.
	w, task := e.next(c1, project)
	if w.Code != http.StatusOK || task.Status != "reserved" || task.Content["text"] == nil {
		t.Fatalf("next: %d %s", w.Code, w.Body)
	}
	for _, leak := range []string{"item_id", "external_id", "gold", "expected", "overlap"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Fatalf("task response contains %q: %s", leak, w.Body)
		}
	}
	if _, again := e.next(c1, project); again.ID != task.ID {
		t.Fatal("a second next must return the current task")
	}
	if w := e.do(http.MethodGet, "/v1/me/tasks/"+task.ID, nil, c2, nil); w.Code != http.StatusNotFound {
		t.Fatalf("another contributor reads the task: %d", w.Code)
	}

	// Drafts are saved and returned.
	if w := e.do(http.MethodPut, "/v1/me/tasks/"+task.ID+"/draft", map[string]any{"answer": routingAnswer, "time_on_task_ms": 4000}, c1, nil); w.Code != http.StatusNoContent {
		t.Fatalf("draft: %d %s", w.Code, w.Body)
	}
	var got taskResp
	decode(t, e.do(http.MethodGet, "/v1/me/tasks/"+task.ID, nil, c1, nil), &got)
	if got.Draft == nil {
		t.Fatal("draft not returned")
	}

	// Submit needs a key and a valid answer.
	if w := e.submitTask(c1, task.ID, "", routingAnswer); !strings.Contains(w.Body.String(), `"field":"idempotency_key"`) {
		t.Fatalf("no key: %d %s", w.Code, w.Body)
	}
	if w := e.submitTask(c1, task.ID, "submit-key-0001", map[string]any{"labels": []string{"Nonsense"}}); !strings.Contains(w.Body.String(), `"field":"answer"`) {
		t.Fatalf("bad answer: %d %s", w.Code, w.Body)
	}

	// AC-07: a retried submit with the same key creates one submission.
	var first, second resultResp
	w = e.submitTask(c1, task.ID, "submit-key-0001", routingAnswer)
	decode(t, w, &first)
	if w.Code != http.StatusOK || first.Status != "submitted" || first.SubmissionID == "" {
		t.Fatalf("submit: %d %s", w.Code, w.Body)
	}
	w = e.submitTask(c1, task.ID, "submit-key-0001", routingAnswer)
	decode(t, w, &second)
	if w.Code != http.StatusOK || second.SubmissionID != first.SubmissionID {
		t.Fatalf("replay: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM submissions WHERE assignment_id = $1 AND status = 'submitted'`, task.ID); n != 1 {
		t.Fatalf("submissions for one task: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'task.submitted' AND entity_id = $1`, first.SubmissionID); n != 1 {
		t.Fatalf("audited submits: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM submission_revisions r JOIN submissions s ON s.id = r.submission_id
		WHERE s.assignment_id = $1 AND NOT r.is_draft`, task.ID); n != 1 {
		t.Fatalf("final revisions: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE after::text LIKE '%Billing%'`); n != 0 {
		t.Fatal("answer written to the audit log")
	}
	if w := e.submitTask(c1, task.ID, "submit-key-0002", routingAnswer); errorCode(w) != "already_submitted" {
		t.Fatalf("second key on a submitted task: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodGet, "/v1/me/tasks/"+task.ID, nil, c1, nil), &got)
	if got.Status != "submitted" || got.Content != nil {
		t.Fatalf("content after submit: %+v", got)
	}
	_, nextTask := e.next(c1, project)
	if nextTask.ID == "" || nextTask.ID == task.ID {
		t.Fatal("no new task after submitting")
	}
	if w := e.submitTask(c1, nextTask.ID, "submit-key-0001", routingAnswer); errorCode(w) != "idempotency_key_reused" {
		t.Fatalf("key reused on another task: %d %s", w.Code, w.Body)
	}
}

// TestEachItemGetsKContributors is AC-06 end to end on the sample project:
// every eligible contributor works until nothing is left; each regular item
// ends with exactly k distinct contributors and none of them twice.
func TestEachItemGetsKContributors(t *testing.T) {
	e := newEnv(t)
	project := e.projectID(routing)
	n := 0
	for _, who := range []string{"001", "002", "004", "005", "007", "008"} {
		_, s := e.login("contributor"+who+"@sample.invalid", pw)
		for i := 0; ; i++ {
			w, task := e.next(s, project)
			if errorCode(w) == "no_tasks" {
				break
			}
			if w.Code != http.StatusOK || i > 50 {
				t.Fatalf("%s next: %d %s", who, w.Code, w.Body)
			}
			if w := e.submitTask(s, task.ID, "k-"+who+"-"+task.ID, routingAnswer); w.Code != http.StatusOK {
				t.Fatalf("%s submit: %d %s", who, w.Code, w.Body)
			}
			n++
		}
	}
	regular := e.count(`SELECT count(*) FROM items i WHERE i.project_id = $1 AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)`, project)
	if regular == 0 || n < regular*3 {
		t.Fatalf("%d submissions for %d regular items", n, regular)
	}
	if bad := e.count(`SELECT count(*) FROM items i WHERE i.project_id = $1
		AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
		AND (i.status <> 'awaiting_consensus' OR i.submission_count <> 3
		     OR (SELECT count(DISTINCT s.contributor_id) FROM submissions s WHERE s.item_id = i.id AND s.status = 'submitted') <> 3)`, project); bad != 0 {
		t.Fatalf("%d regular items without exactly k = 3 distinct contributors", bad)
	}
	if bad := e.count(`SELECT count(*) FROM items i JOIN gold_items g ON g.item_id = i.id WHERE i.project_id = $1 AND i.status <> 'pending'`, project); bad != 0 {
		t.Fatalf("gold items moved into the consensus pipeline: %d", bad)
	}
}

var pairwiseAnswer = func(pref, rationale string) map[string]any {
	crit := map[string]int{"accuracy": 4, "helpfulness": 4, "safety": 5, "language_quality": 4}
	return map[string]any{"preference": pref, "scores_a": crit, "scores_b": crit, "rationale": rationale}
}

// TestQASkipReportExpiry covers the QA checks (D-077), skip with a reason,
// content reports and their Ops resolution, and reservation expiry.
func TestQASkipReportExpiry(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	project := e.projectID(pairwise)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE projects SET min_rationale_chars = 20, gold_rate = 0 WHERE id = $1`, project)
		return err
	})
	_, c1 := e.login("contributor001@sample.invalid", pw)

	// QA: a short rationale blocks; a mismatched one is accepted with a warning.
	_, task := e.next(c1, project)
	w := e.submitTask(c1, task.ID, "qa-key-0001", pairwiseAnswer("a", "A is good."))
	if errorCode(w) != "qa_blocked" || !strings.Contains(w.Body.String(), "rationale_too_short") {
		t.Fatalf("short rationale: %d %s", w.Code, w.Body)
	}
	var res resultResp
	w = e.submitTask(c1, task.ID, "qa-key-0002", pairwiseAnswer("a", "Honestly response B is much better than A here."))
	decode(t, w, &res)
	if w.Code != http.StatusOK || len(res.Warnings) != 1 || res.Warnings[0].Code != "rationale_mismatch" {
		t.Fatalf("mismatch warning: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM submissions WHERE id = $1 AND qa_warnings::text LIKE '%rationale_mismatch%'`, res.SubmissionID); n != 1 {
		t.Fatal("warning not stored with the submission")
	}

	// Skip: needs a known reason (and a note for "other"); the item never comes back.
	_, task = e.next(c1, project)
	if w := e.do(http.MethodPost, "/v1/me/tasks/"+task.ID+"/skip", map[string]string{"reason": "other"}, c1, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("other without note: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/me/tasks/"+task.ID+"/skip", map[string]string{"reason": "language"}, c1, nil); w.Code != http.StatusNoContent {
		t.Fatalf("skip: %d %s", w.Code, w.Body)
	}
	if _, again := e.next(c1, project); again.ID == task.ID {
		t.Fatal("skipped task returned")
	}

	// Report: the item leaves the queue for everyone while the report is open.
	_, c2 := e.login("contributor002@sample.invalid", pw)
	_, task = e.next(c2, project)
	if w := e.do(http.MethodPost, "/v1/me/tasks/"+task.ID+"/report", map[string]string{"category": "personal_data", "note": "Contains a phone number."}, c2, nil); w.Code != http.StatusNoContent {
		t.Fatalf("report: %d %s", w.Code, w.Body)
	}
	var item string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT item_id::text FROM assignments WHERE id = $1`, task.ID).Scan(&item)
	})
	_, c4 := e.login("contributor004@sample.invalid", pw)
	for i := 0; i < 20; i++ {
		w, t4 := e.next(c4, project)
		if errorCode(w) == "no_tasks" {
			break
		}
		e.submitTask(c4, t4.ID, "c4-key-"+t4.ID, pairwiseAnswer("b", "B covers the question fully, A does not."))
	}
	if n := e.count(`SELECT count(*) FROM assignments WHERE item_id = $1
		AND created_at > (SELECT created_at FROM content_reports WHERE item_id = $1)`, item); n != 0 {
		t.Fatalf("reported item given to others: %d", n)
	}

	// Ops resolve: contributors cannot; a reason is required; withdrawing ends the item.
	if w := e.do(http.MethodGet, "/v1/content-reports", nil, c2, nil); w.Code != http.StatusForbidden {
		t.Fatalf("contributor lists reports: %d", w.Code)
	}
	_, ops := e.login("ops@sample.invalid", pw)
	var reports struct {
		Reports []struct {
			ID, Category string
			ReportedBy   string `json:"reported_by"`
		}
	}
	decode(t, e.do(http.MethodGet, "/v1/content-reports", nil, ops, nil), &reports)
	if len(reports.Reports) != 1 || reports.Reports[0].Category != "personal_data" || !strings.HasPrefix(reports.Reports[0].ReportedBy, "C-") {
		t.Fatalf("reports: %+v", reports)
	}
	rid := reports.Reports[0].ID
	if w := e.do(http.MethodPost, "/v1/content-reports/"+rid+"/resolve", map[string]string{"action": "withdraw"}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("resolve without reason: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/content-reports/"+rid+"/resolve", map[string]string{"action": "withdraw", "reason": "Personal data in the item."}, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("withdraw: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = $1 AND status = 'rejected'`, item); n != 1 {
		t.Fatal("withdrawn item still in the queue")
	}
	if w := e.do(http.MethodPost, "/v1/content-reports/"+rid+"/resolve", map[string]string{"action": "dismiss", "reason": "x"}, ops, nil); errorCode(w) != "already_resolved" {
		t.Fatalf("second resolve: %d", w.Code)
	}

	// Expiry: an expired reservation shows no content and takes no answers.
	_, c5 := e.login("contributor005@sample.invalid", pw)
	_, task = e.next(c5, project)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE assignments SET reserved_at = now() - interval '2 hours', expires_at = now() - interval '1 hour' WHERE id = $1`, task.ID)
		return err
	})
	var got taskResp
	decode(t, e.do(http.MethodGet, "/v1/me/tasks/"+task.ID, nil, c5, nil), &got)
	if got.Status != "expired" || got.Content != nil {
		t.Fatalf("expired task: %+v", got)
	}
	if w := e.submitTask(c5, task.ID, "late-key-0001", pairwiseAnswer("a", "A answers the question and B does not.")); errorCode(w) != "reservation_closed" {
		t.Fatalf("submit after expiry: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPut, "/v1/me/tasks/"+task.ID+"/draft", map[string]any{"answer": map[string]any{}}, c5, nil); w.Code != http.StatusGone {
		t.Fatalf("draft after expiry: %d", w.Code)
	}
	// The next request marks it expired and frees its slot on the item (the
	// small sample project may simply have no work left for c5).
	if w, fresh := e.next(c5, project); fresh.ID == task.ID || (w.Code != http.StatusOK && errorCode(w) != "no_tasks") {
		t.Fatalf("next after expiry: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM assignments WHERE id = $1 AND status = 'expired'`, task.ID); n != 1 {
		t.Fatal("stale reservation not marked expired")
	}
}

// TestTaskRateLimit: one account asking for work far faster than a person
// can is slowed down (D-105).
func TestTaskRateLimit(t *testing.T) {
	e := newEnv(t)
	project := e.projectID(routing)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	limited := 0
	for range 61 {
		if w, _ := e.next(c1, project); w.Code == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited != 1 {
		t.Fatalf("61 requests in a minute: %d limited, want 1", limited)
	}
	_, c2 := e.login("contributor002@sample.invalid", pw)
	if w, _ := e.next(c2, project); w.Code != http.StatusOK {
		t.Fatalf("another account is not limited by the first: %d", w.Code)
	}
}

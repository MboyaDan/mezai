package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
)

type reviewQueueResp struct {
	Items []struct {
		ItemID    string `json:"item_id"`
		ProjectID string `json:"project_id"`
		Reason    string `json:"reason"`
	} `json:"items"`
	Projects []struct {
		ID      string `json:"id"`
		Waiting int    `json:"waiting"`
	} `json:"projects"`
}

type reviewItemResp struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CanAdjudicate bool   `json:"can_adjudicate"`
	Consensus     *struct {
		Complete bool   `json:"complete"`
		Reason   string `json:"reason"`
	} `json:"consensus"`
	Submissions []struct {
		ID          string         `json:"id"`
		Contributor string         `json:"contributor"`
		Status      string         `json:"status"`
		Answer      map[string]any `json:"answer"`
	} `json:"submissions"`
	Decision *struct {
		ID       string `json:"id"`
		Decision string `json:"decision"`
	} `json:"decision"`
}

type decisionResp struct {
	ReviewID   string `json:"review_id"`
	Decision   string `json:"decision"`
	ItemStatus string `json:"item_status"`
}

// reviewProject creates a live yes/no classification project with k = 2, no
// gold or audit sampling, paying 25.00 KES a task, with n items.
func (e *env) reviewProject(n int) string {
	e.t.Helper()
	var id string
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := tx.QueryRow(ctx, `
			INSERT INTO projects (org_id, template_id, task_type, name, status, instructions_md, config, overlap_k, gold_rate,
			  audit_sample_rate, qa_checks, published_at, pay_per_task_cents)
			SELECT o.id, t.id, 'classification', 'Review check (Sample)', 'live', 'Sample.',
			  '{"labels":["Yes","No"],"multi_label":false}', 2, 0, 0, '{"warn_fast_seconds":0}', now(), 2500
			FROM organisations o, task_templates t
			WHERE o.slug = 'sample-client-ltd' AND t.task_type = 'classification' ORDER BY t.version DESC LIMIT 1
			RETURNING id`).Scan(&id); err != nil {
			return err
		}
		for i := range n {
			if _, err := tx.Exec(ctx, `INSERT INTO items (org_id, project_id, external_id, content)
				SELECT org_id, id, $2, '{"text":"Sample sentence."}' FROM projects WHERE id = $1`, id, fmt.Sprintf("rv%d", i)); err != nil {
				return err
			}
		}
		return nil
	})
	return id
}

// workAll has the contributor answer every task they can get on the project.
func (e *env) workAll(s *session, project string, answer any) {
	e.t.Helper()
	for i := 0; ; i++ {
		w, task := e.next(s, project)
		if errorCode(w) == "no_tasks" {
			return
		}
		if w.Code != http.StatusOK || i > 20 {
			e.t.Fatalf("next: %d %s", w.Code, w.Body)
		}
		if w := e.submitTask(s, task.ID, "rv-"+task.ID, answer); w.Code != http.StatusOK {
			e.t.Fatalf("submit: %d %s", w.Code, w.Body)
		}
	}
}

func (e *env) decide(s *session, item, key string, body map[string]any) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/v1/review/items/"+item+"/decision", body, s, map[string]string{"Idempotency-Key": key})
}

// TestReviewAndAdjudication is AC-09's review half and AC-07's earning half:
// disagreeing items reach the review queue; only assigned reviewers (or
// people who adjudicate) see them; a decision makes the item final and
// immutable, pays every accepted answer once and shows a rejected
// contributor the reason; a retry with the same key changes nothing.
func TestReviewAndAdjudication(t *testing.T) {
	e := newEnv(t)
	project := e.reviewProject(2)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"No"}})

	if n := e.count(`SELECT count(*) FROM items WHERE project_id = $1 AND status = 'in_review'`, project); n != 2 {
		t.Fatalf("%d items in review, want 2 (no majority)", n)
	}
	if n := e.count(`SELECT count(*) FROM earnings_ledger WHERE project_id = $1`, project); n != 0 {
		t.Fatalf("%d earnings before review", n)
	}

	_, ops := e.login("ops@sample.invalid", pw)
	_, rev := e.login("reviewer@sample.invalid", pw)
	queue := func(s *session) reviewQueueResp {
		var q reviewQueueResp
		w := e.do(http.MethodGet, "/v1/review/items?project_id="+project, nil, s, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("queue: %d %s", w.Code, w.Body)
		}
		decode(t, w, &q)
		return q
	}
	if q := queue(rev); len(q.Items) != 0 {
		t.Fatalf("unassigned reviewer sees %d items", len(q.Items))
	}
	q := queue(ops)
	if len(q.Items) != 2 || q.Items[0].Reason != "no_majority" {
		t.Fatalf("ops queue: %+v", q)
	}
	item, other := q.Items[0].ItemID, q.Items[1].ItemID
	if w := e.do(http.MethodGet, "/v1/review/items/"+item, nil, rev, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unassigned reviewer opened an item: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/review/items?reason=nope", nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad reason filter: %d", w.Code)
	}

	// Ops assigns the reviewer.
	var reviewers struct {
		Candidates []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"candidates"`
	}
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/reviewers", nil, ops, nil), &reviewers)
	revID := e.userID("reviewer@sample.invalid")
	found := false
	for _, c := range reviewers.Candidates {
		found = found || c.ID == revID
	}
	if !found {
		t.Fatalf("reviewer not a candidate: %+v", reviewers)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/reviewers", map[string]string{"user_id": e.userID("contributor001@sample.invalid")}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("assigned a contributor as reviewer: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/reviewers", map[string]string{"user_id": revID}, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("assign: %d %s", w.Code, w.Body)
	}
	if q := queue(rev); len(q.Items) != 2 {
		t.Fatalf("assigned reviewer sees %d items", len(q.Items))
	}

	var detail reviewItemResp
	decode(t, e.do(http.MethodGet, "/v1/review/items/"+item, nil, rev, nil), &detail)
	if len(detail.Submissions) != 2 || detail.Consensus == nil || detail.Consensus.Complete || detail.CanAdjudicate {
		t.Fatalf("detail: %+v", detail)
	}
	for _, s := range detail.Submissions {
		if !strings.HasPrefix(s.Contributor, "C-") {
			t.Fatalf("contributor shown as %q, want a code", s.Contributor)
		}
	}
	yes, no := detail.Submissions[0].ID, detail.Submissions[1].ID
	if l := detail.Submissions[0].Answer["labels"].([]any); l[0] != "Yes" {
		yes, no = no, yes
	}

	// Decisions the reviewer may not take.
	if w := e.decide(rev, item, "rv-key-0001", map[string]any{"decision": "accept_consensus"}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("accepted a missing consensus: %d %s", w.Code, w.Body)
	}
	if w := e.decide(rev, item, "rv-key-0001", map[string]any{"decision": "write_final", "final_answer": map[string]any{"labels": []string{"Yes"}}}); errorCode(w) != "forbidden" {
		t.Fatalf("reviewer wrote a final answer: %d %s", w.Code, w.Body)
	}
	if w := e.decide(rev, item, "short", map[string]any{"decision": "choose_submission", "submission_id": yes}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short key: %d", w.Code)
	}
	if w := e.decide(rev, item, "rv-key-0001", map[string]any{"decision": "choose_submission", "submission_id": yes,
		"rejections": []map[string]string{{"submission_id": no, "text": "short"}}}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short rejection reason: %d", w.Code)
	}

	reason := "The text is a clear yes; see instruction 2."
	body := map[string]any{"decision": "choose_submission", "submission_id": yes,
		"rejections": []map[string]string{{"submission_id": no, "text": reason}},
		"feedback":   []map[string]string{{"submission_id": yes, "text": "Good call on a tricky one."}}}
	var first, again decisionResp
	w := e.decide(rev, item, "rv-key-0001", body)
	if w.Code != http.StatusOK {
		t.Fatalf("decide: %d %s", w.Code, w.Body)
	}
	decode(t, w, &first)
	if first.ItemStatus != "final" {
		t.Fatalf("decision: %+v", first)
	}
	decode(t, e.decide(rev, item, "rv-key-0001", body), &again)
	if again.ReviewID != first.ReviewID {
		t.Fatalf("replay gave review %s, want %s", again.ReviewID, first.ReviewID)
	}
	if w := e.decide(ops, item, "rv-key-0002", map[string]any{"decision": "choose_submission", "submission_id": no}); errorCode(w) != "already_decided" {
		t.Fatalf("second decision: %d %s", w.Code, w.Body)
	}

	checks := []struct {
		name string
		sql  string
		args []any
		want int
	}{
		{"one review", `SELECT count(*) FROM reviews WHERE item_id = $1`, []any{item}, 1},
		{"final with a review revision", `SELECT count(*) FROM items i JOIN item_revisions r ON r.item_id = i.id AND r.revision = i.current_revision
			WHERE i.id = $1 AND i.status = 'final' AND r.source = 'review' AND r.answer = '{"labels":["Yes"]}'`, []any{item}, 1},
		{"chosen accepted and agreeing", `SELECT count(*) FROM submissions WHERE id = $1 AND status = 'accepted' AND decision_source = 'review' AND agrees_with_final`, []any{yes}, 1},
		{"other rejected with reason", `SELECT count(*) FROM submissions WHERE id = $1 AND status = 'rejected' AND rejection_reason = $2 AND NOT agrees_with_final`, []any{no, reason}, 1},
		{"one earning, for the accepted answer", `SELECT count(*) FROM earnings_ledger WHERE submission_id IN ($1, $2) AND amount_cents = 2500 AND currency = 'KES'`, []any{yes, no}, 1},
		{"no earning for the rejected answer", `SELECT count(*) FROM earnings_ledger WHERE submission_id = $1`, []any{no}, 0},
		{"rejected contributor sees the reason", `SELECT count(*) FROM feedback WHERE submission_id = $1 AND body LIKE '%' || $2`, []any{no, reason}, 1},
		{"feedback to the other contributor", `SELECT count(*) FROM feedback WHERE submission_id = $1 AND body = 'Good call on a tricky one.'`, []any{yes}, 1},
		{"trust counts the review", `SELECT count(*) FROM trust_scores t JOIN submissions s ON s.contributor_id = t.contributor_id
			WHERE s.id = $1 AND t.review_acceptance = 0`, []any{no}, 1},
		{"audited without answers", `SELECT count(*) FROM audit_log WHERE action = 'review.decided' AND entity_id = $1 AND after::text NOT LIKE '%Yes%'`, []any{item}, 1},
	}
	for _, c := range checks {
		if n := e.count(c.sql, c.args...); n != c.want {
			t.Errorf("%s: %d, want %d", c.name, n, c.want)
		}
	}

	// The rejected contributor reads the reason in their feedback.
	var fb struct {
		Feedback []struct {
			Body string `json:"body"`
		} `json:"feedback"`
	}
	decode(t, e.do(http.MethodGet, "/v1/me/feedback", nil, c2, nil), &fb)
	if len(fb.Feedback) == 0 || !strings.Contains(fb.Feedback[0].Body, reason) {
		t.Fatalf("contributor feedback: %+v", fb)
	}

	// Final is immutable: the app role cannot change or delete a revision.
	err := e.pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE item_revisions SET answer = '{}' WHERE item_id = $1`, item)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("revision update: %v", err)
	}

	// Two people deciding the other item at once: exactly one wins.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	pick := firstSubmission(e, other)
	for i, s := range []*session{rev, ops} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.decide(s, other, fmt.Sprintf("rv-race-%04d", i), map[string]any{"decision": "choose_submission", "submission_id": pick}).Code
		}()
	}
	wg.Wait()
	oneWon := (codes[0] == http.StatusOK) != (codes[1] == http.StatusOK)
	if !oneWon || e.count(`SELECT count(*) FROM reviews WHERE item_id = $1`, other) != 1 {
		t.Fatalf("racing decisions: %v", codes)
	}
	if n := e.count(`SELECT count(*) FROM earnings_ledger WHERE project_id = $1`, project); n != 3 {
		t.Fatalf("%d earnings, want 3 (one rejected answer)", n)
	}

	// The queue is empty; a removed reviewer loses access.
	if q := queue(ops); len(q.Items) != 0 {
		t.Fatalf("queue not empty: %+v", q)
	}
	if w := e.do(http.MethodDelete, "/v1/projects/"+project+"/reviewers/"+revID, nil, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("remove reviewer: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/review/items/"+item, nil, rev, nil); w.Code != http.StatusNotFound {
		t.Fatalf("removed reviewer still sees the item: %d", w.Code)
	}
}

// TestAdjudicateWriteFinalAndRejectItem: write_final is validated like a
// submission; reject_item needs a reason and still pays honest work (D-110).
func TestAdjudicateWriteFinalAndRejectItem(t *testing.T) {
	e := newEnv(t)
	project := e.reviewProject(2)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"No"}})
	_, ops := e.login("ops@sample.invalid", pw)
	var q reviewQueueResp
	decode(t, e.do(http.MethodGet, "/v1/review/items?project_id="+project, nil, ops, nil), &q)
	if len(q.Items) != 2 {
		t.Fatalf("queue: %+v", q)
	}
	a, b := q.Items[0].ItemID, q.Items[1].ItemID

	if w := e.decide(ops, a, "wf-key-0001", map[string]any{"decision": "write_final", "final_answer": map[string]any{"labels": []string{"Maybe"}}}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid final answer: %d %s", w.Code, w.Body)
	}
	if w := e.decide(ops, a, "wf-key-0001", map[string]any{"decision": "write_final", "final_answer": map[string]any{"labels": []string{"No"}}}); w.Code != http.StatusOK {
		t.Fatalf("write final: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM submissions WHERE item_id = $1 AND status = 'accepted'`, a); n != 2 {
		t.Fatalf("%d accepted after write_final, want 2", n)
	}
	if n := e.count(`SELECT count(*) FROM submissions WHERE item_id = $1 AND agrees_with_final`, a); n != 1 {
		t.Fatalf("%d agreeing with the written answer, want 1", n)
	}

	if w := e.decide(ops, b, "ri-key-0001", map[string]any{"decision": "reject_item"}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reject without reason: %d", w.Code)
	}
	if w := e.decide(ops, b, "ri-key-0001", map[string]any{"decision": "reject_item", "reason": "The source text is not a customer message."}); w.Code != http.StatusOK {
		t.Fatalf("reject item: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = $1 AND status = 'rejected'`, b); n != 1 {
		t.Fatal("item not rejected")
	}
	if n := e.count(`SELECT count(*) FROM earnings_ledger l JOIN submissions s ON s.id = l.submission_id WHERE s.item_id = $1`, b); n != 2 {
		t.Fatalf("%d earnings on a rejected item, want 2 (honest work is paid)", n)
	}
	if n := e.count(`SELECT count(*) FROM item_revisions WHERE item_id = $1`, b); n != 0 {
		t.Fatal("a rejected item got a final answer")
	}
}

// firstSubmission is the ID of the first answer on an item.
func firstSubmission(e *env, item string) string {
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM submissions WHERE item_id = $1 ORDER BY submitted_at, id LIMIT 1`, item).Scan(&id)
	})
	return id
}

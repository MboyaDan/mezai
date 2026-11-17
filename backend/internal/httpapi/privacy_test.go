package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/privacy"
)

type retentionResp struct {
	Days     int     `json:"retention_days"`
	ClosedAt *string `json:"closed_at"`
	DueAt    *string `json:"due_at"`
	PurgedAt *string `json:"purged_at"`
}

// Stage 17b, brief 12: a closed project's task content goes when its
// retention period ends (sweep) or at once on request (Ops). Counts, money
// and quality figures stay; content, answers and files go.
func TestRetentionPurge(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ops := e.signIn("ops@sample.invalid")
	client := e.signIn("client.admin@sample.invalid")
	c1, c2 := e.signIn("contributor001@sample.invalid"), e.signIn("contributor002@sample.invalid")

	project := e.reviewProject(3)
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"Yes"}})
	e.staff(func(tx pgx.Tx) error {
		// A gold item, a qualification test made from it, a skip note and an
		// import file stand in for the rest of a real project's content.
		if _, err := tx.Exec(ctx, `WITH i AS (
			  INSERT INTO items (org_id, project_id, external_id, content) SELECT org_id, id, 'gold-1', '{"text":"Sample gold."}'
			  FROM projects WHERE id = $1 RETURNING id, org_id, project_id)
			INSERT INTO gold_items (org_id, project_id, item_id, expected) SELECT org_id, project_id, id, '{"labels":["Yes"]}' FROM i`, project); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `WITH t AS (
			  INSERT INTO qualification_tests (title, skill_kind, skill_value, task_type, pass_threshold, status, source_project_id)
			  VALUES ('Retention check (Sample)', 'task_type', 'classification', 'classification', 0.8, 'active', $1) RETURNING id)
			INSERT INTO qualification_questions (test_id, position, content, expected, task_type, config)
			SELECT id, 1, '{"text":"Sample gold."}', '{"labels":["Yes"]}', 'classification', '{"labels":["Yes","No"]}' FROM t`, project); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO import_jobs (org_id, project_id, kind, format, file_key, file_name, status, committed_at)
			SELECT org_id, id, 'items', 'csv', 'imports/retention-check.csv', 'items.csv', 'committed', now() FROM projects WHERE id = $1`, project)
		return err
	})
	if err := e.files.Put(ctx, "imports/retention-check.csv", "text/csv", []byte("text\nSample sentence.\n")); err != nil {
		t.Fatal(err)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, ops, nil); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("export: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables/1/publish", nil, ops, nil); w.Code >= 300 {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	var versions struct{ Versions []deliverableVersion }
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, client, nil), &versions)
	if len(versions.Versions) != 1 {
		t.Fatalf("versions: %+v", versions)
	}
	data := versions.Versions[0].file("data", "jsonl")
	if _, _, code := e.fetch(client, data); code != http.StatusOK {
		t.Fatalf("download before purge: %d", code)
	}
	fileKeys := []string{"imports/retention-check.csv"}
	e.staff(func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT file_key FROM deliverables WHERE project_id = $1 UNION SELECT manifest_key FROM deliverables WHERE project_id = $1`, project)
		if err != nil {
			return err
		}
		k, err := pgx.CollectRows(rows, pgx.RowTo[string])
		fileKeys = append(fileKeys, k...)
		return err
	})
	for _, k := range fileKeys {
		if !e.files.Has(k) {
			t.Fatalf("file %s missing before purge", k)
		}
	}
	ledger := func() int {
		return e.count(`SELECT count(*) * 1000000 + COALESCE(sum(amount_cents), 0) FROM earnings_ledger WHERE project_id = $1`, project)
	}
	ledgerBefore := ledger()
	if ledgerBefore == 0 {
		t.Fatal("no earnings to compare")
	}

	// Live: no due date yet; purge refused; clients may read the rule, not use it.
	var r retentionResp
	e.get(client, "/v1/projects/"+project+"/retention", &r)
	if r.Days != 90 || r.DueAt != nil || r.PurgedAt != nil {
		t.Fatalf("live retention: %+v", r)
	}
	body := map[string]string{"confirm": "Review check (Sample)", "reason": "Client asked by email on 17 Nov"}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/purge", body, client, nil); w.Code != http.StatusForbidden {
		t.Fatalf("client purge: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/purge", body, ops, nil); errorCode(w) != "project_not_closed" {
		t.Fatalf("live purge: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE projects SET status = 'closed', closed_at = now() WHERE id = $1`, project)
		return err
	})
	e.get(client, "/v1/projects/"+project+"/retention", &r)
	if r.DueAt == nil {
		t.Fatalf("closed retention: %+v", r)
	}
	// Not yet due: the sweep leaves it alone.
	sweeper := privacy.NewService(e.pool, e.files, nil)
	if _, err := sweeper.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if e.count(`SELECT count(*) FROM projects WHERE id = $1 AND content_purged_at IS NULL`, project) != 1 {
		t.Fatal("swept before due")
	}
	for _, bad := range []map[string]string{{"confirm": "Review check", "reason": "x"}, {"confirm": "Review check (Sample)", "reason": " "}} {
		if w := e.do(http.MethodPost, "/v1/projects/"+project+"/purge", bad, ops, nil); w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("bad purge %v: %d", bad, w.Code)
		}
	}
	w := e.do(http.MethodPost, "/v1/projects/"+project+"/purge", body, ops, nil)
	var res struct {
		Retention retentionResp
		Counts    map[string]int
	}
	decode(t, w, &res)
	if w.Code != http.StatusOK || res.Retention.PurgedAt == nil || res.Counts["items"] != 4 || res.Counts["answers"] != 6 ||
		res.Counts["qualification_questions"] != 1 || res.Counts["files"] != len(fileKeys) {
		t.Fatalf("purge: %d %s", w.Code, w.Body)
	}

	// Content gone everywhere it was held.
	for name, sql := range map[string]string{
		"items":           `SELECT count(*) FROM items WHERE project_id = $1 AND (content <> '{"purged": true}' OR external_id IS NOT NULL)`,
		"answers":         `SELECT count(*) FROM submission_revisions r JOIN submissions s ON s.id = r.submission_id WHERE s.project_id = $1 AND r.answer <> '{}'`,
		"final answers":   `SELECT count(*) FROM item_revisions r JOIN items i ON i.id = r.item_id WHERE i.project_id = $1 AND r.answer <> '{}'`,
		"gold":            `SELECT count(*) FROM gold_items WHERE project_id = $1 AND expected <> '{}'`,
		"consensus":       `SELECT count(*) FROM consensus_results WHERE project_id = $1 AND result <> '{}'`,
		"questions":       `SELECT count(*) FROM qualification_questions q JOIN qualification_tests t ON t.id = q.test_id WHERE t.source_project_id = $1 AND q.content <> '{"purged": true}'`,
		"live tests":      `SELECT count(*) FROM qualification_tests WHERE source_project_id = $1 AND status <> 'retired'`,
		"deliverables":    `SELECT count(*) FROM deliverables WHERE project_id = $1 AND purged_at IS NULL`,
		"queued deletes":  `SELECT count(*) FROM storage_deletions WHERE $1::uuid IS NOT NULL`,
		"no audit record": `SELECT 1 - count(*) FROM audit_log WHERE action = 'project.content_deleted' AND entity_id = $1 AND reason = 'Client asked by email on 17 Nov'`,
	} {
		if n := e.count(sql, project); n != 0 {
			t.Errorf("%s: %d left", name, n)
		}
	}
	for _, k := range fileKeys {
		if e.files.Has(k) {
			t.Errorf("file %s still stored", k)
		}
	}
	// Money and counts stay; quality figures still answer.
	if ledger() != ledgerBefore || e.count(`SELECT count(*) FROM items WHERE project_id = $1`, project) != 4 {
		t.Fatal("ledger or rows changed")
	}
	var quality struct {
		Items, Completed int
	}
	e.get(client, "/v1/projects/"+project+"/quality", &quality)
	if quality.Items != 3 || quality.Completed != 3 {
		t.Fatalf("quality after purge: %+v", quality)
	}
	// Deliverables say so instead of serving a missing file.
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, client, nil), &versions)
	if w := e.do(http.MethodPost, "/v1/deliverables/"+data.ID+"/links", map[string]string{"file": "data"}, client, nil); w.Code != http.StatusGone {
		t.Fatalf("link after purge: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, ops, nil); errorCode(w) != "content_deleted" {
		t.Fatalf("export after purge: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/purge", body, ops, nil); errorCode(w) != "already_deleted" {
		t.Fatalf("second purge: %d", w.Code)
	}
	// A purged project cannot be reopened, and the application still cannot
	// rewrite append-only answers itself: only the purge function can.
	for _, sql := range []string{
		`UPDATE projects SET status = 'live' WHERE id = '` + project + `'`,
		`UPDATE submission_revisions SET answer = '{"labels":["No"]}'`,
	} {
		err := e.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error { _, err := tx.Exec(ctx, sql); return err })
		if err == nil {
			t.Fatalf("allowed: %s", sql)
		}
	}

	// The sweep: a project closed 91 days ago with the default 90, a
	// declined request's sample past the same period, an old sent email.
	old := e.reviewProject(1)
	e.staff(func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE projects SET status = 'closed', closed_at = now() - interval '91 days' WHERE id = $1`, old); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO project_requests (org_id, title, task_type, brief_md, contact_name, contact_email,
			  sample_file_key, sample_file_name, sample_size, status, decision_note, decided_at, created_by)
			SELECT o.id, 'Old request (Sample)', 'classification', 'Sample.', 'Sample Person', 'person@sample.invalid',
			  'requests/old-sample.csv', 'sample.csv', 10, 'declined', 'Not a fit.', now() - interval '91 days', u.id
			FROM organisations o, users u WHERE o.slug = 'sample-client-ltd' AND u.email = 'client.admin@sample.invalid'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO email_outbox (to_address, template, data, status, sent_at, updated_at)
			VALUES ('person@sample.invalid', 'password_reset', '{"Name":"Sample Person"}', 'sent', now() - interval '31 days', now() - interval '31 days')`)
		return err
	})
	if err := e.files.Put(ctx, "requests/old-sample.csv", "text/csv", []byte("x")); err != nil {
		t.Fatal(err)
	}
	got, err := sweeper.Sweep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Projects < 1 || got.RequestSamples != 1 || got.Emails < 1 || e.files.Has("requests/old-sample.csv") {
		t.Fatalf("sweep: %+v", got)
	}
	if e.count(`SELECT count(*) FROM projects WHERE id = $1 AND content_purged_at IS NOT NULL`, old) != 1 ||
		e.count(`SELECT count(*) FROM audit_log WHERE action = 'project.content_deleted' AND entity_id = $1 AND actor_kind = 'system'`, old) != 1 ||
		e.count(`SELECT count(*) FROM email_outbox WHERE to_address = 'person@sample.invalid' AND data = '{}'`) != 1 {
		t.Fatal("sweep effects")
	}
}

type erasureResp struct {
	PublicCode string  `json:"public_code"`
	ErasedAt   *string `json:"erased_at"`
	Request    *struct {
		ID            string `json:"id"`
		ByContributor bool   `json:"by_contributor"`
	} `json:"request"`
	Blockers struct {
		BalanceCents       int64 `json:"balance_cents"`
		PendingPayoutLines int   `json:"pending_payout_lines"`
		UndecidedAnswers   int   `json:"undecided_answers"`
	} `json:"blockers"`
	CanErase bool `json:"can_erase"`
}

// Stage 17b, DPA 2019: a contributor can download their data and ask to be
// erased; Ops erases once nothing is owed. Money records stay, anonymous.
func TestContributorExportAndErasure(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ops := e.signIn("ops@sample.invalid")
	client := e.signIn("client.admin@sample.invalid")
	const email = "contributor001@sample.invalid"
	c1, c2 := e.signIn(email), e.signIn("contributor002@sample.invalid")
	id := e.contributorID(email)

	project := e.reviewProject(2)
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"Yes"}})
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE contributor_profiles SET cv_file_key = 'cv/erase-check.pdf', cv_file_name = 'cv.pdf', cv_uploaded_at = now() WHERE id = $1`, id)
		return err
	})
	if err := e.files.Put(ctx, "cv/erase-check.pdf", "application/pdf", []byte("%PDF-1.4")); err != nil {
		t.Fatal(err)
	}

	// Access: the person's own file, or Ops on their behalf; nobody else.
	w := e.do(http.MethodGet, "/v1/me/data-export", nil, c1, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Disposition"), "mezai-data-C-") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("export: %d %v", w.Code, w.Header())
	}
	var doc struct {
		NotIncluded []string `json:"not_included"`
		Data        struct {
			Account struct{ Email string }
			Profile struct {
				PublicCode string  `json:"public_code"`
				MpesaPhone *string `json:"mpesa_phone"`
			}
			Answers  []map[string]any
			Earnings []map[string]any
			Consents []map[string]any
		}
	}
	decode(t, w, &doc)
	if doc.Data.Account.Email != email || doc.Data.Profile.MpesaPhone == nil || len(doc.Data.Answers) != 2 ||
		len(doc.Data.Earnings) != 2 || doc.Data.Consents == nil || len(doc.NotIncluded) == 0 {
		t.Fatalf("export content: %s", w.Body)
	}
	if strings.Contains(w.Body.String(), "Sample sentence.") || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatal("export holds task content or secrets")
	}
	for who, s := range map[string]*session{"client": client, "contributor": c2} {
		if w := e.do(http.MethodGet, "/v1/contributors/"+id+"/data-export", nil, s, nil); w.Code != http.StatusForbidden {
			t.Fatalf("%s export of another: %d", who, w.Code)
		}
	}
	if w := e.do(http.MethodGet, "/v1/contributors/"+id+"/data-export", nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("ops export: %d", w.Code)
	}
	if e.count(`SELECT count(*) FROM audit_log WHERE action = 'contributor.data_exported' AND entity_id = $1`, id) != 2 {
		t.Fatal("export audit")
	}
	if w := e.do(http.MethodGet, "/v1/me/erasure", nil, ops, nil); w.Code != http.StatusForbidden {
		t.Fatalf("staff own erasure: %d", w.Code)
	}

	// Request: once, idempotent, visible to Ops with what blocks it.
	var er erasureResp
	decode(t, e.do(http.MethodPost, "/v1/me/erasure", map[string]string{"note": "I am leaving."}, c1, nil), &er)
	if er.Request == nil || !er.Request.ByContributor || er.CanErase || er.Blockers.BalanceCents != 5000 {
		t.Fatalf("request: %+v", er)
	}
	first := er.Request.ID
	decode(t, e.do(http.MethodPost, "/v1/me/erasure", map[string]string{}, c1, nil), &er)
	if er.Request == nil || er.Request.ID != first {
		t.Fatalf("second request: %+v", er)
	}
	var queue struct {
		Requests []struct {
			ContributorID string `json:"contributor_id"`
		}
	}
	e.get(ops, "/v1/erasure-requests", &queue)
	if len(queue.Requests) != 1 || queue.Requests[0].ContributorID != id {
		t.Fatalf("queue: %+v", queue)
	}
	body := map[string]string{"confirm": er.PublicCode, "reason": "Contributor asked in the app"}
	w = e.do(http.MethodPost, "/v1/contributors/"+id+"/erasure", body, ops, nil)
	if w.Code != http.StatusConflict || errorCode(w) != "erasure_blocked" || !strings.Contains(w.Body.String(), `"balance_cents":5000`) {
		t.Fatalf("blocked erase: %d %s", w.Code, w.Body)
	}

	// Pay what is owed; then erase.
	e.payContributor(id, 5000, "ERASE00001")
	if w := e.do(http.MethodPost, "/v1/contributors/"+id+"/erasure", map[string]string{"confirm": "C-9999", "reason": "x"}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong code: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+id+"/erasure", body, client, nil); w.Code != http.StatusForbidden {
		t.Fatalf("client erase: %d", w.Code)
	}
	ledgerBefore := e.count(`SELECT count(*) * 1000000 + COALESCE(sum(amount_cents), 0) FROM earnings_ledger WHERE contributor_id = $1`, id)
	w = e.do(http.MethodPost, "/v1/contributors/"+id+"/erasure", body, ops, nil)
	decode(t, w, &er)
	if w.Code != http.StatusOK || er.ErasedAt == nil || er.Request != nil {
		t.Fatalf("erase: %d %s", w.Code, w.Body)
	}

	for name, sql := range map[string]string{
		"account":        `SELECT count(*) FROM users u JOIN contributor_profiles cp ON cp.user_id = u.id WHERE cp.id = $1 AND (u.email NOT LIKE 'erased-%@erased.invalid' OR u.password_hash IS NOT NULL OR u.status <> 'disabled' OR u.display_name <> 'Erased contributor')`,
		"profile":        `SELECT count(*) FROM contributor_profiles WHERE id = $1 AND (mpesa_phone IS NOT NULL OR national_id_fp IS NOT NULL OR city IS NOT NULL OR cv_file_key IS NOT NULL OR experience <> '{}' OR status <> 'removed' OR erased_at IS NULL)`,
		"languages":      `SELECT count(*) FROM contributor_languages WHERE contributor_id = $1`,
		"interviews":     `SELECT count(*) FROM interviews WHERE contributor_id = $1`,
		"sessions":       `SELECT count(*) FROM sessions s JOIN contributor_profiles cp ON cp.user_id = s.user_id WHERE cp.id = $1`,
		"roles":          `SELECT count(*) FROM user_roles r JOIN contributor_profiles cp ON cp.user_id = r.user_id WHERE cp.id = $1`,
		"payout payee":   `SELECT count(*) FROM payout_lines WHERE contributor_id = $1 AND (mpesa_phone <> 'erased' OR payee_name <> 'Erased contributor')`,
		"consent ip":     `SELECT count(*) FROM consents c JOIN contributor_profiles cp ON cp.user_id = c.user_id WHERE cp.id = $1 AND c.ip IS NOT NULL`,
		"audit ip":       `SELECT count(*) FROM audit_log a JOIN contributor_profiles cp ON cp.user_id = a.actor_user_id WHERE cp.id = $1 AND a.ip IS NOT NULL`,
		"old emails":     `SELECT count(*) FROM email_outbox WHERE $1::uuid IS NOT NULL AND to_address = 'contributor001@sample.invalid'`,
		"open request":   `SELECT count(*) FROM erasure_requests WHERE contributor_id = $1 AND status <> 'done'`,
		"no audit entry": `SELECT 1 - count(*) FROM audit_log WHERE action = 'contributor.erased' AND entity_id = $1`,
		"no history":     `SELECT 1 - count(*) FROM contributor_status_history WHERE contributor_id = $1 AND to_status = 'removed'`,
	} {
		if n := e.count(sql, id); n != 0 {
			t.Errorf("%s: %d", name, n)
		}
	}
	if e.files.Has("cv/erase-check.pdf") {
		t.Error("CV still stored")
	}
	ledgerAfter := e.count(`SELECT count(*) * 1000000 + COALESCE(sum(amount_cents), 0) FROM earnings_ledger WHERE contributor_id = $1`, id)
	if ledgerAfter != ledgerBefore || e.count(`SELECT count(*) FROM payout_lines WHERE contributor_id = $1 AND mpesa_receipt = 'ERASE00001'`, id) != 1 {
		t.Fatal("money records changed")
	}
	if e.count(`SELECT count(*) FROM submissions WHERE contributor_id = $1`, id) != 2 {
		t.Fatal("client's answers removed")
	}
	// The person can no longer sign in; the old session is gone.
	if w, _ := e.login(email, pw); w.Code == http.StatusOK {
		t.Fatal("erased account signed in")
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, c1, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("old session: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+id+"/erasure", body, ops, nil); w.Code != http.StatusGone {
		t.Fatalf("erase twice: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/contributors/"+id+"/data-export", nil, ops, nil); w.Code != http.StatusGone {
		t.Fatalf("export after erase: %d", w.Code)
	}

	// Withdrawing a request.
	decode(t, e.do(http.MethodPost, "/v1/me/erasure", map[string]string{}, c2, nil), &er)
	if er.Request == nil {
		t.Fatal("c2 request")
	}
	w = e.do(http.MethodDelete, "/v1/me/erasure", nil, c2, nil)
	decode(t, w, &er)
	if w.Code != http.StatusOK || er.Request != nil {
		t.Fatalf("withdraw: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodDelete, "/v1/me/erasure", nil, c2, nil); w.Code != http.StatusNotFound {
		t.Fatalf("withdraw twice: %d", w.Code)
	}
	var raw json.RawMessage
	e.get(ops, "/v1/erasure-requests", &raw)
	if !strings.Contains(string(raw), `"requests":[]`) {
		t.Fatalf("queue after: %s", raw)
	}
}

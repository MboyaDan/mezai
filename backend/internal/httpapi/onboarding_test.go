package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
}

type profileResp struct {
	ID               string   `json:"id"`
	PublicCode       string   `json:"public_code"`
	Status           string   `json:"status"`
	MpesaPhoneMasked string   `json:"mpesa_phone_masked"`
	NationalIDLast3  *string  `json:"national_id_last3"`
	IdentityStatus   string   `json:"identity_status"`
	IdentityNote     *string  `json:"identity_note"`
	Missing          []string `json:"missing"`
	ConsentRequired  []string `json:"consent_required"`
	Consents         []struct {
		Document, Version string
		AcceptedAt        string `json:"accepted_at"`
	} `json:"consents"`
	Qualifications []struct {
		Title string `json:"title"`
	} `json:"qualifications"`
}

type attemptResp struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Questions []struct {
		ID       string `json:"id"`
		Position int    `json:"position"`
	} `json:"questions"`
	Score  *float64 `json:"score"`
	Passed *bool    `json:"passed"`
}

type testResp struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	State        string  `json:"state"`
	CanStart     bool    `json:"can_start"`
	DecisionNote *string `json:"decision_note"`
	RetakeAfter  *string `json:"retake_after"`
}

var currentConsents = map[string]string{"privacy_notice": "0.2", "contributor_terms": "0.2"}

const newEmail = "new.contributor@sample.invalid"

// signUpAndSignIn registers a contributor, verifies the email and signs in.
func (e *env) signUpAndSignIn(email string) *session {
	e.t.Helper()
	w := e.do(http.MethodPost, "/v1/contributors/signup", map[string]any{
		"email": email, "display_name": "Sample Applicant", "password": pw, "consents": currentConsents,
	}, nil, nil)
	if w.Code != http.StatusAccepted {
		e.t.Fatalf("sign-up: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": e.lastLink(email)}, nil, nil); w.Code != http.StatusNoContent {
		e.t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	_, s := e.login(email, pw)
	if s == nil {
		e.t.Fatal("login after verification failed")
	}
	return s
}

var completeProfile = map[string]any{
	"display_name": "Sample Applicant", "country": "ke", "city": "Nairobi", "education_level": "bachelor",
	"weekly_hours": 15, "domains": []string{"health", "education"},
	"languages":   []map[string]any{{"language": "sw", "proficiency": "native", "dialects": []string{"Sheng"}}, {"language": "en", "proficiency": "fluent"}},
	"mpesa_phone": "0712 345 678", "mpesa_phone_confirm": "+254712345678", "national_id": "2468 1357",
}

func (e *env) tests(s *session) map[string]testResp {
	e.t.Helper()
	w := e.do(http.MethodGet, "/v1/me/qualification/tests", nil, s, nil)
	if w.Code != http.StatusOK {
		e.t.Fatalf("tests: %d %s", w.Code, w.Body)
	}
	var out struct{ Tests []testResp }
	decode(e.t, w, &out)
	m := map[string]testResp{}
	for _, t := range out.Tests {
		m[t.Title] = t
	}
	return m
}

func (e *env) start(s *session, testID string) (*httptest.ResponseRecorder, attemptResp) {
	e.t.Helper()
	w := e.do(http.MethodPost, "/v1/me/qualification/tests/"+testID+"/attempts", map[string]any{}, s, nil)
	var a attemptResp
	if w.Code == http.StatusOK {
		decode(e.t, w, &a)
	}
	return w, a
}

// TestContributorOnboarding is AC-04: sign up, consent (version and time
// stored), complete the profile, take a qualification test, be approved.
func TestContributorOnboarding(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	// Consent is required, at the current version.
	for _, consents := range []map[string]string{nil, {"privacy_notice": "0.1"}, {"privacy_notice": "0.0", "contributor_terms": "0.1"}} {
		w := e.do(http.MethodPost, "/v1/contributors/signup", map[string]any{
			"email": newEmail, "display_name": "Sample Applicant", "password": pw, "consents": consents,
		}, nil, nil)
		if errorCode(w) != "consent_required" {
			t.Fatalf("consents %v: %d %s", consents, w.Code, w.Body)
		}
	}
	if n := e.count(`SELECT count(*) FROM users WHERE email = $1`, newEmail); n != 0 {
		t.Fatal("a refused sign-up created a user")
	}

	// A registered email gets the same answer as a new one.
	existing := e.do(http.MethodPost, "/v1/contributors/signup", map[string]any{
		"email": "contributor001@sample.invalid", "display_name": "X", "password": pw, "consents": currentConsents,
	}, nil, nil)
	if existing.Code != http.StatusAccepted {
		t.Fatalf("existing email: %d %s", existing.Code, existing.Body)
	}

	w := e.do(http.MethodPost, "/v1/contributors/signup", map[string]any{
		"email": newEmail, "display_name": "Sample Applicant", "password": pw, "consents": currentConsents,
	}, nil, nil)
	if w.Code != http.StatusAccepted || w.Body.String() != existing.Body.String() {
		t.Fatalf("sign-up: %d %s", w.Code, w.Body)
	}
	if w, _ := e.login(newEmail, pw); errorCode(w) != "email_not_verified" {
		t.Fatalf("login before verification: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM consents c JOIN users u ON u.id = c.user_id
		WHERE u.email = $1 AND c.version = '0.2' AND c.accepted_at IS NOT NULL
		  AND c.document IN ('privacy_notice', 'contributor_terms')`, newEmail); n != 2 {
		t.Fatalf("consent rows with version and time: %d", n)
	}
	if w := e.do(http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": e.lastLink(newEmail)}, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	_, s := e.login(newEmail, pw)
	if s == nil {
		t.Fatal("login after verification")
	}

	var p profileResp
	w = e.do(http.MethodGet, "/v1/me/contributor", nil, s, nil)
	decode(t, w, &p)
	if p.Status != "applied" || len(p.Missing) == 0 || len(p.ConsentRequired) != 0 || len(p.Consents) != 2 || !strings.HasPrefix(p.PublicCode, "C-") {
		t.Fatalf("new profile: %s", w.Body)
	}

	tests := e.tests(s)
	cls := tests["Swahili message routing test (Sample)"]
	if cls.ID == "" || cls.CanStart {
		t.Fatalf("an applied contributor sees tests but cannot start: %+v", cls)
	}
	if w, _ := e.start(s, cls.ID); errorCode(w) != "profile_incomplete" {
		t.Fatalf("start while applied: %d %s", w.Code, w.Body)
	}

	// Profile: the M-Pesa number must be entered twice.
	bad := map[string]any{}
	for k, v := range completeProfile {
		bad[k] = v
	}
	bad["mpesa_phone_confirm"] = "0712345679"
	if w := e.do(http.MethodPut, "/v1/me/contributor/profile", bad, s, nil); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"field":"mpesa_phone_confirm"`) {
		t.Fatalf("mismatched phone: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", completeProfile, s, nil)
	decode(t, w, &p)
	if w.Code != http.StatusOK || p.Status != "qualifying" || len(p.Missing) != 0 || p.MpesaPhoneMasked != "+254 7•• ••• 678" {
		t.Fatalf("complete profile: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "712345678") {
		t.Fatal("full phone number in the response")
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND mpesa_phone = '+254712345678' AND country = 'KE'`, p.ID); n != 1 {
		t.Fatal("phone not stored in E.164 or country not upper-cased")
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE coalesce(after::text, '') LIKE '%712345678%' OR coalesce(before::text, '') LIKE '%712345678%'`); n != 0 {
		t.Fatal("phone number written to the audit log")
	}

	// Take the classification test.
	cls = e.tests(s)["Swahili message routing test (Sample)"]
	if !cls.CanStart || cls.State != "available" {
		t.Fatalf("qualifying contributor: %+v", cls)
	}
	w, att := e.start(s, cls.ID)
	if w.Code != http.StatusOK || att.Status != "in_progress" || len(att.Questions) != 4 {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "expected") || strings.Contains(w.Body.String(), "results") {
		t.Fatalf("attempt shows answers to the contributor: %s", w.Body)
	}
	if _, again := e.start(s, cls.ID); again.ID != att.ID {
		t.Fatal("starting again must return the open attempt")
	}
	labels := []string{"Billing", "Network", "Account", "Other"} // seed order
	answers := map[string]any{}
	for _, q := range att.Questions {
		answers[q.ID] = map[string]any{"labels": []string{labels[q.Position-1]}}
	}
	first := att.Questions[0].ID
	if w := e.do(http.MethodPut, "/v1/me/qualification/attempts/"+att.ID+"/answers",
		map[string]any{"answers": map[string]any{first: answers[first]}}, s, nil); w.Code != http.StatusNoContent {
		t.Fatalf("autosave: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{}, s, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("submit with unanswered questions: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{"answers": answers}, s, nil)
	decode(t, w, &att)
	if w.Code != http.StatusOK || att.Status != "scored" || att.Score == nil || *att.Score != 1 || att.Passed == nil || !*att.Passed {
		t.Fatalf("submit: %d %s", w.Code, w.Body)
	}
	// A retried submit (for example a double click) changes nothing.
	again := e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{"answers": map[string]any{}}, s, nil)
	if again.Code != http.StatusOK || !strings.Contains(again.Body.String(), `"status":"scored"`) {
		t.Fatalf("second submit: %d %s", again.Code, again.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'qualification.attempt_scored' AND entity_id = $1`, att.ID); n != 1 {
		t.Fatalf("scored %d times", n)
	}
	if got := e.tests(s)["Swahili message routing test (Sample)"]; got.State != "awaiting_review" || got.CanStart {
		t.Fatalf("after submit: %+v", got)
	}

	// Another contributor cannot see the attempt.
	_, other := e.login("contributor002@sample.invalid", pw)
	if w := e.do(http.MethodGet, "/v1/me/qualification/attempts/"+att.ID, nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other contributor reads attempt: %d", w.Code)
	}

	// Ops approves; the contributor becomes active.
	_, ops := e.login("ops@sample.invalid", pw)
	w = e.do(http.MethodGet, "/v1/qualification/attempts", nil, ops, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), att.ID) {
		t.Fatalf("queue: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodGet, "/v1/qualification/attempts/"+att.ID, nil, ops, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"expected"`) || !strings.Contains(w.Body.String(), `"correct":true`) {
		t.Fatalf("review: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]string{"decision": "reject"}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reject without note: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]string{"decision": "approve"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]string{"decision": "approve"}, ops, nil); errorCode(w) != "attempt_closed" {
		t.Fatalf("second decision: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodGet, "/v1/me/contributor", nil, s, nil)
	decode(t, w, &p)
	if p.Status != "qualifying" || p.IdentityStatus != "pending" || len(p.Qualifications) != 1 {
		t.Fatalf("approved but identity not verified yet: %s", w.Body)
	}
	// Verifying the identity completes activation (D-078).
	if w := e.do(http.MethodPost, "/v1/contributors/"+p.ID+"/identity", map[string]string{"decision": "verify"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("verify identity: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodGet, "/v1/me/contributor", nil, s, nil)
	decode(t, w, &p)
	if p.Status != "active" || p.IdentityStatus != "verified" {
		t.Fatalf("after approval and verification: %s", w.Body)
	}
	if n := e.count(`SELECT count(*) FROM contributor_status_history WHERE contributor_id = $1`, p.ID); n != 3 {
		t.Fatalf("status history rows (applied, qualifying, active): %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'contributor.status_changed' AND entity_id = $1`, p.ID); n != 2 {
		t.Fatalf("audited status changes: %d", n)
	}
	if w, _ := e.start(s, cls.ID); errorCode(w) != "already_qualified" {
		t.Fatalf("retake after approval: %d %s", w.Code, w.Body)
	}

	// A failed attempt cannot be approved; a rejection shows the note and
	// reopens the test after the cooldown.
	pwTest := e.tests(s)["Response comparison test (Sample)"]
	_, att = e.start(s, pwTest.ID)
	wrong := map[string]any{}
	crit := map[string]int{"accuracy": 3, "helpfulness": 3, "safety": 3, "language_quality": 3}
	for _, q := range att.Questions {
		wrong[q.ID] = map[string]any{"preference": "b", "scores_a": crit, "scores_b": crit, "rationale": "B reads better to me."}
	}
	w = e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{"answers": wrong}, s, nil)
	decode(t, w, &att)
	if att.Passed == nil || *att.Passed || *att.Score != 0 {
		t.Fatalf("wrong answers: %s", w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]string{"decision": "approve"}, ops, nil); errorCode(w) != "not_passed" {
		t.Fatalf("approve failed attempt: %d %s", w.Code, w.Body)
	}
	note := "Both answers preferred the unsafe response. Read the safety guideline and try again."
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]string{"decision": "reject", "note": note}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	got := e.tests(s)["Response comparison test (Sample)"]
	if got.State != "not_passed" || got.DecisionNote == nil || *got.DecisionNote != note || got.RetakeAfter == nil || got.CanStart {
		t.Fatalf("after rejection: %+v", got)
	}
	if w, _ := e.start(s, pwTest.ID); errorCode(w) != "retake_too_soon" {
		t.Fatalf("retake during cooldown: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE qualification_attempts SET decided_at = now() - interval '8 days' WHERE id = $1`, att.ID)
		return err
	})
	if w, _ := e.start(s, pwTest.ID); w.Code != http.StatusOK {
		t.Fatalf("retake after cooldown: %d %s", w.Code, w.Body)
	}
}

func TestContributorStatusByOps(t *testing.T) {
	e := newEnv(t)
	s := e.signUpAndSignIn(newEmail)
	var p profileResp
	decode(t, e.do(http.MethodGet, "/v1/me/contributor", nil, s, nil), &p)

	_, ops := e.login("ops@sample.invalid", pw)
	set := func(status, reason string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/v1/contributors/"+p.ID+"/status", map[string]string{"status": status, "reason": reason}, ops, nil)
	}
	// Only approval makes a contributor active; applied cannot be paused.
	for _, to := range []string{"active", "paused", "qualifying"} {
		if w := set(to, "Testing"); errorCode(w) != "illegal_transition" {
			t.Fatalf("applied → %s: %d %s", to, w.Code, w.Body)
		}
	}
	if w := set("removed", ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no reason: %d %s", w.Code, w.Body)
	}

	// An active sample contributor can be paused and reinstated.
	active := e.contributorID("contributor001@sample.invalid")
	_, c1 := e.login("contributor001@sample.invalid", pw)
	if w := e.do(http.MethodPost, "/v1/contributors/"+active+"/status", map[string]string{"status": "paused", "reason": "Sample pause"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("pause: %d %s", w.Code, w.Body)
	}
	tests := e.tests(c1)
	for _, tt := range tests {
		if tt.CanStart {
			t.Fatalf("paused contributor can start %q", tt.Title)
		}
		if w, _ := e.start(c1, tt.ID); errorCode(w) != "not_eligible" {
			t.Fatalf("paused start: %d %s", w.Code, w.Body)
		}
		break
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+active+"/status", map[string]string{"status": "active", "reason": "Back"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("reinstate: %d %s", w.Code, w.Body)
	}

	// Removing signs the contributor out and is final.
	if w := set("removed", "Sample removal"); w.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/me/contributor", nil, s, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("removed contributor's session still works: %d", w.Code)
	}
	if w := set("active", "Undo"); errorCode(w) != "illegal_transition" {
		t.Fatalf("removed → active: %d %s", w.Code, w.Body)
	}

	// Lists mask phone numbers.
	w := e.do(http.MethodGet, "/v1/contributors?status=active", nil, ops, nil)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "+2547000000") || !strings.Contains(w.Body.String(), "•• •••") {
		t.Fatalf("contributor list: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodGet, "/v1/contributors/"+p.ID, nil, ops, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"to":"removed"`) {
		t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
}

func TestQualificationTestFromGold(t *testing.T) {
	e := newEnv(t)
	_, ops := e.login("ops@sample.invalid", pw)
	project := e.projectID("Customer message routing (Sample)")
	gold := e.count(`SELECT count(*) FROM gold_items WHERE project_id = $1 AND status = 'active'`, project)
	if gold == 0 {
		t.Fatal("seed has no gold on the sample project")
	}
	in := map[string]any{
		"title": "Routing check (Sample)", "skill_kind": "language", "skill_value": "sw", "pass_threshold": 0.5,
		"project_id": project, "question_count": 10,
	}
	if w := e.do(http.MethodPost, "/v1/qualification/tests", in, ops, nil); errorCode(w) != "content_not_confirmed" {
		t.Fatalf("without confirmation: %d %s", w.Code, w.Body)
	}
	in["content_shareable"] = true
	w := e.do(http.MethodPost, "/v1/qualification/tests", in, ops, nil)
	var created struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		QuestionCount int    `json:"question_count"`
		TaskType      string `json:"task_type"`
	}
	decode(t, w, &created)
	if w.Code != http.StatusCreated || created.Status != "draft" || created.QuestionCount != gold || created.TaskType != "classification" {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM gold_items WHERE project_id = $1 AND status = 'active'`, project); n != 0 {
		t.Fatalf("copied gold left active: %d", n)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/tests", in, ops, nil); errorCode(w) != "no_gold" {
		t.Fatalf("no gold left: %d %s", w.Code, w.Body)
	}

	_, c := e.login("contributor001@sample.invalid", pw)
	if _, ok := e.tests(c)["Routing check (Sample)"]; ok {
		t.Fatal("draft test visible to contributors")
	}
	if w := e.do(http.MethodPost, "/v1/qualification/tests/"+created.ID+"/status", map[string]string{"status": "active"}, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("activate: %d %s", w.Code, w.Body)
	}
	if tt, ok := e.tests(c)["Routing check (Sample)"]; !ok || !tt.CanStart {
		t.Fatalf("active test for an active contributor: %+v", tt)
	}
}

func TestConsentAcceptance(t *testing.T) {
	e := newEnv(t)
	s := e.signUpAndSignIn(newEmail)
	if w := e.do(http.MethodPost, "/v1/me/contributor/consents", map[string]any{"consents": map[string]string{"privacy_notice": "0.0"}}, s, nil); errorCode(w) != "consent_required" {
		t.Fatalf("stale version: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/me/contributor/consents", map[string]any{"consents": map[string]string{"cookies": "0.1"}}, s, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown document: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/me/contributor/consents", map[string]any{"consents": map[string]string{"acceptable_use": "0.1"}}, s, nil); w.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM consents c JOIN users u ON u.id = c.user_id WHERE u.email = $1`, newEmail); n != 3 {
		t.Fatalf("consent rows: %d", n)
	}
	// Staff hold profile.manage through Super Admin but have no contributor profile.
	_, admin := e.login("superadmin@sample.invalid", pw)
	if w := e.do(http.MethodGet, "/v1/me/contributor", nil, admin, nil); errorCode(w) != "not_a_contributor" {
		t.Fatalf("staff profile: %d %s", w.Code, w.Body)
	}
}

func (e *env) contributorID(email string) string {
	e.t.Helper()
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT cp.id::text FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id WHERE u.email = $1`, email).Scan(&id)
	})
	return id
}

func (e *env) projectID(name string) string {
	e.t.Helper()
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM projects WHERE name = $1`, name).Scan(&id)
	})
	return id
}

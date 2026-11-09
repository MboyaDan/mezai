package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const coreTitle = "MezAI core check (Sample)"

var writingSample = strings.TrimSpace(strings.Repeat("I read the instructions again and check each answer against them before I submit the task. ", 6))

// coreAnswers are the right answers to the sample core check, by position.
func coreAnswers(wrongFirst bool) map[int]any {
	crit := map[string]int{"accuracy": 4, "helpfulness": 4, "safety": 5, "language_quality": 4}
	first := "Complaint"
	if wrongFirst {
		first = "Thanks"
	}
	return map[int]any{
		1: map[string]any{"labels": []string{first}},
		2: map[string]any{"labels": []string{"Question"}},
		3: map[string]any{"labels": []string{"Other"}},
		4: map[string]any{"labels": []string{"Sentence C"}},
		5: map[string]any{"labels": []string{"Sentence B"}},
		6: map[string]any{"preference": "a", "scores_a": crit, "scores_b": crit, "rationale": "A gives a safe, correct time; B is wrong."},
		7: map[string]any{"text": writingSample},
	}
}

// takeCore starts and submits the core check with the given answers.
func (e *env) takeCore(s *session, byPosition map[int]any) attemptResp {
	e.t.Helper()
	core := e.tests(s)[coreTitle]
	w, att := e.start(s, core.ID)
	if w.Code != http.StatusOK {
		e.t.Fatalf("start core check: %d %s", w.Code, w.Body)
	}
	answers := map[string]any{}
	for _, q := range att.Questions {
		answers[q.ID] = byPosition[q.Position]
	}
	w = e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{"answers": answers}, s, nil)
	decode(e.t, w, &att)
	if w.Code != http.StatusOK || att.Status != "scored" {
		e.t.Fatalf("submit core check: %d %s", w.Code, w.Body)
	}
	return att
}

// passCore takes the core check with every answer right and has Ops approve it.
func (e *env) passCore(s, ops *session) {
	e.t.Helper()
	att := e.takeCore(s, coreAnswers(false))
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]any{"decision": "approve", "writing_rating": 3}, ops, nil); w.Code != http.StatusOK {
		e.t.Fatalf("approve core check: %d %s", w.Code, w.Body)
	}
}

// TestCoreCheck covers D-100 section 3.3 and 3.5: the core check comes first,
// mixes question types, scores six items and leaves the writing sample to a
// person, has a 30-day retake, and skills lapse after 12 months.
func TestCoreCheck(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.signUpAndSignIn("core.one@sample.invalid")
	_, ops := e.login("ops@sample.invalid", pw)
	var p profileResp
	decode(t, e.do(http.MethodPut, "/v1/me/contributor/profile", completeProfile, s, nil), &p)
	if w := e.do(http.MethodPost, "/v1/contributors/"+p.ID+"/screening", map[string]string{"decision": "shortlist"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("shortlist: %d", w.Code)
	}

	// Skill tests wait for the core check.
	tests := e.tests(s)
	routing := tests["Swahili message routing test (Sample)"]
	if routing.CanStart || !strings.Contains(e.do(http.MethodGet, "/v1/me/qualification/tests", nil, s, nil).Body.String(), `"core_first":true`) {
		t.Fatalf("skill test before the core check: %+v", routing)
	}
	if w, _ := e.start(s, routing.ID); errorCode(w) != "core_check_first" {
		t.Fatalf("start skill test first: %d %s", w.Code, w.Body)
	}
	if !tests[coreTitle].CanStart {
		t.Fatal("core check not open to a shortlisted applicant")
	}

	// Mixed types: each question carries its own type; answers never shown.
	core := tests[coreTitle]
	w, att := e.start(s, core.ID)
	body := w.Body.String()
	for _, want := range []string{`"task_type":"writing"`, `"task_type":"pairwise"`, `"Sentence A"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("attempt lacks %s: %s", want, body)
		}
	}
	if strings.Contains(body, `"expected"`) {
		t.Fatal("expected answers shown to the applicant")
	}
	// A writing sample under 60 words is refused at submit.
	short := coreAnswers(false)
	short[7] = map[string]any{"text": "Too short."}
	answers := map[string]any{}
	for _, q := range att.Questions {
		answers[q.ID] = short[q.Position]
	}
	if w := e.do(http.MethodPost, "/v1/me/qualification/attempts/"+att.ID+"/submit", map[string]any{"answers": answers}, s, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short writing: %d %s", w.Code, w.Body)
	}

	// One wrong of six scored items: 5/6 = 0.8333, passes the 0.8 mark; the
	// writing sample is not part of the score.
	att = e.takeCore(s, coreAnswers(true))
	if att.Score == nil || *att.Score < 0.83 || *att.Score > 0.84 || att.Passed == nil || !*att.Passed {
		t.Fatalf("score: %+v", att)
	}
	// Approval needs the writing read and rated adequate or better.
	for _, body := range []map[string]any{{"decision": "approve"}, {"decision": "approve", "writing_rating": 1}} {
		if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", body, ops, nil); errorCode(w) != "writing_rating_required" {
			t.Fatalf("approve %v: %d %s", body, w.Code, w.Body)
		}
	}
	w = e.do(http.MethodGet, "/v1/qualification/attempts/"+att.ID, nil, ops, nil)
	if !strings.Contains(w.Body.String(), `"has_writing":true`) || !strings.Contains(w.Body.String(), "I read the instructions again") {
		t.Fatalf("Ops review shows the writing: %s", w.Body)
	}
	// A rejected core check reopens after 30 days, not 7.
	if w := e.do(http.MethodPost, "/v1/qualification/attempts/"+att.ID+"/decision", map[string]any{"decision": "reject", "note": "The writing does not answer the prompt."}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM qualification_attempts WHERE id = $1 AND writing_rating IS NULL`, att.ID); n != 1 {
		t.Fatal("rating stored on a rejection without one")
	}
	got := e.tests(s)[coreTitle]
	if got.State != "not_passed" || got.RetakeAfter == nil {
		t.Fatalf("after rejection: %+v", got)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE qualification_attempts SET decided_at = now() - interval '8 days' WHERE id = $1`, att.ID)
		return err
	})
	if w, _ := e.start(s, core.ID); errorCode(w) != "retake_too_soon" {
		t.Fatalf("core retake after 8 days: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE qualification_attempts SET decided_at = now() - interval '31 days' WHERE id = $1`, att.ID)
		return err
	})
	e.passCore(s, ops)
	if got := e.tests(s)["Swahili message routing test (Sample)"]; !got.CanStart {
		t.Fatalf("skill tests after the core check: %+v", got)
	}

	// Skills lapse after 12 months: the project disappears and the test reopens.
	_, c1 := e.login("contributor001@sample.invalid", pw)
	if !strings.Contains(e.do(http.MethodGet, "/v1/me/projects", nil, c1, nil).Body.String(), "Customer message routing (Sample)") {
		t.Fatal("c1 should see the routing project")
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE qualification_attempts a SET decided_at = now() - interval '13 months'
			FROM qualification_tests t WHERE t.id = a.test_id AND t.skill_kind = 'language'
			AND a.contributor_id = (SELECT cp.id FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id WHERE u.email = 'contributor001@sample.invalid')`)
		return err
	})
	if strings.Contains(e.do(http.MethodGet, "/v1/me/projects", nil, c1, nil).Body.String(), "Customer message routing (Sample)") {
		t.Fatal("a lapsed skill still counts")
	}
	if got := e.tests(c1)["Swahili message routing test (Sample)"]; got.State != "expired" || !got.CanStart {
		t.Fatalf("lapsed skill test: %+v", got)
	}
	if got := e.tests(c1)[coreTitle]; got.State != "qualified" {
		t.Fatalf("the core check does not lapse: %+v", got)
	}

	// Ops can set up the real core check once the sample one is retired.
	if w := e.do(http.MethodPost, "/v1/qualification/core-check", map[string]any{}, ops, nil); errorCode(w) != "core_check_exists" {
		t.Fatalf("second core check: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/qualification/tests/"+core.ID+"/status", map[string]string{"status": "retired"}, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("retire sample: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodPost, "/v1/qualification/core-check", map[string]any{}, ops, nil)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"status":"draft"`) || !strings.Contains(w.Body.String(), `"question_count":7`) {
		t.Fatalf("create core check: %d %s", w.Code, w.Body)
	}
}

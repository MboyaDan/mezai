package httpapi_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/internal/identity"
)

var passingInterview = map[string]any{
	"outcome": "completed", "identity_confirmed": true, "confidentiality_ok": true,
	"scenario_score": 3, "language_scores": map[string]int{"sw": 4, "en": 3},
	"availability": "15 hours a week, evenings", "note": "Clear in both languages.",
}

// uploadCV sends a multipart PUT with the file in the "cv" field.
func (e *env) uploadCV(s *session, name string, data []byte) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("cv", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPut, "/v1/me/contributor/cv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", origin)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: s.cookie})
	req.Header.Set(httpapi.HeaderCSRF, s.csrf)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

var samplePDF = []byte("%PDF-1.4\n% Sample CV for tests only\n%%EOF\n")

// TestScreeningFunnel covers D-100 part 1: the experience form, the optional
// CV and when it is deleted, screening decisions, and the interview rubric.
func TestScreeningFunnel(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.signUpAndSignIn("screen.one@sample.invalid")
	b := e.signUpAndSignIn("screen.two@sample.invalid")
	_, ops := e.login("ops@sample.invalid", pw)

	// The experience form is part of a complete application.
	var p profileResp
	noExp := profileWith(map[string]any{"experience": nil})
	decode(t, e.do(http.MethodPut, "/v1/me/contributor/profile", noExp, a, nil), &p)
	if p.Status != "applied" || !strings.Contains(strings.Join(p.Missing, ","), "experience") {
		t.Fatalf("profile without experience: %+v", p)
	}
	long := profileWith(map[string]any{"experience": map[string]string{"highest_study": strings.Repeat("x", 201)}})
	if w := e.do(http.MethodPut, "/v1/me/contributor/profile", long, a, nil); !strings.Contains(w.Body.String(), `"field":"experience.highest_study"`) {
		t.Fatalf("long answer: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"linkedin_url": "https://example.com/me"}), a, nil); !strings.Contains(w.Body.String(), `"field":"linkedin_url"`) {
		t.Fatalf("bad LinkedIn link: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"linkedin_url": "https://www.linkedin.com/in/sample-applicant"}), a, nil), &p)
	if p.Status != "qualifying" {
		t.Fatalf("complete application: %+v", p)
	}
	aID := p.ID
	decode(t, e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{
		"national_id": "13572468", "mpesa_phone": "0711 000 222", "mpesa_phone_confirm": "0711000222"}), b, nil), &p)
	bID := p.ID

	// CV: PDF only, at most 2 MB; Ops downloads it as an attachment.
	if w := e.uploadCV(a, "cv.docx", []byte("PK\x03\x04 not a pdf")); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-PDF: %d %s", w.Code, w.Body)
	}
	if w := e.uploadCV(a, "cv.pdf", append([]byte("%PDF-"), make([]byte, contributors.MaxCVBytes)...)); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d", w.Code)
	}
	w := e.uploadCV(a, "Sample CV.pdf", samplePDF)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"cv_file_name":"Sample CV.pdf"`) {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodGet, "/v1/contributors/"+aID+"/cv", nil, ops, nil)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), samplePDF) ||
		!strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	if w := e.do(http.MethodGet, "/v1/contributors/"+aID+"/cv", nil, b, nil); w.Code != http.StatusForbidden {
		t.Fatalf("contributor downloads a CV: %d", w.Code)
	}
	var key string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT cv_file_key FROM contributor_profiles WHERE id = $1`, aID).Scan(&key)
	})
	if !strings.HasPrefix(key, "cv/"+aID+"/") || strings.Contains(key, "Sample") || !e.files.Has(key) {
		t.Fatalf("CV key %q (IDs only) and stored object", key)
	}

	// Screening: a decline needs a note and deletes the CV at once.
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/screening", map[string]string{"decision": "decline"}, ops, nil); !strings.Contains(w.Body.String(), `"field":"note"`) {
		t.Fatalf("decline without note: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/screening", map[string]string{"decision": "waitlist", "note": "No Swahili projects this month."}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("waitlist: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND cv_delete_after BETWEEN now() + interval '89 days' AND now() + interval '91 days'`, aID); n != 1 {
		t.Fatal("waitlisting did not start the 90-day CV clock")
	}
	note := "Your answers did not show the experience the current projects need."
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/screening", map[string]string{"decision": "decline", "note": note}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("decline: %d %s", w.Code, w.Body)
	}
	if e.files.Has(key) || e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND cv_file_key IS NULL`, aID) != 1 {
		t.Fatal("declined applicant's CV not deleted")
	}
	w = e.do(http.MethodGet, "/v1/me/contributor", nil, a, nil)
	if !strings.Contains(w.Body.String(), `"screening_status":"declined"`) || !strings.Contains(w.Body.String(), note) {
		t.Fatalf("applicant sees the decision: %s", w.Body)
	}
	if w := e.uploadCV(a, "cv.pdf", samplePDF); errorCode(w) != "not_eligible" {
		t.Fatalf("declined applicant uploads a CV: %d %s", w.Code, w.Body)
	}

	// The sweep deletes CVs whose 90 days have passed.
	if w := e.uploadCV(b, "cv.pdf", samplePDF); w.Code != http.StatusOK {
		t.Fatalf("b upload: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+bID+"/screening", map[string]string{"decision": "shortlist"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("shortlist: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE contributor_profiles SET cv_delete_after = now() - interval '1 minute' WHERE id = $1`, bID)
		return err
	})
	if n, err := contributors.NewService(e.pool, identity.DevKey, e.files).SweepCVs(ctx, nil); err != nil || n != 1 {
		t.Fatalf("sweep: %d %v", n, err)
	}
	if e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND cv_file_key IS NULL`, bID) != 1 {
		t.Fatal("expired CV kept")
	}

	// Interview: shortlisted only; languages on the profile only; the rubric decides.
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/interviews", passingInterview, ops, nil); errorCode(w) != "not_shortlisted" {
		t.Fatalf("interview a declined applicant: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+bID+"/interviews", map[string]any{"outcome": "no_answer"}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("no answer: %d %s", w.Code, w.Body)
	}
	bad := map[string]any{}
	for k, v := range passingInterview {
		bad[k] = v
	}
	bad["language_scores"] = map[string]int{"fr": 4}
	if w := e.do(http.MethodPost, "/v1/contributors/"+bID+"/interviews", bad, ops, nil); !strings.Contains(w.Body.String(), `"field":"language_scores"`) {
		t.Fatalf("language not on profile: %d %s", w.Code, w.Body)
	}
	bad["language_scores"] = map[string]int{"sw": 4, "en": 2}
	var d struct {
		Interviews []struct{ Outcome string } `json:"interviews"`
		Profile    struct {
			Status          string `json:"status"`
			InterviewPassed bool   `json:"interview_passed"`
		} `json:"profile"`
	}
	decode(t, e.do(http.MethodPost, "/v1/contributors/"+bID+"/interviews", bad, ops, nil), &d)
	if len(d.Interviews) != 2 || d.Interviews[0].Outcome != "failed" || d.Profile.InterviewPassed {
		t.Fatalf("weak English fails: %+v", d)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+bID+"/interviews", passingInterview, ops, nil); errorCode(w) != "interview_too_soon" {
		t.Fatalf("retry within 30 days: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action LIKE 'contributor.interview_%' AND entity_id = $1`, bID); n != 2 {
		t.Fatalf("audited interviews: %d", n)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+bID+"/interviews", passingInterview, b, nil); w.Code != http.StatusForbidden {
		t.Fatalf("contributor records an interview: %d", w.Code)
	}

	// Ops can list the screening queue.
	w = e.do(http.MethodGet, "/v1/contributors?screening=shortlisted", nil, ops, nil)
	if !strings.Contains(w.Body.String(), bID) || strings.Contains(w.Body.String(), aID) {
		t.Fatalf("screening filter: %s", w.Body)
	}
}

func TestInterviewRubric(t *testing.T) {
	ok := contributors.InterviewInput{IdentityConfirmed: true, ConfidentialityOK: true, ScenarioScore: 3, LanguageScores: map[string]int{"sw": 3}}
	if !contributors.InterviewPasses(ok) {
		t.Fatal("3s across the board pass")
	}
	for name, mod := range map[string]func(*contributors.InterviewInput){
		"identity":        func(i *contributors.InterviewInput) { i.IdentityConfirmed = false },
		"confidentiality": func(i *contributors.InterviewInput) { i.ConfidentialityOK = false },
		"scenario":        func(i *contributors.InterviewInput) { i.ScenarioScore = 2 },
		"language":        func(i *contributors.InterviewInput) { i.LanguageScores = map[string]int{"sw": 4, "en": 2} },
		"no language":     func(i *contributors.InterviewInput) { i.LanguageScores = nil },
	} {
		in := ok
		mod(&in)
		if contributors.InterviewPasses(in) {
			t.Errorf("%s: passed", name)
		}
	}
}

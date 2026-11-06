package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
)

func validEnquiry(email string) map[string]string {
	return map[string]string{
		"name": "Sample Buyer", "organisation": "Sample Lab (Sample)", "email": email,
		"project_type": "ai-evaluation", "languages": "Kiswahili, English", "volume": "1k-10k",
		"timeline": "1-3-month", "message": "We want to compare two model versions.\nPilot first.",
	}
}

// enquiryEnv builds a router with the given enquiry settings over a fresh database.
func enquiryEnv(t *testing.T, deps httpapi.EnquiryDeps) *env {
	t.Helper()
	e := newEnv(t)
	svc, err := auth.NewService(e.pool, func() auth.Options {
		o := auth.DefaultOptions(time.Hour, 30*time.Minute)
		o.AppOrigin = origin
		return o
	}())
	if err != nil {
		t.Fatal(err)
	}
	e.router = httpapi.NewRouter(httpapi.Deps{Auth: &httpapi.AuthDeps{
		Service: svc, Pool: e.pool, AppOrigin: origin, CookieSecure: true, Enquiry: deps,
	}})
	return e
}

func TestEnquirySavedEmailedAndAudited(t *testing.T) {
	e := enquiryEnv(t, httpapi.EnquiryDeps{NotifyTo: "ops-inbox@sample.invalid"})
	w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry(" Buyer@Sample.invalid "), nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("%d %s", w.Code, w.Body)
	}

	var id, status, email, msg string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT id::text, status, email, message FROM enquiries ORDER BY created_at DESC LIMIT 1`).Scan(&id, &status, &email, &msg)
	})
	if status != "new" || email != "buyer@sample.invalid" || !strings.Contains(msg, "\nPilot first.") {
		t.Fatalf("stored: %s %s %q", status, email, msg)
	}

	var data []byte
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT data FROM email_outbox WHERE to_address = 'ops-inbox@sample.invalid' AND template = 'enquiry_received'`).Scan(&data)
	})
	var d map[string]string
	_ = json.Unmarshal(data, &d)
	if d["ID"] != id || d["ProjectType"] != "AI evaluation and RLHF" || d["Volume"] != "1,000 to 10,000 items" {
		t.Fatalf("ops email data: %v", d)
	}

	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'enquiry.create' AND entity_id::text = $1
		AND actor_kind = 'system' AND NOT (after::text LIKE '%buyer@%')`, id); n != 1 {
		t.Fatalf("one audit entry without personal details, got %d", n)
	}
}

func TestEnquiryValidation(t *testing.T) {
	e := enquiryEnv(t, httpapi.EnquiryDeps{NotifyTo: "ops-inbox@sample.invalid"})
	bad := validEnquiry("not an email")
	bad["name"] = "  "
	bad["organisation"] = "Line\rbreak"
	bad["project_type"] = "mining"
	bad["volume"] = "lots"
	bad["message"] = strings.Repeat("x", 5001)
	w := e.do(http.MethodPost, "/v1/enquiries", bad, nil, nil)
	var body struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	want := map[string]string{"name": "required", "organisation": "invalid", "email": "invalid",
		"project_type": "required", "volume": "invalid", "message": "too_long"}
	if w.Code != http.StatusUnprocessableEntity || body.Error != "invalid_enquiry" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	for k, v := range want {
		if body.Fields[k] != v {
			t.Errorf("%s: want %s, got %q", k, v, body.Fields[k])
		}
	}
	if _, ok := body.Fields["timeline"]; ok {
		t.Error("a valid timeline must not be flagged")
	}
	named := validEnquiry("Sample <buyer@sample.invalid>")
	if w := e.do(http.MethodPost, "/v1/enquiries", named, nil, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("display-name addresses are refused: %d", w.Code)
	}
	if n := e.count(`SELECT count(*) FROM enquiries`); n != 0 {
		t.Fatalf("invalid enquiries are never stored, got %d", n)
	}
}

func TestEnquiryHoneypotAndLimits(t *testing.T) {
	e := enquiryEnv(t, httpapi.EnquiryDeps{NotifyTo: "ops-inbox@sample.invalid"})
	trap := validEnquiry("bot@sample.invalid")
	trap["website"] = "http://spam.invalid"
	w := e.do(http.MethodPost, "/v1/enquiries", trap, nil, nil)
	if w.Code != http.StatusAccepted || e.count(`SELECT count(*) FROM enquiries`) != 0 || e.count(`SELECT count(*) FROM email_outbox WHERE template = 'enquiry_received'`) != 0 {
		t.Fatalf("honeypot must look accepted and store nothing: %d", w.Code)
	}

	// Three from one address a day; the IP allows five an hour (the trap used one).
	for i := 0; i < 3; i++ {
		if w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry("same@sample.invalid"), nil, nil); w.Code != http.StatusAccepted {
			t.Fatalf("enquiry %d: %d %s", i+1, w.Code, w.Body)
		}
	}
	if w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry("same@sample.invalid"), nil, nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("fourth from one address: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry("other@sample.invalid"), nil, nil); w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("sixth from one IP: %d", w.Code)
	}
	if n := e.count(`SELECT count(*) FROM enquiries`); n != 3 {
		t.Fatalf("stored %d", n)
	}
}

func TestEnquiryCrossOriginRefused(t *testing.T) {
	e := enquiryEnv(t, httpapi.EnquiryDeps{NotifyTo: "ops-inbox@sample.invalid"})
	w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry("x@sample.invalid"), nil,
		map[string]string{"Origin": "http://evil.invalid", "Sec-Fetch-Site": "cross-site"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("%d", w.Code)
	}
}

func TestEnquiryTurnstile(t *testing.T) {
	var got string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.Form.Get("secret") + "|" + r.Form.Get("response")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": r.Form.Get("response") == "good-token"})
	}))
	defer fake.Close()
	e := enquiryEnv(t, httpapi.EnquiryDeps{NotifyTo: "ops-inbox@sample.invalid",
		Turnstile: httpapi.Turnstile{Secret: "test-secret", Endpoint: fake.URL}})

	req := validEnquiry("t@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/enquiries", req, nil, nil); errorCode(w) != "challenge_failed" {
		t.Fatalf("missing token: %d %s", w.Code, w.Body)
	}
	req["turnstile_token"] = "bad-token"
	if w := e.do(http.MethodPost, "/v1/enquiries", req, nil, nil); errorCode(w) != "challenge_failed" || got != "test-secret|bad-token" {
		t.Fatalf("bad token: %d %s (%s)", w.Code, w.Body, got)
	}
	req["turnstile_token"] = "good-token"
	if w := e.do(http.MethodPost, "/v1/enquiries", req, nil, nil); w.Code != http.StatusAccepted {
		t.Fatalf("good token: %d %s", w.Code, w.Body)
	}

	fake.Close()
	req["email"] = "t2@sample.invalid"
	if w := e.do(http.MethodPost, "/v1/enquiries", req, nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("siteverify down must fail closed: %d", w.Code)
	}
}

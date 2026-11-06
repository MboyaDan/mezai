package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/seed"
)

const (
	origin = "http://site.test"
	pw     = "test-password-only"
)

type env struct {
	t      *testing.T
	pool   *db.Pool
	router *gin.Engine
	rules  []httpapi.RouteRule
	files  *storage.Memory
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool, _ := dbtest.New(t)
	if _, err := seed.Run(context.Background(), pool, seed.Options{Password: pw}); err != nil {
		t.Fatal(err)
	}
	files := storage.NewMemory()
	opts := auth.DefaultOptions(time.Hour, 30*time.Minute)
	opts.AppOrigin = origin
	svc, err := auth.NewService(pool, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, rules := httpapi.NewRouterWithRules(httpapi.Deps{Auth: &httpapi.AuthDeps{
		Service: svc, Pool: pool, AppOrigin: origin, CookieSecure: true,
		Files: files, Enquiry: httpapi.EnquiryDeps{NotifyTo: "ops@mezai.invalid"},
	}})
	return &env{t: t, pool: pool, router: r, rules: rules, files: files}
}

type session struct{ cookie, csrf string }

func (e *env) do(method, path string, body any, s *session, headers map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if s != nil {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: s.cookie})
		req.Header.Set(httpapi.HeaderCSRF, s.csrf)
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *env) login(email, password string) (*httptest.ResponseRecorder, *session) {
	e.t.Helper()
	w := e.do(http.MethodPost, "/v1/auth/login", map[string]string{"email": email, "password": password}, nil, nil)
	if w.Code != http.StatusOK {
		return w, nil
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	for _, c := range w.Result().Cookies() {
		if c.Name == httpapi.SessionCookie {
			return w, &session{cookie: c.Value, csrf: body.CSRFToken}
		}
	}
	e.t.Fatal("no session cookie")
	return w, nil
}

func (e *env) staff(fn func(tx pgx.Tx) error) {
	e.t.Helper()
	if err := e.pool.InScope(context.Background(), db.Staff(), fn); err != nil {
		e.t.Fatal(err)
	}
}

func errorCode(w *httptest.ResponseRecorder) string {
	var b struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Error
}

func TestLoginSetsHardenedCookieAndMeWorks(t *testing.T) {
	e := newEnv(t)
	w, s := e.login("SuperAdmin@sample.invalid ", pw)
	if s == nil {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	var ck *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == httpapi.SessionCookie {
			ck = c
		}
	}
	if !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" {
		t.Errorf("cookie flags: %+v", ck)
	}
	// The database keeps SHA-256(token), never the token itself.
	var hashed, raw int
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash = sha256(convert_to($1, 'UTF8'))`, s.cookie).Scan(&hashed); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash = convert_to($1, 'UTF8')`, s.cookie).Scan(&raw)
	})
	if hashed != 1 || raw != 0 {
		t.Errorf("sessions matching sha256(token): %d, matching the raw token: %d", hashed, raw)
	}

	w = e.do(http.MethodGet, "/v1/auth/me", nil, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d %s", w.Code, w.Body)
	}
	var me struct {
		User struct {
			Kind        string   `json:"kind"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.User.Kind != "staff" || !contains(me.User.Permissions, "system.admin") {
		t.Errorf("me: %+v", me.User)
	}
}

func TestUserKinds(t *testing.T) {
	e := newEnv(t)
	for email, want := range map[string]string{
		"client.viewer@sample.invalid":  `"kind":"client"`,
		"contributor001@sample.invalid": `"kind":"contributor"`,
		"approver@sample.invalid":       `"kind":"staff"`,
	} {
		w, s := e.login(email, pw)
		if s == nil {
			t.Fatalf("%s: %d %s", email, w.Code, w.Body)
		}
		body := w.Body.String()
		if !strings.Contains(body, want) {
			t.Errorf("%s: want %s in %s", email, want, body)
		}
		if want == `"kind":"client"` && (!strings.Contains(body, `"org_id"`) || strings.Contains(body, "org.project.request")) {
			t.Errorf("client viewer: %s", body)
		}
		if want == `"kind":"contributor"` && !strings.Contains(body, `"contributor_id"`) {
			t.Errorf("contributor: %s", body)
		}
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	e := newEnv(t)
	a, _ := e.login("ops@sample.invalid", "not the password")
	b, _ := e.login("nobody@sample.invalid", "not the password")
	if a.Code != http.StatusUnauthorized || b.Code != http.StatusUnauthorized || a.Body.String() != b.Body.String() {
		t.Fatalf("responses differ: %d %s / %d %s", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestLockoutAfterFiveFailures(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		if w, _ := e.login("ops@sample.invalid", "wrong password "+string(rune('a'+i))); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, w.Code)
		}
	}
	// Locked: even the right password is refused, with the same generic answer.
	if w, s := e.login("ops@sample.invalid", pw); s != nil || errorCode(w) != "invalid_credentials" {
		t.Fatalf("locked account signed in: %d %s", w.Code, w.Body)
	}
	var locked int
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action = 'auth.locked'`).Scan(&locked)
	})
	if locked != 1 {
		t.Errorf("auth.locked audit entries: %d", locked)
	}
	// When the lock expires the right password works and the counter resets.
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE users SET locked_until = now() - interval '1 second' WHERE email = 'ops@sample.invalid'`)
		return err
	})
	if w, s := e.login("ops@sample.invalid", pw); s == nil {
		t.Fatalf("after lock expiry: %d %s", w.Code, w.Body)
	}
	var count int
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT failed_login_count FROM users WHERE email = 'ops@sample.invalid'`).Scan(&count)
	})
	if count != 0 {
		t.Errorf("failed_login_count after success: %d", count)
	}
}

func TestUnverifiedEmailCannotSignIn(t *testing.T) {
	e := newEnv(t)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE users SET email_verified_at = NULL WHERE email = 'reviewer@sample.invalid'`)
		return err
	})
	if w, _ := e.login("reviewer@sample.invalid", pw); w.Code != http.StatusForbidden || errorCode(w) != "email_not_verified" {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}

func TestCSRF(t *testing.T) {
	e := newEnv(t)
	_, s := e.login("ops@sample.invalid", pw)

	if w := e.do(http.MethodPost, "/v1/auth/logout", nil, s, map[string]string{httpapi.HeaderCSRF: ""}); w.Code != http.StatusForbidden {
		t.Errorf("logout without token: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/auth/logout", nil, s, map[string]string{httpapi.HeaderCSRF: "forged"}); w.Code != http.StatusForbidden {
		t.Errorf("logout with forged token: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/auth/logout", nil, s, map[string]string{"Origin": "https://evil.test"}); w.Code != http.StatusForbidden {
		t.Errorf("logout from another origin: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/auth/login", map[string]string{"email": "ops@sample.invalid", "password": pw}, nil,
		map[string]string{"Sec-Fetch-Site": "cross-site"}); w.Code != http.StatusForbidden {
		t.Errorf("cross-site login: %d", w.Code)
	}
	// Still signed in: none of the refused requests did anything.
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, s, nil); w.Code != http.StatusOK {
		t.Fatalf("me after refused logouts: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/auth/logout", nil, s, nil); w.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, s, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", w.Code)
	}
}

func TestIdleSessionExpires(t *testing.T) {
	e := newEnv(t)
	_, s := e.login("ops@sample.invalid", pw)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - interval '31 minutes'`)
		return err
	})
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, s, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("idle session still valid: %d", w.Code)
	}
}

func TestAnonymousIsRefused(t *testing.T) {
	e := newEnv(t)
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, nil, nil); w.Code != http.StatusUnauthorized || errorCode(w) != "unauthenticated" {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, &session{cookie: "made-up", csrf: "x"}, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("made-up cookie: %d", w.Code)
	}
}

func TestAuditTrailHasNoSecrets(t *testing.T) {
	e := newEnv(t)
	e.login("ops@sample.invalid", "a wrong password value")
	_, s := e.login("ops@sample.invalid", pw)
	e.do(http.MethodPost, "/v1/auth/logout", nil, s, map[string]string{httpapi.HeaderRequestID: "test-request-0001"})

	var actions []string
	var leaked int
	e.staff(func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT action FROM audit_log WHERE action LIKE 'auth.%' ORDER BY created_at, id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a string
			_ = rows.Scan(&a)
			actions = append(actions, a)
		}
		return tx.QueryRow(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE concat_ws(' ', reason, before::text, after::text) ILIKE ANY (ARRAY['%'||$1||'%', '%a wrong password value%', '%'||$2||'%'])`,
			pw, s.cookie).Scan(&leaked)
	})
	if strings.Join(actions, ",") != "auth.login_failed,auth.login,auth.logout" {
		t.Errorf("audit actions: %v", actions)
	}
	if leaked != 0 {
		t.Error("a password or session token reached the audit log")
	}
	var reqID string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT request_id FROM audit_log WHERE action = 'auth.logout'`).Scan(&reqID)
	})
	if reqID != "test-request-0001" {
		t.Errorf("logout audit request_id = %q", reqID)
	}
}

// Every /v1 route went through the registry with an explicit access rule.
func TestEveryV1RouteDeclaresAccess(t *testing.T) {
	e := newEnv(t)
	declared := map[string]bool{}
	for _, r := range e.rules {
		declared[r.Method+" "+r.Path] = true
		if !r.Access.Public && !r.Access.SignedIn && len(r.Access.Permissions) == 0 {
			t.Errorf("%s %s has an empty access rule", r.Method, r.Path)
		}
	}
	for _, r := range e.router.Routes() {
		if strings.HasPrefix(r.Path, "/v1/") && !declared[r.Method+" "+r.Path] {
			t.Errorf("%s %s registered without an access rule", r.Method, r.Path)
		}
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

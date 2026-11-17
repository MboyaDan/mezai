package httpapi_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/httpapi"
)

// code computes a TOTP code independently of the server (RFC 6238, SHA-1,
// 30 s, 6 digits) from the base32 secret the set-up step shows.
func code(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[off:off+4])&0x7fffffff)%1000000)
}

// passwordStep signs in with the password and returns the sign-in step
// cookie (two-step on) or nil (signed in at once).
func (e *env) passwordStep(email string) (*httptest.ResponseRecorder, string) {
	e.t.Helper()
	w := e.do(http.MethodPost, "/v1/auth/login", map[string]string{"email": email, "password": pw}, nil, nil)
	for _, c := range w.Result().Cookies() {
		if c.Name == httpapi.ChallengeCookie && c.MaxAge > 0 {
			return w, c.Value
		}
	}
	return w, ""
}

// codeStep sends a code with the step cookie; on success it returns the session.
func (e *env) codeStep(step, c string) (*httptest.ResponseRecorder, *session) {
	e.t.Helper()
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]string{"code": c})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login/code", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if step != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.ChallengeCookie, Value: step})
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return w, nil
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == httpapi.SessionCookie && ck.Value != "" {
			return w, &session{cookie: ck.Value, csrf: body.CSRFToken}
		}
	}
	e.t.Fatal("no session cookie after the code step")
	return w, nil
}

type twoStepStatus struct {
	Available         bool `json:"available"`
	Enabled           bool `json:"enabled"`
	RecoveryCodesLeft int  `json:"recovery_codes_left"`
}

// enrol switches two-step on for a signed-in staff user and returns the
// secret and recovery codes.
func (e *env) enrol(s *session) (string, []string) {
	e.t.Helper()
	var setup struct{ Secret, URI string }
	w := e.do(http.MethodPost, "/v1/me/two-step/setup", nil, s, nil)
	decode(e.t, w, &setup)
	if w.Code != http.StatusOK || !strings.HasPrefix(setup.URI, "otpauth://totp/MezAI:") || len(setup.Secret) != 32 {
		e.t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/me/two-step/confirm", map[string]string{"code": "000000"}, s, nil); errorCode(w) != "invalid_code" {
		e.t.Fatalf("wrong confirm code: %d %s", w.Code, w.Body)
	}
	var out struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	w = e.do(http.MethodPost, "/v1/me/two-step/confirm", map[string]string{"code": code(e.t, setup.Secret, time.Now())}, s, nil)
	decode(e.t, w, &out)
	if w.Code != http.StatusOK || len(out.RecoveryCodes) != 10 {
		e.t.Fatalf("confirm: %d %s", w.Code, w.Body)
	}
	return setup.Secret, out.RecoveryCodes
}

// Stage 17d: optional two-step sign-in for staff.
func TestTwoStepSignIn(t *testing.T) {
	e := newEnv(t)
	const opsEmail = "ops@sample.invalid"
	ops := e.signIn(opsEmail)

	var st twoStepStatus
	e.get(ops, "/v1/me/two-step", &st)
	if !st.Available || st.Enabled {
		t.Fatalf("status before: %+v", st)
	}
	// Contributors and clients cannot switch it on.
	for _, who := range []string{"contributor001@sample.invalid", "client.admin@sample.invalid"} {
		s := e.signIn(who)
		e.get(s, "/v1/me/two-step", &st)
		if st.Available {
			t.Fatalf("%s offered two-step", who)
		}
		if w := e.do(http.MethodPost, "/v1/me/two-step/setup", nil, s, nil); errorCode(w) != "two_step_unavailable" {
			t.Fatalf("%s setup: %d", who, w.Code)
		}
	}
	if w := e.do(http.MethodPost, "/v1/me/two-step/confirm", map[string]string{"code": "123456"}, ops, nil); errorCode(w) != "two_step_not_started" {
		t.Fatalf("confirm before setup: %d %s", w.Code, w.Body)
	}

	secret, recovery := e.enrol(ops)
	e.get(ops, "/v1/me/two-step", &st)
	if !st.Enabled || st.RecoveryCodesLeft != 10 {
		t.Fatalf("status after: %+v", st)
	}
	if w := e.do(http.MethodPost, "/v1/me/two-step/setup", nil, ops, nil); errorCode(w) != "two_step_on" {
		t.Fatalf("setup twice: %d", w.Code)
	}
	// The secret is stored encrypted and recovery codes as hashes.
	raw := strings.ToUpper(secret)
	if e.count(`SELECT count(*) FROM users WHERE email = $1 AND position(convert_to($2, 'UTF8') in totp_secret_enc) = 0
		AND totp_last_step IS NOT NULL`, opsEmail, raw) != 1 ||
		e.count(`SELECT count(*) FROM recovery_codes r JOIN users u ON u.id = r.user_id WHERE u.email = $1
		AND position(convert_to($2, 'UTF8') in r.code_hash) = 0`, opsEmail, recovery[0]) != 10 {
		t.Fatal("secrets stored in clear")
	}

	// The password alone no longer signs in: a step cookie and no session.
	w, step := e.passwordStep(opsEmail)
	if w.Code != http.StatusAccepted || step == "" || strings.Contains(w.Header().Get("Set-Cookie"), httpapi.SessionCookie+"=") {
		t.Fatalf("password step: %d %v", w.Code, w.Header())
	}
	if w, _ := e.codeStep("", code(t, secret, time.Now())); errorCode(w) != "challenge_expired" {
		t.Fatalf("no step cookie: %d", w.Code)
	}
	if w, _ := e.codeStep(step, "000000"); errorCode(w) != "invalid_code" {
		t.Fatalf("wrong code: %d", w.Code)
	}
	// The code that switched two-step on cannot be used again; use the next step's.
	next := code(t, secret, time.Now().Add(30*time.Second))
	w, s := e.codeStep(step, next)
	if w.Code != http.StatusOK || s == nil {
		t.Fatalf("code step: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, s, nil); w.Code != http.StatusOK {
		t.Fatalf("me after two-step: %d", w.Code)
	}
	// The same code never works twice, and a used step cookie is gone.
	if w, _ := e.codeStep(step, next); errorCode(w) != "challenge_expired" {
		t.Fatalf("step reused: %d", w.Code)
	}
	_, step2 := e.passwordStep(opsEmail)
	if w, _ := e.codeStep(step2, next); errorCode(w) != "invalid_code" {
		t.Fatalf("code replayed: %d %s", w.Code, w.Body)
	}
	// A recovery code works once, typed in any case.
	if w, s := e.codeStep(step2, strings.ToUpper(recovery[0])); w.Code != http.StatusOK || s == nil {
		t.Fatalf("recovery code: %d %s", w.Code, w.Body)
	}
	_, step3 := e.passwordStep(opsEmail)
	if w, _ := e.codeStep(step3, recovery[0]); errorCode(w) != "invalid_code" {
		t.Fatalf("recovery code reused: %d", w.Code)
	}
	e.get(s, "/v1/me/two-step", &st)
	if st.RecoveryCodesLeft != 9 {
		t.Fatalf("codes left: %+v", st)
	}
	if e.count(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = $1
		AND a.action = 'auth.login' AND a.after->>'second_factor' IN ('totp', 'recovery_code')`, opsEmail) != 2 {
		t.Fatal("audit of two-step sign-ins")
	}

	// Turning it off needs the password and a code.
	cur := code(t, secret, time.Now().Add(60*time.Second))
	if w := e.do(http.MethodPost, "/v1/me/two-step/disable", map[string]string{"password": "wrong-password-x", "code": cur}, s, nil); errorCode(w) != "invalid_credentials" {
		t.Fatalf("disable with wrong password: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/me/two-step/disable", map[string]string{"password": pw, "code": "000000"}, s, nil); errorCode(w) != "invalid_code" {
		t.Fatalf("disable with wrong code: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/me/two-step/disable", map[string]string{"password": pw, "code": recovery[1]}, s, nil); w.Code != http.StatusNoContent {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	if w, step := e.passwordStep(opsEmail); w.Code != http.StatusOK || step != "" {
		t.Fatalf("password alone after disabling: %d", w.Code)
	}
	if e.count(`SELECT count(*) FROM recovery_codes r JOIN users u ON u.id = r.user_id WHERE u.email = $1`, opsEmail) != 0 {
		t.Fatal("recovery codes kept after disabling")
	}
}

// Five wrong codes close the step; an admin can reset a lost phone, but only
// a Super Admin can reset a Super Admin's.
func TestTwoStepLimitsAndReset(t *testing.T) {
	e := newEnv(t)
	const revEmail = "reviewer@sample.invalid"
	rev := e.signIn(revEmail)
	secret, _ := e.enrol(rev)

	_, step := e.passwordStep(revEmail)
	for i := range 4 {
		if w, _ := e.codeStep(step, "000000"); errorCode(w) != "invalid_code" {
			t.Fatalf("wrong code %d: %d", i, w.Code)
		}
	}
	// The fifth wrong code also reaches the account's lockout of 5 failures.
	if w, _ := e.codeStep(step, "000000"); errorCode(w) != "invalid_code" {
		t.Fatalf("fifth wrong code: %d", w.Code)
	}
	if w, _ := e.codeStep(step, code(t, secret, time.Now().Add(30*time.Second))); errorCode(w) != "challenge_expired" {
		t.Fatalf("right code after five wrong: %d %s", w.Code, w.Body)
	}
	if e.count(`SELECT count(*) FROM users WHERE email = $1 AND locked_until > now()`, revEmail) != 1 {
		t.Fatal("wrong codes do not count towards lockout")
	}

	ops := e.signIn("ops@sample.invalid")
	admin := e.signIn("superadmin@sample.invalid")
	revID, adminID := e.userID(revEmail), e.userID("superadmin@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/users/"+revID+"/two-step/reset", map[string]string{"reason": " "[:0]}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reset without reason: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/users/"+revID+"/two-step/reset", map[string]string{"reason": "Lost phone, confirmed by call"}, rev, nil); w.Code != http.StatusForbidden {
		t.Fatalf("reviewer resets: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/users/"+revID+"/two-step/reset", map[string]string{"reason": "Lost phone, confirmed by call"}, ops, nil); w.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", w.Code, w.Body)
	}
	if e.count(`SELECT count(*) FROM users WHERE email = $1 AND totp_enabled_at IS NULL`, revEmail) != 1 ||
		e.count(`SELECT count(*) FROM audit_log WHERE action = 'auth.totp_reset' AND entity_id = $1 AND reason = 'Lost phone, confirmed by call'`, revID) != 1 {
		t.Fatal("reset effects")
	}
	if w := e.do(http.MethodPost, "/v1/users/"+revID+"/two-step/reset", map[string]string{"reason": "Again"}, ops, nil); errorCode(w) != "two_step_off" {
		t.Fatalf("reset twice: %d", w.Code)
	}

	e.enrol(admin)
	if w := e.do(http.MethodPost, "/v1/users/"+adminID+"/two-step/reset", map[string]string{"reason": "Lost phone"}, ops, nil); w.Code != http.StatusForbidden {
		t.Fatalf("Ops resets a Super Admin: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/users/"+adminID+"/two-step/reset", map[string]string{"reason": "Lost phone"}, admin, nil); errorCode(w) != "self" {
		t.Fatalf("own reset: %d", w.Code)
	}
}

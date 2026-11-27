package httpapi_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/googleauth"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/internal/legal"
)

// fakeGoogle is an OpenID Connect provider for tests: discovery, keys and a
// token endpoint that checks PKCE and signs real RS256 ID tokens, so go-oidc
// verifies them exactly as it would Google's.
type fakeGoogle struct {
	t      *testing.T
	srv    *httptest.Server
	key    *rsa.PrivateKey
	client string

	mu    sync.Mutex
	codes map[string]fakeCode
}

type fakeCode struct {
	challenge string
	claims    map[string]any
}

const fakeClientID = "test-client.apps.example"

func newFakeGoogle(t *testing.T) *fakeGoogle {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{t: t, key: key, client: fakeClientID, codes: map[string]fakeCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/auth", "token_endpoint": f.srv.URL + "/token",
			"jwks_uri": f.srv.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		c, ok := f.codes[r.PostForm.Get("code")]
		delete(f.codes, r.PostForm.Get("code"))
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != c.challenge {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": f.sign(c.claims)})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGoogle) sign(claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

// person is who chooses their account on Google's page.
type person struct {
	sub, email, name string
	unverified       bool
	wrongNonce       bool
}

// authorise plays Google's page: it reads the authorization URL our API
// redirected to and returns the callback query Google would send back.
func (f *fakeGoogle) authorise(location string, who person) url.Values {
	f.t.Helper()
	u, err := url.Parse(location)
	if err != nil || !strings.HasPrefix(location, f.srv.URL+"/auth") {
		f.t.Fatalf("redirect to %q, want the provider", location)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != f.client || q.Get("redirect_uri") != origin+"/api/v1/auth/google/callback" {
		f.t.Fatalf("authorization request: %v", q)
	}
	nonce := q.Get("nonce")
	if who.wrongNonce {
		nonce = "another"
	}
	code := "code-" + who.sub + "-" + q.Get("state")[:8]
	f.mu.Lock()
	f.codes[code] = fakeCode{challenge: q.Get("code_challenge"), claims: map[string]any{
		"iss": f.srv.URL, "aud": f.client, "sub": who.sub, "email": who.email, "email_verified": !who.unverified,
		"name": who.name, "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}}
	f.mu.Unlock()
	return url.Values{"code": {code}, "state": {q.Get("state")}}
}

func newGoogleEnv(t *testing.T) (*env, *fakeGoogle) {
	f := newFakeGoogle(t)
	e := newEnvWith(t, func(d *httpapi.AuthDeps) {
		d.Google = googleauth.New(googleauth.Config{ClientID: fakeClientID, ClientSecret: "test-secret",
			Issuer: f.srv.URL, RedirectURL: origin + "/api/v1/auth/google/callback"})
	})
	return e, f
}

var browserIP atomic.Int32

// browserGet is a top-level navigation: no Origin or JSON headers, cookies
// as given. Each call comes from its own test address, so a test with many
// round trips stays under the per-IP sign-in limit.
func (e *env) browserGet(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	n := browserIP.Add(1)
	req.RemoteAddr = fmt.Sprintf("198.51.%d.%d:4000", n/250, n%250+1)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

var signUpQuery = "intent=signup&privacy_notice=" + legal.Current[legal.PrivacyNotice] + "&contributor_terms=" + legal.Current[legal.ContributorTerms]

// googleRoundTrip starts at our API, goes through the fake Google and comes
// back to the callback, as a browser would. It returns the callback answer.
func (e *env) googleRoundTrip(f *fakeGoogle, query string, who person) *httptest.ResponseRecorder {
	e.t.Helper()
	w := e.browserGet("/v1/auth/google/start?" + query)
	if w.Code != http.StatusSeeOther {
		e.t.Fatalf("start: %d %s", w.Code, w.Header().Get("Location"))
	}
	state := cookieNamed(w, "mezai_google_state")
	if state == nil || !state.HttpOnly || !state.Secure || state.Path != "/api/v1/auth/google" || state.SameSite != http.SameSiteLaxMode {
		e.t.Fatalf("state cookie: %+v", state)
	}
	back := f.authorise(w.Header().Get("Location"), who)
	return e.browserGet("/v1/auth/google/callback?"+back.Encode(), state)
}

func wantRedirect(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != origin+want {
		t.Fatalf("answer %d to %q, want %q", w.Code, w.Header().Get("Location"), origin+want)
	}
}

// TestGoogleSignIn (Stage 29, D-171): sign-up with consents, sign-in again,
// linking by verified email (clearing an unproven password), contributors
// only, and the refusals.
func TestGoogleSignIn(t *testing.T) {
	e, f := newGoogleEnv(t)
	ctx := context.Background()

	var prov struct{ Google bool }
	if w := e.do(http.MethodGet, "/v1/auth/providers", nil, nil, nil); w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &prov) != nil || !prov.Google {
		t.Fatalf("providers: %d %s", w.Code, w.Body)
	}

	// Sign-up: a new contributor, signed in, email verified, consents recorded, no password.
	newbie := person{sub: "g-1001", email: "New.Person@Gmail.invalid", name: "  New   Person "}
	w := e.googleRoundTrip(f, signUpQuery, newbie)
	wantRedirect(t, w, "/c")
	sc := cookieNamed(w, httpapi.SessionCookie)
	if sc == nil || !sc.HttpOnly {
		t.Fatal("no session cookie after Google sign-up")
	}
	var me struct {
		User struct{ Kind, Email, DisplayName string } `json:"user"`
	}
	mw := e.do(http.MethodGet, "/v1/auth/me", nil, &session{cookie: sc.Value}, nil)
	if mw.Code != http.StatusOK || json.Unmarshal(mw.Body.Bytes(), &me) != nil || me.User.Kind != "contributor" || me.User.Email != "new.person@gmail.invalid" {
		t.Fatalf("me after sign-up: %d %s", mw.Code, mw.Body)
	}
	if n := e.count(`SELECT count(*) FROM users u JOIN consents c ON c.user_id = u.id
		WHERE u.email = 'new.person@gmail.invalid' AND u.email_verified_at IS NOT NULL AND u.password_hash IS NULL
		  AND u.display_name = 'New Person'`); n != len(legal.ContributorRequired) {
		t.Errorf("new account and consents: %d rows", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'contributor.signed_up' AND after->>'method' = 'google'`); n != 1 {
		t.Errorf("sign-up audit: %d", n)
	}

	// Signing in again with the same Google account: same user, no new one.
	w = e.googleRoundTrip(f, "intent=signin", newbie)
	wantRedirect(t, w, "/c")
	if n := e.count(`SELECT count(*) FROM users WHERE email = 'new.person@gmail.invalid'`); n != 1 {
		t.Errorf("users after second sign-in: %d", n)
	}

	// Sign-in with a Google account nobody has: back to sign-in, nothing created.
	wantRedirect(t, e.googleRoundTrip(f, "intent=signin", person{sub: "g-2002", email: "stranger@gmail.invalid"}), "/login?error=google_no_account")
	if n := e.count(`SELECT count(*) FROM users WHERE email = 'stranger@gmail.invalid'`); n != 0 {
		t.Errorf("sign-in created an account")
	}

	// Sign-up without the documents never leaves for Google.
	wantRedirect(t, e.browserGet("/v1/auth/google/start?intent=signup&privacy_notice=0.1"), "/join?error=consent_required")

	// Someone signed up with this address and a password but never verified
	// it. The real owner arrives through Google: linked, verified, and the
	// unproven password stops working.
	if c := e.do(http.MethodPost, "/v1/contributors/signup", map[string]any{"email": "owner@gmail.invalid", "display_name": "Owner",
		"password": "a-password-someone-else-set", "consents": legal.Current}, nil, nil).Code; c != http.StatusAccepted && c != http.StatusOK && c != http.StatusCreated {
		t.Fatalf("form sign-up: %d", c)
	}
	wantRedirect(t, e.googleRoundTrip(f, "intent=signin", person{sub: "g-3003", email: "owner@gmail.invalid", name: "Owner"}), "/c")
	if n := e.count(`SELECT count(*) FROM users u JOIN user_identities i ON i.user_id = u.id
		WHERE u.email = 'owner@gmail.invalid' AND u.password_hash IS NULL AND u.email_verified_at IS NOT NULL`); n != 1 {
		t.Errorf("linked account state: %d", n)
	}
	if lw, _ := e.login("owner@gmail.invalid", "a-password-someone-else-set"); lw.Code == http.StatusOK {
		t.Error("the unproven password still signs in")
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'auth.identity_linked' AND (after->>'password_cleared')::bool`); n != 1 {
		t.Errorf("link audit: %d", n)
	}

	// Staff keep the password sign-in: no session, nothing linked.
	staffBack := e.googleRoundTrip(f, "intent=signin", person{sub: "g-4004", email: "superadmin@sample.invalid"})
	wantRedirect(t, staffBack, "/login?error=google_use_password")
	if cookieNamed(staffBack, httpapi.SessionCookie) != nil || e.count(`SELECT count(*) FROM user_identities WHERE subject = 'g-4004'`) != 0 {
		t.Error("staff account signed in or linked through Google")
	}

	// Google has not verified the address; the ID token's nonce is not ours.
	wantRedirect(t, e.googleRoundTrip(f, signUpQuery, person{sub: "g-5005", email: "unverified@gmail.invalid", unverified: true}), "/join?error=google_unverified")
	wantRedirect(t, e.googleRoundTrip(f, signUpQuery, person{sub: "g-6006", email: "nonce@gmail.invalid", wrongNonce: true}), "/join?error=google_failed")
	if n := e.count(`SELECT count(*) FROM users WHERE email IN ('unverified@gmail.invalid', 'nonce@gmail.invalid')`); n != 0 {
		t.Errorf("refused sign-ups created %d accounts", n)
	}

	// The callback needs this browser's state cookie, and works once.
	start := e.browserGet("/v1/auth/google/start?intent=signin")
	state := cookieNamed(start, "mezai_google_state")
	back := f.authorise(start.Header().Get("Location"), newbie)
	wantRedirect(t, e.browserGet("/v1/auth/google/callback?"+back.Encode()), "/login?error=google_expired")
	wantRedirect(t, e.browserGet("/v1/auth/google/callback?"+back.Encode(), state), "/c")
	wantRedirect(t, e.browserGet("/v1/auth/google/callback?"+back.Encode(), state), "/login?error=google_expired")

	// Cancelling on Google's page goes back to the page the person came from.
	start = e.browserGet("/v1/auth/google/start?" + signUpQuery)
	q, _ := url.ParseQuery(strings.SplitN(start.Header().Get("Location"), "?", 2)[1])
	wantRedirect(t, e.browserGet("/v1/auth/google/callback?error=access_denied&state="+url.QueryEscape(q.Get("state")),
		cookieNamed(start, "mezai_google_state")), "/join?error=google_cancelled")

	// Only the state's hash is stored, and used rows are gone.
	e.staff(func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM oauth_states WHERE state_hash = convert_to($1, 'UTF8')`, q.Get("state")).Scan(&n); err != nil || n != 0 {
			t.Errorf("state stored in clear or kept: %d %v", n, err)
		}
		return nil
	})
}

// TestGoogleOff: without a client ID the button's routes send people back.
func TestGoogleOff(t *testing.T) {
	e := newEnv(t)
	var prov struct{ Google bool }
	if w := e.do(http.MethodGet, "/v1/auth/providers", nil, nil, nil); json.Unmarshal(w.Body.Bytes(), &prov) != nil || prov.Google {
		t.Fatalf("providers: %s", w.Body)
	}
	wantRedirect(t, e.browserGet("/v1/auth/google/start?intent=signin"), "/login?error=google_off")
}

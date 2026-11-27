package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/googleauth"
	"github.com/MboyaDan/mezai/backend/internal/legal"
)

// Google sign-in for contributors (Stage 29, D-171). Both routes are
// top-level browser navigations, not fetches: they answer with redirects.
//
// The state value travels twice: in the URL to Google and back, and in an
// HttpOnly cookie scoped to these routes. The callback needs both to match,
// so a callback link made for someone else's browser does nothing (login
// CSRF). The database keeps only the state's hash, with the PKCE verifier
// and the nonce, for 10 minutes, and uses it once.

const (
	googleStateCookie = "mezai_google_state"
	// googleCookiePath is the path the browser sees (the site's /api proxy).
	googleCookiePath = "/api/v1/auth/google"
)

type googleHandlers struct {
	*authHandlers
	google    *googleauth.Provider
	appOrigin string
}

func (h *googleHandlers) providers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"google": h.google != nil})
}

// back sends the browser to the page the person started from with an error
// code the page explains.
func (h *googleHandlers) back(c *gin.Context, intent, code string) {
	page := "/login"
	if intent == auth.GoogleSignUp {
		page = "/join"
	}
	c.Redirect(http.StatusSeeOther, h.appOrigin+page+"?error="+url.QueryEscape(code))
}

func (h *googleHandlers) stateCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: googleStateCookie, Value: value, Path: googleCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// start: GET /v1/auth/google/start?intent=signin|signup, plus for sign-up
// the accepted versions of privacy_notice and contributor_terms.
func (h *googleHandlers) start(c *gin.Context) {
	intent := c.Query("intent")
	if h.google == nil {
		h.back(c, intent, "google_off")
		return
	}
	consents := map[string]string{}
	for _, d := range legal.ContributorRequired {
		consents[d] = c.Query(d)
	}
	st, err := h.svc.StartGoogle(c.Request.Context(), intent, consents)
	switch {
	case errors.Is(err, auth.ErrConsentRequired):
		h.back(c, auth.GoogleSignUp, "consent_required")
		return
	case errors.Is(err, auth.ErrInvalidInput):
		h.back(c, auth.GoogleSignIn, "google_failed")
		return
	case err != nil:
		h.log.Error("google start failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		h.back(c, intent, "google_failed")
		return
	}
	to, err := h.google.AuthURL(c.Request.Context(), st.State, st.Nonce, st.Verifier)
	if err != nil {
		h.log.Error("google discovery failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		h.back(c, intent, "google_failed")
		return
	}
	h.stateCookie(c, st.State, 600) // as long as the stored state lives
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusSeeOther, to)
}

// callback: GET /v1/auth/google/callback?code=...&state=... (or &error=...).
func (h *googleHandlers) callback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	state := c.Query("state")
	cookie, _ := c.Cookie(googleStateCookie)
	h.stateCookie(c, "", -1)
	if h.google == nil {
		h.back(c, "", "google_off")
		return
	}
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie)) != 1 {
		h.back(c, "", "google_expired")
		return
	}
	ctx := c.Request.Context()
	st, err := h.svc.TakeGoogleState(ctx, state)
	if errors.Is(err, auth.ErrGoogleState) {
		h.back(c, "", "google_expired")
		return
	}
	if err != nil {
		h.log.Error("google state failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		h.back(c, "", "google_failed")
		return
	}
	if e := c.Query("error"); e != "" || c.Query("code") == "" {
		h.back(c, st.Intent, "google_cancelled") // the person closed Google's page or said no
		return
	}
	code := c.Query("code")
	sess, _, err := h.svc.FinishGoogle(ctx, st, func(ctx2 context.Context, verifier, nonce string) (googleauth.Claims, error) {
		return h.google.Exchange(ctx2, code, verifier, nonce)
	}, meta(c))
	switch {
	case err == nil:
		h.setCookie(c, sess.Token, time.Until(sess.ExpiresAt))
		c.Redirect(http.StatusSeeOther, h.appOrigin+"/c")
	case errors.Is(err, auth.ErrGoogleNoAccount):
		h.back(c, auth.GoogleSignIn, "google_no_account")
	case errors.Is(err, auth.ErrGoogleUsePassword):
		h.back(c, auth.GoogleSignIn, "google_use_password")
	case errors.Is(err, googleauth.ErrUnverifiedEmail):
		h.back(c, st.Intent, "google_unverified")
	case errors.Is(err, auth.ErrConsentRequired):
		h.back(c, auth.GoogleSignUp, "consent_required")
	case errors.Is(err, auth.ErrGoogleState):
		h.back(c, st.Intent, "google_expired")
	case errors.Is(err, auth.ErrInvalidCredentials):
		h.back(c, auth.GoogleSignIn, "google_failed")
	default:
		// Exchange and token errors land here; their text holds no secrets
		// (go-oidc and oauth2 do not echo tokens), but log only the kind.
		h.log.Warn("google sign-in failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		h.back(c, st.Intent, "google_failed")
	}
}

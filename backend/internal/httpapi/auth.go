package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/password"
)

type authHandlers struct {
	svc          *auth.Service
	log          *slog.Logger
	cookieSecure bool
	perEmail     *limiter
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meResponse struct {
	User      userView `json:"user"`
	CSRFToken string   `json:"csrf_token"`
}

type userView struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name"`
	Kind          auth.Kind  `json:"kind"`
	OrgID         *uuid.UUID `json:"org_id,omitempty"`
	ContributorID *uuid.UUID `json:"contributor_id,omitempty"`
	Permissions   []string   `json:"permissions"`
}

func view(p *auth.Principal) meResponse {
	u := userView{ID: p.UserID, Email: p.Email, DisplayName: p.DisplayName, Kind: p.Kind, Permissions: p.Permissions()}
	if p.OrgID != uuid.Nil {
		id := p.OrgID
		u.OrgID = &id
	}
	if p.ContributorID != uuid.Nil {
		id := p.ContributorID
		u.ContributorID = &id
	}
	return meResponse{User: u, CSRFToken: auth.CSRFToken(p.CSRFSecret)}
}

func (h *authHandlers) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abort(c, http.StatusBadRequest, "bad_request")
		return
	}
	email := normEmail(req.Email)
	if email == "" || req.Password == "" || len(req.Password) > 4*password.MaxLength {
		abort(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	// Per-email limit on top of the per-IP one, so one address cannot be
	// hammered from many IPs (the account lockout also applies).
	if ok, wait := h.perEmail.allow(email); !ok {
		tooMany(c, wait)
		return
	}
	sess, p, err := h.svc.Login(c.Request.Context(), email, req.Password, meta(c))
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		abort(c, http.StatusUnauthorized, "invalid_credentials")
		return
	case errors.Is(err, auth.ErrEmailNotVerified):
		abort(c, http.StatusForbidden, "email_not_verified")
		return
	case errors.Is(err, auth.ErrAccountSetup):
		abort(c, http.StatusForbidden, "account_not_ready")
		return
	case err != nil:
		h.log.Error("login failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
		return
	}
	h.setCookie(c, sess.Token, time.Until(sess.ExpiresAt))
	c.JSON(http.StatusOK, view(p))
}

func (h *authHandlers) logout(c *gin.Context) {
	if err := h.svc.Logout(c.Request.Context(), PrincipalFrom(c), meta(c)); err != nil {
		h.log.Error("logout failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
		return
	}
	h.setCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

func (h *authHandlers) me(c *gin.Context) {
	c.JSON(http.StatusOK, view(PrincipalFrom(c)))
}

func normEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (h *authHandlers) setCookie(c *gin.Context, value string, maxAge time.Duration) {
	ck := &http.Cookie{
		Name: SessionCookie, Value: value, Path: "/", HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode,
	}
	if maxAge < 0 {
		ck.MaxAge = -1
	} else {
		ck.MaxAge = int(maxAge.Seconds())
	}
	http.SetCookie(c.Writer, ck)
}

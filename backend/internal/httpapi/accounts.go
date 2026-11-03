package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/password"
)

type accountHandlers struct {
	svc      *auth.Service
	log      *slog.Logger
	perEmail *limiter
}

// fail maps service errors to stable HTTP codes; anything unknown is a 500.
func (h *accountHandlers) fail(c *gin.Context, err error) {
	var code string
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, password.ErrTooShort), errors.Is(err, password.ErrTooLong), errors.Is(err, password.ErrSameAsEmail):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "weak_password", "message": err.Error()})
		return
	case errors.Is(err, auth.ErrInvalidToken):
		code = "invalid_token"
	case errors.Is(err, auth.ErrInvalidInput):
		code = "bad_request"
	case errors.Is(err, auth.ErrEmailInUse):
		status, code = http.StatusConflict, "email_in_use"
	case errors.Is(err, auth.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, auth.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, auth.ErrNotGrantable):
		status, code = http.StatusForbidden, "not_grantable"
	case errors.Is(err, auth.ErrLastSuperAdmin):
		status, code = http.StatusConflict, "last_super_admin"
	case errors.Is(err, auth.ErrMixedKinds):
		status, code = http.StatusConflict, "mixed_user_kinds"
	case errors.Is(err, auth.ErrSelf):
		status, code = http.StatusConflict, "cannot_change_self"
	default:
		h.log.Error("account request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

func bind(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		abort(c, http.StatusBadRequest, "bad_request")
		return false
	}
	return true
}

func pathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		abort(c, http.StatusNotFound, "not_found")
		return uuid.Nil, false
	}
	return id, true
}

// accepted is the uniform answer for requests that must not reveal whether
// an email is registered.
func accepted(c *gin.Context) { c.JSON(http.StatusAccepted, gin.H{"status": "accepted"}) }

func (h *accountHandlers) limitEmail(c *gin.Context, email string) bool {
	if ok, wait := h.perEmail.allow(email); !ok {
		tooMany(c, wait)
		return false
	}
	return true
}

func (h *accountHandlers) requestReset(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if !bind(c, &req) || !h.limitEmail(c, normEmail(req.Email)) {
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), normEmail(req.Email), meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	accepted(c)
}

func (h *accountHandlers) confirmReset(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) verifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) resendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if !bind(c, &req) || !h.limitEmail(c, normEmail(req.Email)) {
		return
	}
	if err := h.svc.SendVerification(c.Request.Context(), normEmail(req.Email), meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	accepted(c)
}

func (h *accountHandlers) acceptInvitation(c *gin.Context) {
	var req struct {
		Token       string `json:"token"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.AcceptInvitation(c.Request.Context(), req.Token, req.DisplayName, req.Password, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type roleRequest struct {
	Role  string     `json:"role"`
	OrgID *uuid.UUID `json:"org_id"`
}

func (h *accountHandlers) invite(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		roleRequest
	}
	if !bind(c, &req) {
		return
	}
	inv, err := h.svc.Invite(c.Request.Context(), PrincipalFrom(c), req.Email, req.Role, req.OrgID, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h *accountHandlers) listInvitations(c *gin.Context) {
	out, err := h.svc.ListInvitations(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invitations": out})
}

func (h *accountHandlers) revokeInvitation(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.RevokeInvitation(c.Request.Context(), PrincipalFrom(c), id, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) listUsers(c *gin.Context) {
	out, err := h.svc.ListUsers(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *accountHandlers) grantRole(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req roleRequest
	if !ok || !bind(c, &req) {
		return
	}
	if err := h.svc.GrantRole(c.Request.Context(), PrincipalFrom(c), id, req.Role, req.OrgID, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) removeRole(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var org *uuid.UUID
	if s := c.Query("org_id"); s != "" {
		o, err := uuid.Parse(s)
		if err != nil {
			abort(c, http.StatusBadRequest, "bad_request")
			return
		}
		org = &o
	}
	if err := h.svc.RemoveRole(c.Request.Context(), PrincipalFrom(c), id, c.Param("role"), org, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) setStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Status string `json:"status"`
	}
	if !ok || !bind(c, &req) {
		return
	}
	if err := h.svc.SetUserStatus(c.Request.Context(), PrincipalFrom(c), id, req.Status, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *accountHandlers) listRoles(c *gin.Context) {
	out, err := h.svc.ListRoles(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"roles": out})
}

func (h *accountHandlers) listPermissions(c *gin.Context) {
	out, err := h.svc.ListPermissions(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": out})
}

func (h *accountHandlers) setRolePermissions(c *gin.Context) {
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.SetRolePermissions(c.Request.Context(), PrincipalFrom(c), c.Param("key"), req.Permissions, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

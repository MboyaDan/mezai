package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
)

// ChallengeCookie carries the sign-in step between the password and the
// two-step code (Stage 17d). Strict, HttpOnly and five minutes long.
const ChallengeCookie = "mezai_signin_step"

func (h *authHandlers) setChallenge(c *gin.Context, value string, maxAge time.Duration) {
	ck := &http.Cookie{Name: ChallengeCookie, Value: value, Path: "/", HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode}
	if maxAge < 0 {
		ck.MaxAge = -1
	} else {
		ck.MaxAge = int(maxAge.Seconds())
	}
	http.SetCookie(c.Writer, ck)
}

func (h *authHandlers) twoStepFail(c *gin.Context, err error) {
	var status int
	var code string
	switch {
	case errors.Is(err, auth.ErrChallengeClosed):
		h.setChallenge(c, "", -1)
		status, code = http.StatusUnauthorized, "challenge_expired"
	case errors.Is(err, auth.ErrBadCode):
		status, code = http.StatusUnauthorized, "invalid_code"
	case errors.Is(err, auth.ErrTOTPUnavailable):
		status, code = http.StatusForbidden, "two_step_unavailable"
	case errors.Is(err, auth.ErrTOTPNotSetUp):
		status, code = http.StatusServiceUnavailable, "two_step_not_configured"
	case errors.Is(err, auth.ErrTOTPEnabled):
		status, code = http.StatusConflict, "two_step_on"
	case errors.Is(err, auth.ErrTOTPOff):
		status, code = http.StatusConflict, "two_step_off"
	case errors.Is(err, auth.ErrNoPending):
		status, code = http.StatusConflict, "two_step_not_started"
	case errors.Is(err, auth.ErrWrongPassword):
		status, code = http.StatusUnauthorized, "invalid_credentials"
	case errors.Is(err, auth.ErrSelfReset):
		status, code = http.StatusConflict, "self"
	case errors.Is(err, auth.ErrReasonRequired):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "reason",
			"message": "give a reason of at most 500 characters"})
		return
	case errors.Is(err, auth.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, auth.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	default:
		h.log.Error("two-step request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

// loginCode completes a sign-in with a code from the app or a recovery code.
func (h *authHandlers) loginCode(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if !bind(c, &req) {
		return
	}
	challenge, _ := c.Cookie(ChallengeCookie)
	sess, p, err := h.svc.CompleteLogin(c.Request.Context(), challenge, req.Code, meta(c))
	if err != nil {
		h.twoStepFail(c, err)
		return
	}
	h.setChallenge(c, "", -1)
	h.setCookie(c, sess.Token, time.Until(sess.ExpiresAt))
	c.JSON(http.StatusOK, view(p))
}

func (h *authHandlers) totpStatus(c *gin.Context) {
	out, err := h.svc.GetTOTPStatus(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.twoStepFail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *authHandlers) totpSetup(c *gin.Context) {
	out, err := h.svc.StartTOTP(c.Request.Context(), PrincipalFrom(c), meta(c))
	if err != nil {
		h.twoStepFail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, out)
}

func (h *authHandlers) totpConfirm(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if !bind(c, &req) {
		return
	}
	codes, err := h.svc.ConfirmTOTP(c.Request.Context(), PrincipalFrom(c), req.Code, meta(c))
	if err != nil {
		h.twoStepFail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

func (h *authHandlers) totpDisable(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.DisableTOTP(c.Request.Context(), PrincipalFrom(c), req.Password, req.Code, meta(c)); err != nil {
		h.twoStepFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *authHandlers) totpReset(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !bind(c, &req) {
		return
	}
	if err := h.svc.ResetTOTP(c.Request.Context(), PrincipalFrom(c), id, req.Reason, meta(c)); err != nil {
		h.twoStepFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

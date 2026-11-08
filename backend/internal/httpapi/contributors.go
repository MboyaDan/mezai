package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/password"
)

// contributorHandlers serve contributor sign-up, profile and qualification
// (Stage 7) and the matching Ops screens.
type contributorHandlers struct {
	auth     *auth.Service
	svc      *contributors.Service
	log      *slog.Logger
	perEmail *limiter
}

// maxBody bounds JSON bodies on these routes (answers are the largest).
const maxBody = 256 << 10

func (h *contributorHandlers) fail(c *gin.Context, err error) {
	var fe *contributors.FieldError
	if errors.As(err, &fe) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": fe.Field, "message": fe.Message})
		return
	}
	status, code := http.StatusConflict, ""
	switch {
	case errors.Is(err, password.ErrTooShort), errors.Is(err, password.ErrTooLong), errors.Is(err, password.ErrSameAsEmail):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "weak_password", "message": err.Error()})
		return
	case errors.Is(err, auth.ErrInvalidInput), errors.Is(err, contributors.ErrInvalidInput):
		status, code = http.StatusBadRequest, "bad_request"
	case errors.Is(err, auth.ErrConsentRequired), errors.Is(err, contributors.ErrConsentRequired):
		code = "consent_required"
	case errors.Is(err, contributors.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, contributors.ErrNotContributor):
		status, code = http.StatusForbidden, "not_a_contributor"
	case errors.Is(err, contributors.ErrNotEligible):
		status, code = http.StatusForbidden, "not_eligible"
	case errors.Is(err, contributors.ErrIllegalTransition):
		code = "illegal_transition"
	case errors.Is(err, contributors.ErrProfileIncomplete):
		code = "profile_incomplete"
	case errors.Is(err, contributors.ErrAlreadyQualified):
		code = "already_qualified"
	case errors.Is(err, contributors.ErrRetakeTooSoon):
		code = "retake_too_soon"
	case errors.Is(err, contributors.ErrTestUnavailable):
		code = "test_unavailable"
	case errors.Is(err, contributors.ErrAttemptClosed):
		code = "attempt_closed"
	case errors.Is(err, contributors.ErrNotPassed):
		code = "not_passed"
	case errors.Is(err, contributors.ErrNoGold):
		code = "no_gold"
	case errors.Is(err, contributors.ErrIdentityIncomplete):
		code = "identity_incomplete"
	case errors.Is(err, contributors.ErrNotShareable):
		status, code = http.StatusUnprocessableEntity, "content_not_confirmed"
	default:
		h.log.Error("contributor request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

func (h *contributorHandlers) bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
	return bind(c, v)
}

func (h *contributorHandlers) signUp(c *gin.Context) {
	var req struct {
		Email       string            `json:"email"`
		DisplayName string            `json:"display_name"`
		Password    string            `json:"password"`
		Consents    map[string]string `json:"consents"`
	}
	if !h.bind(c, &req) {
		return
	}
	if ok, wait := h.perEmail.allow(normEmail(req.Email)); !ok {
		tooMany(c, wait)
		return
	}
	err := h.auth.SignUpContributor(c.Request.Context(), auth.SignUp{
		Email: req.Email, DisplayName: req.DisplayName, Password: req.Password, Consents: req.Consents,
	}, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	accepted(c) // the same answer whether or not the email was new
}

func (h *contributorHandlers) myProfile(c *gin.Context) {
	out, err := h.svc.MyProfile(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) updateMyProfile(c *gin.Context) {
	var req contributors.ProfileInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.UpdateMyProfile(c.Request.Context(), PrincipalFrom(c), req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) acceptConsents(c *gin.Context) {
	var req struct {
		Consents map[string]string `json:"consents"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.AcceptConsents(c.Request.Context(), PrincipalFrom(c), req.Consents, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) myTests(c *gin.Context) {
	out, err := h.svc.ListMyTests(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tests": out})
}

func (h *contributorHandlers) startAttempt(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.StartAttempt(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) myAttempt(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.MyAttempt(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type answersRequest struct {
	Answers map[string]json.RawMessage `json:"answers"`
}

func (h *contributorHandlers) saveAnswers(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req answersRequest
	if !ok || !h.bind(c, &req) {
		return
	}
	if req.Answers == nil {
		req.Answers = map[string]json.RawMessage{}
	}
	if err := h.svc.SaveAnswers(c.Request.Context(), PrincipalFrom(c), id, req.Answers); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *contributorHandlers) submitAttempt(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req answersRequest
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.Submit(c.Request.Context(), PrincipalFrom(c), id, req.Answers, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) listContributors(c *gin.Context) {
	out, err := h.svc.ListContributors(c.Request.Context(), PrincipalFrom(c), c.Query("status"), c.Query("identity"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributors": out})
}

func (h *contributorHandlers) getContributor(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetContributor(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) setContributorStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.SetStatus(c.Request.Context(), PrincipalFrom(c), id, req.Status, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) decideIdentity(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.DecideIdentity(c.Request.Context(), PrincipalFrom(c), id, req.Decision, req.Note, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) recordMpesaNameCheck(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.RecordMpesaNameCheck(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) listAttempts(c *gin.Context) {
	out, err := h.svc.ListAttempts(c.Request.Context(), PrincipalFrom(c), c.Query("status"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"attempts": out})
}

func (h *contributorHandlers) reviewAttempt(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.ReviewAttempt(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) decideAttempt(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.Decide(c.Request.Context(), PrincipalFrom(c), id, req.Decision, req.Note, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *contributorHandlers) listTests(c *gin.Context) {
	out, err := h.svc.ListTests(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tests": out})
}

func (h *contributorHandlers) createTest(c *gin.Context) {
	var req contributors.TestInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.CreateTest(c.Request.Context(), PrincipalFrom(c), req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *contributorHandlers) setTestStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Status string `json:"status"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	if err := h.svc.SetTestStatus(c.Request.Context(), PrincipalFrom(c), id, req.Status, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

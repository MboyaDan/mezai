package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

// taskHandlers serve the contributor task engine (Stage 9a) and the Ops
// content-report queue.
type taskHandlers struct {
	svc *tasks.Service
	log *slog.Logger
}

// HeaderIdempotencyKey identifies one submission attempt; a retry with the
// same key gets the first answer (AC-07).
const HeaderIdempotencyKey = "Idempotency-Key"

func (h *taskHandlers) fail(c *gin.Context, err error) {
	var fe *tasks.FieldError
	if errors.As(err, &fe) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": fe.Field, "message": fe.Message})
		return
	}
	var be *tasks.BlockedError
	if errors.As(err, &be) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "qa_blocked", "issues": be.Issues})
		return
	}
	var ie *quality.Invalid
	status, code := http.StatusConflict, ""
	switch {
	case errors.As(err, &ie):
		h.log.Error("project settings invalid", slog.String("request_id", RequestIDFrom(c)), slog.String("field", ie.Field))
		status, code = http.StatusInternalServerError, "internal_error"
	case errors.Is(err, tasks.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, tasks.ErrNotContributor):
		status, code = http.StatusForbidden, "not_a_contributor"
	case errors.Is(err, tasks.ErrNotEligible):
		status, code = http.StatusForbidden, "not_eligible"
	case errors.Is(err, tasks.ErrProjectClosed):
		code = "project_not_live"
	case errors.Is(err, tasks.ErrNoTasks):
		code = "no_tasks"
	case errors.Is(err, tasks.ErrReservationClosed):
		status, code = http.StatusGone, "reservation_closed"
	case errors.Is(err, tasks.ErrAlreadySubmitted):
		code = "already_submitted"
	case errors.Is(err, tasks.ErrKeyReused):
		status, code = http.StatusUnprocessableEntity, "idempotency_key_reused"
	case errors.Is(err, tasks.ErrDraftLimit):
		code = "draft_limit"
	case errors.Is(err, tasks.ErrReportClosed):
		code = "already_resolved"
	default:
		h.log.Error("task request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

func (h *taskHandlers) bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	return bind(c, v)
}

func (h *taskHandlers) myProjects(c *gin.Context) {
	out, err := h.svc.ListProjects(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

func (h *taskHandlers) myProject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetProject(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *taskHandlers) next(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Next(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *taskHandlers) task(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetTask(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *taskHandlers) saveDraft(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Answer       json.RawMessage `json:"answer"`
		TimeOnTaskMs int64           `json:"time_on_task_ms"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	if err := h.svc.SaveDraft(c.Request.Context(), PrincipalFrom(c), id, req.Answer, req.TimeOnTaskMs); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *taskHandlers) submit(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req tasks.SubmitInput
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.Submit(c.Request.Context(), PrincipalFrom(c), id, c.GetHeader(HeaderIdempotencyKey), req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *taskHandlers) skip(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct{ Reason, Note string }
	if !ok || !h.bind(c, &req) {
		return
	}
	if err := h.svc.Skip(c.Request.Context(), PrincipalFrom(c), id, req.Reason, req.Note, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *taskHandlers) report(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct{ Category, Note string }
	if !ok || !h.bind(c, &req) {
		return
	}
	if err := h.svc.Report(c.Request.Context(), PrincipalFrom(c), id, req.Category, req.Note, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *taskHandlers) listReports(c *gin.Context) {
	out, err := h.svc.ListReports(c.Request.Context(), PrincipalFrom(c), c.DefaultQuery("status", "open"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reports": out})
}

func (h *taskHandlers) resolveReport(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct{ Action, Reason string }
	if !ok || !h.bind(c, &req) {
		return
	}
	if err := h.svc.ResolveReport(c.Request.Context(), PrincipalFrom(c), id, req.Action, req.Reason, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

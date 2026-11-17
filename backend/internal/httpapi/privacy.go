package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/privacy"
)

// Privacy endpoints (Stage 17b): retention of task content and a
// contributor's rights of access and erasure.
type privacyHandlers struct {
	svc *privacy.Service
	log *slog.Logger
}

func (h *privacyHandlers) fail(c *gin.Context, err error) {
	var blocked *privacy.BlockedError
	switch {
	case errors.As(err, &blocked):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "erasure_blocked", "blockers": blocked.Blockers})
	case errors.Is(err, privacy.ErrNotFound):
		abort(c, http.StatusNotFound, "not_found")
	case errors.Is(err, privacy.ErrNotClosed):
		abort(c, http.StatusConflict, "project_not_closed")
	case errors.Is(err, privacy.ErrAlreadyPurged):
		abort(c, http.StatusConflict, "already_deleted")
	case errors.Is(err, privacy.ErrErased):
		abort(c, http.StatusGone, "erased")
	case errors.Is(err, privacy.ErrNoRequest):
		abort(c, http.StatusNotFound, "no_open_request")
	case errors.Is(err, privacy.ErrConfirm):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "confirm",
			"message": "type the name exactly as shown"})
	case errors.Is(err, privacy.ErrReason):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "reason",
			"message": "give a reason of at most 500 characters"})
	default:
		h.log.Error("privacy request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
	}
}

// myContributor answers 404 for accounts that are not contributors.
func myContributor(c *gin.Context) (uuid.UUID, bool) {
	p := PrincipalFrom(c)
	if p.ContributorID == uuid.Nil {
		abort(c, http.StatusNotFound, "not_found")
		return uuid.Nil, false
	}
	return p.ContributorID, true
}

func (h *privacyHandlers) retention(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.ProjectRetention(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) purge(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
		Reason  string `json:"reason"`
	}
	if !bind(c, &req) {
		return
	}
	out, err := h.svc.PurgeNow(c.Request.Context(), PrincipalFrom(c), id, req.Confirm, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) sendExport(c *gin.Context, contributorID uuid.UUID, self bool) {
	out, err := h.svc.ExportContributor(c.Request.Context(), PrincipalFrom(c).UserID, contributorID, self, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": out.FileName}))
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(http.StatusOK, "application/json", out.Data)
}

func (h *privacyHandlers) myExport(c *gin.Context) {
	if id, ok := myContributor(c); ok {
		h.sendExport(c, id, true)
	}
}

func (h *privacyHandlers) contributorExport(c *gin.Context) {
	if id, ok := pathID(c, "id"); ok {
		h.sendExport(c, id, false)
	}
}

func (h *privacyHandlers) myErasure(c *gin.Context) {
	id, ok := myContributor(c)
	if !ok {
		return
	}
	out, err := h.svc.GetErasure(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) requestMyErasure(c *gin.Context) {
	id, ok := myContributor(c)
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if !bind(c, &req) {
		return
	}
	out, err := h.svc.RequestErasure(c.Request.Context(), PrincipalFrom(c).UserID, id, req.Note, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) withdrawMyErasure(c *gin.Context) {
	id, ok := myContributor(c)
	if !ok {
		return
	}
	out, err := h.svc.WithdrawErasure(c.Request.Context(), PrincipalFrom(c).UserID, id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) contributorErasure(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetErasure(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) erase(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
		Reason  string `json:"reason"`
	}
	if !bind(c, &req) {
		return
	}
	out, err := h.svc.Erase(c.Request.Context(), PrincipalFrom(c).UserID, id, req.Confirm, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) withdrawErasure(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.WithdrawErasure(c.Request.Context(), PrincipalFrom(c).UserID, id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *privacyHandlers) listRequests(c *gin.Context) {
	out, err := h.svc.ListRequests(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": out})
}

package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/delivery"
)

// deliveryHandlers serve deliverable exports (Stage 11b).
type deliveryHandlers struct {
	svc *delivery.Service
	log *slog.Logger
}

func (h *deliveryHandlers) fail(c *gin.Context, err error) {
	status, code := http.StatusConflict, ""
	switch {
	case errors.Is(err, delivery.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, delivery.ErrInvalidInput):
		status, code = http.StatusUnprocessableEntity, "invalid_input"
	case errors.Is(err, delivery.ErrNothingToExport):
		code = "nothing_to_export"
	case errors.Is(err, delivery.ErrTooLarge):
		status, code = http.StatusUnprocessableEntity, "too_large"
	case errors.Is(err, delivery.ErrAlreadyPublished):
		code = "already_published"
	case errors.Is(err, delivery.ErrLinkInvalid):
		status, code = http.StatusForbidden, "link_expired"
	case errors.Is(err, delivery.ErrStorage):
		status, code = http.StatusServiceUnavailable, "storage_unavailable"
	default:
		// Includes ErrFileChanged: logged by the service, never shown in detail.
		h.log.Error("delivery request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

func (h *deliveryHandlers) list(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.List(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": out})
}

func (h *deliveryHandlers) export(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Export(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *deliveryHandlers) publish(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	v, err := strconv.ParseInt(c.Param("version"), 10, 32)
	if err != nil || v < 1 {
		abort(c, http.StatusNotFound, "not_found")
		return
	}
	if err := h.svc.Publish(c.Request.Context(), PrincipalFrom(c), id, int32(v), meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "published"})
}

func (h *deliveryHandlers) link(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		File string `json:"file"`
	}
	if !ok || !bind(c, &req) {
		return
	}
	out, err := h.svc.CreateLink(c.Request.Context(), PrincipalFrom(c), id, req.File)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *deliveryHandlers) download(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Download(c.Request.Context(), PrincipalFrom(c), id, c.Query("file"), c.Query("expires"), c.Query("sig"), meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": out.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(http.StatusOK, out.ContentType, out.Data)
}

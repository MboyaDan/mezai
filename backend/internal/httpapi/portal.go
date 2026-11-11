package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/portal"
)

// Client portal (Stage 13a): progress, workforce, activity, team and
// invoices. A client reads only its own organisation; the service checks
// row visibility in the caller's scope first (AC-10).
type portalHandlers struct {
	svc *portal.Service
	log *slog.Logger
}

func (h *portalHandlers) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, portal.ErrNotFound):
		abort(c, http.StatusNotFound, "not_found")
	case errors.Is(err, portal.ErrInvalidInput):
		abort(c, http.StatusUnprocessableEntity, "invalid_input")
	case errors.Is(err, portal.ErrConflict):
		abort(c, http.StatusConflict, "conflict")
	case errors.Is(err, portal.ErrStorage):
		abort(c, http.StatusServiceUnavailable, "storage_unavailable")
	default:
		h.log.Error("portal request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
	}
}

// queryUUID reads an optional UUID query parameter; a malformed one is 422.
func queryUUID(c *gin.Context, name string) (*uuid.UUID, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": name})
		return nil, false
	}
	return &id, true
}

func (h *portalHandlers) progress(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetProgress(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *portalHandlers) workforce(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetWorkforce(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *portalHandlers) activity(c *gin.Context) {
	org, ok := queryUUID(c, "org_id")
	if !ok {
		return
	}
	var before *time.Time
	if v := c.Query("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "before"})
			return
		}
		before = &t
	}
	out, err := h.svc.ListActivity(c.Request.Context(), PrincipalFrom(c), org, before)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *portalHandlers) members(c *gin.Context) {
	org, ok := queryUUID(c, "org_id")
	if !ok {
		return
	}
	out, err := h.svc.ListMembers(c.Request.Context(), PrincipalFrom(c), org)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": out})
}

func (h *portalHandlers) listInvoices(c *gin.Context) {
	org, ok := queryUUID(c, "org_id")
	if !ok {
		return
	}
	out, err := h.svc.ListInvoices(c.Request.Context(), PrincipalFrom(c), org)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invoices": out})
}

func (h *portalHandlers) uploadInvoice(c *gin.Context) {
	name, data, ok := upload(c, "file", portal.MaxInvoiceBytes, true)
	if !ok {
		return
	}
	in := portal.NewInvoice{Number: c.PostForm("number"), IssuedOn: c.PostForm("issued_on"), DueOn: c.PostForm("due_on"),
		Currency: c.PostForm("currency"), Description: c.PostForm("description")}
	org, err := uuid.Parse(c.PostForm("org_id"))
	amount, aerr := strconv.ParseInt(c.PostForm("amount_cents"), 10, 64)
	if err != nil || aerr != nil {
		abort(c, http.StatusUnprocessableEntity, "invalid_input")
		return
	}
	in.OrgID, in.AmountCents = org, amount
	if v := c.PostForm("project_id"); v != "" {
		pid, err := uuid.Parse(v)
		if err != nil {
			abort(c, http.StatusUnprocessableEntity, "invalid_input")
			return
		}
		in.ProjectID = &pid
	}
	out, err := h.svc.UploadInvoice(c.Request.Context(), PrincipalFrom(c), in, name, data, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *portalHandlers) setInvoiceStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Status string `json:"status"`
		PaidOn string `json:"paid_on"`
		Reason string `json:"reason"`
	}
	if !ok || !bind(c, &req) {
		return
	}
	out, err := h.svc.SetInvoiceStatus(c.Request.Context(), PrincipalFrom(c), id, req.Status, req.PaidOn, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// downloadInvoice always sends the PDF as an attachment that the browser
// may not sniff or cache.
func (h *portalHandlers) downloadInvoice(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.DownloadInvoice(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": out.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/pdf", out.Data)
}

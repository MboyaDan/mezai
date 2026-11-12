package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/payouts"
)

// Payouts, manual M-Pesa path (Stage 14a, AC-11). Ops (payout.create)
// creates batches, exports the file and records results; a second person
// (payout.approve) approves. The service refuses the creator's approval.
type payoutHandlers struct {
	svc *payouts.Service
	log *slog.Logger
}

func (h *payoutHandlers) fail(c *gin.Context, err error) {
	var ie *payouts.InputError
	switch {
	case errors.As(err, &ie):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": ie.Field, "message": ie.Message})
	case errors.Is(err, payouts.ErrNotFound):
		abort(c, http.StatusNotFound, "not_found")
	case errors.Is(err, payouts.ErrOwnBatch):
		abort(c, http.StatusForbidden, "own_batch")
	case errors.Is(err, payouts.ErrWrongState):
		abort(c, http.StatusConflict, "wrong_state")
	case errors.Is(err, payouts.ErrOpenBatch):
		abort(c, http.StatusConflict, "open_batch")
	case errors.Is(err, payouts.ErrBalanceChanged):
		abort(c, http.StatusConflict, "balance_changed")
	case errors.Is(err, payouts.ErrReceiptUsed):
		abort(c, http.StatusConflict, "receipt_used")
	case errors.Is(err, payouts.ErrEmpty):
		abort(c, http.StatusUnprocessableEntity, "nothing_to_pay")
	case errors.Is(err, payouts.ErrStorage):
		abort(c, http.StatusServiceUnavailable, "storage_unavailable")
	case errors.Is(err, payouts.ErrFileChanged):
		h.log.Error("payout export checksum mismatch", slog.String("request_id", RequestIDFrom(c)))
		abort(c, http.StatusInternalServerError, "file_changed")
	default:
		h.log.Error("payout request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
	}
}

func (h *payoutHandlers) bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	return bind(c, v)
}

func (h *payoutHandlers) preview(c *gin.Context) {
	out, err := h.svc.Preview(c.Request.Context(), PrincipalFrom(c), c.Query("period_end"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *payoutHandlers) list(c *gin.Context) {
	out, err := h.svc.List(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"batches": out})
}

func (h *payoutHandlers) create(c *gin.Context) {
	var req struct {
		PeriodEnd string `json:"period_end"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, created, err := h.svc.Create(c.Request.Context(), PrincipalFrom(c), req.PeriodEnd, c.GetHeader(HeaderIdempotencyKey), meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, out)
}

func (h *payoutHandlers) get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Get(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// action runs a state change that takes an optional reason.
func (h *payoutHandlers) action(do func(c *gin.Context, reason string) (payouts.Batch, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Reason string `json:"reason"`
		}
		if c.Request.ContentLength != 0 && !h.bind(c, &req) {
			return
		}
		out, err := do(c, req.Reason)
		if errors.Is(err, errAborted) {
			return
		}
		if err != nil {
			h.fail(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func (h *payoutHandlers) withID(c *gin.Context, fn func(id uuid.UUID) (payouts.Batch, error)) (payouts.Batch, error) {
	id, ok := pathID(c, "id")
	if !ok {
		return payouts.Batch{}, errAborted
	}
	return fn(id)
}

var errAborted = errors.New("request already answered")

func (h *payoutHandlers) submit(c *gin.Context, _ string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Submit(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	})
}

func (h *payoutHandlers) approve(c *gin.Context, _ string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Approve(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	})
}

func (h *payoutHandlers) sendBack(c *gin.Context, reason string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Return(c.Request.Context(), PrincipalFrom(c), id, reason, meta(c))
	})
}

func (h *payoutHandlers) cancel(c *gin.Context, reason string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Cancel(c.Request.Context(), PrincipalFrom(c), id, reason, meta(c))
	})
}

func (h *payoutHandlers) export(c *gin.Context, _ string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Export(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	})
}

// download sends the bulk payment file as an attachment; it holds full
// phone numbers, so it is never cached.
func (h *payoutHandlers) download(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Download(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": out.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", out.Data)
}

func (h *payoutHandlers) result(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req payouts.Result
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.RecordResult(c.Request.Context(), PrincipalFrom(c), id, req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *payoutHandlers) nextDate(c *gin.Context) {
	var req struct {
		Date string `json:"date"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.SetNextDate(c.Request.Context(), PrincipalFrom(c), req.Date, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"date": out})
}

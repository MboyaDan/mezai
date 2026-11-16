package httpapi

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/mpesa"
	"github.com/MboyaDan/mezai/backend/internal/payouts"
)

// MpesaDeps switches on M-Pesa B2C payouts (Stage 15, DG-02). Nil keeps
// them off: batches can only be exported for the Business Portal.
type MpesaDeps struct {
	Client      mpesa.Client
	Environment string // fake, sandbox or production
	// CallbackToken is the secret path segment Daraja's callbacks carry.
	CallbackToken string
	// CallbackIPs, when not empty, are the only source addresses accepted.
	CallbackIPs []netip.Prefix
}

// B2C staff actions.

func (h *payoutHandlers) disburse(c *gin.Context, _ string) (payouts.Batch, error) {
	return h.withID(c, func(id uuid.UUID) (payouts.Batch, error) {
		return h.svc.Disburse(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	})
}

func (h *payoutHandlers) transactions(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.Transactions(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"b2c": h.svc.B2CInfo(), "transactions": out})
}

func (h *payoutHandlers) stopUnsent(c *gin.Context) {
	id, ok := pathID(c, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	if !ok || !h.bind(c, &req) {
		return
	}
	out, err := h.svc.StopUnsent(c.Request.Context(), PrincipalFrom(c), id, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *payoutHandlers) reconciliation(c *gin.Context) {
	out, err := h.svc.Reconcile(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// mpesaCallbacks receive Daraja's result and timeout posts. Daraja does not
// sign them, so a callback is believed only when (1) its URL carries our
// secret token, (2) it comes from an allowed address when a list is set,
// and (3) it names a transaction we sent (and, for B2C, the ConversationID
// Daraja gave us). Anything failing (1) or (2) is refused unseen; (3) is
// stored as unmatched for Ops. Answers never say why.
type mpesaCallbacks struct {
	svc  *payouts.Service
	deps *MpesaDeps
	log  *slog.Logger
}

func (h *mpesaCallbacks) handle(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.deps == nil || subtle.ConstantTimeCompare([]byte(c.Param("token")), []byte(h.deps.CallbackToken)) != 1 {
			abort(c, http.StatusNotFound, "not_found")
			return
		}
		ip, err := netip.ParseAddr(c.ClientIP())
		if err != nil {
			abort(c, http.StatusForbidden, "forbidden")
			return
		}
		ip = ip.Unmap()
		if len(h.deps.CallbackIPs) > 0 {
			allowed := false
			for _, p := range h.deps.CallbackIPs {
				if p.Contains(ip) {
					allowed = true
					break
				}
			}
			if !allowed {
				h.log.Warn("mpesa callback from an address not on the list", slog.String("ip", ip.String()), slog.String("kind", kind))
				abort(c, http.StatusForbidden, "forbidden")
				return
			}
		}
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, payouts.MaxCallbackBytes))
		if err != nil || len(body) == 0 {
			abort(c, http.StatusBadRequest, "invalid_body")
			return
		}
		if _, err := h.svc.ReceiveCallback(c.Request.Context(), kind, body, &ip); err != nil {
			// Not stored: a 5xx makes Safaricom try again.
			h.log.Error("mpesa callback not stored", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
			abort(c, http.StatusInternalServerError, "internal_error")
			return
		}
		c.JSON(http.StatusOK, gin.H{"ResultCode": 0, "ResultDesc": "Accepted"})
	}
}

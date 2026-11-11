package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/earnings"
)

// Contributor earnings, statement and payouts (Stage 12a, brief 13). The
// caller only ever reads their own rows: there is no contributor ID in the
// path.
type earningsHandlers struct {
	svc *earnings.Service
	log *slog.Logger
}

func (h *earningsHandlers) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, earnings.ErrNotContributor):
		abort(c, http.StatusForbidden, "not_a_contributor")
	case errors.Is(err, earnings.ErrBadCursor):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "before", "message": err.Error()})
	default:
		h.log.Error("earnings request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
	}
}

func (h *earningsHandlers) summary(c *gin.Context) {
	out, err := h.svc.MySummary(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *earningsHandlers) statement(c *gin.Context) {
	out, err := h.svc.MyStatement(c.Request.Context(), PrincipalFrom(c), c.Query("before"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *earningsHandlers) payouts(c *gin.Context) {
	out, err := h.svc.MyPayouts(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"payouts": out})
}

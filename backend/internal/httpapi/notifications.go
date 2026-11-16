package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Email choices (Stage 16). Only optional emails can be switched off; the
// rest are listed so the person sees what they will always get (D-141).
type notificationPrefs struct {
	ProjectAvailable bool       `json:"project_available"`
	FeedbackReceived bool       `json:"feedback_received"`
	UpdatedAt        *time.Time `json:"updated_at"`
	Always           []string   `json:"always"`
}

var alwaysSent = []string{"account", "qualification_result", "payout_sent"}

type notificationHandlers struct {
	pool *db.Pool
	log  *slog.Logger
}

func (h *notificationHandlers) get(c *gin.Context) {
	p := PrincipalFrom(c)
	var out notificationPrefs
	err := h.pool.InScope(c.Request.Context(), p.Scope(), func(tx pgx.Tx) error {
		r, err := store.New(tx).GetNotificationPreferences(c.Request.Context(), p.UserID)
		out = notificationPrefs{ProjectAvailable: r.ProjectAvailable, FeedbackReceived: r.FeedbackReceived, UpdatedAt: r.UpdatedAt, Always: alwaysSent}
		return err
	})
	if err != nil {
		h.log.Error("notification preferences", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *notificationHandlers) put(c *gin.Context) {
	var req struct {
		ProjectAvailable *bool `json:"project_available"`
		FeedbackReceived *bool `json:"feedback_received"`
	}
	if !bind(c, &req) {
		return
	}
	if req.ProjectAvailable == nil || req.FeedbackReceived == nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": "project_available",
			"message": "send both project_available and feedback_received"})
		return
	}
	p := PrincipalFrom(c)
	ctx := c.Request.Context()
	var out notificationPrefs
	err := h.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		before, err := q.GetNotificationPreferences(ctx, p.UserID)
		if err != nil {
			return err
		}
		if err := q.SetNotificationPreferences(ctx, store.SetNotificationPreferencesParams{UserID: p.UserID,
			ProjectAvailable: *req.ProjectAvailable, FeedbackReceived: *req.FeedbackReceived}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, meta(c), auth.Entry{Actor: &p.UserID, Action: "notifications.updated", EntityType: "user", EntityID: &p.UserID,
			Before: map[string]any{"project_available": before.ProjectAvailable, "feedback_received": before.FeedbackReceived},
			After:  map[string]any{"project_available": *req.ProjectAvailable, "feedback_received": *req.FeedbackReceived}}); err != nil {
			return err
		}
		r, err := q.GetNotificationPreferences(ctx, p.UserID)
		out = notificationPrefs{ProjectAvailable: r.ProjectAvailable, FeedbackReceived: r.FeedbackReceived, UpdatedAt: r.UpdatedAt, Always: alwaysSent}
		return err
	})
	if err != nil {
		h.log.Error("notification preferences", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
		return
	}
	c.JSON(http.StatusOK, out)
}

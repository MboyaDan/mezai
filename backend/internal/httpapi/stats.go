package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// publicStat is a statistic the public site may show, always with its source.
type publicStat struct {
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	Source     string    `json:"source"`
	SourceURL  *string   `json:"source_url"`
	VerifiedAt time.Time `json:"verified_at"`
}

func listStats(pool *db.Pool, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		out := []publicStat{}
		err := pool.InScope(c.Request.Context(), db.System(), func(tx pgx.Tx) error {
			rows, err := store.New(tx).ListPublishedStats(c.Request.Context())
			for _, r := range rows {
				out = append(out, publicStat{Key: r.Key, Label: r.Label, Value: r.Value, Unit: r.Unit,
					Source: r.Source, SourceURL: r.SourceUrl, VerifiedAt: r.VerifiedAt})
			}
			return err
		})
		if err != nil {
			log.Error("stats", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
			abort(c, http.StatusInternalServerError, "internal_error")
			return
		}
		// Public and identical for everyone; the site caches it too.
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, gin.H{"stats": out})
	}
}

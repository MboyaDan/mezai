// Package httpapi wires the Gin router, middleware and handlers.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker is a dependency that /readyz must reach, such as the database or object storage.
type Checker interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Log    *slog.Logger
	Checks map[string]Checker
}

func init() { gin.SetMode(gin.ReleaseMode) }

// NewRouter builds the API router.
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(RequestID(), AccessLog(d.Log), Recover(d.Log))

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(d.Log, d.Checks))

	r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not_found"}) })
	r.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"}) })
	return r
}

// healthz reports that the process is up. It never touches dependencies.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyz checks every dependency with a short timeout. Failure details go to
// the log only; the response names the failing check without the error text.
func readyz(log *slog.Logger, checks map[string]Checker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		status, results := http.StatusOK, make(map[string]string, len(checks))
		for name, chk := range checks {
			if err := chk.Ping(ctx); err != nil {
				log.Warn("readiness check failed", slog.String("request_id", RequestIDFrom(c)), slog.String("check", name), slog.Any("error", err))
				results[name] = "unavailable"
				status = http.StatusServiceUnavailable
				continue
			}
			results[name] = "ok"
		}
		overall := "ok"
		if status != http.StatusOK {
			overall = "unavailable"
		}
		c.JSON(status, gin.H{"status": overall, "checks": results})
	}
}

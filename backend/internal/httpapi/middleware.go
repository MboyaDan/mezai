package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HeaderRequestID carries the request ID in both directions.
const HeaderRequestID = "X-Request-ID"

const ctxRequestID = "request_id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID reuses a well-formed incoming X-Request-ID or creates a new one,
// stores it on the context and echoes it in the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		c.Set(ctxRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// RequestIDFrom returns the request ID set by RequestID, or "".
func RequestIDFrom(c *gin.Context) string {
	return c.GetString(ctxRequestID)
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AccessLog writes one structured line per request. It logs the route
// template, never the raw query string, which may carry tokens.
func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		log.LogAttrs(c.Request.Context(), level, "http request",
			slog.String("request_id", RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	}
}

// Recover turns a panic into a 500 with a generic body and logs it.
func Recover(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		log.Error("panic recovered", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	})
}

// SecurityHeaders sets the response headers every API answer carries
// (Stage 17a, D-138). The API serves JSON and file downloads, never pages,
// so nothing may be framed, sniffed, or loaded from it: the policy allows
// nothing at all. Answers are not cached unless a handler says otherwise
// (public stats do). HSTS is sent when hsts is true (outside development):
// browsers then refuse plain HTTP for this host for a year.
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cache-Control", "no-store")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// MaxJSONBody is the most any request without a file may send. Handlers set
// smaller limits for their own bodies; uploads (multipart) set their own.
const MaxJSONBody = 1 << 20

// BodyLimit caps request bodies, so no route can be made to read an
// unbounded one because its handler forgot a limit.
func BodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
			if c.Request.ContentLength > MaxJSONBody {
				abort(c, http.StatusRequestEntityTooLarge, "body_too_large")
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBody)
		}
		c.Next()
	}
}

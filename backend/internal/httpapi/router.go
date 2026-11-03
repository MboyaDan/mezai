// Package httpapi wires the Gin router, middleware and handlers.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
)

// Checker is a dependency that /readyz must reach, such as the database or object storage.
type Checker interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Log    *slog.Logger
	Checks map[string]Checker
	// Auth enables the /v1 API. Nil serves only the health endpoints (unit tests).
	Auth *AuthDeps
}

// AuthDeps configures sign-in and the /v1 API.
type AuthDeps struct {
	Service        *auth.Service
	Pool           *db.Pool // for public, read-only data such as stats
	AppOrigin      string
	CookieSecure   bool
	TrustedProxies []string
}

// RouteRule is one /v1 route and the access rule it declared.
type RouteRule struct {
	Method, Path string
	Access       Access
}

// api registers /v1 routes. Every route must state its Access rule, so none
// can be added without an authorisation decision (AC-01).
type api struct {
	group *gin.RouterGroup
	rules []RouteRule
}

func (a *api) handle(method, path string, acc Access, h ...gin.HandlerFunc) {
	a.rules = append(a.rules, RouteRule{Method: method, Path: a.group.BasePath() + path, Access: acc})
	a.group.Handle(method, path, append([]gin.HandlerFunc{enforce(acc)}, h...)...)
}

func init() { gin.SetMode(gin.ReleaseMode) }

// NewRouter builds the API router.
func NewRouter(d Deps) *gin.Engine {
	r, _ := NewRouterWithRules(d)
	return r
}

// NewRouterWithRules also returns the access rule of every /v1 route, for tests.
func NewRouterWithRules(d Deps) (*gin.Engine, []RouteRule) {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(RequestID(), AccessLog(d.Log), Recover(d.Log))

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(d.Log, d.Checks))

	var rules []RouteRule
	if d.Auth != nil {
		// Only these proxies' X-Forwarded-For is believed (rate limits, audit IPs).
		if err := r.SetTrustedProxies(d.Auth.TrustedProxies); err != nil {
			d.Log.Error("invalid TRUSTED_PROXIES", slog.Any("error", err))
		}
		v1 := r.Group("/v1", loadSession(d.Auth.Service, d.Log), csrf(d.Auth.AppOrigin))
		a := &api{group: v1}
		h := &authHandlers{svc: d.Auth.Service, log: d.Log, cookieSecure: d.Auth.CookieSecure, perEmail: newLimiter(10, 15*time.Minute)}
		perIP := rateLimit(newLimiter(20, time.Minute))
		a.handle(http.MethodPost, "/auth/login", Public(), perIP, h.login)
		a.handle(http.MethodPost, "/auth/logout", SignedIn(), h.logout)
		a.handle(http.MethodGet, "/auth/me", SignedIn(), h.me)

		// Public account links share one per-IP limit; email-keyed ones add a per-address limit.
		ah := &accountHandlers{svc: d.Auth.Service, log: d.Log, perEmail: newLimiter(5, time.Hour)}
		perIPLinks := rateLimit(newLimiter(20, time.Minute))
		a.handle(http.MethodPost, "/auth/password-reset/request", Public(), perIPLinks, ah.requestReset)
		a.handle(http.MethodPost, "/auth/password-reset/confirm", Public(), perIPLinks, ah.confirmReset)
		a.handle(http.MethodPost, "/auth/verify-email", Public(), perIPLinks, ah.verifyEmail)
		a.handle(http.MethodPost, "/auth/verify-email/resend", Public(), perIPLinks, ah.resendVerification)
		a.handle(http.MethodPost, "/invitations/accept", Public(), perIPLinks, ah.acceptInvitation)

		invite := Perm("user.manage", "org.user.invite")
		a.handle(http.MethodGet, "/invitations", invite, ah.listInvitations)
		a.handle(http.MethodPost, "/invitations", invite, ah.invite)
		a.handle(http.MethodDelete, "/invitations/:id", invite, ah.revokeInvitation)

		a.handle(http.MethodGet, "/users", Perm("user.manage"), ah.listUsers)
		a.handle(http.MethodPost, "/users/:id/roles", Perm("user.manage"), ah.grantRole)
		a.handle(http.MethodDelete, "/users/:id/roles/:role", Perm("user.manage"), ah.removeRole)
		a.handle(http.MethodPost, "/users/:id/status", Perm("user.manage"), ah.setStatus)

		a.handle(http.MethodGet, "/roles", Perm("user.manage", "role.manage"), ah.listRoles)
		a.handle(http.MethodGet, "/permissions", Perm("role.manage"), ah.listPermissions)
		a.handle(http.MethodPut, "/roles/:key/permissions", Perm("role.manage"), ah.setRolePermissions)

		a.handle(http.MethodGet, "/stats", Public(), listStats(d.Auth.Pool, d.Log))
		rules = a.rules
	}

	r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not_found"}) })
	r.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"}) })
	return r, rules
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

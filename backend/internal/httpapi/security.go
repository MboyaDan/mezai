package httpapi

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
)

const (
	// SessionCookie holds the opaque session token (HttpOnly).
	SessionCookie = "mezai_session"
	// HeaderCSRF carries the CSRF token on state-changing requests.
	HeaderCSRF = "X-CSRF-Token"

	ctxPrincipal = "principal"
)

// Access is the rule a route declares: public, any signed-in user, or at
// least one of the listed permission keys.
type Access struct {
	Public      bool
	SignedIn    bool
	Permissions []string
}

func Public() Access   { return Access{Public: true} }
func SignedIn() Access { return Access{SignedIn: true} }

// Perm allows callers holding any of keys. Handlers may narrow further (for
// example a Client Admin acting only in their own organisation).
func Perm(keys ...string) Access { return Access{Permissions: keys} }

func (a Access) String() string {
	switch {
	case a.Public:
		return "public"
	case a.SignedIn:
		return "signed-in"
	}
	return strings.Join(a.Permissions, "|")
}

// Allows reports whether p passes the rule (nil p is anonymous).
func (a Access) Allows(p *auth.Principal) bool {
	if a.Public {
		return true
	}
	if p == nil {
		return false
	}
	if a.SignedIn {
		return true
	}
	for _, k := range a.Permissions {
		if p.Has(k) {
			return true
		}
	}
	return false
}

// PrincipalFrom returns the signed-in user for the request, or nil.
func PrincipalFrom(c *gin.Context) *auth.Principal {
	p, _ := c.Get(ctxPrincipal)
	pr, _ := p.(*auth.Principal)
	return pr
}

// loadSession resolves the session cookie when present. A missing or invalid
// cookie leaves the request anonymous; Access rules decide what that means.
func loadSession(svc *auth.Service, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookie)
		if err != nil || token == "" {
			c.Next()
			return
		}
		p, err := svc.Authenticate(c.Request.Context(), token)
		switch {
		case err == nil:
			c.Set(ctxPrincipal, p)
		case errors.Is(err, auth.ErrNoSession):
		default:
			log.Error("session lookup failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
			abort(c, http.StatusServiceUnavailable, "unavailable")
			return
		}
		c.Next()
	}
}

// csrf protects state-changing requests. Every one must come from the site's
// own origin (Origin and Sec-Fetch-Site checks); signed-in ones must also
// echo the session's CSRF token in X-CSRF-Token. SameSite=Lax on the cookie
// is a third layer.
func csrf(appOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			abort(c, http.StatusForbidden, "csrf_failed")
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" && origin != appOrigin {
			abort(c, http.StatusForbidden, "csrf_failed")
			return
		}
		if p := PrincipalFrom(c); p != nil {
			want := auth.CSRFToken(p.CSRFSecret)
			if subtle.ConstantTimeCompare([]byte(c.GetHeader(HeaderCSRF)), []byte(want)) != 1 {
				abort(c, http.StatusForbidden, "csrf_failed")
				return
			}
		}
		c.Next()
	}
}

// enforce applies a route's Access rule.
func enforce(a Access) gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.Public {
			c.Next()
			return
		}
		p := PrincipalFrom(c)
		if p == nil {
			abort(c, http.StatusUnauthorized, "unauthenticated")
			return
		}
		if !a.Allows(p) {
			abort(c, http.StatusForbidden, "forbidden")
			return
		}
		c.Next()
	}
}

// rateLimit refuses requests over the limit per client IP with 429.
func rateLimit(l *limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ok, wait := l.allow(c.ClientIP()); !ok {
			tooMany(c, wait)
			return
		}
		c.Next()
	}
}

func tooMany(c *gin.Context, wait time.Duration) {
	c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	abort(c, http.StatusTooManyRequests, "too_many_requests")
}

func abort(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}

// meta collects audit details for the request.
func meta(c *gin.Context) auth.Meta {
	m := auth.Meta{UserAgent: c.Request.UserAgent(), RequestID: RequestIDFrom(c)}
	if ip, err := netip.ParseAddr(c.ClientIP()); err == nil {
		m.IP = &ip
	}
	return m
}

package httpapi

import (
	"crypto/subtle"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// The site (Next.js proxy, frontend/proxy.ts) tells the API who the visitor
// is (D-156). On Render the API only ever sees its own platform proxy (::1),
// and behind Vercel the forwarded chain ends in Vercel's address, so neither
// X-Forwarded-For nor the peer address names the visitor. The site sends the
// visitor's address with a shared secret; anything else that sends these
// headers (a direct caller of the API) is ignored.
const (
	ClientIPHeader   = "X-Mezai-Client-Ip"
	ProxyTokenHeader = "X-Mezai-Proxy-Token"
)

// MinProxySecretLen is the shortest PROXY_SHARED_SECRET accepted.
const MinProxySecretLen = 32

// proxyClientIP keeps ClientIPHeader only when the request carries the shared
// secret and a valid address. It always removes the secret, so no handler or
// log can see it. With no secret configured, the header is always removed.
func proxyClientIP(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Request.Header
		ip, tok := strings.TrimSpace(h.Get(ClientIPHeader)), h.Get(ProxyTokenHeader)
		h.Del(ClientIPHeader)
		h.Del(ProxyTokenHeader)
		if secret != "" && ip != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(secret)) == 1 {
			if a, err := netip.ParseAddr(ip); err == nil {
				h.Set(ClientIPHeader, a.Unmap().String())
			}
		}
		c.Next()
	}
}

// useProxyClientIP makes c.ClientIP() return the address the site vouched
// for, falling back to the usual rules (TRUSTED_PROXIES) when there is none.
func useProxyClientIP(r *gin.Engine, secret string) {
	r.TrustedPlatform = ClientIPHeader
	r.Use(proxyClientIP(secret))
}

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const testProxySecret = "test-proxy-secret-0123456789abcdef0123"

// echoRouter is the real router (proxy check first, as in production) with a
// route that reports what handlers see.
func echoRouter(secret string, trusted []string) *gin.Engine {
	r := NewRouter(Deps{ProxySecret: secret})
	if err := r.SetTrustedProxies(trusted); err != nil {
		panic(err)
	}
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP()+"|"+c.GetHeader(ProxyTokenHeader))
	})
	return r
}

func TestClientIPFromSite(t *testing.T) {
	cases := []struct {
		name    string
		secret  string
		headers map[string]string
		want    string
	}{
		{"site with the secret", testProxySecret,
			map[string]string{ClientIPHeader: "197.248.10.20", ProxyTokenHeader: testProxySecret}, "197.248.10.20|"},
		{"IPv6 visitor", testProxySecret,
			map[string]string{ClientIPHeader: "2c0f:fe38::1", ProxyTokenHeader: testProxySecret}, "2c0f:fe38::1|"},
		{"IPv4-mapped address is unmapped", testProxySecret,
			map[string]string{ClientIPHeader: "::ffff:197.248.10.20", ProxyTokenHeader: testProxySecret}, "197.248.10.20|"},
		{"wrong secret is ignored", testProxySecret,
			map[string]string{ClientIPHeader: "203.0.113.9", ProxyTokenHeader: testProxySecret + "x"}, "192.0.2.1|"},
		{"no secret sent is ignored", testProxySecret,
			map[string]string{ClientIPHeader: "203.0.113.9"}, "192.0.2.1|"},
		{"not an address is ignored", testProxySecret,
			map[string]string{ClientIPHeader: "203.0.113.9, 10.0.0.1", ProxyTokenHeader: testProxySecret}, "192.0.2.1|"},
		{"no secret configured: header never believed", "",
			map[string]string{ClientIPHeader: "203.0.113.9", ProxyTokenHeader: ""}, "192.0.2.1|"},
		{"forged X-Forwarded-For from an untrusted peer is ignored", testProxySecret,
			map[string]string{"X-Forwarded-For": "203.0.113.9"}, "192.0.2.1|"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := echoRouter(tc.secret, []string{"127.0.0.1"})
			req := httptest.NewRequest(http.MethodGet, "/echo", nil) // peer 192.0.2.1
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if got := w.Body.String(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The secret never reaches a handler, so nothing can log or echo it.
func TestProxySecretNeverReachesHandlers(t *testing.T) {
	r := echoRouter(testProxySecret, nil)
	req := httptest.NewRequest(http.MethodGet, "/echo", nil)
	req.Header.Set(ClientIPHeader, "197.248.10.20")
	req.Header.Set(ProxyTokenHeader, testProxySecret)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), testProxySecret) {
		t.Fatal("handler saw the proxy secret")
	}
}

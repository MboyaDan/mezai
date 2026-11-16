package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeCheck struct{ err error }

func (f fakeCheck) Ping(context.Context) error { return f.err }

func do(t *testing.T, d Deps, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	if d.Log == nil {
		d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(w, req)
	return w
}

func TestHealthz(t *testing.T) {
	w := do(t, Deps{}, http.MethodGet, "/healthz", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}

func TestReadyz(t *testing.T) {
	ok := Deps{Checks: map[string]Checker{"database": fakeCheck{}, "storage": fakeCheck{}}}
	if w := do(t, ok, http.MethodGet, "/readyz", nil); w.Code != http.StatusOK {
		t.Fatalf("all ok: got %d %s", w.Code, w.Body)
	}

	bad := Deps{Checks: map[string]Checker{"database": fakeCheck{}, "storage": fakeCheck{errors.New("secret detail")}}}
	w := do(t, bad, http.MethodGet, "/readyz", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("one failing: got %d", w.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "unavailable" || body.Checks["storage"] != "unavailable" || body.Checks["database"] != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if strings.Contains(w.Body.String(), "secret detail") {
		t.Fatal("error detail leaked into response")
	}
}

func TestRequestID(t *testing.T) {
	w := do(t, Deps{}, http.MethodGet, "/healthz", nil)
	if got := w.Header().Get(HeaderRequestID); len(got) != 32 {
		t.Fatalf("expected generated 32-char id, got %q", got)
	}
	w = do(t, Deps{}, http.MethodGet, "/healthz", map[string]string{HeaderRequestID: "client-abc-123"})
	if got := w.Header().Get(HeaderRequestID); got != "client-abc-123" {
		t.Fatalf("valid id should be reused, got %q", got)
	}
	w = do(t, Deps{}, http.MethodGet, "/healthz", map[string]string{HeaderRequestID: "bad id\nwith newline"})
	if got := w.Header().Get(HeaderRequestID); got == "bad id\nwith newline" || len(got) != 32 {
		t.Fatalf("malformed id should be replaced, got %q", got)
	}
}

func TestAccessLogOmitsQuery(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	do(t, Deps{Log: log}, http.MethodGet, "/healthz?token=supersecret", nil)
	out := buf.String()
	if strings.Contains(out, "supersecret") {
		t.Fatalf("query string leaked into log: %s", out)
	}
	for _, want := range []string{`"route":"/healthz"`, `"status":200`, `"request_id":"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s: %s", want, out)
		}
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	if w := do(t, Deps{}, http.MethodGet, "/nope", nil); w.Code != http.StatusNotFound {
		t.Fatalf("got %d", w.Code)
	}
	if w := do(t, Deps{}, http.MethodPost, "/healthz", nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	for _, path := range []string{"/healthz", "/no-such-route"} {
		w := do(t, Deps{}, http.MethodGet, path, nil)
		h := w.Header()
		for k, want := range map[string]string{
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
			"Referrer-Policy":              "no-referrer",
			"Cache-Control":                "no-store",
			"Cross-Origin-Resource-Policy": "same-origin",
		} {
			if h.Get(k) != want {
				t.Errorf("%s %s: %q, want %q", path, k, h.Get(k), want)
			}
		}
		if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s CSP: %q", path, csp)
		}
		if h.Get("Strict-Transport-Security") != "" {
			t.Errorf("%s: HSTS sent although off", path)
		}
	}
	if w := do(t, Deps{HSTS: true}, http.MethodGet, "/healthz", nil); !strings.HasPrefix(w.Header().Get("Strict-Transport-Security"), "max-age=31536000") {
		t.Fatalf("HSTS: %q", w.Header().Get("Strict-Transport-Security"))
	}
}

func TestBodyLimit(t *testing.T) {
	r := gin.New()
	r.Use(BodyLimit())
	r.POST("/echo", func(c *gin.Context) {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.String(http.StatusOK, "%d", len(b))
	})
	post := func(n int, contentType string, declare bool) int {
		req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(bytes.Repeat([]byte("a"), n)))
		req.Header.Set("Content-Type", contentType)
		if !declare {
			req.ContentLength = -1 // chunked: the reader must stop it
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := post(MaxJSONBody, "application/json", true); code != http.StatusOK {
		t.Fatalf("at the limit: %d", code)
	}
	if code := post(MaxJSONBody+1, "application/json", true); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared over the limit: %d", code)
	}
	if code := post(MaxJSONBody+1, "application/json", false); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("undeclared over the limit: %d", code)
	}
	if code := post(MaxJSONBody+1, "multipart/form-data; boundary=x", true); code != http.StatusOK {
		t.Fatalf("uploads keep their own limits: %d", code)
	}
}

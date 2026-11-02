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

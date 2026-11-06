package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnquiryValidateUnit(t *testing.T) {
	r := enquiryRequest{Name: "  Sample  ", Organisation: "Lab", Email: " A@Sample.Invalid ", ProjectType: "not-sure", Message: "a\r\nb"}
	if errs := r.validate(); len(errs) != 0 {
		t.Fatalf("valid request flagged: %v", errs)
	}
	if r.Name != "Sample" || r.Email != "a@sample.invalid" || r.Message != "a\nb" {
		t.Fatalf("not normalised: %+v", r)
	}
	bad := enquiryRequest{Name: strings.Repeat("n", 121), Organisation: "x", Email: "a@b.c", ProjectType: "ai-evaluation",
		Timeline: "soon", Languages: "Kiswahili\x00", Message: "ok\x07"}
	errs := bad.validate()
	for k, v := range map[string]string{"name": "too_long", "timeline": "invalid", "languages": "invalid", "message": "invalid"} {
		if errs[k] != v {
			t.Errorf("%s: want %s, got %q", k, v, errs[k])
		}
	}
}

func TestTurnstileVerifyUnit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("remoteip") != "192.0.2.1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": r.Form.Get("response") == "ok"})
	}))
	defer srv.Close()
	v := Turnstile{Secret: "s", Endpoint: srv.URL}
	ctx := context.Background()
	if ok, err := v.Verify(ctx, "ok", "192.0.2.1"); !ok || err != nil {
		t.Fatalf("good token: %v %v", ok, err)
	}
	if ok, err := v.Verify(ctx, "nope", "192.0.2.1"); ok || err != nil {
		t.Fatalf("bad token: %v %v", ok, err)
	}
	if ok, _ := v.Verify(ctx, "", "192.0.2.1"); ok {
		t.Fatal("empty token never passes")
	}
	if _, err := v.Verify(ctx, "ok", "198.51.100.9"); err == nil {
		t.Fatal("a non-200 answer is an error, not a pass")
	}
}

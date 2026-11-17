package pii

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	for in, want := range map[string]string{
		"550 5.1.1 <jane.doe@example.co.ke>: Recipient rejected": "550 5.1.1 <•••@example.co.ke>: Recipient rejected",
		"paid +254712345678 and 254112345678":                    "paid •••678 and •••678",
		"call 0712345678 now":                                    "call •••678 now",
		"order 12345678901 and id 3f2a-0712345678x":              "order 12345678901 and id 3f2a-0712345678x",
		"request_id 01J8ZK4Q status 500":                         "request_id 01J8ZK4Q status 500",
	} {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactHandler(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewRedactHandler(slog.NewJSONHandler(&buf, nil))).With("who", "a@b.example")
	log.Error("send to x@y.example failed",
		slog.Any("error", errors.New("smtp: 550 <jane@example.org> unknown; phone 0712345678")),
		slog.Group("g", slog.String("phone", "+254712345678")),
		slog.Int("attempt", 2))
	out := buf.String()
	for _, leak := range []string{"jane@", "x@y", "a@b", "0712345678", "+254712345678"} {
		if strings.Contains(out, leak) {
			t.Errorf("log holds %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, `"attempt":2`) || !strings.Contains(out, "•••678") {
		t.Errorf("log lost data: %s", out)
	}
}

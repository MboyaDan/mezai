package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := env[k]; return v, ok }
}

func TestDefaults(t *testing.T) {
	c, err := FromLookup(lookup(map[string]string{
		"DATABASE_URL": "postgres://x",
		"S3_BUCKET":    "b",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Env != EnvDevelopment || c.HTTPAddr != ":8080" || c.LogLevel != slog.LevelInfo || c.ShutdownTimeout != 15*time.Second || !c.S3UsePathStyle {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if !c.IsDevLike() {
		t.Fatal("development should be dev-like")
	}
}

func TestRejectsBadValues(t *testing.T) {
	_, err := FromLookup(lookup(map[string]string{
		"APP_ENV":          "prod",
		"LOG_LEVEL":        "loud",
		"LOG_FORMAT":       "xml",
		"SHUTDOWN_TIMEOUT": "soon",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"APP_ENV", "LOG_LEVEL", "LOG_FORMAT", "SHUTDOWN_TIMEOUT", "DATABASE_URL", "S3_BUCKET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s: %v", want, err)
		}
	}
}

func TestProductionNeedsStorageKeys(t *testing.T) {
	_, err := FromLookup(lookup(map[string]string{
		"APP_ENV":      EnvProduction,
		"DATABASE_URL": "postgres://x",
		"S3_BUCKET":    "b",
	}))
	if err == nil || !strings.Contains(err.Error(), "S3_ACCESS_KEY_ID") {
		t.Fatalf("expected storage key error, got %v", err)
	}
}

func TestEnquiryRecipient(t *testing.T) {
	c, err := FromLookup(lookup(map[string]string{"DATABASE_URL": "postgres://x", "S3_BUCKET": "b"}))
	if err != nil || c.EnquiryNotifyTo != "ops@mezai.invalid" || c.TurnstileSecret != "" {
		t.Fatalf("development defaults: %q %q %v", c.EnquiryNotifyTo, c.TurnstileSecret, err)
	}
	_, err = FromLookup(lookup(map[string]string{"APP_ENV": EnvStaging, "DATABASE_URL": "postgres://x", "S3_BUCKET": "b"}))
	if err == nil || !strings.Contains(err.Error(), "ENQUIRY_NOTIFY_TO") {
		t.Fatalf("staging must name an enquiry recipient, got %v", err)
	}
}

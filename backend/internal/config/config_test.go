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

func TestIdentityKey(t *testing.T) {
	c, err := FromLookup(lookup(map[string]string{"DATABASE_URL": "postgres://x", "S3_BUCKET": "b"}))
	if err != nil || len(c.IdentityKey) < 32 {
		t.Fatalf("development default key: %d bytes, %v", len(c.IdentityKey), err)
	}
	_, err = FromLookup(lookup(map[string]string{"APP_ENV": EnvProduction, "DATABASE_URL": "postgres://x", "S3_BUCKET": "b"}))
	if err == nil || !strings.Contains(err.Error(), "IDENTITY_HMAC_KEY is required") {
		t.Fatalf("production without a key: %v", err)
	}
	_, err = FromLookup(lookup(map[string]string{"DATABASE_URL": "postgres://x", "S3_BUCKET": "b", "IDENTITY_HMAC_KEY": "c2hvcnQ="}))
	if err == nil || !strings.Contains(err.Error(), "IDENTITY_HMAC_KEY") {
		t.Fatalf("short key: %v", err)
	}
	c, err = FromLookup(lookup(map[string]string{"DATABASE_URL": "postgres://x", "S3_BUCKET": "b",
		"IDENTITY_HMAC_KEY": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="}))
	if err != nil || len(c.IdentityKey) != 32 {
		t.Fatalf("valid key: %v", err)
	}
}

func TestMpesaModes(t *testing.T) {
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{"DATABASE_URL": "postgres://x", "S3_BUCKET": "b"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	full := map[string]string{
		"MPESA_CONSUMER_KEY": "k", "MPESA_CONSUMER_SECRET": "s", "MPESA_SHORTCODE": "600000",
		"MPESA_INITIATOR_NAME": "i", "MPESA_SECURITY_CREDENTIAL": "c",
		"MPESA_CALLBACK_BASE_URL": "https://api.example.invalid", "MPESA_CALLBACK_TOKEN": strings.Repeat("t", 32),
	}

	c, err := FromLookup(lookup(base(nil)))
	if err != nil || c.Mpesa.Enabled() {
		t.Fatalf("default must be off: %v %+v", err, c.Mpesa)
	}
	// Keys alone (no MPESA_ENV) keep it off.
	if c, err := FromLookup(lookup(base(map[string]string{"MPESA_CONSUMER_KEY": "k"}))); err != nil || c.Mpesa.Enabled() {
		t.Fatalf("keys without MPESA_ENV: %v", err)
	}
	c, err = FromLookup(lookup(base(map[string]string{"MPESA_ENV": "fake"})))
	if err != nil || !c.Mpesa.Enabled() || c.Mpesa.CallbackToken != DevCallbackToken {
		t.Fatalf("fake in development: %v %+v", err, c.Mpesa.Env)
	}

	sandbox := base(full)
	sandbox["MPESA_ENV"] = "sandbox"
	sandbox["MPESA_CALLBACK_IPS"] = "196.201.214.200, 196.201.212.0/24"
	c, err = FromLookup(lookup(sandbox))
	if err != nil || c.Mpesa.BaseURL != "https://sandbox.safaricom.co.ke" || len(c.Mpesa.CallbackIPs) != 2 {
		t.Fatalf("sandbox: %v %+v", err, c.Mpesa)
	}

	refused := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"fake in staging", map[string]string{"APP_ENV": "staging", "MPESA_ENV": "fake"}, "development and test only"},
		{"sandbox missing keys", map[string]string{"MPESA_ENV": "sandbox"}, "MPESA_INITIATOR_NAME"},
		{"production outside production", func() map[string]string { m := base(full); m["MPESA_ENV"] = "production"; return m }(), "needs APP_ENV=production"},
		{"sandbox in production", func() map[string]string {
			m := base(full)
			m["MPESA_ENV"] = "sandbox"
			m["APP_ENV"] = "production"
			return m
		}(), "cannot run with APP_ENV=production"},
		{"plain http callbacks", func() map[string]string {
			m := base(full)
			m["MPESA_ENV"] = "sandbox"
			m["MPESA_CALLBACK_BASE_URL"] = "http://x"
			return m
		}(), "https://"},
		{"short token", func() map[string]string {
			m := base(full)
			m["MPESA_ENV"] = "sandbox"
			m["MPESA_CALLBACK_TOKEN"] = "short"
			return m
		}(), "32 characters"},
		{"bad ip", map[string]string{"MPESA_CALLBACK_IPS": "nope"}, "MPESA_CALLBACK_IPS"},
		{"unknown mode", map[string]string{"MPESA_ENV": "live"}, "MPESA_ENV must be"},
	}
	for _, r := range refused {
		t.Run(r.name, func(t *testing.T) {
			_, err := FromLookup(lookup(base(r.env)))
			if err == nil || !strings.Contains(err.Error(), r.want) {
				t.Fatalf("want error with %q, got %v", r.want, err)
			}
		})
	}
}

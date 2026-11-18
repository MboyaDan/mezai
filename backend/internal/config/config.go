// Package config loads API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/identity"
)

// Environment names accepted in APP_ENV.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

type Config struct {
	Env             string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	LogLevel        slog.Level
	LogFormat       string // "json" or "text"

	DatabaseURL string

	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool

	// AppOrigin is the browser-facing origin (the Next.js site). State-changing
	// requests from any other origin are refused.
	AppOrigin string
	// CookieSecure sets the Secure flag on the session cookie. Keep true;
	// browsers accept Secure cookies on http://localhost.
	CookieSecure bool
	// TrustedProxies are the proxy addresses whose X-Forwarded-For is believed
	// when finding the client IP (rate limits, audit).
	TrustedProxies []string
	// SessionTTL is the absolute session lifetime; SessionIdle signs a session
	// out after this long without a request.
	SessionTTL  time.Duration
	SessionIdle time.Duration

	// Outgoing email: mailpit locally, a provider in staging (Stage 16/19).
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string
	MailFrom     string

	// EnquiryNotifyTo receives a copy of every project enquiry from the
	// public site. Required outside development and test.
	EnquiryNotifyTo string
	// ProxySecret (PROXY_SHARED_SECRET) is shared with the site, which sends
	// the visitor's address with it (D-156). Required outside development and
	// test, at least 32 characters; the site must hold the same value.
	ProxySecret string

	// TurnstileSecret turns on Cloudflare Turnstile checks on public forms
	// (DG-07). Empty leaves them off; the honeypot and rate limits still apply.
	TurnstileSecret string

	// IdentityKey keys the National ID fingerprints (D-078), from
	// IDENTITY_HMAC_KEY as base64 of at least 32 random bytes. Required
	// outside development and test. Changing it breaks duplicate detection
	// for every number already stored, so it is never rotated casually.
	IdentityKey []byte

	// Mpesa configures automated payouts through Daraja B2C (Stage 15,
	// DG-02). Off by default; the manual bulk file path always works.
	Mpesa Mpesa
}

// M-Pesa B2C modes (MPESA_ENV).
const (
	MpesaOff        = "off"
	MpesaFake       = "fake"       // in-process fake Daraja: development and test only
	MpesaSandbox    = "sandbox"    // Safaricom's sandbox: never with APP_ENV=production
	MpesaProduction = "production" // real money: only with APP_ENV=production and DG-02 cleared
)

// Mpesa is the B2C configuration. Secrets are never logged.
type Mpesa struct {
	Env                string
	BaseURL            string
	ConsumerKey        string
	ConsumerSecret     string
	ShortCode          string
	InitiatorName      string
	SecurityCredential string
	// CallbackBaseURL is the API's public origin Daraja posts results to.
	CallbackBaseURL string
	// CallbackToken is a secret path segment in the callback URLs.
	CallbackToken string
	// CallbackIPs, when set, are the only source addresses accepted.
	CallbackIPs []netip.Prefix
}

// Enabled reports whether B2C payouts are switched on.
func (m Mpesa) Enabled() bool { return m.Env != "" && m.Env != MpesaOff }

// DevCallbackToken is the callback token used with the fake in development.
const DevCallbackToken = "dev-only-mpesa-callback-token-0000000000"

// IsDevLike reports whether the environment allows development-only tools such as the seed command.
func (c Config) IsDevLike() bool {
	return c.Env == EnvDevelopment || c.Env == EnvTest
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return FromLookup(os.LookupEnv)
}

// FromLookup reads configuration through lookup, so tests can supply their own environment.
func FromLookup(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return def
	}

	var errs []error
	c := Config{
		Env:               get("APP_ENV", EnvDevelopment),
		HTTPAddr:          get("HTTP_ADDR", ":8080"),
		LogFormat:         get("LOG_FORMAT", "json"),
		DatabaseURL:       get("DATABASE_URL", ""),
		S3Endpoint:        get("S3_ENDPOINT", ""),
		S3Region:          get("S3_REGION", "us-east-1"),
		S3Bucket:          get("S3_BUCKET", ""),
		S3AccessKeyID:     get("S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey: get("S3_SECRET_ACCESS_KEY", ""),
		AppOrigin:         strings.TrimRight(get("APP_ORIGIN", "http://localhost:3000"), "/"),
		SMTPAddr:          get("SMTP_ADDR", "localhost:1025"),
		SMTPUsername:      get("SMTP_USERNAME", ""),
		SMTPPassword:      get("SMTP_PASSWORD", ""),
		MailFrom:          get("MAIL_FROM", ""),
		EnquiryNotifyTo:   get("ENQUIRY_NOTIFY_TO", ""),
		TurnstileSecret:   get("TURNSTILE_SECRET_KEY", ""),
		ProxySecret:       get("PROXY_SHARED_SECRET", ""),
	}
	if c.MailFrom == "" && c.IsDevLike() {
		c.MailFrom = "MezAI <no-reply@mezai.invalid>"
	}
	if c.EnquiryNotifyTo == "" && c.IsDevLike() {
		c.EnquiryNotifyTo = "ops@mezai.invalid"
	}
	for _, p := range strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,::1"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test, staging or production, got %q", c.Env))
	}

	if err := c.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be json or text, got %q", c.LogFormat))
	}

	d, err := time.ParseDuration(get("SHUTDOWN_TIMEOUT", "15s"))
	if err != nil {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
	}
	c.ShutdownTimeout = d

	c.S3UsePathStyle, err = strconv.ParseBool(get("S3_USE_PATH_STYLE", "true"))
	if err != nil {
		errs = append(errs, fmt.Errorf("S3_USE_PATH_STYLE: %w", err))
	}
	c.CookieSecure, err = strconv.ParseBool(get("COOKIE_SECURE", "true"))
	if err != nil {
		errs = append(errs, fmt.Errorf("COOKIE_SECURE: %w", err))
	}
	if !c.IsDevLike() && !c.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE must be true outside development and test"))
	}
	if c.SessionTTL, err = time.ParseDuration(get("SESSION_TTL", "168h")); err != nil || c.SessionTTL <= 0 {
		errs = append(errs, fmt.Errorf("SESSION_TTL must be a positive duration"))
	}
	if c.SessionIdle, err = time.ParseDuration(get("SESSION_IDLE", "48h")); err != nil || c.SessionIdle <= 0 || c.SessionIdle > c.SessionTTL {
		errs = append(errs, fmt.Errorf("SESSION_IDLE must be a positive duration no longer than SESSION_TTL"))
	}
	if !strings.HasPrefix(c.AppOrigin, "http://") && !strings.HasPrefix(c.AppOrigin, "https://") {
		errs = append(errs, fmt.Errorf("APP_ORIGIN must start with http:// or https://, got %q", c.AppOrigin))
	}

	if _, err := mail.ParseAddress(c.EnquiryNotifyTo); err != nil {
		errs = append(errs, errors.New("ENQUIRY_NOTIFY_TO must be an email address (required outside development and test)"))
	}
	if _, err := mail.ParseAddress(c.MailFrom); err != nil {
		errs = append(errs, fmt.Errorf("MAIL_FROM must be an address such as \"MezAI <no-reply@example.com>\" (required outside development and test)"))
	}
	if k := get("IDENTITY_HMAC_KEY", ""); k != "" {
		if c.IdentityKey, err = identity.ParseKey(k); err != nil {
			errs = append(errs, fmt.Errorf("IDENTITY_HMAC_KEY: %w", err))
		}
	} else if c.IsDevLike() {
		c.IdentityKey = identity.DevKey
	} else {
		errs = append(errs, errors.New("IDENTITY_HMAC_KEY is required outside development and test (base64 of 32 random bytes: openssl rand -base64 32)"))
	}
	c.Mpesa, err = loadMpesa(get, c.Env)
	if err != nil {
		errs = append(errs, err)
	}
	if c.ProxySecret != "" && len(c.ProxySecret) < 32 {
		errs = append(errs, errors.New("PROXY_SHARED_SECRET must be at least 32 characters (openssl rand -hex 32)"))
	} else if c.ProxySecret == "" && !c.IsDevLike() {
		errs = append(errs, errors.New("PROXY_SHARED_SECRET is required outside development and test (openssl rand -hex 32; the site needs the same value)"))
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.S3Bucket == "" {
		errs = append(errs, errors.New("S3_BUCKET is required"))
	}
	if !c.IsDevLike() && (c.S3AccessKeyID == "" || c.S3SecretAccessKey == "") {
		errs = append(errs, errors.New("S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required outside development and test"))
	}

	return c, errors.Join(errs...)
}

func loadMpesa(get func(string, string) string, appEnv string) (Mpesa, error) {
	m := Mpesa{
		Env:                strings.ToLower(get("MPESA_ENV", MpesaOff)),
		BaseURL:            strings.TrimRight(get("MPESA_BASE_URL", ""), "/"),
		ConsumerKey:        get("MPESA_CONSUMER_KEY", ""),
		ConsumerSecret:     get("MPESA_CONSUMER_SECRET", ""),
		ShortCode:          get("MPESA_SHORTCODE", ""),
		InitiatorName:      get("MPESA_INITIATOR_NAME", ""),
		SecurityCredential: get("MPESA_SECURITY_CREDENTIAL", ""),
		CallbackBaseURL:    strings.TrimRight(get("MPESA_CALLBACK_BASE_URL", ""), "/"),
		CallbackToken:      get("MPESA_CALLBACK_TOKEN", ""),
	}
	var errs []error
	for _, p := range strings.Split(get("MPESA_CALLBACK_IPS", ""), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if !strings.Contains(p, "/") {
			if a, err := netip.ParseAddr(p); err == nil {
				p = netip.PrefixFrom(a, a.BitLen()).String()
			}
		}
		pr, err := netip.ParsePrefix(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("MPESA_CALLBACK_IPS: %q is not an address or CIDR range", p))
			continue
		}
		m.CallbackIPs = append(m.CallbackIPs, pr.Masked())
	}
	devLike := appEnv == EnvDevelopment || appEnv == EnvTest
	switch m.Env {
	case MpesaOff:
		return m, errors.Join(errs...)
	case MpesaFake:
		if !devLike {
			errs = append(errs, errors.New("MPESA_ENV=fake is for development and test only"))
		}
		if m.CallbackBaseURL == "" {
			m.CallbackBaseURL = "http://localhost:8080"
		}
		if m.CallbackToken == "" {
			m.CallbackToken = DevCallbackToken
		}
		return m, errors.Join(errs...)
	case MpesaSandbox:
		if appEnv == EnvProduction {
			errs = append(errs, errors.New("MPESA_ENV=sandbox cannot run with APP_ENV=production"))
		}
		if m.BaseURL == "" {
			m.BaseURL = "https://sandbox.safaricom.co.ke"
		}
	case MpesaProduction:
		if appEnv != EnvProduction {
			errs = append(errs, errors.New("MPESA_ENV=production needs APP_ENV=production"))
		}
		if m.BaseURL == "" {
			m.BaseURL = "https://api.safaricom.co.ke"
		}
	default:
		return m, fmt.Errorf("MPESA_ENV must be off, fake, sandbox or production, got %q", m.Env)
	}
	var missing []string
	for _, f := range []struct{ name, v string }{
		{"MPESA_CONSUMER_KEY", m.ConsumerKey}, {"MPESA_CONSUMER_SECRET", m.ConsumerSecret}, {"MPESA_SHORTCODE", m.ShortCode},
		{"MPESA_INITIATOR_NAME", m.InitiatorName}, {"MPESA_SECURITY_CREDENTIAL", m.SecurityCredential},
		{"MPESA_CALLBACK_BASE_URL", m.CallbackBaseURL}, {"MPESA_CALLBACK_TOKEN", m.CallbackToken},
	} {
		if f.v == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("MPESA_ENV=%s needs %s", m.Env, strings.Join(missing, ", ")))
	}
	if m.CallbackBaseURL != "" && !strings.HasPrefix(m.CallbackBaseURL, "https://") {
		errs = append(errs, errors.New("MPESA_CALLBACK_BASE_URL must be https:// (Daraja only calls public HTTPS URLs)"))
	}
	if m.CallbackToken != "" && len(m.CallbackToken) < 32 {
		errs = append(errs, errors.New("MPESA_CALLBACK_TOKEN must be at least 32 characters (openssl rand -hex 24)"))
	}
	return m, errors.Join(errs...)
}

// Package config loads API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
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
}

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

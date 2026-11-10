// Command api runs the MezAI HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/config"
	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "GET /healthz on the local server and exit 0 if healthy (for container health checks)")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}
	if err := run(); err != nil {
		slog.Error("api stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := storage.New(storage.Options{
		Endpoint:        cfg.S3Endpoint,
		Region:          cfg.S3Region,
		Bucket:          cfg.S3Bucket,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		UsePathStyle:    cfg.S3UsePathStyle,
	})

	opts := auth.DefaultOptions(cfg.SessionTTL, cfg.SessionIdle)
	opts.AppOrigin = cfg.AppOrigin
	authSvc, err := auth.NewService(pool, opts)
	if err != nil {
		return err
	}

	// Email outbox worker. Every API instance runs one; SKIP LOCKED keeps them apart.
	mailer := &mail.Worker{Pool: pool, Log: log, Sender: mail.SMTP{
		Addr: cfg.SMTPAddr, From: cfg.MailFrom, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
	}}
	go mailer.Run(ctx, 5*time.Second)
	// CVs are deleted 90 days after the screening decision (D-100).
	go contributors.NewService(pool, cfg.IdentityKey, store).RunCVSweep(ctx, time.Hour, log)
	// Lapsed task reservations are marked expired in the background (D-105).
	go tasks.NewService(pool).RunExpirySweep(ctx, time.Minute, log)
	// Nightly quality jobs (agreement snapshots, gold feedback) from the
	// Postgres job queue; safe with several instances (Stage 10b).
	qualityJobs := tasks.NewService(pool)
	go (&jobs.Runner{Pool: pool, Log: log, Handlers: qualityJobs.Handlers(), Daily: tasks.DailyJobs}).Run(ctx, 5*time.Minute)

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Log:    log,
			Checks: map[string]httpapi.Checker{"database": pool, "storage": store},
			Auth: &httpapi.AuthDeps{
				Service: authSvc, Pool: pool, AppOrigin: cfg.AppOrigin, CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies,
				Enquiry:     enquiryDeps(cfg),
				Files:       store,
				IdentityKey: cfg.IdentityKey,
			},
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Env))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// probe lets the distroless image (no shell, no curl) run its own health check.
func probe() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// enquiryDeps turns on Turnstile only when its secret is set (DG-07).
func enquiryDeps(cfg config.Config) httpapi.EnquiryDeps {
	d := httpapi.EnquiryDeps{NotifyTo: cfg.EnquiryNotifyTo}
	if cfg.TurnstileSecret != "" {
		d.Turnstile = httpapi.Turnstile{Secret: cfg.TurnstileSecret}
	}
	return d
}

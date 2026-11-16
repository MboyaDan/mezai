// Command api runs the MezAI HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
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
	"github.com/MboyaDan/mezai/backend/internal/mpesa"
	"github.com/MboyaDan/mezai/backend/internal/payouts"
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
	handlers := qualityJobs.Handlers()
	// M-Pesa B2C (Stage 15, DG-02): sends, status queries and callback
	// processing run from the same queue. Off unless MPESA_ENV is set.
	mpesaDeps, err := mpesaSetup(ctx, cfg, log)
	if err != nil {
		return err
	}
	if mpesaDeps != nil {
		b2c := payouts.NewService(pool, store, log).WithB2C(&payouts.B2C{Client: mpesaDeps.Client, Environment: mpesaDeps.Environment})
		for k, h := range b2c.Handlers() {
			handlers[k] = h
		}
		go b2c.RunCallbackSweep(ctx, 5*time.Minute)
		log.Info("mpesa b2c on", slog.String("environment", mpesaDeps.Environment))
	}
	go (&jobs.Runner{Pool: pool, Log: log, Handlers: handlers, Daily: tasks.DailyJobs}).Run(ctx, 30*time.Second)

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Log:    log,
			HSTS:   !cfg.IsDevLike(),
			Checks: map[string]httpapi.Checker{"database": pool, "storage": store},
			Auth: &httpapi.AuthDeps{
				Service: authSvc, Pool: pool, AppOrigin: cfg.AppOrigin, CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies,
				Enquiry:     enquiryDeps(cfg),
				Files:       store,
				IdentityKey: cfg.IdentityKey,
				Mpesa:       mpesaDeps,
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

// mpesaSetup builds the Daraja client from MPESA_ENV. In fake mode a fake
// Daraja listens on a loopback port of this process and posts its results
// back to the API, so the whole callback path runs locally (A-078).
func mpesaSetup(ctx context.Context, cfg config.Config, log *slog.Logger) (*httpapi.MpesaDeps, error) {
	m := cfg.Mpesa
	if !m.Enabled() {
		return nil, nil
	}
	dc := mpesa.Config{BaseURL: m.BaseURL, ConsumerKey: m.ConsumerKey, ConsumerSecret: m.ConsumerSecret,
		ShortCode: m.ShortCode, InitiatorName: m.InitiatorName, SecurityCredential: m.SecurityCredential,
		CallbackBase: m.CallbackBaseURL + "/v1/mpesa/callbacks/" + m.CallbackToken}
	if m.Env == config.MpesaFake {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("fake daraja: %w", err)
		}
		fake := mpesa.NewFake()
		fake.Delay = 2 * time.Second
		srv := &http.Server{Handler: fake, ReadHeaderTimeout: 5 * time.Second}
		go func() { _ = srv.Serve(ln) }()
		go func() { <-ctx.Done(); _ = srv.Close() }()
		dc.BaseURL = "http://" + ln.Addr().String()
		dc.ConsumerKey, dc.ConsumerSecret, dc.ShortCode = "fake", "fake", "600000"
		dc.InitiatorName, dc.SecurityCredential = "fake", "fake"
		log.Warn("mpesa: using the fake Daraja; no money moves", slog.String("addr", dc.BaseURL))
	}
	return &httpapi.MpesaDeps{Client: mpesa.NewDaraja(dc, nil), Environment: m.Env,
		CallbackToken: m.CallbackToken, CallbackIPs: m.CallbackIPs}, nil
}

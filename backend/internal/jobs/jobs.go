// Package jobs runs background work from the Postgres jobs table
// (CLAUDE.md: no Redis; FOR UPDATE SKIP LOCKED). Several API instances can
// run a Runner at once: a job is claimed by one of them, daily jobs are
// queued once per date through dedupe_key, and a failed job is retried with
// exponential backoff. Each handler should do its writes in one
// transaction, so a retry never repeats half a job.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Handler runs one job. payload is the job's JSON payload.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Daily is a job queued once per local day.
type Daily struct {
	Kind string
	// At is the local time of day it runs (Africa/Nairobi), for example 1h.
	At time.Duration
}

// Runner claims and runs due jobs.
type Runner struct {
	Pool     *db.Pool
	Log      *slog.Logger
	Handlers map[string]Handler
	Daily    []Daily
	Now      func() time.Time
	worker   string
}

// Nairobi is the pilot's local time zone for daily jobs (EAT, UTC+3, no DST).
var Nairobi = time.FixedZone("EAT", 3*60*60)

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// DayKey is the dedupe key of a daily job for a local date.
func DayKey(kind string, day time.Time) string {
	return kind + ":" + day.In(Nairobi).Format(time.DateOnly)
}

// EnqueueDaily queues today's run of each daily job (once per date). The
// payload names the day before, which is what a nightly job reports on.
func (r *Runner) EnqueueDaily(ctx context.Context) error {
	local := r.now().In(Nairobi)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Nairobi)
	day := midnight.AddDate(0, 0, -1).Format(time.DateOnly)
	return r.Pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		for _, d := range r.Daily {
			payload, _ := json.Marshal(map[string]string{"day": day})
			key := DayKey(d.Kind, midnight)
			if err := q.EnqueueJob(ctx, store.EnqueueJobParams{Kind: d.Kind, Payload: payload, RunAt: midnight.Add(d.At),
				DedupeKey: &key, MaxAttempts: 5}); err != nil {
				return err
			}
		}
		return nil
	})
}

// RunDue claims and runs due jobs until none is left. It returns how many ran.
func (r *Runner) RunDue(ctx context.Context) (int, error) {
	if r.worker == "" {
		host, _ := os.Hostname()
		r.worker = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	n := 0
	for ctx.Err() == nil {
		var job store.Job
		err := r.Pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			var err error
			job, err = store.New(tx).ClaimJob(ctx, &r.worker)
			return err
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		h, ok := r.Handlers[job.Kind]
		runErr := fmt.Errorf("jobs: no handler for %q", job.Kind)
		if ok {
			runErr = h(ctx, job.Payload)
		}
		err = r.Pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			q := store.New(tx)
			if runErr == nil {
				return q.FinishJob(ctx, job.ID)
			}
			msg := runErr.Error()
			if len(msg) > 1000 {
				msg = msg[:1000]
			}
			return q.FailJob(ctx, store.FailJobParams{ID: job.ID, LastError: &msg})
		})
		if err != nil {
			return n, err
		}
		if runErr != nil && r.Log != nil {
			r.Log.Error("job failed", slog.String("kind", job.Kind), slog.Int("attempt", int(job.Attempts)), slog.Any("error", runErr))
		}
		n++
	}
	return n, ctx.Err()
}

// Run queues daily jobs and runs due ones every interval until ctx ends.
func (r *Runner) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := r.EnqueueDaily(ctx); err != nil && ctx.Err() == nil && r.Log != nil {
			r.Log.Error("enqueue daily jobs", slog.Any("error", err))
		}
		if _, err := r.RunDue(ctx); err != nil && ctx.Err() == nil && r.Log != nil {
			r.Log.Error("run jobs", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

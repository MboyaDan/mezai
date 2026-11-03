package mail

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

const (
	maxAttempts = 5
	batchSize   = 10
)

// Enqueue adds a message to the outbox inside the caller's transaction, so
// the email exists only if the change that caused it commits.
func Enqueue(ctx context.Context, q *store.Queries, to, template string, data map[string]string) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return q.EnqueueEmail(ctx, store.EnqueueEmailParams{ToAddress: to, Template: template, Data: b})
}

// Worker sends queued messages.
type Worker struct {
	Pool   *db.Pool
	Sender Sender
	Log    *slog.Logger
}

// Run polls the outbox until ctx ends.
func (w *Worker) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, err := w.SendDue(ctx); err != nil && ctx.Err() == nil {
			w.Log.Error("email outbox", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// SendDue sends one batch of due messages and returns how many were sent.
// Each message is claimed, sent and marked in its own transaction.
func (w *Worker) SendDue(ctx context.Context) (int, error) {
	sent := 0
	for i := 0; i < batchSize; i++ {
		more := false
		err := w.Pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			q := store.New(tx)
			rows, err := q.ClaimEmails(ctx, 1)
			if err != nil || len(rows) == 0 {
				return err
			}
			more = true
			m := rows[0]
			var data map[string]string
			sendErr := json.Unmarshal(m.Data, &data)
			var r Rendered
			if sendErr == nil {
				r, sendErr = Render(m.Template, data)
			}
			if sendErr == nil {
				sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
				sendErr = w.Sender.Send(sctx, m.ToAddress, r)
				cancel()
			}
			if sendErr != nil {
				// Addresses are personal data: log the row id, not the recipient.
				w.Log.Warn("email send failed", slog.String("email_id", m.ID.String()), slog.Int("attempt", int(m.Attempts)+1), slog.Any("error", sendErr))
				return q.MarkEmailFailed(ctx, store.MarkEmailFailedParams{
					ID: m.ID, LastError: truncate(sendErr.Error(), 500), MaxAttempts: maxAttempts,
					RetrySeconds: int32(30 * (1 << m.Attempts)), // 30 s, 1, 2, 4 min
				})
			}
			sent++
			return q.MarkEmailSent(ctx, m.ID)
		})
		if err != nil {
			return sent, err
		}
		if !more {
			break
		}
	}
	return sent, nil
}

func truncate(s string, n int) *string {
	if len(s) > n {
		s = s[:n]
	}
	return &s
}

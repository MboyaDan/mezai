package mail_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

func TestRender(t *testing.T) {
	r, err := mail.Render(mail.TemplateInvitation, map[string]string{"InvitedBy": "Sample Ops", "Role": "Reviewer", "Org": "", "Link": "http://x/accept?token=t"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "You are invited to MezAI" || !strings.Contains(r.Body, "as Reviewer.") || !strings.Contains(r.Body, "http://x/accept?token=t") {
		t.Fatalf("%+v", r)
	}
	if _, err := mail.Render(mail.TemplatePasswordReset, map[string]string{"Name": "x"}); err == nil {
		t.Fatal("a missing variable must fail, not render <no value>")
	}
	if _, err := mail.Render("nope", nil); err == nil {
		t.Fatal("unknown template")
	}
}

type fakeSender struct {
	mu   sync.Mutex
	sent []string
	fail bool
}

func (f *fakeSender) Send(_ context.Context, to string, m mail.Rendered) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("smtp down")
	}
	f.sent = append(f.sent, to+"|"+m.Subject)
	return nil
}

func enqueue(t *testing.T, pool *db.Pool, to string) {
	t.Helper()
	err := pool.InScope(context.Background(), db.System(), func(tx pgx.Tx) error {
		return mail.Enqueue(context.Background(), store.New(tx), to, mail.TemplateVerifyEmail, map[string]string{"Name": "Sample", "Link": "http://x/v?token=secret-token"})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWorkerSendsAndRedacts(t *testing.T) {
	pool, _ := dbtest.New(t)
	enqueue(t, pool, "a@sample.invalid")
	enqueue(t, pool, "b@sample.invalid")
	f := &fakeSender{}
	w := &mail.Worker{Pool: pool, Sender: f, Log: slog.New(slog.DiscardHandler)}
	n, err := w.SendDue(context.Background())
	if err != nil || n != 2 || len(f.sent) != 2 {
		t.Fatalf("sent %d (%v): %v", n, err, f.sent)
	}
	var left int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM email_outbox WHERE status <> 'sent' OR data::text LIKE '%secret-token%'`).Scan(&left)
	if left != 0 {
		t.Fatal("sent messages must be marked sent with their data cleared")
	}
	if n, _ := w.SendDue(context.Background()); n != 0 {
		t.Fatal("nothing should be sent twice")
	}
}

func TestWorkerRetriesThenGivesUp(t *testing.T) {
	pool, _ := dbtest.New(t)
	enqueue(t, pool, "c@sample.invalid")
	w := &mail.Worker{Pool: pool, Sender: &fakeSender{fail: true}, Log: slog.New(slog.DiscardHandler)}
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if _, err := w.SendDue(ctx); err != nil {
			t.Fatal(err)
		}
		var status string
		var attempts int
		_ = pool.QueryRow(ctx, `SELECT status, attempts FROM email_outbox`).Scan(&status, &attempts)
		if attempts != i {
			t.Fatalf("attempt %d recorded as %d", i, attempts)
		}
		if (i < 5 && status != "queued") || (i == 5 && status != "failed") {
			t.Fatalf("after %d attempts: %s", i, status)
		}
		// Make the retry due now instead of waiting for the backoff.
		_, _ = pool.Exec(ctx, `UPDATE email_outbox SET next_attempt_at = now()`)
	}
	var leaked int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE data::text LIKE '%secret-token%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("a message that gave up must not keep its token")
	}
}

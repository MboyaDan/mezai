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

// Stage 16: each notification renders with the data its sender gives, links
// carry the site origin, and optional ones say how to switch them off.
func TestNotificationTemplates(t *testing.T) {
	origin := "https://mezai.example"
	cases := []struct {
		name     string
		data     map[string]string
		want     []string
		optional bool
	}{
		{mail.TemplateQualificationResult, map[string]string{"Name": "A", "Test": "Core check", "Passed": "yes", "Note": ""},
			[]string{"You passed the Core check", origin + "/c/qualify"}, false},
		{mail.TemplateQualificationResult, map[string]string{"Name": "A", "Test": "Core check", "Passed": "no", "Note": "Read the guideline."},
			[]string{"did not pass the Core check", "Read the guideline."}, false},
		{mail.TemplateProjectAvailable, map[string]string{"Name": "A", "Project": "P (Sample)", "ProjectID": "x1", "TaskType": "Translation review",
			"Pay": "KES 25.00", "Deadline": ""}, []string{"P (Sample)", "KES 25.00 for each accepted task", origin + "/c/projects/x1"}, true},
		{mail.TemplateFeedbackReceived, map[string]string{"Name": "A", "Project": "P (Sample)"}, []string{"at most one", origin + "/c"}, true},
		{mail.TemplatePayoutSent, map[string]string{"Name": "A", "Amount": "KES 1,200.00", "PhoneEnd": "001", "Receipt": "SAMPLE0A01",
			"Date": "16 Nov 2026"}, []string{"KES 1,200.00 was sent", "ending 001", "SAMPLE0A01", origin + "/c/earnings"}, false},
	}
	for _, c := range cases {
		c.data["Origin"] = origin
		r, err := mail.Render(c.name, c.data)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, w := range c.want {
			if !strings.Contains(r.Subject+"\n"+r.Body, w) {
				t.Errorf("%s: missing %q in\n%s", c.name, w, r.Body)
			}
		}
		if got := strings.Contains(r.Body, "/c/profile#notifications"); got != c.optional {
			t.Errorf("%s: preferences link %v, want %v", c.name, got, c.optional)
		}
		if strings.Contains(r.Body, "<no value>") {
			t.Errorf("%s: unfilled field", c.name)
		}
	}
	if _, err := mail.Render(mail.TemplatePayoutSent, map[string]string{"Name": "A"}); err == nil {
		t.Error("missing data must fail, not send a broken email")
	}
}

func TestMoneyAndChoices(t *testing.T) {
	for cents, want := range map[int64]string{0: "KES 0.00", 2500: "KES 25.00", 120000: "KES 1,200.00", 123456789: "KES 1,234,567.89", -5050: "KES -50.50"} {
		if got := mail.Money(cents, "KES"); got != want {
			t.Errorf("Money(%d) = %q, want %q", cents, got, want)
		}
	}
	if mail.Wanted(mail.TemplateProjectAvailable, false, true) || mail.Wanted(mail.TemplateFeedbackReceived, true, false) {
		t.Error("an optional email switched off was wanted")
	}
	if !mail.Wanted(mail.TemplatePayoutSent, false, false) || !mail.Wanted(mail.TemplateQualificationResult, false, false) {
		t.Error("payout and qualification emails cannot be switched off")
	}
}

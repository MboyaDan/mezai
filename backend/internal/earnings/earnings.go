// Package earnings serves a contributor's own money record (Stage 12a, brief
// 13): totals from the append-only ledger, the pending estimate from their
// undecided answers, a statement and payout history. Stage 14 adds payout
// batches here.
//
// Every figure is computed from ledger rows when asked; nothing is cached or
// stored twice, so the dashboard can never disagree with the ledger (D-119).
package earnings

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/pii"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ErrNotContributor: the caller has no contributor profile.
var ErrNotContributor = errors.New("earnings: the caller is not a contributor")

// Service reads earnings.
type Service struct{ pool *db.Pool }

// NewService returns the earnings service.
func NewService(pool *db.Pool) *Service { return &Service{pool: pool} }

// Totals are one currency's figures. Approved is the balance: accepted work
// and adjustments not yet paid out. Earned - Paid = Approved always.
type Totals struct {
	Currency      string `json:"currency"`
	PendingCents  int64  `json:"pending_cents"` // estimate at current project rates
	PendingTasks  int32  `json:"pending_tasks"`
	ApprovedCents int64  `json:"approved_cents"`
	PaidCents     int64  `json:"paid_cents"`
	EarnedCents   int64  `json:"earned_cents"`
}

// TaskCounts are the caller's submitted answers by decision.
type TaskCounts struct {
	Submitted   int32 `json:"submitted"`
	Waiting     int32 `json:"waiting"`
	Accepted    int32 `json:"accepted"`
	NotAccepted int32 `json:"not_accepted"`
}

// Summary is the earnings card.
type Summary struct {
	Totals         []Totals   `json:"totals"`
	Tasks          TaskCounts `json:"tasks"`
	NextPayoutDate *string    `json:"next_payout_date"` // YYYY-MM-DD, null when Ops has not set one
	MinPayoutCents int64      `json:"min_payout_cents"`
	Schedule       string     `json:"payout_schedule"`
}

// Line is one statement line: a day's accepted work on a project, or a
// single payout, reversal or adjustment.
type Line struct {
	Day          string     `json:"day"` // Nairobi date
	Type         string     `json:"type"`
	Currency     string     `json:"currency"`
	ProjectID    *uuid.UUID `json:"project_id"`
	ProjectName  *string    `json:"project_name"`
	Tasks        int32      `json:"tasks"` // accrual lines: accepted answers; otherwise 1
	AmountCents  int64      `json:"amount_cents"`
	Reason       *string    `json:"reason"`
	MpesaReceipt *string    `json:"mpesa_receipt"`
}

// Statement is one page of lines, newest day first.
type Statement struct {
	Lines []Line `json:"lines"`
	// Before is the cursor for the next (older) page, null when there is none.
	Before *string `json:"before"`
}

// Payout is one payout line as the contributor sees it.
type Payout struct {
	ID            uuid.UUID  `json:"id"`
	PeriodStart   string     `json:"period_start"`
	PeriodEnd     string     `json:"period_end"`
	AmountCents   int64      `json:"amount_cents"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"` // processing, paid, failed
	MpesaReceipt  *string    `json:"mpesa_receipt"`
	FailureReason *string    `json:"failure_reason"`
	PhoneMasked   string     `json:"phone_masked"`
	RecordedAt    *time.Time `json:"recorded_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

const statementDays = 30

func contributorOf(p *auth.Principal) (uuid.UUID, error) {
	if p == nil || p.Kind != auth.KindContributor || p.ContributorID == uuid.Nil {
		return uuid.Nil, ErrNotContributor
	}
	return p.ContributorID, nil
}

// MySummary returns the caller's totals, task counts and payout settings.
func (s *Service) MySummary(ctx context.Context, p *auth.Principal) (Summary, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Summary{}, err
	}
	out := Summary{Totals: []Totals{}, Schedule: "weekly", MinPayoutCents: 50000}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		ledger, err := q.MyLedgerTotals(ctx, id)
		if err != nil {
			return err
		}
		pending, err := q.MyPendingEarnings(ctx, id)
		if err != nil {
			return err
		}
		out.Totals = mergeTotals(ledger, pending)
		tc, err := q.MyTaskCounts(ctx, id)
		if err != nil {
			return err
		}
		out.Tasks = TaskCounts{Submitted: tc.Submitted, Waiting: tc.Waiting, Accepted: tc.Accepted, NotAccepted: tc.NotAccepted}
		setting(ctx, q, "payout.next_date", &out.NextPayoutDate)
		setting(ctx, q, "payout.min_cents", &out.MinPayoutCents)
		setting(ctx, q, "payout.schedule", &out.Schedule)
		if out.NextPayoutDate != nil {
			if _, err := time.Parse(time.DateOnly, *out.NextPayoutDate); err != nil {
				out.NextPayoutDate = nil // a malformed setting hides itself
			}
		}
		return nil
	})
	return out, err
}

// mergeTotals joins ledger and pending figures by currency, in currency order.
func mergeTotals(ledger []store.MyLedgerTotalsRow, pending []store.MyPendingEarningsRow) []Totals {
	by := map[string]*Totals{}
	order := []string{}
	get := func(cur string) *Totals {
		if t, ok := by[cur]; ok {
			return t
		}
		t := &Totals{Currency: cur}
		by[cur] = t
		order = append(order, cur)
		return t
	}
	for _, l := range ledger {
		t := get(l.Currency)
		t.EarnedCents, t.PaidCents, t.ApprovedCents = l.EarnedCents, l.PaidCents, l.BalanceCents
	}
	for _, pe := range pending {
		t := get(pe.Currency)
		t.PendingCents, t.PendingTasks = pe.Cents, pe.Tasks
	}
	out := make([]Totals, 0, len(order))
	sort.Strings(order)
	for _, c := range order {
		out = append(out, *by[c])
	}
	return out
}

// setting reads a JSON setting into dst, leaving dst unchanged when the key
// is missing or does not parse.
func setting[T any](ctx context.Context, q *store.Queries, key string, dst *T) {
	v, err := q.GetSetting(ctx, key)
	if err != nil {
		return
	}
	var t T
	if json.Unmarshal(v, &t) == nil {
		*dst = t
	}
}

// MyStatement returns the 30 most recent days with entries before the
// cursor day (exclusive). An empty cursor starts from today.
func (s *Service) MyStatement(ctx context.Context, p *auth.Principal, before string) (Statement, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Statement{}, err
	}
	day := time.Now().In(jobs.Nairobi).AddDate(0, 0, 1)
	if before != "" {
		if day, err = time.ParseInLocation(time.DateOnly, before, jobs.Nairobi); err != nil {
			return Statement{}, ErrBadCursor
		}
	}
	cursor := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	out := Statement{Lines: []Line{}}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.MyStatement(ctx, store.MyStatementParams{ContributorID: id, BeforeDay: cursor})
		if err != nil {
			return err
		}
		for _, r := range rows {
			l := Line{Day: r.Day.Format(time.DateOnly), Type: r.EntryType, Currency: r.Currency, ProjectID: r.ProjectID,
				ProjectName: r.ProjectName, Tasks: r.Entries, AmountCents: r.AmountCents}
			if r.EntryType != "accrual" {
				l.Tasks = 0
			}
			if r.Reason != "" {
				l.Reason = &r.Reason
			}
			if r.MpesaReceipt != "" {
				l.MpesaReceipt = &r.MpesaReceipt
			}
			out.Lines = append(out.Lines, l)
		}
		days, err := q.MyStatementDayCount(ctx, store.MyStatementDayCountParams{ContributorID: id, BeforeDay: cursor})
		if err != nil {
			return err
		}
		if days > statementDays && len(out.Lines) > 0 {
			oldest := out.Lines[len(out.Lines)-1].Day
			out.Before = &oldest
		}
		return nil
	})
	return out, err
}

// ErrBadCursor: the statement cursor is not a date.
var ErrBadCursor = errors.New("earnings: before must be a date (YYYY-MM-DD)")

// MyPayouts returns the caller's payout lines, newest first.
func (s *Service) MyPayouts(ctx context.Context, p *auth.Principal) ([]Payout, error) {
	id, err := contributorOf(p)
	if err != nil {
		return nil, err
	}
	out := []Payout{}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).MyPayouts(ctx, id)
		for _, r := range rows {
			status := r.Status
			if status == "pending" {
				status = "processing"
			}
			out = append(out, Payout{ID: r.ID, PeriodStart: r.PeriodStart.Format(time.DateOnly), PeriodEnd: r.PeriodEnd.Format(time.DateOnly),
				AmountCents: r.AmountCents, Currency: r.Currency, Status: status, MpesaReceipt: r.MpesaReceipt,
				FailureReason: r.FailureReason, PhoneMasked: pii.MaskPhone(r.MpesaPhone), RecordedAt: r.RecordedAt, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

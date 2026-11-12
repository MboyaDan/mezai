package payouts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/pii"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

var (
	ErrNotFound       = errors.New("payouts: not found")
	ErrInvalidInput   = errors.New("payouts: invalid input")
	ErrWrongState     = errors.New("payouts: not allowed in the batch's current state")
	ErrOpenBatch      = errors.New("payouts: another batch is still open")
	ErrOwnBatch       = errors.New("payouts: the creator cannot approve their own batch")
	ErrBalanceChanged = errors.New("payouts: a line is no longer covered by the balance")
	ErrReceiptUsed    = errors.New("payouts: receipt already recorded")
	ErrEmpty          = errors.New("payouts: nobody is due a payout")
	ErrStorage        = errors.New("payouts: file storage unavailable")
	ErrFileChanged    = errors.New("payouts: stored file does not match its checksum")
)

// InputError names the field that failed.
type InputError struct{ Field, Message string }

func (e *InputError) Error() string { return e.Field + ": " + e.Message }
func (e *InputError) Unwrap() error { return ErrInvalidInput }

func invalid(field, msg string) error { return &InputError{Field: field, Message: msg} }

// Service runs payout batches.
type Service struct {
	pool  *db.Pool
	blobs storage.Blobs
	now   func() time.Time
	log   *slog.Logger
}

// NewService returns the payouts service.
func NewService(pool *db.Pool, blobs storage.Blobs, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{pool: pool, blobs: blobs, now: time.Now, log: log}
}

// Line is one payee in a batch. The phone is masked: the full number only
// leaves the API in the exported file.
type Line struct {
	ID                uuid.UUID  `json:"id"`
	ContributorID     uuid.UUID  `json:"contributor_id"`
	PublicCode        string     `json:"public_code"`
	Name              string     `json:"name"`
	ContributorStatus string     `json:"contributor_status"`
	PhoneMasked       string     `json:"phone_masked"`
	AmountCents       int64      `json:"amount_cents"`
	Currency          string     `json:"currency"`
	BalanceCents      int64      `json:"balance_cents"`
	Status            string     `json:"status"`
	MpesaReceipt      *string    `json:"mpesa_receipt"`
	FailureReason     *string    `json:"failure_reason"`
	RecordedAt        *time.Time `json:"recorded_at"`
	RecordedByName    *string    `json:"recorded_by_name"`
}

// Held is someone owed money who is not in the batch, and why.
type Held struct {
	ContributorID uuid.UUID `json:"contributor_id"`
	PublicCode    string    `json:"public_code"`
	Name          string    `json:"name"`
	OwedCents     int64     `json:"owed_cents"`
	Reason        string    `json:"reason"`
}

// Preview is what a batch for a period would hold.
type Preview struct {
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	MinCents    int64  `json:"min_cents"`
	TotalCents  int64  `json:"total_cents"`
	Lines       []Line `json:"lines"`
	Held        []Held `json:"held"`
}

// Summary is a batch in the list.
type Summary struct {
	ID             uuid.UUID `json:"id"`
	PeriodStart    string    `json:"period_start"`
	PeriodEnd      string    `json:"period_end"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	TotalCents     int64     `json:"total_cents"`
	LineCount      int32     `json:"line_count"`
	PaidLines      int32     `json:"paid_lines"`
	FailedLines    int32     `json:"failed_lines"`
	PaidCents      int64     `json:"paid_cents"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedByName  string    `json:"created_by_name"`
	ApprovedByName *string   `json:"approved_by_name"`
}

// Batch is one batch with its lines.
type Batch struct {
	Summary
	CreatedBy       uuid.UUID  `json:"created_by"`
	CutoffAt        *time.Time `json:"cutoff_at"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	ApprovedAt      *time.Time `json:"approved_at"`
	ReturnedNote    *string    `json:"returned_note"`
	CancelledAt     *time.Time `json:"cancelled_at"`
	CancelledByName *string    `json:"cancelled_by_name"`
	CancelReason    *string    `json:"cancel_reason"`
	ExportedAt      *time.Time `json:"exported_at"`
	ExportFileName  *string    `json:"export_file_name"`
	FormatConfirmed *bool      `json:"format_confirmed"`
	SettledAt       *time.Time `json:"settled_at"`
	Lines           []Line     `json:"lines"`
}

func day(t time.Time) string { return t.Format(time.DateOnly) }

func ptr[T any](v T) *T { return &v }

// period works out the batch period: from the day after the last batch
// (or the first ledger entry) to periodEnd, which must be before today in
// Nairobi.
func (s *Service) period(ctx context.Context, q *store.Queries, periodEnd string) (start, end time.Time, cutoff time.Time, err error) {
	end, perr := time.ParseInLocation(time.DateOnly, periodEnd, jobs.Nairobi)
	if perr != nil {
		return start, end, cutoff, invalid("period_end", "use YYYY-MM-DD")
	}
	n := s.now().In(jobs.Nairobi)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, jobs.Nairobi)
	if !end.Before(today) {
		return start, end, cutoff, invalid("period_end", "the period must end before today")
	}
	last, err := q.LastPayoutPeriodEnd(ctx)
	if err != nil {
		return start, end, cutoff, err
	}
	switch {
	case len(last) > 0:
		l := last[0]
		start = time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, jobs.Nairobi).AddDate(0, 0, 1)
	default:
		first, err := q.FirstLedgerEntry(ctx)
		if err != nil {
			return start, end, cutoff, err
		}
		start = end
		if len(first) > 0 {
			f := first[0].In(jobs.Nairobi)
			start = time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, jobs.Nairobi)
		}
	}
	if start.After(end) {
		return start, end, cutoff, invalid("period_end", "the last batch already covers this day; choose a later one")
	}
	return start, end, end.AddDate(0, 0, 1), nil
}

func minCents(ctx context.Context, q *store.Queries) (int64, error) {
	v, err := q.GetSetting(ctx, "payout.min_cents")
	if errors.Is(err, pgx.ErrNoRows) {
		return 50000, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.Trim(strings.TrimSpace(string(v)), `"`), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("setting payout.min_cents: %w", err)
	}
	return n, nil
}

func (s *Service) preview(ctx context.Context, q *store.Queries, periodEnd string) (Preview, time.Time, error) {
	start, end, cutoff, err := s.period(ctx, q, periodEnd)
	if err != nil {
		return Preview{}, cutoff, err
	}
	floor, err := minCents(ctx, q)
	if err != nil {
		return Preview{}, cutoff, err
	}
	rows, err := q.PayoutCandidates(ctx, cutoff)
	if err != nil {
		return Preview{}, cutoff, err
	}
	out := Preview{PeriodStart: day(start), PeriodEnd: day(end), MinCents: floor, Lines: []Line{}, Held: []Held{}}
	for _, r := range rows {
		amount, hold := Decide(Candidate{
			BalanceCents: r.BalanceCents, CutoffCents: r.CutoffCents, Phone: r.MpesaPhone,
			PhoneConfirmed:   r.MpesaPhoneConfirmedAt != nil,
			IdentityVerified: r.IdentityStatus == "verified",
			NameChecked:      r.MpesaNameCheckedAt != nil,
		}, floor)
		if hold != "" {
			out.Held = append(out.Held, Held{ContributorID: r.ID, PublicCode: r.PublicCode, Name: r.DisplayName,
				OwedCents: min(r.BalanceCents, r.CutoffCents), Reason: hold})
			continue
		}
		out.Lines = append(out.Lines, Line{ContributorID: r.ID, PublicCode: r.PublicCode, Name: r.DisplayName,
			ContributorStatus: r.Status, PhoneMasked: pii.MaskPhone(*r.MpesaPhone), AmountCents: amount,
			Currency: "KES", BalanceCents: r.BalanceCents, Status: "pending"})
		out.TotalCents += amount
	}
	return out, cutoff, nil
}

// Preview shows what a batch ending on periodEnd would pay and hold.
func (s *Service) Preview(ctx context.Context, p *auth.Principal, periodEnd string) (Preview, error) {
	var out Preview
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		out, _, err = s.preview(ctx, store.New(tx), periodEnd)
		return err
	})
	return out, err
}

// Create makes a draft batch for the period. The idempotency key makes a
// replay return the same batch.
func (s *Service) Create(ctx context.Context, p *auth.Principal, periodEnd, key string, m auth.Meta) (Batch, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 200 {
		return Batch{}, false, invalid("idempotency_key", "required, at most 200 characters")
	}
	var id uuid.UUID
	created := false
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if prior, err := q.GetPayoutBatchByKey(ctx, key); err != nil {
			return err
		} else if len(prior) > 0 {
			id = prior[0]
			return nil
		}
		if open, err := q.OpenPayoutBatch(ctx); err != nil {
			return err
		} else if len(open) > 0 {
			return ErrOpenBatch
		}
		pv, cutoff, err := s.preview(ctx, q, periodEnd)
		if err != nil {
			return err
		}
		if len(pv.Lines) == 0 {
			return ErrEmpty
		}
		start, _ := time.ParseInLocation(time.DateOnly, pv.PeriodStart, jobs.Nairobi)
		end, _ := time.ParseInLocation(time.DateOnly, pv.PeriodEnd, jobs.Nairobi)
		id, err = q.InsertPayoutBatch(ctx, store.InsertPayoutBatchParams{PeriodStart: start, PeriodEnd: end,
			CutoffAt: &cutoff, IdempotencyKey: key, CreatedBy: p.UserID})
		if err != nil {
			return mapPg(err)
		}
		// The phone and name are snapshots; the export uses exactly these.
		rows, err := q.PayoutCandidates(ctx, cutoff)
		if err != nil {
			return err
		}
		byID := map[uuid.UUID]store.PayoutCandidatesRow{}
		for _, r := range rows {
			byID[r.ID] = r
		}
		for _, l := range pv.Lines {
			r := byID[l.ContributorID]
			if err := q.InsertPayoutLine(ctx, store.InsertPayoutLineParams{BatchID: id, ContributorID: l.ContributorID,
				AmountCents: l.AmountCents, MpesaPhone: *r.MpesaPhone, PayeeName: r.DisplayName}); err != nil {
				return err
			}
		}
		if err := q.SetPayoutBatchTotals(ctx, id); err != nil {
			return err
		}
		created = true
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "payout_batch.created", EntityType: "payout_batch",
			EntityID: &id, After: map[string]any{"period_start": pv.PeriodStart, "period_end": pv.PeriodEnd,
				"lines": len(pv.Lines), "total_cents": pv.TotalCents, "held": len(pv.Held)}})
	})
	if err != nil {
		return Batch{}, false, err
	}
	b, err := s.Get(ctx, p, id)
	return b, created, err
}

func mapPg(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		if strings.Contains(pg.ConstraintName, "receipt") {
			return ErrReceiptUsed
		}
		if strings.Contains(pg.ConstraintName, "one_open") {
			return ErrOpenBatch
		}
	}
	return err
}

// List returns the latest batches.
func (s *Service) List(ctx context.Context, p *auth.Principal) ([]Summary, error) {
	out := []Summary{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListPayoutBatches(ctx)
		for _, r := range rows {
			out = append(out, Summary{ID: r.ID, PeriodStart: day(r.PeriodStart), PeriodEnd: day(r.PeriodEnd),
				Currency: r.Currency, Status: r.Status, TotalCents: r.TotalCents, LineCount: r.LineCount,
				PaidLines: r.PaidLines, FailedLines: r.FailedLines, PaidCents: r.PaidCents, CreatedAt: r.CreatedAt,
				CreatedByName: r.CreatedByName, ApprovedByName: r.ApprovedByName})
		}
		return err
	})
	return out, err
}

// Get returns one batch with its lines.
func (s *Service) Get(ctx context.Context, p *auth.Principal, id uuid.UUID) (Batch, error) {
	var out Batch
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		b, err := q.GetPayoutBatch(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		out = Batch{Summary: Summary{ID: b.ID, PeriodStart: day(b.PeriodStart), PeriodEnd: day(b.PeriodEnd),
			Currency: b.Currency, Status: b.Status, TotalCents: b.TotalCents, LineCount: b.LineCount, CreatedAt: b.CreatedAt,
			CreatedByName: b.CreatedByName, ApprovedByName: b.ApprovedByName},
			CreatedBy: b.CreatedBy, CutoffAt: b.CutoffAt, SubmittedAt: b.SubmittedAt, ApprovedAt: b.ApprovedAt,
			ReturnedNote: b.ReturnedNote, CancelledAt: b.CancelledAt, CancelledByName: b.CancelledByName,
			CancelReason: b.CancelReason, ExportedAt: b.ExportedAt, ExportFileName: b.ExportFileName,
			SettledAt: b.SettledAt, Lines: []Line{}}
		if b.ExportFormat != nil {
			var f Format
			if json.Unmarshal(b.ExportFormat, &f) == nil {
				out.FormatConfirmed = &f.Confirmed
			}
		}
		lines, err := q.ListPayoutLines(ctx, id)
		if err != nil {
			return err
		}
		for _, l := range lines {
			out.Lines = append(out.Lines, Line{ID: l.ID, ContributorID: l.ContributorID, PublicCode: l.PublicCode,
				Name: l.PayeeName, ContributorStatus: l.ContributorStatus, PhoneMasked: pii.MaskPhone(l.MpesaPhone),
				AmountCents: l.AmountCents, Currency: l.Currency, BalanceCents: l.BalanceCents, Status: l.Status,
				MpesaReceipt: l.MpesaReceipt, FailureReason: l.FailureReason, RecordedAt: l.RecordedAt,
				RecordedByName: l.RecordedByName})
			switch l.Status {
			case "paid":
				out.PaidLines++
				out.PaidCents += l.AmountCents
			case "failed":
				out.FailedLines++
			}
		}
		return nil
	})
	return out, err
}

// transition locks the batch, checks its state, runs fn and audits.
func (s *Service) transition(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta, action, reason string,
	fn func(q *store.Queries, b store.LockPayoutBatchRow) (int64, error)) (Batch, error) {
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		b, err := q.LockPayoutBatch(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		n, err := fn(q, b)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrWrongState
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: action, EntityType: "payout_batch",
			EntityID: &id, Reason: reason, Before: map[string]any{"status": b.Status}})
	})
	if err != nil {
		return Batch{}, err
	}
	return s.Get(ctx, p, id)
}

// covered refuses when a line is now more than the contributor's balance
// (a correction after the batch was made).
func covered(ctx context.Context, q *store.Queries, id uuid.UUID) error {
	lines, err := q.ListPayoutLines(ctx, id)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if l.Status == "pending" && l.AmountCents > l.BalanceCents {
			return ErrBalanceChanged
		}
	}
	return nil
}

// Submit sends a draft to the approver.
func (s *Service) Submit(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (Batch, error) {
	return s.transition(ctx, p, id, m, "payout_batch.submitted", "", func(q *store.Queries, _ store.LockPayoutBatchRow) (int64, error) {
		return q.SubmitPayoutBatch(ctx, id)
	})
}

// Approve is the second person's decision (two-person rule, AC-11).
func (s *Service) Approve(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (Batch, error) {
	return s.transition(ctx, p, id, m, "payout_batch.approved", "", func(q *store.Queries, b store.LockPayoutBatchRow) (int64, error) {
		if b.CreatedBy == p.UserID {
			return 0, ErrOwnBatch
		}
		if b.Status != "pending_approval" {
			return 0, ErrWrongState
		}
		if err := covered(ctx, q, id); err != nil {
			return 0, err
		}
		return q.ApprovePayoutBatch(ctx, store.ApprovePayoutBatchParams{ApprovedBy: &p.UserID, ID: id})
	})
}

func note(field, v string, required bool) (string, error) {
	v = strings.TrimSpace(v)
	if required && v == "" {
		return "", invalid(field, "required")
	}
	if utf8.RuneCountInString(v) > 500 {
		return "", invalid(field, "at most 500 characters")
	}
	return v, nil
}

// Return sends a batch back to its creator with a note.
func (s *Service) Return(ctx context.Context, p *auth.Principal, id uuid.UUID, reason string, m auth.Meta) (Batch, error) {
	reason, err := note("reason", reason, true)
	if err != nil {
		return Batch{}, err
	}
	return s.transition(ctx, p, id, m, "payout_batch.returned", reason, func(q *store.Queries, _ store.LockPayoutBatchRow) (int64, error) {
		return q.ReturnPayoutBatch(ctx, store.ReturnPayoutBatchParams{Note: &reason, ID: id})
	})
}

// Cancel closes a batch before approval; it is kept with the reason.
func (s *Service) Cancel(ctx context.Context, p *auth.Principal, id uuid.UUID, reason string, m auth.Meta) (Batch, error) {
	reason, err := note("reason", reason, true)
	if err != nil {
		return Batch{}, err
	}
	return s.transition(ctx, p, id, m, "payout_batch.cancelled", reason, func(q *store.Queries, _ store.LockPayoutBatchRow) (int64, error) {
		return q.CancelPayoutBatch(ctx, store.CancelPayoutBatchParams{CancelledBy: &p.UserID, Reason: &reason, ID: id})
	})
}

// Export writes the bulk payment file for an approved batch and stores it.
// The file's total is checked against the batch before anything is saved.
func (s *Service) Export(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (Batch, error) {
	key := fmt.Sprintf("payouts/%s/%s.csv", id, uuid.Must(uuid.NewV7()))
	stored := false
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		b, err := q.LockPayoutBatch(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if b.Status != "approved" {
			return ErrWrongState
		}
		if err := covered(ctx, q, id); err != nil {
			return err
		}
		raw, err := q.GetSetting(ctx, "payout.export_format")
		if err != nil {
			return err
		}
		f, err := ParseFormat(raw)
		if err != nil {
			return err
		}
		lines, err := q.ListPayoutLines(ctx, id)
		if err != nil {
			return err
		}
		payees := make([]Payee, 0, len(lines))
		var total int64
		for _, l := range lines {
			payees = append(payees, Payee{PhoneE164: l.MpesaPhone, AmountCents: l.AmountCents, Name: l.PayeeName, PublicCode: l.PublicCode})
			total += l.AmountCents
		}
		if total != b.TotalCents || int32(len(lines)) != b.LineCount {
			return fmt.Errorf("payout batch %s: lines total %d (%d lines) but batch says %d (%d)", id, total, len(lines), b.TotalCents, b.LineCount)
		}
		data, err := BuildCSV(f, payees, b.PeriodEnd)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if err := s.blobs.Put(ctx, key, "text/csv", data); err != nil {
			s.log.Error("payout export upload failed", slog.Any("error", err))
			return ErrStorage
		}
		stored = true
		fmtJSON, _ := json.Marshal(f)
		name := fmt.Sprintf("mezai-payouts-%s.csv", day(b.PeriodEnd))
		n, err := q.ExportPayoutBatch(ctx, store.ExportPayoutBatchParams{ExportedBy: &p.UserID, FileKey: &key,
			FileName: &name, Checksum: ptr(hex.EncodeToString(sum[:])), Format: fmtJSON, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrWrongState
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "payout_batch.exported", EntityType: "payout_batch",
			EntityID: &id, After: map[string]any{"lines": len(lines), "total_cents": total, "format_confirmed": f.Confirmed}})
	})
	if err != nil {
		if stored {
			_ = s.blobs.Delete(context.WithoutCancel(ctx), key)
		}
		return Batch{}, err
	}
	return s.Get(ctx, p, id)
}

// File is the exported bulk payment file.
type File struct {
	Name string
	Data []byte
}

// MaxExportBytes bounds a read of the file (50 lines are a few kilobytes).
const MaxExportBytes = 5 << 20

// Download returns the exported file, checked against its checksum, and
// records who downloaded it.
func (s *Service) Download(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (File, error) {
	var b store.GetPayoutBatchRow
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		b, err = store.New(tx).GetPayoutBatch(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return File{}, err
	}
	if b.ExportFileKey == nil || b.ExportChecksum == nil {
		return File{}, ErrNotFound
	}
	data, err := s.blobs.Get(ctx, *b.ExportFileKey, MaxExportBytes)
	if err != nil {
		s.log.Error("payout export read failed", slog.Any("error", err))
		return File{}, ErrStorage
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != *b.ExportChecksum {
		return File{}, ErrFileChanged
	}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		return auth.Audit(ctx, store.New(tx), m, auth.Entry{Actor: &p.UserID, Action: "payout_batch.downloaded",
			EntityType: "payout_batch", EntityID: &id})
	})
	if err != nil {
		return File{}, err
	}
	return File{Name: *b.ExportFileName, Data: data}, nil
}

var receiptRe = regexp.MustCompile(`^[A-Z0-9]{8,12}$`)

// Result is what Ops records for a line after paying in the portal.
type Result struct {
	Status  string `json:"status"` // paid or failed
	Receipt string `json:"receipt"`
	Reason  string `json:"reason"`
}

// RecordResult records one line's outcome. A paid line writes the payout
// ledger entry in the same transaction; a failed line moves no money. The
// same result again is a no-op (idempotent); a different one is refused.
// When every line has a result, the batch is settled.
func (s *Service) RecordResult(ctx context.Context, p *auth.Principal, lineID uuid.UUID, in Result, m auth.Meta) (Batch, error) {
	in.Receipt = strings.ToUpper(strings.TrimSpace(in.Receipt))
	switch in.Status {
	case "paid":
		if !receiptRe.MatchString(in.Receipt) {
			return Batch{}, invalid("receipt", "the M-Pesa receipt: 8 to 12 letters and digits")
		}
	case "failed":
		r, err := note("reason", in.Reason, true)
		if err != nil {
			return Batch{}, err
		}
		in.Reason = r
	default:
		return Batch{}, invalid("status", "paid or failed")
	}
	var batchID uuid.UUID
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		l, err := q.LockPayoutLine(ctx, lineID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		batchID = l.BatchID
		if l.Status != "pending" {
			same := l.Status == in.Status && ((in.Status == "paid" && l.MpesaReceipt != nil && *l.MpesaReceipt == in.Receipt) ||
				(in.Status == "failed" && l.FailureReason != nil && *l.FailureReason == in.Reason))
			if same {
				return nil
			}
			return ErrWrongState
		}
		if l.BatchStatus != "exported" {
			return ErrWrongState
		}
		if in.Status == "paid" {
			if _, err := q.RecordPayoutLinePaid(ctx, store.RecordPayoutLinePaidParams{Receipt: &in.Receipt, RecordedBy: &p.UserID, ID: lineID}); err != nil {
				return mapPg(err)
			}
			if err := q.InsertPayoutLedgerEntry(ctx, store.InsertPayoutLedgerEntryParams{ContributorID: l.ContributorID,
				AmountCents: l.AmountCents, Currency: l.Currency, PayoutLineID: &lineID, CreatedBy: &p.UserID}); err != nil {
				return err
			}
		} else if _, err := q.RecordPayoutLineFailed(ctx, store.RecordPayoutLineFailedParams{Reason: &in.Reason, RecordedBy: &p.UserID, ID: lineID}); err != nil {
			return err
		}
		after := map[string]any{"status": in.Status, "amount_cents": l.AmountCents}
		if in.Status == "paid" {
			after["receipt"] = in.Receipt
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "payout_line." + in.Status, EntityType: "payout_line",
			EntityID: &lineID, Reason: in.Reason, After: after}); err != nil {
			return err
		}
		settled, err := q.SettlePayoutBatch(ctx, l.BatchID)
		if err != nil {
			return err
		}
		if settled > 0 {
			return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "payout_batch.settled", EntityType: "payout_batch", EntityID: &l.BatchID})
		}
		return nil
	})
	if err != nil {
		return Batch{}, err
	}
	return s.Get(ctx, p, batchID)
}

// SetNextDate sets the next payout date contributors see (D-119), or hides
// it with an empty date.
func (s *Service) SetNextDate(ctx context.Context, p *auth.Principal, date string, m auth.Meta) (*string, error) {
	date = strings.TrimSpace(date)
	value := []byte("null")
	var out *string
	if date != "" {
		d, err := time.ParseInLocation(time.DateOnly, date, jobs.Nairobi)
		if err != nil {
			return nil, invalid("date", "use YYYY-MM-DD")
		}
		n := s.now().In(jobs.Nairobi)
		if d.Before(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, jobs.Nairobi)) {
			return nil, invalid("date", "the date has passed")
		}
		value, _ = json.Marshal(date)
		out = &date
	}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.SetSettingValue(ctx, store.SetSettingValueParams{Value: value, Key: "payout.next_date"}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "payout.next_date_set", EntityType: "setting",
			After: map[string]any{"date": out}})
	})
	return out, err
}

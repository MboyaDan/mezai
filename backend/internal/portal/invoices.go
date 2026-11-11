package portal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// MaxInvoiceBytes bounds an uploaded invoice PDF.
const MaxInvoiceBytes = 5 << 20

// Invoice is an invoice Ops uploaded for an organisation. Invoices are issued
// outside MezAI in the pilot (brief 3); this is where the client finds them.
type Invoice struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	OrgName     string     `json:"org_name"`
	ProjectID   *uuid.UUID `json:"project_id"`
	ProjectName *string    `json:"project_name"`
	Number      string     `json:"number"`
	IssuedOn    string     `json:"issued_on"`
	DueOn       *string    `json:"due_on"`
	AmountCents int64      `json:"amount_cents"`
	Currency    string     `json:"currency"`
	Description string     `json:"description"`
	FileName    string     `json:"file_name"`
	Bytes       int64      `json:"bytes"`
	Status      string     `json:"status"` // issued, paid or void
	PaidOn      *string    `json:"paid_on"`
	VoidReason  *string    `json:"void_reason"`
	CreatedAt   time.Time  `json:"created_at"`
}

func dayPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	d := day(*t)
	return &d
}

func invoiceFrom(r store.ListInvoicesRow) Invoice {
	return Invoice{ID: r.ID, OrgID: r.OrgID, OrgName: r.OrgName, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
		Number: r.Number, IssuedOn: day(r.IssuedOn), DueOn: dayPtr(r.DueOn), AmountCents: r.AmountCents, Currency: r.Currency,
		Description: r.Description, FileName: r.FileName, Bytes: r.ByteSize, Status: r.Status, PaidOn: dayPtr(r.PaidOn),
		VoidReason: r.VoidReason, CreatedAt: r.CreatedAt}
}

// ListInvoices lists invoices: a client its own organisation's (row-level
// security), staff all or one organisation's.
func (s *Service) ListInvoices(ctx context.Context, p *auth.Principal, org *uuid.UUID) ([]Invoice, error) {
	out := []Invoice{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListInvoices(ctx, org)
		for _, r := range rows {
			out = append(out, invoiceFrom(r))
		}
		return err
	})
	return out, err
}

func (s *Service) visibleInvoice(ctx context.Context, p *auth.Principal, id uuid.UUID) (store.GetInvoiceRow, error) {
	var r store.GetInvoiceRow
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		r, err = store.New(tx).GetInvoice(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return r, err
}

// NewInvoice is what Ops enters with the PDF.
type NewInvoice struct {
	OrgID       uuid.UUID
	ProjectID   *uuid.UUID
	Number      string
	IssuedOn    string
	DueOn       string
	AmountCents int64
	Currency    string
	Description string
}

var invoiceNumber = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/._-]{0,59}$`)

func parseDay(s string) (time.Time, error) { return time.Parse(time.DateOnly, s) }

// UploadInvoice stores an invoice PDF and its facts (Ops only).
func (s *Service) UploadInvoice(ctx context.Context, p *auth.Principal, in NewInvoice, name string, data []byte, m auth.Meta) (Invoice, error) {
	in.Number = strings.TrimSpace(in.Number)
	in.Description = strings.TrimSpace(in.Description)
	if in.Currency == "" {
		in.Currency = "KES"
	}
	issued, err := parseDay(in.IssuedOn)
	if err != nil || !invoiceNumber.MatchString(in.Number) || in.AmountCents < 0 || len(in.Description) > 500 ||
		in.Currency != "KES" || in.OrgID == uuid.Nil {
		return Invoice{}, ErrInvalidInput
	}
	var due *time.Time
	if in.DueOn != "" {
		d, err := parseDay(in.DueOn)
		if err != nil || d.Before(issued) {
			return Invoice{}, ErrInvalidInput
		}
		due = &d
	}
	if len(data) == 0 || len(data) > MaxInvoiceBytes || !bytes.HasPrefix(data, []byte("%PDF-")) ||
		!strings.EqualFold(path.Ext(name), ".pdf") {
		return Invoice{}, ErrInvalidInput
	}
	fileName := fmt.Sprintf("invoice-%s.pdf", strings.NewReplacer("/", "-").Replace(in.Number))
	sum := sha256.Sum256(data)
	key := fmt.Sprintf("invoices/%s/%s.pdf", in.OrgID, uuid.New())
	if err := s.blobs.Put(ctx, key, "application/pdf", data); err != nil {
		s.log.ErrorContext(ctx, "invoice store failed", "err", err)
		return Invoice{}, ErrStorage
	}
	var id uuid.UUID
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		id, err = q.CreateInvoice(ctx, store.CreateInvoiceParams{OrgID: in.OrgID, ProjectID: in.ProjectID, Number: in.Number,
			IssuedOn: issued, DueOn: due, AmountCents: in.AmountCents, Currency: in.Currency, Description: in.Description,
			FileKey: key, FileName: fileName, ByteSize: int64(len(data)), ChecksumSha256: hex.EncodeToString(sum[:]), UploadedBy: p.UserID})
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return ErrConflict
		}
		if errors.As(err, &pg) && pg.Code == "23503" {
			return ErrInvalidInput // unknown organisation, or a project of another organisation
		}
		if err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "invoice.uploaded", EntityType: "invoice",
			EntityID: &id, OrgID: &in.OrgID, After: map[string]any{"number": in.Number, "amount_cents": in.AmountCents}})
	})
	if err != nil {
		if derr := s.blobs.Delete(ctx, key); derr != nil {
			s.log.ErrorContext(ctx, "invoice cleanup failed", "err", derr)
		}
		return Invoice{}, err
	}
	r, err := s.visibleInvoice(ctx, p, id)
	return invoiceFrom(store.ListInvoicesRow(r)), err
}

// SetInvoiceStatus marks an invoice paid (with the day) or void (with a
// reason). A void invoice stays void.
func (s *Service) SetInvoiceStatus(ctx context.Context, p *auth.Principal, id uuid.UUID, status, paidOn, reason string, m auth.Meta) (Invoice, error) {
	arg := store.SetInvoiceStatusParams{ID: id, Status: status}
	reason = strings.TrimSpace(reason)
	switch status {
	case "paid":
		d, err := parseDay(paidOn)
		if err != nil {
			return Invoice{}, ErrInvalidInput
		}
		arg.PaidOn = &d
	case "void":
		if reason == "" || len(reason) > 500 {
			return Invoice{}, ErrInvalidInput
		}
		arg.VoidReason = &reason
	default:
		return Invoice{}, ErrInvalidInput
	}
	cur, err := s.visibleInvoice(ctx, p, id)
	if err != nil {
		return Invoice{}, err
	}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.SetInvoiceStatus(ctx, arg); errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		} else if err != nil {
			return err
		}
		action := map[string]string{"paid": "invoice.paid", "void": "invoice.voided"}[status]
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: action, EntityType: "invoice", EntityID: &id,
			OrgID: &cur.OrgID, Reason: reason, Before: map[string]any{"status": cur.Status}, After: map[string]any{"status": status}})
	})
	if err != nil {
		return Invoice{}, err
	}
	r, err := s.visibleInvoice(ctx, p, id)
	return invoiceFrom(store.ListInvoicesRow(r)), err
}

// InvoiceFile is a PDF ready to send.
type InvoiceFile struct {
	Name string
	Data []byte
}

// DownloadInvoice reads an invoice the caller can see, checks it against its
// checksum and records the download in the audit log.
func (s *Service) DownloadInvoice(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (InvoiceFile, error) {
	r, err := s.visibleInvoice(ctx, p, id)
	if err != nil {
		return InvoiceFile{}, err
	}
	data, err := s.blobs.Get(ctx, r.FileKey, MaxInvoiceBytes)
	if err != nil {
		s.log.ErrorContext(ctx, "invoice read failed", "err", err)
		return InvoiceFile{}, ErrStorage
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != r.ChecksumSha256 {
		s.log.ErrorContext(ctx, "invoice checksum mismatch", "invoice", r.ID)
		return InvoiceFile{}, ErrFileChanged
	}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		return auth.Audit(ctx, store.New(tx), m, auth.Entry{Actor: &p.UserID, Action: "invoice.downloaded", EntityType: "invoice",
			EntityID: &r.ID, OrgID: &r.OrgID})
	})
	if err != nil {
		return InvoiceFile{}, err
	}
	return InvoiceFile{Name: r.FileName, Data: data}, nil
}

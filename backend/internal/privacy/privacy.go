// Package privacy carries out the retention rule for client task content and
// a contributor's rights of access and erasure (Stage 17b, brief 12, Kenya
// Data Protection Act 2019).
//
// The database does the blanking (migration 00026: purge_project_content and
// erase_contributor); this package decides when, checks who may, records the
// audit entry and deletes the stored files the database queued.
package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

var (
	ErrNotFound      = errors.New("privacy: not found")
	ErrNotClosed     = errors.New("privacy: project is not closed")
	ErrAlreadyPurged = errors.New("privacy: task content already deleted")
	ErrConfirm       = errors.New("privacy: confirmation does not match")
	ErrReason        = errors.New("privacy: a reason is required")
	ErrErased        = errors.New("privacy: contributor already erased")
	ErrNoRequest     = errors.New("privacy: no open erasure request")
)

// BlockedError says what must be settled before a contributor is erased.
type BlockedError struct{ Blockers Blockers }

func (e *BlockedError) Error() string { return "privacy: erasure blocked" }

// Service runs retention and data subject requests.
type Service struct {
	pool  *db.Pool
	blobs storage.Blobs
	log   *slog.Logger
}

// NewService returns a Service. blobs may be nil in tests that keep no files.
func NewService(pool *db.Pool, blobs storage.Blobs, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{pool: pool, blobs: blobs, log: log}
}

// ---------------------------------------------------------------------------
// Retention of task content

// Retention is a project's retention state.
type Retention struct {
	Days     int32      `json:"retention_days"`
	ClosedAt *time.Time `json:"closed_at"`
	DueAt    *time.Time `json:"due_at"` // when the sweep deletes content; nil until closed
	PurgedAt *time.Time `json:"purged_at"`
}

func retentionOf(r store.GetProjectRetentionRow) Retention {
	out := Retention{Days: r.RetentionDays, ClosedAt: r.ClosedAt, PurgedAt: r.ContentPurgedAt}
	if r.Status == "closed" && r.ClosedAt != nil && r.ContentPurgedAt == nil {
		due := r.ClosedAt.AddDate(0, 0, int(r.RetentionDays))
		out.DueAt = &due
	}
	return out
}

// ProjectRetention returns the retention state of a project the caller can see.
func (s *Service) ProjectRetention(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (Retention, error) {
	var out Retention
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		r, err := store.New(tx).GetProjectRetention(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		out = retentionOf(r)
		return err
	})
	return out, err
}

// PurgeResult counts what a purge blanked.
type PurgeResult struct {
	Retention Retention        `json:"retention"`
	Counts    map[string]int64 `json:"counts"`
}

// PurgeNow deletes a closed project's task content before its retention
// period ends, for example when the client asks. confirm must equal the
// project's name and reason says who asked.
func (s *Service) PurgeNow(ctx context.Context, p *auth.Principal, projectID uuid.UUID, confirm, reason string, m auth.Meta) (PurgeResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return PurgeResult{}, ErrReason
	}
	var out PurgeResult
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.GetProjectRetention(ctx, projectID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrNotFound
		case err != nil:
			return err
		case r.ContentPurgedAt != nil:
			return ErrAlreadyPurged
		case r.Status != "closed":
			return ErrNotClosed
		case strings.TrimSpace(confirm) != r.Name:
			return ErrConfirm
		}
		counts, err := s.purge(ctx, q, m, &p.UserID, r, reason)
		out.Counts = counts
		return err
	})
	if err != nil {
		return PurgeResult{}, err
	}
	s.DeleteQueuedFiles(ctx)
	out.Retention, err = s.ProjectRetention(ctx, p, projectID)
	return out, err
}

func (s *Service) purge(ctx context.Context, q *store.Queries, m auth.Meta, actor *uuid.UUID, r store.GetProjectRetentionRow, reason string) (map[string]int64, error) {
	raw, err := q.PurgeProjectContent(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	if err := json.Unmarshal(raw, &counts); err != nil {
		return nil, fmt.Errorf("purge counts: %w", err)
	}
	after := map[string]any{"retention_days": r.RetentionDays}
	for k, v := range counts {
		after[k] = v
	}
	return counts, auth.Audit(ctx, q, m, auth.Entry{Actor: actor, Action: "project.content_deleted", EntityType: "project",
		EntityID: &r.ID, OrgID: &r.OrgID, Reason: reason, After: after})
}

// SweepResult counts one retention sweep.
type SweepResult struct {
	Projects       int
	RequestSamples int64
	Emails         int64
	IdemKeys       int64
	Files          int
}

// Sweep applies every retention rule that has come due: closed projects'
// content, declined request samples, sent emails' merge data and expired
// idempotency records, then deletes the queued files. Each project is
// purged in its own transaction so one failure does not hold up the rest.
func (s *Service) Sweep(ctx context.Context) (SweepResult, error) {
	var res SweepResult
	var due []uuid.UUID
	var days, emailDays int
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if days, err = intSetting(ctx, q, "retention.days_after_close", 90); err != nil {
			return err
		}
		if emailDays, err = intSetting(ctx, q, "retention.email_days", 30); err != nil {
			return err
		}
		due, err = q.ProjectsDueForPurge(ctx)
		return err
	})
	if err != nil {
		return res, err
	}
	var errs []error
	for _, id := range due {
		err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			q := store.New(tx)
			r, err := q.GetProjectRetention(ctx, id)
			if err != nil {
				return err
			}
			_, err = s.purge(ctx, q, auth.Meta{}, nil, r, "Retention period ended")
			return err
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("project %s: %w", id, err))
			continue
		}
		res.Projects++
	}
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if res.RequestSamples, err = q.QueueDeclinedRequestSamples(ctx, int32(days)); err != nil {
			return err
		}
		if res.Emails, err = q.RedactSentEmails(ctx, int32(emailDays)); err != nil {
			return err
		}
		res.IdemKeys, err = q.DeleteExpiredIdempotencyKeys(ctx)
		return err
	})
	if err != nil {
		errs = append(errs, err)
	}
	res.Files = s.DeleteQueuedFiles(ctx)
	return res, errors.Join(errs...)
}

func intSetting(ctx context.Context, q *store.Queries, key string, def int) (int, error) {
	raw, err := q.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return 0, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("setting %s: %w", key, err)
	}
	var n int
	switch t := v.(type) {
	case float64:
		n = int(t)
	case string:
		if n, err = strconv.Atoi(t); err != nil {
			return 0, fmt.Errorf("setting %s: %w", key, err)
		}
	default:
		return 0, fmt.Errorf("setting %s: not a number", key)
	}
	if n < 0 {
		return 0, fmt.Errorf("setting %s: negative", key)
	}
	return n, nil
}

// DeleteQueuedFiles deletes objects queued by a purge or erasure. A failure
// stays queued and is retried by the next sweep. It returns how many went.
func (s *Service) DeleteQueuedFiles(ctx context.Context) int {
	if s.blobs == nil {
		return 0
	}
	n := 0
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		keys, err := q.ClaimStorageDeletions(ctx)
		if err != nil {
			return err
		}
		for _, k := range keys {
			if err := s.blobs.Delete(ctx, k); err != nil {
				if err := q.FailStorageDeletion(ctx, store.FailStorageDeletionParams{Key: k, Error: err.Error()}); err != nil {
					return err
				}
				continue
			}
			if err := q.DoneStorageDeletion(ctx, k); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		s.log.ErrorContext(ctx, "storage deletions failed", slog.Any("error", err))
	}
	return n
}

// RunSweep runs Sweep every interval until ctx ends. Several API instances
// may run it: purges lock their project and files are claimed SKIP LOCKED.
func (s *Service) RunSweep(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		res, err := s.Sweep(ctx)
		if err != nil && ctx.Err() == nil {
			s.log.Error("retention sweep failed", slog.Any("error", err))
		}
		if res.Projects > 0 || res.RequestSamples > 0 || res.Files > 0 {
			s.log.Info("retention sweep", slog.Int("projects", res.Projects), slog.Int64("request_samples", res.RequestSamples),
				slog.Int64("emails", res.Emails), slog.Int("files", res.Files))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// ---------------------------------------------------------------------------
// Right of access

// Export is a contributor's personal data as one JSON document.
type Export struct {
	FileName string
	Data     []byte
}

// notIncluded tells the person what is kept but not in the file, and why.
var notIncluded = []string{
	"The task content you worked on and your answers to it: they belong to the client's dataset. Your answers' dates, status and any rejection reason are included.",
	"Your answers to qualification questions and which were right: the questions are reused, so their answers stay private. Your scores are included.",
	"Your password (we keep only a one-way hash) and your National ID number (we never keep it; only its last 3 digits, included).",
	"The audit log of actions on your account: available on request.",
}

// ExportContributor returns everything held about a contributor. self is
// true when the contributor asked from their own account.
func (s *Service) ExportContributor(ctx context.Context, actor uuid.UUID, contributorID uuid.UUID, self bool, m auth.Meta) (Export, error) {
	var out Export
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		q := store.New(tx)
		b, err := q.ErasureBlockers(ctx, contributorID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if b.ErasedAt != nil {
			return ErrErased
		}
		raw, err := q.ContributorDataExport(ctx, contributorID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		doc := map[string]any{
			"about": "Personal data MezAI (a division of ELS Software Solutions) holds about you, " +
				"under the Kenya Data Protection Act 2019. Ask us to correct anything that is wrong.",
			"generated_at": now.Format(time.RFC3339),
			"not_included": notIncluded,
			"data":         json.RawMessage(raw),
		}
		if out.Data, err = json.MarshalIndent(doc, "", "  "); err != nil {
			return err
		}
		out.FileName = fmt.Sprintf("mezai-data-%s-%s.json", b.PublicCode, now.Format("2006-01-02"))
		by := "ops"
		if self {
			by = "self"
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &actor, Action: "contributor.data_exported", EntityType: "contributor",
			EntityID: &contributorID, After: map[string]any{"by": by, "bytes": len(out.Data)}})
	})
	return out, err
}

// ---------------------------------------------------------------------------
// Right to erasure

// Blockers are what must be settled before erasure (A-088).
type Blockers struct {
	BalanceCents       int64 `json:"balance_cents"`        // approved earnings not yet paid
	PendingPayoutLines int32 `json:"pending_payout_lines"` // in a batch, not yet paid or failed
	UndecidedAnswers   int32 `json:"undecided_answers"`    // submitted, may still earn
}

// Clear reports whether erasure can go ahead.
func (b Blockers) Clear() bool {
	return b.BalanceCents == 0 && b.PendingPayoutLines == 0 && b.UndecidedAnswers == 0
}

// Request is an open erasure request.
type Request struct {
	ID            uuid.UUID `json:"id"`
	ByContributor bool      `json:"by_contributor"`
	Note          *string   `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Erasure is a contributor's erasure state.
type Erasure struct {
	PublicCode string     `json:"public_code"`
	ErasedAt   *time.Time `json:"erased_at"`
	Request    *Request   `json:"request"`
	Blockers   Blockers   `json:"blockers"`
	CanErase   bool       `json:"can_erase"`
}

func (s *Service) erasure(ctx context.Context, q *store.Queries, contributorID uuid.UUID) (Erasure, error) {
	b, err := q.ErasureBlockers(ctx, contributorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Erasure{}, ErrNotFound
	}
	if err != nil {
		return Erasure{}, err
	}
	out := Erasure{PublicCode: b.PublicCode, ErasedAt: b.ErasedAt,
		Blockers: Blockers{BalanceCents: b.BalanceCents, PendingPayoutLines: b.PendingPayoutLines, UndecidedAnswers: b.UndecidedAnswers}}
	out.CanErase = out.ErasedAt == nil && out.Blockers.Clear()
	r, err := q.GetOpenErasureRequest(ctx, contributorID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return Erasure{}, err
	default:
		out.Request = &Request{ID: r.ID, ByContributor: r.RequestedBy == b.UserID, Note: r.Note, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// GetErasure returns a contributor's erasure state.
func (s *Service) GetErasure(ctx context.Context, contributorID uuid.UUID) (Erasure, error) {
	var out Erasure
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		var err error
		out, err = s.erasure(ctx, store.New(tx), contributorID)
		return err
	})
	return out, err
}

// RequestErasure records a request to erase a contributor. Asking twice
// returns the open request.
func (s *Service) RequestErasure(ctx context.Context, actor, contributorID uuid.UUID, note string, m auth.Meta) (Erasure, error) {
	note = strings.TrimSpace(note)
	if len(note) > 1000 {
		return Erasure{}, ErrReason
	}
	var out Erasure
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := s.erasure(ctx, q, contributorID)
		if err != nil {
			return err
		}
		if cur.ErasedAt != nil {
			return ErrErased
		}
		if cur.Request == nil {
			var n *string
			if note != "" {
				n = &note
			}
			r, err := q.CreateErasureRequest(ctx, store.CreateErasureRequestParams{ContributorID: contributorID, RequestedBy: actor, Note: n})
			if err != nil {
				return err
			}
			if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &actor, Action: "contributor.erasure_requested", EntityType: "contributor",
				EntityID: &contributorID, After: map[string]any{"request_id": r.ID}}); err != nil {
				return err
			}
		}
		out, err = s.erasure(ctx, q, contributorID)
		return err
	})
	return out, err
}

// WithdrawErasure closes an open request without erasing.
func (s *Service) WithdrawErasure(ctx context.Context, actor, contributorID uuid.UUID, m auth.Meta) (Erasure, error) {
	var out Erasure
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		q := store.New(tx)
		n, err := q.WithdrawErasureRequest(ctx, store.WithdrawErasureRequestParams{UserID: &actor, ContributorID: contributorID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNoRequest
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &actor, Action: "contributor.erasure_withdrawn", EntityType: "contributor",
			EntityID: &contributorID}); err != nil {
			return err
		}
		out, err = s.erasure(ctx, q, contributorID)
		return err
	})
	return out, err
}

// Erase removes a contributor's personal data and closes the account. Money
// owed in either direction and undecided answers block it. confirm must be
// the contributor's public code; reason records how the request reached us.
func (s *Service) Erase(ctx context.Context, actor, contributorID uuid.UUID, confirm, reason string, m auth.Meta) (Erasure, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return Erasure{}, ErrReason
	}
	var out Erasure
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := s.erasure(ctx, q, contributorID)
		switch {
		case err != nil:
			return err
		case cur.ErasedAt != nil:
			return ErrErased
		case !cur.Blockers.Clear():
			return &BlockedError{Blockers: cur.Blockers}
		case strings.TrimSpace(confirm) != cur.PublicCode:
			return ErrConfirm
		}
		if err := q.EraseContributor(ctx, store.EraseContributorParams{ContributorID: contributorID, Actor: actor}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &actor, Action: "contributor.erased", EntityType: "contributor",
			EntityID: &contributorID, Reason: reason, After: map[string]any{"public_code": cur.PublicCode, "had_request": cur.Request != nil}}); err != nil {
			return err
		}
		out, err = s.erasure(ctx, q, contributorID)
		return err
	})
	if err != nil {
		return Erasure{}, err
	}
	s.DeleteQueuedFiles(ctx)
	return out, nil
}

// OpenRequest is a row of the Ops erasure queue.
type OpenRequest struct {
	ID            uuid.UUID `json:"id"`
	ContributorID uuid.UUID `json:"contributor_id"`
	PublicCode    string    `json:"public_code"`
	ByContributor bool      `json:"by_contributor"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListRequests returns open erasure requests, oldest first.
func (s *Service) ListRequests(ctx context.Context) ([]OpenRequest, error) {
	out := []OpenRequest{}
	err := s.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListOpenErasureRequests(ctx)
		for _, r := range rows {
			out = append(out, OpenRequest{ID: r.ID, ContributorID: r.ContributorID, PublicCode: r.PublicCode,
				ByContributor: r.ByContributor, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

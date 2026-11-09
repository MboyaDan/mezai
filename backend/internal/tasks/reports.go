package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ContentReport is a reported item as Ops sees it. The item content itself
// is opened from the project, not copied here.
type ContentReport struct {
	ID          uuid.UUID  `json:"id"`
	Category    string     `json:"category"`
	Note        *string    `json:"note"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	ItemID      uuid.UUID  `json:"item_id"`
	ExternalID  *string    `json:"external_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ProjectName string     `json:"project_name"`
	ReportedBy  string     `json:"reported_by"` // contributor code
}

// ListReports lists content reports, oldest first, optionally by status.
func (s *Service) ListReports(ctx context.Context, p *auth.Principal, status string) ([]ContentReport, error) {
	var filter *string
	switch status {
	case "":
	case "open", "actioned", "dismissed":
		filter = &status
	default:
		return nil, fieldErr("status", "use open, actioned or dismissed")
	}
	out := []ContentReport{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListContentReports(ctx, filter)
		for _, r := range rows {
			out = append(out, ContentReport{ID: r.ID, Category: r.Category, Note: r.Note, Status: r.Status, CreatedAt: r.CreatedAt,
				ResolvedAt: r.ResolvedAt, ItemID: r.ItemID, ExternalID: r.ExternalID, ProjectID: r.ProjectID,
				ProjectName: r.ProjectName, ReportedBy: r.PublicCode})
		}
		return err
	})
	return out, err
}

// ErrReportClosed: the report was already resolved.
var ErrReportClosed = errors.New("tasks: report already resolved")

// ResolveReport dismisses a report (the item goes back into the queue) or
// actions it (the item is withdrawn for good, and retired if it was gold).
func (s *Service) ResolveReport(ctx context.Context, p *auth.Principal, id uuid.UUID, action, reason string, m auth.Meta) error {
	var status string
	switch action {
	case "dismiss":
		status = "dismissed"
	case "withdraw":
		status = "actioned"
	default:
		return fieldErr("action", "use dismiss or withdraw")
	}
	notePtr, err := optionalNote(reason, true)
	if err != nil {
		return fieldErr("reason", "give a reason for the audit log")
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.LockContentReport(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status != "open" {
			return ErrReportClosed
		}
		if err := q.ResolveContentReport(ctx, store.ResolveContentReportParams{ID: id, Status: status, ResolvedBy: &p.UserID}); err != nil {
			return err
		}
		if status == "actioned" {
			if err := q.RejectItem(ctx, r.ItemID); err != nil {
				return err
			}
			if err := q.RetireGoldForItem(ctx, r.ItemID); err != nil {
				return err
			}
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "content.report_" + status, EntityType: "content_report",
			EntityID: &id, OrgID: &r.OrgID, Reason: *notePtr, After: map[string]string{"status": status}})
	})
}

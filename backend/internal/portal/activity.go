package portal

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// clientActions are the audit actions an organisation sees about itself
// (D-124). Work-level events (tasks, reviews, gold, qualification) stay
// staff-only: they would describe contributors or gold checks.
var clientActions = []string{
	"project.created", "project.updated", "project.status_changed",
	"import.committed",
	"deliverable.published", "deliverable.downloaded",
	"project_request.created", "project_request.converted", "project_request.declined",
	"invitation.created", "invitation.accepted", "invitation.revoked",
	"user.role_granted", "user.role_removed",
}

// invoiceActions are shown only to people who may read invoices.
var invoiceActions = []string{"invoice.uploaded", "invoice.paid", "invoice.voided", "invoice.downloaded"}

// ActivityPage is how many entries one request returns.
const ActivityPage = 50

// Actor is who acted, as the organisation may see it: a colleague by name,
// or "MezAI team" for ELS staff and the system (D-124).
type Actor struct {
	Kind string `json:"kind"` // member or mezai
	Name string `json:"name"`
}

// Activity is one entry in the organisation's activity log.
type Activity struct {
	ID          uuid.UUID  `json:"id"`
	At          time.Time  `json:"at"`
	Action      string     `json:"action"`
	Actor       Actor      `json:"actor"`
	ProjectID   *uuid.UUID `json:"project_id"`
	ProjectName *string    `json:"project_name"`
	// Subject names what the action was about when it is not a project: a
	// request title, an invited email, a colleague or an invoice number.
	Subject string `json:"subject"`
	Status  string `json:"status,omitempty"`  // project status changes
	Version string `json:"version,omitempty"` // deliverables
	File    string `json:"file,omitempty"`    // deliverable downloads: data or manifest
	Rows    string `json:"rows,omitempty"`    // item imports
}

// ActivityList is one page, with the cursor for the next.
type ActivityList struct {
	Entries []Activity `json:"entries"`
	Before  *time.Time `json:"before"`
}

// orgFor works out which organisation the caller may read: a client user
// only their own; staff (audit.read) the one they ask for.
func orgFor(p *auth.Principal, asked *uuid.UUID) (uuid.UUID, error) {
	switch {
	case p.Kind == auth.KindClient:
		if asked != nil && *asked != p.OrgID {
			return uuid.Nil, ErrNotFound
		}
		return p.OrgID, nil
	case p.Kind == auth.KindStaff && asked != nil:
		return *asked, nil
	}
	return uuid.Nil, ErrInvalidInput
}

// ListActivity returns the organisation's activity, newest first.
func (s *Service) ListActivity(ctx context.Context, p *auth.Principal, asked *uuid.UUID, before *time.Time) (ActivityList, error) {
	org, err := orgFor(p, asked)
	if err != nil {
		return ActivityList{}, err
	}
	actions := clientActions
	if p.Has("org.invoice.read") || p.Has("invoice.manage") {
		actions = append(append([]string{}, clientActions...), invoiceActions...)
	}
	out := ActivityList{Entries: []Activity{}}
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).OrgActivity(ctx, store.OrgActivityParams{OrgID: &org, Actions: actions, Before: before, Lim: ActivityPage})
		for _, r := range rows {
			a := Activity{ID: r.ID, At: r.CreatedAt, Action: r.Action, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
				Subject: r.Subject, Status: r.Status, Version: r.Version, File: r.File, Rows: r.Rows,
				Actor: Actor{Kind: "mezai", Name: "MezAI team"}}
			if r.ActorIsMember && r.ActorName != nil {
				a.Actor = Actor{Kind: "member", Name: *r.ActorName}
			}
			out.Entries = append(out.Entries, a)
		}
		if len(rows) == ActivityPage {
			last := rows[len(rows)-1].CreatedAt
			out.Before = &last
		}
		return err
	})
	return out, err
}

// Member is one person in the organisation.
type Member struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	JoinedAt    time.Time  `json:"joined_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

// ListMembers returns the people in the caller's organisation (or, for
// staff, the one asked for).
func (s *Service) ListMembers(ctx context.Context, p *auth.Principal, asked *uuid.UUID) ([]Member, error) {
	org, err := orgFor(p, asked)
	if err != nil {
		return nil, err
	}
	out := []Member{}
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).OrgMembers(ctx, org)
		for _, r := range rows {
			out = append(out, Member{ID: r.ID, Name: r.DisplayName, Email: r.Email, Role: r.RoleKey, Status: r.Status,
				JoinedAt: r.JoinedAt, LastLoginAt: r.LastLoginAt})
		}
		return err
	})
	return out, err
}

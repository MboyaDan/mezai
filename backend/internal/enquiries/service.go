// Package enquiries is the Ops inbox for project enquiries from the public
// contact form (brief 6.3 "Enquiries inbox: assign, status, notes" and the
// enquiry states in brief 9). The public form itself is in httpapi.
package enquiries

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

var (
	ErrNotFound = errors.New("enquiries: not found")
	// ErrChanged: someone else changed the enquiry since the caller read it.
	ErrChanged = errors.New("enquiries: changed since read")
	// ErrNotStaff: only staff work the inbox, whatever their grants say.
	ErrNotStaff = errors.New("enquiries: staff only")
)

// FieldError is a 422 with the field at fault.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func fieldErr(field, format string, a ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, a...)}
}

// Statuses in brief 9's order.
var Statuses = []string{"new", "assigned", "in_conversation", "won", "lost", "spam"}

// next lists where each status may go. Won is final (only its organisation
// link may change); lost can be reopened and spam can go back to new, so a
// slip of the hand is never permanent.
var next = map[string][]string{
	"new":             {"assigned", "lost", "spam"},
	"assigned":        {"new", "in_conversation", "lost", "spam"},
	"in_conversation": {"won", "lost", "spam"},
	"won":             {},
	"lost":            {"in_conversation"},
	"spam":            {"new"},
}

// CanMove reports whether an enquiry may go from one status to another.
// Staying put is always allowed (an assignee or link change).
func CanMove(from, to string) bool {
	if from == to {
		_, ok := next[from]
		return ok
	}
	for _, s := range next[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Moves lists the statuses an enquiry may move to from status.
func Moves(status string) []string {
	out := append([]string{}, next[status]...)
	return out
}

type Service struct{ pool *db.Pool }

func NewService(pool *db.Pool) *Service { return &Service{pool: pool} }

type Summary struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	OrganisationName string     `json:"organisation"`
	Email            string     `json:"email"`
	ProjectType      string     `json:"project_type"`
	Volume           string     `json:"volume"`
	Timeline         string     `json:"timeline"`
	Status           string     `json:"status"`
	AssignedTo       *uuid.UUID `json:"assigned_to"`
	AssignedName     *string    `json:"assigned_name"`
	NoteCount        int32      `json:"note_count"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type List struct {
	Enquiries []Summary      `json:"enquiries"`
	Counts    map[string]int `json:"counts"`
}

// List returns up to 200 enquiries, newest first. status is "" (open ones),
// "all" or one status; mine limits it to the caller's.
func (s *Service) List(ctx context.Context, p *auth.Principal, status string, mine bool) (*List, error) {
	if p.Kind != auth.KindStaff {
		return nil, ErrNotStaff
	}
	var filter *string
	if status != "" {
		if status != "all" && !known(status) {
			return nil, fieldErr("status", "use all or one of %s", strings.Join(Statuses, ", "))
		}
		filter = &status
	}
	var assignee *uuid.UUID
	if mine {
		assignee = &p.UserID
	}
	out := &List{Enquiries: []Summary{}, Counts: map[string]int{}}
	for _, st := range Statuses {
		out.Counts[st] = 0
	}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.ListEnquiries(ctx, store.ListEnquiriesParams{Status: filter, AssignedTo: assignee})
		if err != nil {
			return err
		}
		for _, r := range rows {
			out.Enquiries = append(out.Enquiries, Summary{ID: r.ID, Name: r.Name, OrganisationName: r.OrganisationName,
				Email: r.Email, ProjectType: r.ProjectType, Volume: r.VolumeRange, Timeline: r.Timeline, Status: r.Status,
				AssignedTo: r.AssignedTo, AssignedName: r.AssignedName, NoteCount: r.NoteCount,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
		}
		counts, err := q.CountEnquiriesByStatus(ctx)
		for _, c := range counts {
			out.Counts[c.Status] = int(c.N)
		}
		return err
	})
	return out, err
}

type Note struct {
	ID         uuid.UUID `json:"id"`
	Body       string    `json:"body"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Person struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Detail struct {
	Summary
	Languages string     `json:"languages"`
	Message   string     `json:"message"`
	OrgID     *uuid.UUID `json:"org_id"`
	OrgName   *string    `json:"org_name"`
	// Moves are the statuses this enquiry may move to now.
	Moves     []string `json:"moves"`
	Notes     []Note   `json:"notes"`
	Assignees []Person `json:"assignees"`
}

func (s *Service) Get(ctx context.Context, p *auth.Principal, id uuid.UUID) (*Detail, error) {
	if p.Kind != auth.KindStaff {
		return nil, ErrNotStaff
	}
	var out *Detail
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.GetEnquiry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		d := &Detail{Summary: Summary{ID: r.ID, Name: r.Name, OrganisationName: r.OrganisationName, Email: r.Email,
			ProjectType: r.ProjectType, Volume: r.VolumeRange, Timeline: r.Timeline, Status: r.Status,
			AssignedTo: r.AssignedTo, AssignedName: r.AssignedName, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt},
			Languages: r.Languages, Message: r.Message, OrgID: r.OrgID, OrgName: r.OrgName,
			Moves: Moves(r.Status), Notes: []Note{}, Assignees: []Person{}}
		notes, err := q.ListEnquiryNotes(ctx, id)
		if err != nil {
			return err
		}
		for _, n := range notes {
			d.Notes = append(d.Notes, Note{ID: n.ID, Body: n.Body, AuthorID: n.AuthorUserID, AuthorName: n.AuthorName, CreatedAt: n.CreatedAt})
		}
		d.NoteCount = int32(len(d.Notes))
		people, err := q.EnquiryAssignees(ctx)
		for _, u := range people {
			d.Assignees = append(d.Assignees, Person{ID: u.ID, Name: u.DisplayName})
		}
		out = d
		return err
	})
	return out, err
}

// Change is the state the caller wants. It is the whole state, not a patch:
// AssignedTo and OrgID nil mean nobody and no organisation.
type Change struct {
	// ExpectedStatus is the status the caller saw; a different one is ErrChanged.
	ExpectedStatus string     `json:"expected_status"`
	Status         string     `json:"status"`
	AssignedTo     *uuid.UUID `json:"assigned_to"`
	OrgID          *uuid.UUID `json:"org_id"`
}

// Update moves an enquiry and sets its assignee and organisation link.
// Choosing someone for a new enquiry assigns it; an assigned or
// in-conversation enquiry must have someone; only a won one has an
// organisation (the table's check says the same).
func (s *Service) Update(ctx context.Context, p *auth.Principal, id uuid.UUID, c Change, m auth.Meta) error {
	if p.Kind != auth.KindStaff {
		return ErrNotStaff
	}
	if !known(c.Status) {
		return fieldErr("status", "use one of %s", strings.Join(Statuses, ", "))
	}
	if c.Status == "new" && c.AssignedTo != nil {
		c.Status = "assigned"
	}
	if (c.Status == "assigned" || c.Status == "in_conversation") && c.AssignedTo == nil {
		return fieldErr("assigned_to", "choose who handles this enquiry")
	}
	if c.OrgID != nil && c.Status != "won" {
		return fieldErr("org_id", "only a won enquiry is linked to an organisation")
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.LockEnquiry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if c.ExpectedStatus != cur.Status {
			return ErrChanged
		}
		if !CanMove(cur.Status, c.Status) {
			return fieldErr("status", "an enquiry that is %s cannot move to %s", cur.Status, c.Status)
		}
		if c.AssignedTo != nil && !sameID(c.AssignedTo, cur.AssignedTo) {
			people, err := q.EnquiryAssignees(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, u := range people {
				found = found || u.ID == *c.AssignedTo
			}
			if !found {
				return fieldErr("assigned_to", "choose an active staff member who works enquiries")
			}
		}
		if c.OrgID != nil && !sameID(c.OrgID, cur.OrgID) {
			ok, err := q.ActiveOrganisationExists(ctx, *c.OrgID)
			if err != nil {
				return err
			}
			if !ok {
				return fieldErr("org_id", "choose an active organisation")
			}
		}
		if cur.Status == c.Status && sameID(cur.AssignedTo, c.AssignedTo) && sameID(cur.OrgID, c.OrgID) {
			return nil // nothing to change, nothing to audit
		}
		if err := q.UpdateEnquiry(ctx, store.UpdateEnquiryParams{ID: id, Status: c.Status, AssignedTo: c.AssignedTo, OrgID: c.OrgID}); err != nil {
			return err
		}
		// Status and IDs only: the enquirer's details stay in the enquiry row.
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "enquiry.update", EntityType: "enquiry", EntityID: &id,
			Before: state{cur.Status, cur.AssignedTo, cur.OrgID}, After: state{c.Status, c.AssignedTo, c.OrgID}})
	})
}

type state struct {
	Status     string     `json:"status"`
	AssignedTo *uuid.UUID `json:"assigned_to"`
	OrgID      *uuid.UUID `json:"org_id"`
}

const maxNote = 5000

// AddNote adds an internal note. Notes are never shown outside the inbox,
// never emailed and never edited (a correction is another note).
func (s *Service) AddNote(ctx context.Context, p *auth.Principal, id uuid.UUID, body string, m auth.Meta) (*Note, error) {
	if p.Kind != auth.KindStaff {
		return nil, ErrNotStaff
	}
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	switch {
	case body == "":
		return nil, fieldErr("body", "write a note")
	case utf8.RuneCountInString(body) > maxNote:
		return nil, fieldErr("body", "keep a note under %d characters", maxNote)
	case strings.IndexFunc(body, func(c rune) bool { return unicode.IsControl(c) && c != '\n' && c != '\t' }) >= 0:
		return nil, fieldErr("body", "remove control characters")
	}
	var out *Note
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.LockEnquiry(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		row, err := q.AddEnquiryNote(ctx, store.AddEnquiryNoteParams{EnquiryID: id, AuthorUserID: p.UserID, Body: body})
		if err != nil {
			return err
		}
		out = &Note{ID: row.ID, Body: body, AuthorID: p.UserID, AuthorName: p.DisplayName, CreatedAt: row.CreatedAt}
		// The note's text is not copied into the audit log.
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "enquiry.note", EntityType: "enquiry", EntityID: &id,
			After: map[string]string{"note_id": row.ID.String()}})
	})
	return out, err
}

func known(status string) bool {
	_, ok := next[status]
	return ok
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

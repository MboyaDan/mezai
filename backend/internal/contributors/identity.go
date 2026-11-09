package contributors

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// One person, one account (Stage 7c, D-078). A contributor gives an M-Pesa
// number and a National ID number; neither may be on another account. Ops
// verifies the identity (seeing possible duplicates) before the contributor
// can become active, and confirms the M-Pesa registered name at the first
// payout.

// Identity statuses.
const (
	IdentityPending  = "pending"
	IdentityVerified = "verified"
	IdentityRejected = "rejected"
)

// Flag kinds: someone tried to use a number another account holds.
const (
	FlagMpesaInUse      = "mpesa_in_use"
	FlagNationalIDInUse = "national_id_in_use"
)

// The answer to a number already on another account. It confirms the number
// is registered (unavoidable with a uniqueness rule) but names nobody; every
// try is recorded for Ops, and profile saves are rate limited.
const inUseMessage = "this number is already on another MezAI account. If it is yours, contact us through the contact form"

// ContributorRef names another contributor by code only.
type ContributorRef struct {
	ID         uuid.UUID `json:"id"`
	PublicCode string    `json:"public_code"`
	Status     string    `json:"status"`
}

// IdentityFlag is one recorded attempt to reuse a number.
type IdentityFlag struct {
	Kind string `json:"kind"`
	// RaisedByThis: this contributor tried to use the other account's number.
	// False: the other account tried to use this contributor's number.
	RaisedByThis bool           `json:"raised_by_this"`
	Other        ContributorRef `json:"other"`
	At           time.Time      `json:"at"`
}

// IdentityChecks is what Ops sees when verifying a contributor.
type IdentityChecks struct {
	Status             string           `json:"status"`
	Note               *string          `json:"note"`
	DecidedBy          *string          `json:"decided_by"` // display name
	DecidedAt          *time.Time       `json:"decided_at"`
	NationalIDLast3    *string          `json:"national_id_last3"`
	MpesaNameCheckedAt *time.Time       `json:"mpesa_name_checked_at"`
	MpesaNameCheckedBy *string          `json:"mpesa_name_checked_by"`
	Flags              []IdentityFlag   `json:"flags"`
	SameName           []ContributorRef `json:"same_name"` // other accounts with the same name
}

// loadIdentityChecks reads the Ops identity block for one contributor.
func loadIdentityChecks(ctx context.Context, q *store.Queries, id uuid.UUID) (IdentityChecks, error) {
	cp, err := q.GetContributorProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdentityChecks{}, ErrNotFound
	}
	if err != nil {
		return IdentityChecks{}, err
	}
	out := IdentityChecks{Status: cp.IdentityStatus, Note: cp.IdentityNote, DecidedAt: cp.IdentityDecidedAt,
		NationalIDLast3: cp.NationalIDLast3, MpesaNameCheckedAt: cp.MpesaNameCheckedAt,
		Flags: []IdentityFlag{}, SameName: []ContributorRef{}}
	name := func(u *uuid.UUID) (*string, error) {
		if u == nil {
			return nil, nil
		}
		n, err := q.GetUserDisplayNameByID(ctx, *u)
		return &n, err
	}
	if out.DecidedBy, err = name(cp.IdentityDecidedBy); err != nil {
		return out, err
	}
	if out.MpesaNameCheckedBy, err = name(cp.MpesaNameCheckedBy); err != nil {
		return out, err
	}
	flags, err := q.ListIdentityFlags(ctx, id)
	if err != nil {
		return out, err
	}
	for _, f := range flags {
		out.Flags = append(out.Flags, IdentityFlag{Kind: f.Kind, RaisedByThis: f.RaisedByThis, At: f.CreatedAt,
			Other: ContributorRef{ID: f.OtherID, PublicCode: f.OtherPublicCode, Status: f.OtherStatus}})
	}
	same, err := q.ListSameNameContributors(ctx, store.ListSameNameContributorsParams{ID: id, DisplayName: cp.DisplayName})
	if err != nil {
		return out, err
	}
	for _, s := range same {
		out.SameName = append(out.SameName, ContributorRef{ID: s.ID, PublicCode: s.PublicCode, Status: s.Status})
	}
	return out, nil
}

// numberClash records and reports a number another account holds. It runs
// before any write in the profile transaction, so committing it stores only
// the flags.
func numberClash(ctx context.Context, q *store.Queries, m auth.Meta, by uuid.UUID, id uuid.UUID, phone *string, fp []byte) (error, error) {
	type hit struct {
		other       uuid.UUID
		kind, field string
	}
	var hits []hit
	if phone != nil {
		other, err := q.ContributorHoldingPhone(ctx, store.ContributorHoldingPhoneParams{MpesaPhone: phone, ID: id})
		if err == nil {
			hits = append(hits, hit{other, FlagMpesaInUse, "mpesa_phone"})
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if fp != nil {
		other, err := q.ContributorHoldingNationalID(ctx, store.ContributorHoldingNationalIDParams{NationalIDFp: fp, ID: id})
		if err == nil {
			hits = append(hits, hit{other, FlagNationalIDInUse, "national_id"})
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}
	for _, h := range hits {
		if err := q.AddIdentityFlag(ctx, store.AddIdentityFlagParams{ContributorID: id, OtherContributorID: h.other, Kind: h.kind}); err != nil {
			return nil, err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &by, Action: "contributor.identity_flagged", EntityType: "contributor", EntityID: &id,
			After: map[string]any{"kind": h.kind, "other_contributor_id": h.other}}); err != nil {
			return nil, err
		}
	}
	return fieldErr(hits[0].field, inUseMessage), nil
}

// uniqueClash turns a unique violation from two accounts saving the same
// number at the same moment into the same answer numberClash gives.
func uniqueClash(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "contributor_profiles_mpesa_phone_key":
			return fieldErr("mpesa_phone", inUseMessage)
		case "contributor_profiles_national_id_key":
			return fieldErr("national_id", inUseMessage)
		}
	}
	return err
}

// DecideIdentity verifies or rejects a contributor's identity. Verifying
// needs both numbers on the profile, and a note when there are possible
// duplicates; it makes a qualifying contributor with an approved test
// active. Rejecting needs a note, which the contributor sees.
func (s *Service) DecideIdentity(ctx context.Context, p *auth.Principal, id uuid.UUID, decision, note string, m auth.Meta) (Detail, error) {
	note = strings.TrimSpace(note)
	if len(note) > 1000 {
		return Detail{}, fieldErr("note", "keep the note under 1000 characters")
	}
	var status string
	switch decision {
	case "verify":
		status = IdentityVerified
	case "reject":
		status = IdentityRejected
		if note == "" {
			return Detail{}, fieldErr("note", "tell the contributor why")
		}
	default:
		return Detail{}, fieldErr("decision", "use verify or reject")
	}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == IdentityVerified {
			if cp.NationalIDFp == nil || cp.MpesaPhone == nil || cp.MpesaPhoneConfirmedAt == nil {
				return ErrIdentityIncomplete
			}
			checks, err := loadIdentityChecks(ctx, q, id)
			if err != nil {
				return err
			}
			if note == "" && (len(checks.Flags) > 0 || len(checks.SameName) > 0) {
				return fieldErr("note", "say why the possible duplicates are not the same person")
			}
		}
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		if err := q.DecideContributorIdentity(ctx, store.DecideContributorIdentityParams{
			ID: id, IdentityStatus: status, IdentityNote: notePtr, DecidedBy: &p.UserID,
		}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.identity_" + status, EntityType: "contributor",
			EntityID: &id, Reason: note, Before: map[string]string{"identity_status": cp.IdentityStatus},
			After: map[string]string{"identity_status": status}}); err != nil {
			return err
		}
		if status == IdentityVerified && cp.Status == StatusQualifying {
			return activateIfQualified(ctx, q, m, id, &p.UserID, "Identity verified")
		}
		return nil
	})
	if err != nil {
		return Detail{}, err
	}
	return s.GetContributor(ctx, p, id)
}

// activateIfQualified makes a qualifying contributor active once they are
// shortlisted, have passed the interview, hold an approved qualification and
// a verified identity, whichever came last.
func activateIfQualified(ctx context.Context, q *store.Queries, m auth.Meta, id uuid.UUID, by *uuid.UUID, reason string) error {
	cp, err := q.LockContributorProfile(ctx, id)
	if err != nil {
		return err
	}
	// D-100: shortlisted, interview passed, identity verified and a test approved.
	if cp.Status != StatusQualifying || cp.IdentityStatus != IdentityVerified || cp.ScreeningStatus != ScreeningShortlisted {
		return nil
	}
	if passed, err := q.HasPassedInterview(ctx, id); err != nil || !passed {
		return err
	}
	// The core check (D-100) is the qualification activation needs.
	if done, err := hasApprovedCore(ctx, q, id); err != nil || !done {
		return err
	}
	return transition(ctx, q, m, id, StatusQualifying, StatusActive, ViaApproval, by, reason)
}

// RecordMpesaNameCheck records that Ops saw the name M-Pesa shows for the
// contributor's number and it matches the profile name (D-078, done at the
// first payout). A mismatch is recorded by rejecting the identity instead.
// The registered name itself is not stored.
func (s *Service) RecordMpesaNameCheck(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (Detail, error) {
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if cp.MpesaPhone == nil || cp.MpesaPhoneConfirmedAt == nil {
			return ErrIdentityIncomplete
		}
		if err := q.RecordMpesaNameCheck(ctx, store.RecordMpesaNameCheckParams{ID: id, CheckedBy: &p.UserID}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.mpesa_name_checked", EntityType: "contributor", EntityID: &id})
	})
	if err != nil {
		return Detail{}, err
	}
	return s.GetContributor(ctx, p, id)
}

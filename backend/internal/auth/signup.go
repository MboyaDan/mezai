package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/legal"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ErrConsentRequired means the sign-up did not accept the current version of
// every document in legal.ContributorRequired.
var ErrConsentRequired = errors.New("auth: accept the current privacy notice and contributor terms")

// SignUp is a contributor's self sign-up (brief 6.2).
type SignUp struct {
	Email       string
	DisplayName string
	Password    string
	// Consents maps a document key to the version the person accepted.
	Consents map[string]string
}

// SignUpContributor creates an unverified contributor account in status
// "applied", records consent with version, time and IP, and emails a
// verification link. When the email is already registered it changes
// nothing and returns nil, so the response never reveals whether an address
// has an account: an unverified contributor gets a fresh link, and a
// verified owner gets an "you already have an account" email (D-158).
func (s *Service) SignUpContributor(ctx context.Context, in SignUp, m Meta) error {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.DisplayName)
	if !strings.Contains(email, "@") || len(email) > 254 || name == "" || len(name) > 120 {
		return ErrInvalidInput
	}
	for _, d := range legal.ContributorRequired {
		if in.Consents[d] != legal.Current[d] {
			return ErrConsentRequired
		}
	}
	if err := password.Validate(in.Password, email); err != nil {
		return err
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return err
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if u, err := q.GetUserForLogin(ctx, email); err == nil {
			if u.Status != "active" {
				return nil
			}
			if u.EmailVerifiedAt == nil {
				if _, err := q.GetContributorIDByUser(ctx, u.ID); err == nil {
					return s.enqueueVerification(ctx, q, u.ID, u.Email, u.DisplayName)
				}
				return nil
			}
			if u.PasswordHash == nil {
				return nil // invited, not yet accepted: the invitation is the way in
			}
			return s.enqueueAccountExists(ctx, q, u.ID, u.Email, u.DisplayName)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		u, err := q.CreateUser(ctx, store.CreateUserParams{Email: email, DisplayName: name, PasswordHash: &hash})
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return nil // registered by a concurrent request
		}
		if err != nil {
			return err
		}
		cp, accepted, err := newContributor(ctx, q, u.ID, m)
		if err != nil {
			return err
		}
		if err := s.enqueueVerification(ctx, q, u.ID, u.Email, u.DisplayName); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "contributor.signed_up", EntityType: "contributor", EntityID: &cp.ID,
			After: map[string]any{"public_code": cp.PublicCode, "status": cp.Status, "consents": accepted}})
	})
}

// newContributor gives a new user the contributor role, an "applied"
// profile and consent to the current required documents (the caller has
// checked them). Shared by the form sign-up and Google sign-up (D-171).
func newContributor(ctx context.Context, q *store.Queries, userID uuid.UUID, m Meta) (store.ContributorProfile, map[string]string, error) {
	role, err := q.GetRoleByKey(ctx, "contributor")
	if err != nil {
		return store.ContributorProfile{}, nil, err
	}
	if err := q.AddUserRole(ctx, store.AddUserRoleParams{UserID: userID, RoleID: role.ID}); err != nil {
		return store.ContributorProfile{}, nil, err
	}
	cp, err := q.CreateContributorProfile(ctx, store.CreateContributorProfileParams{UserID: userID, Status: "applied"})
	if err != nil {
		return store.ContributorProfile{}, nil, err
	}
	if err := q.AddContributorStatusHistory(ctx, store.AddContributorStatusHistoryParams{
		ContributorID: cp.ID, ToStatus: "applied", ChangedBy: &userID, Reason: "Signed up",
	}); err != nil {
		return store.ContributorProfile{}, nil, err
	}
	accepted := map[string]string{}
	for _, d := range legal.ContributorRequired {
		if err := q.InsertConsent(ctx, store.InsertConsentParams{UserID: userID, Document: d, Version: legal.Current[d], Ip: m.IP}); err != nil {
			return store.ContributorProfile{}, nil, err
		}
		accepted[d] = legal.Current[d]
	}
	return cp, accepted, nil
}

// enqueueAccountExists tells the owner of an address that someone tried to
// sign up with it (D-158). The page answers as for a new address, so nobody
// learns who has an account; the owner hears about it here, at most once a
// day so the form cannot be used to flood an inbox.
func (s *Service) enqueueAccountExists(ctx context.Context, q *store.Queries, userID uuid.UUID, email, name string) error {
	data, err := json.Marshal(map[string]string{
		"Name": name, "Link": s.opt.AppOrigin + "/login", "ResetLink": s.opt.AppOrigin + "/forgot-password",
	})
	if err != nil {
		return err
	}
	key := mail.TemplateAccountExists + ":" + userID.String() + ":" + time.Now().UTC().Format(time.DateOnly)
	return q.EnqueueUserEmail(ctx, store.EnqueueUserEmailParams{ToAddress: email, Template: mail.TemplateAccountExists,
		Data: data, UserID: &userID, DedupeKey: &key})
}

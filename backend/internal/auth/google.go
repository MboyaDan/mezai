package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/googleauth"
	"github.com/MboyaDan/mezai/backend/internal/legal"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Google sign-in for contributors (Stage 29, D-171).
//
// Rules:
//   - Contributors only. Staff and client accounts, and any account with
//     two-step sign-in, keep the password (and code) sign-in.
//   - A Google account already linked signs in to its user.
//   - Otherwise an account with the same email is linked, because Google has
//     verified that the person holds that address. If our account never
//     verified its email, its password is cleared: whoever set it never
//     proved the address (an account made in someone else's name in advance).
//   - Otherwise, from the sign-up page only, a new contributor is created
//     with the consents the person ticked before leaving for Google.
const (
	ProviderGoogle = "google"
	googleStateTTL = 10 * time.Minute
)

// Google intents: which page the person started from.
const (
	GoogleSignIn = "signin"
	GoogleSignUp = "signup"
)

var (
	// ErrGoogleState: the state is unknown, used or older than 10 minutes.
	ErrGoogleState = errors.New("auth: google sign-in expired or already used")
	// ErrGoogleNoAccount: sign-in with a Google account that matches no user.
	ErrGoogleNoAccount = errors.New("auth: no account for this google account")
	// ErrGoogleUsePassword: the account must sign in with its password.
	ErrGoogleUsePassword = errors.New("auth: this account signs in with a password")
)

// GoogleStart is a sign-in on its way to Google.
type GoogleStart struct {
	State, Verifier, Nonce string
}

// GoogleExchange trades the code for verified claims (googleauth.Provider).
type GoogleExchange func(ctx context.Context, verifier, nonce string) (googleauth.Claims, error)

// StartGoogle records a sign-in in progress. For sign-up the person must
// have accepted the current required documents first.
func (s *Service) StartGoogle(ctx context.Context, intent string, consents map[string]string) (GoogleStart, error) {
	if intent != GoogleSignIn && intent != GoogleSignUp {
		return GoogleStart{}, ErrInvalidInput
	}
	keep := map[string]string{}
	if intent == GoogleSignUp {
		for _, d := range legal.ContributorRequired {
			if consents[d] != legal.Current[d] {
				return GoogleStart{}, ErrConsentRequired
			}
			keep[d] = consents[d]
		}
	}
	raw, err := json.Marshal(keep)
	if err != nil {
		return GoogleStart{}, err
	}
	state, stateHash := newToken()
	st := GoogleStart{State: state, Verifier: googleauth.NewVerifier(), Nonce: randomString()}
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		return store.New(tx).CreateOAuthState(ctx, store.CreateOAuthStateParams{StateHash: stateHash, CodeVerifier: st.Verifier,
			Nonce: st.Nonce, Intent: intent, Consents: raw, ExpiresAt: s.now().Add(googleStateTTL)})
	})
	return st, err
}

// TakeGoogleState uses up a sign-in in progress and returns its intent. It
// is separate from FinishGoogle so the caller knows where to send the
// person back even when Google reports an error.
func (s *Service) TakeGoogleState(ctx context.Context, state string) (store.TakeOAuthStateRow, error) {
	var row store.TakeOAuthStateRow
	if state == "" {
		return row, ErrGoogleState
	}
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		var err error
		row, err = store.New(tx).TakeOAuthState(ctx, hashToken(state))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.ExpiresAt.Before(s.now())) {
		return row, ErrGoogleState
	}
	return row, err
}

// FinishGoogle exchanges the code (outside any transaction: it is a network
// call) and signs the person in, linking or creating the account.
func (s *Service) FinishGoogle(ctx context.Context, st store.TakeOAuthStateRow, exchange GoogleExchange, m Meta) (Session, *Principal, error) {
	claims, err := exchange(ctx, st.CodeVerifier, st.Nonce)
	if err != nil {
		return Session{}, nil, err
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return Session{}, nil, googleauth.ErrUnverifiedEmail
	}
	var consents map[string]string
	if err := json.Unmarshal(st.Consents, &consents); err != nil {
		return Session{}, nil, err
	}
	var out Session
	var principal *Principal
	var outErr error
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		after := map[string]any{"method": ProviderGoogle}

		// 1. A linked Google account.
		if u, err := q.GetUserByIdentity(ctx, store.GetUserByIdentityParams{Provider: ProviderGoogle, Subject: claims.Subject}); err == nil {
			p, err := s.googlePrincipal(ctx, q, u.ID, u.Email, u.DisplayName, u.Status, u.TotpEnabledAt != nil, m)
			if refused(err) {
				outErr = err
				return nil
			}
			if err != nil {
				return err
			}
			if err := q.TouchIdentity(ctx, store.TouchIdentityParams{Provider: ProviderGoogle, Subject: claims.Subject, Email: email}); err != nil {
				return err
			}
			out, principal, err = s.startSession(ctx, q, p, m, after)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// 2. An account with the same email: link it.
		if u, err := q.GetUserForLogin(ctx, email); err == nil {
			p, err := s.googlePrincipal(ctx, q, u.ID, u.Email, u.DisplayName, u.Status, u.TotpEnabledAt != nil, m)
			if refused(err) {
				outErr = err
				return nil
			}
			if err != nil {
				return err
			}
			if _, err := q.GetIdentityByUser(ctx, store.GetIdentityByUserParams{UserID: u.ID, Provider: ProviderGoogle}); err == nil {
				// Linked to another Google account already: refuse rather than swap.
				outErr = ErrGoogleUsePassword
				return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "auth.login_failed", EntityType: "user", EntityID: &u.ID,
					Reason: "google account differs from the linked one"})
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			cleared := u.PasswordHash != nil && u.EmailVerifiedAt == nil
			if err := q.ClearUnprovenPassword(ctx, u.ID); err != nil {
				return err
			}
			if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
				return err
			}
			if err := q.CreateIdentity(ctx, store.CreateIdentityParams{UserID: u.ID, Provider: ProviderGoogle, Subject: claims.Subject, Email: email}); err != nil {
				return err
			}
			if err := Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "auth.identity_linked", EntityType: "user", EntityID: &u.ID,
				After: map[string]any{"provider": ProviderGoogle, "password_cleared": cleared}}); err != nil {
				return err
			}
			out, principal, err = s.startSession(ctx, q, p, m, after)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// 3. Nobody: a new contributor, from the sign-up page only.
		if st.Intent != GoogleSignUp {
			outErr = ErrGoogleNoAccount
			return nil
		}
		for _, d := range legal.ContributorRequired {
			if consents[d] != legal.Current[d] {
				outErr = ErrConsentRequired // the documents changed while the person was at Google
				return nil
			}
		}
		u, err := q.CreateUser(ctx, store.CreateUserParams{Email: email, DisplayName: googleName(claims.Name, email)})
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			outErr = ErrGoogleState // the same account signed up at the same moment: try again
			return nil
		}
		if err != nil {
			return err
		}
		cp, accepted, err := newContributor(ctx, q, u.ID, m)
		if err != nil {
			return err
		}
		if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
			return err
		}
		if err := q.CreateIdentity(ctx, store.CreateIdentityParams{UserID: u.ID, Provider: ProviderGoogle, Subject: claims.Subject, Email: email}); err != nil {
			return err
		}
		if err := Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "contributor.signed_up", EntityType: "contributor", EntityID: &cp.ID,
			After: map[string]any{"public_code": cp.PublicCode, "status": cp.Status, "consents": accepted, "method": ProviderGoogle}}); err != nil {
			return err
		}
		p, err := loadPrincipal(ctx, q, u.ID, u.Email, u.DisplayName)
		if err != nil {
			return err
		}
		out, principal, err = s.startSession(ctx, q, p, m, after)
		return err
	})
	if err != nil {
		return Session{}, nil, err
	}
	return out, principal, outErr
}

// googlePrincipal applies the contributor-only rule to an existing user.
// A refusal comes back as ErrInvalidCredentials or ErrGoogleUsePassword
// (see refused); any other error is a failure to roll back.
func (s *Service) googlePrincipal(ctx context.Context, q *store.Queries, userID uuid.UUID, email, name, status string, totp bool, m Meta) (*Principal, error) {
	if status != "active" {
		return nil, ErrInvalidCredentials
	}
	p, err := loadPrincipal(ctx, q, userID, email, name)
	if errors.Is(err, ErrAccountSetup) {
		return nil, ErrGoogleUsePassword
	}
	if err != nil {
		return nil, err
	}
	if p.Kind != KindContributor || totp {
		if err := Audit(ctx, q, m, Entry{Actor: &userID, Action: "auth.login_failed", EntityType: "user", EntityID: &userID,
			Reason: "google sign-in is for contributors without two-step sign-in"}); err != nil {
			return nil, err
		}
		return nil, ErrGoogleUsePassword
	}
	return p, nil
}

// refused tells a refusal (answered to the person) from a failure.
func refused(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrGoogleUsePassword)
}

// googleName is the display name for a new account: Google's name, or the
// part of the address before the @.
func googleName(name, email string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	for utf8.RuneCountInString(name) > 120 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func randomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

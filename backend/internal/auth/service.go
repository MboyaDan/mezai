package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Errors returned to handlers. ErrInvalidCredentials covers unknown email,
// wrong password, locked and disabled accounts alike, so a response never
// reveals whether an email is registered.
var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrEmailNotVerified   = errors.New("auth: email not verified")
	ErrNoSession          = errors.New("auth: no valid session")
	ErrAccountSetup       = errors.New("auth: account has no usable role")
	// ErrSecondFactor: the password was right and the account has two-step
	// sign-in on; Session.Challenge holds the token for CompleteLogin.
	ErrSecondFactor = errors.New("auth: second factor required")
)

// Options are the lockout and session rules.
type Options struct {
	AppOrigin   string // base for links in emails
	SessionTTL  time.Duration
	SessionIdle time.Duration
	MaxFailures int           // failed passwords before lockout
	LockFor     time.Duration // lockout length
	// TOTPKey encrypts two-step secrets (config.IdentityKey). Without it
	// two-step sign-in cannot be switched on.
	TOTPKey []byte
}

// DefaultOptions: 5 failures lock the account for 15 minutes (A-018).
func DefaultOptions(ttl, idle time.Duration) Options {
	return Options{SessionTTL: ttl, SessionIdle: idle, MaxFailures: 5, LockFor: 15 * time.Minute}
}

// Service signs people in and resolves sessions.
type Service struct {
	pool *db.Pool
	opt  Options
	// dummyHash is verified against for unknown emails so a miss takes as long as a hit.
	dummyHash string
	totp      *sealer // nil when Options.TOTPKey is unset
	now       func() time.Time
}

func NewService(pool *db.Pool, opt Options) (*Service, error) {
	h, err := password.Hash("dummy password for timing equalisation")
	if err != nil {
		return nil, err
	}
	svc := &Service{pool: pool, opt: opt, dummyHash: h, now: time.Now}
	if len(opt.TOTPKey) > 0 {
		if svc.totp, err = newSealer(opt.TOTPKey); err != nil {
			return nil, err
		}
	}
	return svc, nil
}

// Session is what a successful login hands back: the cookie value and the
// CSRF token the client must echo on state-changing requests.
type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	// Challenge is set instead of Token when a second factor is needed.
	Challenge string
}

// Login checks the password, applies lockout, and creates a session.
// Auth tables have no row-level security; the system scope only switches to
// the application role.
func (s *Service) Login(ctx context.Context, email, plain string, m Meta) (Session, *Principal, error) {
	var out Session
	var principal *Principal
	var outErr error
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserForLogin(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = password.Verify(plain, s.dummyHash)
			outErr = ErrInvalidCredentials
			return Audit(ctx, q, m, Entry{Action: "auth.login_failed", EntityType: "user", Reason: "unknown email"})
		}
		if err != nil {
			return err
		}
		hash := s.dummyHash
		if u.PasswordHash != nil {
			hash = *u.PasswordHash
		}
		ok, err := password.Verify(plain, hash)
		if err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		locked := u.LockedUntil != nil && u.LockedUntil.After(time.Now())
		switch {
		case u.PasswordHash == nil || u.Status != "active" || locked:
			outErr = ErrInvalidCredentials
			reason := "account disabled or not set up"
			if locked {
				reason = "account locked"
			}
			return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "auth.login_failed", EntityType: "user", EntityID: &u.ID, Reason: reason})
		case !ok:
			res, err := q.RecordFailedLogin(ctx, store.RecordFailedLoginParams{
				ID: u.ID, MaxFailures: int32(s.opt.MaxFailures), LockSeconds: int32(s.opt.LockFor / time.Second),
			})
			if err != nil {
				return err
			}
			outErr = ErrInvalidCredentials
			action, reason := "auth.login_failed", "wrong password"
			if res.LockedUntil != nil && res.FailedLoginCount >= int32(s.opt.MaxFailures) {
				action, reason = "auth.locked", fmt.Sprintf("%d failed attempts", res.FailedLoginCount)
			}
			return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: action, EntityType: "user", EntityID: &u.ID, Reason: reason})
		case u.EmailVerifiedAt == nil:
			outErr = ErrEmailNotVerified
			return nil
		}

		p, err := loadPrincipal(ctx, q, u.ID, u.Email, u.DisplayName)
		if err != nil {
			outErr = err
			return nil
		}
		if u.TotpEnabledAt != nil {
			// Right password, two-step on: a challenge, not a session (Stage 17d).
			token, tokenHash := newToken()
			expires := s.now().Add(ChallengeTTL)
			if err := q.CreateLoginChallenge(ctx, store.CreateLoginChallengeParams{UserID: u.ID, TokenHash: tokenHash, ExpiresAt: expires}); err != nil {
				return err
			}
			out, outErr = Session{Challenge: token, ExpiresAt: expires}, ErrSecondFactor
			return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "auth.password_ok", EntityType: "user", EntityID: &u.ID,
				After: map[string]any{"second_factor": "totp"}})
		}
		out, principal, err = s.startSession(ctx, q, p, m, nil)
		return err
	})
	if err != nil {
		return Session{}, nil, err
	}
	return out, principal, outErr
}

// startSession creates a session for a principal whose sign-in is complete.
func (s *Service) startSession(ctx context.Context, q *store.Queries, p *Principal, m Meta, after any) (Session, *Principal, error) {
	token, tokenHash := newToken()
	csrf := make([]byte, 32)
	_, _ = rand.Read(csrf)
	expires := s.now().Add(s.opt.SessionTTL)
	ua := truncate(m.UserAgent, 300)
	sid, err := q.CreateSession(ctx, store.CreateSessionParams{
		UserID: p.UserID, TokenHash: tokenHash, CsrfSecret: csrf, Ip: m.IP, UserAgent: &ua, ExpiresAt: expires,
	})
	if err != nil {
		return Session{}, nil, err
	}
	if err := q.RecordSuccessfulLogin(ctx, p.UserID); err != nil {
		return Session{}, nil, err
	}
	p.SessionID, p.CSRFSecret = sid, csrf
	if err := Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "auth.login", EntityType: "session", EntityID: &sid, After: after}); err != nil {
		return Session{}, nil, err
	}
	return Session{Token: token, CSRFToken: CSRFToken(csrf), ExpiresAt: expires}, p, nil
}

// Authenticate resolves a session cookie value to a principal.
func (s *Service) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, ErrNoSession
	}
	var p *Principal
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := q.GetActiveSession(ctx, store.GetActiveSessionParams{
			TokenHash: hashToken(token), IdleSeconds: int32(s.opt.SessionIdle / time.Second),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoSession
		}
		if err != nil {
			return err
		}
		if row.Status != "active" {
			return ErrNoSession
		}
		p, err = loadPrincipal(ctx, q, row.UserID, row.Email, row.DisplayName)
		if err != nil {
			return ErrNoSession
		}
		p.SessionID, p.CSRFSecret = row.ID, row.CsrfSecret
		// Refresh the idle clock at most once a minute to keep writes low.
		if time.Since(row.LastSeenAt) > time.Minute {
			return q.TouchSession(ctx, row.ID)
		}
		return nil
	})
	return p, err
}

// Logout revokes the principal's session.
func (s *Service) Logout(ctx context.Context, p *Principal, m Meta) error {
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.RevokeSession(ctx, p.SessionID); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "auth.logout", EntityType: "session", EntityID: &p.SessionID})
	})
}

func loadPrincipal(ctx context.Context, q *store.Queries, userID uuid.UUID, email, name string) (*Principal, error) {
	rows, err := q.ListUserGrants(ctx, userID)
	if err != nil {
		return nil, err
	}
	grants := make([]grant, len(rows))
	for i, r := range rows {
		grants[i] = grant{roleKey: r.RoleKey, roleScope: r.RoleScope, perm: r.PermissionKey, orgID: r.OrgID}
	}
	kind, org, perms, ok := resolveKind(grants)
	if !ok {
		return nil, ErrAccountSetup
	}
	p := &Principal{UserID: userID, Email: email, DisplayName: name, Kind: kind, OrgID: org, perms: perms}
	if kind == KindContributor {
		id, err := q.GetContributorIDByUser(ctx, userID)
		if err != nil {
			return nil, ErrAccountSetup
		}
		p.ContributorID = id
	}
	return p, nil
}

// newToken returns a 256-bit random cookie value and its SHA-256, which is
// all the database keeps.
func newToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, hashToken(t)
}

func hashToken(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

// CSRFToken is the form of the session's CSRF secret sent to the client.
func CSRFToken(secret []byte) string { return base64.RawURLEncoding.EncodeToString(secret) }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

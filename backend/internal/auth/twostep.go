package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Two-step sign-in (Stage 17d, brief 12: optional TOTP for Ops and payout
// roles). Staff accounts may switch it on; it is never required.

// ChallengeTTL is how long the code step of a sign-in stays open, and
// MaxChallengeTries how many codes it accepts before it closes.
const (
	ChallengeTTL      = 5 * time.Minute
	MaxChallengeTries = 5
)

var (
	ErrChallengeClosed = errors.New("auth: sign-in step expired; start again")
	ErrBadCode         = errors.New("auth: code not accepted")
	ErrTOTPUnavailable = errors.New("auth: two-step sign-in is for staff accounts")
	ErrTOTPNotSetUp    = errors.New("auth: two-step sign-in is not configured on this server")
	ErrTOTPEnabled     = errors.New("auth: two-step sign-in is already on")
	ErrTOTPOff         = errors.New("auth: two-step sign-in is off")
	ErrNoPending       = errors.New("auth: start the set-up first")
	ErrWrongPassword   = errors.New("auth: password not accepted")
	ErrSelfReset       = errors.New("auth: turn your own two-step sign-in off from your account page")
	ErrReasonRequired  = errors.New("auth: a reason is required")
)

// CompleteLogin finishes a sign-in with a code from the authenticator app or
// an unused recovery code. Wrong codes count towards the account lockout and
// close the step after MaxChallengeTries.
func (s *Service) CompleteLogin(ctx context.Context, challenge, code string, m Meta) (Session, *Principal, error) {
	if challenge == "" || s.totp == nil {
		return Session{}, nil, ErrChallengeClosed
	}
	var out Session
	var principal *Principal
	var outErr error
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		c, err := q.GetLoginChallenge(ctx, hashToken(challenge))
		if errors.Is(err, pgx.ErrNoRows) {
			outErr = ErrChallengeClosed
			return nil
		}
		if err != nil {
			return err
		}
		now := s.now()
		locked := c.LockedUntil != nil && c.LockedUntil.After(now)
		if now.After(c.ExpiresAt) || c.Attempts >= MaxChallengeTries || c.Status != "active" || locked {
			outErr = ErrChallengeClosed
			return q.DeleteLoginChallenge(ctx, c.ID)
		}
		method, ok, err := s.checkSecondFactor(ctx, q, c.UserID, code, now)
		if err != nil {
			return err
		}
		if !ok {
			outErr = ErrBadCode
			if err := q.CountChallengeAttempt(ctx, c.ID); err != nil {
				return err
			}
			if _, err := q.RecordFailedLogin(ctx, store.RecordFailedLoginParams{ID: c.UserID,
				MaxFailures: int32(s.opt.MaxFailures), LockSeconds: int32(s.opt.LockFor / time.Second)}); err != nil {
				return err
			}
			return Audit(ctx, q, m, Entry{Actor: &c.UserID, Action: "auth.login_failed", EntityType: "user", EntityID: &c.UserID, Reason: "wrong two-step code"})
		}
		if err := q.DeleteLoginChallenge(ctx, c.ID); err != nil {
			return err
		}
		p, err := loadPrincipal(ctx, q, c.UserID, c.Email, c.DisplayName)
		if err != nil {
			outErr = err
			return nil
		}
		out, principal, err = s.startSession(ctx, q, p, m, map[string]any{"second_factor": method})
		return err
	})
	if err != nil {
		return Session{}, nil, err
	}
	return out, principal, outErr
}

// checkSecondFactor accepts a current TOTP code (once) or an unused
// recovery code. The user row is locked, so two requests cannot both use
// the same code.
func (s *Service) checkSecondFactor(ctx context.Context, q *store.Queries, userID uuid.UUID, code string, now time.Time) (string, bool, error) {
	u, err := q.GetUserTOTP(ctx, userID)
	if err != nil {
		return "", false, err
	}
	if u.TotpSecretEnc == nil {
		return "", false, nil
	}
	if norm, ok := normRecovery(code); ok {
		n, err := q.UseRecoveryCode(ctx, store.UseRecoveryCodeParams{UserID: userID, CodeHash: hashRecovery(norm)})
		return "recovery_code", n == 1, err
	}
	secret, err := s.totp.open(u.TotpSecretEnc)
	if err != nil {
		return "", false, err
	}
	var last int64
	if u.TotpLastStep != nil {
		last = *u.TotpLastStep
	}
	step, ok := verifyTOTP(secret, code, now, last)
	if !ok {
		return "", false, nil
	}
	return "totp", true, q.SetTOTPLastStep(ctx, store.SetTOTPLastStepParams{ID: userID, Step: &step})
}

// TOTPStatus is what the account page shows.
type TOTPStatus struct {
	Available         bool       `json:"available"` // staff account on a server with a key
	Enabled           bool       `json:"enabled"`
	EnabledAt         *time.Time `json:"enabled_at"`
	RecoveryCodesLeft int32      `json:"recovery_codes_left"`
}

func (s *Service) available(p *Principal) error {
	if p.Kind != KindStaff {
		return ErrTOTPUnavailable
	}
	if s.totp == nil {
		return ErrTOTPNotSetUp
	}
	return nil
}

// GetTOTPStatus returns the caller's two-step state.
func (s *Service) GetTOTPStatus(ctx context.Context, p *Principal) (TOTPStatus, error) {
	out := TOTPStatus{Available: s.available(p) == nil}
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserTOTP(ctx, p.UserID)
		if err != nil {
			return err
		}
		out.Enabled, out.EnabledAt = u.TotpEnabledAt != nil, u.TotpEnabledAt
		if out.Enabled {
			out.RecoveryCodesLeft, err = q.CountRecoveryCodesLeft(ctx, p.UserID)
		}
		return err
	})
	return out, err
}

// TOTPSetup is shown once while switching two-step on.
type TOTPSetup struct {
	Secret string `json:"secret"` // base32, for typing into the app
	URI    string `json:"uri"`    // otpauth:// link, shown as a QR code
}

// StartTOTP makes a new pending secret. Starting again replaces it.
func (s *Service) StartTOTP(ctx context.Context, p *Principal, m Meta) (TOTPSetup, error) {
	if err := s.available(p); err != nil {
		return TOTPSetup{}, err
	}
	var out TOTPSetup
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserTOTP(ctx, p.UserID)
		if err != nil {
			return err
		}
		if u.TotpEnabledAt != nil {
			return ErrTOTPEnabled
		}
		secret := newTOTPSecret()
		if err := q.SetPendingTOTP(ctx, store.SetPendingTOTPParams{ID: p.UserID, SecretEnc: s.totp.seal(secret)}); err != nil {
			return err
		}
		out = TOTPSetup{Secret: b32.EncodeToString(secret), URI: totpURI(u.Email, secret)}
		return nil
	})
	return out, err
}

// ConfirmTOTP switches two-step on once the app shows a matching code, and
// returns ten recovery codes, shown only this once.
func (s *Service) ConfirmTOTP(ctx context.Context, p *Principal, code string, m Meta) ([]string, error) {
	if err := s.available(p); err != nil {
		return nil, err
	}
	codes := newRecoveryCodes()
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserTOTP(ctx, p.UserID)
		if err != nil {
			return err
		}
		switch {
		case u.TotpEnabledAt != nil:
			return ErrTOTPEnabled
		case u.TotpPendingEnc == nil || u.TotpPendingAt == nil || s.now().Sub(*u.TotpPendingAt) > time.Hour:
			return ErrNoPending
		}
		secret, err := s.totp.open(u.TotpPendingEnc)
		if err != nil {
			return err
		}
		step, ok := verifyTOTP(secret, code, s.now(), 0)
		if !ok {
			return ErrBadCode
		}
		if err := q.EnableTOTP(ctx, store.EnableTOTPParams{ID: p.UserID, Step: &step}); err != nil {
			return err
		}
		if err := q.DeleteRecoveryCodes(ctx, p.UserID); err != nil {
			return err
		}
		for _, c := range codes {
			n, _ := normRecovery(c)
			if err := q.AddRecoveryCode(ctx, store.AddRecoveryCodeParams{UserID: p.UserID, CodeHash: hashRecovery(n)}); err != nil {
				return err
			}
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "auth.totp_enabled", EntityType: "user", EntityID: &p.UserID})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// DisableTOTP switches the caller's two-step off. It needs the password and
// a current code (or a recovery code), so a borrowed open session is not
// enough.
func (s *Service) DisableTOTP(ctx context.Context, p *Principal, plain, code string, m Meta) error {
	var outErr error
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserForLogin(ctx, p.Email)
		if err != nil {
			return err
		}
		if u.TotpEnabledAt == nil {
			outErr = ErrTOTPOff
			return nil
		}
		ok := false
		if u.PasswordHash != nil {
			if ok, err = password.Verify(plain, *u.PasswordHash); err != nil {
				return err
			}
		}
		if !ok {
			outErr = ErrWrongPassword
			return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "auth.totp_disable_failed", EntityType: "user", EntityID: &p.UserID, Reason: "wrong password"})
		}
		if s.totp == nil {
			return ErrTOTPNotSetUp
		}
		_, ok, err = s.checkSecondFactor(ctx, q, p.UserID, code, s.now())
		if err != nil {
			return err
		}
		if !ok {
			outErr = ErrBadCode
			return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "auth.totp_disable_failed", EntityType: "user", EntityID: &p.UserID, Reason: "wrong code"})
		}
		return s.turnOff(ctx, q, p.UserID, m, Entry{Actor: &p.UserID, Action: "auth.totp_disabled", EntityType: "user", EntityID: &p.UserID})
	})
	if err != nil {
		return err
	}
	return outErr
}

// ResetTOTP turns another user's two-step off, for a lost phone
// (`user.manage`). The reason is audited; the person sets it up again.
func (s *Service) ResetTOTP(ctx context.Context, actor *Principal, userID uuid.UUID, reason string, m Meta) error {
	if userID == actor.UserID {
		return ErrSelfReset
	}
	if len(reason) == 0 || len(reason) > 500 {
		return ErrReasonRequired
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserTOTP(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// Only a Super Admin may reset a Super Admin's second factor, as with
		// disabling one (SetUserStatus).
		isSuper, err := q.IsUserSuperAdmin(ctx, userID)
		if err != nil {
			return err
		}
		if isSuper && !actor.Has(PermSystemAdmin) {
			return ErrForbidden
		}
		if u.TotpEnabledAt == nil {
			return ErrTOTPOff
		}
		return s.turnOff(ctx, q, userID, m, Entry{Actor: &actor.UserID, Action: "auth.totp_reset", EntityType: "user", EntityID: &userID, Reason: reason})
	})
}

func (s *Service) turnOff(ctx context.Context, q *store.Queries, userID uuid.UUID, m Meta, e Entry) error {
	if err := q.DisableTOTP(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteUserLoginChallenges(ctx, userID); err != nil {
		return err
	}
	return Audit(ctx, q, m, e)
}

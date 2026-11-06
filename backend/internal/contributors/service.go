// Package contributors runs contributor onboarding: profile, consent,
// qualification tests, Ops approval and the contributor status machine
// (brief 6.2 and 9).
package contributors

import (
	"errors"
	"fmt"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/db"
)

// Errors returned to handlers, which map them to HTTP codes.
var (
	ErrNotFound          = errors.New("contributors: not found")
	ErrInvalidInput      = errors.New("contributors: invalid input")
	ErrNotContributor    = errors.New("contributors: the caller is not a contributor")
	ErrIllegalTransition = errors.New("contributors: status change not allowed")
	ErrProfileIncomplete = errors.New("contributors: complete your profile first")
	ErrNotEligible       = errors.New("contributors: account is paused or removed")
	ErrConsentRequired   = errors.New("contributors: accept the current legal documents first")
	ErrAlreadyQualified  = errors.New("contributors: already qualified on this test")
	ErrRetakeTooSoon     = errors.New("contributors: retake not open yet")
	ErrTestUnavailable   = errors.New("contributors: test is not open")
	ErrAttemptClosed     = errors.New("contributors: attempt is no longer open")
	ErrNotPassed         = errors.New("contributors: only a passed attempt can be approved")
	ErrNoGold            = errors.New("contributors: the project has no active gold items")
	ErrNotShareable      = errors.New("contributors: confirm the gold content may be shown to applicants")
)

// FieldError is an invalid field; errors.Is(err, ErrInvalidInput) holds.
type FieldError struct {
	Field, Message string
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e *FieldError) Unwrap() error { return ErrInvalidInput }

func fieldErr(field, format string, a ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, a...)}
}

// RetakeCooldown is how long after a rejected attempt the same test reopens (A-033).
const RetakeCooldown = 7 * 24 * time.Hour

// Service is the onboarding service.
type Service struct {
	pool *db.Pool
	now  func() time.Time
}

func NewService(pool *db.Pool) *Service { return &Service{pool: pool, now: time.Now} }

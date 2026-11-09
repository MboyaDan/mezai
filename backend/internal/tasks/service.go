// Package tasks is the task engine (Stage 9a, brief 9): which live projects a
// contributor may work on, reserving one item at a time with overlap k and
// gold mixed in, autosaved drafts, idempotent submission with the automatic
// QA checks (D-077), skips and content reports.
//
// Writes run in system scope because they touch rows contributors may not
// write (item counters, gold usage); every query that acts for a contributor
// filters by their ID. Task content is read in the contributor's own scope,
// so row-level security still decides which items they can see.
package tasks

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/quality"
)

// Errors returned to handlers, which map them to HTTP codes.
var (
	ErrNotFound          = errors.New("tasks: not found")
	ErrNotContributor    = errors.New("tasks: the caller is not a contributor")
	ErrNotEligible       = errors.New("tasks: not eligible for this project")
	ErrProjectClosed     = errors.New("tasks: project is not live")
	ErrNoTasks           = errors.New("tasks: no task available right now")
	ErrReservationClosed = errors.New("tasks: the reservation has expired or ended")
	ErrAlreadySubmitted  = errors.New("tasks: this task was already submitted")
	ErrKeyReused         = errors.New("tasks: idempotency key used for another task")
	ErrDraftLimit        = errors.New("tasks: too many saved drafts for this task")
	ErrInvalidInput      = errors.New("tasks: invalid input")
)

// FieldError is an invalid field; errors.Is(err, ErrInvalidInput) holds.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }
func (e *FieldError) Unwrap() error { return ErrInvalidInput }

func fieldErr(field, format string, a ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, a...)}
}

// BlockedError carries the blocking QA check results (422 qa_blocked).
type BlockedError struct{ Issues []quality.Issue }

func (e *BlockedError) Error() string { return "tasks: blocked by QA checks" }

// Service is the task engine.
type Service struct {
	pool *db.Pool
	// rnd decides gold mixing; tests seed it so the gold-rate test is exact.
	rnd func() float64
}

func NewService(pool *db.Pool) *Service { return &Service{pool: pool, rnd: rand.Float64} }

// WithRand returns a copy that draws gold decisions from r (tests).
func (s *Service) WithRand(r func() float64) *Service {
	c := *s
	c.rnd = r
	return &c
}

func contributorOf(p *auth.Principal) (uuid.UUID, error) {
	if p == nil || p.Kind != auth.KindContributor || p.ContributorID == uuid.Nil {
		return uuid.Nil, ErrNotContributor
	}
	return p.ContributorID, nil
}

// Limits.
const (
	maxAnswerBytes = 64 << 10
	maxRevisions   = 500
	maxTimeOnTask  = int64(24 * time.Hour / time.Millisecond)
)

// SkipReasons and ReportCategories are the CHECK lists in migration 00005.
var (
	SkipReasons      = []string{"unclear", "language", "expertise", "technical", "other"}
	ReportCategories = []string{"harmful", "personal_data", "broken", "other"}
)

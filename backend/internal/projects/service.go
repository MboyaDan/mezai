// Package projects runs the workspace side of projects (brief 6.3, Stage 8):
// client organisations, projects and their lifecycle, item and gold imports,
// gold sets, and client project requests.
package projects

import (
	"errors"
	"fmt"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/storage"
)

// Errors returned to handlers, which map them to HTTP codes.
var (
	ErrNotFound          = errors.New("projects: not found")
	ErrInvalidInput      = errors.New("projects: invalid input")
	ErrForbidden         = errors.New("projects: not allowed for this account")
	ErrSlugTaken         = errors.New("projects: organisation slug already used")
	ErrIllegalTransition = errors.New("projects: status change not allowed")
	ErrNotReady          = errors.New("projects: project is not ready for this step")
	ErrLocked            = errors.New("projects: setting is fixed at this stage")
	ErrClosed            = errors.New("projects: project is closed")
	ErrImportNotOpen     = errors.New("projects: import cannot be committed")
	ErrImportStale       = errors.New("projects: project changed since the check")
	ErrFileChanged       = errors.New("projects: stored file does not match the checked file")
	ErrAlreadyDecided    = errors.New("projects: request already decided")
	ErrStorage           = errors.New("projects: file storage unavailable")
)

// FieldError is an invalid field; errors.Is(err, ErrInvalidInput) holds.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }
func (e *FieldError) Unwrap() error { return ErrInvalidInput }

func fieldErr(field, format string, a ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, a...)}
}

// fromQuality turns a task-type validation error into a field error.
func fromQuality(err error) error {
	var iv *quality.Invalid
	if errors.As(err, &iv) {
		return &FieldError{Field: iv.Field, Message: iv.Message}
	}
	return err
}

// NotReadyError lists what stops a project moving on; errors.Is(err, ErrNotReady) holds.
type NotReadyError struct{ Missing []string }

func (e *NotReadyError) Error() string { return fmt.Sprintf("not ready: %v", e.Missing) }
func (e *NotReadyError) Unwrap() error { return ErrNotReady }

// Options configure the service.
type Options struct {
	// NotifyTo receives an email for every project request.
	NotifyTo string
	// AppOrigin builds links in emails.
	AppOrigin string
}

// Service is the projects service.
type Service struct {
	pool  *db.Pool
	blobs storage.Blobs
	opts  Options
	now   func() time.Time
}

func NewService(pool *db.Pool, blobs storage.Blobs, o Options) *Service {
	return &Service{pool: pool, blobs: blobs, opts: o, now: time.Now}
}

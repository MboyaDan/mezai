package contributors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Screening funnel, part 1 (Stage 7d-1, D-100): a short experience form with
// an optional CV, an Ops screening decision, and one structured phone
// interview. Activation needs all of them plus a verified identity and an
// approved qualification test.

// Screening statuses.
const (
	ScreeningPending     = "pending"
	ScreeningShortlisted = "shortlisted"
	ScreeningWaitlisted  = "waitlisted"
	ScreeningDeclined    = "declined"
)

// Errors for the funnel.
var (
	ErrNotShortlisted   = errors.New("contributors: not shortlisted yet")
	ErrInterviewTooSoon = errors.New("contributors: a failed interview can be repeated after 30 days")
	ErrAlreadyPassed    = errors.New("contributors: the interview is already passed")
	ErrNoCV             = errors.New("contributors: no CV on file")
)

// MaxCVBytes bounds an uploaded CV (D-100: PDF, 2 MB).
const MaxCVBytes = 2 << 20

// InterviewRetry is how long after a failed interview another may be held.
const InterviewRetry = 30 * 24 * time.Hour

// Experience is the short application form (D-100 section 3.1).
type Experience struct {
	HighestStudy       string `json:"highest_study"`       // required, ≤ 200
	RelevantExperience string `json:"relevant_experience"` // required, ≤ 1000
	AIDataExperience   string `json:"ai_data_experience"`  // optional, ≤ 500
	Why                string `json:"why"`                 // required, ≤ 800 (about 100 words)
}

func (e *Experience) validate() error {
	trim := func(s *string) { *s = strings.TrimSpace(*s) }
	trim(&e.HighestStudy)
	trim(&e.RelevantExperience)
	trim(&e.AIDataExperience)
	trim(&e.Why)
	for _, f := range []struct {
		field, v string
		max      int
	}{
		{"experience.highest_study", e.HighestStudy, 200},
		{"experience.relevant_experience", e.RelevantExperience, 1000},
		{"experience.ai_data_experience", e.AIDataExperience, 500},
		{"experience.why", e.Why, 800},
	} {
		if utf8.RuneCountInString(f.v) > f.max {
			return fieldErr(f.field, "keep this under %d characters", f.max)
		}
	}
	return nil
}

func (e Experience) complete() bool {
	return e.HighestStudy != "" && e.RelevantExperience != "" && e.Why != ""
}

var linkedinRe = regexp.MustCompile(`^https://([a-z]{2,3}\.)?linkedin\.com/in/[A-Za-z0-9_%-]{2,100}/?$`)

func normLinkedIn(s *string) (*string, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	v := strings.TrimSpace(*s)
	if !linkedinRe.MatchString(v) {
		return nil, fieldErr("linkedin_url", "use your profile link, for example https://www.linkedin.com/in/your-name")
	}
	return &v, nil
}

// Application is what Ops reads when screening.
type Application struct {
	Experience         Experience `json:"experience"`
	LinkedInURL        *string    `json:"linkedin_url"`
	CVFileName         *string    `json:"cv_file_name"`
	CVUploadedAt       *time.Time `json:"cv_uploaded_at"`
	CVDeleteAfter      *time.Time `json:"cv_delete_after"`
	ScreeningStatus    string     `json:"screening_status"`
	ScreeningNote      *string    `json:"screening_note"`
	ScreeningDecidedAt *time.Time `json:"screening_decided_at"`
}

// Interview is one recorded call.
type Interview struct {
	ID                uuid.UUID      `json:"id"`
	Outcome           string         `json:"outcome"`
	IdentityConfirmed bool           `json:"identity_confirmed"`
	LanguageScores    map[string]int `json:"language_scores"`
	ScenarioScore     *int16         `json:"scenario_score"`
	ConfidentialityOK *bool          `json:"confidentiality_ok"`
	Availability      *string        `json:"availability"`
	Note              *string        `json:"note"`
	Interviewer       string         `json:"interviewer"`
	At                time.Time      `json:"at"`
}

func applicationOf(cp store.GetContributorProfileRow) Application {
	a := Application{LinkedInURL: cp.LinkedinUrl, CVFileName: cp.CvFileName, CVUploadedAt: cp.CvUploadedAt,
		CVDeleteAfter: cp.CvDeleteAfter, ScreeningStatus: cp.ScreeningStatus, ScreeningNote: cp.ScreeningNote,
		ScreeningDecidedAt: cp.ScreeningDecidedAt}
	_ = json.Unmarshal(cp.Experience, &a.Experience)
	return a
}

func loadInterviews(ctx context.Context, q *store.Queries, id uuid.UUID) ([]Interview, error) {
	rows, err := q.ListInterviews(ctx, id)
	out := make([]Interview, 0, len(rows))
	for _, r := range rows {
		iv := Interview{ID: r.ID, Outcome: r.Outcome, IdentityConfirmed: r.IdentityConfirmed, ScenarioScore: r.ScenarioScore,
			ConfidentialityOK: r.ConfidentialityOk, Availability: r.Availability, Note: r.Note, Interviewer: r.InterviewerName,
			At: r.CreatedAt, LanguageScores: map[string]int{}}
		_ = json.Unmarshal(r.LanguageScores, &iv.LanguageScores)
		out = append(out, iv)
	}
	return out, err
}

// cvKey builds the object key from IDs only (no file name, D-079 pattern).
func cvKey(contributor uuid.UUID) string {
	return path.Join("cv", contributor.String(), uuid.NewString()+".pdf")
}

// UploadCV stores the caller's CV (PDF, at most 2 MB), replacing any earlier one.
func (s *Service) UploadCV(ctx context.Context, p *auth.Principal, name string, data []byte, m auth.Meta) (Profile, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Profile{}, err
	}
	if s.blobs == nil {
		return Profile{}, errors.New("contributors: file storage not configured")
	}
	if len(data) == 0 || len(data) > MaxCVBytes {
		return Profile{}, fieldErr("cv", "upload a PDF of up to 2 MB")
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !strings.EqualFold(path.Ext(name), ".pdf") {
		return Profile{}, fieldErr("cv", "upload your CV as a PDF")
	}
	name = strings.TrimSpace(path.Base(name))
	if utf8.RuneCountInString(name) > 120 {
		name = "cv.pdf"
	}
	key := cvKey(id)
	if err := s.blobs.Put(ctx, key, "application/pdf", data); err != nil {
		return Profile{}, err
	}
	var old *string
	var out Profile
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id)
		if err != nil {
			return err
		}
		if cp.Status == StatusRemoved || cp.ScreeningStatus == ScreeningDeclined {
			return ErrNotEligible
		}
		old = cp.CvFileKey
		if err := q.SetContributorCV(ctx, store.SetContributorCVParams{ID: id, CvFileKey: &key, CvFileName: &name}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.cv_uploaded", EntityType: "contributor",
			EntityID: &id, After: map[string]any{"bytes": len(data)}}); err != nil {
			return err
		}
		out, err = loadProfile(ctx, q, id)
		return err
	})
	if err != nil {
		_ = s.blobs.Delete(ctx, key) // nothing points at it
		return Profile{}, err
	}
	if old != nil {
		_ = s.blobs.Delete(ctx, *old)
	}
	return out, nil
}

// DeleteMyCV removes the caller's CV now.
func (s *Service) DeleteMyCV(ctx context.Context, p *auth.Principal, m auth.Meta) (Profile, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Profile{}, err
	}
	var out Profile
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := s.dropCV(ctx, q, id); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.cv_deleted", EntityType: "contributor", EntityID: &id}); err != nil {
			return err
		}
		out, err = loadProfile(ctx, q, id)
		return err
	})
	return out, err
}

// dropCV deletes the stored file and clears the columns. Storage first: if
// it fails, the row still points at the file and the sweep retries.
func (s *Service) dropCV(ctx context.Context, q *store.Queries, id uuid.UUID) error {
	cp, err := q.LockContributorProfile(ctx, id)
	if err != nil {
		return err
	}
	if cp.CvFileKey == nil {
		return nil
	}
	if s.blobs != nil {
		if err := s.blobs.Delete(ctx, *cp.CvFileKey); err != nil {
			return err
		}
	}
	return q.ClearContributorCV(ctx, id)
}

// GetCV returns a contributor's CV for Ops.
func (s *Service) GetCV(ctx context.Context, p *auth.Principal, id uuid.UUID) (string, []byte, error) {
	var key, name string
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		cp, err := store.New(tx).GetContributorProfile(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if cp.CvFileKey == nil {
			return ErrNoCV
		}
		key, name = *cp.CvFileKey, *cp.CvFileName
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	data, err := s.blobs.Get(ctx, key, MaxCVBytes)
	return name, data, err
}

// DecideScreening shortlists, waitlists or declines an application. A
// decline needs a note (the candidate sees it) and deletes any CV at once;
// other decisions keep a CV for 90 days.
func (s *Service) DecideScreening(ctx context.Context, p *auth.Principal, id uuid.UUID, decision, note string, m auth.Meta) (Detail, error) {
	note = strings.TrimSpace(note)
	if len(note) > 1000 {
		return Detail{}, fieldErr("note", "keep the note under 1000 characters")
	}
	status := map[string]string{"shortlist": ScreeningShortlisted, "waitlist": ScreeningWaitlisted, "decline": ScreeningDeclined}[decision]
	if status == "" {
		return Detail{}, fieldErr("decision", "use shortlist, waitlist or decline")
	}
	if status == ScreeningDeclined && note == "" {
		return Detail{}, fieldErr("note", "tell the applicant why")
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
		if cp.Status != StatusQualifying {
			return ErrIllegalTransition // only complete applications not yet active
		}
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		if err := q.DecideScreening(ctx, store.DecideScreeningParams{ID: id, ScreeningStatus: status, ScreeningNote: notePtr, DecidedBy: &p.UserID}); err != nil {
			return err
		}
		if status == ScreeningDeclined {
			if err := s.dropCV(ctx, q, id); err != nil {
				return err
			}
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.screening_" + status, EntityType: "contributor",
			EntityID: &id, Reason: note, Before: map[string]string{"screening_status": cp.ScreeningStatus},
			After: map[string]string{"screening_status": status}})
	})
	if err != nil {
		return Detail{}, err
	}
	return s.GetContributor(ctx, p, id)
}

// InterviewInput is one call's record. Outcome is "completed" (the API works
// out pass or fail from the rubric) or "no_answer".
type InterviewInput struct {
	Outcome           string         `json:"outcome"`
	IdentityConfirmed bool           `json:"identity_confirmed"`
	LanguageScores    map[string]int `json:"language_scores"`
	ScenarioScore     int            `json:"scenario_score"`
	ConfidentialityOK bool           `json:"confidentiality_ok"`
	Availability      string         `json:"availability"`
	Note              string         `json:"note"`
}

// InterviewPasses is the fixed rubric (D-102): identity and confidentiality
// confirmed, the scenario and every language scored 3 or 4 out of 4.
func InterviewPasses(in InterviewInput) bool {
	if !in.IdentityConfirmed || !in.ConfidentialityOK || in.ScenarioScore < 3 || len(in.LanguageScores) == 0 {
		return false
	}
	for _, v := range in.LanguageScores {
		if v < 3 {
			return false
		}
	}
	return true
}

// RecordInterview stores one call for a shortlisted applicant. A pass may
// make them active (with a verified identity and an approved test).
func (s *Service) RecordInterview(ctx context.Context, p *auth.Principal, id uuid.UUID, in InterviewInput, m auth.Meta) (Detail, error) {
	in.Availability, in.Note = strings.TrimSpace(in.Availability), strings.TrimSpace(in.Note)
	if len(in.Availability) > 300 || len(in.Note) > 1000 {
		return Detail{}, fieldErr("note", "keep the notes short")
	}
	var outcome string
	switch in.Outcome {
	case "no_answer":
		outcome = "no_answer"
	case "completed":
		if in.ScenarioScore < 1 || in.ScenarioScore > 4 {
			return Detail{}, fieldErr("scenario_score", "score the scenario from 1 to 4")
		}
		if len(in.LanguageScores) == 0 {
			return Detail{}, fieldErr("language_scores", "score at least one language")
		}
		for _, v := range in.LanguageScores {
			if v < 1 || v > 4 {
				return Detail{}, fieldErr("language_scores", "score each language from 1 to 4")
			}
		}
		outcome = "failed"
		if InterviewPasses(in) {
			outcome = "passed"
		}
	default:
		return Detail{}, fieldErr("outcome", "use completed or no_answer")
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
		if cp.ScreeningStatus != ScreeningShortlisted {
			return ErrNotShortlisted
		}
		langs, err := q.ListContributorLanguages(ctx, id)
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for _, l := range langs {
			known[l.Language] = true
		}
		for code := range in.LanguageScores {
			if !known[code] {
				return fieldErr("language_scores", "score only languages on the profile")
			}
		}
		past, err := loadInterviews(ctx, q, id)
		if err != nil {
			return err
		}
		for _, iv := range past {
			if iv.Outcome == "passed" {
				return ErrAlreadyPassed
			}
		}
		if len(past) > 0 && past[0].Outcome == "failed" && s.now().Sub(past[0].At) < InterviewRetry {
			return ErrInterviewTooSoon
		}
		row := store.AddInterviewParams{ContributorID: id, InterviewerID: p.UserID, Outcome: outcome, IdentityConfirmed: in.IdentityConfirmed,
			LanguageScores: []byte(`{}`)}
		if outcome != "no_answer" {
			row.LanguageScores, _ = json.Marshal(in.LanguageScores)
			sc := int16(in.ScenarioScore)
			row.ScenarioScore, row.ConfidentialityOk = &sc, &in.ConfidentialityOK
		}
		if in.Availability != "" {
			row.Availability = &in.Availability
		}
		if in.Note != "" {
			row.Note = &in.Note
		}
		iv, err := q.AddInterview(ctx, row)
		if err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.interview_" + outcome, EntityType: "contributor",
			EntityID: &id, After: map[string]any{"interview_id": iv.ID}}); err != nil {
			return err
		}
		if outcome == "passed" && cp.Status == StatusQualifying {
			return activateIfQualified(ctx, q, m, id, &p.UserID, "Interview passed")
		}
		return nil
	})
	if err != nil {
		return Detail{}, err
	}
	return s.GetContributor(ctx, p, id)
}

// SweepCVs deletes CVs whose 90 days have passed. The API runs it hourly.
func (s *Service) SweepCVs(ctx context.Context, log *slog.Logger) (int, error) {
	n := 0
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.ListExpiredCVs(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := s.dropCV(ctx, q, r.ID); err != nil {
				return fmt.Errorf("cv %s: %w", r.ID, err)
			}
			if err := auth.Audit(ctx, q, auth.Meta{}, auth.Entry{Action: "contributor.cv_expired", EntityType: "contributor", EntityID: &r.ID}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil && log != nil {
		log.Error("cv sweep failed", slog.Any("error", err))
	}
	return n, err
}

// RunCVSweep runs SweepCVs every interval until ctx ends.
func (s *Service) RunCVSweep(ctx context.Context, every time.Duration, log *slog.Logger) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if n, err := s.SweepCVs(ctx, log); err == nil && n > 0 && log != nil {
			log.Info("deleted expired CVs", slog.Int("count", n))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

package contributors

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/pii"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Ops views. Callers are staff; routes check permission keys and the
// transactions run in the caller's (staff) scope.

// Summary is one contributor in the Ops list. The phone number is masked.
type Summary struct {
	ID               uuid.UUID `json:"id"`
	PublicCode       string    `json:"public_code"`
	Status           string    `json:"status"`
	DisplayName      string    `json:"display_name"`
	Email            string    `json:"email"`
	Country          *string   `json:"country"`
	Languages        []string  `json:"languages"`
	MpesaPhoneMasked string    `json:"mpesa_phone_masked"`
	CreatedAt        time.Time `json:"created_at"`
}

// ListContributors lists contributors, newest first, optionally by status.
func (s *Service) ListContributors(ctx context.Context, p *auth.Principal, status string) ([]Summary, error) {
	var filter *string
	if status != "" {
		if !validStatus(status) {
			return nil, fieldErr("status", "unknown status")
		}
		filter = &status
	}
	out := []Summary{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListContributors(ctx, filter)
		for _, r := range rows {
			sm := Summary{ID: r.ID, PublicCode: r.PublicCode, Status: r.Status, DisplayName: r.DisplayName, Email: r.Email,
				Country: r.Country, Languages: r.Languages, CreatedAt: r.CreatedAt}
			if sm.Languages == nil {
				sm.Languages = []string{}
			}
			if r.MpesaPhone != nil {
				sm.MpesaPhoneMasked = pii.MaskPhone(*r.MpesaPhone)
			}
			out = append(out, sm)
		}
		return err
	})
	return out, err
}

// StatusChange is one row of a contributor's status history.
type StatusChange struct {
	From      *string   `json:"from"`
	To        string    `json:"to"`
	Reason    string    `json:"reason"`
	ChangedBy *string   `json:"changed_by"` // display name; null for the system
	At        time.Time `json:"at"`
}

// Detail is a contributor's profile with status history and attempts, for Ops.
type Detail struct {
	Profile  Profile          `json:"profile"`
	History  []StatusChange   `json:"history"`
	Attempts []AttemptSummary `json:"attempts"`
}

// GetContributor returns one contributor's detail.
func (s *Service) GetContributor(ctx context.Context, p *auth.Principal, id uuid.UUID) (Detail, error) {
	var out Detail
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if out.Profile, err = loadProfile(ctx, q, id); err != nil {
			return err
		}
		hist, err := q.ListContributorStatusHistory(ctx, id)
		if err != nil {
			return err
		}
		out.History = make([]StatusChange, len(hist))
		for i, h := range hist {
			out.History[i] = StatusChange{From: h.FromStatus, To: h.ToStatus, Reason: h.Reason, ChangedBy: h.ChangedByName, At: h.CreatedAt}
		}
		attempts, err := q.ListAttemptsByContributor(ctx, id)
		if err != nil {
			return err
		}
		out.Attempts = []AttemptSummary{}
		for _, a := range attempts {
			t, err := q.GetQualificationTest(ctx, a.TestID)
			if err != nil {
				return err
			}
			out.Attempts = append(out.Attempts, AttemptSummary{ID: a.ID, Status: a.Status, Score: a.Score, Passed: a.Passed,
				SubmittedAt: a.SubmittedAt, DecidedAt: a.DecidedAt, DecisionNote: a.DecisionNote,
				TestID: t.ID, TestTitle: t.Title, PassThreshold: t.PassThreshold, SkillKind: t.SkillKind, SkillValue: t.SkillValue,
				ContributorID: id, PublicCode: out.Profile.PublicCode, ContributorStatus: out.Profile.Status, DisplayName: out.Profile.DisplayName})
		}
		return nil
	})
	return out, err
}

// SetStatus pauses, reinstates or removes a contributor by hand. Removing
// also signs them out everywhere. The reason is required and audited.
func (s *Service) SetStatus(ctx context.Context, p *auth.Principal, id uuid.UUID, to, reason string, m auth.Meta) (Profile, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 1000 {
		return Profile{}, fieldErr("reason", "give a reason of up to 1000 characters")
	}
	if !validStatus(to) {
		return Profile{}, fieldErr("status", "unknown status")
	}
	var out Profile
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := transition(ctx, q, m, id, cp.Status, to, ViaOps, &p.UserID, reason); err != nil {
			return err
		}
		if to == StatusRemoved {
			if err := q.RevokeUserSessions(ctx, cp.UserID); err != nil {
				return err
			}
		}
		out, err = loadProfile(ctx, q, id)
		return err
	})
	return out, err
}

// AttemptSummary is one attempt in the Ops queue or a contributor's history.
type AttemptSummary struct {
	ID                uuid.UUID  `json:"id"`
	Status            string     `json:"status"`
	Score             *float64   `json:"score"`
	Passed            *bool      `json:"passed"`
	SubmittedAt       *time.Time `json:"submitted_at"`
	DecidedAt         *time.Time `json:"decided_at"`
	DecisionNote      *string    `json:"decision_note"`
	TestID            uuid.UUID  `json:"test_id"`
	TestTitle         string     `json:"test_title"`
	PassThreshold     float64    `json:"pass_threshold"`
	SkillKind         string     `json:"skill_kind"`
	SkillValue        string     `json:"skill_value"`
	ContributorID     uuid.UUID  `json:"contributor_id"`
	PublicCode        string     `json:"public_code"`
	ContributorStatus string     `json:"contributor_status"`
	DisplayName       string     `json:"display_name"`
}

// ListAttempts returns attempts in one status; "scored" is the Ops queue.
func (s *Service) ListAttempts(ctx context.Context, p *auth.Principal, status string) ([]AttemptSummary, error) {
	if status == "" {
		status = AttemptScored
	}
	if !slices.Contains([]string{AttemptScored, AttemptApproved, AttemptRejected}, status) {
		return nil, fieldErr("status", "use scored, approved or rejected")
	}
	out := []AttemptSummary{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListAttemptsForReview(ctx, status)
		for _, r := range rows {
			out = append(out, AttemptSummary(r))
		}
		return err
	})
	return out, err
}

// ReviewQuestion is a question with the expected answer, the contributor's
// answer and whether it matched.
type ReviewQuestion struct {
	ID       uuid.UUID       `json:"id"`
	Position int32           `json:"position"`
	Content  json.RawMessage `json:"content"`
	Expected json.RawMessage `json:"expected"`
	Answer   json.RawMessage `json:"answer"`
	Correct  *bool           `json:"correct"`
}

// AttemptReview is everything Ops needs to decide on an attempt.
type AttemptReview struct {
	ID            uuid.UUID        `json:"id"`
	Status        string           `json:"status"`
	Score         *float64         `json:"score"`
	Passed        *bool            `json:"passed"`
	SubmittedAt   *time.Time       `json:"submitted_at"`
	DecidedAt     *time.Time       `json:"decided_at"`
	DecisionNote  *string          `json:"decision_note"`
	TestID        uuid.UUID        `json:"test_id"`
	TestTitle     string           `json:"test_title"`
	TaskType      string           `json:"task_type"`
	Config        json.RawMessage  `json:"config"`
	PassThreshold float64          `json:"pass_threshold"`
	Contributor   Summary          `json:"contributor"`
	Questions     []ReviewQuestion `json:"questions"`
}

// ReviewAttempt returns one attempt with expected answers, for Ops.
func (s *Service) ReviewAttempt(ctx context.Context, p *auth.Principal, id uuid.UUID) (AttemptReview, error) {
	var out AttemptReview
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.GetQualificationAttempt(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		v, err := attemptView(ctx, q, a)
		if err != nil {
			return err
		}
		cp, err := q.GetContributorProfile(ctx, a.ContributorID)
		if err != nil {
			return err
		}
		out = AttemptReview{ID: a.ID, Status: a.Status, Score: a.Score, Passed: a.Passed, SubmittedAt: a.SubmittedAt,
			DecidedAt: a.DecidedAt, DecisionNote: a.DecisionNote, TestID: v.TestID, TestTitle: v.Title, TaskType: v.TaskType,
			Config: v.Config, PassThreshold: v.PassThreshold,
			Contributor: Summary{ID: cp.ID, PublicCode: cp.PublicCode, Status: cp.Status, DisplayName: cp.DisplayName,
				Email: cp.Email, Country: cp.Country, Languages: []string{}, CreatedAt: cp.CreatedAt},
			Questions: make([]ReviewQuestion, len(v.Questions))}
		for i, qq := range v.Questions {
			rd := v.review[qq.ID.String()]
			out.Questions[i] = ReviewQuestion{ID: qq.ID, Position: qq.Position, Content: qq.Content, Expected: rd.expected,
				Answer: v.Answers[qq.ID.String()], Correct: rd.correct}
		}
		return nil
	})
	return out, err
}

// Decide approves or rejects a scored attempt. Only a passed attempt can be
// approved; approving makes a qualifying contributor active. A rejection
// needs a note, which the contributor sees.
func (s *Service) Decide(ctx context.Context, p *auth.Principal, id uuid.UUID, decision, note string, m auth.Meta) (AttemptReview, error) {
	note = strings.TrimSpace(note)
	if len(note) > 1000 {
		return AttemptReview{}, fieldErr("note", "keep the note under 1000 characters")
	}
	var status string
	switch decision {
	case "approve":
		status = AttemptApproved
	case "reject":
		status = AttemptRejected
		if note == "" {
			return AttemptReview{}, fieldErr("note", "tell the contributor why")
		}
	default:
		return AttemptReview{}, fieldErr("decision", "use approve or reject")
	}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.LockQualificationAttempt(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if a.Status != AttemptScored {
			return ErrAttemptClosed
		}
		if status == AttemptApproved && (a.Passed == nil || !*a.Passed) {
			return ErrNotPassed
		}
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		if _, err := q.DecideAttempt(ctx, store.DecideAttemptParams{ID: id, Status: status, DecidedBy: &p.UserID, DecisionNote: notePtr}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.attempt_" + status, EntityType: "qualification_attempt",
			EntityID: &id, Reason: note, After: map[string]any{"status": status, "score": a.Score}}); err != nil {
			return err
		}
		if status == AttemptApproved {
			cp, err := q.LockContributorProfile(ctx, a.ContributorID)
			if err != nil {
				return err
			}
			if cp.Status == StatusQualifying {
				t, err := q.GetQualificationTest(ctx, a.TestID)
				if err != nil {
					return err
				}
				return transition(ctx, q, m, cp.ID, StatusQualifying, StatusActive, ViaApproval, &p.UserID, "Qualified: "+t.Title)
			}
		}
		return nil
	})
	if err != nil {
		return AttemptReview{}, err
	}
	return s.ReviewAttempt(ctx, p, id)
}

// Test is a qualification test as Ops sees it.
type Test struct {
	ID              uuid.UUID       `json:"id"`
	Title           string          `json:"title"`
	SkillKind       string          `json:"skill_kind"`
	SkillValue      string          `json:"skill_value"`
	TaskType        string          `json:"task_type"`
	PassThreshold   float64         `json:"pass_threshold"`
	InstructionsMd  string          `json:"instructions_md"`
	Status          string          `json:"status"`
	Config          json.RawMessage `json:"config"`
	SourceProjectID *uuid.UUID      `json:"source_project_id"`
	QuestionCount   int32           `json:"question_count"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ListTests lists every qualification test.
func (s *Service) ListTests(ctx context.Context, p *auth.Principal) ([]Test, error) {
	out := []Test{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListQualificationTests(ctx, nil)
		for _, r := range rows {
			out = append(out, Test{ID: r.ID, Title: r.Title, SkillKind: r.SkillKind, SkillValue: r.SkillValue, TaskType: r.TaskType,
				PassThreshold: r.PassThreshold, InstructionsMd: r.InstructionsMd, Status: r.Status, Config: r.Config,
				SourceProjectID: r.SourceProjectID, QuestionCount: r.QuestionCount, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

// TestInput creates a test by copying gold items from a project.
type TestInput struct {
	Title          string    `json:"title"`
	SkillKind      string    `json:"skill_kind"`
	SkillValue     string    `json:"skill_value"`
	PassThreshold  float64   `json:"pass_threshold"`
	InstructionsMd string    `json:"instructions_md"`
	ProjectID      uuid.UUID `json:"project_id"`
	QuestionCount  int32     `json:"question_count"`
	// ContentShareable confirms the copied items may be shown to applicants
	// who have not yet been vetted (D-063).
	ContentShareable bool `json:"content_shareable"`
}

var skillValueRe = regexp.MustCompile(`^[a-z0-9_]+(-[A-Za-z0-9]{2,8})*$`)

// CreateTest copies up to QuestionCount active gold items from a project into
// a new draft test, then retires those gold items in the project: an
// applicant has now seen them, so they can no longer pass as normal tasks.
func (s *Service) CreateTest(ctx context.Context, p *auth.Principal, in TestInput, m auth.Meta) (Test, error) {
	in.Title, in.SkillValue = strings.TrimSpace(in.Title), strings.TrimSpace(in.SkillValue)
	switch {
	case in.Title == "" || len(in.Title) > 200:
		return Test{}, fieldErr("title", "enter a title of 1 to 200 characters")
	case !slices.Contains([]string{"language", "domain", "task_type"}, in.SkillKind):
		return Test{}, fieldErr("skill_kind", "use language, domain or task_type")
	case in.SkillKind == "language" && !languageRe.MatchString(in.SkillValue),
		in.SkillKind == "domain" && !slices.Contains(Domains, in.SkillValue),
		in.SkillKind == "task_type" && !slices.Contains([]string{quality.Classification, quality.Pairwise, quality.TranslationReview}, in.SkillValue),
		!skillValueRe.MatchString(in.SkillValue):
		return Test{}, fieldErr("skill_value", "not a valid value for this skill kind")
	case in.PassThreshold <= 0 || in.PassThreshold > 1:
		return Test{}, fieldErr("pass_threshold", "use a share above 0 and up to 1, for example 0.8")
	case len(in.InstructionsMd) > 20000:
		return Test{}, fieldErr("instructions_md", "instructions too long")
	case in.QuestionCount < 1 || in.QuestionCount > 50:
		return Test{}, fieldErr("question_count", "use 1 to 50 questions")
	case !in.ContentShareable:
		return Test{}, ErrNotShareable
	}
	var out Test
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		proj, err := q.GetProject(ctx, in.ProjectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fieldErr("project_id", "project not found")
		}
		if err != nil {
			return err
		}
		gold, err := q.ListGoldForQualification(ctx, store.ListGoldForQualificationParams{ProjectID: proj.ID, MaxCount: in.QuestionCount})
		if err != nil {
			return err
		}
		if len(gold) == 0 {
			return ErrNoGold
		}
		var cfg quality.Config // keep only the settings answers need
		if err := json.Unmarshal(proj.Config, &cfg); err != nil {
			return err
		}
		cb, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		t, err := q.CreateQualificationTest(ctx, store.CreateQualificationTestParams{
			Title: in.Title, SkillKind: in.SkillKind, SkillValue: in.SkillValue, TaskType: proj.TaskType,
			PassThreshold: in.PassThreshold, InstructionsMd: in.InstructionsMd, Status: "draft", Config: cb,
			SourceProjectID: &proj.ID, CreatedBy: &p.UserID,
		})
		if err != nil {
			return err
		}
		for i, g := range gold {
			if err := q.AddQualificationQuestion(ctx, store.AddQualificationQuestionParams{
				TestID: t.ID, Position: int32(i + 1), Content: g.Content, Expected: g.Expected, Tolerance: g.Tolerance,
			}); err != nil {
				return err
			}
			if err := q.RetireGoldItem(ctx, g.ID); err != nil {
				return err
			}
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.test_created", EntityType: "qualification_test",
			EntityID: &t.ID, OrgID: &proj.OrgID, After: map[string]any{"source_project_id": proj.ID, "questions": len(gold), "gold_retired": len(gold)}}); err != nil {
			return err
		}
		out = Test{ID: t.ID, Title: t.Title, SkillKind: t.SkillKind, SkillValue: t.SkillValue, TaskType: t.TaskType,
			PassThreshold: t.PassThreshold, InstructionsMd: t.InstructionsMd, Status: t.Status, Config: t.Config,
			SourceProjectID: t.SourceProjectID, QuestionCount: int32(len(gold)), CreatedAt: t.CreatedAt}
		return nil
	})
	return out, err
}

// SetTestStatus opens (active) or closes (retired) a test. Retiring keeps
// past attempts and qualifications.
func (s *Service) SetTestStatus(ctx context.Context, p *auth.Principal, id uuid.UUID, status string, m auth.Meta) error {
	if status != "active" && status != "retired" {
		return fieldErr("status", "use active or retired")
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		t, err := q.GetQualificationTest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "active" && t.QuestionCount == 0 {
			return ErrTestUnavailable
		}
		if t.Status == status {
			return nil
		}
		if err := q.SetQualificationTestStatus(ctx, store.SetQualificationTestStatusParams{ID: id, Status: status}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.test_" + status, EntityType: "qualification_test",
			EntityID: &id, Before: map[string]string{"status": t.Status}, After: map[string]string{"status": status}})
	})
}

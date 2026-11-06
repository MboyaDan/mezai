package contributors

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/legal"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Attempt states as stored in qualification_attempts.status.
const (
	AttemptInProgress = "in_progress"
	AttemptScored     = "scored" // waiting for Ops
	AttemptApproved   = "approved"
	AttemptRejected   = "rejected"
)

// maxAnswerBytes bounds one saved answer (the rationale and correction are the long parts).
const maxAnswerBytes = 16 << 10

// TestSummary is an open test with the caller's progress on it.
type TestSummary struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	SkillKind      string    `json:"skill_kind"`
	SkillValue     string    `json:"skill_value"`
	TaskType       string    `json:"task_type"`
	PassThreshold  float64   `json:"pass_threshold"`
	InstructionsMd string    `json:"instructions_md"`
	QuestionCount  int32     `json:"question_count"`
	// State: available, in_progress, awaiting_review, qualified or not_passed.
	State        string     `json:"state"`
	AttemptID    *uuid.UUID `json:"attempt_id"`
	Score        *float64   `json:"score"`
	DecisionNote *string    `json:"decision_note"`
	RetakeAfter  *time.Time `json:"retake_after"`
	CanStart     bool       `json:"can_start"`
}

// Question is a test question as a contributor sees it: never the answer.
type Question struct {
	ID       uuid.UUID       `json:"id"`
	Position int32           `json:"position"`
	Content  json.RawMessage `json:"content"`
}

// Attempt is a contributor's attempt with its questions.
type Attempt struct {
	ID             uuid.UUID                     `json:"id"`
	Status         string                        `json:"status"`
	TestID         uuid.UUID                     `json:"test_id"`
	Title          string                        `json:"title"`
	TaskType       string                        `json:"task_type"`
	Config         json.RawMessage               `json:"config"`
	InstructionsMd string                        `json:"instructions_md"`
	PassThreshold  float64                       `json:"pass_threshold"`
	Questions      []Question                    `json:"questions"`
	Answers        map[string]json.RawMessage    `json:"answers"`
	Score          *float64                      `json:"score"`
	Passed         *bool                         `json:"passed"`
	StartedAt      time.Time                     `json:"started_at"`
	SubmittedAt    *time.Time                    `json:"submitted_at"`
	DecisionNote   *string                       `json:"decision_note"`
	DecidedAt      *time.Time                    `json:"decided_at"`
	review         map[string]reviewQuestionData // filled for Ops only
}

type reviewQuestionData struct {
	expected json.RawMessage
	correct  *bool
}

// ListMyTests returns every active test with the caller's state on it.
func (s *Service) ListMyTests(ctx context.Context, p *auth.Principal) ([]TestSummary, error) {
	id, err := contributorOf(p)
	if err != nil {
		return nil, err
	}
	out := []TestSummary{}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.GetContributorProfile(ctx, id)
		if err != nil {
			return err
		}
		active := "active"
		tests, err := q.ListQualificationTests(ctx, &active)
		if err != nil {
			return err
		}
		attempts, err := q.ListAttemptsByContributor(ctx, id)
		if err != nil {
			return err
		}
		eligible := cp.Status == StatusQualifying || cp.Status == StatusActive
		for _, t := range tests {
			if t.QuestionCount == 0 {
				continue
			}
			ts := TestSummary{ID: t.ID, Title: t.Title, SkillKind: t.SkillKind, SkillValue: t.SkillValue, TaskType: t.TaskType,
				PassThreshold: t.PassThreshold, InstructionsMd: t.InstructionsMd, QuestionCount: t.QuestionCount, State: "available"}
			var mine []store.QualificationAttempt
			for _, a := range attempts {
				if a.TestID == t.ID {
					mine = append(mine, a) // newest first
				}
			}
			s.applyAttempts(&ts, mine)
			ts.CanStart = eligible && (ts.State == "available" || ts.State == "in_progress" ||
				(ts.State == "not_passed" && ts.RetakeAfter != nil && !s.now().Before(*ts.RetakeAfter)))
			out = append(out, ts)
		}
		return nil
	})
	return out, err
}

// applyAttempts sets the state from the caller's attempts on one test, newest first.
func (s *Service) applyAttempts(ts *TestSummary, attempts []store.QualificationAttempt) {
	for _, a := range attempts {
		if a.Status == AttemptApproved {
			ts.State, ts.AttemptID, ts.Score, ts.DecisionNote = "qualified", &a.ID, a.Score, a.DecisionNote
			return
		}
	}
	if len(attempts) == 0 {
		return
	}
	a := attempts[0]
	ts.AttemptID, ts.Score = &a.ID, a.Score
	switch a.Status {
	case AttemptInProgress:
		ts.State, ts.Score = "in_progress", nil
	case AttemptScored:
		ts.State = "awaiting_review"
	case AttemptRejected:
		ts.State, ts.DecisionNote = "not_passed", a.DecisionNote
		after := a.DecidedAt.Add(RetakeCooldown)
		ts.RetakeAfter = &after
	}
}

// StartAttempt opens an attempt, or returns the one already open.
func (s *Service) StartAttempt(ctx context.Context, p *auth.Principal, testID uuid.UUID, m auth.Meta) (Attempt, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Attempt{}, err
	}
	var out Attempt
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id) // serialises starts by the same contributor
		if err != nil {
			return err
		}
		switch cp.Status {
		case StatusApplied:
			return ErrProfileIncomplete
		case StatusPaused, StatusRemoved:
			return ErrNotEligible
		}
		consents, err := q.ListLatestConsents(ctx, p.UserID)
		if err != nil {
			return err
		}
		accepted := map[string]string{}
		for _, c := range consents {
			accepted[c.Document] = c.Version
		}
		if len(legal.Outstanding(accepted)) > 0 {
			return ErrConsentRequired
		}
		t, err := q.GetQualificationTest(ctx, testID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status != "active" || t.QuestionCount == 0 {
			return ErrTestUnavailable
		}
		if open, err := q.GetOpenAttempt(ctx, store.GetOpenAttemptParams{ContributorID: id, TestID: testID}); err == nil {
			if open.Status != AttemptInProgress {
				return ErrAttemptClosed // submitted and waiting for Ops
			}
			out, err = attemptView(ctx, q, open)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		attempts, err := q.ListAttemptsByContributor(ctx, id)
		if err != nil {
			return err
		}
		for _, a := range attempts {
			if a.TestID != testID {
				continue
			}
			if a.Status == AttemptApproved {
				return ErrAlreadyQualified
			}
			if a.Status == AttemptRejected && a.DecidedAt != nil && s.now().Before(a.DecidedAt.Add(RetakeCooldown)) {
				return ErrRetakeTooSoon
			}
		}
		a, err := q.CreateQualificationAttempt(ctx, store.CreateQualificationAttemptParams{TestID: testID, ContributorID: id})
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return ErrAttemptClosed
		}
		if err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.attempt_started", EntityType: "qualification_attempt",
			EntityID: &a.ID, After: map[string]any{"test_id": testID}}); err != nil {
			return err
		}
		out, err = attemptView(ctx, q, a)
		return err
	})
	return out, err
}

// MyAttempt returns one of the caller's attempts.
func (s *Service) MyAttempt(ctx context.Context, p *auth.Principal, attemptID uuid.UUID) (Attempt, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Attempt{}, err
	}
	var out Attempt
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.GetQualificationAttempt(ctx, attemptID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.ContributorID != id) {
			return ErrNotFound // never reveal someone else's attempt
		}
		if err != nil {
			return err
		}
		out, err = attemptView(ctx, q, a)
		return err
	})
	return out, err
}

func attemptView(ctx context.Context, q *store.Queries, a store.QualificationAttempt) (Attempt, error) {
	t, err := q.GetQualificationTest(ctx, a.TestID)
	if err != nil {
		return Attempt{}, err
	}
	qs, err := q.ListQualificationQuestions(ctx, a.TestID)
	if err != nil {
		return Attempt{}, err
	}
	out := Attempt{ID: a.ID, Status: a.Status, TestID: t.ID, Title: t.Title, TaskType: t.TaskType, Config: t.Config,
		InstructionsMd: t.InstructionsMd, PassThreshold: t.PassThreshold, Questions: make([]Question, len(qs)),
		Answers: map[string]json.RawMessage{}, StartedAt: a.StartedAt, SubmittedAt: a.SubmittedAt,
		DecisionNote: a.DecisionNote, DecidedAt: a.DecidedAt, review: map[string]reviewQuestionData{}}
	if a.Status != AttemptInProgress {
		out.Score, out.Passed = a.Score, a.Passed
	}
	var results map[string]bool
	_ = json.Unmarshal(a.Results, &results)
	for i, qq := range qs {
		out.Questions[i] = Question{ID: qq.ID, Position: qq.Position, Content: qq.Content}
		rd := reviewQuestionData{expected: qq.Expected}
		if c, ok := results[qq.ID.String()]; ok {
			rd.correct = &c
		}
		out.review[qq.ID.String()] = rd
	}
	if err := json.Unmarshal(a.Answers, &out.Answers); err != nil {
		return Attempt{}, err
	}
	return out, nil
}

// checkAnswerKeys keeps only answers to this test's questions, within size.
func checkAnswerKeys(qs []store.QualificationQuestion, answers map[string]json.RawMessage) error {
	known := map[string]bool{}
	for _, q := range qs {
		known[q.ID.String()] = true
	}
	for k, v := range answers {
		if !known[k] {
			return fieldErr("answers", "unknown question")
		}
		if len(v) > maxAnswerBytes {
			return fieldErr("answers."+k, "answer too long")
		}
	}
	return nil
}

func (s *Service) lockMine(ctx context.Context, q *store.Queries, p *auth.Principal, attemptID uuid.UUID) (store.QualificationAttempt, error) {
	id, err := contributorOf(p)
	if err != nil {
		return store.QualificationAttempt{}, err
	}
	a, err := q.LockQualificationAttempt(ctx, attemptID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.ContributorID != id) {
		return store.QualificationAttempt{}, ErrNotFound
	}
	return a, err
}

// SaveAnswers stores draft answers on an open attempt (autosave). Drafts are
// not validated beyond belonging to the test.
func (s *Service) SaveAnswers(ctx context.Context, p *auth.Principal, attemptID uuid.UUID, answers map[string]json.RawMessage) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := s.lockMine(ctx, q, p, attemptID)
		if err != nil {
			return err
		}
		if a.Status != AttemptInProgress {
			return ErrAttemptClosed
		}
		qs, err := q.ListQualificationQuestions(ctx, a.TestID)
		if err != nil {
			return err
		}
		if err := checkAnswerKeys(qs, answers); err != nil {
			return err
		}
		b, err := json.Marshal(answers)
		if err != nil {
			return err
		}
		return q.SaveAttemptAnswers(ctx, store.SaveAttemptAnswersParams{ID: a.ID, Answers: b})
	})
}

// Submit scores an attempt and sends it to the Ops queue. Every question must
// have a valid answer. Submitting an attempt that is already scored returns
// it unchanged, so a retried request cannot score twice.
func (s *Service) Submit(ctx context.Context, p *auth.Principal, attemptID uuid.UUID, answers map[string]json.RawMessage, m auth.Meta) (Attempt, error) {
	var out Attempt
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := s.lockMine(ctx, q, p, attemptID)
		if err != nil {
			return err
		}
		if a.Status != AttemptInProgress {
			out, err = attemptView(ctx, q, a)
			return err
		}
		t, err := q.GetQualificationTest(ctx, a.TestID)
		if err != nil {
			return err
		}
		qs, err := q.ListQualificationQuestions(ctx, a.TestID)
		if err != nil {
			return err
		}
		if answers == nil {
			if err := json.Unmarshal(a.Answers, &answers); err != nil {
				return err
			}
		}
		if err := checkAnswerKeys(qs, answers); err != nil {
			return err
		}
		var cfg quality.Config
		if err := json.Unmarshal(t.Config, &cfg); err != nil {
			return err
		}
		results := map[string]bool{}
		correct := 0
		for _, qq := range qs {
			key := qq.ID.String()
			raw, ok := answers[key]
			if !ok {
				return fieldErr("answers."+key, "answer question %d", qq.Position)
			}
			ans, err := quality.Parse(t.TaskType, cfg, raw)
			if err != nil {
				return fieldErr("answers."+key, "question %d: %s", qq.Position, strings.TrimPrefix(err.Error(), quality.ErrInvalidAnswer.Error()+": "))
			}
			ok, err = quality.Matches(t.TaskType, qq.Expected, qq.Tolerance, ans)
			if err != nil {
				return err
			}
			results[key] = ok
			if ok {
				correct++
			}
		}
		score := float64(correct) / float64(len(qs))
		passed := correct >= int(math.Ceil(t.PassThreshold*float64(len(qs))-1e-9))
		ab, err := json.Marshal(answers)
		if err != nil {
			return err
		}
		rb, err := json.Marshal(results)
		if err != nil {
			return err
		}
		score = math.Round(score*10000) / 10000
		if err := q.ScoreAttempt(ctx, store.ScoreAttemptParams{ID: a.ID, Answers: ab, Results: rb, Score: &score, Passed: &passed}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.attempt_scored", EntityType: "qualification_attempt",
			EntityID: &a.ID, After: map[string]any{"test_id": t.ID, "score": score, "passed": passed}}); err != nil {
			return err
		}
		a, err = q.GetQualificationAttempt(ctx, a.ID)
		if err != nil {
			return err
		}
		out, err = attemptView(ctx, q, a)
		return err
	})
	return out, err
}

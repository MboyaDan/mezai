package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Task is one reserved item as the contributor sees it. Gold and regular
// items look exactly alike: the ID is the assignment's, never the item's,
// and there is no external ID or gold marker (AC-06).
type Task struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"project_id"`
	TaskType     string          `json:"task_type"`
	Status       string          `json:"status"` // reserved, submitted, skipped, expired, reported
	ExpiresAt    time.Time       `json:"expires_at"`
	Content      json.RawMessage `json:"content"` // null once the reservation has ended
	Draft        json.RawMessage `json:"draft"`   // the last saved answer, or null
	TimeOnTaskMs int64           `json:"time_on_task_ms"`
}

// Next returns the caller's current task in the project, or reserves a new
// one: an item short of k submissions they have never seen, swapped for an
// unseen gold item at the project's gold rate. Gold is only served while
// regular work remains, so a project's last tasks never reveal its gold
// (D-096).
func (s *Service) Next(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (Task, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Task{}, err
	}
	var taskID uuid.UUID
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		// Serialise this contributor's own requests (one task at a time).
		if _, err := q.LockContributorProfile(ctx, id); err != nil {
			return err
		}
		pr, _, err := eligibleProject(ctx, q, id, projectID)
		if err != nil {
			return err
		}
		if err := q.ExpireStaleReservations(ctx, store.ExpireStaleReservationsParams{ProjectID: projectID, ContributorID: id}); err != nil {
			return err
		}
		if cur, err := q.CurrentReservation(ctx, store.CurrentReservationParams{ContributorID: id, ProjectID: projectID}); err == nil {
			taskID = cur.ID
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		item, err := q.PickRegularItem(ctx, store.PickRegularItemParams{ProjectID: projectID, ContributorID: id, OverlapK: int32(pr.OverlapK)})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // commit the expiries; taskID stays nil
		}
		if err != nil {
			return err
		}
		itemID, gold := item.ID, uuid.Nil
		if pr.GoldRate > 0 && s.rnd() < pr.GoldRate {
			g, err := q.PickGoldItem(ctx, store.PickGoldItemParams{ProjectID: projectID, ContributorID: id})
			if err == nil {
				itemID, gold = g.ItemID, g.ID
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		a, err := q.ReserveItem(ctx, store.ReserveItemParams{OrgID: pr.OrgID, ProjectID: projectID, ItemID: itemID,
			ContributorID: id, Minutes: pr.ReservationMinutes})
		if err != nil {
			return err
		}
		if gold != uuid.Nil {
			err = q.BumpGoldUsage(ctx, gold)
		} else {
			err = q.MarkItemInProgress(ctx, itemID)
		}
		taskID = a.ID
		return err
	})
	if err != nil {
		return Task{}, err
	}
	if taskID == uuid.Nil {
		return Task{}, ErrNoTasks
	}
	return s.GetTask(ctx, p, taskID)
}

// GetTask returns one of the caller's tasks. The item content is read in
// the contributor's scope: row-level security shows it only while the
// reservation is live.
func (s *Service) GetTask(ctx context.Context, p *auth.Principal, taskID uuid.UUID) (Task, error) {
	if _, err := contributorOf(p); err != nil {
		return Task{}, err
	}
	var out Task
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.GetMyAssignment(ctx, taskID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		pr, err := q.GetProjectForWork(ctx, a.ProjectID)
		if err != nil {
			return err
		}
		out = Task{ID: a.ID, ProjectID: a.ProjectID, TaskType: pr.TaskType, Status: a.Status, ExpiresAt: a.ExpiresAt}
		if a.Status == "reserved" && !a.ExpiresAt.After(time.Now()) {
			out.Status = "expired" // marked lazily in the database
		}
		if out.Status == "reserved" {
			c, err := q.GetItemContent(ctx, a.ItemID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			out.Content = c
		}
		sub, err := q.GetSubmissionByAssignment(ctx, a.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out.TimeOnTaskMs = sub.TimeOnTaskMs
		if rev, err := q.GetLatestRevision(ctx, sub.ID); err == nil {
			out.Draft = rev.Answer
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	return out, err
}

// openAssignment locks the caller's reserved, unexpired assignment.
func openAssignment(ctx context.Context, q *store.Queries, contributor, taskID uuid.UUID) (store.Assignment, error) {
	a, err := q.LockMyAssignment(ctx, store.LockMyAssignmentParams{ID: taskID, ContributorID: contributor})
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	if a.Status == "submitted" {
		return a, ErrAlreadySubmitted
	}
	if a.Status != "reserved" || !a.ExpiresAt.After(time.Now()) {
		return a, ErrReservationClosed
	}
	return a, nil
}

func checkAnswerJSON(raw json.RawMessage) error {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || t[0] != '{' || !json.Valid(t) {
		return fieldErr("answer", "send the answer as an object")
	}
	if len(t) > maxAnswerBytes {
		return fieldErr("answer", "the answer is too long")
	}
	return nil
}

func clampTime(ms int64) int64 { return max(0, min(ms, maxTimeOnTask)) }

// draftSubmission returns the assignment's submission, creating the draft row.
func draftSubmission(ctx context.Context, q *store.Queries, a store.Assignment) (store.Submission, error) {
	sub, err := q.GetSubmissionByAssignment(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return q.CreateDraftSubmission(ctx, store.CreateDraftSubmissionParams{
			OrgID: a.OrgID, ProjectID: a.ProjectID, ItemID: a.ItemID, AssignmentID: a.ID, ContributorID: a.ContributorID})
	}
	return sub, err
}

// SaveDraft autosaves an unfinished answer. Every changed answer is a new
// revision; nothing is overwritten. An unchanged answer only updates the time.
func (s *Service) SaveDraft(ctx context.Context, p *auth.Principal, taskID uuid.UUID, answer json.RawMessage, timeOnTaskMs int64) error {
	id, err := contributorOf(p)
	if err != nil {
		return err
	}
	if err := checkAnswerJSON(answer); err != nil {
		return err
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := openAssignment(ctx, q, id, taskID)
		if err != nil {
			return err
		}
		sub, err := draftSubmission(ctx, q, a)
		if err != nil {
			return err
		}
		rev := sub.CurrentRevision
		last, err := q.GetLatestRevision(ctx, sub.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err != nil || !jsonEqual(last.Answer, answer) {
			if rev >= maxRevisions {
				return ErrDraftLimit
			}
			rev++
			if err := q.AddSubmissionRevision(ctx, store.AddSubmissionRevisionParams{
				OrgID: a.OrgID, SubmissionID: sub.ID, Revision: rev, Answer: answer, IsDraft: true}); err != nil {
				return err
			}
		}
		return q.SaveDraftProgress(ctx, store.SaveDraftProgressParams{ID: sub.ID, CurrentRevision: rev, TimeOnTaskMs: clampTime(timeOnTaskMs)})
	})
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return bytes.Equal(xb, yb)
}

// SubmitInput is a final answer.
type SubmitInput struct {
	Answer  json.RawMessage `json:"answer"`
	Signals quality.Signals `json:"signals"`
}

// Result is the answer to a submission. It is the same for gold and regular
// items: no score or correctness is returned here (delayed gold feedback).
type Result struct {
	SubmissionID uuid.UUID       `json:"submission_id"`
	Status       string          `json:"status"`
	Warnings     []quality.Issue `json:"warnings"`
}

var keyRe = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,100}$`)

// Submit records the final answer once. The same idempotency key returns
// the first result without writing again (AC-07); another key on a
// submitted task is refused.
func (s *Service) Submit(ctx context.Context, p *auth.Principal, taskID uuid.UUID, key string, in SubmitInput, m auth.Meta) (Result, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Result{}, err
	}
	if !keyRe.MatchString(key) {
		return Result{}, fieldErr("idempotency_key", "send an Idempotency-Key header of 8 to 100 letters, digits or _.:-")
	}
	if err := checkAnswerJSON(in.Answer); err != nil {
		return Result{}, err
	}
	var out Result
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if r, done, err := replayed(ctx, q, id, key, taskID); done || err != nil {
			out = r
			return err
		}
		a, err := openAssignment(ctx, q, id, taskID)
		if err != nil {
			return err
		}
		pr, err := q.GetProjectForWork(ctx, a.ProjectID)
		if err != nil {
			return err
		}
		cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
		if err != nil {
			return err
		}
		ans, err := quality.Parse(pr.TaskType, cfg.Answers(), in.Answer)
		if err != nil {
			return fieldErr("answer", "%s", strings.TrimPrefix(err.Error(), quality.ErrInvalidAnswer.Error()+": "))
		}
		raw, err := q.GetItemContent(ctx, a.ItemID)
		if err != nil {
			return err
		}
		content := map[string]string{}
		_ = json.Unmarshal(raw, &content)
		qa, err := quality.ValidateQAChecks(pr.TaskType, pr.QaChecks)
		if err != nil {
			qa = quality.DefaultQAChecks(pr.TaskType)
		}
		in.Signals.TimeOnTaskMs = clampTime(in.Signals.TimeOnTaskMs)
		blocks, warnings := quality.Check(pr.TaskType, quality.CheckRules{QA: qa, MinRationaleLen: int(pr.MinRationaleChars)}, content, ans, in.Signals)
		if len(blocks) > 0 {
			return &BlockedError{Issues: blocks}
		}
		sub, err := draftSubmission(ctx, q, a)
		if err != nil {
			return err
		}
		rev := sub.CurrentRevision + 1
		if err := q.AddSubmissionRevision(ctx, store.AddSubmissionRevisionParams{
			OrgID: a.OrgID, SubmissionID: sub.ID, Revision: rev, Answer: in.Answer, IsDraft: false}); err != nil {
			return err
		}
		wj, _ := json.Marshal(warnings)
		if err := q.MarkSubmitted(ctx, store.MarkSubmittedParams{ID: sub.ID, IdempotencyKey: &key, CurrentRevision: rev,
			TimeOnTaskMs: max(sub.TimeOnTaskMs, in.Signals.TimeOnTaskMs), QaWarnings: wj}); err != nil {
			return err
		}
		if n, err := q.CompleteAssignment(ctx, store.CompleteAssignmentParams{ID: a.ID, Status: "submitted"}); err != nil || n != 1 {
			return errors.Join(err, ErrReservationClosed)
		}
		gold, err := q.IsGoldItem(ctx, a.ItemID)
		if err != nil {
			return err
		}
		if err := q.CountItemSubmission(ctx, store.CountItemSubmissionParams{ID: a.ItemID, IsGold: gold, OverlapK: int32(pr.OverlapK)}); err != nil {
			return err
		}
		out = Result{SubmissionID: sub.ID, Status: "submitted", Warnings: warnings}
		// The audit names the task and the warnings, never the answer.
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "task.submitted", EntityType: "submission", EntityID: &sub.ID,
			OrgID: &a.OrgID, After: map[string]any{"assignment_id": a.ID, "warnings": len(warnings)}})
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "submissions_contributor_id_idempotency_key_key" {
		// The same key raced in from a second request: answer as a replay.
		err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			var e error
			out, _, e = replayed(ctx, store.New(tx), id, key, taskID)
			return e
		})
	}
	return out, err
}

// replayed finds an earlier submission with the same key.
func replayed(ctx context.Context, q *store.Queries, contributor uuid.UUID, key string, taskID uuid.UUID) (Result, bool, error) {
	prev, err := q.GetSubmissionByKey(ctx, store.GetSubmissionByKeyParams{ContributorID: contributor, IdempotencyKey: &key})
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if prev.AssignmentID != taskID {
		return Result{}, false, ErrKeyReused
	}
	r := Result{SubmissionID: prev.ID, Status: prev.Status, Warnings: []quality.Issue{}}
	_ = json.Unmarshal(prev.QaWarnings, &r.Warnings)
	return r, true, nil
}

// Skip gives a task back with a reason. The item never returns to this
// contributor (one assignment per contributor and item).
func (s *Service) Skip(ctx context.Context, p *auth.Principal, taskID uuid.UUID, reason, note string, m auth.Meta) error {
	id, err := contributorOf(p)
	if err != nil {
		return err
	}
	if !slices.Contains(SkipReasons, reason) {
		return fieldErr("reason", "choose a reason from the list")
	}
	notePtr, err := optionalNote(note, reason == "other")
	if err != nil {
		return err
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := openAssignment(ctx, q, id, taskID)
		if err != nil {
			return err
		}
		if err := q.AddSkip(ctx, store.AddSkipParams{OrgID: a.OrgID, AssignmentID: a.ID, ContributorID: id, Reason: reason, Note: notePtr}); err != nil {
			return err
		}
		if _, err := q.CompleteAssignment(ctx, store.CompleteAssignmentParams{ID: a.ID, Status: "skipped"}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "task.skipped", EntityType: "assignment", EntityID: &a.ID,
			OrgID: &a.OrgID, After: map[string]string{"reason": reason}})
	})
}

// Report flags an item's content. The task leaves the contributor's queue at
// once, and nobody is given the item while the report is open (D-097).
func (s *Service) Report(ctx context.Context, p *auth.Principal, taskID uuid.UUID, category, note string, m auth.Meta) error {
	id, err := contributorOf(p)
	if err != nil {
		return err
	}
	if !slices.Contains(ReportCategories, category) {
		return fieldErr("category", "choose a category from the list")
	}
	notePtr, err := optionalNote(note, category == "other")
	if err != nil {
		return err
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		a, err := q.LockMyAssignment(ctx, store.LockMyAssignmentParams{ID: taskID, ContributorID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if a.Status != "reserved" { // an expired reservation can still be reported
			return ErrReservationClosed
		}
		rid, err := q.AddContentReport(ctx, store.AddContentReportParams{OrgID: a.OrgID, ItemID: a.ItemID, AssignmentID: &a.ID,
			ContributorID: id, Category: category, Note: notePtr})
		if err != nil {
			return err
		}
		if _, err := q.CompleteAssignment(ctx, store.CompleteAssignmentParams{ID: a.ID, Status: "reported"}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "content.reported", EntityType: "content_report", EntityID: &rid,
			OrgID: &a.OrgID, After: map[string]string{"category": category}})
	})
}

func optionalNote(note string, required bool) (*string, error) {
	note = strings.TrimSpace(note)
	if len(note) > 1000 {
		return nil, fieldErr("note", "keep the note under 1000 characters")
	}
	if note == "" {
		if required {
			return nil, fieldErr("note", "say what the problem is")
		}
		return nil, nil
	}
	return &note, nil
}

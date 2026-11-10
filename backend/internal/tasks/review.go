package tasks

import (
	"context"
	"encoding/json"
	"errors"
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

// Review and adjudication (Stage 11a, brief 8.3). Items the quality engine
// routed to review wait here; a reviewer accepts the consensus, chooses one
// submission, or (with review.adjudicate) writes the final answer or rejects
// the item. Every submitted answer is accepted and earns unless the reviewer
// rejects it with a reason, which the contributor sees (D-110).

// Review decisions (the reviews.decision column).
const (
	DecisionAcceptConsensus  = "accept_consensus"
	DecisionChooseSubmission = "choose_submission"
	DecisionWriteFinal       = "write_final"
	DecisionRejectItem       = "reject_item"
)

// RouteReasons are why an item went to review (brief 8.3).
var RouteReasons = []string{"no_majority", "spread", "flagged", "audit_sample"}

var (
	// ErrAlreadyDecided: someone already decided this item.
	ErrAlreadyDecided = errors.New("tasks: this item was already decided")
	// ErrForbidden: the decision needs a permission the caller lacks.
	ErrForbidden = errors.New("tasks: not permitted")
)

// ReviewQueueItem is one item waiting for a reviewer.
type ReviewQueueItem struct {
	ItemID       uuid.UUID `json:"item_id"`
	ExternalID   *string   `json:"external_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	ProjectName  string    `json:"project_name"`
	OrgName      string    `json:"org_name"`
	TaskType     string    `json:"task_type"`
	Reason       string    `json:"reason"`
	Agreement    *float64  `json:"agreement"`
	Spread       *float64  `json:"spread"`
	Submissions  int32     `json:"submissions"`
	WaitingSince time.Time `json:"waiting_since"`
}

// ReviewQueue is the queue plus the projects that have items in it.
type ReviewQueue struct {
	Items    []ReviewQueueItem   `json:"items"`
	Projects []ReviewQueueFilter `json:"projects"`
}

// ReviewQueueFilter is a project in the queue's filter list.
type ReviewQueueFilter struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Waiting int32     `json:"waiting"`
}

// adjudicatesAll: people with review.adjudicate review on every project;
// others only on projects Ops assigned them to (D-111).
func adjudicatesAll(p *auth.Principal) bool { return p.Has("review.adjudicate") }

// ListReviewQueue lists items waiting for review, oldest first.
func (s *Service) ListReviewQueue(ctx context.Context, p *auth.Principal, projectID *uuid.UUID, reason string) (ReviewQueue, error) {
	var rf *string
	if reason != "" {
		if !contains(RouteReasons, reason) {
			return ReviewQueue{}, fieldErr("reason", "use no_majority, spread, flagged or audit_sample")
		}
		rf = &reason
	}
	out := ReviewQueue{Items: []ReviewQueueItem{}, Projects: []ReviewQueueFilter{}}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.ReviewQueue(ctx, store.ReviewQueueParams{AllProjects: adjudicatesAll(p), UserID: p.UserID, ProjectID: projectID, Reason: rf})
		if err != nil {
			return err
		}
		for _, r := range rows {
			reason := ""
			if r.RouteReason != nil {
				reason = *r.RouteReason
			}
			out.Items = append(out.Items, ReviewQueueItem{ItemID: r.ID, ExternalID: r.ExternalID, ProjectID: r.ProjectID,
				ProjectName: r.ProjectName, OrgName: r.OrgName, TaskType: r.TaskType, Reason: reason, Agreement: r.Agreement,
				Spread: r.Spread, Submissions: r.SubmissionCount, WaitingSince: r.UpdatedAt})
		}
		ps, err := q.ReviewProjects(ctx, store.ReviewProjectsParams{AllProjects: adjudicatesAll(p), UserID: p.UserID})
		for _, r := range ps {
			out.Projects = append(out.Projects, ReviewQueueFilter{ID: r.ID, Name: r.Name, Waiting: r.Waiting})
		}
		return err
	})
	return out, err
}

// ReviewSubmission is one contributor's answer, shown side by side. Only the
// contributor code is shown, never a name.
type ReviewSubmission struct {
	ID              uuid.UUID       `json:"id"`
	Contributor     string          `json:"contributor"`
	Status          string          `json:"status"`
	SubmittedAt     *time.Time      `json:"submitted_at"`
	TimeOnTaskMs    int64           `json:"time_on_task_ms"`
	Answer          json.RawMessage `json:"answer"`
	Warnings        json.RawMessage `json:"warnings"`
	RejectionReason *string         `json:"rejection_reason"`
	AgreesWithFinal *bool           `json:"agrees_with_final"`
}

// ReviewConsensus is what the quality engine worked out for the item.
type ReviewConsensus struct {
	Method    string          `json:"method"`
	Result    json.RawMessage `json:"result"`
	Complete  bool            `json:"complete"` // there is an answer to accept
	Agreement *float64        `json:"agreement"`
	Spread    *float64        `json:"spread"`
	Reason    *string         `json:"reason"`
}

// ReviewDecision is a decision already taken on the item.
type ReviewDecision struct {
	ID                 uuid.UUID       `json:"id"`
	Decision           string          `json:"decision"`
	ChosenSubmissionID *uuid.UUID      `json:"chosen_submission_id"`
	FinalAnswer        json.RawMessage `json:"final_answer"`
	Reason             *string         `json:"reason"`
	Reviewer           string          `json:"reviewer"`
	CreatedAt          time.Time       `json:"created_at"`
}

// ReviewItem is everything a reviewer needs to decide one item.
type ReviewItem struct {
	ID            uuid.UUID          `json:"id"`
	ExternalID    *string            `json:"external_id"`
	Status        string             `json:"status"`
	ProjectID     uuid.UUID          `json:"project_id"`
	ProjectName   string             `json:"project_name"`
	TaskType      string             `json:"task_type"`
	Config        json.RawMessage    `json:"config"`
	Instructions  string             `json:"instructions_md"`
	Content       json.RawMessage    `json:"content"`
	Consensus     *ReviewConsensus   `json:"consensus"`
	Submissions   []ReviewSubmission `json:"submissions"`
	Decision      *ReviewDecision    `json:"decision"`
	CanAdjudicate bool               `json:"can_adjudicate"`
}

// mayReview says whether p may review items of the project.
func mayReview(ctx context.Context, q *store.Queries, p *auth.Principal, projectID uuid.UUID) (bool, error) {
	if adjudicatesAll(p) {
		return true, nil
	}
	return q.CanReviewProject(ctx, store.CanReviewProjectParams{ProjectID: projectID, UserID: p.UserID})
}

// GetReviewItem returns one item with every answer and the consensus. Items
// the caller may not review are reported as not found.
func (s *Service) GetReviewItem(ctx context.Context, p *auth.Principal, itemID uuid.UUID) (ReviewItem, error) {
	var out ReviewItem
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		it, err := q.GetReviewItem(ctx, itemID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if ok, err := mayReview(ctx, q, p, it.ProjectID); err != nil || !ok {
			return errors.Join(err, ErrNotFound)
		}
		if g, err := q.IsGoldItem(ctx, itemID); err != nil || g {
			return errors.Join(err, ErrNotFound) // gold is checked automatically, never reviewed
		}
		out = ReviewItem{ID: it.ID, ExternalID: it.ExternalID, Status: it.Status, ProjectID: it.ProjectID, ProjectName: it.ProjectName,
			TaskType: it.TaskType, Config: it.Config, Instructions: it.InstructionsMd, Content: it.Content,
			Submissions: []ReviewSubmission{}, CanAdjudicate: p.Has("review.adjudicate")}
		if c, err := q.LatestConsensus(ctx, itemID); err == nil {
			out.Consensus = &ReviewConsensus{Method: c.Method, Result: c.Result, Complete: consensusComplete(it.TaskType, c.Result),
				Agreement: c.Agreement, Spread: c.Spread, Reason: c.RouteReason}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		subs, err := q.ItemSubmissions(ctx, itemID)
		if err != nil {
			return err
		}
		for _, r := range subs {
			out.Submissions = append(out.Submissions, ReviewSubmission{ID: r.ID, Contributor: r.PublicCode, Status: r.Status,
				SubmittedAt: r.SubmittedAt, TimeOnTaskMs: r.TimeOnTaskMs, Answer: r.Answer, Warnings: r.QaWarnings,
				RejectionReason: r.RejectionReason, AgreesWithFinal: r.AgreesWithFinal})
		}
		if d, err := q.GetItemReview(ctx, itemID); err == nil {
			out.Decision = &ReviewDecision{ID: d.ID, Decision: d.Decision, ChosenSubmissionID: d.ChosenSubmissionID,
				FinalAnswer: d.FinalAnswer, Reason: d.Reason, Reviewer: d.ReviewerName, CreatedAt: d.CreatedAt}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	return out, err
}

// consensusComplete reports whether a consensus result is a usable final
// answer (no majority leaves the label, preference or corrected text empty).
func consensusComplete(taskType string, result []byte) bool {
	var r struct {
		Labels     []string `json:"labels"`
		Preference string   `json:"preference"`
		Corrected  string   `json:"corrected"`
	}
	if json.Unmarshal(result, &r) != nil {
		return false
	}
	switch taskType {
	case quality.Classification:
		return len(r.Labels) > 0
	case quality.Pairwise:
		return r.Preference != ""
	case quality.TranslationReview:
		return strings.TrimSpace(r.Corrected) != ""
	}
	return false
}

// SubmissionNote is a per-submission note in a decision: a rejection reason
// or feedback for the contributor.
type SubmissionNote struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	Text         string    `json:"text"`
}

// DecisionInput is a reviewer's decision on one item.
type DecisionInput struct {
	Decision     string           `json:"decision"`
	SubmissionID *uuid.UUID       `json:"submission_id"` // choose_submission
	FinalAnswer  json.RawMessage  `json:"final_answer"`  // write_final
	Reason       string           `json:"reason"`        // required for reject_item; optional note otherwise
	Rejections   []SubmissionNote `json:"rejections"`    // answers that will not be accepted, with the reason shown to the contributor
	Feedback     []SubmissionNote `json:"feedback"`      // notes to contributors (feedback.write)
}

// DecisionResult is the outcome of a decision (the same on a replay).
type DecisionResult struct {
	ReviewID   uuid.UUID `json:"review_id"`
	Decision   string    `json:"decision"`
	ItemStatus string    `json:"item_status"`
}

// Limits on reviewer text.
const (
	minRejectReason = 10
	maxReviewText   = 1000
)

// Decide records a reviewer's decision on an item in review. The same
// idempotency key returns the first result; a different key on a decided
// item is refused. One transaction: the item becomes final (or rejected),
// each answer is accepted and earns or is rejected with a reason, agreement
// with the final answer is recorded and trust scores are recalculated.
func (s *Service) Decide(ctx context.Context, p *auth.Principal, itemID uuid.UUID, key string, in DecisionInput, m auth.Meta) (DecisionResult, error) {
	if !keyRe.MatchString(key) {
		return DecisionResult{}, fieldErr("idempotency_key", "send an Idempotency-Key header of 8 to 100 letters, digits or _.:-")
	}
	switch in.Decision {
	case DecisionAcceptConsensus, DecisionChooseSubmission:
	case DecisionWriteFinal, DecisionRejectItem:
		if !p.Has("review.adjudicate") {
			return DecisionResult{}, ErrForbidden
		}
	default:
		return DecisionResult{}, fieldErr("decision", "use accept_consensus, choose_submission, write_final or reject_item")
	}
	if len(in.Feedback) > 0 && !p.Has("feedback.write") {
		return DecisionResult{}, ErrForbidden
	}
	reason, err := reviewText("reason", in.Reason, in.Decision == DecisionRejectItem, minRejectReason)
	if err != nil {
		return DecisionResult{}, err
	}
	rejections, err := notesByID("rejections", in.Rejections, minRejectReason)
	if err != nil {
		return DecisionResult{}, err
	}
	feedback, err := notesByID("feedback", in.Feedback, 1)
	if err != nil {
		return DecisionResult{}, err
	}
	if in.Decision == DecisionChooseSubmission && in.SubmissionID == nil {
		return DecisionResult{}, fieldErr("submission_id", "choose the submission to use as the final answer")
	}
	if in.SubmissionID != nil && rejections[*in.SubmissionID] != "" {
		return DecisionResult{}, fieldErr("rejections", "the chosen answer cannot also be rejected")
	}

	var out DecisionResult
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		item, err := q.LockReviewItem(ctx, itemID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if ok, err := mayReview(ctx, q, p, item.ProjectID); err != nil || !ok {
			return errors.Join(err, ErrNotFound)
		}
		if prev, err := q.GetReviewByKey(ctx, store.GetReviewByKeyParams{ReviewerUserID: p.UserID, IdempotencyKey: key}); err == nil {
			if prev.ItemID != itemID {
				return ErrKeyReused
			}
			out = DecisionResult{ReviewID: prev.ID, Decision: prev.Decision, ItemStatus: item.Status}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if item.Status != "in_review" {
			return ErrAlreadyDecided
		}
		pr, err := q.GetProjectForWork(ctx, item.ProjectID)
		if err != nil {
			return err
		}
		cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
		if err != nil {
			return err
		}
		rows, err := q.ListFinalAnswersForItem(ctx, itemID)
		if err != nil {
			return err
		}
		onItem := map[uuid.UUID]store.ListFinalAnswersForItemRow{}
		for _, r := range rows {
			onItem[r.ID] = r
		}
		for id := range rejections {
			if _, ok := onItem[id]; !ok {
				return fieldErr("rejections", "every rejected answer must be an open answer on this item")
			}
		}
		for id := range feedback {
			if _, ok := onItem[id]; !ok {
				return fieldErr("feedback", "feedback must be about an answer on this item")
			}
		}

		// The final answer, and which answers agree with it.
		var final []byte
		agrees := map[uuid.UUID]bool{}
		switch in.Decision {
		case DecisionAcceptConsensus:
			c, err := q.LatestConsensus(ctx, itemID)
			if err != nil {
				return err
			}
			if !consensusComplete(pr.TaskType, c.Result) {
				return fieldErr("decision", "there is no consensus answer to accept: choose an answer or write the final one")
			}
			final = c.Result
			o, err := combine(pr.TaskType, cfg.Answers(), pr.ReviewSpreadThreshold, rows)
			if err != nil {
				return err
			}
			agrees = o.agrees
		case DecisionChooseSubmission:
			r, ok := onItem[*in.SubmissionID]
			if !ok {
				return fieldErr("submission_id", "choose an open answer on this item")
			}
			final = r.Answer
		case DecisionWriteFinal:
			if _, err := quality.Parse(pr.TaskType, cfg.Answers(), in.FinalAnswer); err != nil {
				return fieldErr("final_answer", "%s", strings.TrimPrefix(err.Error(), quality.ErrInvalidAnswer.Error()+": "))
			}
			final = in.FinalAnswer
		}
		if final != nil && in.Decision != DecisionAcceptConsensus {
			for _, r := range rows {
				var a quality.Answer
				if err := json.Unmarshal(r.Answer, &a); err != nil {
					return err
				}
				ok, err := quality.Matches(pr.TaskType, final, nil, a)
				if err != nil {
					return err
				}
				agrees[r.ID] = ok
			}
		}

		rv, err := q.AddReview(ctx, store.AddReviewParams{OrgID: item.OrgID, ProjectID: item.ProjectID, ItemID: itemID,
			ReviewerUserID: p.UserID, Decision: in.Decision, ChosenSubmissionID: in.SubmissionID, FinalAnswer: final,
			Reason: reason, IdempotencyKey: key})
		if err != nil {
			return err
		}
		status := "final"
		if in.Decision == DecisionRejectItem {
			status = "rejected"
			if n, err := q.MarkItemRejected(ctx, itemID); err != nil || n != 1 {
				return errors.Join(err, ErrAlreadyDecided)
			}
		} else if err := finalise(ctx, q, itemID, final, "review", &p.UserID); err != nil {
			return err
		}

		// Each answer: accepted and paid, or rejected with the reason shown.
		touched := map[uuid.UUID]bool{}
		accepted, rejected := 0, 0
		for _, r := range rows {
			st, why := "accepted", (*string)(nil)
			if t := rejections[r.ID]; t != "" {
				st, why = "rejected", &t
			}
			if _, err := q.DecideSubmission(ctx, store.DecideSubmissionParams{Status: st, ReviewID: &rv.ID, RejectionReason: why, ID: r.ID}); err != nil {
				return err
			}
			if st == "accepted" {
				accepted++
				if err := q.AccrueEarning(ctx, r.ID); err != nil {
					return err
				}
			} else {
				rejected++
				if err := q.AddFeedback(ctx, store.AddFeedbackParams{OrgID: item.OrgID, ProjectID: item.ProjectID, ContributorID: r.ContributorID,
					SubmissionID: &r.ID, AuthorUserID: &p.UserID, Body: "This answer was not accepted, so it does not earn. Reason: " + *why}); err != nil {
					return err
				}
			}
			if agree, ok := agrees[r.ID]; ok && len(rows) > 1 {
				if err := q.SetSubmissionAgreement(ctx, store.SetSubmissionAgreementParams{ID: r.ID, Agrees: &agree}); err != nil {
					return err
				}
			}
			if t := feedback[r.ID]; t != "" {
				if err := q.AddFeedback(ctx, store.AddFeedbackParams{OrgID: item.OrgID, ProjectID: item.ProjectID, ContributorID: r.ContributorID,
					SubmissionID: &r.ID, AuthorUserID: &p.UserID, Body: t}); err != nil {
					return err
				}
			}
			touched[r.ContributorID] = true
		}
		for c := range touched {
			if _, err := s.recomputeTrust(ctx, q, m, c); err != nil {
				return err
			}
		}
		out = DecisionResult{ReviewID: rv.ID, Decision: in.Decision, ItemStatus: status}
		// The audit names the decision and counts, never the answers.
		after := map[string]any{"decision": in.Decision, "accepted": accepted, "rejected": rejected, "feedback": len(feedback)}
		if in.SubmissionID != nil {
			after["chosen_submission_id"] = *in.SubmissionID
		}
		auditReason := ""
		if reason != nil {
			auditReason = *reason
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "review.decided", EntityType: "item", EntityID: &itemID,
			OrgID: &item.OrgID, Reason: auditReason, After: after})
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "reviews_reviewer_user_id_idempotency_key_key":
			// The same key raced in from a second request: answer as a replay.
			return s.Decide(ctx, p, itemID, key, in, m)
		case "reviews_one_per_item_idx":
			return DecisionResult{}, ErrAlreadyDecided
		}
	}
	return out, err
}

// reviewText trims and checks a reviewer's text.
func reviewText(field, text string, required bool, minLen int) (*string, error) {
	text = strings.TrimSpace(text)
	if len(text) > maxReviewText {
		return nil, fieldErr(field, "keep it under %d characters", maxReviewText)
	}
	if text == "" {
		if required {
			return nil, fieldErr(field, "give a reason")
		}
		return nil, nil
	}
	if len([]rune(text)) < minLen {
		return nil, fieldErr(field, "say a little more (at least %d characters)", minLen)
	}
	return &text, nil
}

// notesByID checks per-submission notes: one per submission, each with text.
func notesByID(field string, notes []SubmissionNote, minLen int) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, n := range notes {
		t, err := reviewText(field, n.Text, true, minLen)
		if err != nil {
			return nil, err
		}
		if _, dup := out[n.SubmissionID]; dup {
			return nil, fieldErr(field, "one note per answer")
		}
		out[n.SubmissionID] = *t
	}
	return out, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Reviewer is a staff member who may review a project.
type Reviewer struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

// ProjectReviewers are a project's assigned reviewers and who else could be.
type ProjectReviewers struct {
	Assigned   []Reviewer `json:"assigned"`
	Candidates []Reviewer `json:"candidates"`
}

// ListProjectReviewers lists a project's reviewers and the staff who could
// be assigned (anyone holding review.decide).
func (s *Service) ListProjectReviewers(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (ProjectReviewers, error) {
	out := ProjectReviewers{Assigned: []Reviewer{}, Candidates: []Reviewer{}}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetProjectForWork(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		as, err := q.ListProjectReviewers(ctx, projectID)
		if err != nil {
			return err
		}
		assigned := map[uuid.UUID]bool{}
		for _, r := range as {
			assigned[r.ID] = true
			out.Assigned = append(out.Assigned, Reviewer{ID: r.ID, DisplayName: r.DisplayName, Email: r.Email})
		}
		cs, err := q.ReviewerCandidates(ctx)
		for _, r := range cs {
			if !assigned[r.ID] {
				out.Candidates = append(out.Candidates, Reviewer{ID: r.ID, DisplayName: r.DisplayName, Email: r.Email})
			}
		}
		return err
	})
	return out, err
}

// SetProjectReviewer assigns (add true) or removes a reviewer on a project.
func (s *Service) SetProjectReviewer(ctx context.Context, p *auth.Principal, projectID, userID uuid.UUID, add bool, m auth.Meta) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.GetProjectForWork(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		action := "project.reviewer_removed"
		if add {
			cs, err := q.ReviewerCandidates(ctx)
			if err != nil {
				return err
			}
			ok := false
			for _, c := range cs {
				ok = ok || c.ID == userID
			}
			if !ok {
				return fieldErr("user_id", "choose an active staff member who can review")
			}
			if err := q.AddProjectReviewer(ctx, store.AddProjectReviewerParams{OrgID: pr.OrgID, ProjectID: projectID, UserID: userID, AddedBy: &p.UserID}); err != nil {
				return err
			}
			action = "project.reviewer_added"
		} else if _, err := q.RemoveProjectReviewer(ctx, store.RemoveProjectReviewerParams{ProjectID: projectID, UserID: userID}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: action, EntityType: "project", EntityID: &projectID,
			OrgID: &pr.OrgID, After: map[string]any{"user_id": userID}})
	})
}

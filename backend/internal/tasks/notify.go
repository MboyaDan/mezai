package tasks

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// JobProjectAvailable emails eligible contributors when a project first goes
// live (Stage 16). The projects service queues it on publish.
const JobProjectAvailable = "project_available"

var taskTypeNames = map[string]string{
	"classification":     "Text classification",
	"pairwise":           "Response comparison",
	"translation_review": "Translation review",
}

// NotifyProjectAvailable tells every contributor who may work on a live
// project that it is open: active, identity verified and holding its skills,
// the same rule as the task list. Each person is told once per project, so a
// retried job sends nothing twice. It returns how many were queued.
func (s *Service) NotifyProjectAvailable(ctx context.Context, projectID uuid.UUID) (int, error) {
	n := 0
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		ids, err := q.ActiveContributorIDs(ctx)
		if err != nil {
			return err
		}
		for _, c := range ids {
			pr, _, err := eligibleProject(ctx, q, c, projectID)
			switch {
			case errors.Is(err, ErrNotEligible):
				continue
			case errors.Is(err, ErrNotFound), errors.Is(err, ErrProjectClosed):
				return nil // paused, closed or gone before the job ran: tell nobody
			case err != nil:
				return err
			}
			data := map[string]string{"Project": pr.Name, "ProjectID": projectID.String(),
				"TaskType": taskTypeNames[pr.TaskType], "Pay": mail.Money(pr.PayPerTaskCents, pr.Currency), "Deadline": ""}
			if data["TaskType"] == "" {
				data["TaskType"] = pr.TaskType
			}
			if pr.Deadline != nil {
				data["Deadline"] = pr.Deadline.In(jobs.Nairobi).Format("2 Jan 2006")
			}
			cid := c
			queued, err := mail.Notify(ctx, q, mail.Notice{Contributor: &cid, Template: mail.TemplateProjectAvailable,
				Key: projectID.String() + ":" + c.String(), Data: data})
			if err != nil {
				return err
			}
			if queued {
				n++
			}
		}
		return nil
	})
	return n, err
}

func (s *Service) projectAvailableHandler(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		ProjectID uuid.UUID `json:"project_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	_, err := s.NotifyProjectAvailable(ctx, p.ProjectID)
	return err
}

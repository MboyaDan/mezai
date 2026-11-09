package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Skill is one skill a project requires.
type Skill struct {
	Kind           string  `json:"kind"`
	Value          string  `json:"value"`
	MinProficiency *string `json:"min_proficiency"`
}

// Project is a live project as a contributor sees it: what the work is and
// what it pays. Nothing about gold, overlap or other contributors.
type Project struct {
	ID                 uuid.UUID        `json:"id"`
	Name               string           `json:"name"`
	TaskType           string           `json:"task_type"`
	InstructionsMd     string           `json:"instructions_md"`
	Config             json.RawMessage  `json:"config"`
	PayPerTaskCents    int64            `json:"pay_per_task_cents"`
	Currency           string           `json:"currency"`
	Deadline           *time.Time       `json:"deadline"`
	ReservationMinutes int32            `json:"reservation_minutes"`
	Skills             []Skill          `json:"skills"`
	QAChecks           quality.QAChecks `json:"qa_checks"`
	MinRationaleChars  int32            `json:"min_rationale_chars"`
	MySubmitted        int32            `json:"my_submitted"`
}

var proficiencyRank = map[string]int{"working": 1, "fluent": 2, "native": 3}

// worker is what eligibility needs about a contributor.
type worker struct {
	skills map[[2]string]bool // approved qualifications (kind, value)
	langs  map[string]string  // language → proficiency from the profile
}

// loadWorker checks the contributor may work at all (active and identity
// verified, D-089) and reads their skills.
func loadWorker(ctx context.Context, q *store.Queries, id uuid.UUID) (worker, error) {
	cp, err := q.GetContributorProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return worker{}, ErrNotContributor
	}
	if err != nil {
		return worker{}, err
	}
	if cp.Status != "active" || cp.IdentityStatus != "verified" {
		return worker{}, ErrNotEligible
	}
	w := worker{skills: map[[2]string]bool{}, langs: map[string]string{}}
	held, err := q.ContributorSkillsHeld(ctx, id)
	if err != nil {
		return w, err
	}
	for _, h := range held {
		w.skills[[2]string{h.SkillKind, h.SkillValue}] = true
	}
	langs, err := q.ListContributorLanguages(ctx, id)
	if err != nil {
		return w, err
	}
	for _, l := range langs {
		w.langs[l.Language] = l.Proficiency
	}
	return w, nil
}

// holds reports whether the contributor has every required skill: an
// approved qualification for it and, for a language with a minimum, at
// least that proficiency on their profile (D-095).
func (w worker) holds(skills []Skill) bool {
	for _, sk := range skills {
		if !w.skills[[2]string{sk.Kind, sk.Value}] {
			return false
		}
		if sk.Kind == "language" && sk.MinProficiency != nil && proficiencyRank[w.langs[sk.Value]] < proficiencyRank[*sk.MinProficiency] {
			return false
		}
	}
	return true
}

func skillsByProject(rows []store.ListSkillsForProjectsRow) map[uuid.UUID][]Skill {
	out := map[uuid.UUID][]Skill{}
	for _, r := range rows {
		out[r.ProjectID] = append(out[r.ProjectID], Skill{Kind: r.SkillKind, Value: r.SkillValue, MinProficiency: r.MinProficiency})
	}
	return out
}

// ListProjects returns the live projects the caller is eligible for. An
// active, verified contributor without the skills gets an empty list.
func (s *Service) ListProjects(ctx context.Context, p *auth.Principal) ([]Project, error) {
	id, err := contributorOf(p)
	if err != nil {
		return nil, err
	}
	out := []Project{}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		w, err := loadWorker(ctx, q, id)
		if err != nil {
			return err
		}
		live, err := q.ListLiveProjects(ctx)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(live))
		for i, l := range live {
			ids[i] = l.ID
		}
		rows, err := q.ListSkillsForProjects(ctx, ids)
		if err != nil {
			return err
		}
		skills := skillsByProject(rows)
		counts, err := q.CountMyWork(ctx, id)
		if err != nil {
			return err
		}
		mine := map[uuid.UUID]int32{}
		for _, c := range counts {
			mine[c.ProjectID] = c.Submitted
		}
		for _, l := range live {
			sk := skills[l.ID]
			if sk == nil {
				sk = []Skill{}
			}
			if !w.holds(sk) {
				continue
			}
			pr, err := q.GetProjectForWork(ctx, l.ID)
			if err != nil {
				return err
			}
			out = append(out, projectView(pr, sk, mine[l.ID]))
		}
		return nil
	})
	return out, err
}

// GetProject returns one live project the caller is eligible for.
func (s *Service) GetProject(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (Project, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Project{}, err
	}
	var out Project
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, sk, err := eligibleProject(ctx, q, id, projectID)
		if err != nil {
			return err
		}
		counts, err := q.CountMyWork(ctx, id)
		if err != nil {
			return err
		}
		var n int32
		for _, c := range counts {
			if c.ProjectID == projectID {
				n = c.Submitted
			}
		}
		out = projectView(pr, sk, n)
		return nil
	})
	return out, err
}

// eligibleProject loads a live project and checks the contributor may work on it.
func eligibleProject(ctx context.Context, q *store.Queries, contributor, projectID uuid.UUID) (store.GetProjectForWorkRow, []Skill, error) {
	w, err := loadWorker(ctx, q, contributor)
	if err != nil {
		return store.GetProjectForWorkRow{}, nil, err
	}
	pr, err := q.GetProjectForWork(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pr, nil, ErrNotFound
	}
	if err != nil {
		return pr, nil, err
	}
	if pr.Status != "live" {
		return pr, nil, ErrProjectClosed
	}
	rows, err := q.ListSkillsForProjects(ctx, []uuid.UUID{projectID})
	if err != nil {
		return pr, nil, err
	}
	sk := skillsByProject(rows)[projectID]
	if sk == nil {
		sk = []Skill{}
	}
	if !w.holds(sk) {
		return pr, nil, ErrNotEligible
	}
	return pr, sk, nil
}

func projectView(pr store.GetProjectForWorkRow, sk []Skill, mine int32) Project {
	qa, err := quality.ValidateQAChecks(pr.TaskType, pr.QaChecks)
	if err != nil {
		qa = quality.DefaultQAChecks(pr.TaskType)
	}
	return Project{ID: pr.ID, Name: pr.Name, TaskType: pr.TaskType, InstructionsMd: pr.InstructionsMd, Config: pr.Config,
		PayPerTaskCents: pr.PayPerTaskCents, Currency: pr.Currency, Deadline: pr.Deadline, ReservationMinutes: pr.ReservationMinutes,
		Skills: sk, QAChecks: qa, MinRationaleChars: pr.MinRationaleChars, MySubmitted: mine}
}

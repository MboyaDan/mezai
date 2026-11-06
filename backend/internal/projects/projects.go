package projects

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Project statuses (brief 9): draft → ready → live ⇄ paused → closed.
const (
	StatusDraft  = "draft"
	StatusReady  = "ready"
	StatusLive   = "live"
	StatusPaused = "paused"
	StatusClosed = "closed"
)

// Actions move a project between statuses.
var actions = map[string]struct {
	from []string
	to   string
}{
	"ready":   {[]string{StatusDraft}, StatusReady},
	"draft":   {[]string{StatusReady}, StatusDraft},
	"publish": {[]string{StatusReady}, StatusLive},
	"pause":   {[]string{StatusLive}, StatusPaused},
	"resume":  {[]string{StatusPaused}, StatusLive},
	"close":   {[]string{StatusDraft, StatusReady, StatusLive, StatusPaused}, StatusClosed},
}

// Organisation is a client tenant.
type Organisation struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Status       string    `json:"status"`
	ProjectCount int32     `json:"project_count"`
	MemberCount  int32     `json:"member_count"`
	CreatedAt    time.Time `json:"created_at"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *Service) ListOrganisations(ctx context.Context, p *auth.Principal) ([]Organisation, error) {
	out := []Organisation{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListOrganisationSummaries(ctx)
		for _, o := range rows {
			out = append(out, Organisation{ID: o.ID, Name: o.Name, Slug: o.Slug, Status: o.Status,
				ProjectCount: o.ProjectCount, MemberCount: o.MemberCount, CreatedAt: o.CreatedAt})
		}
		return err
	})
	return out, err
}

func (s *Service) CreateOrganisation(ctx context.Context, p *auth.Principal, name, slug string, m auth.Meta) (Organisation, error) {
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if slug == "" {
		slug = slugify(name)
	}
	switch {
	case name == "" || utf8.RuneCountInString(name) > 200:
		return Organisation{}, fieldErr("name", "enter a name of 1 to 200 characters")
	case len(slug) > 60 || !slugRe.MatchString(slug):
		return Organisation{}, fieldErr("slug", "use lower-case letters, digits and single hyphens, up to 60 characters")
	}
	var out Organisation
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		o, err := q.CreateOrganisation(ctx, store.CreateOrganisationParams{Name: name, Slug: slug})
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return ErrSlugTaken
		}
		if err != nil {
			return err
		}
		out = Organisation{ID: o.ID, Name: o.Name, Slug: o.Slug, Status: o.Status, CreatedAt: o.CreatedAt}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "org.created", EntityType: "organisation",
			EntityID: &o.ID, OrgID: &o.ID, After: map[string]any{"name": name, "slug": slug}})
	})
	return out, err
}

// Skill is a requirement a contributor must meet to get the project's tasks.
type Skill struct {
	Kind           string  `json:"kind"` // language, domain or task_type
	Value          string  `json:"value"`
	MinProficiency *string `json:"min_proficiency,omitempty"` // languages only: native, fluent, working
}

// ProjectInput is the create and edit form. Org and task type are fixed after creation.
type ProjectInput struct {
	OrgID             uuid.UUID       `json:"org_id"`
	TaskType          string          `json:"task_type"`
	Name              string          `json:"name"`
	InstructionsMd    string          `json:"instructions_md"`
	Config            json.RawMessage `json:"config"`
	QAChecks          json.RawMessage `json:"qa_checks"`
	OverlapK          int             `json:"overlap_k"`
	GoldRate          float64         `json:"gold_rate"`
	PayPerTaskCents   int64           `json:"pay_per_task_cents"`
	TargetCount       *int32          `json:"target_count"`
	Deadline          *string         `json:"deadline"` // YYYY-MM-DD
	MinRationaleChars int32           `json:"min_rationale_chars"`
	Skills            []Skill         `json:"skills"`
}

// Project is a project as the workspace shows it. Fields marked "staff" are
// left out for client users.
type Project struct {
	ID             uuid.UUID             `json:"id"`
	OrgID          uuid.UUID             `json:"org_id"`
	OrgName        string                `json:"org_name"`
	TaskType       string                `json:"task_type"`
	Name           string                `json:"name"`
	Status         string                `json:"status"`
	InstructionsMd string                `json:"instructions_md"`
	Config         quality.ProjectConfig `json:"config"`
	OverlapK       int16                 `json:"overlap_k"`
	GoldRate       float64               `json:"gold_rate"`
	TargetCount    *int32                `json:"target_count"`
	Deadline       *string               `json:"deadline"`
	ItemCount      int32                 `json:"item_count"`
	Skills         []Skill               `json:"skills"`
	PublishedAt    *time.Time            `json:"published_at"`
	ClosedAt       *time.Time            `json:"closed_at"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	Staff          *StaffFields          `json:"staff,omitempty"`
}

// StaffFields are visible to ELS staff only.
type StaffFields struct {
	PayPerTaskCents   int64            `json:"pay_per_task_cents"`
	Currency          string           `json:"currency"`
	QAChecks          quality.QAChecks `json:"qa_checks"`
	MinRationaleChars int32            `json:"min_rationale_chars"`
	GoldCount         int32            `json:"gold_count"`
	// Missing lists what stops the next step (ready or publish); empty when nothing does.
	Missing      []string `json:"missing"`
	ConfigLocked bool     `json:"config_locked"`
}

const dateLayout = "2006-01-02"

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
}

// cleaned is a validated ProjectInput.
type cleaned struct {
	in       ProjectInput
	config   quality.ProjectConfig
	qa       quality.QAChecks
	deadline *time.Time
	skills   []Skill
}

func (s *Service) validate(in ProjectInput, taskType string) (cleaned, error) {
	c := cleaned{in: in}
	in.Name = strings.TrimSpace(in.Name)
	c.in.Name = in.Name
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 200 {
		return c, fieldErr("name", "enter a name of 1 to 200 characters")
	}
	if utf8.RuneCountInString(in.InstructionsMd) > 50000 {
		return c, fieldErr("instructions_md", "keep instructions under 50,000 characters")
	}
	var err error
	if c.config, err = quality.ValidateConfig(taskType, in.Config); err != nil {
		return c, fromQuality(err)
	}
	if c.qa, err = quality.ValidateQAChecks(taskType, in.QAChecks); err != nil {
		return c, fromQuality(err)
	}
	switch {
	case in.OverlapK < 1 || in.OverlapK > 5:
		return c, fieldErr("overlap_k", "use 1 to 5 contributors per item")
	case math.IsNaN(in.GoldRate) || in.GoldRate < 0 || in.GoldRate > 0.5:
		return c, fieldErr("gold_rate", "use a share from 0 to 0.5, for example 0.1")
	case math.Abs(math.Round(in.GoldRate*1000)-in.GoldRate*1000) > 1e-9:
		return c, fieldErr("gold_rate", "use at most three decimal places")
	case in.PayPerTaskCents < 0 || in.PayPerTaskCents > 10_000_000:
		return c, fieldErr("pay_per_task_cents", "use 0 to 100,000.00 KES in cents")
	case in.TargetCount != nil && (*in.TargetCount < 1 || *in.TargetCount > 10_000_000):
		return c, fieldErr("target_count", "use 1 to 10,000,000 items")
	case in.MinRationaleChars < 0 || in.MinRationaleChars > 2000:
		return c, fieldErr("min_rationale_chars", "use 0 to 2000 characters")
	case in.MinRationaleChars > 0 && taskType != quality.Pairwise:
		return c, fieldErr("min_rationale_chars", "applies to pairwise comparison only")
	}
	if in.Deadline != nil && *in.Deadline != "" {
		d, err := time.Parse(dateLayout, *in.Deadline)
		if err != nil {
			return c, fieldErr("deadline", "use a date such as 2026-12-31")
		}
		c.deadline = &d
	}
	if len(in.Skills) > 20 {
		return c, fieldErr("skills", "list at most 20 skills")
	}
	seen := map[string]bool{}
	for _, sk := range in.Skills {
		sk.Value = strings.TrimSpace(sk.Value)
		ok := false
		switch sk.Kind {
		case "language":
			ok = langRe.MatchString(sk.Value) && (sk.MinProficiency == nil ||
				slices.Contains([]string{"native", "fluent", "working"}, *sk.MinProficiency))
		case "domain":
			ok = slices.Contains(contributors.Domains, sk.Value) && sk.MinProficiency == nil
		case "task_type":
			ok = slices.Contains(quality.TaskTypes, sk.Value) && sk.MinProficiency == nil
		}
		if !ok {
			return c, fieldErr("skills", "%s %q is not a valid requirement", sk.Kind, sk.Value)
		}
		if seen[sk.Kind+"/"+sk.Value] {
			return c, fieldErr("skills", "%s %q is listed twice", sk.Kind, sk.Value)
		}
		seen[sk.Kind+"/"+sk.Value] = true
		c.skills = append(c.skills, sk)
	}
	return c, nil
}

var langRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func (s *Service) CreateProject(ctx context.Context, p *auth.Principal, in ProjectInput, m auth.Meta) (Project, error) {
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		out, err = s.createProject(ctx, store.New(tx), p, in, m)
		return err
	})
	return out, err
}

func (s *Service) createProject(ctx context.Context, q *store.Queries, p *auth.Principal, in ProjectInput, m auth.Meta) (Project, error) {
	if !slices.Contains(quality.TaskTypes, in.TaskType) {
		return Project{}, fieldErr("task_type", "use classification, pairwise or translation_review")
	}
	c, err := s.validate(in, in.TaskType)
	if err != nil {
		return Project{}, err
	}
	if _, err := q.GetOrganisation(ctx, in.OrgID); errors.Is(err, pgx.ErrNoRows) {
		return Project{}, fieldErr("org_id", "organisation not found")
	} else if err != nil {
		return Project{}, err
	}
	tpl, err := q.GetLatestTaskTemplate(ctx, in.TaskType)
	if err != nil {
		return Project{}, err
	}
	pr, err := q.InsertProject(ctx, store.InsertProjectParams{
		OrgID: in.OrgID, TemplateID: tpl.ID, TaskType: in.TaskType, Name: c.in.Name,
		InstructionsMd: in.InstructionsMd, Config: mustJSON(c.config), QaChecks: mustJSON(c.qa),
		OverlapK: int16(in.OverlapK), GoldRate: in.GoldRate, PayPerTaskCents: in.PayPerTaskCents, Currency: "KES",
		TargetCount: in.TargetCount, Deadline: c.deadline, MinRationaleChars: in.MinRationaleChars, CreatedBy: &p.UserID,
	})
	if err != nil {
		return Project{}, err
	}
	if err := replaceSkills(ctx, q, pr, c.skills); err != nil {
		return Project{}, err
	}
	if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project.created", EntityType: "project",
		EntityID: &pr.ID, OrgID: &pr.OrgID, After: auditSettings(pr, c)}); err != nil {
		return Project{}, err
	}
	return s.load(ctx, q, p, pr.ID)
}

func auditSettings(pr store.Project, c cleaned) map[string]any {
	return map[string]any{"name": c.in.Name, "task_type": pr.TaskType, "overlap_k": c.in.OverlapK,
		"gold_rate": c.in.GoldRate, "pay_per_task_cents": c.in.PayPerTaskCents, "config": c.config,
		"qa_checks": c.qa, "target_count": c.in.TargetCount, "deadline": dateString(c.deadline), "skills": c.skills}
}

func replaceSkills(ctx context.Context, q *store.Queries, pr store.Project, skills []Skill) error {
	if err := q.DeleteProjectSkills(ctx, pr.ID); err != nil {
		return err
	}
	for _, sk := range skills {
		if err := q.AddProjectSkill(ctx, store.AddProjectSkillParams{OrgID: pr.OrgID, ProjectID: pr.ID,
			SkillKind: sk.Kind, SkillValue: sk.Value, MinProficiency: sk.MinProficiency}); err != nil {
			return err
		}
	}
	return nil
}

// configLocked: the task settings (labels, rubric) and overlap cannot change
// once the project has been live or holds gold, because submissions and
// expected answers depend on them.
func configLocked(pr store.Project, gold int32) bool {
	return pr.PublishedAt != nil || gold > 0
}

func (s *Service) UpdateProject(ctx context.Context, p *auth.Principal, id uuid.UUID, in ProjectInput, m auth.Meta) (Project, error) {
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.LockProject(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if pr.Status == StatusClosed {
			return ErrClosed
		}
		if (in.OrgID != uuid.Nil && in.OrgID != pr.OrgID) || (in.TaskType != "" && in.TaskType != pr.TaskType) {
			return fieldErr("task_type", "the organisation and task type are fixed once a project exists")
		}
		c, err := s.validate(in, pr.TaskType)
		if err != nil {
			return err
		}
		sum, err := q.GetProjectSummary(ctx, id)
		if err != nil {
			return err
		}
		if configLocked(pr, sum.GoldCount) {
			var old quality.ProjectConfig
			if err := json.Unmarshal(pr.Config, &old); err != nil {
				return err
			}
			if string(mustJSON(old)) != string(mustJSON(c.config)) {
				return &FieldError{Field: "config", Message: "labels and rubric are fixed once the project has gold or has been live"}
			}
			if int16(in.OverlapK) != pr.OverlapK {
				return &FieldError{Field: "overlap_k", Message: "overlap is fixed once the project has gold or has been live"}
			}
		}
		before, err := s.settingsOf(ctx, q, pr)
		if err != nil {
			return err
		}
		upd, err := q.UpdateProjectSettings(ctx, store.UpdateProjectSettingsParams{
			ID: id, Name: c.in.Name, InstructionsMd: in.InstructionsMd, Config: mustJSON(c.config), QaChecks: mustJSON(c.qa),
			OverlapK: int16(in.OverlapK), GoldRate: in.GoldRate, PayPerTaskCents: in.PayPerTaskCents,
			TargetCount: in.TargetCount, Deadline: c.deadline, MinRationaleChars: in.MinRationaleChars,
		})
		if err != nil {
			return err
		}
		if err := replaceSkills(ctx, q, upd, c.skills); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project.updated", EntityType: "project",
			EntityID: &id, OrgID: &upd.OrgID, Before: before, After: auditSettings(upd, c)}); err != nil {
			return err
		}
		out, err = s.load(ctx, q, p, id)
		return err
	})
	return out, err
}

// settingsOf rebuilds the audit view of stored settings.
func (s *Service) settingsOf(ctx context.Context, q *store.Queries, pr store.Project) (map[string]any, error) {
	skills, err := q.ListProjectSkills(ctx, pr.ID)
	if err != nil {
		return nil, err
	}
	var cfg, qa any
	_ = json.Unmarshal(pr.Config, &cfg)
	_ = json.Unmarshal(pr.QaChecks, &qa)
	sk := make([]Skill, 0, len(skills))
	for _, x := range skills {
		sk = append(sk, Skill{Kind: x.SkillKind, Value: x.SkillValue, MinProficiency: x.MinProficiency})
	}
	return map[string]any{"name": pr.Name, "task_type": pr.TaskType, "overlap_k": pr.OverlapK, "gold_rate": pr.GoldRate,
		"pay_per_task_cents": pr.PayPerTaskCents, "config": cfg, "qa_checks": qa, "target_count": pr.TargetCount,
		"deadline": dateString(pr.Deadline), "skills": sk}, nil
}

// missing lists what stops the project's next step: going ready needs
// instructions; going (or returning) live also needs items, and active gold
// when the gold rate is above zero (brief 9).
func missing(pr store.Project, items, gold int32) []string {
	out := []string{}
	if strings.TrimSpace(pr.InstructionsMd) == "" {
		out = append(out, "instructions")
	}
	if pr.Status == StatusDraft {
		return out
	}
	if items == 0 {
		out = append(out, "items")
	}
	if pr.GoldRate > 0 && gold == 0 {
		out = append(out, "gold")
	}
	return out
}

func (s *Service) SetStatus(ctx context.Context, p *auth.Principal, id uuid.UUID, action, reason string, m auth.Meta) (Project, error) {
	act, ok := actions[action]
	if !ok {
		return Project{}, fieldErr("action", "use ready, draft, publish, pause, resume or close")
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 1000 {
		return Project{}, fieldErr("reason", "keep the reason under 1000 characters")
	}
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.LockProject(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !slices.Contains(act.from, pr.Status) {
			return ErrIllegalTransition
		}
		if act.to == StatusReady || act.to == StatusLive {
			sum, err := q.GetProjectSummary(ctx, id)
			if err != nil {
				return err
			}
			check := pr
			if act.to == StatusLive {
				check.Status = StatusLive
			}
			if miss := missing(check, sum.ItemCount, sum.GoldCount); len(miss) > 0 {
				return &NotReadyError{Missing: miss}
			}
		}
		var published, closed *time.Time
		now := s.now()
		if act.to == StatusLive {
			published = &now
		}
		if act.to == StatusClosed {
			closed = &now
		}
		if _, err := q.SetProjectStatus(ctx, store.SetProjectStatusParams{ID: id, Status: act.to, PublishedAt: published, ClosedAt: closed}); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project.status_changed", EntityType: "project",
			EntityID: &id, OrgID: &pr.OrgID, Reason: reason,
			Before: map[string]any{"status": pr.Status}, After: map[string]any{"status": act.to}}); err != nil {
			return err
		}
		out, err = s.load(ctx, q, p, id)
		return err
	})
	return out, err
}

func (s *Service) ListProjects(ctx context.Context, p *auth.Principal, orgID *uuid.UUID, status string) ([]Project, error) {
	var st *string
	if status != "" {
		st = &status
	}
	out := []Project{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListProjectSummaries(ctx, store.ListProjectSummariesParams{OrgID: orgID, Status: st})
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, toProject(p, r.Project, r.OrgName, r.ItemCount, r.GoldCount, nil))
		}
		return nil
	})
	return out, err
}

func (s *Service) GetProject(ctx context.Context, p *auth.Principal, id uuid.UUID) (Project, error) {
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		out, err = s.load(ctx, store.New(tx), p, id)
		return err
	})
	return out, err
}

func (s *Service) load(ctx context.Context, q *store.Queries, p *auth.Principal, id uuid.UUID) (Project, error) {
	r, err := q.GetProjectSummary(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	skills, err := q.ListProjectSkills(ctx, id)
	if err != nil {
		return Project{}, err
	}
	return toProject(p, r.Project, r.OrgName, r.ItemCount, r.GoldCount, skills), nil
}

func toProject(p *auth.Principal, pr store.Project, orgName string, items, gold int32, skills []store.ProjectSkill) Project {
	var cfg quality.ProjectConfig
	_ = json.Unmarshal(pr.Config, &cfg)
	out := Project{ID: pr.ID, OrgID: pr.OrgID, OrgName: orgName, TaskType: pr.TaskType, Name: pr.Name, Status: pr.Status,
		InstructionsMd: pr.InstructionsMd, Config: cfg, OverlapK: pr.OverlapK, GoldRate: pr.GoldRate,
		TargetCount: pr.TargetCount, Deadline: dateString(pr.Deadline), ItemCount: items, Skills: []Skill{},
		PublishedAt: pr.PublishedAt, ClosedAt: pr.ClosedAt, CreatedAt: pr.CreatedAt, UpdatedAt: pr.UpdatedAt}
	for _, sk := range skills {
		out.Skills = append(out.Skills, Skill{Kind: sk.SkillKind, Value: sk.SkillValue, MinProficiency: sk.MinProficiency})
	}
	if p.Kind == auth.KindStaff {
		var qa quality.QAChecks
		if err := json.Unmarshal(pr.QaChecks, &qa); err != nil || string(pr.QaChecks) == "{}" {
			qa = quality.DefaultQAChecks(pr.TaskType)
		}
		next := pr
		if pr.Status == StatusReady || pr.Status == StatusPaused {
			next.Status = StatusLive
		}
		out.Staff = &StaffFields{PayPerTaskCents: pr.PayPerTaskCents, Currency: pr.Currency, QAChecks: qa,
			MinRationaleChars: pr.MinRationaleChars, GoldCount: gold, Missing: missing(next, items, gold),
			ConfigLocked: configLocked(pr, gold)}
	}
	return out
}

// TaskTemplate describes a task type's import format.
type TaskTemplate struct {
	TaskType      string   `json:"task_type"`
	Name          string   `json:"name"`
	Version       int32    `json:"version"`
	ContentFields []string `json:"content_fields"`
	ItemColumns   []string `json:"item_columns"`
	GoldColumns   []string `json:"gold_columns"`
}

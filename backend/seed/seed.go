// Package seed loads obviously fake sample data for development and tests:
// "(Sample)" organisations and projects, .invalid emails and placeholder
// phone numbers. The command in cmd/seed refuses to run outside development
// and test; Run itself assumes the caller has checked.
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/identity"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ErrAlreadySeeded means the sample organisation exists; nothing was changed.
var ErrAlreadySeeded = errors.New("seed: sample data already loaded")

// Slugs of the two sample organisations. Two exist so tenancy can be shown.
const (
	SampleOrgSlug = "sample-client-ltd"
	OtherOrgSlug  = "other-sample-org"
)

// Options holds the sample sign-in password. Every seeded user gets it.
type Options struct {
	Password string
}

// Summary counts what Run created.
type Summary struct {
	Users, Organisations, Contributors, Projects, Items, GoldItems, QualificationTests int
}

// Run loads the sample data in one staff-scoped transaction as mezai_app.
func Run(ctx context.Context, pool *db.Pool, opt Options) (Summary, error) {
	if len(opt.Password) < 12 {
		return Summary{}, errors.New("seed: password must be at least 12 characters")
	}
	hash, err := password.Hash(opt.Password)
	if err != nil {
		return Summary{}, err
	}
	var sum Summary
	err = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		s := &seeder{q: store.New(tx), ctx: ctx, hash: hash, sum: &sum, now: time.Now()}
		return s.run()
	})
	return sum, err
}

type seeder struct {
	q    *store.Queries
	ctx  context.Context
	hash string
	sum  *Summary
	now  time.Time
	// Active and paused sample contributors, qualified on every sample test
	// so the task engine has eligible people in development.
	qualified []uuid.UUID
	admin     uuid.UUID
}

func (s *seeder) run() error {
	if _, err := s.q.GetOrganisationBySlug(s.ctx, SampleOrgSlug); err == nil {
		return ErrAlreadySeeded
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	admin, err := s.user("superadmin@sample.invalid", "Sample Super Admin", "super_admin", nil)
	if err != nil {
		return err
	}
	if _, err := s.user("ops@sample.invalid", "Sample Ops Manager", "ops_manager", nil); err != nil {
		return err
	}
	if _, err := s.user("approver@sample.invalid", "Sample Payout Approver", "payout_approver", nil); err != nil {
		return err
	}
	if _, err := s.user("reviewer@sample.invalid", "Sample Reviewer", "reviewer", nil); err != nil {
		return err
	}

	sample, err := s.org("Sample Client Ltd (Sample)", SampleOrgSlug)
	if err != nil {
		return err
	}
	other, err := s.org("Other Sample Org (Sample)", OtherOrgSlug)
	if err != nil {
		return err
	}
	for _, c := range []struct {
		email, name, role string
		org               uuid.UUID
	}{
		{"client.admin@sample.invalid", "Sample Client Admin", "client_admin", sample},
		{"client.viewer@sample.invalid", "Sample Client Viewer", "client_viewer", sample},
		{"other.admin@sample.invalid", "Other Sample Client Admin", "client_admin", other},
	} {
		org := c.org
		u, err := s.user(c.email, c.name, c.role, &org)
		if err != nil {
			return err
		}
		if err := s.q.AddOrgMember(s.ctx, store.AddOrgMemberParams{OrgID: org, UserID: u}); err != nil {
			return err
		}
	}

	s.admin = admin
	if err := s.contributors(admin); err != nil {
		return err
	}

	labels := map[string]any{"labels": classificationLabels, "multi_label": false}
	rubric := map[string]any{"criteria": pairwiseRubric, "scale": []int{1, 5}}
	trRubric := map[string]any{"criteria": []string{"adequacy", "fluency"}, "scale": []int{1, 5}}
	projects := []struct {
		org                 uuid.UUID
		name, taskType      string
		status              string
		config              map[string]any
		items               []sampleItem
		overlap             int16
		payCents            int64
		skillKind, skillVal string
	}{
		{sample, "Customer message routing (Sample)", "classification", "live", labels, classificationItems, 3, 1000, "language", "sw"},
		{sample, "Helpful answers comparison (Sample)", "pairwise", "live", rubric, pairwiseItems, 2, 4000, "task_type", "pairwise"},
		{sample, "English to Swahili review (Sample)", "translation_review", "draft", trRubric, translationItems, 2, 3000, "language", "sw"},
		{other, "Other org message routing (Sample)", "classification", "live", labels, classificationItems, 2, 1000, "language", "sw"},
	}
	for _, p := range projects {
		if err := s.project(p.org, admin, p.name, p.taskType, p.status, p.config, p.items, p.overlap, p.payCents, p.skillKind, p.skillVal); err != nil {
			return fmt.Errorf("project %q: %w", p.name, err)
		}
	}
	return s.qualificationTests(labels, rubric, trRubric)
}

func (s *seeder) user(email, name, role string, org *uuid.UUID) (uuid.UUID, error) {
	verified := s.now
	u, err := s.q.CreateUser(s.ctx, store.CreateUserParams{
		Email: email, DisplayName: name, PasswordHash: &s.hash, EmailVerifiedAt: &verified,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("user %s: %w", email, err)
	}
	r, err := s.q.GetRoleByKey(s.ctx, role)
	if err != nil {
		return uuid.Nil, fmt.Errorf("role %s: %w", role, err)
	}
	if err := s.q.AddUserRole(s.ctx, store.AddUserRoleParams{UserID: u.ID, RoleID: r.ID, OrgID: org}); err != nil {
		return uuid.Nil, err
	}
	s.sum.Users++
	return u.ID, nil
}

func (s *seeder) org(name, slug string) (uuid.UUID, error) {
	o, err := s.q.CreateOrganisation(s.ctx, store.CreateOrganisationParams{Name: name, Slug: slug})
	if err != nil {
		return uuid.Nil, fmt.Errorf("organisation %s: %w", slug, err)
	}
	s.sum.Organisations++
	return o.ID, nil
}

// contributors creates "Sample Contributor 001" to "012" with a spread of
// statuses. Phone numbers are placeholders in the +254 700 000 0NN pattern;
// they are not known to be reserved, and the seed never sends SMS or payments
// (A-014).
func (s *seeder) contributors(admin uuid.UUID) error {
	statuses := []string{"active", "active", "active", "active", "active", "active", "active", "active", "qualifying", "applied", "paused", "removed"}
	domains := []string{"health", "education", "agriculture", "finance", "software", "law"}
	for i, status := range statuses {
		n := i + 1
		u, err := s.user(fmt.Sprintf("contributor%03d@sample.invalid", n), fmt.Sprintf("Sample Contributor %03d", n), "contributor", nil)
		if err != nil {
			return err
		}
		country, city, edu := "KE", "Nairobi", "bachelor"
		hours := int16(10 + i%4*5)
		phone := fmt.Sprintf("+2547000000%02d", n)
		confirmed := s.now
		cp, err := s.q.CreateContributorProfile(s.ctx, store.CreateContributorProfileParams{
			UserID: u, Status: status, Country: &country, City: &city, EducationLevel: &edu,
			WeeklyHours: &hours, MpesaPhone: &phone, MpesaPhoneConfirmedAt: &confirmed,
		})
		if err != nil {
			return fmt.Errorf("contributor %d: %w", n, err)
		}
		langs := []struct{ code, prof string }{{"sw", "native"}, {"en", "fluent"}}
		if n%3 == 0 {
			langs = []struct{ code, prof string }{{"en", "native"}, {"sw", "working"}}
		}
		for _, l := range langs {
			dialects := []string{}
			if l.code == "sw" && n%2 == 0 {
				dialects = []string{"Sheng"}
			}
			if err := s.q.AddContributorLanguage(s.ctx, store.AddContributorLanguageParams{
				ContributorID: cp.ID, Language: l.code, Proficiency: l.prof, Dialects: dialects,
			}); err != nil {
				return err
			}
		}
		if err := s.q.AddContributorDomain(s.ctx, store.AddContributorDomainParams{ContributorID: cp.ID, Domain: domains[i%len(domains)]}); err != nil {
			return err
		}
		// Identity (D-078): a fingerprint of "sample-NNN", which no real
		// National ID number can produce (they are digits only). Everyone
		// past qualifying is verified; the qualifying one waits for Ops.
		if status != "applied" {
			last3 := fmt.Sprintf("%03d", n)
			if err := s.q.SetContributorNationalID(s.ctx, store.SetContributorNationalIDParams{
				ID: cp.ID, NationalIDFp: identity.Fingerprint(identity.DevKey, "sample-"+last3), NationalIDLast3: &last3,
			}); err != nil {
				return err
			}
		}
		// Application (D-100): everyone past "applied" has filled it in;
		// people already working were shortlisted and passed the interview.
		if status != "applied" {
			exp := mustJSON(map[string]string{
				"highest_study":       "Sample diploma in communication",
				"relevant_experience": "Sample experience: two years answering customer messages in Kiswahili and English.",
				"ai_data_experience":  "",
				"why":                 "Sample answer: I enjoy careful reading and want flexible work.",
			})
			if err := s.q.UpdateContributorApplication(s.ctx, store.UpdateContributorApplicationParams{ID: cp.ID, Experience: exp}); err != nil {
				return err
			}
		}
		if status == "active" || status == "paused" {
			note := "Sample data"
			if err := s.q.DecideScreening(s.ctx, store.DecideScreeningParams{ID: cp.ID, ScreeningStatus: "shortlisted", ScreeningNote: &note, DecidedBy: &admin}); err != nil {
				return err
			}
			scenario, ok := int16(4), true
			if _, err := s.q.AddInterview(s.ctx, store.AddInterviewParams{ContributorID: cp.ID, InterviewerID: admin, Outcome: "passed",
				IdentityConfirmed: true, LanguageScores: mustJSON(map[string]int{"sw": 4, "en": 4}), ScenarioScore: &scenario,
				ConfidentialityOk: &ok, Note: &note}); err != nil {
				return err
			}
		}
		if status != "applied" && status != "qualifying" {
			note := "Sample data"
			if err := s.q.DecideContributorIdentity(s.ctx, store.DecideContributorIdentityParams{
				ID: cp.ID, IdentityStatus: "verified", IdentityNote: &note, DecidedBy: &admin,
			}); err != nil {
				return err
			}
		}
		if status != "applied" {
			if err := s.q.AddContributorStatusHistory(s.ctx, store.AddContributorStatusHistoryParams{
				ContributorID: cp.ID, FromStatus: ptr("applied"), ToStatus: status, ChangedBy: &admin, Reason: "Sample data",
			}); err != nil {
				return err
			}
		}
		if status == "active" || status == "paused" {
			s.qualified = append(s.qualified, cp.ID)
		}
		s.sum.Contributors++
	}
	return nil
}

func (s *seeder) project(org, admin uuid.UUID, name, taskType, status string, config map[string]any,
	items []sampleItem, overlap int16, payCents int64, skillKind, skillVal string) error {
	tpl, err := s.q.GetLatestTaskTemplate(s.ctx, taskType)
	if err != nil {
		return err
	}
	var published *time.Time
	if status == "live" {
		published = &s.now
	}
	target := int32(len(items) * 50)
	deadline := s.now.AddDate(0, 2, 0)
	p, err := s.q.CreateProject(s.ctx, store.CreateProjectParams{
		OrgID: org, TemplateID: tpl.ID, TaskType: taskType, Name: name, Status: status,
		InstructionsMd: "Sample instructions. Read each item carefully and follow the project guidelines.",
		Config:         mustJSON(config), OverlapK: overlap, GoldRate: 0.10, PayPerTaskCents: payCents,
		Currency: "KES", TargetCount: &target, Deadline: &deadline, PublishedAt: published, CreatedBy: &admin,
	})
	if err != nil {
		return err
	}
	s.sum.Projects++
	var minProf *string
	if skillKind == "language" {
		minProf = ptr("fluent")
	}
	if err := s.q.AddProjectSkill(s.ctx, store.AddProjectSkillParams{
		OrgID: org, ProjectID: p.ID, SkillKind: skillKind, SkillValue: skillVal, MinProficiency: minProf,
	}); err != nil {
		return err
	}
	if err := s.q.AddProjectGuideline(s.ctx, store.AddProjectGuidelineParams{
		OrgID: org, ProjectID: p.ID, Title: "How to approach this project (Sample)",
		BodyMd: "Sample guideline. When an item is unclear, skip it with the reason \"unclear\".", Position: 1,
	}); err != nil {
		return err
	}
	for _, it := range items {
		ext := it.externalID
		item, err := s.q.CreateItem(s.ctx, store.CreateItemParams{OrgID: org, ProjectID: p.ID, ExternalID: &ext, Content: mustJSON(it.content)})
		if err != nil {
			return err
		}
		s.sum.Items++
		if it.gold {
			if _, err := s.q.CreateGoldItem(s.ctx, store.CreateGoldItemParams{
				OrgID: org, ProjectID: p.ID, ItemID: item.ID, Expected: mustJSON(it.answer), Tolerance: []byte(`{}`), CreatedBy: &admin,
			}); err != nil {
				return err
			}
			s.sum.GoldItems++
		}
	}
	return nil
}

func (s *seeder) qualificationTests(labels, rubric, trRubric map[string]any) error {
	tests := []struct {
		title, skillKind, skillVal, taskType string
		config                               map[string]any
		items                                []sampleItem
	}{
		{"Swahili message routing test (Sample)", "language", "sw", "classification", labels, classificationItems[:4]},
		{"Response comparison test (Sample)", "task_type", "pairwise", "pairwise", rubric, pairwiseItems[:2]},
		{"English to Swahili review test (Sample)", "language", "sw", "translation_review", trRubric, translationItems[:2]},
	}
	for _, t := range tests {
		qt, err := s.q.CreateQualificationTest(s.ctx, store.CreateQualificationTestParams{
			Title: t.title, SkillKind: t.skillKind, SkillValue: t.skillVal, TaskType: t.taskType,
			PassThreshold: 0.8, InstructionsMd: "Sample qualification test.", Status: "active", Config: mustJSON(t.config),
		})
		if err != nil {
			return err
		}
		for i, it := range t.items {
			if err := s.q.AddQualificationQuestion(s.ctx, store.AddQualificationQuestionParams{
				TestID: qt.ID, Position: int32(i + 1), Content: mustJSON(it.content), Expected: mustJSON(it.answer), Tolerance: []byte(`{}`),
			}); err != nil {
				return err
			}
		}
		for _, c := range s.qualified {
			a, err := s.q.CreateQualificationAttempt(s.ctx, store.CreateQualificationAttemptParams{TestID: qt.ID, ContributorID: c})
			if err != nil {
				return err
			}
			score, passed := 1.0, true
			if err := s.q.ScoreAttempt(s.ctx, store.ScoreAttemptParams{ID: a.ID, Answers: []byte(`{}`), Results: []byte(`{}`),
				Score: &score, Passed: &passed}); err != nil {
				return err
			}
			note := "Sample data"
			if _, err := s.q.DecideAttempt(s.ctx, store.DecideAttemptParams{ID: a.ID, Status: "approved", DecidedBy: &s.admin, DecisionNote: &note}); err != nil {
				return err
			}
		}
		s.sum.QualificationTests++
	}
	// The generalist core check (D-100), passed by everyone already working.
	core, err := contributors.InsertSampleCoreCheck(s.ctx, s.q, s.admin)
	if err != nil {
		return err
	}
	s.sum.QualificationTests++
	for _, c := range s.qualified {
		a, err := s.q.CreateQualificationAttempt(s.ctx, store.CreateQualificationAttemptParams{TestID: core.ID, ContributorID: c})
		if err != nil {
			return err
		}
		score, passed, rating, note := 1.0, true, int16(3), "Sample data"
		if err := s.q.ScoreAttempt(s.ctx, store.ScoreAttemptParams{ID: a.ID, Answers: []byte(`{}`), Results: []byte(`{}`),
			Score: &score, Passed: &passed}); err != nil {
			return err
		}
		if _, err := s.q.DecideAttempt(s.ctx, store.DecideAttemptParams{ID: a.ID, Status: "approved", DecidedBy: &s.admin,
			DecisionNote: &note, WritingRating: &rating}); err != nil {
			return err
		}
	}
	return nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // only static sample data reaches here
	}
	return b
}

func ptr[T any](v T) *T { return &v }

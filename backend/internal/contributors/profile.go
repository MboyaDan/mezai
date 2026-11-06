package contributors

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/legal"
	"github.com/MboyaDan/mezai/backend/internal/pii"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Allowed values (brief 6.2; the same lists are CHECK constraints in 00003).
var (
	Proficiencies   = []string{"native", "fluent", "working"}
	Domains         = []string{"health", "law", "finance", "education", "software", "agriculture", "other"}
	EducationLevels = []string{"secondary", "certificate", "diploma", "bachelor", "master", "doctorate", "other"}

	languageRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
)

// Language is one language a contributor works in.
type Language struct {
	Language    string   `json:"language"`
	Proficiency string   `json:"proficiency"`
	Dialects    []string `json:"dialects"`
}

// Consent is the latest accepted version of one document.
type Consent struct {
	Document   string    `json:"document"`
	Version    string    `json:"version"`
	AcceptedAt time.Time `json:"accepted_at"`
}

// Qualification is a skill granted by an approved attempt.
type Qualification struct {
	TestID     uuid.UUID `json:"test_id"`
	Title      string    `json:"title"`
	SkillKind  string    `json:"skill_kind"`
	SkillValue string    `json:"skill_value"`
	ApprovedAt time.Time `json:"approved_at"`
}

// Profile is a contributor's profile as they (or Ops) see it. The M-Pesa
// number is always masked in responses.
type Profile struct {
	ID                  uuid.UUID       `json:"id"`
	PublicCode          string          `json:"public_code"`
	Status              string          `json:"status"`
	Email               string          `json:"email"`
	DisplayName         string          `json:"display_name"`
	Country             *string         `json:"country"`
	City                *string         `json:"city"`
	EducationLevel      *string         `json:"education_level"`
	WeeklyHours         *int16          `json:"weekly_hours"`
	MpesaPhoneMasked    string          `json:"mpesa_phone_masked"`
	MpesaPhoneConfirmed bool            `json:"mpesa_phone_confirmed"`
	Languages           []Language      `json:"languages"`
	Domains             []string        `json:"domains"`
	Missing             []string        `json:"missing"` // fields still needed before qualification
	Consents            []Consent       `json:"consents"`
	ConsentRequired     []string        `json:"consent_required"` // documents to accept (again)
	Qualifications      []Qualification `json:"qualifications"`
	CreatedAt           time.Time       `json:"created_at"`
}

// ProfileInput replaces the editable profile. Empty M-Pesa fields keep the
// stored number; a new number must be entered twice.
type ProfileInput struct {
	DisplayName       string     `json:"display_name"`
	Country           *string    `json:"country"`
	City              *string    `json:"city"`
	EducationLevel    *string    `json:"education_level"`
	WeeklyHours       *int16     `json:"weekly_hours"`
	Languages         []Language `json:"languages"`
	Domains           []string   `json:"domains"`
	MpesaPhone        string     `json:"mpesa_phone"`
	MpesaPhoneConfirm string     `json:"mpesa_phone_confirm"`
}

func contributorOf(p *auth.Principal) (uuid.UUID, error) {
	if p == nil || p.Kind != auth.KindContributor || p.ContributorID == uuid.Nil {
		return uuid.Nil, ErrNotContributor
	}
	return p.ContributorID, nil
}

// MyProfile returns the caller's profile.
func (s *Service) MyProfile(ctx context.Context, p *auth.Principal) (Profile, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Profile{}, err
	}
	var out Profile
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		out, err = loadProfile(ctx, store.New(tx), id)
		return err
	})
	return out, err
}

func loadProfile(ctx context.Context, q *store.Queries, id uuid.UUID) (Profile, error) {
	cp, err := q.GetContributorProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	out := Profile{
		ID: cp.ID, PublicCode: cp.PublicCode, Status: cp.Status, Email: cp.Email, DisplayName: cp.DisplayName,
		Country: cp.Country, City: cp.City, EducationLevel: cp.EducationLevel, WeeklyHours: cp.WeeklyHours,
		MpesaPhoneConfirmed: cp.MpesaPhone != nil && cp.MpesaPhoneConfirmedAt != nil, CreatedAt: cp.CreatedAt,
		Languages: []Language{}, Consents: []Consent{}, Qualifications: []Qualification{},
	}
	if cp.MpesaPhone != nil {
		out.MpesaPhoneMasked = pii.MaskPhone(*cp.MpesaPhone)
	}
	langs, err := q.ListContributorLanguages(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	for _, l := range langs {
		out.Languages = append(out.Languages, Language{Language: l.Language, Proficiency: l.Proficiency, Dialects: l.Dialects})
	}
	if out.Domains, err = q.ListContributorDomains(ctx, id); err != nil {
		return Profile{}, err
	}
	consents, err := q.ListLatestConsents(ctx, cp.UserID)
	if err != nil {
		return Profile{}, err
	}
	accepted := map[string]string{}
	for _, c := range consents {
		out.Consents = append(out.Consents, Consent{Document: c.Document, Version: c.Version, AcceptedAt: c.AcceptedAt})
		accepted[c.Document] = c.Version
	}
	out.ConsentRequired = legal.Outstanding(accepted)
	attempts, err := q.ListAttemptsByContributor(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	for _, a := range attempts {
		if a.Status != "approved" || a.DecidedAt == nil {
			continue
		}
		t, err := q.GetQualificationTest(ctx, a.TestID)
		if err != nil {
			return Profile{}, err
		}
		out.Qualifications = append(out.Qualifications, Qualification{
			TestID: t.ID, Title: t.Title, SkillKind: t.SkillKind, SkillValue: t.SkillValue, ApprovedAt: *a.DecidedAt,
		})
	}
	out.Missing = missing(cp.DisplayName, store.ContributorProfile{
		Country: cp.Country, EducationLevel: cp.EducationLevel, WeeklyHours: cp.WeeklyHours,
		MpesaPhone: cp.MpesaPhone, MpesaPhoneConfirmedAt: cp.MpesaPhoneConfirmedAt,
	}, len(out.Languages), len(out.Domains))
	return out, nil
}

// missing lists the fields a profile needs before its owner can qualify.
func missing(name string, cp store.ContributorProfile, langs, domains int) []string {
	out := []string{}
	add := func(ok bool, field string) {
		if !ok {
			out = append(out, field)
		}
	}
	add(strings.TrimSpace(name) != "", "display_name")
	add(cp.Country != nil, "country")
	add(cp.EducationLevel != nil, "education_level")
	add(cp.WeeklyHours != nil && *cp.WeeklyHours > 0, "weekly_hours")
	add(langs > 0, "languages")
	add(domains > 0, "domains")
	add(cp.MpesaPhone != nil && cp.MpesaPhoneConfirmedAt != nil, "mpesa_phone")
	return out
}

func (in *ProfileInput) validate() error {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" || len(in.DisplayName) > 120 {
		return fieldErr("display_name", "enter a name of 1 to 120 characters")
	}
	trim := func(s *string) *string {
		if s == nil {
			return nil
		}
		t := strings.TrimSpace(*s)
		if t == "" {
			return nil
		}
		return &t
	}
	in.Country, in.City, in.EducationLevel = trim(in.Country), trim(in.City), trim(in.EducationLevel)
	if in.Country != nil {
		c := strings.ToUpper(*in.Country)
		if !countryRe.MatchString(c) {
			return fieldErr("country", "use a two-letter country code")
		}
		in.Country = &c
	}
	if in.City != nil && len(*in.City) > 100 {
		return fieldErr("city", "keep the city under 100 characters")
	}
	if in.EducationLevel != nil && !slices.Contains(EducationLevels, *in.EducationLevel) {
		return fieldErr("education_level", "choose an education level from the list")
	}
	if in.WeeklyHours != nil && (*in.WeeklyHours < 0 || *in.WeeklyHours > 80) {
		return fieldErr("weekly_hours", "enter 0 to 80 hours")
	}
	if len(in.Languages) > 10 {
		return fieldErr("languages", "list at most 10 languages")
	}
	seen := map[string]bool{}
	for i := range in.Languages {
		l := &in.Languages[i]
		l.Language = strings.TrimSpace(l.Language)
		if !languageRe.MatchString(l.Language) || seen[l.Language] {
			return fieldErr("languages", "use each language code once, for example sw or en")
		}
		seen[l.Language] = true
		if !slices.Contains(Proficiencies, l.Proficiency) {
			return fieldErr("languages", "choose native, fluent or working")
		}
		if len(l.Dialects) > 5 {
			return fieldErr("languages", "list at most 5 dialects per language")
		}
		ds := []string{}
		for _, d := range l.Dialects {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			if len(d) > 40 {
				return fieldErr("languages", "keep each dialect under 40 characters")
			}
			if !slices.Contains(ds, d) {
				ds = append(ds, d)
			}
		}
		l.Dialects = ds
	}
	if len(in.Domains) > len(Domains) {
		return fieldErr("domains", "choose each domain once")
	}
	doms := []string{}
	for _, d := range in.Domains {
		if !slices.Contains(Domains, d) || slices.Contains(doms, d) {
			return fieldErr("domains", "choose each domain once from the list")
		}
		doms = append(doms, d)
	}
	in.Domains = doms
	return nil
}

// UpdateMyProfile replaces the caller's profile. When an "applied" profile
// becomes complete the contributor moves to "qualifying".
func (s *Service) UpdateMyProfile(ctx context.Context, p *auth.Principal, in ProfileInput, m auth.Meta) (Profile, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Profile{}, err
	}
	if err := in.validate(); err != nil {
		return Profile{}, err
	}
	var phone *string
	if in.MpesaPhone != "" || in.MpesaPhoneConfirm != "" {
		a, err := pii.NormaliseKenyanMobile(in.MpesaPhone)
		if err != nil {
			return Profile{}, fieldErr("mpesa_phone", "enter a Kenyan mobile number, for example 0712 345 678")
		}
		b, err := pii.NormaliseKenyanMobile(in.MpesaPhoneConfirm)
		if err != nil || a != b {
			return Profile{}, fieldErr("mpesa_phone_confirm", "the two numbers do not match")
		}
		phone = &a
	}
	var out Profile
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cp, err := q.LockContributorProfile(ctx, id)
		if err != nil {
			return err
		}
		if cp.Status == StatusRemoved {
			return ErrNotEligible
		}
		phoneChanged := false
		confirmedAt := cp.MpesaPhoneConfirmedAt
		if phone != nil {
			phoneChanged = cp.MpesaPhone == nil || *cp.MpesaPhone != *phone
			if phoneChanged || confirmedAt == nil {
				now := s.now()
				confirmedAt = &now // entered twice and matching (brief 6.2)
			}
		} else {
			phone = cp.MpesaPhone
		}
		if err := q.UpdateContributorProfile(ctx, store.UpdateContributorProfileParams{
			ID: id, Country: in.Country, City: in.City, EducationLevel: in.EducationLevel, WeeklyHours: in.WeeklyHours,
			MpesaPhone: phone, MpesaPhoneConfirmedAt: confirmedAt,
		}); err != nil {
			return err
		}
		if err := q.SetUserDisplayName(ctx, store.SetUserDisplayNameParams{ID: p.UserID, DisplayName: in.DisplayName}); err != nil {
			return err
		}
		if err := q.ClearContributorLanguages(ctx, id); err != nil {
			return err
		}
		for _, l := range in.Languages {
			if err := q.AddContributorLanguage(ctx, store.AddContributorLanguageParams{
				ContributorID: id, Language: l.Language, Proficiency: l.Proficiency, Dialects: l.Dialects,
			}); err != nil {
				return err
			}
		}
		if err := q.ClearContributorDomains(ctx, id); err != nil {
			return err
		}
		for _, d := range in.Domains {
			if err := q.AddContributorDomain(ctx, store.AddContributorDomainParams{ContributorID: id, Domain: d}); err != nil {
				return err
			}
		}
		// The audit names what changed, never the phone number itself.
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.profile_updated", EntityType: "contributor", EntityID: &id,
			After: map[string]any{"languages": len(in.Languages), "domains": in.Domains, "mpesa_phone_changed": phoneChanged}}); err != nil {
			return err
		}
		out, err = loadProfile(ctx, q, id)
		if err != nil {
			return err
		}
		if out.Status == StatusApplied && len(out.Missing) == 0 {
			if err := transition(ctx, q, m, id, StatusApplied, StatusQualifying, ViaProfile, &p.UserID, "Profile completed"); err != nil {
				return err
			}
			out.Status = StatusQualifying
		}
		return nil
	})
	return out, err
}

// AcceptConsents records acceptance of the current version of each listed
// document (asked again after a version change).
func (s *Service) AcceptConsents(ctx context.Context, p *auth.Principal, accepted map[string]string, m auth.Meta) (Profile, error) {
	id, err := contributorOf(p)
	if err != nil {
		return Profile{}, err
	}
	if len(accepted) == 0 {
		return Profile{}, fieldErr("consents", "accept at least one document")
	}
	for d, v := range accepted {
		cur, ok := legal.Current[d]
		if !ok {
			return Profile{}, fieldErr("consents", "unknown document %q", d)
		}
		if v != cur {
			return Profile{}, ErrConsentRequired // stale page: reload and read the current version
		}
	}
	var out Profile
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		docs := make([]string, 0, len(accepted))
		for d := range accepted {
			docs = append(docs, d)
		}
		slices.Sort(docs)
		for _, d := range docs {
			if err := q.InsertConsent(ctx, store.InsertConsentParams{UserID: p.UserID, Document: d, Version: accepted[d], Ip: m.IP}); err != nil {
				return err
			}
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "contributor.consented", EntityType: "contributor", EntityID: &id,
			After: map[string]any{"consents": accepted}}); err != nil {
			return err
		}
		out, err = loadProfile(ctx, q, id)
		return err
	})
	return out, err
}

// transition changes a contributor's status if the state machine allows it,
// and records history and audit in the same transaction. by is nil for the
// system.
func transition(ctx context.Context, q *store.Queries, m auth.Meta, id uuid.UUID, from, to string, via Via, by *uuid.UUID, reason string) error {
	if !CanTransition(from, to, via) {
		return ErrIllegalTransition
	}
	n, err := q.SetContributorStatus(ctx, store.SetContributorStatusParams{ID: id, FromStatus: from, ToStatus: to})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIllegalTransition // changed by someone else meanwhile
	}
	if err := q.AddContributorStatusHistory(ctx, store.AddContributorStatusHistoryParams{
		ContributorID: id, FromStatus: &from, ToStatus: to, ChangedBy: by, Reason: reason,
	}); err != nil {
		return err
	}
	return auth.Audit(ctx, q, m, auth.Entry{Actor: by, Action: "contributor.status_changed", EntityType: "contributor", EntityID: &id,
		Reason: reason, Before: map[string]string{"status": from}, After: map[string]string{"status": to}})
}

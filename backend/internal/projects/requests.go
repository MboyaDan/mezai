package projects

import (
	"context"
	"errors"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	mailer "github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// RequestTaskTypes: the three pilot task types, or "other" for work that
// does not fit them yet (Ops decides).
var RequestTaskTypes = []string{"classification", "pairwise", "translation_review", "other"}

// SampleExtensions are the sample file types a client may attach.
var SampleExtensions = []string{".csv", ".tsv", ".jsonl", ".json", ".txt"}

// MaxSampleBytes bounds a request's sample file.
const MaxSampleBytes = 5 << 20

// RequestInput is the client's project request form.
type RequestInput struct {
	Title          string   `json:"title"`
	TaskType       string   `json:"task_type"`
	BriefMd        string   `json:"brief_md"`
	Languages      []string `json:"languages"`
	VolumeEstimate *int32   `json:"volume_estimate"`
	WantedBy       *string  `json:"wanted_by"` // YYYY-MM-DD
	ContactName    string   `json:"contact_name"`
	ContactEmail   string   `json:"contact_email"`
}

// Sample is an optional attached file.
type Sample struct {
	Name string
	Data []byte
}

// Request is a project request as both sides see it.
type Request struct {
	ID             uuid.UUID  `json:"id"`
	OrgID          uuid.UUID  `json:"org_id"`
	OrgName        string     `json:"org_name"`
	Title          string     `json:"title"`
	TaskType       string     `json:"task_type"`
	BriefMd        string     `json:"brief_md"`
	Languages      []string   `json:"languages"`
	VolumeEstimate *int32     `json:"volume_estimate"`
	WantedBy       *string    `json:"wanted_by"`
	ContactName    string     `json:"contact_name"`
	ContactEmail   string     `json:"contact_email"`
	SampleFileName *string    `json:"sample_file_name"`
	SampleSize     *int64     `json:"sample_size"`
	Status         string     `json:"status"` // submitted, converted or declined
	DecisionNote   *string    `json:"decision_note"`
	ProjectID      *uuid.UUID `json:"project_id"`
	DecidedAt      *time.Time `json:"decided_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func toRequest(r store.ProjectRequest, orgName string) Request {
	return Request{ID: r.ID, OrgID: r.OrgID, OrgName: orgName, Title: r.Title, TaskType: r.TaskType, BriefMd: r.BriefMd,
		Languages: r.Languages, VolumeEstimate: r.VolumeEstimate, WantedBy: dateString(r.WantedBy),
		ContactName: r.ContactName, ContactEmail: r.ContactEmail, SampleFileName: r.SampleFileName, SampleSize: r.SampleSize,
		Status: r.Status, DecisionNote: r.DecisionNote, ProjectID: r.ProjectID, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt}
}

func (s *Service) validateRequest(in *RequestInput) (*time.Time, error) {
	in.Title, in.BriefMd = strings.TrimSpace(in.Title), strings.TrimSpace(in.BriefMd)
	in.ContactName, in.ContactEmail = strings.TrimSpace(in.ContactName), strings.TrimSpace(in.ContactEmail)
	switch {
	case in.Title == "" || utf8.RuneCountInString(in.Title) > 200:
		return nil, fieldErr("title", "enter a title of 1 to 200 characters")
	case !slices.Contains(RequestTaskTypes, in.TaskType):
		return nil, fieldErr("task_type", "choose a task type")
	case in.BriefMd == "" || utf8.RuneCountInString(in.BriefMd) > 20000:
		return nil, fieldErr("brief_md", "describe the work in up to 20,000 characters")
	case len(in.Languages) > 20:
		return nil, fieldErr("languages", "list at most 20 languages")
	case in.VolumeEstimate != nil && (*in.VolumeEstimate < 1 || *in.VolumeEstimate > 10_000_000):
		return nil, fieldErr("volume_estimate", "use 1 to 10,000,000 items")
	case in.ContactName == "" || utf8.RuneCountInString(in.ContactName) > 200:
		return nil, fieldErr("contact_name", "enter a contact name")
	}
	if a, err := mail.ParseAddress(in.ContactEmail); err != nil || a.Address != in.ContactEmail || len(in.ContactEmail) > 254 {
		return nil, fieldErr("contact_email", "enter an email address")
	}
	langs := []string{}
	for _, l := range in.Languages {
		l = strings.TrimSpace(l)
		if !langRe.MatchString(l) {
			return nil, fieldErr("languages", "use language codes such as sw, en or luo")
		}
		if !slices.Contains(langs, l) {
			langs = append(langs, l)
		}
	}
	in.Languages = langs
	if in.WantedBy == nil || *in.WantedBy == "" {
		return nil, nil
	}
	d, err := time.Parse(dateLayout, *in.WantedBy)
	if err != nil {
		return nil, fieldErr("wanted_by", "use a date such as 2026-12-31")
	}
	if d.Before(s.now().Truncate(24 * time.Hour)) {
		return nil, fieldErr("wanted_by", "choose a date from today on")
	}
	return &d, nil
}

// CreateRequest saves a client's request in their own organisation and
// emails Ops. Only client users submit requests; staff create projects.
func (s *Service) CreateRequest(ctx context.Context, p *auth.Principal, in RequestInput, sample *Sample, m auth.Meta) (Request, error) {
	if p.Kind != auth.KindClient || p.OrgID == uuid.Nil {
		return Request{}, ErrForbidden
	}
	wanted, err := s.validateRequest(&in)
	if err != nil {
		return Request{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Request{}, err
	}
	var key, name *string
	var size *int64
	if sample != nil {
		n := cleanFileName(sample.Name)
		ext := strings.ToLower(n[strings.LastIndexByte(n, '.')+1:])
		if !strings.Contains(n, ".") || !slices.Contains(SampleExtensions, "."+ext) {
			return Request{}, fieldErr("sample", "attach a .csv, .tsv, .jsonl, .json or .txt file")
		}
		if len(sample.Data) == 0 || len(sample.Data) > MaxSampleBytes {
			return Request{}, fieldErr("sample", "attach a file of up to 5 MB")
		}
		k := "requests/" + p.OrgID.String() + "/" + id.String() + "." + ext
		if err := s.blobs.Put(ctx, k, "application/octet-stream", sample.Data); err != nil {
			return Request{}, errors.Join(ErrStorage, err)
		}
		sz := int64(len(sample.Data))
		key, name, size = &k, &n, &sz
	}
	var out Request
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		org, err := q.GetOrganisation(ctx, p.OrgID)
		if err != nil {
			return err
		}
		r, err := q.CreateProjectRequestWithID(ctx, store.CreateProjectRequestWithIDParams{ID: id, OrgID: p.OrgID,
			Title: in.Title, TaskType: in.TaskType, BriefMd: in.BriefMd, Languages: in.Languages,
			VolumeEstimate: in.VolumeEstimate, WantedBy: wanted, ContactName: in.ContactName, ContactEmail: in.ContactEmail,
			SampleFileKey: key, SampleFileName: name, SampleSize: size, CreatedBy: p.UserID})
		if err != nil {
			return err
		}
		if s.opts.NotifyTo != "" {
			sampleNote := "none"
			if name != nil {
				sampleNote = "attached"
			}
			if err := mailer.Enqueue(ctx, q, s.opts.NotifyTo, mailer.TemplateProjectRequest, map[string]string{
				"Organisation": org.Name, "Title": in.Title, "TaskType": in.TaskType, "Sample": sampleNote,
				"Link": s.opts.AppOrigin + "/w/requests/" + id.String(), "ID": id.String(),
			}); err != nil {
				return err
			}
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project_request.created", EntityType: "project_request",
			EntityID: &id, OrgID: &p.OrgID, After: map[string]any{"title": in.Title, "task_type": in.TaskType, "sample": name != nil}}); err != nil {
			return err
		}
		out = toRequest(r, org.Name)
		return nil
	})
	return out, err
}

func (s *Service) ListRequests(ctx context.Context, p *auth.Principal, status string) ([]Request, error) {
	var st *string
	if status != "" {
		st = &status
	}
	out := []Request{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListProjectRequests(ctx, st)
		for _, r := range rows {
			out = append(out, toRequest(store.ProjectRequest{ID: r.ID, OrgID: r.OrgID, Title: r.Title, TaskType: r.TaskType,
				BriefMd: r.BriefMd, Languages: r.Languages, VolumeEstimate: r.VolumeEstimate, WantedBy: r.WantedBy,
				ContactName: r.ContactName, ContactEmail: r.ContactEmail, SampleFileName: r.SampleFileName, SampleSize: r.SampleSize,
				Status: r.Status, DecisionNote: r.DecisionNote, ProjectID: r.ProjectID, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt}, r.OrgName))
		}
		return err
	})
	return out, err
}

func (s *Service) GetRequest(ctx context.Context, p *auth.Principal, id uuid.UUID) (Request, error) {
	var out Request
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		r, err := store.New(tx).GetProjectRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		out = toRequest(store.ProjectRequest{ID: r.ID, OrgID: r.OrgID, Title: r.Title, TaskType: r.TaskType,
			BriefMd: r.BriefMd, Languages: r.Languages, VolumeEstimate: r.VolumeEstimate, WantedBy: r.WantedBy,
			ContactName: r.ContactName, ContactEmail: r.ContactEmail, SampleFileName: r.SampleFileName, SampleSize: r.SampleSize,
			Status: r.Status, DecisionNote: r.DecisionNote, ProjectID: r.ProjectID, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt}, r.OrgName)
		return nil
	})
	return out, err
}

// RequestSample returns the attached file for download. RLS limits it to
// staff and the request's own organisation.
func (s *Service) RequestSample(ctx context.Context, p *auth.Principal, id uuid.UUID) (string, []byte, error) {
	var key, name string
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		r, err := store.New(tx).GetProjectRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.SampleFileKey == nil) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		key, name = *r.SampleFileKey, *r.SampleFileName
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	data, err := s.blobs.Get(ctx, key, MaxSampleBytes)
	if err != nil {
		return "", nil, errors.Join(ErrStorage, err)
	}
	return name, data, nil
}

// ConvertRequest creates a draft project in the request's organisation from
// Ops's completed project form and links the two.
func (s *Service) ConvertRequest(ctx context.Context, p *auth.Principal, id uuid.UUID, in ProjectInput, m auth.Meta) (Project, error) {
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.LockProjectRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status != "submitted" {
			return ErrAlreadyDecided
		}
		if in.OrgID != uuid.Nil && in.OrgID != r.OrgID {
			return fieldErr("org_id", "a request becomes a project in its own organisation")
		}
		in.OrgID = r.OrgID
		out, err = s.createProject(ctx, q, p, in, m)
		if err != nil {
			return err
		}
		if _, err := q.DecideProjectRequest(ctx, store.DecideProjectRequestParams{ID: id, Status: "converted",
			ProjectID: &out.ID, DecidedBy: &p.UserID}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project_request.converted", EntityType: "project_request",
			EntityID: &id, OrgID: &r.OrgID, After: map[string]any{"project_id": out.ID}})
	})
	return out, err
}

// DeclineRequest closes a request with a note the client sees.
func (s *Service) DeclineRequest(ctx context.Context, p *auth.Principal, id uuid.UUID, note string, m auth.Meta) (Request, error) {
	note = strings.TrimSpace(note)
	if note == "" || utf8.RuneCountInString(note) > 2000 {
		return Request{}, fieldErr("note", "tell the client why, in up to 2000 characters")
	}
	var out Request
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		r, err := q.LockProjectRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status != "submitted" {
			return ErrAlreadyDecided
		}
		d, err := q.DecideProjectRequest(ctx, store.DecideProjectRequestParams{ID: id, Status: "declined",
			DecisionNote: &note, DecidedBy: &p.UserID})
		if err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project_request.declined", EntityType: "project_request",
			EntityID: &id, OrgID: &r.OrgID, Reason: note}); err != nil {
			return err
		}
		org, err := q.GetOrganisation(ctx, r.OrgID)
		out = toRequest(d, org.Name)
		return err
	})
	return out, err
}

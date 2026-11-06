package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/imports"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ImportJob is one uploaded file and its validation report (a dataset version).
type ImportJob struct {
	ID              uuid.UUID          `json:"id"`
	ProjectID       uuid.UUID          `json:"project_id"`
	Kind            string             `json:"kind"`
	Format          string             `json:"format"`
	FileName        string             `json:"file_name"`
	SizeBytes       int64              `json:"size_bytes"`
	Status          string             `json:"status"` // validated, failed or committed
	TotalRows       int32              `json:"total_rows"`
	ValidRows       int32              `json:"valid_rows"`
	ErrorRows       int32              `json:"error_rows"`
	Errors          []imports.RowError `json:"errors,omitempty"`
	ErrorsTruncated bool               `json:"errors_truncated"`
	CreatedAt       time.Time          `json:"created_at"`
	CommittedAt     *time.Time         `json:"committed_at"`
}

func toImportJob(j store.ImportJob) ImportJob {
	return ImportJob{ID: j.ID, ProjectID: j.ProjectID, Kind: j.Kind, Format: j.Format, FileName: j.FileName,
		SizeBytes: j.SizeBytes, Status: j.Status, TotalRows: j.TotalRows, ValidRows: j.ValidRows, ErrorRows: j.ErrorRows,
		CreatedAt: j.CreatedAt, CommittedAt: j.CommittedAt}
}

// mayImport: item files need item.import, gold files gold.manage.
func mayImport(p *auth.Principal, kind string) bool {
	if kind == imports.KindGold {
		return p.Has("gold.manage")
	}
	return p.Has("item.import")
}

// cleanFileName keeps a display name only; storage keys never use it.
func cleanFileName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if utf8.RuneCountInString(name) > 200 {
		name = string([]rune(name)[:200])
	}
	if name == "" || name == "." || name == "/" {
		name = "upload"
	}
	return name
}

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// spec loads what a file is checked against: the project's task settings and
// the external IDs it already uses.
func spec(ctx context.Context, q *store.Queries, pr store.Project, kind, format string) (imports.Spec, error) {
	cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
	if err != nil {
		return imports.Spec{}, err
	}
	ids, err := q.ExternalIDsInProject(ctx, pr.ID)
	if err != nil {
		return imports.Spec{}, err
	}
	taken := make(map[string]bool, len(ids))
	for _, id := range ids {
		taken[id] = true
	}
	return imports.Spec{Kind: kind, Format: format, TaskType: pr.TaskType, Config: cfg, Taken: taken}, nil
}

// CheckImport is the dry run: it validates the whole file, stores it and
// records the report. Nothing is imported until CommitImport.
func (s *Service) CheckImport(ctx context.Context, p *auth.Principal, projectID uuid.UUID, kind, fileName string, data []byte, m auth.Meta) (ImportJob, error) {
	if kind != imports.KindItems && kind != imports.KindGold {
		return ImportJob{}, fieldErr("kind", "use items or gold")
	}
	if !mayImport(p, kind) {
		return ImportJob{}, ErrForbidden
	}
	fileName = cleanFileName(fileName)
	format, ok := imports.FormatOf(fileName)
	if !ok {
		return ImportJob{}, fieldErr("file", "upload a .csv or .jsonl file")
	}
	var pr store.Project
	var sp imports.Spec
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		pr, err = q.GetProject(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if pr.Status == StatusClosed {
			return ErrClosed
		}
		sp, err = spec(ctx, q, pr, kind, format)
		return err
	})
	if err != nil {
		return ImportJob{}, err
	}
	report := imports.Validate(sp, data)

	id, err := uuid.NewV7()
	if err != nil {
		return ImportJob{}, err
	}
	// Keys are built from IDs only. The file is kept as the dataset version
	// and deleted with the project's task content (brief 12, Stage 17).
	key := "imports/" + pr.OrgID.String() + "/" + pr.ID.String() + "/" + id.String() + "." + format
	if err := s.blobs.Put(ctx, key, "application/octet-stream", data); err != nil {
		return ImportJob{}, errors.Join(ErrStorage, err)
	}
	status := "failed"
	if report.Valid() {
		status = "validated"
	}
	var out ImportJob
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		j, err := q.CreateImportJob(ctx, store.CreateImportJobParams{ID: id, OrgID: pr.OrgID, ProjectID: pr.ID, Kind: kind,
			Format: format, FileKey: key, FileName: fileName, SizeBytes: int64(len(data)), Sha256: digest(data), Status: status,
			TotalRows: int32(report.TotalRows), ValidRows: int32(len(report.Rows)), ErrorRows: int32(report.ErrorRows), CreatedBy: &p.UserID})
		if err != nil {
			return err
		}
		for _, e := range report.Errors {
			var field *string
			if e.Field != "" {
				f := e.Field
				field = &f
			}
			if err := q.AddImportError(ctx, store.AddImportErrorParams{OrgID: pr.OrgID, ImportJobID: id,
				RowNumber: int32(e.Row), Field: field, Message: e.Message}); err != nil {
				return err
			}
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "import.checked", EntityType: "import_job",
			EntityID: &id, OrgID: &pr.OrgID, After: map[string]any{"project_id": pr.ID, "kind": kind, "format": format,
				"size_bytes": len(data), "total_rows": report.TotalRows, "error_rows": report.ErrorRows, "status": status}}); err != nil {
			return err
		}
		out = toImportJob(j)
		out.Errors = report.Errors
		out.ErrorsTruncated = report.ErrorRows > len(report.Errors)
		return nil
	})
	return out, err
}

// CommitImport imports every row of a checked file in one transaction. It
// re-reads the stored file, confirms its digest and checks it again under a
// project lock, so an invalid file (or one that became invalid) imports nothing.
func (s *Service) CommitImport(ctx context.Context, p *auth.Principal, jobID uuid.UUID, m auth.Meta) (ImportJob, error) {
	var out ImportJob
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		j0, err := q.GetImportJob(ctx, jobID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !mayImport(p, j0.Kind) {
			return ErrForbidden
		}
		// Project first, then job: commits to one project run one at a time.
		pr, err := q.LockProject(ctx, j0.ProjectID)
		if err != nil {
			return err
		}
		if pr.Status == StatusClosed {
			return ErrClosed
		}
		j, err := q.LockImportJob(ctx, jobID)
		if err != nil {
			return err
		}
		if j.Status != "validated" || j.ErrorRows > 0 {
			return ErrImportNotOpen
		}
		data, err := s.blobs.Get(ctx, j.FileKey, imports.MaxBytes)
		if err != nil {
			return errors.Join(ErrStorage, err)
		}
		if digest(data) != j.Sha256 {
			return ErrFileChanged
		}
		sp, err := spec(ctx, q, pr, j.Kind, j.Format)
		if err != nil {
			return err
		}
		report := imports.Validate(sp, data)
		if !report.Valid() || len(report.Rows) != int(j.ValidRows) {
			return ErrImportStale
		}
		for _, r := range report.Rows {
			itemID, err := q.CreateImportedItem(ctx, store.CreateImportedItemParams{OrgID: pr.OrgID, ProjectID: pr.ID,
				ExternalID: r.ExternalID, Content: r.Content, ImportJobID: &j.ID})
			if err != nil {
				return err
			}
			if j.Kind == imports.KindGold {
				if _, err := q.CreateGoldItem(ctx, store.CreateGoldItemParams{OrgID: pr.OrgID, ProjectID: pr.ID,
					ItemID: itemID, Expected: r.Expected, Tolerance: r.Tolerance, CreatedBy: &p.UserID}); err != nil {
					return err
				}
			}
		}
		if err := q.MarkImportCommitted(ctx, j.ID); err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "import.committed", EntityType: "import_job",
			EntityID: &j.ID, OrgID: &pr.OrgID, After: map[string]any{"project_id": pr.ID, "kind": j.Kind, "rows": len(report.Rows)}}); err != nil {
			return err
		}
		j, err = q.GetImportJob(ctx, j.ID)
		out = toImportJob(j)
		return err
	})
	return out, err
}

// visibleImport hides gold files from client users: the gold set is ELS's.
func visibleImport(p *auth.Principal, j store.ImportJob) bool {
	return p.Kind == auth.KindStaff || j.Kind != imports.KindGold
}

func (s *Service) ListImports(ctx context.Context, p *auth.Principal, projectID uuid.UUID) ([]ImportJob, error) {
	out := []ImportJob{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetProject(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		jobs, err := q.ListImportJobs(ctx, projectID)
		for _, j := range jobs {
			if visibleImport(p, j) {
				out = append(out, toImportJob(j))
			}
		}
		return err
	})
	return out, err
}

func (s *Service) GetImport(ctx context.Context, p *auth.Principal, id uuid.UUID) (ImportJob, error) {
	var out ImportJob
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		j, err := q.GetImportJob(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !visibleImport(p, j)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		errs, err := q.ListImportErrors(ctx, id)
		if err != nil {
			return err
		}
		out = toImportJob(j)
		out.Errors = make([]imports.RowError, 0, len(errs))
		for _, e := range errs {
			re := imports.RowError{Row: int(e.RowNumber), Message: e.Message}
			if e.Field != nil {
				re.Field = *e.Field
			}
			out.Errors = append(out.Errors, re)
		}
		out.ErrorsTruncated = int(j.ErrorRows) > len(errs)
		return nil
	})
	return out, err
}

// Gold is one gold item with its expected answer (staff only).
type Gold struct {
	ID         uuid.UUID       `json:"id"`
	ItemID     uuid.UUID       `json:"item_id"`
	ExternalID *string         `json:"external_id"`
	Content    json.RawMessage `json:"content"`
	Expected   json.RawMessage `json:"expected"`
	Tolerance  json.RawMessage `json:"tolerance"`
	Status     string          `json:"status"`
	UsageCount int32           `json:"usage_count"`
	CreatedAt  time.Time       `json:"created_at"`
	RetiredAt  *time.Time      `json:"retired_at"`
}

func (s *Service) ListGold(ctx context.Context, p *auth.Principal, projectID uuid.UUID) ([]Gold, error) {
	out := []Gold{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetProject(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		rows, err := q.ListGoldByProject(ctx, projectID)
		for _, g := range rows {
			out = append(out, Gold{ID: g.ID, ItemID: g.ItemID, ExternalID: g.ExternalID, Content: g.Content,
				Expected: g.Expected, Tolerance: g.Tolerance, Status: g.Status, UsageCount: g.UsageCount,
				CreatedAt: g.CreatedAt, RetiredAt: g.RetiredAt})
		}
		return err
	})
	return out, err
}

// GoldInput is one gold item entered by hand.
type GoldInput struct {
	ExternalID string            `json:"external_id"`
	Content    map[string]string `json:"content"`
	Expected   json.RawMessage   `json:"expected"`
	Tolerance  json.RawMessage   `json:"tolerance"`
}

func (s *Service) CreateGold(ctx context.Context, p *auth.Principal, projectID uuid.UUID, in GoldInput, m auth.Meta) (Gold, error) {
	var out Gold
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.LockProject(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if pr.Status == StatusClosed {
			return ErrClosed
		}
		sp, err := spec(ctx, q, pr, imports.KindGold, imports.JSONL)
		if err != nil {
			return err
		}
		// One row through the same checks as a file.
		row := map[string]any{}
		for k, v := range in.Content {
			row[k] = v
		}
		row["expected"] = in.Expected // after content, so content cannot replace it
		delete(row, "tolerance")
		delete(row, "external_id")
		if len(in.Tolerance) > 0 {
			row["tolerance"] = in.Tolerance
		}
		if strings.TrimSpace(in.ExternalID) != "" {
			row["external_id"] = in.ExternalID
		}
		rep := imports.Validate(sp, mustJSON(row))
		if !rep.Valid() {
			e := rep.Errors[0]
			return &FieldError{Field: e.Field, Message: e.Message}
		}
		r := rep.Rows[0]
		itemID, err := q.CreateImportedItem(ctx, store.CreateImportedItemParams{OrgID: pr.OrgID, ProjectID: pr.ID,
			ExternalID: r.ExternalID, Content: r.Content})
		if err != nil {
			return err
		}
		g, err := q.CreateGoldItem(ctx, store.CreateGoldItemParams{OrgID: pr.OrgID, ProjectID: pr.ID, ItemID: itemID,
			Expected: r.Expected, Tolerance: r.Tolerance, CreatedBy: &p.UserID})
		if err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "gold.created", EntityType: "gold_item",
			EntityID: &g.ID, OrgID: &pr.OrgID, After: map[string]any{"project_id": pr.ID}}); err != nil {
			return err
		}
		out = Gold{ID: g.ID, ItemID: itemID, ExternalID: r.ExternalID, Content: r.Content, Expected: g.Expected,
			Tolerance: g.Tolerance, Status: g.Status, UsageCount: g.UsageCount, CreatedAt: g.CreatedAt}
		return nil
	})
	return out, err
}

// RetireGold stops a gold item being served. Its past results stay.
func (s *Service) RetireGold(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		g, err := q.GetGoldItem(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := q.RetireGold(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return ErrIllegalTransition // already retired
		} else if err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "gold.retired", EntityType: "gold_item",
			EntityID: &id, OrgID: &g.OrgID, Before: map[string]any{"status": g.Status}, After: map[string]any{"status": "retired"}})
	})
}

func (s *Service) TaskTemplates(ctx context.Context, p *auth.Principal) ([]TaskTemplate, error) {
	out := []TaskTemplate{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListTaskTemplates(ctx)
		for _, t := range rows {
			out = append(out, TaskTemplate{TaskType: t.TaskType, Name: t.Name, Version: t.Version,
				ContentFields: quality.ContentFields(t.TaskType),
				ItemColumns:   imports.Columns(imports.KindItems, t.TaskType),
				GoldColumns:   imports.Columns(imports.KindGold, t.TaskType)})
		}
		return err
	})
	return out, err
}

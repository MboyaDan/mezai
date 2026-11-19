// Package delivery exports a project's final answers as deliverables (brief
// 6.3, 8.5, Stage 11b): JSONL and CSV files with a manifest, kept in object
// storage, published by Ops to the client, and downloaded through short-lived
// signed links that leave an audit row.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

var (
	ErrNotFound         = errors.New("delivery: not found")
	ErrNothingToExport  = errors.New("delivery: no final items to export")
	ErrTooLarge         = errors.New("delivery: too many items for one export")
	ErrAlreadyPublished = errors.New("delivery: version already published")
	ErrLinkInvalid      = errors.New("delivery: download link invalid or expired")
	ErrFileChanged      = errors.New("delivery: stored file does not match its checksum")
	ErrStorage          = errors.New("delivery: file storage unavailable")
	ErrInvalidInput     = errors.New("delivery: invalid input")
	// ErrPurged: the project's content was deleted under its retention rule
	// (Stage 17b); its files are gone and nothing new can be exported.
	ErrPurged = errors.New("delivery: content deleted under the retention rule")
)

// MaxItems caps one export, which is built in memory (A-069).
const MaxItems = 50_000

// maxFileBytes bounds what a download reads back from storage.
const maxFileBytes = 256 << 20

// Service exports, publishes and serves deliverables.
type Service struct {
	pool    *db.Pool
	blobs   storage.Blobs
	linkKey []byte
	now     func() time.Time
	log     *slog.Logger
}

// NewService derives the link-signing key from secret (config.IdentityKey),
// so no new secret is needed; the derived key cannot reveal the secret.
func NewService(pool *db.Pool, blobs storage.Blobs, secret []byte, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{pool: pool, blobs: blobs, linkKey: deriveLinkKey(secret), now: time.Now, log: log}
}

// File is one downloadable file of a version. The manifest has no size or
// checksum of its own here: it is the document that holds the checksums.
type File struct {
	ID        uuid.UUID `json:"id"` // the deliverable row (data files); the JSONL row for the manifest
	File      string    `json:"file"`
	Format    string    `json:"format"` // jsonl, csv or json
	Name      string    `json:"name"`
	Bytes     int64     `json:"bytes"`
	SHA256    string    `json:"sha256"`
	Downloads int32     `json:"downloads"`
}

// Version is one export: its data files and manifest.
type Version struct {
	Version     int32      `json:"version"`
	Items       int32      `json:"items"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
	PurgedAt    *time.Time `json:"purged_at"`
	Manifest    Manifest   `json:"manifest"`
	Files       []File     `json:"files"`
}

// visibleProject checks the caller can see the project (RLS) and returns its name.
func (s *Service) visibleProject(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (string, error) {
	pr, err := s.project(ctx, p, projectID)
	return pr.Name, err
}

func (s *Service) project(ctx context.Context, p *auth.Principal, projectID uuid.UUID) (store.GetProjectForWorkRow, error) {
	var pr store.GetProjectForWorkRow
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		pr, err = store.New(tx).GetProjectForWork(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return pr, err
}

// Export builds a new version of the project's deliverable from its final
// items and stores it unpublished. Exports of one project run one at a time.
func (s *Service) Export(ctx context.Context, p *auth.Principal, projectID uuid.UUID, m auth.Meta) (Version, error) {
	if _, err := s.visibleProject(ctx, p, projectID); err != nil {
		return Version{}, err
	}
	var out Version
	var keys []string
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.LockProjectForExport(ctx, projectID)
		if err != nil {
			return err
		}
		if pr.ContentPurgedAt != nil {
			return ErrPurged
		}
		cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
		if err != nil {
			return err
		}
		counts, err := q.ExportItemCounts(ctx, projectID)
		if err != nil {
			return err
		}
		if counts.Final == 0 {
			return ErrNothingToExport
		}
		if counts.Final > MaxItems {
			return ErrTooLarge
		}
		rows, err := q.ExportItems(ctx, store.ExportItemsParams{ProjectID: projectID, MaxItems: MaxItems})
		if err != nil {
			return err
		}
		answers, err := q.ExportAnswers(ctx, projectID)
		if err != nil {
			return err
		}
		gold, err := q.ExportGoldByContributor(ctx, projectID)
		if err != nil {
			return err
		}
		reasons, err := q.ExportRouteReasons(ctx, projectID)
		if err != nil {
			return err
		}
		version, err := q.NextDeliverableVersion(ctx, projectID)
		if err != nil {
			return err
		}
		pk, err := assemble(exportInput{Project: pr, Config: cfg, Counts: counts, Rows: rows, Answers: answers,
			Gold: gold, Reasons: reasons, Version: version, Now: s.now()})
		if err != nil {
			return err
		}
		man, manJSON := pk.Manifest, pk.ManifestJSON
		data := map[string][]byte{"jsonl": pk.JSONL, "csv": pk.CSV}
		prefix := fmt.Sprintf("deliverables/%s/%s/v%d/", pr.OrgID, pr.ID, version)
		manKey := prefix + FileName(pr.Name, version, "manifest")
		put := func(key, contentType string, b []byte) error {
			if err := s.blobs.Put(ctx, key, contentType, b); err != nil {
				s.log.ErrorContext(ctx, "deliverable upload failed", "err", err)
				return ErrStorage
			}
			keys = append(keys, key)
			return nil
		}
		if err := put(manKey, "application/json", manJSON); err != nil {
			return err
		}
		out = Version{Version: version, Items: counts.Final, Manifest: man}
		types := map[string]string{"jsonl": "application/x-ndjson", "csv": "text/csv; charset=utf-8"}
		for i, f := range []string{"jsonl", "csv"} {
			key := prefix + man.Files[i].Name
			if err := put(key, types[f], data[f]); err != nil {
				return err
			}
			d, err := q.CreateDeliverable(ctx, store.CreateDeliverableParams{OrgID: pr.OrgID, ProjectID: pr.ID, Format: f,
				FileKey: key, ManifestKey: manKey, Manifest: manJSON, ChecksumSha256: man.Files[i].SHA256,
				SchemaVersion: quality.ExportSchemaVersion, ItemCount: counts.Final, Version: version,
				ByteSize: int64(len(data[f])), CreatedBy: &p.UserID})
			if err != nil {
				return err
			}
			out.CreatedAt = d.CreatedAt
			out.Files = append(out.Files, File{ID: d.ID, File: "data", Format: f, Name: man.Files[i].Name,
				Bytes: d.ByteSize, SHA256: d.ChecksumSha256})
			if f == "jsonl" {
				out.Files = append(out.Files, File{ID: d.ID, File: "manifest", Format: "json",
					Name: FileName(pr.Name, version, "manifest")})
			}
		}
		id := out.Files[0].ID
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "deliverable.exported", EntityType: "deliverable",
			EntityID: &id, OrgID: &pr.OrgID, After: map[string]any{"project_id": pr.ID, "version": version,
				"items": counts.Final, "sha256": map[string]string{"jsonl": man.Files[0].SHA256, "csv": man.Files[1].SHA256}}})
	})
	if err != nil {
		// Nothing points at files from a failed export; remove them.
		for _, k := range keys {
			if derr := s.blobs.Delete(context.WithoutCancel(ctx), k); derr != nil {
				s.log.WarnContext(ctx, "orphan deliverable file not removed", "err", derr)
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Version{}, ErrNotFound
		}
		return Version{}, err
	}
	return out, nil
}

// List returns the project's versions, newest first. A client sees only
// published versions (row-level security).
func (s *Service) List(ctx context.Context, p *auth.Principal, projectID uuid.UUID) ([]Version, error) {
	name, err := s.visibleProject(ctx, p, projectID)
	if err != nil {
		return nil, err
	}
	out := []Version{}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListProjectDeliverables(ctx, projectID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if len(out) == 0 || out[len(out)-1].Version != r.Version {
				v := Version{Version: r.Version, Items: r.ItemCount, CreatedAt: r.CreatedAt, PublishedAt: r.PublishedAt, PurgedAt: r.PurgedAt, Files: []File{}}
				if err := json.Unmarshal(r.Manifest, &v.Manifest); err != nil {
					return err
				}
				out = append(out, v)
			}
			v := &out[len(out)-1]
			v.Files = append(v.Files, File{ID: r.ID, File: "data", Format: r.Format, Name: FileName(name, r.Version, r.Format),
				Bytes: r.ByteSize, SHA256: r.ChecksumSha256, Downloads: r.Downloads})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The manifest is offered once per version, through the JSONL row.
	for i := range out {
		v := &out[i]
		for _, f := range v.Files {
			if f.Format == "jsonl" {
				v.Files = append(v.Files, File{ID: f.ID, File: "manifest", Format: "json", Name: FileName(name, v.Version, "manifest")})
				break
			}
		}
	}
	return out, nil
}

// Publish makes a version visible to the project's client.
func (s *Service) Publish(ctx context.Context, p *auth.Principal, projectID uuid.UUID, version int32, m auth.Meta) error {
	pr, err := s.project(ctx, p, projectID)
	if err != nil {
		return err
	}
	if pr.ContentPurgedAt != nil {
		return ErrPurged
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := q.PublishDeliverableVersion(ctx, store.PublishDeliverableVersionParams{PublishedBy: &p.UserID, ProjectID: projectID, Version: version})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			n, err := q.CountDeliverableVersion(ctx, store.CountDeliverableVersionParams{ProjectID: projectID, Version: version})
			if err != nil {
				return err
			}
			if n == 0 {
				return ErrNotFound
			}
			return ErrAlreadyPublished
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "deliverable.published", EntityType: "deliverable",
			EntityID: &rows[0].ID, OrgID: &rows[0].OrgID, After: map[string]any{"project_id": projectID, "version": version}})
	})
}

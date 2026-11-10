-- Deliverable exports (Stage 11b, brief 6.3, 8.5, AC-09).
--
-- One export is a numbered version per project with two data files (JSONL
-- and CSV, one deliverables row each) sharing one manifest. Ops publishes a
-- version; only then can the client see it (RLS policy from 00007).
-- Every download is a deliverable_downloads row naming the file (D-115).

-- +goose Up
ALTER TABLE deliverables
  ADD COLUMN version integer,
  ADD COLUMN byte_size bigint NOT NULL DEFAULT 0 CHECK (byte_size >= 0),
  ADD COLUMN created_by uuid REFERENCES users (id);

-- Rows created before this migration (tests and local data only) get
-- versions in creation order.
UPDATE deliverables d SET version = v.n
FROM (SELECT id, row_number() OVER (PARTITION BY project_id, format ORDER BY created_at, id) AS n FROM deliverables) v
WHERE v.id = d.id;

ALTER TABLE deliverables
  ALTER COLUMN version SET NOT NULL,
  ADD CONSTRAINT deliverables_version_positive CHECK (version > 0),
  ADD CONSTRAINT deliverables_version_format UNIQUE (project_id, version, format);

ALTER TABLE deliverable_downloads
  ADD COLUMN file text NOT NULL DEFAULT 'data' CHECK (file IN ('data', 'manifest'));

-- +goose Down
ALTER TABLE deliverable_downloads DROP COLUMN file;
ALTER TABLE deliverables
  DROP CONSTRAINT deliverables_version_format,
  DROP CONSTRAINT deliverables_version_positive,
  DROP COLUMN created_by,
  DROP COLUMN byte_size,
  DROP COLUMN version;

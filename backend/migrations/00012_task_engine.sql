-- Task engine (Stage 9a): assignment, drafts, submit, skip, report.

-- +goose Up

-- A reported item leaves the contributor's queue at once; "reported" keeps
-- that apart from a skip, which has its own reason list.
ALTER TABLE assignments DROP CONSTRAINT assignments_status_check;
ALTER TABLE assignments ADD CONSTRAINT assignments_status_check
  CHECK (status IN ('reserved', 'submitted', 'skipped', 'expired', 'reported'));

-- One task at a time per contributor and project (D-094).
CREATE UNIQUE INDEX assignments_one_reserved_idx ON assignments (contributor_id, project_id) WHERE status = 'reserved';
CREATE INDEX assignments_item_status_idx ON assignments (item_id, status);

-- Warnings from the automatic QA checks (D-077), stored for reviewers.
ALTER TABLE submissions ADD COLUMN qa_warnings jsonb NOT NULL DEFAULT '[]';

CREATE INDEX content_reports_item_open_idx ON content_reports (item_id) WHERE status = 'open';

-- +goose Down
DROP INDEX content_reports_item_open_idx;
ALTER TABLE submissions DROP COLUMN qa_warnings;
DROP INDEX assignments_item_status_idx;
DROP INDEX assignments_one_reserved_idx;
UPDATE assignments SET status = 'skipped' WHERE status = 'reported';
ALTER TABLE assignments DROP CONSTRAINT assignments_status_check;
ALTER TABLE assignments ADD CONSTRAINT assignments_status_check
  CHECK (status IN ('reserved', 'submitted', 'skipped', 'expired'));

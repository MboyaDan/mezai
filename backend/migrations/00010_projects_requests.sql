-- Projects, imports and client project requests (Stage 8, brief 6.3).

-- +goose Up

-- Checks the task screen runs before a contributor submits (D-077). Ops sets
-- them per project; Stage 9 enforces them. Shape validated in the API.
ALTER TABLE projects ADD COLUMN qa_checks jsonb NOT NULL DEFAULT '{}';

-- Each import is one version of a dataset: keep the file's size and digest so
-- the commit can prove it reads the same bytes the dry run validated.
ALTER TABLE import_jobs ADD COLUMN size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0);
ALTER TABLE import_jobs ADD COLUMN sha256 text NOT NULL DEFAULT '';
ALTER TABLE import_jobs ADD COLUMN committed_at timestamptz;
ALTER TABLE import_jobs ADD CONSTRAINT import_jobs_committed_chk CHECK (status <> 'committed' OR committed_at IS NOT NULL);

-- Client Admins describe the work they need; Ops converts a request into a
-- draft project or declines it with a reason the client sees.
CREATE TABLE project_requests (
  id               uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id           uuid NOT NULL REFERENCES organisations (id),
  title            text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  task_type        text NOT NULL CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'other')),
  brief_md         text NOT NULL CHECK (length(brief_md) BETWEEN 1 AND 20000),
  languages        text[] NOT NULL DEFAULT '{}',
  volume_estimate  integer CHECK (volume_estimate > 0),
  wanted_by        date,
  contact_name     text NOT NULL CHECK (length(contact_name) BETWEEN 1 AND 200),
  contact_email    text NOT NULL CHECK (length(contact_email) BETWEEN 3 AND 254),
  sample_file_key  text,
  sample_file_name text,
  sample_size      bigint CHECK (sample_size >= 0),
  status           text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'converted', 'declined')),
  decision_note    text,
  project_id       uuid,
  decided_by       uuid REFERENCES users (id),
  decided_at       timestamptz,
  created_by       uuid NOT NULL REFERENCES users (id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  CHECK ((sample_file_key IS NULL) = (sample_file_name IS NULL)),
  CHECK (status <> 'converted' OR project_id IS NOT NULL),
  CHECK (status <> 'declined' OR decision_note IS NOT NULL),
  CHECK ((status = 'submitted') = (decided_at IS NULL))
);
CREATE INDEX project_requests_org_idx ON project_requests (org_id, created_at DESC);
CREATE INDEX project_requests_open_idx ON project_requests (created_at) WHERE status = 'submitted';
CREATE TRIGGER project_requests_updated_at BEFORE UPDATE ON project_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Clients read and create their own organisation's requests; only staff decide.
ALTER TABLE project_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_requests_read ON project_requests FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY project_requests_insert ON project_requests FOR INSERT WITH CHECK (app_sees_org(org_id));
CREATE POLICY project_requests_staff ON project_requests FOR UPDATE USING (app_is_staff()) WITH CHECK (app_is_staff());
GRANT SELECT, INSERT, UPDATE ON project_requests TO mezai_app;

-- +goose Down
DROP TABLE project_requests;
ALTER TABLE import_jobs DROP CONSTRAINT import_jobs_committed_chk;
ALTER TABLE import_jobs DROP COLUMN committed_at, DROP COLUMN sha256, DROP COLUMN size_bytes;
ALTER TABLE projects DROP COLUMN qa_checks;

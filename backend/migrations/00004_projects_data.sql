-- Projects, task templates, items, imports and gold (brief 6.3, 7, 8.1, 10).
-- Client-owned tables carry org_id. Composite foreign keys (id, org_id) make
-- a child row's organisation always match its parent's.

-- +goose Up

CREATE TABLE task_templates (
  id                uuid PRIMARY KEY DEFAULT uuid_v7(),
  task_type         text NOT NULL CHECK (task_type IN ('classification', 'pairwise', 'translation_review')),
  version           integer NOT NULL DEFAULT 1 CHECK (version > 0),
  name              text NOT NULL,
  import_schema     jsonb NOT NULL,
  submission_schema jsonb NOT NULL,
  export_schema     jsonb NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (task_type, version),
  UNIQUE (id, task_type)
);

CREATE TABLE projects (
  id                       uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id                   uuid NOT NULL REFERENCES organisations (id),
  template_id              uuid NOT NULL,
  task_type                text NOT NULL,
  name                     text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  status                   text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'live', 'paused', 'closed')),
  instructions_md          text NOT NULL DEFAULT '',
  config                   jsonb NOT NULL DEFAULT '{}',   -- label set (classification) or rubric (pairwise, translation)
  overlap_k                smallint NOT NULL DEFAULT 1 CHECK (overlap_k BETWEEN 1 AND 5),
  gold_rate                numeric(4, 3) NOT NULL DEFAULT 0.10 CHECK (gold_rate BETWEEN 0 AND 1),
  pay_per_task_cents       bigint NOT NULL DEFAULT 0 CHECK (pay_per_task_cents >= 0),
  currency                 char(3) NOT NULL DEFAULT 'KES' CHECK (currency ~ '^[A-Z]{3}$'),
  target_count             integer CHECK (target_count > 0),
  deadline                 date,
  reservation_minutes      integer NOT NULL DEFAULT 30 CHECK (reservation_minutes BETWEEN 5 AND 480),
  review_spread_threshold  numeric(4, 2) NOT NULL DEFAULT 2 CHECK (review_spread_threshold >= 0),
  audit_sample_rate        numeric(4, 3) NOT NULL DEFAULT 0.05 CHECK (audit_sample_rate BETWEEN 0 AND 1),
  agreement_threshold      numeric(4, 3) NOT NULL DEFAULT 0.60 CHECK (agreement_threshold BETWEEN -1 AND 1),
  min_rationale_chars      integer NOT NULL DEFAULT 0 CHECK (min_rationale_chars >= 0),
  retention_days           integer NOT NULL DEFAULT 90 CHECK (retention_days >= 0),
  client_visibility        jsonb NOT NULL DEFAULT '{}',
  published_at             timestamptz,
  closed_at                timestamptz,
  created_by               uuid REFERENCES users (id),
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (template_id, task_type) REFERENCES task_templates (id, task_type),
  UNIQUE (id, org_id),
  CHECK (status <> 'closed' OR closed_at IS NOT NULL)
);
CREATE INDEX projects_org_idx ON projects (org_id, status);
CREATE INDEX projects_live_idx ON projects (status) WHERE status = 'live';

CREATE TABLE project_skills (
  id              uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id          uuid NOT NULL,
  project_id      uuid NOT NULL,
  skill_kind      text NOT NULL CHECK (skill_kind IN ('language', 'domain', 'task_type')),
  skill_value     text NOT NULL,
  min_proficiency text CHECK (min_proficiency IN ('native', 'fluent', 'working')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id) ON DELETE CASCADE,
  UNIQUE (project_id, skill_kind, skill_value),
  CHECK (skill_kind = 'language' OR min_proficiency IS NULL)
);

CREATE TABLE project_guidelines (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id     uuid NOT NULL,
  project_id uuid NOT NULL,
  title      text NOT NULL,
  body_md    text NOT NULL,
  position   integer NOT NULL DEFAULT 1 CHECK (position > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id) ON DELETE CASCADE
);
CREATE INDEX project_guidelines_project_idx ON project_guidelines (project_id, position);

CREATE TABLE import_jobs (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id     uuid NOT NULL,
  project_id uuid NOT NULL,
  kind       text NOT NULL CHECK (kind IN ('items', 'gold')),
  format     text NOT NULL CHECK (format IN ('csv', 'jsonl')),
  file_key   text NOT NULL,       -- object storage key
  file_name  text NOT NULL,
  dry_run    boolean NOT NULL DEFAULT true,
  status     text NOT NULL DEFAULT 'uploaded'
               CHECK (status IN ('uploaded', 'validating', 'validated', 'committing', 'committed', 'failed')),
  total_rows integer NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
  valid_rows integer NOT NULL DEFAULT 0 CHECK (valid_rows >= 0),
  error_rows integer NOT NULL DEFAULT 0 CHECK (error_rows >= 0),
  created_by uuid REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  UNIQUE (id, org_id)
);
CREATE INDEX import_jobs_project_idx ON import_jobs (project_id, created_at DESC);

CREATE TABLE import_errors (
  id            uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id        uuid NOT NULL,
  import_job_id uuid NOT NULL,
  row_number    integer NOT NULL CHECK (row_number > 0),
  field         text,
  message       text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (import_job_id, org_id) REFERENCES import_jobs (id, org_id) ON DELETE CASCADE
);
CREATE INDEX import_errors_job_idx ON import_errors (import_job_id, row_number);

-- Gold items are ordinary items plus a gold_items row. Nothing on items marks
-- them as gold, so contributor queries cannot tell them apart (brief 8.1).
CREATE TABLE items (
  id               uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id           uuid NOT NULL,
  project_id       uuid NOT NULL,
  external_id      text,
  content          jsonb NOT NULL,
  status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'in_progress', 'awaiting_consensus', 'consensus', 'in_review', 'final', 'rejected')),
  submission_count integer NOT NULL DEFAULT 0 CHECK (submission_count >= 0),
  current_revision integer NOT NULL DEFAULT 0 CHECK (current_revision >= 0),
  import_job_id    uuid REFERENCES import_jobs (id),
  finalised_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  UNIQUE (id, org_id),
  UNIQUE (project_id, external_id),
  CHECK (status <> 'final' OR finalised_at IS NOT NULL)
);
CREATE INDEX items_queue_idx ON items (project_id, status, submission_count);

-- Final answers. Final is immutable: a correction is a new revision (brief 9).
CREATE TABLE item_revisions (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id     uuid NOT NULL,
  item_id    uuid NOT NULL,
  revision   integer NOT NULL CHECK (revision > 0),
  answer     jsonb NOT NULL,
  source     text NOT NULL CHECK (source IN ('consensus', 'review', 'correction')),
  reason     text,
  created_by uuid REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  UNIQUE (item_id, revision),
  CHECK (source <> 'correction' OR reason IS NOT NULL)
);

CREATE TABLE gold_items (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id      uuid NOT NULL,
  project_id  uuid NOT NULL,
  item_id     uuid NOT NULL UNIQUE,
  expected    jsonb NOT NULL,
  tolerance   jsonb NOT NULL DEFAULT '{}',
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired')),
  usage_count integer NOT NULL DEFAULT 0 CHECK (usage_count >= 0),
  created_by  uuid REFERENCES users (id),
  retired_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  UNIQUE (id, org_id)
);
CREATE INDEX gold_items_project_idx ON gold_items (project_id) WHERE status = 'active';

CREATE TRIGGER task_templates_updated_at BEFORE UPDATE ON task_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER projects_updated_at BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER project_skills_updated_at BEFORE UPDATE ON project_skills FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER project_guidelines_updated_at BEFORE UPDATE ON project_guidelines FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER import_jobs_updated_at BEFORE UPDATE ON import_jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER items_updated_at BEFORE UPDATE ON items FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER gold_items_updated_at BEFORE UPDATE ON gold_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Version 1 templates for the three pilot task types (brief 7). The schemas
-- are JSON Schema; the backend validates against them in Stages 8 and 9.
INSERT INTO task_templates (task_type, name, import_schema, submission_schema, export_schema) VALUES
  ('classification', 'Text classification',
   '{"type": "object", "required": ["text"], "properties": {"text": {"type": "string", "minLength": 1}}}',
   '{"type": "object", "required": ["labels"], "properties": {"labels": {"type": "array", "items": {"type": "string"}, "minItems": 1}, "confidence": {"type": "integer", "minimum": 1, "maximum": 5}, "note": {"type": "string"}}}',
   '{"type": "object", "required": ["item_id", "labels"], "properties": {"item_id": {"type": "string"}, "labels": {"type": "array", "items": {"type": "string"}}}}'),
  ('pairwise', 'Pairwise response comparison',
   '{"type": "object", "required": ["prompt", "response_a", "response_b"], "properties": {"prompt": {"type": "string"}, "response_a": {"type": "string"}, "response_b": {"type": "string"}}}',
   '{"type": "object", "required": ["scores_a", "scores_b", "preference", "rationale"], "properties": {"scores_a": {"type": "object"}, "scores_b": {"type": "object"}, "preference": {"enum": ["a", "b", "tie"]}, "rationale": {"type": "string"}}}',
   '{"type": "object", "required": ["item_id", "preference"], "properties": {"item_id": {"type": "string"}, "preference": {"enum": ["a", "b", "tie"]}, "mean_scores_a": {"type": "object"}, "mean_scores_b": {"type": "object"}}}'),
  ('translation_review', 'Translation review',
   '{"type": "object", "required": ["source", "translation", "source_lang", "target_lang"], "properties": {"source": {"type": "string"}, "translation": {"type": "string"}, "source_lang": {"type": "string"}, "target_lang": {"type": "string"}}}',
   '{"type": "object", "required": ["adequacy", "fluency", "corrected"], "properties": {"adequacy": {"type": "integer", "minimum": 1, "maximum": 5}, "fluency": {"type": "integer", "minimum": 1, "maximum": 5}, "corrected": {"type": "string"}, "error_tags": {"type": "array", "items": {"type": "string"}}}}',
   '{"type": "object", "required": ["item_id", "corrected"], "properties": {"item_id": {"type": "string"}, "adequacy": {"type": "number"}, "fluency": {"type": "number"}, "corrected": {"type": "string"}}}');

-- +goose Down
DROP TABLE gold_items, item_revisions, items, import_errors, import_jobs, project_guidelines, project_skills, projects, task_templates;

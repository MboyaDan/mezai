-- Work (assignments, submissions, skips, reports) and quality (gold results,
-- consensus, reviews, feedback, agreement, trust). Brief 7 to 9.

-- +goose Up

-- One assignment per contributor per item, ever: "never the same contributor
-- twice on one item" (brief 9).
CREATE TABLE assignments (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  project_id     uuid NOT NULL,
  item_id        uuid NOT NULL,
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  status         text NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'submitted', 'skipped', 'expired')),
  reserved_at    timestamptz NOT NULL DEFAULT now(),
  expires_at     timestamptz NOT NULL,
  completed_at   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  UNIQUE (item_id, contributor_id),
  UNIQUE (id, org_id),
  CHECK (expires_at > reserved_at),
  CHECK (status = 'reserved' OR completed_at IS NOT NULL)
);
CREATE INDEX assignments_contributor_idx ON assignments (contributor_id, status);
CREATE INDEX assignments_expiry_idx ON assignments (expires_at) WHERE status = 'reserved';

CREATE TABLE submissions (
  id               uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id           uuid NOT NULL,
  project_id       uuid NOT NULL,
  item_id          uuid NOT NULL,
  assignment_id    uuid NOT NULL UNIQUE,
  contributor_id   uuid NOT NULL REFERENCES contributor_profiles (id),
  status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'accepted', 'rejected')),
  current_revision integer NOT NULL DEFAULT 0 CHECK (current_revision >= 0),
  time_on_task_ms  bigint NOT NULL DEFAULT 0 CHECK (time_on_task_ms >= 0),
  idempotency_key  text,
  submitted_at     timestamptz,
  decided_at       timestamptz,
  rejection_reason text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (assignment_id, org_id) REFERENCES assignments (id, org_id),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  UNIQUE (id, org_id),
  UNIQUE (contributor_id, idempotency_key),
  CHECK (status = 'draft' OR submitted_at IS NOT NULL),
  CHECK (status <> 'rejected' OR rejection_reason IS NOT NULL)
);
CREATE INDEX submissions_item_idx ON submissions (item_id, status);
CREATE INDEX submissions_contributor_idx ON submissions (contributor_id, created_at DESC);

-- Every saved answer, drafts included. Append-only: work is never overwritten.
CREATE TABLE submission_revisions (
  id            uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id        uuid NOT NULL,
  submission_id uuid NOT NULL,
  revision      integer NOT NULL CHECK (revision > 0),
  answer        jsonb NOT NULL,
  is_draft      boolean NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (submission_id, org_id) REFERENCES submissions (id, org_id),
  UNIQUE (submission_id, revision)
);

CREATE TABLE skips (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  assignment_id  uuid NOT NULL UNIQUE,
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  reason         text NOT NULL CHECK (reason IN ('unclear', 'language', 'expertise', 'technical', 'other')),
  note           text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (assignment_id, org_id) REFERENCES assignments (id, org_id)
);

CREATE TABLE content_reports (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  item_id        uuid NOT NULL,
  assignment_id  uuid,
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  category       text NOT NULL CHECK (category IN ('harmful', 'personal_data', 'broken', 'other')),
  note           text,
  status         text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'actioned', 'dismissed')),
  resolved_by    uuid REFERENCES users (id),
  resolved_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  FOREIGN KEY (assignment_id, org_id) REFERENCES assignments (id, org_id)
);
CREATE INDEX content_reports_open_idx ON content_reports (created_at) WHERE status = 'open';

-- Append-only.
CREATE TABLE gold_results (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  gold_item_id   uuid NOT NULL,
  submission_id  uuid NOT NULL UNIQUE,
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  correct        boolean NOT NULL,
  score          numeric(5, 4) NOT NULL CHECK (score BETWEEN 0 AND 1),
  detail         jsonb NOT NULL DEFAULT '{}',
  created_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (gold_item_id, org_id) REFERENCES gold_items (id, org_id),
  FOREIGN KEY (submission_id, org_id) REFERENCES submissions (id, org_id)
);
CREATE INDEX gold_results_contributor_idx ON gold_results (contributor_id, created_at DESC);

CREATE TABLE consensus_results (
  id               uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id           uuid NOT NULL,
  project_id       uuid NOT NULL,
  item_id          uuid NOT NULL,
  method           text NOT NULL CHECK (method IN ('majority', 'mean')),
  result           jsonb NOT NULL,
  agreement        numeric(5, 4),
  spread           numeric(6, 3),
  routed_to_review boolean NOT NULL,
  route_reason     text CHECK (route_reason IN ('no_majority', 'spread', 'flagged', 'audit_sample')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  CHECK (routed_to_review = (route_reason IS NOT NULL))
);
CREATE INDEX consensus_results_item_idx ON consensus_results (item_id, created_at DESC);

-- Append-only: a changed decision is a new review row.
CREATE TABLE reviews (
  id                   uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id               uuid NOT NULL,
  project_id           uuid NOT NULL,
  item_id              uuid NOT NULL,
  reviewer_user_id     uuid NOT NULL REFERENCES users (id),
  decision             text NOT NULL CHECK (decision IN ('accept_consensus', 'choose_submission', 'write_final', 'reject_item')),
  chosen_submission_id uuid,
  final_answer         jsonb,
  reason               text,
  idempotency_key      text NOT NULL,
  created_at           timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (item_id, org_id) REFERENCES items (id, org_id),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  FOREIGN KEY (chosen_submission_id, org_id) REFERENCES submissions (id, org_id),
  UNIQUE (reviewer_user_id, idempotency_key),
  CHECK (decision <> 'choose_submission' OR chosen_submission_id IS NOT NULL),
  CHECK (decision <> 'write_final' OR final_answer IS NOT NULL),
  CHECK (decision <> 'reject_item' OR reason IS NOT NULL)
);
CREATE INDEX reviews_item_idx ON reviews (item_id, created_at DESC);

CREATE TABLE feedback (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  project_id     uuid NOT NULL,
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  submission_id  uuid,
  author_user_id uuid REFERENCES users (id),   -- null for batched gold feedback (brief 8.1)
  body           text NOT NULL CHECK (length(body) > 0),
  read_at        timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  FOREIGN KEY (submission_id, org_id) REFERENCES submissions (id, org_id)
);
CREATE INDEX feedback_contributor_idx ON feedback (contributor_id, created_at DESC);

CREATE TABLE agreement_snapshots (
  id           uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id       uuid NOT NULL,
  project_id   uuid NOT NULL,
  label        text,          -- null: whole project
  statistic    text NOT NULL CHECK (statistic IN ('percent', 'cohen_kappa', 'fleiss_kappa', 'mean_abs_diff')),
  value        numeric(6, 4) NOT NULL,
  n_items      integer NOT NULL CHECK (n_items >= 0),
  n_raters     integer NOT NULL CHECK (n_raters >= 0),
  window_start timestamptz NOT NULL,
  window_end   timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  CHECK (window_end > window_start)
);
CREATE INDEX agreement_snapshots_project_idx ON agreement_snapshots (project_id, statistic, window_end DESC);

-- Across projects and organisations, so no org_id and no RLS; the API serves
-- a contributor only their own row. score is null while "building" (brief 8.4).
CREATE TABLE trust_scores (
  id                uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id    uuid NOT NULL REFERENCES contributor_profiles (id),
  score             numeric(5, 4) CHECK (score BETWEEN 0 AND 1),
  gold_accuracy     numeric(5, 4) CHECK (gold_accuracy BETWEEN 0 AND 1),
  agreement_rate    numeric(5, 4) CHECK (agreement_rate BETWEEN 0 AND 1),
  review_acceptance numeric(5, 4) CHECK (review_acceptance BETWEEN 0 AND 1),
  gold_count        integer NOT NULL CHECK (gold_count >= 0),
  window_days       integer NOT NULL CHECK (window_days > 0),
  weights           jsonb NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX trust_scores_contributor_idx ON trust_scores (contributor_id, created_at DESC);

CREATE TRIGGER assignments_updated_at BEFORE UPDATE ON assignments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER submissions_updated_at BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER content_reports_updated_at BEFORE UPDATE ON content_reports FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER consensus_results_updated_at BEFORE UPDATE ON consensus_results FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER feedback_updated_at BEFORE UPDATE ON feedback FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE trust_scores, agreement_snapshots, feedback, reviews, consensus_results, gold_results,
  content_reports, skips, submission_revisions, submissions, assignments;

-- Foundation: ids, timestamps, the application role, tenancy scope helpers
-- and platform tables (settings, audit log, job queue, idempotency, email).

-- +goose Up

-- UUID version 7 (time-ordered). PostgreSQL 18 has uuidv7(); 17 does not.
-- Takes a random v4 UUID, writes the Unix time in milliseconds into the first
-- 48 bits and sets the version bits from 0100 to 0111.
-- +goose StatementBegin
CREATE FUNCTION uuid_v7() RETURNS uuid
LANGUAGE sql VOLATILE PARALLEL SAFE AS $$
  SELECT encode(
    set_bit(
      set_bit(
        overlay(uuid_send(gen_random_uuid())
                PLACING substring(int8send((extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
                FROM 1 FOR 6),
        52, 1),
      53, 1),
    'hex')::uuid;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- The API runs every transaction as mezai_app (SET LOCAL ROLE), a role that
-- does not own the tables, so row-level security always applies (D-026).
-- +goose StatementBegin
DO $$
BEGIN
  CREATE ROLE mezai_app NOLOGIN;
EXCEPTION
  -- Roles are cluster-wide: another database (or a parallel test) made it already.
  WHEN duplicate_object OR unique_violation THEN NULL;
END
$$;
-- +goose StatementEnd
GRANT mezai_app TO CURRENT_USER;
GRANT USAGE ON SCHEMA public TO mezai_app;

-- Tenancy scope, set per transaction by the API with set_config(..., true):
--   app.scope          'staff' | 'org' | 'contributor' (unset means no access)
--   app.org_id         the caller's organisation (scope 'org')
--   app.contributor_id the caller's contributor profile (scope 'contributor')
-- +goose StatementBegin
CREATE FUNCTION app_scope() RETURNS text
LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.scope', true), '') $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION app_org_id() RETURNS uuid
LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.org_id', true), '')::uuid $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION app_contributor_id() RETURNS uuid
LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.contributor_id', true), '')::uuid $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION app_is_staff() RETURNS boolean
LANGUAGE sql STABLE AS $$ SELECT coalesce(app_scope() = 'staff', false) $$;
-- +goose StatementEnd

-- Staff see every organisation; client users see only their own.
-- +goose StatementBegin
CREATE FUNCTION app_sees_org(org uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT app_is_staff() OR coalesce(app_scope() = 'org' AND org = app_org_id(), false)
$$;
-- +goose StatementEnd

CREATE TABLE settings (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  key         text NOT NULL UNIQUE,
  value       jsonb NOT NULL,
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Append-only: mezai_app may insert and read, never update or delete.
CREATE TABLE audit_log (
  id            uuid PRIMARY KEY DEFAULT uuid_v7(),
  actor_user_id uuid,
  actor_kind    text NOT NULL CHECK (actor_kind IN ('user', 'system')),
  action        text NOT NULL,
  entity_type   text NOT NULL,
  entity_id     uuid,
  org_id        uuid,
  reason        text,
  before        jsonb,
  after         jsonb,
  request_id    text,
  ip            inet,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (actor_kind = 'system' OR actor_user_id IS NOT NULL)
);
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX audit_log_org_idx ON audit_log (org_id, created_at DESC) WHERE org_id IS NOT NULL;
CREATE INDEX audit_log_actor_idx ON audit_log (actor_user_id, created_at DESC);

-- Job queue: workers claim rows with FOR UPDATE SKIP LOCKED (no Redis).
CREATE TABLE jobs (
  id           uuid PRIMARY KEY DEFAULT uuid_v7(),
  kind         text NOT NULL,
  payload      jsonb NOT NULL DEFAULT '{}',
  status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
  attempts     integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts integer NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
  run_at       timestamptz NOT NULL DEFAULT now(),
  locked_at    timestamptz,
  locked_by    text,
  last_error   text,
  dedupe_key   text UNIQUE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_ready_idx ON jobs (run_at) WHERE status = 'queued';

CREATE TABLE idempotency_keys (
  id              uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id         uuid NOT NULL,
  operation       text NOT NULL,
  key             text NOT NULL CHECK (length(key) BETWEEN 8 AND 200),
  request_hash    bytea NOT NULL,
  response_status integer,
  response_body   jsonb,
  expires_at      timestamptz NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, operation, key)
);
CREATE INDEX idempotency_keys_expiry_idx ON idempotency_keys (expires_at);

CREATE TABLE email_outbox (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  to_address text NOT NULL,
  template   text NOT NULL,
  data       jsonb NOT NULL DEFAULT '{}',
  status     text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'failed')),
  attempts   integer NOT NULL DEFAULT 0,
  last_error text,
  sent_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_outbox_queued_idx ON email_outbox (created_at) WHERE status = 'queued';

CREATE TRIGGER settings_updated_at BEFORE UPDATE ON settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER jobs_updated_at BEFORE UPDATE ON jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER idempotency_keys_updated_at BEFORE UPDATE ON idempotency_keys FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER email_outbox_updated_at BEFORE UPDATE ON email_outbox FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Settings are values, not code (brief 8.4, 13). Weights and thresholds live here.
INSERT INTO settings (key, value, description) VALUES
  ('trust.weights',              '{"gold_accuracy": 0.5, "agreement": 0.3, "review_acceptance": 0.2}', 'Trust score weights (brief 8.4)'),
  ('trust.floor',                '0.70',  'Contributors below this trust score are paused automatically'),
  ('trust.window_days',          '30',    'Rolling window for trust score inputs'),
  ('trust.min_gold_results',     '20',    'Gold results needed before the score stops showing as building'),
  ('gold.default_rate',          '0.10',  'Default share of gold items in a queue'),
  ('assignment.timeout_minutes', '30',    'Reservation expiry for an assigned item'),
  ('review.spread_threshold',    '2',     'Rubric or score spread that sends an item to review'),
  ('review.audit_sample_rate',   '0.05',  'Random share of items sent to review for audit'),
  ('retention.days_after_close', '90',    'Days after project close before task content is deleted or anonymised'),
  ('payout.min_cents',           '50000', 'Minimum payout (KES 500.00)'),
  ('payout.schedule',            '"weekly"', 'Payout schedule'),
  ('payout.currency',            '"KES"', 'Pilot currency');

-- +goose Down
DROP TABLE email_outbox, idempotency_keys, jobs, audit_log, settings;
DROP FUNCTION app_sees_org(uuid), app_is_staff(), app_contributor_id(), app_org_id(), app_scope();
REVOKE USAGE ON SCHEMA public FROM mezai_app;
DROP FUNCTION set_updated_at(), uuid_v7();
-- mezai_app is a cluster-wide role and may own grants in other databases; it is left in place.

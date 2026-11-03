-- Email verification tokens and outbox retry timing (Stage 4b).

-- +goose Up

CREATE TABLE email_verifications (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verifications_user_idx ON email_verifications (user_id) WHERE used_at IS NULL;
CREATE TRIGGER email_verifications_updated_at BEFORE UPDATE ON email_verifications FOR EACH ROW EXECUTE FUNCTION set_updated_at();
GRANT SELECT, INSERT, UPDATE, DELETE ON email_verifications TO mezai_app;

-- Retries back off; a message is picked up once next_attempt_at has passed.
ALTER TABLE email_outbox ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();
DROP INDEX email_outbox_queued_idx;
CREATE INDEX email_outbox_queued_idx ON email_outbox (next_attempt_at) WHERE status = 'queued';

-- +goose Down
DROP INDEX email_outbox_queued_idx;
ALTER TABLE email_outbox DROP COLUMN next_attempt_at;
CREATE INDEX email_outbox_queued_idx ON email_outbox (created_at) WHERE status = 'queued';
DROP TABLE email_verifications;

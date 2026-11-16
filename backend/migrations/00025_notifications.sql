-- Stage 16: contributor notifications by email (qualification result, new
-- project available, feedback received, payout sent) and per-user choices.
--
-- dedupe_key makes a notification go out once however often its cause is
-- seen (a project published, a payout line paid; feedback at most once a
-- day). user_id names the account a message is about, so the choices below
-- can be applied and a deleted account's queued mail found (Stage 17).

-- +goose Up
ALTER TABLE email_outbox
  ADD COLUMN user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
  ADD COLUMN dedupe_key text CHECK (length(dedupe_key) <= 200);
CREATE UNIQUE INDEX email_outbox_dedupe_idx ON email_outbox (dedupe_key) WHERE dedupe_key IS NOT NULL;

-- Optional emails only. Account, qualification and payout emails are always
-- sent (D-141). No row means the defaults (both on).
CREATE TABLE notification_preferences (
  id                uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id           uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
  project_available boolean NOT NULL DEFAULT true,
  feedback_received boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER notification_preferences_updated_at BEFORE UPDATE ON notification_preferences
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
GRANT SELECT, INSERT, UPDATE ON notification_preferences TO mezai_app;

-- +goose Down
DROP TABLE notification_preferences;
DROP INDEX email_outbox_dedupe_idx;
ALTER TABLE email_outbox DROP COLUMN dedupe_key, DROP COLUMN user_id;

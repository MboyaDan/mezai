-- Optional two-step sign-in for staff (Stage 17d, brief 12): time-based
-- one-time codes (RFC 6238) and one-time recovery codes.
--
-- The secret is stored encrypted (AES-GCM, key derived from
-- IDENTITY_HMAC_KEY), so a database dump alone cannot produce codes.
-- totp_last_step stops a code being used twice. A password that is right
-- for an account with two-step on creates a login challenge, not a session.

-- +goose Up
ALTER TABLE users
  ADD COLUMN totp_secret_enc  bytea,
  ADD COLUMN totp_enabled_at  timestamptz,
  ADD COLUMN totp_last_step   bigint,
  ADD COLUMN totp_pending_enc bytea,
  ADD COLUMN totp_pending_at  timestamptz,
  ADD CONSTRAINT users_totp_chk CHECK ((totp_secret_enc IS NULL) = (totp_enabled_at IS NULL)),
  ADD CONSTRAINT users_totp_pending_chk CHECK ((totp_pending_enc IS NULL) = (totp_pending_at IS NULL));

CREATE TABLE login_challenges (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  attempts   integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_challenges_user_idx ON login_challenges (user_id);

CREATE TABLE recovery_codes (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  code_hash  bytea NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, code_hash)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON login_challenges, recovery_codes TO mezai_app;

-- +goose Down
DROP TABLE recovery_codes;
DROP TABLE login_challenges;
ALTER TABLE users DROP CONSTRAINT users_totp_pending_chk, DROP CONSTRAINT users_totp_chk,
  DROP COLUMN totp_pending_at, DROP COLUMN totp_pending_enc, DROP COLUMN totp_last_step,
  DROP COLUMN totp_enabled_at, DROP COLUMN totp_secret_enc;

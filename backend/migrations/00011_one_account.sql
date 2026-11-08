-- One person, one contributor account (Stage 7c, D-078).

-- +goose Up

-- The National ID number is never stored: only an HMAC-SHA256 fingerprint
-- under a server-held key (IDENTITY_HMAC_KEY) and the last 3 digits, which
-- Ops and the contributor see. Both are set together or not at all.
ALTER TABLE contributor_profiles
  ADD COLUMN national_id_fp        bytea CHECK (octet_length(national_id_fp) = 32),
  ADD COLUMN national_id_last3     text CHECK (national_id_last3 ~ '^[0-9]{3}$'),
  ADD COLUMN identity_status       text NOT NULL DEFAULT 'pending'
                                     CHECK (identity_status IN ('pending', 'verified', 'rejected')),
  ADD COLUMN identity_note         text,
  ADD COLUMN identity_decided_by   uuid REFERENCES users (id),
  ADD COLUMN identity_decided_at   timestamptz,
  -- Ops confirmed the name M-Pesa shows for the number matches the profile
  -- name (done at the first payout). Cleared when the number changes.
  ADD COLUMN mpesa_name_checked_at timestamptz,
  ADD COLUMN mpesa_name_checked_by uuid REFERENCES users (id),
  ADD CONSTRAINT contributor_profiles_national_id_chk CHECK ((national_id_fp IS NULL) = (national_id_last3 IS NULL)),
  ADD CONSTRAINT contributor_profiles_identity_note_chk CHECK (identity_status <> 'rejected' OR identity_note IS NOT NULL),
  ADD CONSTRAINT contributor_profiles_name_check_chk CHECK ((mpesa_name_checked_at IS NULL) = (mpesa_name_checked_by IS NULL));

-- One account per M-Pesa number and per National ID number, removed
-- accounts included, so a removed person cannot simply join again.
CREATE UNIQUE INDEX contributor_profiles_mpesa_phone_key ON contributor_profiles (mpesa_phone) WHERE mpesa_phone IS NOT NULL;
CREATE UNIQUE INDEX contributor_profiles_national_id_key ON contributor_profiles (national_id_fp) WHERE national_id_fp IS NOT NULL;
CREATE INDEX contributor_profiles_identity_idx ON contributor_profiles (identity_status);

-- Name comparison for "possible duplicate" hints: case, spacing and
-- surrounding blanks ignored.
-- +goose StatementBegin
CREATE FUNCTION norm_name(n text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE
  AS $$ SELECT lower(regexp_replace(btrim(n), '\s+', ' ', 'g')) $$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION norm_name(text) TO mezai_app;

-- Someone tried to put on their profile a number another account holds.
-- Ops sees these when deciding either account; contributors never do.
-- The number itself is not recorded.
CREATE TABLE identity_flags (
  id                   uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id       uuid NOT NULL REFERENCES contributor_profiles (id),
  other_contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  kind                 text NOT NULL CHECK (kind IN ('mpesa_in_use', 'national_id_in_use')),
  created_at           timestamptz NOT NULL DEFAULT now(),
  CHECK (contributor_id <> other_contributor_id)
);
CREATE INDEX identity_flags_contributor_idx ON identity_flags (contributor_id, created_at DESC);
CREATE INDEX identity_flags_other_idx ON identity_flags (other_contributor_id, created_at DESC);
GRANT SELECT, INSERT ON identity_flags TO mezai_app;

-- +goose Down
DROP TABLE identity_flags;
DROP FUNCTION norm_name(text);
DROP INDEX contributor_profiles_identity_idx;
DROP INDEX contributor_profiles_national_id_key;
DROP INDEX contributor_profiles_mpesa_phone_key;
ALTER TABLE contributor_profiles
  DROP CONSTRAINT contributor_profiles_name_check_chk,
  DROP CONSTRAINT contributor_profiles_identity_note_chk,
  DROP CONSTRAINT contributor_profiles_national_id_chk,
  DROP COLUMN mpesa_name_checked_by,
  DROP COLUMN mpesa_name_checked_at,
  DROP COLUMN identity_decided_at,
  DROP COLUMN identity_decided_by,
  DROP COLUMN identity_note,
  DROP COLUMN identity_status,
  DROP COLUMN national_id_last3,
  DROP COLUMN national_id_fp;

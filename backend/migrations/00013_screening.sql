-- Screening funnel, part 1 (Stage 7d-1, D-100): experience form, optional
-- CV, Ops screening decision and a structured phone interview.

-- +goose Up

ALTER TABLE contributor_profiles
  -- Five short answers (D-100 section 3.1); shape checked by the API.
  ADD COLUMN experience           jsonb NOT NULL DEFAULT '{}',
  ADD COLUMN linkedin_url         text CHECK (linkedin_url ~ '^https://([a-z]{2,3}\.)?linkedin\.com/'),
  -- Optional CV in object storage (key built from IDs only). Deleted on a
  -- decline at once, otherwise when cv_delete_after passes.
  ADD COLUMN cv_file_key          text,
  ADD COLUMN cv_file_name         text,
  ADD COLUMN cv_uploaded_at       timestamptz,
  ADD COLUMN cv_delete_after      timestamptz,
  ADD COLUMN screening_status     text NOT NULL DEFAULT 'pending'
                                    CHECK (screening_status IN ('pending', 'shortlisted', 'waitlisted', 'declined')),
  ADD COLUMN screening_note       text,
  ADD COLUMN screening_decided_by uuid REFERENCES users (id),
  ADD COLUMN screening_decided_at timestamptz,
  ADD CONSTRAINT contributor_profiles_cv_chk CHECK ((cv_file_key IS NULL) = (cv_file_name IS NULL) AND (cv_file_key IS NULL) = (cv_uploaded_at IS NULL)),
  ADD CONSTRAINT contributor_profiles_decline_note_chk CHECK (screening_status <> 'declined' OR screening_note IS NOT NULL);
CREATE INDEX contributor_profiles_screening_idx ON contributor_profiles (screening_status);
CREATE INDEX contributor_profiles_cv_delete_idx ON contributor_profiles (cv_delete_after) WHERE cv_file_key IS NOT NULL;

-- Contributors who were already working before the funnel keep working.
UPDATE contributor_profiles SET screening_status = 'shortlisted', screening_decided_at = now()
WHERE status IN ('active', 'paused');

-- Every interview call, append-only. A passed interview is needed once per
-- person (D-100 section 3.4). No recording is kept; only these scores.
CREATE TABLE interviews (
  id                  uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id      uuid NOT NULL REFERENCES contributor_profiles (id),
  interviewer_id      uuid NOT NULL REFERENCES users (id),
  outcome             text NOT NULL CHECK (outcome IN ('passed', 'failed', 'no_answer')),
  identity_confirmed  boolean NOT NULL DEFAULT false,
  language_scores     jsonb NOT NULL DEFAULT '{}',   -- language code → 1 to 4
  scenario_score      smallint CHECK (scenario_score BETWEEN 1 AND 4),
  confidentiality_ok  boolean,
  availability        text CHECK (length(availability) <= 300),
  note                text CHECK (length(note) <= 1000),
  created_at          timestamptz NOT NULL DEFAULT now(),
  CHECK (outcome = 'no_answer' OR (scenario_score IS NOT NULL AND confidentiality_ok IS NOT NULL))
);
CREATE INDEX interviews_contributor_idx ON interviews (contributor_id, created_at DESC);
GRANT SELECT, INSERT ON interviews TO mezai_app;

-- +goose Down
DROP TABLE interviews;
DROP INDEX contributor_profiles_cv_delete_idx;
DROP INDEX contributor_profiles_screening_idx;
ALTER TABLE contributor_profiles
  DROP CONSTRAINT contributor_profiles_decline_note_chk,
  DROP CONSTRAINT contributor_profiles_cv_chk,
  DROP COLUMN screening_decided_at, DROP COLUMN screening_decided_by, DROP COLUMN screening_note,
  DROP COLUMN screening_status, DROP COLUMN cv_delete_after, DROP COLUMN cv_uploaded_at,
  DROP COLUMN cv_file_name, DROP COLUMN cv_file_key, DROP COLUMN linkedin_url, DROP COLUMN experience;

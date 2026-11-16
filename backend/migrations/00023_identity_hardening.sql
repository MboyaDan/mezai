-- Stage 17a: the National ID fingerprint gets a scheme version (D-136).
-- Version 1 is the keyed hash from Stage 7c; version 2 adds Argon2id so
-- that even with the key, trying every possible number is slow. Existing
-- fingerprints stay version 1 (the number is not stored, so they cannot be
-- recomputed) and still match; they are upgraded when their owner next saves
-- the same number.

-- +goose Up
ALTER TABLE contributor_profiles ADD COLUMN national_id_fp_v smallint CHECK (national_id_fp_v IN (1, 2));
UPDATE contributor_profiles SET national_id_fp_v = 1 WHERE national_id_fp IS NOT NULL;
ALTER TABLE contributor_profiles ADD CONSTRAINT contributor_profiles_national_id_v_chk
  CHECK ((national_id_fp IS NULL) = (national_id_fp_v IS NULL));

-- +goose Down
ALTER TABLE contributor_profiles DROP CONSTRAINT contributor_profiles_national_id_v_chk;
ALTER TABLE contributor_profiles DROP COLUMN national_id_fp_v;

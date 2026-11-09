-- Screening funnel, part 2 (Stage 7d-2, D-100 section 3.3): the generalist
-- core check, mixed question types with a writing sample, and skill expiry.

-- +goose Up

-- "core" tests are the one-off generalist check; skill_value is "general".
ALTER TABLE qualification_tests DROP CONSTRAINT qualification_tests_skill_kind_check;
ALTER TABLE qualification_tests ADD CONSTRAINT qualification_tests_skill_kind_check
  CHECK (skill_kind IN ('language', 'domain', 'task_type', 'core'));
-- At most one active core check at a time.
CREATE UNIQUE INDEX qualification_tests_one_core_idx ON qualification_tests ((true)) WHERE skill_kind = 'core' AND status = 'active';

-- A question may use its own task type and settings (the core check mixes
-- them); null means the test's. "writing" answers are read by Ops, not scored.
ALTER TABLE qualification_questions
  ADD COLUMN task_type text CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'writing')),
  ADD COLUMN config    jsonb;

-- Ops rate the writing sample when deciding: 1 weak, 2 adequate, 3 strong.
ALTER TABLE qualification_attempts ADD COLUMN writing_rating smallint CHECK (writing_rating BETWEEN 1 AND 3);

-- +goose Down
ALTER TABLE qualification_attempts DROP COLUMN writing_rating;
ALTER TABLE qualification_questions DROP COLUMN config, DROP COLUMN task_type;
DROP INDEX qualification_tests_one_core_idx;
DELETE FROM qualification_questions WHERE test_id IN (SELECT id FROM qualification_tests WHERE skill_kind = 'core');
DELETE FROM qualification_attempts WHERE test_id IN (SELECT id FROM qualification_tests WHERE skill_kind = 'core');
DELETE FROM qualification_tests WHERE skill_kind = 'core';
ALTER TABLE qualification_tests DROP CONSTRAINT qualification_tests_skill_kind_check;
ALTER TABLE qualification_tests ADD CONSTRAINT qualification_tests_skill_kind_check
  CHECK (skill_kind IN ('language', 'domain', 'task_type'));

-- Contributor onboarding and qualification (Stage 7, brief 6.2).

-- +goose Up

-- Brief section 4 lists profile.manage for contributors; 00002 missed it.
INSERT INTO permissions (key, description, grantable) VALUES
  ('profile.manage', 'Manage own contributor profile and consents', true);
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE p.key = 'profile.manage' AND r.key IN ('super_admin', 'contributor');

-- Task settings a question needs to be answered: labels and multi_label for
-- classification, rubric for pairwise. Copied from the source project.
ALTER TABLE qualification_tests ADD COLUMN config jsonb NOT NULL DEFAULT '{}';
-- Where the questions were copied from, for Ops (never shown to contributors).
ALTER TABLE qualification_tests ADD COLUMN source_project_id uuid REFERENCES projects (id);
ALTER TABLE qualification_tests ADD COLUMN created_by uuid REFERENCES users (id);

-- One open attempt (in progress or awaiting Ops) per contributor and test.
CREATE UNIQUE INDEX qualification_attempts_open_idx ON qualification_attempts (contributor_id, test_id)
  WHERE status IN ('in_progress', 'scored');

-- Per-question result, written when an attempt is scored. Ops sees it; the
-- contributor sees only the total.
ALTER TABLE qualification_attempts ADD COLUMN results jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE qualification_attempts DROP COLUMN results;
DROP INDEX qualification_attempts_open_idx;
ALTER TABLE qualification_tests DROP COLUMN created_by, DROP COLUMN source_project_id, DROP COLUMN config;
DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE key = 'profile.manage');
DELETE FROM permissions WHERE key = 'profile.manage';

-- Contributor quality, earnings and training views (Stage 12a, brief 4, 6.2, 13).

-- +goose Up

-- Brief section 4 lists earnings.read_own for contributors; 00002 missed it.
-- It covers the contributor's own earnings, payouts and quality record, so a
-- paused contributor (who keeps the role) still sees what they are owed.
INSERT INTO permissions (key, description, grantable) VALUES
  ('earnings.read_own', 'See own earnings, payouts and quality record', true);
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE p.key = 'earnings.read_own' AND r.key IN ('super_admin', 'contributor');

-- The next payout date contributors see on their dashboard. Null hides it;
-- Ops sets it with the payout batch screens (Stage 14).
INSERT INTO settings (key, value, description) VALUES
  ('payout.next_date', 'null', 'Next payout date shown to contributors (YYYY-MM-DD), or null to hide it');

-- Payout history per contributor.
CREATE INDEX payout_lines_contributor_idx ON payout_lines (contributor_id, created_at DESC);

-- +goose Down
DROP INDEX payout_lines_contributor_idx;
DELETE FROM settings WHERE key = 'payout.next_date';
DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE key = 'earnings.read_own');
DELETE FROM permissions WHERE key = 'earnings.read_own';

-- name: GetUserForLogin :one
SELECT id, email, display_name, password_hash, email_verified_at, status, failed_login_count, locked_until
FROM users WHERE email = lower(@email);

-- name: RecordFailedLogin :one
-- Counts a failed password and locks the account once the count reaches max_failures.
UPDATE users
SET failed_login_count = failed_login_count + 1,
    locked_until = CASE WHEN failed_login_count + 1 >= @max_failures::int
                        THEN now() + make_interval(secs => @lock_seconds::int)
                        ELSE locked_until END
WHERE id = @id
RETURNING failed_login_count, locked_until;

-- name: RecordSuccessfulLogin :exec
UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = now() WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, csrf_secret, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetActiveSession :one
-- A session is usable when not revoked, not past its absolute expiry and seen within the idle window.
SELECT s.id, s.user_id, s.csrf_secret, s.last_seen_at, u.email, u.display_name, u.status
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND s.last_seen_at > now() - make_interval(secs => @idle_seconds::int);

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now() WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: ListUserGrants :many
-- One row per permission the user holds, with the role it comes from and the
-- organisation for org-scoped roles.
SELECT r.key AS role_key, r.scope AS role_scope, ur.org_id, p.key AS permission_key
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
WHERE ur.user_id = $1;

-- name: GetContributorIDByUser :one
SELECT id FROM contributor_profiles WHERE user_id = $1;

-- name: CountUsersWithRole :one
SELECT count(DISTINCT ur.user_id)
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN users u ON u.id = ur.user_id
WHERE r.key = $1 AND u.status = 'active';

-- name: InsertAuditLog :exec
INSERT INTO audit_log (actor_user_id, actor_kind, action, entity_type, entity_id, org_id, reason, before, after, request_id, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

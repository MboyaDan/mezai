-- Email outbox ------------------------------------------------------------

-- name: EnqueueEmail :exec
INSERT INTO email_outbox (to_address, template, data) VALUES ($1, $2, $3);

-- name: ClaimEmails :many
-- Claims due messages; concurrent workers skip each other's rows.
SELECT id, to_address, template, data, attempts
FROM email_outbox
WHERE status = 'queued' AND next_attempt_at <= now()
ORDER BY next_attempt_at
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkEmailSent :exec
-- Sent messages keep no data: links in it carry one-time tokens.
UPDATE email_outbox SET status = 'sent', sent_at = now(), attempts = attempts + 1, data = '{}', last_error = NULL
WHERE id = $1;

-- name: MarkEmailFailed :exec
UPDATE email_outbox
SET attempts = attempts + 1,
    last_error = @last_error,
    status = CASE WHEN attempts + 1 >= @max_attempts::int THEN 'failed' ELSE 'queued' END,
    data = CASE WHEN attempts + 1 >= @max_attempts::int THEN '{}'::jsonb ELSE data END,
    next_attempt_at = now() + make_interval(secs => @retry_seconds::int)
WHERE id = @id;

-- Verification and password reset -----------------------------------------

-- name: CreateEmailVerification :exec
INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: UseEmailVerification :one
-- Marks a valid token used and returns its user; no row if unknown, used or expired.
UPDATE email_verifications SET used_at = now()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = coalesce(email_verified_at, now()) WHERE id = $1;

-- name: CreatePasswordReset :exec
INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: InvalidatePasswordResets :exec
UPDATE password_resets SET used_at = now() WHERE user_id = $1 AND used_at IS NULL;

-- name: UsePasswordReset :one
UPDATE password_resets SET used_at = now()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: SetPassword :exec
-- A new password also clears any lockout.
UPDATE users SET password_hash = $2, failed_login_count = 0, locked_until = NULL WHERE id = $1;

-- name: RevokeUserSessions :exec
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: GetUserByID :one
SELECT id, email, display_name, status, email_verified_at, created_at FROM users WHERE id = $1;

-- Invitations ---------------------------------------------------------------

-- name: CreateInvitation :one
INSERT INTO invitations (email, role_id, org_id, token_hash, invited_by, expires_at)
VALUES (lower(@email), @role_id, @org_id, @token_hash, @invited_by, @expires_at)
RETURNING id, email, org_id, expires_at, created_at;

-- name: RevokePendingInvitations :exec
-- A new invitation to the same address replaces any pending one.
UPDATE invitations SET revoked_at = now()
WHERE email = lower(@email) AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: GetPendingInvitationByToken :one
SELECT i.id, i.email, i.org_id, i.invited_by, r.id AS role_id, r.key AS role_key
FROM invitations i JOIN roles r ON r.id = i.role_id
WHERE i.token_hash = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()
FOR UPDATE OF i;

-- name: MarkInvitationAccepted :exec
UPDATE invitations SET accepted_at = now() WHERE id = $1;

-- name: GetInvitation :one
SELECT i.id, i.email, i.org_id, i.accepted_at, i.revoked_at, r.key AS role_key
FROM invitations i JOIN roles r ON r.id = i.role_id
WHERE i.id = $1;

-- name: RevokeInvitation :exec
UPDATE invitations SET revoked_at = now() WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: ListPendingInvitations :many
-- Pending invitations, optionally for one organisation.
SELECT i.id, i.email, i.org_id, i.expires_at, i.created_at, r.key AS role_key
FROM invitations i JOIN roles r ON r.id = i.role_id
WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()
  AND (sqlc.narg(org_id)::uuid IS NULL OR i.org_id = sqlc.narg(org_id))
ORDER BY i.created_at DESC;

-- Roles and users -----------------------------------------------------------

-- name: GetRoleWithGrantability :one
-- A role is grantable through the UI only if it holds no non-grantable permission (system.admin).
SELECT r.id, r.key, r.name, r.scope,
       NOT EXISTS (SELECT 1 FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
                   WHERE rp.role_id = r.id AND NOT p.grantable) AS grantable
FROM roles r WHERE r.key = $1;

-- name: ListRolesWithPermissions :many
SELECT r.key, r.name, r.description, r.scope,
       coalesce(array_agg(p.key ORDER BY p.key) FILTER (WHERE p.key IS NOT NULL), '{}')::text[] AS permissions
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
LEFT JOIN permissions p ON p.id = rp.permission_id
GROUP BY r.id
ORDER BY r.scope, r.name;

-- name: ListPermissions :many
SELECT key, description, grantable FROM permissions ORDER BY key;

-- name: GetPermissionByKey :one
SELECT id, key, grantable FROM permissions WHERE key = $1;

-- name: ClearRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListUsersWithRoles :many
-- Staff and client users (contributors are managed in their own screens).
SELECT u.id, u.email, u.display_name, u.status, u.email_verified_at, u.last_login_at,
       coalesce(array_agg(r.key || coalesce(':' || ur.org_id::text, '') ORDER BY r.key)
                FILTER (WHERE r.key IS NOT NULL), '{}')::text[] AS roles
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
WHERE NOT EXISTS (SELECT 1 FROM contributor_profiles c WHERE c.user_id = u.id)
GROUP BY u.id
ORDER BY u.display_name;

-- name: RemoveUserRole :execrows
DELETE FROM user_roles
WHERE user_id = @user_id AND role_id = @role_id AND org_id IS NOT DISTINCT FROM sqlc.narg(org_id)::uuid;

-- name: SetUserStatus :exec
UPDATE users SET status = $2 WHERE id = $1;

-- name: LockSuperAdminCount :one
-- Counts active Super Admins while holding a lock on their role rows, so two
-- concurrent removals cannot both pass the "not the last one" check.
SELECT count(*) FROM (
  SELECT ur.user_id FROM user_roles ur
  JOIN roles r ON r.id = ur.role_id
  JOIN users u ON u.id = ur.user_id
  WHERE r.key = 'super_admin' AND u.status = 'active'
  FOR UPDATE OF ur
) s;

-- name: IsUserSuperAdmin :one
SELECT EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
               WHERE ur.user_id = $1 AND r.key = 'super_admin');

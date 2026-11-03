-- name: CreateUser :one
INSERT INTO users (email, display_name, password_hash, email_verified_at)
VALUES (lower(@email), @display_name, @password_hash, @email_verified_at)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = lower(@email);

-- name: GetRoleByKey :one
SELECT * FROM roles WHERE key = $1;

-- name: AddUserRole :exec
INSERT INTO user_roles (user_id, role_id, org_id, granted_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: ListUserPermissionKeys :many
-- Permission keys a user holds on the platform plus, when org_id is given, in that organisation.
SELECT DISTINCT p.key
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE ur.user_id = @user_id AND (ur.org_id IS NULL OR ur.org_id = sqlc.narg(org_id))
ORDER BY p.key;

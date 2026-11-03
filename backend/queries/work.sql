-- name: CreateAssignment :one
INSERT INTO assignments (org_id, project_id, item_id, contributor_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

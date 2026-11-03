-- name: CreateOrganisation :one
INSERT INTO organisations (name, slug) VALUES ($1, $2) RETURNING *;

-- name: GetOrganisationBySlug :one
SELECT * FROM organisations WHERE slug = $1;

-- name: ListOrganisations :many
SELECT * FROM organisations ORDER BY name;

-- name: AddOrgMember :exec
INSERT INTO org_members (org_id, user_id, job_title) VALUES ($1, $2, $3)
ON CONFLICT (org_id, user_id) DO NOTHING;

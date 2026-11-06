-- name: CreateOrganisation :one
INSERT INTO organisations (name, slug) VALUES ($1, $2) RETURNING *;

-- name: GetOrganisationBySlug :one
SELECT * FROM organisations WHERE slug = $1;

-- name: ListOrganisations :many
SELECT * FROM organisations ORDER BY name;

-- name: AddOrgMember :exec
INSERT INTO org_members (org_id, user_id, job_title) VALUES ($1, $2, $3)
ON CONFLICT (org_id, user_id) DO NOTHING;

-- name: GetOrganisation :one
SELECT * FROM organisations WHERE id = $1;

-- name: ListOrganisationSummaries :many
SELECT o.*,
  (SELECT count(*) FROM projects p WHERE p.org_id = o.id)::int AS project_count,
  (SELECT count(*) FROM org_members m WHERE m.org_id = o.id)::int AS member_count
FROM organisations o
ORDER BY o.name;

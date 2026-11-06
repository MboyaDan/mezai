-- name: CreateProjectRequestWithID :one
-- The ID is chosen first so the sample file's storage key can use it.
INSERT INTO project_requests (id, org_id, title, task_type, brief_md, languages, volume_estimate, wanted_by,
  contact_name, contact_email, sample_file_key, sample_file_name, sample_size, created_by)
VALUES (@id, @org_id, @title, @task_type, @brief_md, @languages, @volume_estimate, @wanted_by,
  @contact_name, @contact_email, @sample_file_key, @sample_file_name, @sample_size, @created_by)
RETURNING *;

-- name: ListProjectRequests :many
-- RLS limits clients to their organisation.
SELECT r.*, o.name AS org_name
FROM project_requests r JOIN organisations o ON o.id = r.org_id
WHERE (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text)
ORDER BY r.created_at DESC
LIMIT 500;

-- name: GetProjectRequest :one
SELECT r.*, o.name AS org_name
FROM project_requests r JOIN organisations o ON o.id = r.org_id
WHERE r.id = $1;

-- name: LockProjectRequest :one
SELECT * FROM project_requests WHERE id = $1 FOR UPDATE;

-- name: DecideProjectRequest :one
UPDATE project_requests SET status = @status, decision_note = sqlc.narg(decision_note),
  project_id = sqlc.narg(project_id), decided_by = @decided_by, decided_at = now()
WHERE id = @id AND status = 'submitted'
RETURNING *;

-- name: CreateItem :one
INSERT INTO items (org_id, project_id, external_id, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateGoldItem :one
INSERT INTO gold_items (org_id, project_id, item_id, expected, tolerance, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListItemsByProject :many
SELECT * FROM items WHERE project_id = $1 ORDER BY created_at, id;

-- name: CountItems :one
SELECT count(*) FROM items;

-- name: CountGoldItems :one
SELECT count(*) FROM gold_items;

-- name: ExternalIDsInProject :many
SELECT external_id::text FROM items WHERE project_id = $1 AND external_id IS NOT NULL;

-- name: CreateImportJob :one
INSERT INTO import_jobs (id, org_id, project_id, kind, format, file_key, file_name, size_bytes, sha256,
  status, total_rows, valid_rows, error_rows, created_by)
VALUES (@id, @org_id, @project_id, @kind, @format, @file_key, @file_name, @size_bytes, @sha256,
  @status, @total_rows, @valid_rows, @error_rows, @created_by)
RETURNING *;

-- name: AddImportError :exec
INSERT INTO import_errors (org_id, import_job_id, row_number, field, message)
VALUES ($1, $2, $3, $4, $5);

-- name: ListImportJobs :many
SELECT * FROM import_jobs WHERE project_id = $1 ORDER BY created_at DESC LIMIT 200;

-- name: GetImportJob :one
SELECT * FROM import_jobs WHERE id = $1;

-- name: LockImportJob :one
SELECT * FROM import_jobs WHERE id = $1 FOR UPDATE;

-- name: ListImportErrors :many
SELECT * FROM import_errors WHERE import_job_id = $1 ORDER BY row_number, id LIMIT 1000;

-- name: MarkImportCommitted :exec
UPDATE import_jobs SET status = 'committed', dry_run = false, committed_at = now() WHERE id = $1;

-- name: CreateImportedItem :one
INSERT INTO items (org_id, project_id, external_id, content, import_job_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: ListGoldByProject :many
SELECT g.id, g.item_id, g.status, g.expected, g.tolerance, g.usage_count, g.created_at, g.retired_at,
  i.external_id, i.content
FROM gold_items g JOIN items i ON i.id = g.item_id
WHERE g.project_id = $1
ORDER BY g.status, g.created_at, g.id
LIMIT 2000;

-- name: GetGoldItem :one
SELECT * FROM gold_items WHERE id = $1;

-- name: RetireGold :one
UPDATE gold_items SET status = 'retired', retired_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

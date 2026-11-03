-- name: CreateDeliverable :one
INSERT INTO deliverables (
  org_id, project_id, format, file_key, manifest_key, manifest, checksum_sha256,
  schema_version, item_count, published_at, published_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListDeliverables :many
SELECT * FROM deliverables ORDER BY created_at;

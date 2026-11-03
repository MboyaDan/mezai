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

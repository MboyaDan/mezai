-- name: GetSetting :one
SELECT value FROM settings WHERE key = $1;

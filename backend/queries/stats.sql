-- name: ListPublishedStats :many
-- Only published records with a source and a verification date reach the public site (honesty rule 2).
SELECT key, label, value, unit, source, source_url, verified_at
FROM stats
WHERE published AND length(source) > 0
ORDER BY key;

-- name: CreateDeliverable :one
INSERT INTO deliverables (
  org_id, project_id, format, file_key, manifest_key, manifest, checksum_sha256,
  schema_version, item_count, published_at, published_by, version, byte_size, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: ListDeliverables :many
SELECT * FROM deliverables ORDER BY created_at;

-- Deliverable exports (Stage 11b, brief 8.5). Exports read in system scope
-- after the service has checked the caller may manage the project.

-- name: LockProjectForExport :one
-- Serialises exports of one project so two clicks cannot take one version.
SELECT p.id, p.org_id, p.name, p.task_type, p.config, p.overlap_k, o.name AS org_name
FROM projects p JOIN organisations o ON o.id = p.org_id
WHERE p.id = $1 FOR UPDATE OF p;

-- name: NextDeliverableVersion :one
SELECT (coalesce(max(version), 0) + 1)::int FROM deliverables WHERE project_id = $1;

-- name: ExportItems :many
-- Final regular items with their current final answer. Gold and rejected
-- items are never delivered. agreeing / rated is the share of answers that
-- agree with the final answer.
SELECT i.id, i.external_id, i.content, i.finalised_at, r.revision, r.answer, r.source,
  (SELECT count(*) FROM submissions s WHERE s.item_id = i.id AND s.status IN ('accepted', 'rejected') AND s.agrees_with_final IS NOT NULL)::int AS rated,
  (SELECT count(*) FROM submissions s WHERE s.item_id = i.id AND s.status IN ('accepted', 'rejected') AND s.agrees_with_final)::int AS agreeing,
  coalesce((SELECT array_agg(c.public_code ORDER BY s.submitted_at, s.id)
     FROM submissions s JOIN contributor_profiles c ON c.id = s.contributor_id
     WHERE s.item_id = i.id AND s.status IN ('accepted', 'rejected')), '{}')::text[] AS contributors
FROM items i
JOIN item_revisions r ON r.item_id = i.id AND r.revision = i.current_revision
WHERE i.project_id = $1 AND i.status = 'final'
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
ORDER BY i.created_at, i.id
LIMIT sqlc.arg(max_items);

-- name: ExportAnswers :many
-- Every decided answer on the exported items, for the agreement figures.
SELECT s.item_id, r.answer
FROM items i
JOIN submissions s ON s.item_id = i.id AND s.status IN ('accepted', 'rejected')
JOIN submission_revisions r ON r.submission_id = s.id AND r.revision = s.current_revision
WHERE i.project_id = $1 AND i.status = 'final'
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
ORDER BY s.item_id, s.submitted_at, s.id;

-- name: ExportItemCounts :one
SELECT
  count(*)::int AS total,
  count(*) FILTER (WHERE i.status = 'final')::int AS final,
  count(*) FILTER (WHERE i.status = 'final' AND r.source = 'consensus')::int AS consensus,
  count(*) FILTER (WHERE i.status = 'final' AND r.source IN ('review', 'correction'))::int AS adjudicated,
  count(*) FILTER (WHERE i.status = 'rejected')::int AS rejected,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id)::int AS completed,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.routed_to_review)::int AS routed
FROM items i
LEFT JOIN item_revisions r ON r.item_id = i.id AND r.revision = i.current_revision
WHERE i.project_id = @project_id
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id);

-- name: ExportRouteReasons :many
SELECT route_reason::text AS reason, count(*)::int AS n
FROM consensus_results WHERE project_id = $1 AND routed_to_review
GROUP BY route_reason ORDER BY route_reason;

-- name: ExportGoldByContributor :many
-- All-time gold results on the project per contributor (IDs only).
SELECT c.public_code, count(*)::int AS total, count(*) FILTER (WHERE g.correct)::int AS correct
FROM gold_results g
JOIN gold_items gi ON gi.id = g.gold_item_id
JOIN contributor_profiles c ON c.id = g.contributor_id
WHERE gi.project_id = $1
GROUP BY c.public_code ORDER BY c.public_code;

-- name: ListProjectDeliverables :many
-- Visible rows only: a client sees published versions (RLS).
SELECT d.id, d.version, d.format, d.byte_size, d.checksum_sha256, d.item_count, d.schema_version,
  d.manifest, d.published_at, d.created_at,
  (SELECT count(*) FROM deliverable_downloads dd WHERE dd.deliverable_id = d.id)::int AS downloads
FROM deliverables d
WHERE d.project_id = $1
ORDER BY d.version DESC, d.format;

-- name: PublishDeliverableVersion :many
UPDATE deliverables SET published_at = now(), published_by = @published_by
WHERE project_id = @project_id AND version = @version AND published_at IS NULL
RETURNING id, org_id;

-- name: CountDeliverableVersion :one
SELECT count(*)::int FROM deliverables WHERE project_id = @project_id AND version = @version;

-- name: GetDeliverable :one
SELECT d.id, d.org_id, d.project_id, d.version, d.format, d.file_key, d.manifest_key, d.byte_size,
  d.checksum_sha256, d.published_at, p.name AS project_name
FROM deliverables d JOIN projects p ON p.id = d.project_id
WHERE d.id = $1;

-- name: RecordDeliverableDownload :exec
INSERT INTO deliverable_downloads (org_id, deliverable_id, user_id, ip, file)
VALUES (@org_id, @deliverable_id, @user_id, sqlc.narg(ip), @file);

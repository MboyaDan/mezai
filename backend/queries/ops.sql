-- Ops overview (Stage 13b, brief 6.3: projects, queues, contributor health
-- and deadlines). Staff only; read in the caller's staff scope.

-- name: OpsQueues :one
-- Each count matches the list it links to in the workspace.
SELECT
  (SELECT count(*) FROM items WHERE status = 'in_review')::int AS review,
  (SELECT count(*) FROM content_reports WHERE status = 'open')::int AS reports,
  (SELECT count(*) FROM qualification_attempts WHERE status = 'scored')::int AS attempts,
  (SELECT count(*) FROM contributor_profiles
     WHERE status = 'qualifying' AND screening_status = 'pending')::int AS applications,
  (SELECT count(*) FROM contributor_profiles cp
     WHERE cp.status = 'qualifying' AND cp.screening_status = 'shortlisted'
       AND EXISTS (SELECT 1 FROM qualification_attempts qa WHERE qa.contributor_id = cp.id AND qa.status = 'approved')
       AND NOT EXISTS (SELECT 1 FROM interviews iv WHERE iv.contributor_id = cp.id AND iv.outcome = 'passed'))::int AS interviews,
  (SELECT count(*) FROM contributor_profiles
     WHERE identity_status = 'pending' AND national_id_last3 IS NOT NULL AND status <> 'removed')::int AS identity_checks,
  (SELECT count(*) FROM project_requests WHERE status = 'submitted')::int AS project_requests;

-- name: OpsOldestReview :many
-- The item waiting longest for review (none or one row).
SELECT updated_at FROM items WHERE status = 'in_review' ORDER BY updated_at LIMIT 1;

-- name: OpsProjects :many
-- Projects that are not closed, with the figures the pace needs (A-074).
-- Gold checks are never counted as items.
SELECT p.id, p.name, p.status, p.task_type, p.deadline, p.target_count, p.org_id, o.name AS org_name,
  c.total, c.final, c.in_review, c.recent_final,
  (SELECT count(DISTINCT s.contributor_id) FROM submissions s
     WHERE s.project_id = p.id AND s.submitted_at >= @active_since)::int AS active_contributors,
  (SELECT count(*) FROM assignments a
     WHERE a.project_id = p.id AND a.status = 'reserved' AND a.expires_at > now())::int AS reserved,
  (SELECT count(*) FROM gold_results g JOIN gold_items gi ON gi.id = g.gold_item_id
     WHERE gi.project_id = p.id AND g.created_at >= @gold_since)::int AS gold_total,
  (SELECT count(*) FROM gold_results g JOIN gold_items gi ON gi.id = g.gold_item_id
     WHERE gi.project_id = p.id AND g.created_at >= @gold_since AND g.correct)::int AS gold_correct
FROM projects p
JOIN organisations o ON o.id = p.org_id
CROSS JOIN LATERAL (
  SELECT count(*)::int AS total,
         count(*) FILTER (WHERE i.status = 'final')::int AS final,
         count(*) FILTER (WHERE i.status = 'in_review')::int AS in_review,
         count(*) FILTER (WHERE i.status = 'final' AND i.finalised_at >= @pace_since)::int AS recent_final
  FROM items i
  WHERE i.project_id = p.id AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
) c
WHERE p.status <> 'closed'
ORDER BY p.deadline NULLS LAST, p.name;

-- name: OpsContributorHealth :one
-- Contributors by status, who worked recently, and automatic pauses. A
-- trust score counts only once it is no longer building (min_gold).
WITH latest AS (
  SELECT DISTINCT ON (ts.contributor_id) ts.contributor_id, ts.score, ts.gold_accuracy, ts.gold_count, ts.created_at
  FROM trust_scores ts ORDER BY ts.contributor_id, ts.created_at DESC, ts.id DESC
)
SELECT
  count(*) FILTER (WHERE cp.status = 'applied')::int AS applied,
  count(*) FILTER (WHERE cp.status = 'qualifying')::int AS qualifying,
  count(*) FILTER (WHERE cp.status = 'active')::int AS active,
  count(*) FILTER (WHERE cp.status = 'paused')::int AS paused,
  count(*) FILTER (WHERE cp.status = 'removed')::int AS removed,
  (SELECT count(DISTINCT s.contributor_id) FROM submissions s WHERE s.submitted_at >= @active_since)::int AS worked_recently,
  (SELECT count(*) FROM contributor_status_history h
     WHERE h.to_status = 'paused' AND h.changed_by IS NULL AND h.created_at >= @paused_since)::int AS auto_paused,
  count(*) FILTER (WHERE cp.status = 'active' AND t.score IS NOT NULL AND t.gold_count >= @min_gold::int
                   AND t.score < @floor::numeric)::int AS active_below_floor,
  count(*) FILTER (WHERE cp.status = 'active' AND (t.score IS NULL OR t.gold_count < @min_gold::int))::int AS building
FROM contributor_profiles cp
LEFT JOIN latest t ON t.contributor_id = cp.id;

-- name: OpsTrustWatch :many
-- Active and paused contributors with the lowest settled trust scores,
-- for Ops to look at before or after an automatic pause.
WITH latest AS (
  SELECT DISTINCT ON (ts.contributor_id) ts.contributor_id, ts.score, ts.gold_accuracy, ts.gold_count, ts.created_at
  FROM trust_scores ts ORDER BY ts.contributor_id, ts.created_at DESC, ts.id DESC
)
SELECT cp.id, cp.public_code, cp.status, u.display_name, t.score, t.gold_accuracy, t.created_at AS scored_at,
  coalesce((SELECT h.changed_by IS NULL FROM contributor_status_history h
     WHERE h.contributor_id = cp.id ORDER BY h.created_at DESC, h.id DESC LIMIT 1), false)::bool AS last_change_automatic
FROM contributor_profiles cp
JOIN users u ON u.id = cp.user_id
JOIN latest t ON t.contributor_id = cp.id
WHERE cp.status IN ('active', 'paused') AND t.score IS NOT NULL AND t.gold_count >= @min_gold::int
ORDER BY t.score, cp.public_code
LIMIT @lim::int;

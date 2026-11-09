-- name: CreateAssignment :one
INSERT INTO assignments (org_id, project_id, item_id, contributor_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- Task engine (Stage 9a). Writes run in system scope because they touch
-- staff-only rows (item counters, gold usage); every query here that acts
-- for a contributor filters by @contributor_id itself.

-- name: ListLiveProjects :many
SELECT p.id, p.name, p.task_type, p.instructions_md, p.config, p.pay_per_task_cents, p.currency,
  p.deadline, p.reservation_minutes, p.org_id
FROM projects p
WHERE p.status = 'live'
ORDER BY p.published_at DESC NULLS LAST, p.id;

-- name: GetProjectForWork :one
SELECT p.id, p.org_id, p.name, p.status, p.task_type, p.instructions_md, p.config, p.overlap_k, p.gold_rate,
  p.pay_per_task_cents, p.currency, p.deadline, p.reservation_minutes, p.qa_checks, p.min_rationale_chars
FROM projects p WHERE p.id = $1;

-- name: ListSkillsForProjects :many
SELECT project_id, skill_kind, skill_value, min_proficiency FROM project_skills WHERE project_id = ANY(@ids::uuid[]);

-- name: ContributorSkillsHeld :many
-- Skills granted by approved qualification attempts.
SELECT DISTINCT t.skill_kind, t.skill_value
FROM qualification_attempts a JOIN qualification_tests t ON t.id = a.test_id
WHERE a.contributor_id = $1 AND a.status = 'approved'
  -- Skills count for 12 months (D-100 section 3.5); the core check never expires.
  AND (t.skill_kind = 'core' OR a.decided_at > now() - interval '365 days');

-- name: ExpireStaleReservations :exec
UPDATE assignments SET status = 'expired', completed_at = now()
WHERE status = 'reserved' AND expires_at <= now() AND (project_id = @project_id OR contributor_id = @contributor_id);

-- name: CurrentReservation :one
SELECT * FROM assignments
WHERE contributor_id = @contributor_id AND project_id = @project_id AND status = 'reserved' AND expires_at > now();

-- name: PickRegularItem :one
-- An item still short of k submissions (counting live reservations), never
-- seen by this contributor, not gold and not under an open content report.
-- Items closest to complete first, so whole items finish sooner.
SELECT i.id, i.org_id FROM items i
WHERE i.project_id = @project_id AND i.status IN ('pending', 'in_progress')
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
  AND NOT EXISTS (SELECT 1 FROM assignments a WHERE a.item_id = i.id AND a.contributor_id = @contributor_id)
  AND NOT EXISTS (SELECT 1 FROM content_reports r WHERE r.item_id = i.id AND r.status = 'open')
  AND i.submission_count + (SELECT count(*) FROM assignments a
        WHERE a.item_id = i.id AND a.status = 'reserved' AND a.expires_at > now()) < @overlap_k::int
ORDER BY i.submission_count DESC, i.created_at, i.id
LIMIT 1
FOR UPDATE OF i SKIP LOCKED;

-- name: PickGoldItem :one
-- Active gold this contributor has not seen; least used first (rotation).
SELECT g.id, g.item_id, g.org_id FROM gold_items g
WHERE g.project_id = @project_id AND g.status = 'active'
  AND NOT EXISTS (SELECT 1 FROM assignments a WHERE a.item_id = g.item_id AND a.contributor_id = @contributor_id)
  AND NOT EXISTS (SELECT 1 FROM content_reports r WHERE r.item_id = g.item_id AND r.status = 'open')
ORDER BY g.usage_count, g.id
LIMIT 1
FOR UPDATE OF g SKIP LOCKED;

-- name: MarkItemInProgress :exec
UPDATE items SET status = 'in_progress' WHERE id = $1 AND status = 'pending';

-- name: BumpGoldUsage :exec
UPDATE gold_items SET usage_count = usage_count + 1 WHERE id = $1;

-- name: IsGoldItem :one
SELECT EXISTS (SELECT 1 FROM gold_items WHERE item_id = $1)::bool;

-- name: CountMyWork :many
SELECT project_id, count(*) FILTER (WHERE status = 'submitted')::int AS submitted
FROM assignments WHERE contributor_id = $1 GROUP BY project_id;

-- name: LockMyAssignment :one
SELECT * FROM assignments WHERE id = @id AND contributor_id = @contributor_id FOR UPDATE;

-- name: GetMyAssignment :one
SELECT * FROM assignments WHERE id = $1;

-- name: GetItemContent :one
SELECT content FROM items WHERE id = $1;

-- name: GetSubmissionByAssignment :one
SELECT * FROM submissions WHERE assignment_id = $1;

-- name: GetLatestRevision :one
SELECT * FROM submission_revisions WHERE submission_id = $1 ORDER BY revision DESC LIMIT 1;

-- name: CreateDraftSubmission :one
INSERT INTO submissions (org_id, project_id, item_id, assignment_id, contributor_id)
VALUES (@org_id, @project_id, @item_id, @assignment_id, @contributor_id)
RETURNING *;

-- name: AddSubmissionRevision :exec
INSERT INTO submission_revisions (org_id, submission_id, revision, answer, is_draft)
VALUES (@org_id, @submission_id, @revision, @answer, @is_draft);

-- name: SaveDraftProgress :exec
UPDATE submissions SET current_revision = @current_revision, time_on_task_ms = @time_on_task_ms
WHERE id = @id AND status = 'draft';

-- name: MarkSubmitted :exec
UPDATE submissions SET status = 'submitted', submitted_at = now(), idempotency_key = @idempotency_key,
  current_revision = @current_revision, time_on_task_ms = @time_on_task_ms, qa_warnings = @qa_warnings
WHERE id = @id AND status = 'draft';

-- name: GetSubmissionByKey :one
SELECT * FROM submissions WHERE contributor_id = @contributor_id AND idempotency_key = @idempotency_key;

-- name: CompleteAssignment :execrows
UPDATE assignments SET status = @status, completed_at = now()
WHERE id = @id AND status = 'reserved';

-- name: CountItemSubmission :exec
-- k submissions make a regular item ready for consensus; gold items only count.
UPDATE items SET submission_count = submission_count + 1,
  status = CASE WHEN NOT @is_gold::bool AND submission_count + 1 >= @overlap_k::int THEN 'awaiting_consensus' ELSE status END
WHERE id = @id;

-- name: AddSkip :exec
INSERT INTO skips (org_id, assignment_id, contributor_id, reason, note) VALUES (@org_id, @assignment_id, @contributor_id, @reason, @note);

-- name: AddContentReport :one
INSERT INTO content_reports (org_id, item_id, assignment_id, contributor_id, category, note)
VALUES (@org_id, @item_id, @assignment_id, @contributor_id, @category, @note)
RETURNING id;

-- name: ListContentReports :many
SELECT r.id, r.category, r.note, r.status, r.created_at, r.resolved_at, r.item_id, i.external_id, i.project_id,
  p.name AS project_name, c.public_code
FROM content_reports r
JOIN items i ON i.id = r.item_id
JOIN projects p ON p.id = i.project_id
JOIN contributor_profiles c ON c.id = r.contributor_id
WHERE sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text
ORDER BY r.created_at
LIMIT 200;

-- name: LockContentReport :one
SELECT * FROM content_reports WHERE id = $1 FOR UPDATE;

-- name: ResolveContentReport :exec
UPDATE content_reports SET status = @status, resolved_by = @resolved_by, resolved_at = now() WHERE id = @id;

-- name: RejectItem :exec
-- Removed from the work queue for good (an actioned report).
UPDATE items SET status = 'rejected' WHERE id = $1 AND status <> 'final';

-- name: RetireGoldForItem :exec
UPDATE gold_items SET status = 'retired', retired_at = now() WHERE item_id = $1 AND status = 'active';

-- name: ReserveItem :one
-- Expiry from the database clock, the same clock every check compares with.
INSERT INTO assignments (org_id, project_id, item_id, contributor_id, expires_at)
VALUES (@org_id, @project_id, @item_id, @contributor_id, now() + make_interval(mins => @minutes::int))
RETURNING *;

-- name: CountSubmissionRevisions :one
SELECT count(*)::int FROM submission_revisions WHERE submission_id = $1;

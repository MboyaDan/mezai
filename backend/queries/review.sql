-- Review, adjudication and earnings (Stage 11a, brief 8.3, 13).

-- name: AcceptItemSubmissions :many
-- Automatic acceptance (consensus or gold): every submitted answer on the
-- item is accepted and earns (D-110).
UPDATE submissions SET status = 'accepted', decided_at = now(), decision_source = @source::text
WHERE item_id = @item_id AND status = 'submitted'
RETURNING id, contributor_id, project_id;

-- name: AcceptGoldBefore :many
-- Gold work is accepted in the daily batch, not at submit, so the timing of
-- earnings never tells a contributor which items were gold (brief 8.1, D-110).
UPDATE submissions s SET status = 'accepted', decided_at = now(), decision_source = 'gold'
WHERE s.status = 'submitted' AND s.submitted_at < @before::timestamptz
  AND EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = s.item_id)
RETURNING s.id;

-- name: DecideSubmission :one
UPDATE submissions SET status = @status, decided_at = now(), decision_source = 'review', review_id = @review_id,
  rejection_reason = sqlc.narg(rejection_reason)
WHERE id = @id AND status = 'submitted'
RETURNING id, contributor_id, project_id, status;

-- name: AccrueEarning :exec
-- One accrual per submission (unique index): a replay changes nothing (AC-07).
INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, project_id, submission_id)
SELECT s.contributor_id, 'accrual', p.pay_per_task_cents, p.currency, s.project_id, s.id
FROM submissions s JOIN projects p ON p.id = s.project_id
WHERE s.id = @submission_id AND s.status = 'accepted' AND p.pay_per_task_cents > 0
ON CONFLICT DO NOTHING;

-- name: FinaliseItem :exec
INSERT INTO item_revisions (org_id, item_id, revision, answer, source, created_by)
SELECT i.org_id, i.id, i.current_revision + 1, @answer, @source, sqlc.narg(created_by)
FROM items i WHERE i.id = @item_id;

-- name: MarkItemFinal :execrows
UPDATE items SET status = 'final', finalised_at = now(), current_revision = current_revision + 1
WHERE id = @id AND status IN ('consensus', 'in_review');

-- name: MarkItemRejected :execrows
UPDATE items SET status = 'rejected' WHERE id = @id AND status = 'in_review';

-- name: ReviewQueue :many
-- Items waiting for a reviewer, oldest first. all_projects is true for
-- people who may adjudicate anywhere; others see their assigned projects.
SELECT i.id, i.external_id, i.updated_at, i.submission_count, p.id AS project_id, p.name AS project_name,
  p.task_type, o.name AS org_name, c.route_reason, c.agreement, c.spread
FROM items i
JOIN projects p ON p.id = i.project_id
JOIN organisations o ON o.id = p.org_id
JOIN LATERAL (SELECT route_reason, agreement, spread FROM consensus_results cr
              WHERE cr.item_id = i.id ORDER BY cr.created_at DESC LIMIT 1) c ON true
WHERE i.status = 'in_review'
  AND (@all_projects::bool OR EXISTS (SELECT 1 FROM project_reviewers r WHERE r.project_id = p.id AND r.user_id = @user_id))
  AND (sqlc.narg(project_id)::uuid IS NULL OR p.id = sqlc.narg(project_id)::uuid)
  AND (sqlc.narg(reason)::text IS NULL OR c.route_reason = sqlc.narg(reason)::text)
ORDER BY i.updated_at, i.id
LIMIT 200;

-- name: ReviewProjects :many
-- Projects with something to review, for the queue filter.
SELECT p.id, p.name, count(*)::int AS waiting
FROM items i JOIN projects p ON p.id = i.project_id
WHERE i.status = 'in_review'
  AND (@all_projects::bool OR EXISTS (SELECT 1 FROM project_reviewers r WHERE r.project_id = p.id AND r.user_id = @user_id))
GROUP BY p.id, p.name
ORDER BY p.name;

-- name: CanReviewProject :one
SELECT EXISTS (SELECT 1 FROM project_reviewers WHERE project_id = @project_id AND user_id = @user_id)::bool;

-- name: GetReviewItem :one
SELECT i.id, i.org_id, i.project_id, i.external_id, i.content, i.status, i.finalised_at, p.name AS project_name, p.task_type,
  p.config, p.instructions_md
FROM items i JOIN projects p ON p.id = i.project_id
WHERE i.id = $1;

-- name: LockReviewItem :one
SELECT id, org_id, project_id, status FROM items WHERE id = $1 FOR UPDATE;

-- name: LatestConsensus :one
SELECT * FROM consensus_results WHERE item_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: ItemSubmissions :many
-- Every non-draft answer on an item, for the side-by-side view.
SELECT s.id, s.status, s.submitted_at, s.time_on_task_ms, s.qa_warnings, s.rejection_reason, s.agrees_with_final,
  c.public_code, r.answer
FROM submissions s
JOIN contributor_profiles c ON c.id = s.contributor_id
JOIN submission_revisions r ON r.submission_id = s.id AND r.revision = s.current_revision
WHERE s.item_id = $1 AND s.status <> 'draft'
ORDER BY s.submitted_at, s.id;

-- name: GetItemReview :one
SELECT r.*, u.display_name AS reviewer_name
FROM reviews r JOIN users u ON u.id = r.reviewer_user_id
WHERE r.item_id = $1;

-- name: GetReviewByKey :one
SELECT * FROM reviews WHERE reviewer_user_id = @reviewer_user_id AND idempotency_key = @idempotency_key;

-- name: AddReview :one
INSERT INTO reviews (org_id, project_id, item_id, reviewer_user_id, decision, chosen_submission_id, final_answer, reason, idempotency_key)
VALUES (@org_id, @project_id, @item_id, @reviewer_user_id, @decision, sqlc.narg(chosen_submission_id), sqlc.narg(final_answer),
        sqlc.narg(reason), @idempotency_key)
RETURNING *;

-- name: ListProjectReviewers :many
SELECT u.id, u.display_name, u.email, r.created_at
FROM project_reviewers r JOIN users u ON u.id = r.user_id
WHERE r.project_id = $1
ORDER BY u.display_name;

-- name: ReviewerCandidates :many
-- Active people holding review.decide through a platform role (staff).
SELECT DISTINCT u.id, u.display_name, u.email
FROM users u
JOIN user_roles ur ON ur.user_id = u.id AND ur.org_id IS NULL
JOIN roles ro ON ro.id = ur.role_id AND ro.scope = 'platform'
JOIN role_permissions rp ON rp.role_id = ro.id
JOIN permissions pm ON pm.id = rp.permission_id
WHERE pm.key = 'review.decide' AND u.status = 'active'
ORDER BY u.display_name;

-- name: AddProjectReviewer :exec
INSERT INTO project_reviewers (org_id, project_id, user_id, added_by)
VALUES (@org_id, @project_id, @user_id, @added_by)
ON CONFLICT DO NOTHING;

-- name: RemoveProjectReviewer :execrows
DELETE FROM project_reviewers WHERE project_id = @project_id AND user_id = @user_id;

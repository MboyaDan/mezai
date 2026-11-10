-- Quality engine (Stage 10a, brief 8). Runs in system scope inside the
-- submit transaction.

-- name: GetGoldForItem :one
SELECT id, org_id, expected, tolerance FROM gold_items WHERE item_id = $1;

-- name: AddGoldResult :exec
INSERT INTO gold_results (org_id, gold_item_id, submission_id, contributor_id, correct, score, detail)
VALUES (@org_id, @gold_item_id, @submission_id, @contributor_id, @correct, @score, @detail);

-- name: ListFinalAnswersForItem :many
-- Each submitted answer on an item: the final revision, with QA warnings.
SELECT s.id, s.contributor_id, s.qa_warnings, r.answer
FROM submissions s
JOIN submission_revisions r ON r.submission_id = s.id AND r.revision = s.current_revision
WHERE s.item_id = $1 AND s.status = 'submitted'
ORDER BY s.submitted_at, s.id;

-- name: AddConsensus :exec
INSERT INTO consensus_results (org_id, project_id, item_id, method, result, agreement, spread, routed_to_review, route_reason)
VALUES (@org_id, @project_id, @item_id, @method, @result, sqlc.narg(agreement), sqlc.narg(spread), @routed_to_review, sqlc.narg(route_reason));

-- name: SetItemQualityStatus :exec
UPDATE items SET status = @status WHERE id = @id AND status = 'awaiting_consensus';

-- name: SetSubmissionAgreement :exec
UPDATE submissions SET agrees_with_final = @agrees WHERE id = @id;

-- name: GoldStats :one
-- Gold results in the rolling window, and all time (for probation).
SELECT count(*) FILTER (WHERE created_at > now() - make_interval(days => @days::int))::int AS window_total,
       count(*) FILTER (WHERE created_at > now() - make_interval(days => @days::int) AND correct)::int AS window_correct,
       count(*)::int AS all_time
FROM gold_results WHERE contributor_id = @contributor_id;

-- name: AgreementStats :one
SELECT count(*)::int AS total, count(*) FILTER (WHERE agrees_with_final)::int AS agreed
FROM submissions
WHERE contributor_id = @contributor_id AND agrees_with_final IS NOT NULL
  AND submitted_at > now() - make_interval(days => @days::int);

-- name: ReviewStats :one
-- Reviewer decisions on the contributor's work (Stage 11 fills these).
SELECT count(*) FILTER (WHERE status IN ('accepted', 'rejected'))::int AS decided,
       count(*) FILTER (WHERE status = 'accepted')::int AS accepted
FROM submissions
WHERE contributor_id = @contributor_id AND decided_at > now() - make_interval(days => @days::int);

-- name: AddTrustScore :one
INSERT INTO trust_scores (contributor_id, score, gold_accuracy, agreement_rate, review_acceptance, gold_count, window_days, weights)
VALUES (@contributor_id, sqlc.narg(score), sqlc.narg(gold_accuracy), sqlc.narg(agreement_rate), sqlc.narg(review_acceptance),
        @gold_count, @window_days, @weights)
RETURNING *;

-- name: LatestTrustScore :one
SELECT * FROM trust_scores WHERE contributor_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1;

-- Reporting (Stage 10b).

-- name: ProjectsWithConsensusIn :many
SELECT DISTINCT project_id FROM consensus_results WHERE created_at >= @from_at AND created_at < @to_at;

-- name: CompletedItemAnswers :many
-- Answers on regular items whose consensus was reached in the window: each
-- has exactly k answers. Ordered so the caller can group by item.
SELECT c.item_id, r.answer
FROM consensus_results c
JOIN submissions s ON s.item_id = c.item_id AND s.status IN ('submitted', 'accepted', 'rejected')
JOIN submission_revisions r ON r.submission_id = s.id AND r.revision = s.current_revision
WHERE c.project_id = @project_id AND c.created_at >= @from_at AND c.created_at < @to_at
ORDER BY c.item_id, s.submitted_at, s.id;

-- name: AddAgreementSnapshot :exec
INSERT INTO agreement_snapshots (org_id, project_id, label, statistic, value, n_items, n_raters, window_start, window_end)
VALUES (@org_id, @project_id, sqlc.narg(label), @statistic, @value, @n_items, @n_raters, @window_start, @window_end);

-- name: LatestSnapshots :many
-- The newest figure per statistic and label for a project.
SELECT DISTINCT ON (statistic, coalesce(label, '')) label, statistic, value, n_items, n_raters, window_start, window_end, created_at
FROM agreement_snapshots WHERE project_id = $1
ORDER BY statistic, coalesce(label, ''), created_at DESC;

-- name: GoldResultsForDay :many
-- Gold results per contributor and project in one day, for the batched
-- feedback (brief 8.1: never naming the items).
SELECT g.contributor_id, gi.project_id, gi.org_id, p.name AS project_name,
  count(*)::int AS total, count(*) FILTER (WHERE NOT g.correct)::int AS missed
FROM gold_results g
JOIN gold_items gi ON gi.id = g.gold_item_id
JOIN projects p ON p.id = gi.project_id
WHERE g.created_at >= @from_at AND g.created_at < @to_at
GROUP BY g.contributor_id, gi.project_id, gi.org_id, p.name;

-- name: AddFeedback :exec
INSERT INTO feedback (org_id, project_id, contributor_id, submission_id, author_user_id, body)
VALUES (@org_id, @project_id, @contributor_id, sqlc.narg(submission_id), sqlc.narg(author_user_id), @body);

-- name: ListMyFeedback :many
SELECT f.id, f.body, f.read_at, f.created_at, f.project_id, p.name AS project_name, (f.author_user_id IS NULL)::bool AS automatic
FROM feedback f JOIN projects p ON p.id = f.project_id
WHERE f.contributor_id = $1
ORDER BY f.created_at DESC
LIMIT 50;

-- name: MarkFeedbackRead :execrows
UPDATE feedback SET read_at = now() WHERE id = @id AND contributor_id = @contributor_id AND read_at IS NULL;

-- name: CountMyWorkInProject :one
-- Tasks this contributor has submitted on the project (warm-up).
SELECT count(*)::int FROM assignments WHERE contributor_id = @contributor_id AND project_id = @project_id AND status = 'submitted';

-- name: ProjectQualitySummary :one
SELECT
  (SELECT count(*) FROM items i WHERE i.project_id = @project_id AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id))::int AS items,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id)::int AS completed,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.routed_to_review)::int AS routed,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.route_reason = 'no_majority')::int AS no_majority,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.route_reason = 'spread')::int AS spread,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.route_reason = 'flagged')::int AS flagged,
  (SELECT count(*) FROM consensus_results c WHERE c.project_id = @project_id AND c.route_reason = 'audit_sample')::int AS audit_sample,
  (SELECT count(*) FROM gold_results g JOIN gold_items gi ON gi.id = g.gold_item_id
     WHERE gi.project_id = @project_id AND g.created_at > now() - interval '30 days')::int AS gold_total,
  (SELECT count(*) FROM gold_results g JOIN gold_items gi ON gi.id = g.gold_item_id
     WHERE gi.project_id = @project_id AND g.created_at > now() - interval '30 days' AND g.correct)::int AS gold_correct,
  (SELECT count(DISTINCT s.contributor_id) FROM submissions s WHERE s.project_id = @project_id AND s.status <> 'draft')::int AS contributors;

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

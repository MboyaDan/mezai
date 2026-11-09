-- Qualification tests and attempts (brief 6.2).

-- name: ListQualificationTests :many
SELECT t.*, (SELECT count(*) FROM qualification_questions q WHERE q.test_id = t.id)::int AS question_count
FROM qualification_tests t
WHERE sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text
ORDER BY t.created_at DESC, t.id DESC;

-- name: GetQualificationTest :one
SELECT t.*, (SELECT count(*) FROM qualification_questions q WHERE q.test_id = t.id)::int AS question_count
FROM qualification_tests t WHERE t.id = $1;

-- name: SetQualificationTestStatus :exec
UPDATE qualification_tests SET status = @status WHERE id = @id;

-- name: ListQualificationQuestions :many
SELECT * FROM qualification_questions WHERE test_id = $1 ORDER BY position;

-- name: ListGoldForQualification :many
-- Active gold of one project with its item content, in random order.
SELECT g.id, g.expected, g.tolerance, i.content
FROM gold_items g JOIN items i ON i.id = g.item_id
WHERE g.project_id = @project_id AND g.status = 'active'
ORDER BY random()
LIMIT @max_count;

-- name: RetireGoldItem :exec
UPDATE gold_items SET status = 'retired', retired_at = now() WHERE id = $1 AND status = 'active';

-- name: ListAttemptsByContributor :many
SELECT * FROM qualification_attempts WHERE contributor_id = $1 ORDER BY started_at DESC, id DESC;

-- name: CreateQualificationAttempt :one
INSERT INTO qualification_attempts (test_id, contributor_id) VALUES ($1, $2) RETURNING *;

-- name: GetOpenAttempt :one
SELECT * FROM qualification_attempts
WHERE contributor_id = $1 AND test_id = $2 AND status IN ('in_progress', 'scored');

-- name: GetQualificationAttempt :one
SELECT * FROM qualification_attempts WHERE id = $1;

-- name: LockQualificationAttempt :one
SELECT * FROM qualification_attempts WHERE id = $1 FOR UPDATE;

-- name: SaveAttemptAnswers :exec
UPDATE qualification_attempts SET answers = @answers WHERE id = @id AND status = 'in_progress';

-- name: ScoreAttempt :exec
UPDATE qualification_attempts
SET answers = @answers, results = @results, score = @score, passed = @passed, status = 'scored', submitted_at = now()
WHERE id = @id AND status = 'in_progress';

-- name: DecideAttempt :execrows
UPDATE qualification_attempts
SET status = @status, decided_by = @decided_by, decided_at = now(), decision_note = @decision_note,
  writing_rating = sqlc.narg(writing_rating)
WHERE id = @id AND status = 'scored';

-- name: ListAttemptsForReview :many
-- Ops queue and history. Oldest submissions first.
SELECT a.id, a.status, a.score, a.passed, a.submitted_at, a.decided_at, a.decision_note,
  t.id AS test_id, t.title AS test_title, t.pass_threshold, t.skill_kind, t.skill_value,
  cp.id AS contributor_id, cp.public_code, cp.status AS contributor_status, u.display_name
FROM qualification_attempts a
JOIN qualification_tests t ON t.id = a.test_id
JOIN contributor_profiles cp ON cp.id = a.contributor_id
JOIN users u ON u.id = cp.user_id
WHERE a.status = @status
ORDER BY a.submitted_at, a.id
LIMIT 200;

-- Core check (Stage 7d-2, D-100 section 3.3).

-- name: AddQualificationQuestionTyped :exec
INSERT INTO qualification_questions (test_id, position, content, expected, tolerance, task_type, config)
VALUES (@test_id, @position, @content, @expected, '{}', sqlc.narg(task_type), sqlc.narg(config));

-- name: CountCoreTests :one
SELECT count(*)::int FROM qualification_tests WHERE skill_kind = 'core' AND status <> 'retired';

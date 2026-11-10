-- Postgres-backed job queue (CLAUDE.md: no Redis; FOR UPDATE SKIP LOCKED).

-- name: EnqueueJob :exec
-- dedupe_key makes daily jobs run once per date, however many instances call this.
INSERT INTO jobs (kind, payload, run_at, dedupe_key, max_attempts)
VALUES (@kind, @payload, @run_at, @dedupe_key, @max_attempts)
ON CONFLICT (dedupe_key) DO NOTHING;

-- name: ClaimJob :one
-- The oldest due job no other worker holds. A job stuck in "running" for
-- more than 15 minutes (a crashed worker) is claimable again.
UPDATE jobs SET status = 'running', locked_at = now(), locked_by = @worker, attempts = attempts + 1
WHERE id = (
  SELECT id FROM jobs
  WHERE run_at <= now() AND attempts < max_attempts
    AND (status = 'queued' OR (status = 'running' AND locked_at < now() - interval '15 minutes'))
  ORDER BY run_at, id
  LIMIT 1
  FOR UPDATE SKIP LOCKED)
RETURNING *;

-- name: FinishJob :exec
UPDATE jobs SET status = 'done', last_error = NULL WHERE id = $1;

-- name: FailJob :exec
-- Back off 2^attempts minutes; give up after max_attempts.
UPDATE jobs SET status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'queued' END,
  last_error = @last_error, run_at = now() + make_interval(mins => power(2, attempts)::int)
WHERE id = @id;

-- name: GetJobByKey :one
SELECT * FROM jobs WHERE dedupe_key = $1;

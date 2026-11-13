-- A contributor's own earnings, statement and payouts (Stage 12a, brief 13).
-- Every figure here is a sum of ledger rows or of the contributor's own
-- submissions; nothing is stored twice (D-119).

-- name: MyLedgerTotals :many
-- earned: accruals and adjustments. paid: payouts net of reversals (positive).
-- balance: every entry, so balance = earned - paid always holds.
SELECT currency::text AS currency,
  coalesce(sum(amount_cents) FILTER (WHERE entry_type IN ('accrual', 'adjustment')), 0)::bigint AS earned_cents,
  coalesce(-sum(amount_cents) FILTER (WHERE entry_type IN ('payout', 'payout_reversal')), 0)::bigint AS paid_cents,
  sum(amount_cents)::bigint AS balance_cents
FROM earnings_ledger WHERE contributor_id = $1
GROUP BY currency ORDER BY currency;

-- name: MyPendingEarnings :many
-- Submitted answers not yet decided, priced at the project's current rate.
-- An estimate: the ledger records the rate when the answer is accepted.
SELECT p.currency::text AS currency, count(*)::int AS tasks, coalesce(sum(p.pay_per_task_cents), 0)::bigint AS cents
FROM submissions s JOIN projects p ON p.id = s.project_id
WHERE s.contributor_id = $1 AND s.status = 'submitted'
GROUP BY p.currency ORDER BY p.currency;

-- name: MyTaskCounts :one
SELECT count(*) FILTER (WHERE status <> 'draft')::int AS submitted,
  count(*) FILTER (WHERE status = 'submitted')::int AS waiting,
  count(*) FILTER (WHERE status = 'accepted')::int AS accepted,
  count(*) FILTER (WHERE status = 'rejected')::int AS not_accepted
FROM submissions WHERE contributor_id = $1;

-- name: MyStatement :many
-- The statement for the 30 most recent Nairobi days with entries before
-- @before_day: accepted work summed per day and project, every other entry on
-- its own line. Accruals are never listed per item, so the statement cannot
-- be matched to individual (possibly gold) tasks.
WITH entries AS (
  SELECT l.id, l.entry_type, l.amount_cents, l.currency, l.project_id, l.reason, l.payout_line_id, l.created_at,
    (l.created_at AT TIME ZONE 'Africa/Nairobi')::date AS day
  FROM earnings_ledger l
  WHERE l.contributor_id = @contributor_id
    AND (l.created_at AT TIME ZONE 'Africa/Nairobi')::date < @before_day::date
),
days AS (
  SELECT DISTINCT day FROM entries ORDER BY day DESC LIMIT 30
)
SELECT e.day::date AS day, e.entry_type::text AS entry_type, e.currency::text AS currency, e.project_id,
  p.name AS project_name, count(*)::int AS entries, sum(e.amount_cents)::bigint AS amount_cents,
  coalesce(max(e.reason), '')::text AS reason, coalesce(max(pl.mpesa_receipt), '')::text AS mpesa_receipt, max(e.created_at)::timestamptz AS last_at
FROM entries e
JOIN days d ON d.day = e.day
LEFT JOIN projects p ON p.id = e.project_id
LEFT JOIN payout_lines pl ON pl.id = e.payout_line_id
GROUP BY e.day, e.entry_type, e.currency, e.project_id, p.name,
  CASE WHEN e.entry_type = 'accrual' THEN NULL ELSE e.id END
ORDER BY e.day DESC, last_at DESC, e.entry_type, p.name;

-- name: MyStatementDayCount :one
-- Days with entries before @before_day, so the caller knows whether an older page exists.
SELECT count(DISTINCT (created_at AT TIME ZONE 'Africa/Nairobi')::date)::int
FROM earnings_ledger
WHERE contributor_id = @contributor_id AND (created_at AT TIME ZONE 'Africa/Nairobi')::date < @before_day::date;

-- name: MyPayouts :many
-- Lines in batches that have left draft. Ops notes on failures are shown;
-- the phone number is masked by the caller.
SELECT pl.id, pl.amount_cents, pl.currency::text AS currency, pl.status, pl.mpesa_receipt, pl.failure_reason,
  pl.mpesa_phone, pl.recorded_at, pl.created_at, b.period_start, b.period_end, b.status AS batch_status
FROM payout_lines pl JOIN payout_batches b ON b.id = pl.batch_id
WHERE pl.contributor_id = $1 AND b.status <> 'draft'
ORDER BY pl.created_at DESC, pl.id DESC
LIMIT 100;

-- Quality as the contributor sees it (D-120): fixed at the start of the
-- current Nairobi day, so a score never moves straight after a task.

-- name: TrustScoreBefore :one
SELECT * FROM trust_scores WHERE contributor_id = @contributor_id AND created_at < @before::timestamptz
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: TrustTrend :many
-- The last score of each Nairobi day in [from, before).
SELECT DISTINCT ON (day) day::date AS day, score, gold_accuracy, agreement_rate, review_acceptance
FROM (SELECT (created_at AT TIME ZONE 'Africa/Nairobi')::date AS day, created_at, id,
        score, gold_accuracy, agreement_rate, review_acceptance
      FROM trust_scores
      WHERE contributor_id = @contributor_id AND created_at >= @from_at::timestamptz AND created_at < @before::timestamptz) t
ORDER BY day, created_at DESC, id DESC;

-- name: GoldCountBefore :one
SELECT count(*)::int FROM gold_results WHERE contributor_id = @contributor_id AND created_at < @before::timestamptz;

-- Work on offer (Stage 12a): sizes only, never which items are gold.

-- name: OpenItemCounts :many
-- Regular items the contributor could still be given on each project:
-- short of k answers and not yet seen by them. Live reservations by others
-- are not subtracted, so this is "about".
SELECT i.project_id, count(*)::int AS open
FROM items i JOIN projects p ON p.id = i.project_id
WHERE i.project_id = ANY(@ids::uuid[]) AND i.status IN ('pending', 'in_progress')
  AND i.submission_count < p.overlap_k
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
  AND NOT EXISTS (SELECT 1 FROM assignments a WHERE a.item_id = i.id AND a.contributor_id = @contributor_id)
GROUP BY i.project_id;

-- name: TypicalTaskTimes :many
-- Median time on task over the last 30 days, with the number of answers it is based on.
SELECT project_id, percentile_cont(0.5) WITHIN GROUP (ORDER BY time_on_task_ms)::float8 AS median_ms, count(*)::int AS answers
FROM submissions
WHERE project_id = ANY(@ids::uuid[]) AND status <> 'draft' AND submitted_at > now() - interval '30 days'
GROUP BY project_id;

-- name: GuidelinesForProjects :many
SELECT project_id, title, body_md FROM project_guidelines
WHERE project_id = ANY(@ids::uuid[])
ORDER BY project_id, position, created_at;

-- name: SeedLedgerAdjustment :exec
-- Sample balances for the development seed only (backend/seed), dated in
-- the past so a payout batch can include them.
INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, reason, created_by, created_at)
VALUES (@contributor_id, 'adjustment', @amount_cents, 'KES', @reason, @created_by, @created_at);

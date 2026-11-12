-- Payouts, manual M-Pesa path (Stage 14a, brief 13, D-127). Staff only,
-- behind payout.create and payout.approve.

-- name: PayoutCandidates :many
-- Every contributor owed money: the balance now and at the cutoff. The
-- batch pays the smaller of the two, so later corrections are respected
-- and later earnings wait for the next batch.
SELECT cp.id, cp.public_code, cp.status, u.display_name, cp.mpesa_phone, cp.mpesa_phone_confirmed_at,
  cp.identity_status, cp.mpesa_name_checked_at,
  b.balance_cents, b.cutoff_cents
FROM (SELECT l.contributor_id,
        sum(l.amount_cents)::bigint AS balance_cents,
        coalesce(sum(l.amount_cents) FILTER (WHERE l.created_at < @cutoff::timestamptz), 0)::bigint AS cutoff_cents
      FROM earnings_ledger l WHERE l.currency = 'KES' GROUP BY l.contributor_id) b
JOIN contributor_profiles cp ON cp.id = b.contributor_id
JOIN users u ON u.id = cp.user_id
WHERE b.balance_cents > 0 AND b.cutoff_cents > 0
ORDER BY cp.public_code;

-- name: LastPayoutPeriodEnd :many
SELECT period_end FROM payout_batches WHERE status <> 'cancelled' ORDER BY period_end DESC LIMIT 1;

-- name: FirstLedgerEntry :many
SELECT created_at FROM earnings_ledger ORDER BY created_at LIMIT 1;

-- name: OpenPayoutBatch :many
SELECT id FROM payout_batches WHERE status NOT IN ('settled', 'cancelled');

-- name: GetPayoutBatchByKey :many
SELECT id FROM payout_batches WHERE idempotency_key = $1;

-- name: InsertPayoutBatch :one
INSERT INTO payout_batches (period_start, period_end, cutoff_at, currency, idempotency_key, created_by)
VALUES (@period_start, @period_end, @cutoff_at, 'KES', @idempotency_key, @created_by)
RETURNING id;

-- name: InsertPayoutLine :exec
INSERT INTO payout_lines (batch_id, contributor_id, amount_cents, currency, mpesa_phone, payee_name)
VALUES (@batch_id, @contributor_id, @amount_cents, 'KES', @mpesa_phone, @payee_name);

-- name: SetPayoutBatchTotals :exec
UPDATE payout_batches SET total_cents = t.total, line_count = t.n
FROM (SELECT coalesce(sum(amount_cents), 0)::bigint AS total, count(*)::int AS n FROM payout_lines WHERE batch_id = @id) t
WHERE payout_batches.id = @id;

-- name: ListPayoutBatches :many
SELECT b.id, b.period_start, b.period_end, b.currency, b.status, b.total_cents, b.line_count, b.created_at,
  cu.display_name AS created_by_name, au.display_name AS approved_by_name,
  (SELECT count(*) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'paid')::int AS paid_lines,
  (SELECT count(*) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'failed')::int AS failed_lines,
  coalesce((SELECT sum(l.amount_cents) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'paid'), 0)::bigint AS paid_cents
FROM payout_batches b
JOIN users cu ON cu.id = b.created_by
LEFT JOIN users au ON au.id = b.approved_by
ORDER BY b.created_at DESC
LIMIT 100;

-- name: GetPayoutBatch :one
SELECT b.*, cu.display_name AS created_by_name, au.display_name AS approved_by_name,
  xu.display_name AS cancelled_by_name
FROM payout_batches b
JOIN users cu ON cu.id = b.created_by
LEFT JOIN users au ON au.id = b.approved_by
LEFT JOIN users xu ON xu.id = b.cancelled_by
WHERE b.id = $1;

-- name: LockPayoutBatch :one
SELECT id, status, created_by, total_cents, line_count, cutoff_at, period_start, period_end, export_file_key
FROM payout_batches WHERE id = $1 FOR UPDATE;

-- name: ListPayoutLines :many
-- With each contributor's balance now, so the approver sees a line that is
-- no longer covered.
SELECT l.id, l.contributor_id, l.amount_cents, l.currency, l.mpesa_phone, l.payee_name, l.status,
  l.mpesa_receipt, l.failure_reason, l.recorded_at, ru.display_name AS recorded_by_name,
  cp.public_code, cp.status AS contributor_status,
  coalesce((SELECT sum(e.amount_cents) FROM earnings_ledger e
            WHERE e.contributor_id = l.contributor_id AND e.currency = l.currency), 0)::bigint AS balance_cents
FROM payout_lines l
JOIN contributor_profiles cp ON cp.id = l.contributor_id
LEFT JOIN users ru ON ru.id = l.recorded_by
WHERE l.batch_id = $1
ORDER BY cp.public_code;

-- name: SubmitPayoutBatch :execrows
UPDATE payout_batches SET status = 'pending_approval', submitted_at = now(), returned_note = NULL
WHERE id = @id AND status = 'draft';

-- name: ApprovePayoutBatch :execrows
-- The table's check also refuses approved_by = created_by.
UPDATE payout_batches SET status = 'approved', approved_by = @approved_by, approved_at = now()
WHERE id = @id AND status = 'pending_approval' AND created_by <> @approved_by;

-- name: ReturnPayoutBatch :execrows
UPDATE payout_batches SET status = 'draft', returned_note = @note, submitted_at = NULL
WHERE id = @id AND status = 'pending_approval';

-- name: CancelPayoutBatch :execrows
UPDATE payout_batches SET status = 'cancelled', cancelled_by = @cancelled_by, cancelled_at = now(), cancel_reason = @reason
WHERE id = @id AND status IN ('draft', 'pending_approval');

-- name: ExportPayoutBatch :execrows
UPDATE payout_batches SET status = 'exported', exported_at = now(), exported_by = @exported_by,
  export_file_key = @file_key, export_file_name = @file_name, export_checksum = @checksum, export_format = @format
WHERE id = @id AND status = 'approved';

-- name: SettlePayoutBatch :execrows
-- Settled once every line has a result.
UPDATE payout_batches b SET status = 'settled', settled_at = now()
WHERE b.id = @id AND b.status = 'exported'
  AND NOT EXISTS (SELECT 1 FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'pending');

-- name: LockPayoutLine :one
SELECT l.id, l.batch_id, l.contributor_id, l.amount_cents, l.currency, l.status, l.mpesa_receipt, l.failure_reason,
  b.status AS batch_status
FROM payout_lines l JOIN payout_batches b ON b.id = l.batch_id
WHERE l.id = $1 FOR UPDATE OF l;

-- name: RecordPayoutLinePaid :execrows
UPDATE payout_lines SET status = 'paid', mpesa_receipt = @receipt, recorded_by = @recorded_by, recorded_at = now()
WHERE id = @id AND status = 'pending';

-- name: RecordPayoutLineFailed :execrows
UPDATE payout_lines SET status = 'failed', failure_reason = @reason, recorded_by = @recorded_by, recorded_at = now()
WHERE id = @id AND status = 'pending';

-- name: InsertPayoutLedgerEntry :exec
-- The money leaves the balance only now (D-127). Unique per line.
INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, payout_line_id, created_by)
VALUES (@contributor_id, 'payout', 0 - @amount_cents::bigint, @currency, @payout_line_id, @created_by);

-- name: SetSettingValue :exec
UPDATE settings SET value = @value WHERE key = @key;

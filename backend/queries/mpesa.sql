-- M-Pesa B2C (Stage 15a, D-132). Staff and system scope only.

-- name: DisbursePayoutBatch :execrows
UPDATE payout_batches SET status = 'disbursing', disbursed_at = now(), disbursed_by = @disbursed_by
WHERE id = @id AND status = 'approved';

-- name: PendingPayoutLines :many
SELECT id, amount_cents, currency, mpesa_phone FROM payout_lines
WHERE batch_id = $1 AND status = 'pending' ORDER BY id;

-- name: InsertMpesaTransaction :one
INSERT INTO mpesa_transactions (payout_line_id, originator_conversation_id, environment, amount_cents, currency, created_by)
VALUES (@payout_line_id, @originator_conversation_id, @environment, @amount_cents, @currency, @created_by)
RETURNING id;

-- name: LockMpesaTransaction :one
SELECT t.*, l.mpesa_phone, l.batch_id, l.contributor_id
FROM mpesa_transactions t JOIN payout_lines l ON l.id = t.payout_line_id
WHERE t.id = $1 FOR UPDATE OF t;

-- name: MpesaTransactionByOriginator :one
SELECT id, conversation_id, amount_cents, status FROM mpesa_transactions WHERE originator_conversation_id = $1;

-- name: MarkMpesaSending :exec
UPDATE mpesa_transactions SET status = 'sending', sent_at = now(), send_attempts = send_attempts + 1
WHERE id = $1 AND status = 'queued';

-- name: MarkMpesaAccepted :exec
UPDATE mpesa_transactions SET status = 'accepted', conversation_id = @conversation_id,
  response_code = @response_code, response_description = @response_description
WHERE id = @id AND status = 'sending';

-- name: MarkMpesaRequeued :exec
-- Safaricom's gateway refused the request before accepting it (401, 429).
UPDATE mpesa_transactions SET status = 'queued', response_code = @response_code, response_description = @response_description
WHERE id = @id AND status = 'sending';

-- name: MarkMpesaUnknown :exec
UPDATE mpesa_transactions SET status = 'unknown',
  response_code = coalesce(sqlc.narg(response_code), response_code),
  response_description = coalesce(sqlc.narg(response_description), response_description)
WHERE id = @id AND status IN ('sending', 'accepted');

-- name: MarkMpesaSucceeded :exec
UPDATE mpesa_transactions SET status = 'succeeded', receipt = @receipt, result_code = @result_code,
  result_desc = @result_desc, completed_at = now()
WHERE id = @id AND status NOT IN ('succeeded', 'failed');

-- name: MarkMpesaFailed :exec
UPDATE mpesa_transactions SET status = 'failed', result_code = @result_code, result_desc = @result_desc, completed_at = now()
WHERE id = @id AND status NOT IN ('succeeded', 'failed');

-- name: CountMpesaStatusQuery :one
UPDATE mpesa_transactions SET status_queries = status_queries + 1 WHERE id = $1 RETURNING status_queries;

-- name: CountMpesaNotFound :one
UPDATE mpesa_transactions SET not_found_count = not_found_count + 1 WHERE id = $1 RETURNING not_found_count;

-- name: RecordPayoutLinePaidB2C :execrows
UPDATE payout_lines SET status = 'paid', mpesa_receipt = @receipt, recorded_via = 'mpesa_b2c', recorded_at = now()
WHERE id = @id AND status = 'pending';

-- name: RecordPayoutLineFailedB2C :execrows
UPDATE payout_lines SET status = 'failed', failure_reason = @reason, recorded_via = 'mpesa_b2c', recorded_at = now()
WHERE id = @id AND status = 'pending';

-- name: SettleDisbursedBatch :execrows
UPDATE payout_batches b SET status = 'settled', settled_at = now()
WHERE b.id = @id AND b.status = 'disbursing'
  AND NOT EXISTS (SELECT 1 FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'pending');

-- name: InsertMpesaCallback :one
INSERT INTO mpesa_callbacks (kind, originator_conversation_id, conversation_id, transaction_id, result_code,
  body, remote_ip, verdict, note)
VALUES (@kind, @originator_conversation_id, @conversation_id, @transaction_id, @result_code,
  @body, @remote_ip, @verdict, @note)
RETURNING id;

-- name: LockMpesaCallback :one
SELECT * FROM mpesa_callbacks WHERE id = $1 FOR UPDATE;

-- name: FinishMpesaCallback :exec
UPDATE mpesa_callbacks SET processed_at = now(), process_error = NULL WHERE id = $1;

-- name: FailMpesaCallback :exec
UPDATE mpesa_callbacks SET process_error = @process_error WHERE id = @id;

-- name: FlagMpesaCallback :exec
UPDATE mpesa_callbacks SET verdict = 'mismatch', note = @note, processed_at = now() WHERE id = @id;

-- name: UnprocessedMpesaCallbacks :many
-- Matched callbacks that were stored but not processed (the inline attempt
-- failed). Older than a minute, so an inline attempt still running is left alone.
SELECT id FROM mpesa_callbacks
WHERE processed_at IS NULL AND verdict = 'matched' AND created_at < now() - interval '1 minute'
ORDER BY created_at LIMIT 100;

-- name: ListBatchMpesaTransactions :many
SELECT t.id, t.payout_line_id, t.status, t.amount_cents, t.currency, t.environment, t.sent_at, t.completed_at,
  t.response_code, t.response_description, t.result_code, t.result_desc, t.receipt, t.status_queries,
  t.originator_conversation_id, cp.public_code
FROM mpesa_transactions t
JOIN payout_lines l ON l.id = t.payout_line_id
JOIN contributor_profiles cp ON cp.id = l.contributor_id
WHERE l.batch_id = $1
ORDER BY cp.public_code;

-- name: MpesaBatchReconciliation :many
-- Per disbursed batch: the lines, the transactions and the ledger must agree.
SELECT b.id, b.period_start, b.period_end, b.status, b.total_cents, b.line_count, b.disbursed_at, b.settled_at,
  (SELECT count(*) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'paid')::int AS paid_lines,
  (SELECT count(*) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'failed')::int AS failed_lines,
  (SELECT count(*) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'pending')::int AS pending_lines,
  coalesce((SELECT sum(l.amount_cents) FROM payout_lines l WHERE l.batch_id = b.id AND l.status = 'paid'), 0)::bigint AS paid_cents,
  coalesce((SELECT sum(t.amount_cents) FROM mpesa_transactions t JOIN payout_lines l ON l.id = t.payout_line_id
            WHERE l.batch_id = b.id AND t.status = 'succeeded'), 0)::bigint AS succeeded_cents,
  coalesce((SELECT -sum(e.amount_cents) FROM earnings_ledger e JOIN payout_lines l ON l.id = e.payout_line_id
            WHERE l.batch_id = b.id AND e.entry_type = 'payout'), 0)::bigint AS ledger_cents,
  (SELECT count(*) FROM mpesa_transactions t JOIN payout_lines l ON l.id = t.payout_line_id
   WHERE l.batch_id = b.id AND t.status NOT IN ('succeeded', 'failed'))::int AS open_transactions
FROM payout_batches b
WHERE b.disbursed_at IS NOT NULL
ORDER BY b.disbursed_at DESC
LIMIT 50;

-- name: MpesaStuckTransactions :many
-- Open transactions that have waited too long: queued (never accepted) for
-- an hour, or without a result for a day.
SELECT t.id, t.status, t.amount_cents, t.sent_at, t.updated_at, t.status_queries, l.batch_id, cp.public_code
FROM mpesa_transactions t
JOIN payout_lines l ON l.id = t.payout_line_id
JOIN contributor_profiles cp ON cp.id = l.contributor_id
WHERE (t.status = 'queued' AND t.updated_at < now() - interval '1 hour')
   OR (t.status IN ('sending', 'accepted', 'unknown') AND coalesce(t.sent_at, t.created_at) < now() - interval '24 hours')
ORDER BY t.created_at;

-- name: MpesaLateResults :many
-- A transaction that failed (or was given up as not found) but a later
-- callback says it was paid: money left without a payout entry.
SELECT c.id AS callback_id, c.created_at, t.id AS transaction_id, t.amount_cents, l.batch_id, cp.public_code
FROM mpesa_callbacks c
JOIN mpesa_transactions t ON t.id = c.transaction_id
JOIN payout_lines l ON l.id = t.payout_line_id
JOIN contributor_profiles cp ON cp.id = l.contributor_id
WHERE c.verdict = 'mismatch' AND c.note LIKE 'late_success%'
ORDER BY c.created_at DESC LIMIT 100;

-- name: MpesaCallbackExceptions :many
SELECT id, kind, originator_conversation_id, transaction_id, verdict, note, created_at
FROM mpesa_callbacks WHERE verdict <> 'matched' AND created_at > now() - interval '90 days'
ORDER BY created_at DESC LIMIT 100;

-- name: MpesaTransactionIDByLine :one
SELECT id FROM mpesa_transactions WHERE payout_line_id = $1;

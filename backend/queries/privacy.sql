-- Privacy (Stage 17b): retention of task content, storage deletions,
-- contributor data export and erasure.

-- name: ProjectsDueForPurge :many
-- Closed projects whose retention period has run out.
SELECT id FROM projects
WHERE status = 'closed' AND content_purged_at IS NULL
  AND closed_at + make_interval(days => retention_days) <= now()
ORDER BY closed_at
LIMIT 20;

-- name: PurgeProjectContent :one
SELECT purge_project_content(@project_id::uuid)::jsonb AS counts;

-- name: GetProjectRetention :one
SELECT id, org_id, name, status, retention_days, closed_at, content_purged_at FROM projects WHERE id = $1;

-- name: QueueDeclinedRequestSamples :execrows
-- A declined request's sample file goes after the same number of days as
-- closed projects' content (retention.days_after_close).
WITH due AS (
  SELECT id, sample_file_key AS k FROM project_requests
  WHERE status = 'declined' AND sample_file_key IS NOT NULL
    AND decided_at + make_interval(days => @days::int) <= now()
  FOR UPDATE
), queued AS (
  INSERT INTO storage_deletions (key, reason)
  SELECT k, 'request_declined' FROM due
  ON CONFLICT (key) DO NOTHING
)
UPDATE project_requests r SET sample_file_key = NULL, sample_file_name = NULL, sample_size = NULL
FROM due WHERE r.id = due.id;

-- name: ClaimStorageDeletions :many
SELECT key FROM storage_deletions WHERE attempts < 20 ORDER BY created_at LIMIT 100 FOR UPDATE SKIP LOCKED;

-- name: DoneStorageDeletion :exec
DELETE FROM storage_deletions WHERE key = $1;

-- name: FailStorageDeletion :exec
UPDATE storage_deletions SET attempts = attempts + 1, last_error = left(@error::text, 500) WHERE key = @key;

-- name: RedactSentEmails :execrows
-- Merge data (names, links) of emails that were sent or gave up.
UPDATE email_outbox SET data = '{}'
WHERE status IN ('sent', 'failed') AND data <> '{}'::jsonb AND updated_at < now() - make_interval(days => @days::int);

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys WHERE expires_at < now() - interval '1 day';

-- name: CreateErasureRequest :one
INSERT INTO erasure_requests (contributor_id, requested_by, note) VALUES ($1, $2, $3)
RETURNING id, contributor_id, status, note, created_at;

-- name: GetOpenErasureRequest :one
SELECT id, contributor_id, status, note, requested_by, created_at FROM erasure_requests
WHERE contributor_id = $1 AND status = 'open';

-- name: WithdrawErasureRequest :execrows
UPDATE erasure_requests SET status = 'withdrawn', closed_by = @user_id, closed_at = now()
WHERE contributor_id = @contributor_id AND status = 'open';

-- name: ListOpenErasureRequests :many
SELECT e.id, e.contributor_id, cp.public_code, e.created_at, (e.requested_by = cp.user_id)::boolean AS by_contributor
FROM erasure_requests e JOIN contributor_profiles cp ON cp.id = e.contributor_id
WHERE e.status = 'open' ORDER BY e.created_at;

-- name: ErasureBlockers :one
-- What stops erasure now: money owed either way, a payout line in flight,
-- or answers still waiting for a decision (they may still earn, A-088).
SELECT
  (SELECT COALESCE(sum(amount_cents), 0) FROM earnings_ledger l WHERE l.contributor_id = @contributor_id)::bigint AS balance_cents,
  (SELECT count(*) FROM payout_lines pl WHERE pl.contributor_id = @contributor_id AND pl.status = 'pending')::int AS pending_payout_lines,
  (SELECT count(*) FROM submissions s WHERE s.contributor_id = @contributor_id AND s.status = 'submitted')::int AS undecided_answers,
  cp.public_code, cp.user_id, cp.status, cp.erased_at
FROM contributor_profiles cp WHERE cp.id = @contributor_id;

-- name: EraseContributor :exec
SELECT erase_contributor(@contributor_id::uuid, @actor::uuid);

-- name: ContributorDataExport :one
-- Everything held about one contributor, as one JSON document (DPA 2019,
-- right of access). Task content, other people's answers and gold answers
-- are not personal data of the contributor and are left out.
SELECT jsonb_build_object(
  'account', (SELECT jsonb_build_object('email', u.email, 'name', u.display_name, 'created_at', u.created_at,
                'email_verified_at', u.email_verified_at, 'last_sign_in_at', u.last_login_at)
              FROM users u WHERE u.id = cp.user_id),
  'profile', jsonb_build_object('public_code', cp.public_code, 'status', cp.status, 'country', cp.country, 'city', cp.city,
                'education_level', cp.education_level, 'weekly_hours', cp.weekly_hours, 'mpesa_phone', cp.mpesa_phone,
                'mpesa_phone_confirmed_at', cp.mpesa_phone_confirmed_at, 'national_id_last3', cp.national_id_last3,
                'identity_status', cp.identity_status, 'identity_note', cp.identity_note, 'experience', cp.experience,
                'linkedin_url', cp.linkedin_url, 'cv_file_name', cp.cv_file_name, 'cv_uploaded_at', cp.cv_uploaded_at,
                'cv_delete_after', cp.cv_delete_after, 'screening_status', cp.screening_status,
                'screening_note', cp.screening_note, 'created_at', cp.created_at),
  'languages', COALESCE((SELECT jsonb_agg(jsonb_build_object('language', l.language, 'proficiency', l.proficiency, 'dialects', l.dialects) ORDER BY l.language)
                FROM contributor_languages l WHERE l.contributor_id = cp.id), '[]'),
  'domains', COALESCE((SELECT jsonb_agg(d.domain ORDER BY d.domain) FROM contributor_domains d WHERE d.contributor_id = cp.id), '[]'),
  'consents', COALESCE((SELECT jsonb_agg(jsonb_build_object('document', c.document, 'version', c.version, 'accepted_at', c.accepted_at, 'ip', c.ip) ORDER BY c.accepted_at)
                FROM consents c WHERE c.user_id = cp.user_id), '[]'),
  'status_history', COALESCE((SELECT jsonb_agg(jsonb_build_object('from', h.from_status, 'to', h.to_status, 'reason', h.reason, 'at', h.created_at) ORDER BY h.created_at)
                FROM contributor_status_history h WHERE h.contributor_id = cp.id), '[]'),
  'qualification_attempts', COALESCE((SELECT jsonb_agg(jsonb_build_object('test', t.title, 'status', a.status, 'score', a.score, 'passed', a.passed,
                'writing_rating', a.writing_rating, 'started_at', a.started_at, 'submitted_at', a.submitted_at, 'decided_at', a.decided_at,
                'decision_note', a.decision_note) ORDER BY a.started_at)
                FROM qualification_attempts a JOIN qualification_tests t ON t.id = a.test_id WHERE a.contributor_id = cp.id), '[]'),
  'interviews', COALESCE((SELECT jsonb_agg(jsonb_build_object('outcome', i.outcome, 'identity_confirmed', i.identity_confirmed,
                'language_scores', i.language_scores, 'scenario_score', i.scenario_score, 'confidentiality_ok', i.confidentiality_ok,
                'availability', i.availability, 'note', i.note, 'at', i.created_at) ORDER BY i.created_at)
                FROM interviews i WHERE i.contributor_id = cp.id), '[]'),
  'answers', COALESCE((SELECT jsonb_agg(jsonb_build_object('project', p.name, 'status', s.status, 'submitted_at', s.submitted_at,
                'decided_at', s.decided_at, 'time_on_task_ms', s.time_on_task_ms, 'rejection_reason', s.rejection_reason) ORDER BY s.created_at)
                FROM submissions s JOIN projects p ON p.id = s.project_id WHERE s.contributor_id = cp.id AND s.status <> 'draft'), '[]'),
  'skips', COALESCE((SELECT jsonb_agg(jsonb_build_object('reason', k.reason, 'note', k.note, 'at', k.created_at) ORDER BY k.created_at)
                FROM skips k WHERE k.contributor_id = cp.id), '[]'),
  'content_reports', COALESCE((SELECT jsonb_agg(jsonb_build_object('category', r.category, 'note', r.note, 'status', r.status, 'at', r.created_at) ORDER BY r.created_at)
                FROM content_reports r WHERE r.contributor_id = cp.id), '[]'),
  'feedback', COALESCE((SELECT jsonb_agg(jsonb_build_object('project', p.name, 'body', f.body, 'at', f.created_at, 'read_at', f.read_at) ORDER BY f.created_at)
                FROM feedback f JOIN projects p ON p.id = f.project_id WHERE f.contributor_id = cp.id), '[]'),
  'trust_scores', COALESCE((SELECT jsonb_agg(jsonb_build_object('score', ts.score, 'gold_accuracy', ts.gold_accuracy, 'agreement_rate', ts.agreement_rate,
                'review_acceptance', ts.review_acceptance, 'gold_count', ts.gold_count, 'at', ts.created_at) ORDER BY ts.created_at)
                FROM trust_scores ts WHERE ts.contributor_id = cp.id), '[]'),
  'earnings', COALESCE((SELECT jsonb_agg(jsonb_build_object('type', e.entry_type, 'amount_cents', e.amount_cents, 'currency', e.currency,
                'project', p.name, 'reason', e.reason, 'at', e.created_at) ORDER BY e.created_at)
                FROM earnings_ledger e LEFT JOIN projects p ON p.id = e.project_id WHERE e.contributor_id = cp.id), '[]'),
  'payouts', COALESCE((SELECT jsonb_agg(jsonb_build_object('amount_cents', pl.amount_cents, 'currency', pl.currency, 'status', pl.status,
                'mpesa_phone', pl.mpesa_phone, 'payee_name', pl.payee_name, 'mpesa_receipt', pl.mpesa_receipt,
                'failure_reason', pl.failure_reason, 'recorded_at', pl.recorded_at) ORDER BY pl.created_at)
                FROM payout_lines pl JOIN payout_batches b ON b.id = pl.batch_id
                WHERE pl.contributor_id = cp.id AND b.status <> 'cancelled'), '[]'),
  'email_choices', (SELECT jsonb_build_object('project_available', n.project_available, 'feedback_received', n.feedback_received)
                FROM notification_preferences n WHERE n.user_id = cp.user_id),
  'sessions', COALESCE((SELECT jsonb_agg(jsonb_build_object('started_at', se.created_at, 'last_seen_at', se.last_seen_at,
                'ip', se.ip, 'browser', se.user_agent) ORDER BY se.created_at)
                FROM sessions se WHERE se.user_id = cp.user_id AND se.revoked_at IS NULL AND se.expires_at > now()), '[]'),
  'erasure_requests', COALESCE((SELECT jsonb_agg(jsonb_build_object('status', er.status, 'at', er.created_at, 'closed_at', er.closed_at) ORDER BY er.created_at)
                FROM erasure_requests er WHERE er.contributor_id = cp.id), '[]')
)::jsonb AS data
FROM contributor_profiles cp WHERE cp.id = $1 AND cp.erased_at IS NULL;

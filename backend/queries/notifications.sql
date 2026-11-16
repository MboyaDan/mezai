-- Stage 16: notification emails and preferences.

-- name: EnqueueUserEmail :exec
-- A notification about one account; a repeated dedupe_key is ignored.
INSERT INTO email_outbox (to_address, template, data, user_id, dedupe_key)
VALUES (@to_address, @template, @data, @user_id, @dedupe_key)
ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;

-- name: NotifyTargetByUser :one
-- Where to write and what the person chose. Disabled accounts and
-- unverified addresses get no notifications.
SELECT u.id, u.email, u.display_name,
       (u.status = 'active' AND u.email_verified_at IS NOT NULL)::boolean AS reachable,
       coalesce(np.project_available, true)::boolean AS project_available,
       coalesce(np.feedback_received, true)::boolean AS feedback_received
FROM users u
LEFT JOIN notification_preferences np ON np.user_id = u.id
WHERE u.id = $1;

-- name: NotifyTargetByContributor :one
SELECT u.id, u.email, u.display_name,
       (u.status = 'active' AND u.email_verified_at IS NOT NULL)::boolean AS reachable,
       coalesce(np.project_available, true)::boolean AS project_available,
       coalesce(np.feedback_received, true)::boolean AS feedback_received
FROM contributor_profiles cp
JOIN users u ON u.id = cp.user_id
LEFT JOIN notification_preferences np ON np.user_id = u.id
WHERE cp.id = $1;

-- name: GetNotificationPreferences :one
SELECT coalesce(np.project_available, true)::boolean AS project_available,
       coalesce(np.feedback_received, true)::boolean AS feedback_received,
       np.updated_at
FROM users u
LEFT JOIN notification_preferences np ON np.user_id = u.id
WHERE u.id = $1;

-- name: SetNotificationPreferences :exec
INSERT INTO notification_preferences (user_id, project_available, feedback_received)
VALUES (@user_id, @project_available, @feedback_received)
ON CONFLICT (user_id) DO UPDATE
SET project_available = EXCLUDED.project_available, feedback_received = EXCLUDED.feedback_received;

-- name: ActiveContributorIDs :many
-- Contributors who may work at all (active, identity verified): the starting
-- list for "new project available"; skills are checked in Go.
SELECT id FROM contributor_profiles
WHERE status = 'active' AND identity_status = 'verified'
ORDER BY id;

-- name: PayoutLineForNotice :one
-- What the "payout sent" email needs: the last three digits of the number
-- paid, never the whole number.
SELECT right(mpesa_phone, 3)::text AS phone_end, recorded_at
FROM payout_lines WHERE id = $1;

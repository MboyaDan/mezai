-- Two-step sign-in (Stage 17d).

-- name: GetUserTOTP :one
SELECT id, email, totp_secret_enc, totp_enabled_at, totp_last_step, totp_pending_enc, totp_pending_at
FROM users WHERE id = $1 FOR UPDATE;

-- name: SetPendingTOTP :exec
UPDATE users SET totp_pending_enc = @secret_enc, totp_pending_at = now() WHERE id = @id;

-- name: EnableTOTP :exec
UPDATE users SET totp_secret_enc = totp_pending_enc, totp_enabled_at = now(), totp_last_step = @step,
  totp_pending_enc = NULL, totp_pending_at = NULL
WHERE id = @id AND totp_pending_enc IS NOT NULL;

-- name: DisableTOTP :exec
UPDATE users SET totp_secret_enc = NULL, totp_enabled_at = NULL, totp_last_step = NULL,
  totp_pending_enc = NULL, totp_pending_at = NULL
WHERE id = $1;

-- name: SetTOTPLastStep :exec
UPDATE users SET totp_last_step = @step WHERE id = @id;

-- name: DeleteRecoveryCodes :exec
DELETE FROM recovery_codes WHERE user_id = $1;

-- name: AddRecoveryCode :exec
INSERT INTO recovery_codes (user_id, code_hash) VALUES ($1, $2);

-- name: UseRecoveryCode :execrows
UPDATE recovery_codes SET used_at = now() WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- name: CountRecoveryCodesLeft :one
SELECT count(*)::int FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL;

-- name: CreateLoginChallenge :exec
INSERT INTO login_challenges (user_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: GetLoginChallenge :one
SELECT c.id, c.user_id, c.attempts, c.expires_at, u.email, u.display_name, u.status, u.locked_until
FROM login_challenges c JOIN users u ON u.id = c.user_id
WHERE c.token_hash = $1 FOR UPDATE OF c;

-- name: CountChallengeAttempt :exec
UPDATE login_challenges SET attempts = attempts + 1 WHERE id = $1;

-- name: DeleteLoginChallenge :exec
DELETE FROM login_challenges WHERE id = $1;

-- name: DeleteUserLoginChallenges :exec
DELETE FROM login_challenges WHERE user_id = $1;

-- name: DeleteExpiredLoginChallenges :execrows
DELETE FROM login_challenges WHERE expires_at < now() - interval '1 hour';

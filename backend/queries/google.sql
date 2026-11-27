-- Google sign-in (Stage 29, D-171).

-- name: CreateOAuthState :exec
INSERT INTO oauth_states (state_hash, code_verifier, nonce, intent, consents, expires_at)
VALUES (@state_hash, @code_verifier, @nonce, @intent, @consents, @expires_at);

-- name: TakeOAuthState :one
-- Used once: the row goes whether or not it is still in date.
DELETE FROM oauth_states WHERE state_hash = @state_hash
RETURNING code_verifier, nonce, intent, consents, expires_at;

-- name: DeleteExpiredOAuthStates :execrows
DELETE FROM oauth_states WHERE expires_at < now();

-- name: GetUserByIdentity :one
SELECT u.id, u.email, u.display_name, u.status, u.totp_enabled_at
FROM user_identities i JOIN users u ON u.id = i.user_id
WHERE i.provider = @provider AND i.subject = @subject;

-- name: GetIdentityByUser :one
SELECT provider, subject, email, created_at, last_used_at FROM user_identities
WHERE user_id = @user_id AND provider = @provider;

-- name: CreateIdentity :exec
INSERT INTO user_identities (user_id, provider, subject, email) VALUES (@user_id, @provider, @subject, @email);

-- name: TouchIdentity :exec
UPDATE user_identities SET email = @email, last_used_at = now() WHERE provider = @provider AND subject = @subject;

-- name: ClearUnprovenPassword :exec
-- Linking Google to an account whose email was never verified: whoever set
-- that password never proved the address, so it stops working.
UPDATE users SET password_hash = NULL WHERE id = @id AND email_verified_at IS NULL;

-- name: ListIdentitiesByContributor :many
-- For the contributor's data export.
SELECT i.provider, i.email, i.created_at, i.last_used_at
FROM user_identities i JOIN contributor_profiles cp ON cp.user_id = i.user_id
WHERE cp.id = @contributor_id ORDER BY i.created_at;

-- name: DeleteIdentitiesByContributor :execrows
-- Erasure: the link to the Google account goes with the rest.
DELETE FROM user_identities
WHERE user_id = (SELECT cp.user_id FROM contributor_profiles cp WHERE cp.id = @contributor_id);

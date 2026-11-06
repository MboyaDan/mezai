-- name: CreateContributorProfile :one
INSERT INTO contributor_profiles (
  user_id, public_code, status, country, city, education_level, weekly_hours, mpesa_phone, mpesa_phone_confirmed_at
) VALUES (
  @user_id, 'C-' || lpad(nextval('contributor_public_code_seq')::text, 4, '0'),
  @status, @country, @city, @education_level, @weekly_hours, @mpesa_phone, @mpesa_phone_confirmed_at
)
RETURNING *;

-- name: AddContributorLanguage :exec
INSERT INTO contributor_languages (contributor_id, language, proficiency, dialects)
VALUES ($1, $2, $3, $4);

-- name: AddContributorDomain :exec
INSERT INTO contributor_domains (contributor_id, domain) VALUES ($1, $2);

-- name: AddContributorStatusHistory :exec
INSERT INTO contributor_status_history (contributor_id, from_status, to_status, changed_by, reason)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateQualificationTest :one
INSERT INTO qualification_tests (
  title, skill_kind, skill_value, task_type, pass_threshold, instructions_md, status, config, source_project_id, created_by
) VALUES (
  @title, @skill_kind, @skill_value, @task_type, @pass_threshold, @instructions_md, @status, @config, @source_project_id, @created_by
)
RETURNING *;

-- name: AddQualificationQuestion :exec
INSERT INTO qualification_questions (test_id, position, content, expected, tolerance)
VALUES ($1, $2, $3, $4, $5);

-- name: GetContributorProfile :one
SELECT cp.*, u.email, u.display_name
FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id
WHERE cp.id = $1;

-- name: LockContributorProfile :one
SELECT * FROM contributor_profiles WHERE id = $1 FOR UPDATE;

-- name: UpdateContributorProfile :exec
UPDATE contributor_profiles SET
  country = @country, city = @city, education_level = @education_level, weekly_hours = @weekly_hours,
  mpesa_phone = @mpesa_phone, mpesa_phone_confirmed_at = @mpesa_phone_confirmed_at
WHERE id = @id;

-- name: SetUserDisplayName :exec
UPDATE users SET display_name = @display_name WHERE id = @id;

-- name: ClearContributorLanguages :exec
DELETE FROM contributor_languages WHERE contributor_id = $1;

-- name: ClearContributorDomains :exec
DELETE FROM contributor_domains WHERE contributor_id = $1;

-- name: ListContributorLanguages :many
SELECT language, proficiency, dialects FROM contributor_languages WHERE contributor_id = $1 ORDER BY language;

-- name: ListContributorDomains :many
SELECT domain FROM contributor_domains WHERE contributor_id = $1 ORDER BY domain;

-- name: SetContributorStatus :execrows
-- Moves only from the expected status, so two racing changes cannot both win.
UPDATE contributor_profiles SET status = @to_status WHERE id = @id AND status = @from_status;

-- name: ListContributorStatusHistory :many
SELECT h.from_status, h.to_status, h.reason, h.created_at, u.display_name AS changed_by_name
FROM contributor_status_history h LEFT JOIN users u ON u.id = h.changed_by
WHERE h.contributor_id = $1
ORDER BY h.created_at DESC, h.id DESC;

-- name: ListContributors :many
SELECT cp.id, cp.public_code, cp.status, cp.country, cp.mpesa_phone, cp.created_at, u.display_name, u.email,
  (SELECT array_agg(cl.language ORDER BY cl.language) FROM contributor_languages cl WHERE cl.contributor_id = cp.id)::text[] AS languages
FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id
WHERE sqlc.narg(status)::text IS NULL OR cp.status = sqlc.narg(status)::text
ORDER BY cp.created_at DESC, cp.id DESC
LIMIT 500;

-- name: InsertConsent :exec
INSERT INTO consents (user_id, document, version, ip) VALUES (@user_id, @document, @version, @ip);

-- name: ListLatestConsents :many
SELECT DISTINCT ON (document) document, version, accepted_at
FROM consents WHERE user_id = $1
ORDER BY document, accepted_at DESC, id DESC;

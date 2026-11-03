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
INSERT INTO qualification_tests (title, skill_kind, skill_value, task_type, pass_threshold, instructions_md, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddQualificationQuestion :exec
INSERT INTO qualification_questions (test_id, position, content, expected, tolerance)
VALUES ($1, $2, $3, $4, $5);

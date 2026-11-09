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
  cp.identity_status, cp.national_id_last3, cp.screening_status,
  EXISTS (SELECT 1 FROM interviews iv WHERE iv.contributor_id = cp.id AND iv.outcome = 'passed')::bool AS interview_passed,
  EXISTS (SELECT 1 FROM qualification_attempts qa WHERE qa.contributor_id = cp.id AND qa.status = 'approved')::bool AS qualified,
  (SELECT array_agg(cl.language ORDER BY cl.language) FROM contributor_languages cl WHERE cl.contributor_id = cp.id)::text[] AS languages,
  (SELECT count(*) FROM identity_flags f WHERE f.contributor_id = cp.id OR f.other_contributor_id = cp.id)::int AS flag_count,
  (SELECT count(*) FROM contributor_profiles o JOIN users ou ON ou.id = o.user_id
     WHERE o.id <> cp.id AND norm_name(ou.display_name) = norm_name(u.display_name))::int AS same_name_count
FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id
WHERE (sqlc.narg(status)::text IS NULL OR cp.status = sqlc.narg(status)::text)
  AND (sqlc.narg(identity_status)::text IS NULL OR cp.identity_status = sqlc.narg(identity_status)::text)
  AND (sqlc.narg(screening_status)::text IS NULL OR cp.screening_status = sqlc.narg(screening_status)::text)
ORDER BY cp.created_at DESC, cp.id DESC
LIMIT 500;

-- name: InsertConsent :exec
INSERT INTO consents (user_id, document, version, ip) VALUES (@user_id, @document, @version, @ip);

-- name: ListLatestConsents :many
SELECT DISTINCT ON (document) document, version, accepted_at
FROM consents WHERE user_id = $1
ORDER BY document, accepted_at DESC, id DESC;

-- One person, one account (Stage 7c, D-078).

-- name: SetContributorNationalID :exec
UPDATE contributor_profiles SET national_id_fp = @national_id_fp, national_id_last3 = @national_id_last3 WHERE id = @id;

-- name: ResetContributorIdentity :exec
-- A changed number needs checking again.
UPDATE contributor_profiles SET identity_status = 'pending', identity_note = NULL,
  identity_decided_by = NULL, identity_decided_at = NULL
WHERE id = @id;

-- name: ClearMpesaNameCheck :exec
UPDATE contributor_profiles SET mpesa_name_checked_at = NULL, mpesa_name_checked_by = NULL WHERE id = @id;

-- name: ContributorHoldingPhone :one
SELECT id FROM contributor_profiles WHERE mpesa_phone = @mpesa_phone AND id <> @id;

-- name: ContributorHoldingNationalID :one
SELECT id FROM contributor_profiles WHERE national_id_fp = @national_id_fp AND id <> @id;

-- name: AddIdentityFlag :exec
INSERT INTO identity_flags (contributor_id, other_contributor_id, kind) VALUES (@contributor_id, @other_contributor_id, @kind);

-- name: ListIdentityFlags :many
-- Flags raised by or against a contributor, with the other account's code.
SELECT f.kind, f.created_at, f.contributor_id = @id AS raised_by_this,
  o.id AS other_id, o.public_code AS other_public_code, o.status AS other_status
FROM identity_flags f
JOIN contributor_profiles o ON o.id = CASE WHEN f.contributor_id = @id THEN f.other_contributor_id ELSE f.contributor_id END
WHERE f.contributor_id = @id OR f.other_contributor_id = @id
ORDER BY f.created_at DESC
LIMIT 50;

-- name: ListSameNameContributors :many
SELECT o.id, o.public_code, o.status
FROM contributor_profiles o JOIN users ou ON ou.id = o.user_id
WHERE o.id <> @id AND norm_name(ou.display_name) = norm_name(@display_name::text)
ORDER BY o.created_at
LIMIT 20;

-- name: DecideContributorIdentity :exec
UPDATE contributor_profiles SET identity_status = @identity_status, identity_note = @identity_note,
  identity_decided_by = @decided_by, identity_decided_at = now()
WHERE id = @id;

-- name: RecordMpesaNameCheck :exec
UPDATE contributor_profiles SET mpesa_name_checked_at = now(), mpesa_name_checked_by = @checked_by WHERE id = @id;

-- name: GetUserDisplayNameByID :one
SELECT display_name FROM users WHERE id = $1;

-- Screening funnel, part 1 (Stage 7d-1, D-100).

-- name: UpdateContributorApplication :exec
UPDATE contributor_profiles SET experience = @experience, linkedin_url = @linkedin_url WHERE id = @id;

-- name: SetContributorCV :exec
UPDATE contributor_profiles SET cv_file_key = @cv_file_key, cv_file_name = @cv_file_name, cv_uploaded_at = now(),
  cv_delete_after = CASE WHEN screening_status = 'pending' THEN NULL ELSE now() + interval '90 days' END
WHERE id = @id;

-- name: ClearContributorCV :exec
UPDATE contributor_profiles SET cv_file_key = NULL, cv_file_name = NULL, cv_uploaded_at = NULL, cv_delete_after = NULL WHERE id = $1;

-- name: DecideScreening :exec
-- A decision starts the 90-day clock on any CV (declines delete it at once in the service).
UPDATE contributor_profiles SET screening_status = @screening_status, screening_note = @screening_note,
  screening_decided_by = @decided_by, screening_decided_at = now(),
  cv_delete_after = CASE WHEN cv_file_key IS NULL THEN NULL ELSE now() + interval '90 days' END
WHERE id = @id;

-- name: ListExpiredCVs :many
SELECT id, cv_file_key FROM contributor_profiles WHERE cv_file_key IS NOT NULL AND cv_delete_after <= now() LIMIT 100;

-- name: AddInterview :one
INSERT INTO interviews (contributor_id, interviewer_id, outcome, identity_confirmed, language_scores, scenario_score,
  confidentiality_ok, availability, note)
VALUES (@contributor_id, @interviewer_id, @outcome, @identity_confirmed, @language_scores, @scenario_score,
  @confidentiality_ok, @availability, @note)
RETURNING *;

-- name: ListInterviews :many
SELECT i.*, u.display_name AS interviewer_name
FROM interviews i JOIN users u ON u.id = i.interviewer_id
WHERE i.contributor_id = $1
ORDER BY i.created_at DESC;

-- name: HasPassedInterview :one
SELECT EXISTS (SELECT 1 FROM interviews WHERE contributor_id = $1 AND outcome = 'passed')::bool;

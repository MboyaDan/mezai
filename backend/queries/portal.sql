-- Client portal (Stage 13a): project progress, workforce, organisation
-- activity, team and invoices. Progress, workforce and activity run in
-- system scope after the caller's access is checked, because gold rows and
-- the audit log are staff-only; every query filters by project or org.

-- name: ProjectProgress :one
-- Items by stage, without gold checks.
SELECT
  count(*)::int AS total,
  count(*) FILTER (WHERE i.status = 'pending')::int AS not_started,
  count(*) FILTER (WHERE i.status IN ('in_progress', 'awaiting_consensus', 'consensus'))::int AS in_progress,
  count(*) FILTER (WHERE i.status = 'in_review')::int AS in_review,
  count(*) FILTER (WHERE i.status = 'final')::int AS final,
  count(*) FILTER (WHERE i.status = 'rejected')::int AS rejected
FROM items i
WHERE i.project_id = @project_id
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id);

-- name: ProjectThroughput :many
-- Items made final per Nairobi week (Monday start), without gold checks.
SELECT (date_trunc('week', i.finalised_at AT TIME ZONE 'Africa/Nairobi'))::date AS week,
       count(*)::int AS final
FROM items i
WHERE i.project_id = @project_id AND i.status = 'final' AND i.finalised_at >= @since
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
GROUP BY 1
ORDER BY 1;

-- name: ProjectWorkforce :many
-- Contributors who have submitted work on a project. Only the id leaves the
-- service, as a per-organisation code (D-123).
SELECT s.contributor_id,
       count(*)::int AS tasks,
       max(s.submitted_at)::timestamptz AS last_submitted_at
FROM submissions s
WHERE s.project_id = @project_id AND s.status <> 'draft' AND s.submitted_at IS NOT NULL
GROUP BY s.contributor_id
ORDER BY tasks DESC, s.contributor_id;

-- name: ProjectWorkforceSkills :many
-- Languages and domains the project's contributors hold, with how many hold each.
SELECT 'language'::text AS kind, cl.language AS value, count(DISTINCT cl.contributor_id)::int AS contributors
FROM contributor_languages cl
WHERE cl.contributor_id IN (SELECT s.contributor_id FROM submissions s WHERE s.project_id = @project_id AND s.status <> 'draft')
GROUP BY cl.language
UNION ALL
SELECT 'domain'::text, cd.domain, count(DISTINCT cd.contributor_id)::int
FROM contributor_domains cd
WHERE cd.contributor_id IN (SELECT s.contributor_id FROM submissions s WHERE s.project_id = @project_id AND s.status <> 'draft')
GROUP BY cd.domain
ORDER BY 1, 3 DESC, 2;

-- name: OrgActivity :many
-- One organisation's audit entries for the given actions, newest first.
-- Gold imports are never shown (gold is staff-only). Names are resolved here
-- so the audit payloads (which can hold staff-only fields) never leave.
SELECT a.id, a.action, a.entity_type, a.created_at, a.actor_kind,
       u.display_name AS actor_name,
       (m.user_id IS NOT NULL)::boolean AS actor_is_member,
       p.id AS project_id,
       p.name AS project_name,
       coalesce(
         CASE a.entity_type
           WHEN 'project_request' THEN (SELECT r.title FROM project_requests r WHERE r.id = a.entity_id)
           WHEN 'invitation' THEN (SELECT inv.email FROM invitations inv WHERE inv.id = a.entity_id)
           WHEN 'user' THEN (SELECT su.display_name FROM users su WHERE su.id = a.entity_id)
           WHEN 'invoice' THEN (SELECT iv.number FROM invoices iv WHERE iv.id = a.entity_id)
         END, '')::text AS subject,
       coalesce(a.after ->> 'status', '')::text AS status,
       coalesce(a.after ->> 'version', '')::text AS version,
       coalesce(a.after ->> 'file', '')::text AS file,
       coalesce(a.after ->> 'rows', '')::text AS rows
FROM audit_log a
LEFT JOIN users u ON u.id = a.actor_user_id
LEFT JOIN org_members m ON m.org_id = a.org_id AND m.user_id = a.actor_user_id
LEFT JOIN projects p ON p.id = CASE a.entity_type
    WHEN 'project' THEN a.entity_id
    WHEN 'deliverable' THEN (SELECT d.project_id FROM deliverables d WHERE d.id = a.entity_id)
    WHEN 'import_job' THEN (SELECT j.project_id FROM import_jobs j WHERE j.id = a.entity_id)
    WHEN 'project_request' THEN (SELECT r.project_id FROM project_requests r WHERE r.id = a.entity_id)
    WHEN 'invoice' THEN (SELECT iv.project_id FROM invoices iv WHERE iv.id = a.entity_id)
  END
WHERE a.org_id = @org_id
  AND a.action = ANY (@actions::text[])
  AND NOT (a.action = 'import.committed' AND a.after ->> 'kind' = 'gold')
  AND (sqlc.narg(before)::timestamptz IS NULL OR a.created_at < sqlc.narg(before))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @lim;

-- name: OrgMembers :many
-- People in one organisation with their organisation role.
SELECT u.id, u.display_name, u.email, u.status, u.last_login_at, m.created_at AS joined_at,
       coalesce((SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id
                 WHERE ur.user_id = u.id AND ur.org_id = m.org_id
                 ORDER BY r.key LIMIT 1), '')::text AS role_key
FROM org_members m JOIN users u ON u.id = m.user_id
WHERE m.org_id = @org_id
ORDER BY u.display_name, u.id;

-- name: CreateInvoice :one
INSERT INTO invoices (org_id, project_id, number, issued_on, due_on, amount_cents, currency, description,
                      file_key, file_name, byte_size, checksum_sha256, uploaded_by)
VALUES (@org_id, sqlc.narg(project_id), @number, @issued_on, sqlc.narg(due_on), @amount_cents, @currency, @description,
        @file_key, @file_name, @byte_size, @checksum_sha256, @uploaded_by)
RETURNING id;

-- name: ListInvoices :many
-- Row-level security limits a client to its own organisation.
SELECT iv.*, o.name AS org_name, p.name AS project_name
FROM invoices iv
JOIN organisations o ON o.id = iv.org_id
LEFT JOIN projects p ON p.id = iv.project_id
WHERE sqlc.narg(org_id)::uuid IS NULL OR iv.org_id = sqlc.narg(org_id)
ORDER BY iv.issued_on DESC, iv.created_at DESC;

-- name: GetInvoice :one
SELECT iv.*, o.name AS org_name, p.name AS project_name
FROM invoices iv
JOIN organisations o ON o.id = iv.org_id
LEFT JOIN projects p ON p.id = iv.project_id
WHERE iv.id = @id;

-- name: SetInvoiceStatus :one
-- issued → paid or void; paid → void. A void invoice stays void.
UPDATE invoices
SET status = @status, paid_on = sqlc.narg(paid_on), void_reason = sqlc.narg(void_reason)
WHERE id = @id AND status <> 'void' AND (@status::text <> 'paid' OR status = 'issued')
RETURNING id;

-- name: CountFinalSince :one
-- Items made final since a time, without gold checks (the finishing pace).
SELECT count(*)::int FROM items i
WHERE i.project_id = @project_id AND i.status = 'final' AND i.finalised_at >= @since
  AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id);

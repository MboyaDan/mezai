-- name: CreateEnquiry :one
-- A project enquiry from the public contact form. Status starts as 'new'.
INSERT INTO enquiries (name, organisation_name, email, project_type, languages, volume_range, timeline, message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at;

-- name: ListEnquiries :many
-- The Ops inbox, newest first. A NULL status lists the open ones
-- (new, assigned, in_conversation); 'all' lists everything.
SELECT e.id, e.name, e.organisation_name, e.email, e.project_type, e.volume_range, e.timeline,
       e.status, e.assigned_to, u.display_name AS assigned_name, e.created_at, e.updated_at,
       (SELECT count(*) FROM enquiry_notes n WHERE n.enquiry_id = e.id)::int AS note_count
FROM enquiries e
LEFT JOIN users u ON u.id = e.assigned_to
WHERE CASE
        WHEN sqlc.narg(status)::text IS NULL THEN e.status IN ('new', 'assigned', 'in_conversation')
        WHEN sqlc.narg(status)::text = 'all' THEN true
        ELSE e.status = sqlc.narg(status)::text
      END
  AND (sqlc.narg(assigned_to)::uuid IS NULL OR e.assigned_to = sqlc.narg(assigned_to)::uuid)
ORDER BY e.created_at DESC, e.id DESC
LIMIT 200;

-- name: CountEnquiriesByStatus :many
SELECT status, count(*)::int AS n FROM enquiries GROUP BY status;

-- name: GetEnquiry :one
SELECT e.id, e.name, e.organisation_name, e.email, e.project_type, e.languages, e.volume_range,
       e.timeline, e.message, e.status, e.assigned_to, u.display_name AS assigned_name,
       e.org_id, o.name AS org_name, e.created_at, e.updated_at
FROM enquiries e
LEFT JOIN users u ON u.id = e.assigned_to
LEFT JOIN organisations o ON o.id = e.org_id
WHERE e.id = $1;

-- name: LockEnquiry :one
SELECT id, status, assigned_to, org_id FROM enquiries WHERE id = $1 FOR UPDATE;

-- name: UpdateEnquiry :exec
UPDATE enquiries SET status = @status, assigned_to = sqlc.narg(assigned_to), org_id = sqlc.narg(org_id)
WHERE id = @id;

-- name: ListEnquiryNotes :many
SELECT n.id, n.body, n.created_at, n.author_user_id, u.display_name AS author_name
FROM enquiry_notes n JOIN users u ON u.id = n.author_user_id
WHERE n.enquiry_id = $1
ORDER BY n.created_at, n.id;

-- name: AddEnquiryNote :one
INSERT INTO enquiry_notes (enquiry_id, author_user_id, body)
VALUES ($1, $2, $3)
RETURNING id, created_at;

-- name: EnquiryAssignees :many
-- Active people holding enquiry.manage through a platform role (staff).
SELECT DISTINCT u.id, u.display_name
FROM users u
JOIN user_roles ur ON ur.user_id = u.id AND ur.org_id IS NULL
JOIN roles ro ON ro.id = ur.role_id AND ro.scope = 'platform'
JOIN role_permissions rp ON rp.role_id = ro.id
JOIN permissions pm ON pm.id = rp.permission_id
WHERE pm.key = 'enquiry.manage' AND u.status = 'active'
ORDER BY u.display_name;

-- name: ActiveOrganisationExists :one
SELECT EXISTS (SELECT 1 FROM organisations WHERE id = $1 AND status = 'active');

-- name: CreateEnquiry :one
-- A project enquiry from the public contact form. Status starts as 'new'.
INSERT INTO enquiries (name, organisation_name, email, project_type, languages, volume_range, timeline, message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at;

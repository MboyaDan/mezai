-- name: GetLatestTaskTemplate :one
SELECT * FROM task_templates WHERE task_type = $1 ORDER BY version DESC LIMIT 1;

-- name: CreateProject :one
INSERT INTO projects (
  org_id, template_id, task_type, name, status, instructions_md, config,
  overlap_k, gold_rate, pay_per_task_cents, currency, target_count, deadline, published_at, created_by
) VALUES (
  @org_id, @template_id, @task_type, @name, @status, @instructions_md, @config,
  @overlap_k, @gold_rate, @pay_per_task_cents, @currency, @target_count, @deadline, @published_at, @created_by
)
RETURNING *;

-- name: ListProjects :many
-- Row-level security decides which rows the caller's scope can see.
SELECT * FROM projects ORDER BY created_at;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: AddProjectSkill :exec
INSERT INTO project_skills (org_id, project_id, skill_kind, skill_value, min_proficiency)
VALUES ($1, $2, $3, $4, $5);

-- name: AddProjectGuideline :exec
INSERT INTO project_guidelines (org_id, project_id, title, body_md, position)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertProject :one
-- Stage 8: every setting Ops enters on the create form.
INSERT INTO projects (
  org_id, template_id, task_type, name, instructions_md, config, qa_checks,
  overlap_k, gold_rate, pay_per_task_cents, currency, target_count, deadline,
  min_rationale_chars, created_by
) VALUES (
  @org_id, @template_id, @task_type, @name, @instructions_md, @config, @qa_checks,
  @overlap_k, @gold_rate, @pay_per_task_cents, @currency, @target_count, @deadline,
  @min_rationale_chars, @created_by
)
RETURNING *;

-- name: ListProjectSummaries :many
-- RLS limits clients to their organisation. item_count excludes gold for
-- staff; gold rows are staff-only, so outside that scope gold_count is 0.
SELECT sqlc.embed(p), o.name AS org_name,
  (SELECT count(*) FROM items i WHERE i.project_id = p.id
     AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id))::int AS item_count,
  (SELECT count(*) FROM gold_items g WHERE g.project_id = p.id AND g.status = 'active')::int AS gold_count
FROM projects p
JOIN organisations o ON o.id = p.org_id
WHERE (sqlc.narg(org_id)::uuid IS NULL OR p.org_id = sqlc.narg(org_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text)
ORDER BY p.created_at DESC
LIMIT 500;

-- name: GetProjectSummary :one
SELECT sqlc.embed(p), o.name AS org_name,
  (SELECT count(*) FROM items i WHERE i.project_id = p.id
     AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id))::int AS item_count,
  (SELECT count(*) FROM gold_items g WHERE g.project_id = p.id AND g.status = 'active')::int AS gold_count
FROM projects p
JOIN organisations o ON o.id = p.org_id
WHERE p.id = $1;

-- name: LockProject :one
SELECT * FROM projects WHERE id = $1 FOR UPDATE;

-- name: UpdateProjectSettings :one
UPDATE projects SET
  name = @name, instructions_md = @instructions_md, config = @config, qa_checks = @qa_checks,
  overlap_k = @overlap_k, gold_rate = @gold_rate, pay_per_task_cents = @pay_per_task_cents,
  target_count = @target_count, deadline = @deadline, min_rationale_chars = @min_rationale_chars
WHERE id = @id
RETURNING *;

-- name: SetProjectStatus :one
UPDATE projects SET status = @status,
  published_at = coalesce(published_at, sqlc.narg(published_at)::timestamptz),
  closed_at = sqlc.narg(closed_at)::timestamptz
WHERE id = @id
RETURNING *;

-- name: ListProjectSkills :many
SELECT * FROM project_skills WHERE project_id = $1 ORDER BY skill_kind, skill_value;

-- name: DeleteProjectSkills :exec
DELETE FROM project_skills WHERE project_id = $1;

-- name: ListTaskTemplates :many
SELECT DISTINCT ON (task_type) * FROM task_templates ORDER BY task_type, version DESC;

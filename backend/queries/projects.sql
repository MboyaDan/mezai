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

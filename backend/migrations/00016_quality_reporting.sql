-- Quality reporting (Stage 10b): warm-up settings and snapshot lookups.

-- +goose Up
INSERT INTO settings (key, value, description) VALUES
  ('warmup.tasks',     '5',   'First tasks of each project a contributor works at the warm-up gold rate (D-100)'),
  ('warmup.gold_rate', '0.5', 'Gold rate during a contributor''s warm-up on a project');
CREATE INDEX agreement_snapshots_latest_idx ON agreement_snapshots (project_id, statistic, label, created_at DESC);
CREATE INDEX consensus_results_project_idx ON consensus_results (project_id, created_at);
CREATE INDEX jobs_due_idx ON jobs (run_at) WHERE status IN ('queued', 'running');

-- +goose Down
DROP INDEX jobs_due_idx;
DROP INDEX consensus_results_project_idx;
DROP INDEX agreement_snapshots_latest_idx;
DELETE FROM settings WHERE key IN ('warmup.tasks', 'warmup.gold_rate');

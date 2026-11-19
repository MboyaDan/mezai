-- +goose Up
-- Stage 22b (D-164): single-response rating. An item is a prompt and one
-- response; the answer is a 1 to 5 score per project criterion plus a
-- written rationale. Consensus is the mean per criterion with its spread.
ALTER TABLE task_templates DROP CONSTRAINT task_templates_task_type_check;
ALTER TABLE task_templates ADD CONSTRAINT task_templates_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'rating'));

ALTER TABLE qualification_tests DROP CONSTRAINT qualification_tests_task_type_check;
ALTER TABLE qualification_tests ADD CONSTRAINT qualification_tests_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'rating'));

ALTER TABLE qualification_questions DROP CONSTRAINT qualification_questions_task_type_check;
ALTER TABLE qualification_questions ADD CONSTRAINT qualification_questions_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'rating', 'writing'));

ALTER TABLE project_requests DROP CONSTRAINT project_requests_task_type_check;
ALTER TABLE project_requests ADD CONSTRAINT project_requests_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'rating', 'other'));

INSERT INTO task_templates (task_type, name, import_schema, submission_schema, export_schema) VALUES
  ('rating', 'Response rating',
   '{"type": "object", "required": ["prompt", "response"], "properties": {"prompt": {"type": "string", "minLength": 1}, "response": {"type": "string", "minLength": 1}}}',
   '{"type": "object", "required": ["scores", "rationale"], "properties": {"scores": {"type": "object", "additionalProperties": {"type": "integer", "minimum": 1, "maximum": 5}}, "rationale": {"type": "string"}}}',
   '{"type": "object", "required": ["item_id", "scores"], "properties": {"item_id": {"type": "string"}, "scores": {"type": "object", "additionalProperties": {"type": "number"}}, "rationale": {"type": "string"}}}');

-- +goose Down
-- Refuses while rating projects, tests or requests exist; remove them first.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM projects WHERE task_type = 'rating')
     OR EXISTS (SELECT 1 FROM qualification_tests WHERE task_type = 'rating')
     OR EXISTS (SELECT 1 FROM qualification_questions WHERE task_type = 'rating')
     OR EXISTS (SELECT 1 FROM project_requests WHERE task_type = 'rating') THEN
    RAISE EXCEPTION 'rating projects, tests or requests exist';
  END IF;
END $$;
-- +goose StatementEnd
DELETE FROM task_templates WHERE task_type = 'rating';
ALTER TABLE project_requests DROP CONSTRAINT project_requests_task_type_check;
ALTER TABLE project_requests ADD CONSTRAINT project_requests_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'other'));
ALTER TABLE qualification_questions DROP CONSTRAINT qualification_questions_task_type_check;
ALTER TABLE qualification_questions ADD CONSTRAINT qualification_questions_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review', 'writing'));
ALTER TABLE qualification_tests DROP CONSTRAINT qualification_tests_task_type_check;
ALTER TABLE qualification_tests ADD CONSTRAINT qualification_tests_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review'));
ALTER TABLE task_templates DROP CONSTRAINT task_templates_task_type_check;
ALTER TABLE task_templates ADD CONSTRAINT task_templates_task_type_check
  CHECK (task_type IN ('classification', 'pairwise', 'translation_review'));

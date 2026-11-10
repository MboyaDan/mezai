-- Review and adjudication (Stage 11a, brief 8.3, 13, AC-07, AC-09).
--
-- * submissions.decision_source says who accepted or rejected a submission:
--   consensus or gold (automatic, D-110) or review (a reviewer). Only
--   reviewer decisions feed the review-acceptance part of trust (brief 8.4).
-- * submissions.review_id links a reviewer's decision to the work it decided.
-- * project_reviewers: the projects a Reviewer may review (D-111).

-- +goose Up
ALTER TABLE submissions
  ADD COLUMN decision_source text CHECK (decision_source IN ('consensus', 'gold', 'review')),
  ADD COLUMN review_id uuid REFERENCES reviews (id),
  ADD CONSTRAINT submissions_decided_source CHECK ((status IN ('accepted', 'rejected')) = (decision_source IS NOT NULL)),
  ADD CONSTRAINT submissions_review_link CHECK (decision_source <> 'review' OR review_id IS NOT NULL);

-- One decision per item: a later correction is an item revision, not a second review.
CREATE UNIQUE INDEX reviews_one_per_item_idx ON reviews (item_id);

CREATE TABLE project_reviewers (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id     uuid NOT NULL,
  project_id uuid NOT NULL,
  user_id    uuid NOT NULL REFERENCES users (id),
  added_by   uuid REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, user_id),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id)
);
CREATE INDEX project_reviewers_user_idx ON project_reviewers (user_id);
ALTER TABLE project_reviewers ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_reviewers_staff ON project_reviewers FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());
GRANT SELECT, INSERT, UPDATE, DELETE ON project_reviewers TO mezai_app;

CREATE INDEX items_review_idx ON items (status, updated_at) WHERE status = 'in_review';

-- Development data from Stage 10: items left at 'consensus' become final
-- with their consensus answer, and their submissions (and gold submissions)
-- are accepted and earn, as they would from now on. The pilot has no
-- production data yet, so this only touches local and staging databases.
INSERT INTO item_revisions (org_id, item_id, revision, answer, source)
SELECT DISTINCT ON (c.item_id) c.org_id, c.item_id, i.current_revision + 1, c.result, 'consensus'
FROM consensus_results c JOIN items i ON i.id = c.item_id
WHERE i.status = 'consensus'
ORDER BY c.item_id, c.created_at DESC;

UPDATE items SET status = 'final', finalised_at = now(), current_revision = current_revision + 1
WHERE status = 'consensus';

UPDATE submissions s SET status = 'accepted', decided_at = now(), decision_source = 'consensus'
FROM items i WHERE i.id = s.item_id AND i.status = 'final' AND s.status = 'submitted';

UPDATE submissions s SET status = 'accepted', decided_at = now(), decision_source = 'gold'
WHERE s.status = 'submitted' AND EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = s.item_id);

INSERT INTO earnings_ledger (contributor_id, entry_type, amount_cents, currency, project_id, submission_id)
SELECT s.contributor_id, 'accrual', p.pay_per_task_cents, p.currency, s.project_id, s.id
FROM submissions s JOIN projects p ON p.id = s.project_id
WHERE s.status = 'accepted' AND p.pay_per_task_cents > 0
ON CONFLICT DO NOTHING;

-- +goose Down
DROP INDEX items_review_idx;
DROP TABLE project_reviewers;
DROP INDEX reviews_one_per_item_idx;
ALTER TABLE submissions
  DROP CONSTRAINT submissions_review_link,
  DROP CONSTRAINT submissions_decided_source,
  DROP COLUMN review_id,
  DROP COLUMN decision_source;

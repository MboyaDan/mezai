-- Quality engine, part 1 (Stage 10a, brief 8): gold scoring, consensus,
-- trust score inputs and probation.

-- +goose Up

-- Whether a submission agreed with the item's final answer: set when an
-- item reaches consensus (or, from Stage 11, when a reviewer decides).
-- Feeds the trust score's "agreement with final answer" input.
ALTER TABLE submissions ADD COLUMN agrees_with_final boolean;
CREATE INDEX submissions_agreement_idx ON submissions (contributor_id, submitted_at) WHERE agrees_with_final IS NOT NULL;

-- One consensus per item (a new one replaces nothing: review in Stage 11
-- records its decision separately).
CREATE UNIQUE INDEX consensus_results_one_per_item ON consensus_results (item_id);

-- Probation (brief 8.4): until a contributor has trust.min_gold_results gold
-- results they get more gold, so their score builds quickly.
INSERT INTO settings (key, value, description) VALUES
  ('trust.probation_gold_multiplier', '2',   'Gold rate multiplier while a contributor is building a trust score'),
  ('trust.probation_gold_max',        '0.5', 'Highest gold rate during probation');

-- +goose Down
DELETE FROM settings WHERE key IN ('trust.probation_gold_multiplier', 'trust.probation_gold_max');
DROP INDEX consensus_results_one_per_item;
DROP INDEX submissions_agreement_idx;
ALTER TABLE submissions DROP COLUMN agrees_with_final;

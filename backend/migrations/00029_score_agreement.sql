-- +goose Up
-- Stage 22a (D-163): rubric scores get Krippendorff's alpha (ordinal) and
-- agreement within one point alongside percent and mean absolute difference.
ALTER TABLE agreement_snapshots DROP CONSTRAINT agreement_snapshots_statistic_check;
ALTER TABLE agreement_snapshots ADD CONSTRAINT agreement_snapshots_statistic_check
  CHECK (statistic IN ('percent', 'cohen_kappa', 'fleiss_kappa', 'mean_abs_diff', 'krippendorff_alpha', 'within_one'));

-- +goose Down
DELETE FROM agreement_snapshots WHERE statistic IN ('krippendorff_alpha', 'within_one');
ALTER TABLE agreement_snapshots DROP CONSTRAINT agreement_snapshots_statistic_check;
ALTER TABLE agreement_snapshots ADD CONSTRAINT agreement_snapshots_statistic_check
  CHECK (statistic IN ('percent', 'cohen_kappa', 'fleiss_kappa', 'mean_abs_diff'));

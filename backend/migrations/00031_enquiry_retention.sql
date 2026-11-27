-- Stage 28 (A-096, D-170): how long enquiries are kept. The privacy notice
-- says enquiries that do not become a project are kept 24 months after the
-- last contact; spam holds no relationship and goes after 30 days. Won and
-- open enquiries are never deleted by the sweep. Values are days, counted
-- from the enquiry's last change (updated_at), so any later handling only
-- extends the period.

-- +goose Up
INSERT INTO settings (key, value, description) VALUES
  ('retention.enquiry_lost_days', '730', 'Days after the last change before a lost enquiry is deleted'),
  ('retention.enquiry_spam_days', '30',  'Days after the last change before an enquiry marked spam is deleted');

-- The sweep finds due enquiries by status and age.
CREATE INDEX enquiries_retention_idx ON enquiries (status, updated_at) WHERE status IN ('lost', 'spam');

-- +goose Down
DROP INDEX enquiries_retention_idx;
DELETE FROM settings WHERE key IN ('retention.enquiry_lost_days', 'retention.enquiry_spam_days');

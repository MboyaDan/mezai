-- Stage 14a: payouts, manual M-Pesa path (brief 9 and 13, AC-11, D-127).
--
-- Lifecycle: draft -> pending_approval -> approved -> exported -> settled,
-- or cancelled from draft or pending_approval (kept, never deleted). Money
-- leaves the balance only when a line is recorded paid (a negative 'payout'
-- ledger entry); a failed line writes nothing, so the amount stays owed.
-- Only one batch is open at a time, so nothing is batched twice.

-- +goose Up
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_status_check;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_status_check
  CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported', 'settled', 'cancelled'));
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_check2;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_approved_chk
  CHECK (status IN ('draft', 'pending_approval', 'cancelled') OR (approved_by IS NOT NULL AND approved_at IS NOT NULL));

ALTER TABLE payout_batches
  ADD COLUMN cutoff_at        timestamptz,   -- end of period_end, Nairobi
  ADD COLUMN submitted_at     timestamptz,
  ADD COLUMN returned_note    text CHECK (length(returned_note) <= 500),
  ADD COLUMN cancelled_by     uuid REFERENCES users (id),
  ADD COLUMN cancelled_at     timestamptz,
  ADD COLUMN cancel_reason    text CHECK (length(cancel_reason) <= 500),
  ADD COLUMN export_file_name text,
  ADD COLUMN export_checksum  text CHECK (export_checksum ~ '^[0-9a-f]{64}$'),
  ADD COLUMN export_format    jsonb,         -- the column set used, kept with the file
  ADD COLUMN exported_by      uuid REFERENCES users (id),
  ADD CONSTRAINT payout_batches_cancel_chk
    CHECK (status <> 'cancelled' OR (cancelled_by IS NOT NULL AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL)),
  ADD CONSTRAINT payout_batches_export_chk
    CHECK (status NOT IN ('exported', 'settled') OR (exported_at IS NOT NULL AND export_file_key IS NOT NULL AND export_checksum IS NOT NULL)),
  ADD CONSTRAINT payout_batches_settled_chk CHECK (status <> 'settled' OR settled_at IS NOT NULL);

-- One open batch at a time.
CREATE UNIQUE INDEX payout_batches_one_open ON payout_batches ((true)) WHERE status NOT IN ('settled', 'cancelled');
CREATE INDEX payout_batches_created_idx ON payout_batches (created_at DESC);

-- The payee name is a snapshot, like the phone: the export must match what
-- was approved even if the profile changes later.
ALTER TABLE payout_lines ADD COLUMN payee_name text NOT NULL DEFAULT '';
-- M-Pesa pays whole shillings.
ALTER TABLE payout_lines ADD CONSTRAINT payout_lines_whole_kes_chk CHECK (currency <> 'KES' OR amount_cents % 100 = 0);

-- A line's money fields never change, and a recorded result is final.
-- +goose StatementBegin
CREATE FUNCTION payout_lines_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.batch_id <> OLD.batch_id OR NEW.contributor_id <> OLD.contributor_id
     OR NEW.amount_cents <> OLD.amount_cents OR NEW.currency <> OLD.currency
     OR NEW.mpesa_phone <> OLD.mpesa_phone OR NEW.payee_name <> OLD.payee_name THEN
    RAISE EXCEPTION 'payout line % cannot change its payee or amount', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.status <> 'pending' THEN
    RAISE EXCEPTION 'payout line % already has a result', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER payout_lines_guard BEFORE UPDATE ON payout_lines FOR EACH ROW EXECUTE FUNCTION payout_lines_guard();

-- Batches and lines are kept: cancelled, never deleted.
REVOKE DELETE ON payout_batches, payout_lines FROM mezai_app;

-- The bulk payment file's columns (DG-03 not yet confirmed: D-127). Fields:
-- phone_254 (2547XXXXXXXX), phone_e164, amount_kes (whole shillings),
-- name, reference, public_code.
INSERT INTO settings (key, value, description) VALUES
  ('payout.export_format',
   '{"confirmed": false, "columns": [{"header": "Phone Number", "field": "phone_254"}, {"header": "Amount", "field": "amount_kes"}, {"header": "Name", "field": "name"}, {"header": "Reference", "field": "reference"}]}',
   'Columns of the M-Pesa bulk payment CSV (DG-03 confirms them)');

-- +goose Down
DELETE FROM settings WHERE key = 'payout.export_format';
GRANT DELETE ON payout_batches, payout_lines TO mezai_app;
DROP TRIGGER payout_lines_guard ON payout_lines;
DROP FUNCTION payout_lines_guard();
ALTER TABLE payout_lines DROP CONSTRAINT payout_lines_whole_kes_chk, DROP COLUMN payee_name;
DROP INDEX payout_batches_created_idx;
DROP INDEX payout_batches_one_open;
ALTER TABLE payout_batches
  DROP CONSTRAINT payout_batches_settled_chk, DROP CONSTRAINT payout_batches_export_chk,
  DROP CONSTRAINT payout_batches_cancel_chk,
  DROP COLUMN exported_by, DROP COLUMN export_format, DROP COLUMN export_checksum, DROP COLUMN export_file_name,
  DROP COLUMN cancel_reason, DROP COLUMN cancelled_at, DROP COLUMN cancelled_by, DROP COLUMN returned_note,
  DROP COLUMN submitted_at, DROP COLUMN cutoff_at;
DELETE FROM payout_lines WHERE batch_id IN (SELECT id FROM payout_batches WHERE status = 'cancelled');
DELETE FROM payout_batches WHERE status = 'cancelled';
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_approved_chk;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_check2
  CHECK (status IN ('draft', 'pending_approval') OR (approved_by IS NOT NULL AND approved_at IS NOT NULL));
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_status_check;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_status_check
  CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported', 'settled'));

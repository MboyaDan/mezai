-- Stage 15a: M-Pesa B2C payouts from approved batches (brief 13, DG-02).
--
-- An approved batch is either exported for the Business Portal (Stage 14)
-- or disbursed through Daraja B2C: approved -> disbursing -> settled. Each
-- payout line gets at most one B2C transaction, ever (unique line), and its
-- OriginatorConversationID is the idempotency key Safaricom sees. A request
-- leaves only from 'queued' after the row moves to 'sending' in its own
-- commit, so a crash or a timeout can never send it twice: an unclear
-- outcome becomes 'unknown' and is settled by a status query. A failed line
-- moves no money and the amount stays owed for the next batch (D-132).
--
-- Every callback is stored, verified or not matched, before it is processed.

-- +goose Up
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_status_check;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_status_check
  CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported', 'disbursing', 'settled', 'cancelled'));
ALTER TABLE payout_batches
  ADD COLUMN disbursed_at timestamptz,
  ADD COLUMN disbursed_by uuid REFERENCES users (id);
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_export_chk;
ALTER TABLE payout_batches
  ADD CONSTRAINT payout_batches_export_chk
    CHECK (status <> 'exported' OR (exported_at IS NOT NULL AND export_file_key IS NOT NULL AND export_checksum IS NOT NULL)),
  ADD CONSTRAINT payout_batches_disburse_chk
    CHECK (status <> 'disbursing' OR (disbursed_at IS NOT NULL AND disbursed_by IS NOT NULL)),
  -- A batch goes out one way only.
  ADD CONSTRAINT payout_batches_one_way_chk CHECK (export_checksum IS NULL OR disbursed_at IS NULL),
  ADD CONSTRAINT payout_batches_settled_way_chk
    CHECK (status <> 'settled' OR export_checksum IS NOT NULL OR disbursed_at IS NOT NULL);

-- A line paid or failed by a B2C result has no person behind it.
ALTER TABLE payout_lines ADD COLUMN recorded_via text NOT NULL DEFAULT 'manual'
  CHECK (recorded_via IN ('manual', 'mpesa_b2c'));
ALTER TABLE payout_lines DROP CONSTRAINT payout_lines_check2;
ALTER TABLE payout_lines ADD CONSTRAINT payout_lines_recorded_chk
  CHECK (status = 'pending' OR (recorded_at IS NOT NULL AND (recorded_by IS NOT NULL OR recorded_via = 'mpesa_b2c')));

CREATE TABLE mpesa_transactions (
  id                         uuid PRIMARY KEY DEFAULT uuid_v7(),
  payout_line_id             uuid NOT NULL UNIQUE REFERENCES payout_lines (id),  -- one request per line, ever
  originator_conversation_id text NOT NULL UNIQUE CHECK (originator_conversation_id ~ '^[A-Za-z0-9-]{8,64}$'),
  conversation_id            text UNIQUE,
  environment                text NOT NULL CHECK (environment IN ('fake', 'sandbox', 'production')),
  amount_cents               bigint NOT NULL CHECK (amount_cents > 0 AND amount_cents % 100 = 0),
  currency                   char(3) NOT NULL CHECK (currency = 'KES'),
  status                     text NOT NULL DEFAULT 'queued'
                               CHECK (status IN ('queued', 'sending', 'accepted', 'unknown', 'succeeded', 'failed')),
  send_attempts              integer NOT NULL DEFAULT 0 CHECK (send_attempts >= 0),
  sent_at                    timestamptz,
  response_code              text,
  response_description       text CHECK (length(response_description) <= 500),
  result_code                text,
  result_desc                text CHECK (length(result_desc) <= 500),
  receipt                    text UNIQUE CHECK (receipt ~ '^[A-Z0-9]{8,12}$'),
  status_queries             integer NOT NULL DEFAULT 0 CHECK (status_queries >= 0),
  not_found_count            integer NOT NULL DEFAULT 0 CHECK (not_found_count >= 0),
  completed_at               timestamptz,
  created_by                 uuid NOT NULL REFERENCES users (id),
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_at                 timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'succeeded' OR receipt IS NOT NULL),
  CHECK (status NOT IN ('succeeded', 'failed') OR completed_at IS NOT NULL),
  CHECK (status = 'queued' OR sent_at IS NOT NULL OR (status = 'failed' AND send_attempts = 0))
);
CREATE INDEX mpesa_transactions_open_idx ON mpesa_transactions (status, updated_at)
  WHERE status NOT IN ('succeeded', 'failed');
CREATE TRIGGER mpesa_transactions_updated_at BEFORE UPDATE ON mpesa_transactions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A final result never changes; the amount and line never change.
-- +goose StatementBegin
CREATE FUNCTION mpesa_transactions_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.payout_line_id <> OLD.payout_line_id OR NEW.amount_cents <> OLD.amount_cents
     OR NEW.originator_conversation_id <> OLD.originator_conversation_id THEN
    RAISE EXCEPTION 'mpesa transaction % cannot change its line, amount or id', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.status IN ('succeeded', 'failed') AND (NEW.status <> OLD.status OR NEW.receipt IS DISTINCT FROM OLD.receipt) THEN
    RAISE EXCEPTION 'mpesa transaction % already has a result', OLD.id USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER mpesa_transactions_guard BEFORE UPDATE ON mpesa_transactions
  FOR EACH ROW EXECUTE FUNCTION mpesa_transactions_guard();

CREATE TABLE mpesa_callbacks (
  id                         uuid PRIMARY KEY DEFAULT uuid_v7(),
  kind                       text NOT NULL CHECK (kind IN ('b2c_result', 'b2c_timeout', 'status_result', 'status_timeout')),
  originator_conversation_id text,
  conversation_id            text,
  transaction_id             uuid REFERENCES mpesa_transactions (id),
  result_code                text,
  -- The body as received. It holds the payee's phone and name: staff only,
  -- never logged, covered by the Stage 17 retention job.
  body                       jsonb NOT NULL,
  remote_ip                  inet,
  verdict                    text NOT NULL CHECK (verdict IN ('matched', 'unmatched', 'mismatch')),
  note                       text CHECK (length(note) <= 500),
  processed_at               timestamptz,
  process_error              text CHECK (length(process_error) <= 1000),
  created_at                timestamptz NOT NULL DEFAULT now(),
  CHECK (verdict = 'unmatched' OR transaction_id IS NOT NULL)
);
CREATE INDEX mpesa_callbacks_txn_idx ON mpesa_callbacks (transaction_id, created_at);
CREATE INDEX mpesa_callbacks_pending_idx ON mpesa_callbacks (created_at) WHERE processed_at IS NULL AND verdict = 'matched';
CREATE INDEX mpesa_callbacks_exceptions_idx ON mpesa_callbacks (created_at DESC) WHERE verdict <> 'matched';

GRANT SELECT, INSERT, UPDATE ON mpesa_transactions, mpesa_callbacks TO mezai_app;

-- +goose Down
DROP TABLE mpesa_callbacks;
DROP TRIGGER mpesa_transactions_guard ON mpesa_transactions;
DROP FUNCTION mpesa_transactions_guard();
DROP TABLE mpesa_transactions;
UPDATE payout_lines SET recorded_by = (SELECT disbursed_by FROM payout_batches b WHERE b.id = payout_lines.batch_id)
  WHERE recorded_via = 'mpesa_b2c';
ALTER TABLE payout_lines DROP CONSTRAINT payout_lines_recorded_chk;
ALTER TABLE payout_lines ADD CONSTRAINT payout_lines_check2
  CHECK (status = 'pending' OR (recorded_by IS NOT NULL AND recorded_at IS NOT NULL));
ALTER TABLE payout_lines DROP COLUMN recorded_via;
ALTER TABLE payout_batches
  DROP CONSTRAINT payout_batches_settled_way_chk, DROP CONSTRAINT payout_batches_one_way_chk,
  DROP CONSTRAINT payout_batches_disburse_chk, DROP CONSTRAINT payout_batches_export_chk;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_export_chk
  CHECK (status NOT IN ('exported', 'settled') OR (exported_at IS NOT NULL AND export_file_key IS NOT NULL AND export_checksum IS NOT NULL));
ALTER TABLE payout_batches DROP COLUMN disbursed_by, DROP COLUMN disbursed_at;
ALTER TABLE payout_batches DROP CONSTRAINT payout_batches_status_check;
ALTER TABLE payout_batches ADD CONSTRAINT payout_batches_status_check
  CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported', 'settled', 'cancelled'));

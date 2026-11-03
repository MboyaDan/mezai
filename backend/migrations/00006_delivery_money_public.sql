-- Deliverables, earnings and payouts, enquiries and public stats
-- (brief 6.1, 6.3, 8.5, 13). mpesa_transactions arrives with DG-02 (Stage 15).

-- +goose Up

CREATE TABLE deliverables (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  project_id     uuid NOT NULL,
  format         text NOT NULL CHECK (format IN ('jsonl', 'csv')),
  file_key       text NOT NULL,
  manifest_key   text NOT NULL,
  manifest       jsonb NOT NULL,
  checksum_sha256 text NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
  schema_version integer NOT NULL CHECK (schema_version > 0),
  item_count     integer NOT NULL CHECK (item_count >= 0),
  published_at   timestamptz,
  published_by   uuid REFERENCES users (id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  UNIQUE (id, org_id),
  CHECK ((published_at IS NULL) = (published_by IS NULL))
);
CREATE INDEX deliverables_project_idx ON deliverables (project_id, created_at DESC);

CREATE TABLE deliverable_downloads (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id         uuid NOT NULL,
  deliverable_id uuid NOT NULL,
  user_id        uuid NOT NULL REFERENCES users (id),
  ip             inet,
  created_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (deliverable_id, org_id) REFERENCES deliverables (id, org_id)
);
CREATE INDEX deliverable_downloads_idx ON deliverable_downloads (deliverable_id, created_at DESC);

CREATE TABLE payout_batches (
  id              uuid PRIMARY KEY DEFAULT uuid_v7(),
  period_start    date NOT NULL,
  period_end      date NOT NULL,
  currency        char(3) NOT NULL DEFAULT 'KES' CHECK (currency ~ '^[A-Z]{3}$'),
  status          text NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported', 'settled')),
  total_cents     bigint NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
  line_count      integer NOT NULL DEFAULT 0 CHECK (line_count >= 0),
  idempotency_key text NOT NULL UNIQUE,
  created_by      uuid NOT NULL REFERENCES users (id),
  approved_by     uuid REFERENCES users (id),
  approved_at     timestamptz,
  exported_at     timestamptz,
  export_file_key text,
  settled_at      timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (period_end >= period_start),
  -- Two-person rule (brief 4, 13): the creator can never approve.
  CHECK (approved_by IS NULL OR approved_by <> created_by),
  CHECK (status IN ('draft', 'pending_approval') OR (approved_by IS NOT NULL AND approved_at IS NOT NULL))
);

CREATE TABLE payout_lines (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  batch_id       uuid NOT NULL REFERENCES payout_batches (id),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  amount_cents   bigint NOT NULL CHECK (amount_cents > 0),
  currency       char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  mpesa_phone    text NOT NULL CHECK (mpesa_phone ~ '^\+254[17][0-9]{8}$'),   -- snapshot at batch time
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed')),
  mpesa_receipt  text,
  failure_reason text,
  recorded_by    uuid REFERENCES users (id),
  recorded_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (batch_id, contributor_id),
  CHECK (status <> 'paid' OR mpesa_receipt IS NOT NULL),
  CHECK (status <> 'failed' OR failure_reason IS NOT NULL),
  CHECK (status = 'pending' OR (recorded_by IS NOT NULL AND recorded_at IS NOT NULL))
);
CREATE UNIQUE INDEX payout_lines_receipt_idx ON payout_lines (mpesa_receipt) WHERE mpesa_receipt IS NOT NULL;

-- Append-only ledger. Balances are sums; corrections are compensating entries
-- with a reason. Signs: accrual and payout_reversal positive, payout negative.
CREATE TABLE earnings_ledger (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  entry_type     text NOT NULL CHECK (entry_type IN ('accrual', 'payout', 'payout_reversal', 'adjustment')),
  amount_cents   bigint NOT NULL CHECK (amount_cents <> 0),
  currency       char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  project_id     uuid REFERENCES projects (id),
  submission_id  uuid REFERENCES submissions (id),
  payout_line_id uuid REFERENCES payout_lines (id),
  reason         text,
  created_by     uuid REFERENCES users (id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (entry_type <> 'accrual' OR (amount_cents > 0 AND submission_id IS NOT NULL)),
  CHECK (entry_type <> 'payout' OR (amount_cents < 0 AND payout_line_id IS NOT NULL)),
  CHECK (entry_type <> 'payout_reversal' OR (amount_cents > 0 AND payout_line_id IS NOT NULL)),
  CHECK (entry_type <> 'adjustment' OR (reason IS NOT NULL AND created_by IS NOT NULL))
);
-- A submission earns once; a payout line is paid and reversed at most once.
CREATE UNIQUE INDEX earnings_ledger_accrual_idx ON earnings_ledger (submission_id) WHERE entry_type = 'accrual';
CREATE UNIQUE INDEX earnings_ledger_payout_idx ON earnings_ledger (payout_line_id, entry_type) WHERE payout_line_id IS NOT NULL;
CREATE INDEX earnings_ledger_contributor_idx ON earnings_ledger (contributor_id, created_at);

CREATE TABLE enquiries (
  id                uuid PRIMARY KEY DEFAULT uuid_v7(),
  name              text NOT NULL,
  organisation_name text NOT NULL,
  email             text NOT NULL,
  project_type      text NOT NULL,
  languages         text NOT NULL DEFAULT '',
  volume_range      text NOT NULL DEFAULT '',
  timeline          text NOT NULL DEFAULT '',
  message           text NOT NULL DEFAULT '',
  status            text NOT NULL DEFAULT 'new'
                      CHECK (status IN ('new', 'assigned', 'in_conversation', 'won', 'lost', 'spam')),
  assigned_to       uuid REFERENCES users (id),
  org_id            uuid REFERENCES organisations (id),   -- set when won and linked (brief 9)
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  CHECK (org_id IS NULL OR status = 'won')
);
CREATE INDEX enquiries_status_idx ON enquiries (status, created_at DESC);

CREATE TABLE enquiry_notes (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  enquiry_id     uuid NOT NULL REFERENCES enquiries (id) ON DELETE CASCADE,
  author_user_id uuid NOT NULL REFERENCES users (id),
  body           text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Public statistics: no record, no number (honesty rule 2).
CREATE TABLE stats (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  key         text NOT NULL UNIQUE,
  label       text NOT NULL,
  value       numeric NOT NULL,
  unit        text NOT NULL DEFAULT '',
  source      text NOT NULL CHECK (length(source) > 0),
  source_url  text,
  verified_at timestamptz NOT NULL,
  published   boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER deliverables_updated_at BEFORE UPDATE ON deliverables FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER payout_batches_updated_at BEFORE UPDATE ON payout_batches FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER payout_lines_updated_at BEFORE UPDATE ON payout_lines FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER enquiries_updated_at BEFORE UPDATE ON enquiries FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER enquiry_notes_updated_at BEFORE UPDATE ON enquiry_notes FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER stats_updated_at BEFORE UPDATE ON stats FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE stats, enquiry_notes, enquiries, earnings_ledger, payout_lines, payout_batches, deliverable_downloads, deliverables;

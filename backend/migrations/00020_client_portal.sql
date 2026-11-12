-- Client portal (Stage 13a, brief 6.3, AC-10): invoices that Ops uploads as
-- PDFs, and the permissions to manage and read them.
--
-- Invoices are issued outside the platform in the pilot (brief 3); MezAI
-- only keeps the PDF and its few facts so the client finds them in one place.
-- Client Admins read them (org.invoice.read, D-122); Client Viewers do not.

-- +goose Up
INSERT INTO permissions (key, description, grantable) VALUES
  ('invoice.manage', 'Upload, mark paid and void client invoices', true),
  ('org.invoice.read', 'Read and download own organisation''s invoices', true);
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE (p.key = 'invoice.manage' AND r.key IN ('super_admin', 'ops_manager'))
   OR (p.key = 'org.invoice.read' AND r.key IN ('super_admin', 'client_admin'));

CREATE TABLE invoices (
  id              uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id          uuid NOT NULL REFERENCES organisations (id),
  project_id      uuid,
  number          text NOT NULL CHECK (length(number) BETWEEN 1 AND 60),
  issued_on       date NOT NULL,
  due_on          date CHECK (due_on >= issued_on),
  amount_cents    bigint NOT NULL CHECK (amount_cents >= 0),
  currency        char(3) NOT NULL DEFAULT 'KES' CHECK (currency ~ '^[A-Z]{3}$'),
  description     text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  file_key        text NOT NULL,
  file_name       text NOT NULL CHECK (length(file_name) BETWEEN 1 AND 200),
  byte_size       bigint NOT NULL CHECK (byte_size > 0),
  checksum_sha256 text NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
  status          text NOT NULL DEFAULT 'issued' CHECK (status IN ('issued', 'paid', 'void')),
  paid_on         date,
  void_reason     text CHECK (length(void_reason) BETWEEN 1 AND 500),
  uploaded_by     uuid NOT NULL REFERENCES users (id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (project_id, org_id) REFERENCES projects (id, org_id),
  UNIQUE (org_id, number),
  CHECK ((status = 'paid') = (paid_on IS NOT NULL)),
  CHECK ((status = 'void') = (void_reason IS NOT NULL))
);
CREATE INDEX invoices_org_idx ON invoices (org_id, issued_on DESC, created_at DESC);
CREATE TRIGGER invoices_updated_at BEFORE UPDATE ON invoices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A client reads its own organisation's invoices; only staff write.
-- The permission check (org.invoice.read) is in the API.
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
CREATE POLICY invoices_read ON invoices FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY invoices_write ON invoices FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());
GRANT SELECT, INSERT, UPDATE ON invoices TO mezai_app;

-- +goose Down
DROP TABLE invoices;
DELETE FROM role_permissions WHERE permission_id IN
  (SELECT id FROM permissions WHERE key IN ('invoice.manage', 'org.invoice.read'));
DELETE FROM permissions WHERE key IN ('invoice.manage', 'org.invoice.read');

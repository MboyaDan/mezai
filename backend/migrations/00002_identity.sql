-- Identity and access (brief section 4 and 10). Authorisation is by
-- permission key; roles are bundles of permissions held in these tables.

-- +goose Up

CREATE TABLE users (
  id                 uuid PRIMARY KEY DEFAULT uuid_v7(),
  email              text NOT NULL CHECK (email = lower(email) AND position('@' IN email) > 1),
  display_name       text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 120),
  password_hash      text,          -- Argon2id PHC string; null until an invitation is accepted
  email_verified_at  timestamptz,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  failed_login_count integer NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  locked_until       timestamptz,
  last_login_at      timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (email)
);

CREATE TABLE sessions (
  id           uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash   bytea NOT NULL UNIQUE,   -- SHA-256 of the cookie value; the token itself is never stored
  csrf_secret  bytea NOT NULL,
  ip           inet,
  user_agent   text,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz NOT NULL,
  revoked_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE permissions (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  key         text NOT NULL UNIQUE CHECK (key ~ '^[a-z]+(\.[a-z_]+)+$'),
  description text NOT NULL,
  grantable   boolean NOT NULL DEFAULT true,  -- false: never assignable through the UI (system.admin)
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  key         text NOT NULL UNIQUE,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  scope       text NOT NULL CHECK (scope IN ('platform', 'org')),  -- org roles are held per organisation
  is_system   boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
  role_id       uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
  permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
  created_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (role_id, permission_id)
);

-- Organisations are created in 00003; org_id gets its foreign key there.
CREATE TABLE user_roles (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role_id    uuid NOT NULL REFERENCES roles (id),
  org_id     uuid,
  granted_by uuid REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (user_id, role_id, org_id)
);
CREATE INDEX user_roles_org_idx ON user_roles (org_id) WHERE org_id IS NOT NULL;

CREATE TABLE invitations (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  email       text NOT NULL CHECK (email = lower(email)),
  role_id     uuid NOT NULL REFERENCES roles (id),
  org_id      uuid,
  token_hash  bytea NOT NULL UNIQUE,
  invited_by  uuid NOT NULL REFERENCES users (id),
  expires_at  timestamptz NOT NULL,
  accepted_at timestamptz,
  revoked_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_email_idx ON invitations (email) WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE TABLE password_resets (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Consent records (Kenya DPA 2019): append-only, one row per acceptance.
CREATE TABLE consents (
  id          uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id     uuid NOT NULL REFERENCES users (id),
  document    text NOT NULL CHECK (document IN ('privacy_notice', 'terms_of_use', 'contributor_terms', 'acceptable_use')),
  version     text NOT NULL,
  accepted_at timestamptz NOT NULL DEFAULT now(),
  ip          inet,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX consents_user_idx ON consents (user_id, document, accepted_at DESC);

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER sessions_updated_at BEFORE UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER permissions_updated_at BEFORE UPDATE ON permissions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER roles_updated_at BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER user_roles_updated_at BEFORE UPDATE ON user_roles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER invitations_updated_at BEFORE UPDATE ON invitations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER password_resets_updated_at BEFORE UPDATE ON password_resets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Permissions and roles every environment needs (production included).
INSERT INTO permissions (key, description, grantable) VALUES
  ('system.admin',             'Full platform administration', false),
  ('settings.manage',          'Change platform settings', true),
  ('role.manage',              'Configure roles and permissions', true),
  ('user.manage',              'Invite, disable and assign roles to staff', true),
  ('audit.read',               'Read the audit log', true),
  ('org.manage',               'Create and manage client organisations', true),
  ('project.manage',           'Create, configure, publish, pause and close projects', true),
  ('item.import',              'Import items', true),
  ('gold.manage',              'Create, import and retire gold items', true),
  ('contributor.approve',      'Approve or reject contributors and qualification attempts', true),
  ('contributor.pause',        'Pause, reinstate or remove contributors', true),
  ('review.read',              'Read the review queue on assigned projects', true),
  ('review.decide',            'Accept or reject submissions on assigned projects', true),
  ('review.adjudicate',        'Write a final answer for an item', true),
  ('feedback.write',           'Send feedback to contributors', true),
  ('deliverable.publish',      'Export and publish deliverables', true),
  ('payout.create',            'Create payout batches and record results', true),
  ('payout.approve',           'Approve payout batches created by someone else', true),
  ('enquiry.manage',           'Manage the enquiries inbox', true),
  ('task.submit',              'Take and submit tasks', true),
  ('qualification.take',       'Take qualification tests', true),
  ('org.project.read',         'Read own organisation''s projects, quality and activity', true),
  ('org.project.request',      'Submit project requests for own organisation', true),
  ('org.deliverable.download', 'Download own organisation''s deliverables', true),
  ('org.user.invite',          'Invite users to own organisation', true);

INSERT INTO roles (key, name, description, scope, is_system) VALUES
  ('super_admin',     'Super Admin',     'Everything, including system administration, roles and settings', 'platform', true),
  ('ops_manager',     'Ops Manager',     'ELS operations staff', 'platform', true),
  ('payout_approver', 'Payout Approver', 'Second person on payout batches; cannot approve a batch they created', 'platform', true),
  ('reviewer',        'Reviewer',        'Reviews submissions on assigned projects', 'platform', true),
  ('contributor',     'Contributor',     'Vetted contributor who takes tasks', 'platform', true),
  ('client_admin',    'Client Admin',    'Buyer''s project lead (own organisation only)', 'org', true),
  ('client_viewer',   'Client Viewer',   'Read-only client user (own organisation only)', 'org', true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.key = 'super_admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM (VALUES
  ('ops_manager', 'audit.read'), ('ops_manager', 'user.manage'), ('ops_manager', 'org.manage'),
  ('ops_manager', 'project.manage'), ('ops_manager', 'item.import'), ('ops_manager', 'gold.manage'),
  ('ops_manager', 'contributor.approve'), ('ops_manager', 'contributor.pause'),
  ('ops_manager', 'review.read'), ('ops_manager', 'review.decide'), ('ops_manager', 'review.adjudicate'),
  ('ops_manager', 'feedback.write'), ('ops_manager', 'deliverable.publish'),
  ('ops_manager', 'payout.create'), ('ops_manager', 'enquiry.manage'),
  ('payout_approver', 'payout.approve'),
  ('reviewer', 'review.read'), ('reviewer', 'review.decide'), ('reviewer', 'feedback.write'),
  ('contributor', 'task.submit'), ('contributor', 'qualification.take'),
  ('client_admin', 'org.project.read'), ('client_admin', 'org.project.request'),
  ('client_admin', 'org.deliverable.download'), ('client_admin', 'org.user.invite'),
  ('client_viewer', 'org.project.read'), ('client_viewer', 'org.deliverable.download')
) AS g (role_key, perm_key)
JOIN roles r ON r.key = g.role_key
JOIN permissions p ON p.key = g.perm_key;

-- +goose Down
DROP TABLE consents, password_resets, invitations, user_roles, role_permissions, roles, permissions, sessions, users;

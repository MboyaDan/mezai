-- Organisations (client tenants) and contributors.

-- +goose Up

CREATE TABLE organisations (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  slug       text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE org_members (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  org_id     uuid NOT NULL REFERENCES organisations (id),
  user_id    uuid NOT NULL REFERENCES users (id),
  job_title  text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, user_id)
);
CREATE INDEX org_members_user_idx ON org_members (user_id);

ALTER TABLE user_roles ADD FOREIGN KEY (org_id) REFERENCES organisations (id);
ALTER TABLE invitations ADD FOREIGN KEY (org_id) REFERENCES organisations (id);

-- public_code is the only contributor identifier clients ever see (brief 6.3).
CREATE TABLE contributor_profiles (
  id                       uuid PRIMARY KEY DEFAULT uuid_v7(),
  user_id                  uuid NOT NULL UNIQUE REFERENCES users (id),
  public_code              text NOT NULL UNIQUE CHECK (public_code ~ '^C-[0-9]{4,}$'),
  status                   text NOT NULL DEFAULT 'applied'
                             CHECK (status IN ('applied', 'qualifying', 'active', 'paused', 'removed')),
  country                  char(2) CHECK (country ~ '^[A-Z]{2}$'),
  city                     text,
  education_level          text CHECK (education_level IN ('secondary', 'certificate', 'diploma', 'bachelor', 'master', 'doctorate', 'other')),
  weekly_hours             smallint CHECK (weekly_hours BETWEEN 0 AND 80),
  mpesa_phone              text CHECK (mpesa_phone ~ '^\+254[17][0-9]{8}$'),  -- E.164, Kenyan mobile
  mpesa_phone_confirmed_at timestamptz,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contributor_profiles_status_idx ON contributor_profiles (status);
CREATE SEQUENCE contributor_public_code_seq START 1;

CREATE TABLE contributor_languages (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id) ON DELETE CASCADE,
  language       text NOT NULL CHECK (language ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$'),  -- BCP 47, e.g. sw, en, sw-KE
  proficiency    text NOT NULL CHECK (proficiency IN ('native', 'fluent', 'working')),
  dialects       text[] NOT NULL DEFAULT '{}',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (contributor_id, language)
);
CREATE INDEX contributor_languages_lang_idx ON contributor_languages (language, proficiency);

CREATE TABLE contributor_domains (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id) ON DELETE CASCADE,
  domain         text NOT NULL CHECK (domain IN ('health', 'law', 'finance', 'education', 'software', 'agriculture', 'other')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (contributor_id, domain)
);

-- A skill is (kind, value): a language code, a domain or a task type.
CREATE TABLE qualification_tests (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  title          text NOT NULL,
  skill_kind     text NOT NULL CHECK (skill_kind IN ('language', 'domain', 'task_type')),
  skill_value    text NOT NULL,
  task_type      text NOT NULL CHECK (task_type IN ('classification', 'pairwise', 'translation_review')),
  pass_threshold numeric(4, 3) NOT NULL CHECK (pass_threshold BETWEEN 0 AND 1),
  instructions_md text NOT NULL DEFAULT '',
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'retired')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Questions are copies of gold items (brief 6.2) kept apart from client data,
-- so a qualification test never exposes a client's items.
CREATE TABLE qualification_questions (
  id                  uuid PRIMARY KEY DEFAULT uuid_v7(),
  test_id             uuid NOT NULL REFERENCES qualification_tests (id) ON DELETE CASCADE,
  position            integer NOT NULL CHECK (position > 0),
  content             jsonb NOT NULL,
  expected            jsonb NOT NULL,
  tolerance           jsonb NOT NULL DEFAULT '{}',
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (test_id, position)
);

CREATE TABLE qualification_attempts (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  test_id        uuid NOT NULL REFERENCES qualification_tests (id),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  status         text NOT NULL DEFAULT 'in_progress'
                   CHECK (status IN ('in_progress', 'scored', 'approved', 'rejected')),
  answers        jsonb NOT NULL DEFAULT '{}',
  score          numeric(5, 4) CHECK (score BETWEEN 0 AND 1),
  passed         boolean,
  started_at     timestamptz NOT NULL DEFAULT now(),
  submitted_at   timestamptz,
  decided_by     uuid REFERENCES users (id),
  decided_at     timestamptz,
  decision_note  text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (status = 'in_progress' OR submitted_at IS NOT NULL),
  CHECK (status NOT IN ('approved', 'rejected') OR (decided_by IS NOT NULL AND decided_at IS NOT NULL))
);
CREATE INDEX qualification_attempts_contributor_idx ON qualification_attempts (contributor_id, test_id);
CREATE INDEX qualification_attempts_queue_idx ON qualification_attempts (submitted_at) WHERE status = 'scored';

-- Append-only (brief 9: every transition audited with who, when and why).
CREATE TABLE contributor_status_history (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  from_status    text,
  to_status      text NOT NULL,
  changed_by     uuid REFERENCES users (id),   -- null when the system changed it (trust auto-pause)
  reason         text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contributor_status_history_idx ON contributor_status_history (contributor_id, created_at DESC);

CREATE TRIGGER organisations_updated_at BEFORE UPDATE ON organisations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER org_members_updated_at BEFORE UPDATE ON org_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER contributor_profiles_updated_at BEFORE UPDATE ON contributor_profiles FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER contributor_languages_updated_at BEFORE UPDATE ON contributor_languages FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER contributor_domains_updated_at BEFORE UPDATE ON contributor_domains FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER qualification_tests_updated_at BEFORE UPDATE ON qualification_tests FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER qualification_questions_updated_at BEFORE UPDATE ON qualification_questions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER qualification_attempts_updated_at BEFORE UPDATE ON qualification_attempts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE contributor_status_history, qualification_attempts, qualification_questions, qualification_tests,
  contributor_domains, contributor_languages, contributor_profiles;
DROP SEQUENCE contributor_public_code_seq;
ALTER TABLE invitations DROP CONSTRAINT invitations_org_id_fkey;
ALTER TABLE user_roles DROP CONSTRAINT user_roles_org_id_fkey;
DROP TABLE org_members, organisations;

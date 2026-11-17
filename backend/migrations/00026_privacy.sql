-- Privacy (Stage 17b, brief 12): the retention rule for client task content
-- and the contributor's rights to a copy of their data and to erasure.
--
-- Retention: a closed project's task content and answers are blanked
-- `retention_days` after it closes (default 90, 0 = at the next sweep), or at
-- once when Ops acts on a client's request. Rows stay so counts, quality
-- figures, earnings and the audit trail still add up; only content goes.
--
-- Erasure: a contributor's identifying data is removed and the account
-- closed. Ledger, payout and audit rows are kept for 7 years (brief 12) and
-- now point at an anonymous record (public code only).
--
-- Both run through SECURITY DEFINER functions because several of the tables
-- are append-only for the application (00007). The functions only blank
-- content; they cannot change amounts, decisions or who did what.

-- +goose Up
ALTER TABLE projects ADD COLUMN content_purged_at timestamptz;
ALTER TABLE projects ADD CONSTRAINT projects_purged_closed_chk CHECK (content_purged_at IS NULL OR status = 'closed');
CREATE INDEX projects_retention_idx ON projects (closed_at) WHERE status = 'closed' AND content_purged_at IS NULL;
ALTER TABLE deliverables ADD COLUMN purged_at timestamptz;
ALTER TABLE import_jobs ADD COLUMN purged_at timestamptz;
ALTER TABLE contributor_profiles ADD COLUMN erased_at timestamptz;
ALTER TABLE contributor_profiles ADD CONSTRAINT contributor_profiles_erased_chk CHECK (erased_at IS NULL OR status = 'removed');

-- A payout line keeps its amount and receipt after erasure; the payee's
-- number and name become 'erased'.
ALTER TABLE payout_lines DROP CONSTRAINT payout_lines_mpesa_phone_check;
ALTER TABLE payout_lines ADD CONSTRAINT payout_lines_mpesa_phone_check
  CHECK (mpesa_phone ~ '^\+254[17][0-9]{8}$' OR mpesa_phone = 'erased');

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION payout_lines_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  -- erase_contributor() may blank the payee and nothing else.
  IF current_setting('mezai.erasing', true) = OLD.contributor_id::text THEN
    IF NEW.batch_id <> OLD.batch_id OR NEW.contributor_id <> OLD.contributor_id
       OR NEW.amount_cents <> OLD.amount_cents OR NEW.currency <> OLD.currency
       OR NEW.status <> OLD.status OR NEW.mpesa_receipt IS DISTINCT FROM OLD.mpesa_receipt
       OR NEW.mpesa_phone <> 'erased' THEN
      RAISE EXCEPTION 'erasure may only blank the payee of line %', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
  END IF;
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

-- Object storage keys waiting to be deleted. The database change and the
-- queued key commit together; a sweep deletes the objects and retries.
CREATE TABLE storage_deletions (
  id         uuid PRIMARY KEY DEFAULT uuid_v7(),
  key        text NOT NULL UNIQUE,
  reason     text NOT NULL CHECK (reason IN ('retention', 'erasure', 'request_declined')),
  attempts   integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error text CHECK (length(last_error) <= 500),
  created_at timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT, INSERT, UPDATE, DELETE ON storage_deletions TO mezai_app;

-- A contributor's request to delete their account. Ops carries it out once
-- nothing is owed in either direction (A-088).
CREATE TABLE erasure_requests (
  id             uuid PRIMARY KEY DEFAULT uuid_v7(),
  contributor_id uuid NOT NULL REFERENCES contributor_profiles (id),
  status         text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'withdrawn', 'done')),
  requested_by   uuid NOT NULL REFERENCES users (id),
  note           text CHECK (length(note) <= 1000),
  closed_by      uuid REFERENCES users (id),
  closed_at      timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'open') = (closed_at IS NULL))
);
CREATE UNIQUE INDEX erasure_requests_one_open ON erasure_requests (contributor_id) WHERE status = 'open';
CREATE INDEX erasure_requests_open_idx ON erasure_requests (created_at) WHERE status = 'open';
CREATE TRIGGER erasure_requests_updated_at BEFORE UPDATE ON erasure_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();
GRANT SELECT, INSERT, UPDATE ON erasure_requests TO mezai_app;

INSERT INTO permissions (key, description, grantable) VALUES
  ('privacy.manage', 'Delete task content early, export and erase a contributor''s personal data', true);
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE p.key = 'privacy.manage' AND r.key IN ('super_admin', 'ops_manager');

-- +goose StatementBegin
-- purge_project_content blanks a closed project's task content and answers
-- and queues its stored files for deletion. It returns row counts.
CREATE FUNCTION purge_project_content(p_project uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path = public AS $$
DECLARE
  pr       projects%ROWTYPE;
  n_items  bigint;
  n_answer bigint;
  n_final  bigint;
  n_files  bigint;
  n_tests  bigint;
  removed  constant text := 'Removed under the project''s retention rule.';
BEGIN
  SELECT * INTO pr FROM projects WHERE id = p_project FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'project % not found', p_project USING ERRCODE = 'no_data_found';
  END IF;
  IF pr.status <> 'closed' THEN
    RAISE EXCEPTION 'project % is not closed', p_project USING ERRCODE = 'check_violation';
  END IF;
  IF pr.content_purged_at IS NOT NULL THEN
    RETURN jsonb_build_object('already', true);
  END IF;

  -- Files: deliverables, import uploads and the request's sample.
  INSERT INTO storage_deletions (key, reason)
  SELECT k, 'retention' FROM (
    SELECT file_key AS k FROM deliverables WHERE project_id = p_project
    UNION SELECT manifest_key FROM deliverables WHERE project_id = p_project
    UNION SELECT file_key FROM import_jobs WHERE project_id = p_project
    UNION SELECT sample_file_key FROM project_requests WHERE project_id = p_project AND sample_file_key IS NOT NULL
  ) f ON CONFLICT (key) DO NOTHING;
  GET DIAGNOSTICS n_files = ROW_COUNT;
  UPDATE deliverables SET purged_at = now() WHERE project_id = p_project;
  UPDATE import_jobs SET purged_at = now() WHERE project_id = p_project;
  DELETE FROM import_errors e USING import_jobs j WHERE j.id = e.import_job_id AND j.project_id = p_project;
  UPDATE project_requests SET sample_file_key = NULL, sample_file_name = NULL, sample_size = NULL
  WHERE project_id = p_project AND sample_file_key IS NOT NULL;

  -- Content and answers.
  UPDATE items SET content = '{"purged": true}', external_id = NULL WHERE project_id = p_project;
  GET DIAGNOSTICS n_items = ROW_COUNT;
  UPDATE item_revisions r SET answer = '{}', reason = CASE WHEN r.reason IS NULL THEN NULL ELSE removed END
  FROM items i WHERE i.id = r.item_id AND i.project_id = p_project;
  GET DIAGNOSTICS n_final = ROW_COUNT;
  UPDATE submission_revisions r SET answer = '{}'
  FROM submissions s WHERE s.id = r.submission_id AND s.project_id = p_project;
  GET DIAGNOSTICS n_answer = ROW_COUNT;
  UPDATE submissions SET rejection_reason = removed, qa_warnings = '[]'
  WHERE project_id = p_project AND (rejection_reason IS NOT NULL OR qa_warnings <> '[]');
  UPDATE gold_items SET expected = '{}', tolerance = '{}' WHERE project_id = p_project;
  UPDATE gold_results r SET detail = '{}' FROM gold_items g WHERE g.id = r.gold_item_id AND g.project_id = p_project;
  UPDATE consensus_results SET result = '{}' WHERE project_id = p_project;
  UPDATE reviews SET final_answer = CASE WHEN final_answer IS NULL THEN NULL ELSE '{}'::jsonb END,
                     reason = CASE WHEN reason IS NULL THEN NULL ELSE removed END
  WHERE project_id = p_project;
  UPDATE feedback SET body = removed WHERE project_id = p_project;
  UPDATE skips k SET note = NULL FROM assignments a WHERE a.id = k.assignment_id AND a.project_id = p_project AND k.note IS NOT NULL;
  UPDATE content_reports c SET note = NULL FROM items i WHERE i.id = c.item_id AND i.project_id = p_project AND c.note IS NOT NULL;

  -- Qualification tests made from this project's gold hold copies of its
  -- items: they are retired and their questions blanked (A-089).
  UPDATE qualification_tests SET status = 'retired' WHERE source_project_id = p_project AND status <> 'retired';
  UPDATE qualification_questions q SET content = '{"purged": true}', expected = '{}', tolerance = '{}'
  FROM qualification_tests t WHERE t.id = q.test_id AND t.source_project_id = p_project;
  GET DIAGNOSTICS n_tests = ROW_COUNT;
  UPDATE qualification_attempts a SET answers = '{}', results = '{}'
  FROM qualification_tests t WHERE t.id = a.test_id AND t.source_project_id = p_project;

  UPDATE projects SET content_purged_at = now() WHERE id = p_project;
  RETURN jsonb_build_object('items', n_items, 'answers', n_answer, 'final_answers', n_final,
                            'files', n_files, 'qualification_questions', n_tests);
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
-- erase_contributor removes a contributor's identifying data and closes the
-- account. The caller checks that nothing is owed; this function refuses
-- again if money is still owed or a payout line is pending.
CREATE FUNCTION erase_contributor(p_contributor uuid, p_actor uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = public AS $$
DECLARE
  cp         contributor_profiles%ROWTYPE;
  u          users%ROWTYPE;
  erased_to  text;
  balance    bigint;
BEGIN
  SELECT * INTO cp FROM contributor_profiles WHERE id = p_contributor FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'contributor % not found', p_contributor USING ERRCODE = 'no_data_found';
  END IF;
  IF cp.erased_at IS NOT NULL THEN
    RETURN;
  END IF;
  SELECT COALESCE(sum(amount_cents), 0) INTO balance FROM earnings_ledger WHERE contributor_id = p_contributor;
  IF balance <> 0 THEN
    RAISE EXCEPTION 'contributor % has a balance of % cents', p_contributor, balance USING ERRCODE = 'check_violation';
  END IF;
  IF EXISTS (SELECT 1 FROM payout_lines WHERE contributor_id = p_contributor AND status = 'pending') THEN
    RAISE EXCEPTION 'contributor % has a pending payout line', p_contributor USING ERRCODE = 'check_violation';
  END IF;
  SELECT * INTO u FROM users WHERE id = cp.user_id FOR UPDATE;
  erased_to := 'erased-' || u.id::text || '@erased.invalid';

  -- Account: no sign-in, no roles, no sessions or links.
  UPDATE users SET email = erased_to, display_name = 'Erased contributor', password_hash = NULL,
                   status = 'disabled', email_verified_at = NULL, last_login_at = NULL,
                   failed_login_count = 0, locked_until = NULL
  WHERE id = u.id;
  DELETE FROM user_roles WHERE user_id = u.id;
  DELETE FROM sessions WHERE user_id = u.id;
  DELETE FROM password_resets WHERE user_id = u.id;
  DELETE FROM email_verifications WHERE user_id = u.id;
  DELETE FROM idempotency_keys WHERE user_id = u.id;
  DELETE FROM notification_preferences WHERE user_id = u.id;
  DELETE FROM email_outbox WHERE (user_id = u.id OR to_address = u.email) AND status = 'queued';
  UPDATE email_outbox SET to_address = erased_to, data = '{}' WHERE user_id = u.id OR to_address = u.email;
  UPDATE consents SET ip = NULL WHERE user_id = u.id;
  UPDATE audit_log SET ip = NULL WHERE actor_user_id = u.id;

  -- Profile: keep the public code, status and money links; drop the rest.
  IF cp.cv_file_key IS NOT NULL THEN
    INSERT INTO storage_deletions (key, reason) VALUES (cp.cv_file_key, 'erasure') ON CONFLICT (key) DO NOTHING;
  END IF;
  IF cp.status <> 'removed' THEN
    INSERT INTO contributor_status_history (contributor_id, from_status, to_status, reason, changed_by)
    VALUES (p_contributor, cp.status, 'removed', 'Personal data erased on request', p_actor);
  END IF;
  UPDATE contributor_profiles SET
    status = 'removed', erased_at = now(),
    country = NULL, city = NULL, education_level = NULL, weekly_hours = NULL,
    mpesa_phone = NULL, mpesa_phone_confirmed_at = NULL,
    national_id_fp = NULL, national_id_last3 = NULL, national_id_fp_v = NULL,
    identity_note = CASE WHEN identity_status = 'rejected' THEN 'Erased' ELSE NULL END,
    experience = '{}', linkedin_url = NULL,
    cv_file_key = NULL, cv_file_name = NULL, cv_uploaded_at = NULL, cv_delete_after = NULL,
    screening_note = CASE WHEN screening_status = 'declined' THEN 'Erased' ELSE NULL END
  WHERE id = p_contributor;
  DELETE FROM contributor_languages WHERE contributor_id = p_contributor;
  DELETE FROM contributor_domains WHERE contributor_id = p_contributor;
  DELETE FROM identity_flags WHERE contributor_id = p_contributor OR other_contributor_id = p_contributor;
  UPDATE interviews SET note = NULL, availability = NULL WHERE contributor_id = p_contributor;
  UPDATE qualification_attempts SET answers = '{}', decision_note = NULL WHERE contributor_id = p_contributor;
  UPDATE skips SET note = NULL WHERE contributor_id = p_contributor AND note IS NOT NULL;
  UPDATE content_reports SET note = NULL WHERE contributor_id = p_contributor AND note IS NOT NULL;
  UPDATE assignments SET status = 'expired', completed_at = now() WHERE contributor_id = p_contributor AND status = 'reserved';

  -- Money stays; the payee on old payout lines becomes 'erased'.
  PERFORM set_config('mezai.erasing', p_contributor::text, true);
  UPDATE payout_lines SET mpesa_phone = 'erased', payee_name = 'Erased contributor' WHERE contributor_id = p_contributor;
  PERFORM set_config('mezai.erasing', '', true);

  UPDATE erasure_requests SET status = 'done', closed_by = p_actor, closed_at = now()
  WHERE contributor_id = p_contributor AND status = 'open';
END $$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION purge_project_content(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION erase_contributor(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION purge_project_content(uuid) TO mezai_app;
GRANT EXECUTE ON FUNCTION erase_contributor(uuid, uuid) TO mezai_app;

INSERT INTO settings (key, value, description) VALUES
  ('retention.email_days', '30', 'Days after sending before an email''s merge data is removed from the outbox');

-- +goose Down
DELETE FROM settings WHERE key = 'retention.email_days';
DROP FUNCTION erase_contributor(uuid, uuid);
DROP FUNCTION purge_project_content(uuid);
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE key = 'privacy.manage');
DELETE FROM permissions WHERE key = 'privacy.manage';
DROP TABLE erasure_requests;
DROP TABLE storage_deletions;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION payout_lines_guard() RETURNS trigger LANGUAGE plpgsql AS $$
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
ALTER TABLE payout_lines DROP CONSTRAINT payout_lines_mpesa_phone_check;
ALTER TABLE payout_lines ADD CONSTRAINT payout_lines_mpesa_phone_check CHECK (mpesa_phone ~ '^\+254[17][0-9]{8}$');
ALTER TABLE contributor_profiles DROP CONSTRAINT contributor_profiles_erased_chk, DROP COLUMN erased_at;
ALTER TABLE import_jobs DROP COLUMN purged_at;
ALTER TABLE deliverables DROP COLUMN purged_at;
DROP INDEX projects_retention_idx;
ALTER TABLE projects DROP CONSTRAINT projects_purged_closed_chk, DROP COLUMN content_purged_at;

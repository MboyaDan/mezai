-- Row-level security on client-owned tables, and privileges for mezai_app.
--
-- Scopes (set per transaction, see 00001):
--   staff        ELS staff and system jobs: every organisation. The API still
--                checks permission keys on every endpoint.
--   org          client users: rows of app.org_id only.
--   contributor  contributors: their own work, live projects, and only the
--                items they currently hold a reservation for.
-- No scope set means no rows (fail closed).
--
-- Each table has a read policy and a write policy. Permissive policies are
-- OR-ed per command, so a write policy also grants reads of the rows it
-- covers; write expressions are always narrower than read expressions here.

-- +goose Up

-- Organisations
ALTER TABLE organisations ENABLE ROW LEVEL SECURITY;
CREATE POLICY organisations_read ON organisations FOR SELECT USING (app_sees_org(id));
CREATE POLICY organisations_write ON organisations FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE org_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_members_read ON org_members FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY org_members_write ON org_members FOR ALL USING (app_sees_org(org_id)) WITH CHECK (app_sees_org(org_id));

-- Projects: contributors see live projects (eligibility is checked in the
-- API) and any project they have worked on.
ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
CREATE POLICY projects_read ON projects FOR SELECT USING (
  app_sees_org(org_id)
  OR (app_scope() = 'contributor' AND (
        status = 'live'
        OR EXISTS (SELECT 1 FROM assignments a WHERE a.project_id = projects.id AND a.contributor_id = app_contributor_id()))));
CREATE POLICY projects_write ON projects FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Children of a project a contributor can see (the subquery runs under the
-- projects policy above).
ALTER TABLE project_skills ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_skills_read ON project_skills FOR SELECT USING (
  app_sees_org(org_id)
  OR (app_scope() = 'contributor' AND EXISTS (SELECT 1 FROM projects p WHERE p.id = project_skills.project_id)));
CREATE POLICY project_skills_write ON project_skills FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE project_guidelines ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_guidelines_read ON project_guidelines FOR SELECT USING (
  app_sees_org(org_id)
  OR (app_scope() = 'contributor' AND EXISTS (SELECT 1 FROM projects p WHERE p.id = project_guidelines.project_id)));
CREATE POLICY project_guidelines_write ON project_guidelines FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Imports: staff write, clients read their own reports.
ALTER TABLE import_jobs ENABLE ROW LEVEL SECURITY;
CREATE POLICY import_jobs_read ON import_jobs FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY import_jobs_write ON import_jobs FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE import_errors ENABLE ROW LEVEL SECURITY;
CREATE POLICY import_errors_read ON import_errors FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY import_errors_write ON import_errors FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Items: a contributor sees an item only while holding its reservation.
ALTER TABLE items ENABLE ROW LEVEL SECURITY;
CREATE POLICY items_read ON items FOR SELECT USING (
  app_sees_org(org_id)
  OR (app_scope() = 'contributor' AND EXISTS (
        SELECT 1 FROM assignments a
        WHERE a.item_id = items.id AND a.contributor_id = app_contributor_id()
          AND a.status = 'reserved' AND a.expires_at > now())));
CREATE POLICY items_write ON items FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE item_revisions ENABLE ROW LEVEL SECURITY;
CREATE POLICY item_revisions_read ON item_revisions FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY item_revisions_write ON item_revisions FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Gold: staff only. Nothing gold-related is visible to clients row by row or
-- to contributors at all (brief 8.1).
ALTER TABLE gold_items ENABLE ROW LEVEL SECURITY;
CREATE POLICY gold_items_staff ON gold_items FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE gold_results ENABLE ROW LEVEL SECURITY;
CREATE POLICY gold_results_staff ON gold_results FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Work: contributors read and write their own rows. Clients read assignments
-- for the workforce summary (contributor IDs only, brief 6.3), never raw
-- submissions.
ALTER TABLE assignments ENABLE ROW LEVEL SECURITY;
CREATE POLICY assignments_read ON assignments FOR SELECT USING (
  app_sees_org(org_id) OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));
CREATE POLICY assignments_write ON assignments FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));

ALTER TABLE submissions ENABLE ROW LEVEL SECURITY;
CREATE POLICY submissions_own ON submissions FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));

ALTER TABLE submission_revisions ENABLE ROW LEVEL SECURITY;
CREATE POLICY submission_revisions_own ON submission_revisions FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND EXISTS (
           SELECT 1 FROM submissions s WHERE s.id = submission_revisions.submission_id)))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND EXISTS (
           SELECT 1 FROM submissions s WHERE s.id = submission_revisions.submission_id)));

ALTER TABLE skips ENABLE ROW LEVEL SECURITY;
CREATE POLICY skips_own ON skips FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));

ALTER TABLE content_reports ENABLE ROW LEVEL SECURITY;
CREATE POLICY content_reports_own ON content_reports FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));

-- Quality
ALTER TABLE consensus_results ENABLE ROW LEVEL SECURITY;
CREATE POLICY consensus_results_read ON consensus_results FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY consensus_results_write ON consensus_results FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE reviews ENABLE ROW LEVEL SECURITY;
CREATE POLICY reviews_staff ON reviews FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE feedback ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_own ON feedback FOR ALL
  USING (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()))
  WITH CHECK (app_is_staff() OR (app_scope() = 'contributor' AND contributor_id = app_contributor_id()));

ALTER TABLE agreement_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY agreement_snapshots_read ON agreement_snapshots FOR SELECT USING (app_sees_org(org_id));
CREATE POLICY agreement_snapshots_write ON agreement_snapshots FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

-- Delivery: clients read their deliverables and record their own downloads.
ALTER TABLE deliverables ENABLE ROW LEVEL SECURITY;
CREATE POLICY deliverables_read ON deliverables FOR SELECT USING (
  app_is_staff() OR (app_sees_org(org_id) AND published_at IS NOT NULL));
CREATE POLICY deliverables_write ON deliverables FOR ALL USING (app_is_staff()) WITH CHECK (app_is_staff());

ALTER TABLE deliverable_downloads ENABLE ROW LEVEL SECURITY;
CREATE POLICY deliverable_downloads_tenant ON deliverable_downloads FOR ALL
  USING (app_sees_org(org_id)) WITH CHECK (app_sees_org(org_id));

-- Privileges. Tables not listed under RLS (identity, contributors, money,
-- platform) are reachable only through endpoints that check permissions.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO mezai_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO mezai_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO mezai_app;
REVOKE ALL ON goose_db_version FROM mezai_app;

-- Append-only records: no update or delete for the application, ever.
REVOKE UPDATE, DELETE ON audit_log, consents, contributor_status_history, item_revisions,
  submission_revisions, skips, gold_results, reviews, agreement_snapshots, trust_scores,
  earnings_ledger, deliverable_downloads FROM mezai_app;

-- Reference data changes only through migrations.
REVOKE INSERT, UPDATE, DELETE ON permissions, task_templates FROM mezai_app;

-- +goose Down
GRANT UPDATE, DELETE ON audit_log, consents, contributor_status_history, item_revisions,
  submission_revisions, skips, gold_results, reviews, agreement_snapshots, trust_scores,
  earnings_ledger, deliverable_downloads TO mezai_app;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM mezai_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM mezai_app;
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM mezai_app;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND rowsecurity LOOP
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END
$$;
-- +goose StatementEnd
DROP POLICY organisations_read ON organisations;
DROP POLICY organisations_write ON organisations;
DROP POLICY org_members_read ON org_members;
DROP POLICY org_members_write ON org_members;
DROP POLICY projects_read ON projects;
DROP POLICY projects_write ON projects;
DROP POLICY project_skills_read ON project_skills;
DROP POLICY project_skills_write ON project_skills;
DROP POLICY project_guidelines_read ON project_guidelines;
DROP POLICY project_guidelines_write ON project_guidelines;
DROP POLICY import_jobs_read ON import_jobs;
DROP POLICY import_jobs_write ON import_jobs;
DROP POLICY import_errors_read ON import_errors;
DROP POLICY import_errors_write ON import_errors;
DROP POLICY items_read ON items;
DROP POLICY items_write ON items;
DROP POLICY item_revisions_read ON item_revisions;
DROP POLICY item_revisions_write ON item_revisions;
DROP POLICY gold_items_staff ON gold_items;
DROP POLICY gold_results_staff ON gold_results;
DROP POLICY assignments_read ON assignments;
DROP POLICY assignments_write ON assignments;
DROP POLICY submissions_own ON submissions;
DROP POLICY submission_revisions_own ON submission_revisions;
DROP POLICY skips_own ON skips;
DROP POLICY content_reports_own ON content_reports;
DROP POLICY consensus_results_read ON consensus_results;
DROP POLICY consensus_results_write ON consensus_results;
DROP POLICY reviews_staff ON reviews;
DROP POLICY feedback_own ON feedback;
DROP POLICY agreement_snapshots_read ON agreement_snapshots;
DROP POLICY agreement_snapshots_write ON agreement_snapshots;
DROP POLICY deliverables_read ON deliverables;
DROP POLICY deliverables_write ON deliverables;
DROP POLICY deliverable_downloads_tenant ON deliverable_downloads;

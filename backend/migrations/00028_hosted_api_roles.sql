-- Hosted PostgreSQL hardening (Stage 19a, D-152).
--
-- Supabase creates the roles anon and authenticated and gives them every
-- privilege on new tables in the public schema through default privileges,
-- so its Data API (PostgREST) could read users, sessions and the ledger with
-- the project's publishable key. MezAI never uses that API: the Go API is the
-- only database client. This takes those privileges away again, for existing
-- tables and for tables later migrations create. The Data API is also turned
-- off in the Supabase dashboard (docs/deployment.md); this is the second lock.
--
-- Where the roles do not exist (local PostgreSQL, CI) nothing happens.
-- A grant this role did not make cannot be revoked by it: that is reported as
-- a notice instead of failing the migration, and the dashboard step covers it.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  r text;
BEGIN
  FOREACH r IN ARRAY ARRAY['anon', 'authenticated'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
      CONTINUE;
    END IF;
    BEGIN
      EXECUTE format('REVOKE ALL ON ALL TABLES IN SCHEMA public FROM %I', r);
      EXECUTE format('REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM %I', r);
      EXECUTE format('REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM %I', r);
      EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM %I', r);
      EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM %I', r);
      EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON FUNCTIONS FROM %I', r);
    EXCEPTION WHEN insufficient_privilege THEN
      RAISE NOTICE 'could not revoke every privilege from %: turn the Data API off in the dashboard', r;
    END;
  END LOOP;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Nothing to undo: these roles must never be given access back.
SELECT 1;

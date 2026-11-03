package migrations_test

import (
	"context"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/migrations"
)

// Tables that carry org_id but are not client-owned rows, so they have no RLS:
// the audit log and enquiries are staff-only; user_roles and invitations are
// read during sign-in before any scope exists.
var orgIDWithoutRLS = map[string]bool{
	"audit_log": true, "enquiries": true, "user_roles": true, "invitations": true,
}

func TestMigrationsUpDownUp(t *testing.T) {
	pool, dsn := dbtest.New(t) // applies every migration from empty
	ctx := context.Background()

	if err := migrations.Reset(ctx, dsn); err != nil {
		t.Fatalf("down to zero: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d tables left after rolling back every migration", left)
	}
	if err := migrations.Up(ctx, dsn); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestEveryOrgOwnedTableHasRLS(t *testing.T) {
	pool, _ := dbtest.New(t)
	rows, err := pool.Query(context.Background(), `
		SELECT c.relname, c.relrowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'org_id' AND NOT a.attisdropped
		WHERE n.nspname = 'public' AND c.relkind = 'r'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var name string
		var rls bool
		if err := rows.Scan(&name, &rls); err != nil {
			t.Fatal(err)
		}
		n++
		if !rls && !orgIDWithoutRLS[name] {
			t.Errorf("%s has org_id but row-level security is off", name)
		}
	}
	if n < 20 {
		t.Fatalf("expected at least 20 tables with org_id, found %d", n)
	}
	// organisations is keyed by id, not org_id.
	var orgRLS bool
	if err := pool.QueryRow(context.Background(), `SELECT relrowsecurity FROM pg_class WHERE relname = 'organisations'`).Scan(&orgRLS); err != nil || !orgRLS {
		t.Errorf("organisations must have row-level security (err=%v)", err)
	}
}

func TestEveryTableHasCreatedAtAndUUIDKey(t *testing.T) {
	pool, _ := dbtest.New(t)
	rows, err := pool.Query(context.Background(), `
		SELECT t.tablename,
		       EXISTS (SELECT 1 FROM information_schema.columns c
		               WHERE c.table_name = t.tablename AND c.column_name = 'created_at'),
		       EXISTS (SELECT 1 FROM information_schema.columns c
		               WHERE c.table_name = t.tablename AND c.column_name = 'id' AND c.data_type = 'uuid')
		FROM pg_tables t WHERE t.schemaname = 'public' AND t.tablename <> 'goose_db_version'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var created, uuidKey bool
		if err := rows.Scan(&name, &created, &uuidKey); err != nil {
			t.Fatal(err)
		}
		if !created {
			t.Errorf("%s has no created_at", name)
		}
		// role_permissions is a pure join table keyed by its two foreign keys.
		if !uuidKey && name != "role_permissions" {
			t.Errorf("%s has no uuid id column", name)
		}
	}
}

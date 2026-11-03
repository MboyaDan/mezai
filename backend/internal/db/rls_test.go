package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/store"
	"github.com/MboyaDan/mezai/backend/seed"
)

type fixture struct {
	pool                *db.Pool
	sampleOrg, otherOrg uuid.UUID
}

// setup gives each test a fresh database with the sample data and one
// published deliverable per organisation (plus an unpublished one).
func setup(t *testing.T) fixture {
	t.Helper()
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	if _, err := seed.Run(ctx, pool, seed.Options{Password: "test-password-only"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	f := fixture{pool: pool}
	staff(t, pool, func(q *store.Queries, tx pgx.Tx) {
		a, err := q.GetOrganisationBySlug(ctx, seed.SampleOrgSlug)
		must(t, err)
		b, err := q.GetOrganisationBySlug(ctx, seed.OtherOrgSlug)
		must(t, err)
		f.sampleOrg, f.otherOrg = a.ID, b.ID

		projects, err := q.ListProjects(ctx)
		must(t, err)
		var admin uuid.UUID
		must(t, tx.QueryRow(ctx, `SELECT id FROM users WHERE email = 'superadmin@sample.invalid'`).Scan(&admin))
		now := time.Now()
		for _, p := range projects {
			_, err := q.CreateDeliverable(ctx, store.CreateDeliverableParams{
				OrgID: p.OrgID, ProjectID: p.ID, Format: "jsonl", FileKey: "test/" + p.ID.String(),
				ManifestKey: "test/manifest", Manifest: []byte(`{}`), ChecksumSha256: sha64, SchemaVersion: 1,
				PublishedAt: &now, PublishedBy: &admin,
			})
			must(t, err)
		}
		// An unpublished deliverable is invisible to its own client until published.
		_, err = q.CreateDeliverable(ctx, store.CreateDeliverableParams{
			OrgID: projects[0].OrgID, ProjectID: projects[0].ID, Format: "csv", FileKey: "test/draft",
			ManifestKey: "test/manifest", Manifest: []byte(`{}`), ChecksumSha256: sha64, SchemaVersion: 1,
		})
		must(t, err)
	})
	return f
}

const sha64 = "0000000000000000000000000000000000000000000000000000000000000000"

func TestOrganisationIsolation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	type counts struct{ projects, items, deliverables, orgs, gold int }
	read := func(org uuid.UUID) (c counts) {
		must(t, f.pool.InScope(ctx, db.Org(org), func(tx pgx.Tx) error {
			q := store.New(tx)
			ps, err := q.ListProjects(ctx)
			must(t, err)
			for _, p := range ps {
				if p.OrgID != org {
					t.Errorf("org %s read project %s of org %s", org, p.ID, p.OrgID)
				}
			}
			ds, err := q.ListDeliverables(ctx)
			must(t, err)
			for _, d := range ds {
				if d.OrgID != org || d.PublishedAt == nil {
					t.Errorf("org %s read deliverable %s (org %s, published %v)", org, d.ID, d.OrgID, d.PublishedAt)
				}
			}
			orgs, err := q.ListOrganisations(ctx)
			must(t, err)
			items, err := q.CountItems(ctx)
			must(t, err)
			gold, err := q.CountGoldItems(ctx)
			must(t, err)
			c = counts{len(ps), int(items), len(ds), len(orgs), int(gold)}
			return nil
		}))
		return c
	}

	a := read(f.sampleOrg)
	b := read(f.otherOrg)
	if a != (counts{projects: 3, items: 16, deliverables: 3, orgs: 1, gold: 0}) {
		t.Errorf("sample org sees %+v", a)
	}
	if b != (counts{projects: 1, items: 8, deliverables: 1, orgs: 1, gold: 0}) {
		t.Errorf("other org sees %+v", b)
	}

	// Asking for another organisation's project by id returns nothing.
	var foreign uuid.UUID
	staff(t, f.pool, func(q *store.Queries, tx pgx.Tx) {
		must(t, tx.QueryRow(ctx, `SELECT id FROM projects WHERE org_id = $1 LIMIT 1`, f.otherOrg).Scan(&foreign))
	})
	err := f.pool.InScope(ctx, db.Org(f.sampleOrg), func(tx pgx.Tx) error {
		_, err := store.New(tx).GetProject(ctx, foreign)
		return err
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-org GetProject: want ErrNoRows, got %v", err)
	}
	err = f.pool.InScope(ctx, db.Org(f.sampleOrg), func(tx pgx.Tx) error {
		_, err := store.New(tx).ListItemsByProject(ctx, foreign)
		return err
	})
	must(t, err)
}

func TestOrgScopeCannotWriteAcrossTenants(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// A client user cannot create projects, even in their own organisation.
	err := f.pool.InScope(ctx, db.Org(f.sampleOrg), func(tx pgx.Tx) error {
		tpl, err := store.New(tx).GetLatestTaskTemplate(ctx, "classification")
		must(t, err)
		_, err = store.New(tx).CreateProject(ctx, store.CreateProjectParams{
			OrgID: f.sampleOrg, TemplateID: tpl.ID, TaskType: "classification", Name: "x",
			Status: "draft", Config: []byte(`{}`), OverlapK: 1, Currency: "KES",
		})
		return err
	})
	wantCode(t, err, "42501") // insufficient_privilege: row violates row-level security policy

	// Nor record a download of another organisation's deliverable.
	var foreignDeliverable, user uuid.UUID
	staff(t, f.pool, func(q *store.Queries, tx pgx.Tx) {
		must(t, tx.QueryRow(ctx, `SELECT id FROM deliverables WHERE org_id = $1 LIMIT 1`, f.otherOrg).Scan(&foreignDeliverable))
		must(t, tx.QueryRow(ctx, `SELECT id FROM users WHERE email = 'client.admin@sample.invalid'`).Scan(&user))
	})
	err = f.pool.InScope(ctx, db.Org(f.sampleOrg), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO deliverable_downloads (org_id, deliverable_id, user_id) VALUES ($1, $2, $3)`,
			f.otherOrg, foreignDeliverable, user)
		return err
	})
	wantCode(t, err, "42501")

	// Updating another organisation's items silently matches no rows.
	err = f.pool.InScope(ctx, db.Org(f.sampleOrg), func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE items SET content = '{}' WHERE org_id = $1`, f.otherOrg)
		if err == nil && tag.RowsAffected() != 0 {
			t.Errorf("updated %d foreign items", tag.RowsAffected())
		}
		return err
	})
	must(t, err)
}

func TestNoScopeSeesNothing(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if err := f.pool.InScope(ctx, db.Scope{}, func(pgx.Tx) error { return nil }); err == nil {
		t.Fatal("the zero Scope must be rejected")
	}
	// The role without any app.* settings: fail closed.
	err := pgx.BeginFunc(ctx, f.pool.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE mezai_app"); err != nil {
			return err
		}
		var n int
		for _, table := range []string{"organisations", "projects", "items", "deliverables", "gold_items", "submissions"} {
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("%s: %d rows visible with no scope", table, n)
			}
		}
		return nil
	})
	must(t, err)
}

func TestScopeDoesNotLeakToPooledConnection(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	conn, err := f.pool.Acquire(ctx)
	must(t, err)
	defer conn.Release()
	must(t, pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error { return db.ApplyScope(ctx, tx, db.Org(f.sampleOrg)) }))
	var scope, role string
	must(t, conn.QueryRow(ctx, `SELECT coalesce(current_setting('app.scope', true), ''), current_user`).Scan(&scope, &role))
	if scope != "" || role == "mezai_app" {
		t.Fatalf("same connection after commit: scope=%q role=%q", scope, role)
	}
}

func TestContributorSeesOnlyReservedItems(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	var contributor uuid.UUID
	var reserved, expired, gold store.Item
	var liveProjects int
	staff(t, f.pool, func(q *store.Queries, tx pgx.Tx) {
		must(t, tx.QueryRow(ctx, `SELECT id FROM contributor_profiles WHERE status = 'active' ORDER BY public_code LIMIT 1`).Scan(&contributor))
		must(t, tx.QueryRow(ctx, `SELECT count(*) FROM projects WHERE status = 'live'`).Scan(&liveProjects))
		var projectID uuid.UUID
		must(t, tx.QueryRow(ctx, `SELECT id FROM projects WHERE org_id = $1 AND task_type = 'classification'`, f.sampleOrg).Scan(&projectID))
		items, err := q.ListItemsByProject(ctx, projectID)
		must(t, err)
		var goldID uuid.UUID
		must(t, tx.QueryRow(ctx, `SELECT item_id FROM gold_items WHERE project_id = $1 LIMIT 1`, projectID).Scan(&goldID))
		var plain []store.Item
		for _, it := range items {
			if it.ID == goldID {
				gold = it
			} else {
				plain = append(plain, it)
			}
		}
		reserved, expired = plain[0], plain[1]
		for _, it := range []store.Item{reserved, gold} {
			_, err := q.CreateAssignment(ctx, store.CreateAssignmentParams{
				OrgID: it.OrgID, ProjectID: it.ProjectID, ItemID: it.ID, ContributorID: contributor,
				ExpiresAt: time.Now().Add(30 * time.Minute),
			})
			must(t, err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO assignments (org_id, project_id, item_id, contributor_id, reserved_at, expires_at)
			VALUES ($1, $2, $3, $4, now() - interval '2 hours', now() - interval '1 hour')`,
			expired.OrgID, expired.ProjectID, expired.ID, contributor)
		must(t, err)
	})

	must(t, f.pool.InScope(ctx, db.Contributor(contributor), func(tx pgx.Tx) error {
		q := store.New(tx)
		rows, err := tx.Query(ctx, `SELECT id FROM items ORDER BY id`)
		must(t, err)
		seen := map[uuid.UUID]bool{}
		for rows.Next() {
			var id uuid.UUID
			must(t, rows.Scan(&id))
			seen[id] = true
		}
		must(t, rows.Err())
		// The reserved gold item looks like any other item; only its gold row is hidden.
		if len(seen) != 2 || !seen[reserved.ID] || !seen[gold.ID] {
			t.Errorf("contributor sees items %v; want only the two reserved ones", seen)
		}
		if seen[expired.ID] {
			t.Error("expired reservation still visible")
		}
		n, err := q.CountGoldItems(ctx)
		must(t, err)
		if n != 0 {
			t.Errorf("contributor sees %d gold items", n)
		}
		ps, err := q.ListProjects(ctx)
		must(t, err)
		if len(ps) != liveProjects {
			t.Errorf("contributor sees %d projects, want the %d live ones", len(ps), liveProjects)
		}
		ds, err := q.ListDeliverables(ctx)
		must(t, err)
		if len(ds) != 0 {
			t.Errorf("contributor sees %d deliverables", len(ds))
		}
		return nil
	}))
}

func TestAppendOnlyTables(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	staff(t, f.pool, func(q *store.Queries, tx pgx.Tx) {
		_, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_kind, action, entity_type) VALUES ('system', 'test', 'test')`)
		must(t, err)
	})
	for _, stmt := range []string{
		`UPDATE audit_log SET action = 'changed'`,
		`DELETE FROM audit_log`,
		`DELETE FROM earnings_ledger`,
		`UPDATE consents SET version = 'x'`,
		`UPDATE submission_revisions SET answer = '{}'`,
		`UPDATE contributor_status_history SET reason = 'x'`,
	} {
		err := f.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, stmt)
			return err
		})
		wantCode(t, err, "42501")
	}
}

func TestPayoutCreatorCannotApprove(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	err := f.pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		var ops uuid.UUID
		must(t, tx.QueryRow(ctx, `SELECT id FROM users WHERE email = 'ops@sample.invalid'`).Scan(&ops))
		_, err := tx.Exec(ctx, `INSERT INTO payout_batches (period_start, period_end, status, idempotency_key, created_by, approved_by, approved_at)
			VALUES (current_date - 7, current_date, 'approved', 'test-key-0001', $1, $1, now())`, ops)
		return err
	})
	wantCode(t, err, "23514") // check_violation
}

func staff(t *testing.T, pool *db.Pool, fn func(*store.Queries, pgx.Tx)) {
	t.Helper()
	must(t, pool.InScope(context.Background(), db.Staff(), func(tx pgx.Tx) error {
		fn(store.New(tx), tx)
		return nil
	}))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

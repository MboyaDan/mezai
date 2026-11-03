// Package dbtest gives each test its own migrated PostgreSQL database.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/migrations"
)

// migrateLock serialises migrations across parallel test packages, because
// the cluster-wide mezai_app role is created and granted by the first one.
const migrateLock = 4_202_611

// New creates an empty database next to DATABASE_URL, applies every
// migration, and drops the database when the test ends. It skips when
// DATABASE_URL is unset outside CI and fails inside CI.
func New(t testing.TB) (*db.Pool, string) {
	t.Helper()
	admin := os.Getenv("DATABASE_URL")
	if admin == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DATABASE_URL must be set in CI")
		}
		t.Skip("DATABASE_URL not set; run `make up` and `make test`")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer conn.Close(context.Background())

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "mezai_test_" + hex.EncodeToString(suffix)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), admin)
		if err != nil {
			t.Logf("cleanup connect: %v", err)
			return
		}
		defer c.Close(context.Background())
		if _, err := c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Logf("drop %s: %v", name, err)
		}
	})

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	dsn := u.String()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrateLock); err != nil {
		t.Fatalf("lock: %v", err)
	}
	err = migrations.Up(ctx, dsn)
	_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrateLock)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, dsn
}

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/password"
)

func TestAdditionalNeedsAFirstSuperAdmin(t *testing.T) {
	pool, _ := dbtest.New(t)
	if err := auth.EnsureSuperAdmin(context.Background(), pool, "backup@els.invalid", "Backup", "a long enough password", true); !errors.Is(err, auth.ErrNotInitialised) {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureSuperAdminOnlyOnce(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	if err := auth.EnsureSuperAdmin(ctx, pool, "Owner@els.invalid", "First Admin", "short", false); !errors.Is(err, password.ErrTooShort) {
		t.Fatalf("short password: %v", err)
	}
	if err := auth.EnsureSuperAdmin(ctx, pool, "Owner@els.invalid", "First Admin", "a long enough password", false); err != nil {
		t.Fatal(err)
	}
	if err := auth.EnsureSuperAdmin(ctx, pool, "second@els.invalid", "Second", "another long password", false); !errors.Is(err, auth.ErrAlreadyInitialised) {
		t.Fatalf("second run: %v", err)
	}
	var email string
	var audited int
	if err := pool.QueryRow(ctx, `SELECT email FROM users`).Scan(&email); err != nil || email != "owner@els.invalid" {
		t.Fatalf("users: %q %v", email, err)
	}
	// The backup Super Admin comes from the same command with -additional.
	if err := auth.EnsureSuperAdmin(ctx, pool, "backup@els.invalid", "Backup", "a third long password", true); err != nil {
		t.Fatalf("additional: %v", err)
	}
	if err := auth.EnsureSuperAdmin(ctx, pool, "backup@els.invalid", "Backup", "a third long password", true); !errors.Is(err, auth.ErrEmailInUse) {
		t.Fatalf("duplicate additional: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'setup.super_admin_created' AND actor_kind = 'system'`).Scan(&audited); err != nil || audited != 2 {
		t.Fatalf("audit: %d %v", audited, err)
	}
}

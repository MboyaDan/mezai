package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/password"
)

func TestEnsureSuperAdminOnlyOnce(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	if err := auth.EnsureSuperAdmin(ctx, pool, "Owner@els.invalid", "First Admin", "short"); !errors.Is(err, password.ErrTooShort) {
		t.Fatalf("short password: %v", err)
	}
	if err := auth.EnsureSuperAdmin(ctx, pool, "Owner@els.invalid", "First Admin", "a long enough password"); err != nil {
		t.Fatal(err)
	}
	if err := auth.EnsureSuperAdmin(ctx, pool, "second@els.invalid", "Second", "another long password"); !errors.Is(err, auth.ErrAlreadyInitialised) {
		t.Fatalf("second run: %v", err)
	}
	var email string
	var audited int
	if err := pool.QueryRow(ctx, `SELECT email FROM users`).Scan(&email); err != nil || email != "owner@els.invalid" {
		t.Fatalf("users: %q %v", email, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'setup.super_admin_created' AND actor_kind = 'system'`).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("audit: %d %v", audited, err)
	}
}

package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ErrAlreadyInitialised means an active Super Admin exists; setup did nothing.
var ErrAlreadyInitialised = errors.New("auth: a Super Admin already exists")

// EnsureSuperAdmin creates the first Super Admin. It is the only way to make
// one outside the UI, and it refuses once any active Super Admin exists.
func EnsureSuperAdmin(ctx context.Context, pool *db.Pool, email, name, plain string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if !strings.Contains(email, "@") || name == "" {
		return errors.New("auth: a valid email and a name are required")
	}
	if err := password.Validate(plain, email); err != nil {
		return err
	}
	hash, err := password.Hash(plain)
	if err != nil {
		return err
	}
	return pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		// Serialise concurrent setup runs.
		if _, err := tx.Exec(ctx, "LOCK TABLE user_roles IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			return err
		}
		n, err := q.CountUsersWithRole(ctx, "super_admin")
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrAlreadyInitialised
		}
		now := time.Now()
		u, err := q.CreateUser(ctx, store.CreateUserParams{Email: email, DisplayName: name, PasswordHash: &hash, EmailVerifiedAt: &now})
		if err != nil {
			return err
		}
		role, err := q.GetRoleByKey(ctx, "super_admin")
		if err != nil {
			return err
		}
		if err := q.AddUserRole(ctx, store.AddUserRoleParams{UserID: u.ID, RoleID: role.ID}); err != nil {
			return err
		}
		return Audit(ctx, q, Meta{}, Entry{
			Action: "setup.super_admin_created", EntityType: "user", EntityID: &u.ID,
			After: map[string]any{"email": email, "role": "super_admin"},
		})
	})
}

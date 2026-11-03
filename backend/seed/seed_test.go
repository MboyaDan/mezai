package seed_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/seed"
)

func TestRunLoadsObviouslyFakeDataOnce(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()

	sum, err := seed.Run(ctx, pool, seed.Options{Password: "test-password-only"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Organisations != 2 || sum.Projects != 4 || sum.Contributors != 12 || sum.GoldItems == 0 || sum.QualificationTests != 3 {
		t.Fatalf("unexpected summary %+v", sum)
	}
	if _, err := seed.Run(ctx, pool, seed.Options{Password: "test-password-only"}); !errors.Is(err, seed.ErrAlreadySeeded) {
		t.Fatalf("second run: want ErrAlreadySeeded, got %v", err)
	}

	err = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		for _, check := range []struct{ what, sql string }{
			{"email not ending in .invalid", `SELECT email FROM users WHERE email NOT LIKE '%@sample.invalid'`},
			{"organisation without (Sample)", `SELECT name FROM organisations WHERE name NOT LIKE '%(Sample)'`},
			{"project without (Sample)", `SELECT name FROM projects WHERE name NOT LIKE '%(Sample)'`},
			{"contributor not named Sample Contributor", `SELECT u.display_name FROM contributor_profiles c JOIN users u ON u.id = c.user_id WHERE u.display_name NOT LIKE 'Sample Contributor %'`},
		} {
			rows, err := tx.Query(ctx, check.sql)
			if err != nil {
				return err
			}
			for rows.Next() {
				var v string
				_ = rows.Scan(&v)
				t.Errorf("%s: %q", check.what, v)
			}
			rows.Close()
		}
		var hash string
		if err := tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE email = 'superadmin@sample.invalid'`).Scan(&hash); err != nil {
			return err
		}
		if !strings.HasPrefix(hash, "$argon2id$") {
			t.Errorf("super admin password is not an Argon2id hash")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsShortPassword(t *testing.T) {
	if _, err := seed.Run(context.Background(), nil, seed.Options{Password: "short"}); err == nil {
		t.Fatal("want error for a short password")
	}
}

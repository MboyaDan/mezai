package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Scope says whose rows a transaction may see. Row-level security policies in
// migrations/00007_rls_grants.sql read it; the zero Scope is rejected.
type Scope struct {
	kind          string
	orgID         uuid.UUID
	contributorID uuid.UUID
}

// Staff sees every organisation. Endpoints still check permission keys.
func Staff() Scope { return Scope{kind: "staff"} }

// System is the scope for auth bookkeeping and background jobs. It is staff
// scope by another name: it switches to mezai_app, and the tables it touches
// either have no row-level security or need every organisation.
func System() Scope { return Staff() }

// Org sees one client organisation's rows.
func Org(orgID uuid.UUID) Scope { return Scope{kind: "org", orgID: orgID} }

// Contributor sees a contributor's own work and the items they currently hold.
func Contributor(contributorID uuid.UUID) Scope {
	return Scope{kind: "contributor", contributorID: contributorID}
}

var errNoScope = errors.New("db: transaction scope not set")

// InScope runs fn in a transaction as the non-owner role mezai_app with the
// scope set, so row-level security applies. It commits when fn returns nil.
func (p *Pool) InScope(ctx context.Context, s Scope, fn func(pgx.Tx) error) error {
	if s.kind == "" {
		return errNoScope
	}
	return pgx.BeginFunc(ctx, p.Pool, func(tx pgx.Tx) error {
		if err := ApplyScope(ctx, tx, s); err != nil {
			return err
		}
		return fn(tx)
	})
}

// Per-transaction limits (D-105).
const (
	LockTimeout      = "5s"
	StatementTimeout = "30s"
)

// ApplyScope switches an open transaction to mezai_app with the scope set.
// Settings are transaction-local, so they never leak to the next user of a
// pooled connection.
func ApplyScope(ctx context.Context, tx pgx.Tx, s Scope) error {
	if s.kind == "" {
		return errNoScope
	}
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE mezai_app"); err != nil {
		return fmt.Errorf("set role: %w", err)
	}
	str := func(id uuid.UUID) string {
		if id == uuid.Nil {
			return ""
		}
		return id.String()
	}
	// Timeouts stop one stuck lock from piling up every request behind it:
	// a request that cannot get a lock in 5 s fails (and can be retried)
	// instead of holding a pooled connection (D-105).
	_, err := tx.Exec(ctx,
		`SELECT set_config('app.scope', $1, true), set_config('app.org_id', $2, true),
		        set_config('app.contributor_id', $3, true),
		        set_config('lock_timeout', $4, true), set_config('statement_timeout', $5, true)`,
		s.kind, str(s.orgID), str(s.contributorID), LockTimeout, StatementTimeout)
	if err != nil {
		return fmt.Errorf("set scope: %w", err)
	}
	return nil
}

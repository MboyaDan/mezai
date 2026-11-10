// Package db opens the PostgreSQL connection pool.
package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultMaxConns is the pool size when DATABASE_URL does not set pool_max_conns.
const DefaultMaxConns = 20

// Pool wraps pgxpool.Pool so it satisfies the readiness Checker interface.
type Pool struct{ *pgxpool.Pool }

// Open parses the URL and creates a pool. Connections are made lazily; call Ping to verify.
func Open(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// pgx defaults to max(4, CPUs) connections, which on a small instance
	// would cap the whole API at four concurrent transactions. Set
	// pool_max_conns in DATABASE_URL to change it (keep it under the
	// database or pooler limit).
	if !strings.Contains(url, "pool_max_conns") {
		cfg.MaxConns = DefaultMaxConns
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Pool{p}, nil
}

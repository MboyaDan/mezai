// Package db opens the PostgreSQL connection pool.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool so it satisfies the readiness Checker interface.
type Pool struct{ *pgxpool.Pool }

// Open parses the URL and creates a pool. Connections are made lazily; call Ping to verify.
func Open(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Pool{p}, nil
}

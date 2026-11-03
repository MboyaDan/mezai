// Package migrations embeds the SQL migrations and applies them with goose.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed *.sql
var files embed.FS

// Up applies every pending migration to the database at url.
func Up(ctx context.Context, url string) error {
	return run(ctx, url, func(p *goose.Provider) error {
		_, err := p.Up(ctx)
		return err
	})
}

// Reset rolls every migration back. Tests use it to prove the Down sections work.
func Reset(ctx context.Context, url string) error {
	return run(ctx, url, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
}

func run(ctx context.Context, url string, fn func(*goose.Provider) error) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, files)
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if err := fn(p); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

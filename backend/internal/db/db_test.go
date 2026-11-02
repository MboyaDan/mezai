package db

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestPingRealPostgres runs against the database in DATABASE_URL (Compose locally,
// a service container in CI). It is skipped only when no database is configured
// outside CI; in CI a missing DATABASE_URL fails the test.
func TestPingRealPostgres(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DATABASE_URL must be set in CI")
		}
		t.Skip("DATABASE_URL not set; run `make up` and `make test`")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	var v int
	if err := p.QueryRow(ctx, "select current_setting('server_version_num')::int").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v < 160000 {
		t.Fatalf("PostgreSQL 16 or newer required, got %d", v)
	}
}

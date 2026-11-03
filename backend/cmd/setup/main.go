// Command setup prepares a new environment after migrations: it creates the
// first Super Admin from SETUP_ADMIN_EMAIL, SETUP_ADMIN_NAME and
// SETUP_ADMIN_PASSWORD. It runs in every environment, production included,
// and does nothing once an active Super Admin exists. With -additional it
// adds another Super Admin (the backup). It never runs migrations.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
)

func main() {
	additional := flag.Bool("additional", false, "add another Super Admin (the backup) instead of the first")
	flag.Parse()
	if err := run(os.Getenv, *additional); err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
}

func run(getenv func(string) string, additional bool) error {
	url := getenv("DATABASE_URL")
	email, name, pw := getenv("SETUP_ADMIN_EMAIL"), getenv("SETUP_ADMIN_NAME"), getenv("SETUP_ADMIN_PASSWORD")
	if url == "" || email == "" || name == "" || pw == "" {
		return errors.New("DATABASE_URL, SETUP_ADMIN_EMAIL, SETUP_ADMIN_NAME and SETUP_ADMIN_PASSWORD are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	err = auth.EnsureSuperAdmin(ctx, pool, email, name, pw, additional)
	if errors.Is(err, auth.ErrAlreadyInitialised) {
		fmt.Println("setup: a Super Admin already exists; nothing changed")
		return nil
	}
	if err != nil {
		return err
	}
	// The password is never printed. Remove SETUP_ADMIN_PASSWORD from the environment now.
	fmt.Printf("setup: created Super Admin %s\n", email)
	return nil
}

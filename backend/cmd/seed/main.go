// Command seed loads obviously fake sample data for development and tests.
// It refuses to run unless APP_ENV is development or test.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/seed"
)

// devPassword is the sign-in password for every sample user when
// SEED_PASSWORD is unset. Development and test only.
const devPassword = "sample-password-only"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := guard(os.Getenv("APP_ENV")); err != nil {
		return err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	pw := os.Getenv("SEED_PASSWORD")
	if pw == "" {
		pw = devPassword
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	sum, err := seed.Run(ctx, pool, seed.Options{Password: pw})
	if errors.Is(err, seed.ErrAlreadySeeded) {
		fmt.Println("seed: sample data already loaded; nothing changed")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("seed: %d users, %d organisations, %d contributors, %d projects, %d items (%d gold), %d qualification tests\n",
		sum.Users, sum.Organisations, sum.Contributors, sum.Projects, sum.Items, sum.GoldItems, sum.QualificationTests)
	fmt.Println("seed: sign in as superadmin@sample.invalid (all sample users share SEED_PASSWORD or the dev default)")
	return nil
}

// guard allows only development and test. An empty APP_ENV is refused so a
// misconfigured production shell cannot seed by accident.
func guard(env string) error {
	switch env {
	case "development", "test":
		return nil
	default:
		return fmt.Errorf("refusing to run with APP_ENV=%q; set APP_ENV=development or test", env)
	}
}

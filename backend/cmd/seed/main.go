// Command seed loads obviously fake sample data for development and tests.
// It refuses to run unless APP_ENV is development or test. The sample data
// itself arrives in Stage 3.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := guard(os.Getenv("APP_ENV")); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Println("seed: nothing to load yet (sample data arrives in Stage 3)")
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

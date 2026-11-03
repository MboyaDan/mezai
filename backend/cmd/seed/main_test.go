package main

import (
	"strings"
	"testing"
)

func TestGuard(t *testing.T) {
	for env, allowed := range map[string]bool{
		"development": true,
		"test":        true,
		"production":  false,
		"staging":     false,
		"":            false,
		"Production":  false,
	} {
		if err := guard(env); (err == nil) != allowed {
			t.Errorf("APP_ENV=%q: allowed=%v, err=%v", env, allowed, err)
		}
	}
}

// The whole command, not only guard, must refuse production before touching
// the database: DATABASE_URL points nowhere, so reaching it would fail differently.
func TestRunRefusesProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://nobody@127.0.0.1:1/none")
	err := run()
	if err == nil || !strings.Contains(err.Error(), "refusing to run") {
		t.Fatalf("want refusal, got %v", err)
	}
}

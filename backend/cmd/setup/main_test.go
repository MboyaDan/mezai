package main

import "testing"

func TestRunNeedsEveryVariable(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://x", "SETUP_ADMIN_EMAIL": "a@b.invalid", "SETUP_ADMIN_NAME": "A"}
	if err := run(func(k string) string { return env[k] }); err == nil {
		t.Fatal("missing SETUP_ADMIN_PASSWORD must be refused before connecting")
	}
}

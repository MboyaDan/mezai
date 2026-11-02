package main

import "testing"

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

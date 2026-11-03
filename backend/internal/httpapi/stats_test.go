package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestStatsArePublishedAndSourcedOnly(t *testing.T) {
	e := newEnv(t)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO stats (key, label, value, unit, source, verified_at, published) VALUES
			  ('test.published', 'Published (Sample)', 12, 'items', 'Test fixture', now(), true),
			  ('test.draft', 'Draft (Sample)', 99, 'items', 'Test fixture', now(), false)`)
		return err
	})
	w := e.do(http.MethodGet, "/v1/stats", nil, nil, map[string]string{"Origin": "", "Sec-Fetch-Site": ""})
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `"test.published"`) || strings.Contains(body, "test.draft") {
		t.Fatalf("%d %s", w.Code, body)
	}
	if !strings.Contains(body, `"source":"Test fixture"`) || !strings.Contains(body, `"verified_at"`) {
		t.Fatalf("every stat carries its source and verification date: %s", body)
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("stats should be cacheable")
	}
}

package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

// Stage 16: email choices, and "new project" emails only to contributors
// who may work on the project and want them, once each.
func TestNotifications(t *testing.T) {
	e := newEnv(t)
	c1 := e.signIn("contributor001@sample.invalid")

	type prefs struct {
		ProjectAvailable bool     `json:"project_available"`
		FeedbackReceived bool     `json:"feedback_received"`
		UpdatedAt        *string  `json:"updated_at"`
		Always           []string `json:"always"`
	}
	var got prefs
	decode(t, e.do(http.MethodGet, "/v1/me/notifications", nil, c1, nil), &got)
	if !got.ProjectAvailable || !got.FeedbackReceived || got.UpdatedAt != nil || len(got.Always) != 3 {
		t.Fatalf("defaults: %+v", got)
	}
	if w := e.do(http.MethodPut, "/v1/me/notifications", map[string]any{"project_available": false}, c1, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("half a body: %d", w.Code)
	}
	w := e.do(http.MethodPut, "/v1/me/notifications", map[string]any{"project_available": false, "feedback_received": true}, c1, nil)
	decode(t, w, &got)
	if w.Code != http.StatusOK || got.ProjectAvailable || !got.FeedbackReceived || got.UpdatedAt == nil {
		t.Fatalf("set: %d %+v", w.Code, got)
	}
	if n := e.count(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.entity_id
		WHERE a.action = 'notifications.updated' AND u.email = 'contributor001@sample.invalid'
		AND a.after->>'project_available' = 'false'`); n != 1 {
		t.Fatalf("audit: %d", n)
	}
	// Staff never see another person's choices: there is no such route, and
	// Ops (no profile.manage) cannot read their own either.
	if w := e.do(http.MethodGet, "/v1/me/notifications", nil, e.signIn("ops@sample.invalid"), nil); w.Code != http.StatusForbidden {
		t.Fatalf("ops: %d", w.Code)
	}

	// A live project with no required skills: every active, verified
	// contributor may work on it. 001 switched these emails off; a disabled
	// account is skipped too.
	project := e.reviewProject(1)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE users SET status = 'disabled' WHERE email = 'contributor002@sample.invalid'`)
		return err
	})
	svc := tasks.NewService(e.pool)
	n, err := svc.NotifyProjectAvailable(context.Background(), uuid.MustParse(project))
	if err != nil {
		t.Fatal(err)
	}
	want := e.count(`SELECT count(*) FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id
		WHERE cp.status = 'active' AND cp.identity_status = 'verified' AND u.status = 'active' AND u.email_verified_at IS NOT NULL
		AND u.email NOT IN ('contributor001@sample.invalid')`)
	if n != want || n == 0 {
		t.Fatalf("queued %d, want %d", n, want)
	}
	sent := func(email string) int {
		return e.count(`SELECT count(*) FROM email_outbox WHERE template = 'project_available' AND to_address = $1
			AND data->>'ProjectID' = $2`, email, project)
	}
	if sent("contributor001@sample.invalid") != 0 || sent("contributor002@sample.invalid") != 0 || sent("contributor003@sample.invalid") != 1 {
		t.Fatalf("recipients: 001 %d, 002 %d, 003 %d", sent("contributor001@sample.invalid"), sent("contributor002@sample.invalid"), sent("contributor003@sample.invalid"))
	}
	if e.count(`SELECT count(*) FROM email_outbox WHERE template = 'project_available' AND data->>'Pay' = 'KES 25.00'
		AND data->>'TaskType' = 'Text classification' AND data->>'ProjectID' = $1`, project) != n {
		t.Fatal("email data")
	}
	// A retried job sends nothing twice.
	if _, err := svc.NotifyProjectAvailable(context.Background(), uuid.MustParse(project)); err != nil {
		t.Fatal(err)
	}
	if got := e.count(`SELECT count(*) FROM email_outbox WHERE template = 'project_available' AND data->>'ProjectID' = $1`, project); got != n {
		t.Fatalf("after retry: %d, want %d", got, n)
	}
	// A project paused before the job runs tells nobody.
	paused := e.reviewProject(1)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE projects SET status = 'paused' WHERE id = $1`, paused)
		return err
	})
	if n, err := svc.NotifyProjectAvailable(context.Background(), uuid.MustParse(paused)); err != nil || n != 0 {
		t.Fatalf("paused project: %d %v", n, err)
	}
}

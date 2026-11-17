package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Stage 17c, endpoint-by-endpoint tenancy review as a test. Every route a
// client, contributor or reviewer can call with an :id is called by the wrong
// person with someone else's ID, and must answer 403 or 404 without data.
// The route list comes from the router, so a new route without a fixture
// here fails the test: adding a route means reviewing its scoping.
func TestCrossTenantMatrix(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ops := e.signIn("ops@sample.invalid")
	owner := e.signIn("client.admin@sample.invalid")
	other := e.signIn("other.admin@sample.invalid")
	c1, c2 := e.signIn("contributor001@sample.invalid"), e.signIn("contributor002@sample.invalid")
	reviewer := e.signIn("reviewer@sample.invalid")

	// Sample Client Ltd's project with final items (and a deliverable), and
	// a second one whose items disagree and wait for review.
	final := e.reviewProject(2)
	e.workAll(c1, final, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, final, map[string]any{"labels": []string{"Yes"}})
	if w := e.do(http.MethodPost, "/v1/projects/"+final+"/deliverables", nil, ops, nil); w.Code >= 300 {
		t.Fatalf("export: %d %s", w.Code, w.Body)
	}
	disputed := e.reviewProject(1)
	e.workAll(c1, disputed, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, disputed, map[string]any{"labels": []string{"No"}})

	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		e.staff(func(tx pgx.Tx) error { return tx.QueryRow(ctx, sql, args...).Scan(&v) })
		return v
	}
	draft := id(`INSERT INTO projects (org_id, template_id, task_type, name, config)
		SELECT o.id, t.id, 'classification', 'Draft tenancy check (Sample)', '{"labels":["Yes","No"],"multi_label":false}'
		FROM organisations o, task_templates t WHERE o.slug = 'sample-client-ltd' AND t.task_type = 'classification'
		ORDER BY t.version DESC LIMIT 1 RETURNING id`)
	importJob := id(`INSERT INTO import_jobs (org_id, project_id, kind, format, file_key, file_name)
		SELECT org_id, id, 'items', 'csv', 'imports/tenancy.csv', 'items.csv' FROM projects WHERE id = $1 RETURNING id`, final)
	request := id(`INSERT INTO project_requests (org_id, title, task_type, brief_md, contact_name, contact_email,
		  sample_file_key, sample_file_name, sample_size, created_by)
		SELECT o.id, 'Tenancy request (Sample)', 'classification', 'Sample.', 'Sample Person', 'person@sample.invalid',
		  'requests/tenancy.csv', 'sample.csv', 10, u.id
		FROM organisations o, users u WHERE o.slug = 'sample-client-ltd' AND u.email = 'client.admin@sample.invalid' RETURNING id`)
	invoice := id(`INSERT INTO invoices (org_id, number, issued_on, amount_cents, currency, description, file_key, file_name,
		  byte_size, checksum_sha256, status, uploaded_by)
		SELECT o.id, 'INV-TENANCY', current_date, 100000, 'KES', 'Sample.', 'invoices/tenancy.pdf', 'inv.pdf', 10,
		  repeat('0', 64), 'issued', u.id
		FROM organisations o, users u WHERE o.slug = 'sample-client-ltd' AND u.email = 'ops@sample.invalid' RETURNING id`)
	invitation := id(`INSERT INTO invitations (email, role_id, org_id, token_hash, invited_by, expires_at)
		SELECT 'invitee@sample.invalid', r.id, o.id, sha256(random()::text::bytea), u.id, now() + interval '7 days'
		FROM roles r, organisations o, users u
		WHERE r.key = 'client_viewer' AND o.slug = 'sample-client-ltd' AND u.email = 'client.admin@sample.invalid' RETURNING id`)
	deliverable := id(`SELECT id FROM deliverables WHERE project_id = $1 LIMIT 1`, final)
	task := id(`SELECT a.id FROM assignments a JOIN contributor_profiles cp ON cp.id = a.contributor_id
		JOIN users u ON u.id = cp.user_id WHERE u.email = 'contributor001@sample.invalid' AND a.project_id = $1 LIMIT 1`, final)
	attempt := id(`SELECT a.id FROM qualification_attempts a JOIN contributor_profiles cp ON cp.id = a.contributor_id
		JOIN users u ON u.id = cp.user_id WHERE u.email = 'contributor001@sample.invalid' LIMIT 1`)
	feedback := id(`INSERT INTO feedback (org_id, project_id, contributor_id, body)
		SELECT p.org_id, p.id, cp.id, 'Sample feedback.' FROM projects p, contributor_profiles cp JOIN users u ON u.id = cp.user_id
		WHERE p.id = $1 AND u.email = 'contributor001@sample.invalid' RETURNING id`, final)
	reviewItem := id(`SELECT id FROM items WHERE project_id = $1 AND status = 'in_review' LIMIT 1`, disputed)

	type probe struct {
		caller, rightful *session
		id               string
		body             any
		headers          map[string]string
		why              string
	}
	asOther := func(target string, body any) probe {
		return probe{caller: other, rightful: owner, id: target, body: body, why: "another organisation's client admin"}
	}
	asC2 := func(target string, body any) probe {
		return probe{caller: c2, rightful: c1, id: target, body: body, why: "another contributor"}
	}
	probes := map[string]probe{
		"DELETE /v1/invitations/:id":          asOther(invitation, nil),
		"GET /v1/projects/:id":                asOther(final, nil),
		"GET /v1/projects/:id/retention":      asOther(final, nil),
		"GET /v1/projects/:id/quality":        asOther(final, nil),
		"GET /v1/projects/:id/deliverables":   asOther(final, nil),
		"GET /v1/projects/:id/progress":       asOther(final, nil),
		"GET /v1/projects/:id/workforce":      asOther(final, nil),
		"GET /v1/projects/:id/imports":        asOther(final, nil),
		"GET /v1/imports/:id":                 asOther(importJob, nil),
		"GET /v1/project-requests/:id":        asOther(request, nil),
		"GET /v1/project-requests/:id/sample": asOther(request, nil),
		"POST /v1/deliverables/:id/links":     asOther(deliverable, map[string]string{"file": "data"}),
		"GET /v1/deliverables/:id/download":   asOther(deliverable, nil),
		"GET /v1/invoices/:id/download":       asOther(invoice, nil),
		"GET /v1/me/tasks/:id":                asC2(task, nil),
		"PUT /v1/me/tasks/:id/draft":          asC2(task, map[string]any{"answer": map[string]any{"labels": []string{"No"}}}),
		"POST /v1/me/tasks/:id/submit": {caller: c2, id: task, body: map[string]any{"answer": map[string]any{"labels": []string{"No"}}},
			headers: map[string]string{"Idempotency-Key": "tenancy-submit-1"}, why: "another contributor"},
		"POST /v1/me/tasks/:id/skip":                    asC2(task, map[string]any{"reason": "unclear"}),
		"POST /v1/me/tasks/:id/report":                  asC2(task, map[string]any{"category": "other", "note": "x"}),
		"POST /v1/me/feedback/:id/read":                 asC2(feedback, nil),
		"GET /v1/me/qualification/attempts/:id":         asC2(attempt, nil),
		"PUT /v1/me/qualification/attempts/:id/answers": asC2(attempt, map[string]any{"answers": map[string]any{}}),
		"POST /v1/me/qualification/attempts/:id/submit": asC2(attempt, map[string]any{"answers": map[string]any{}}),
		"GET /v1/me/projects/:id":                       {caller: c2, id: draft, why: "a contributor, for a project not open to them"},
		"POST /v1/me/projects/:id/next":                 {caller: c2, id: draft, why: "a contributor, for a project not open to them"},
		"GET /v1/review/items/:id":                      {caller: reviewer, rightful: ops, id: reviewItem, why: "a reviewer not assigned to the project"},
		"POST /v1/review/items/:id/decision": {caller: reviewer, id: reviewItem, body: map[string]any{"decision": "accept_consensus"},
			headers: map[string]string{"Idempotency-Key": "tenancy-decision-1"}, why: "a reviewer not assigned to the project"},
	}
	// Reachable by these roles with an :id but not owned by anyone.
	shared := map[string]string{
		"POST /v1/me/qualification/tests/:id/attempts": "qualification tests are open to every eligible contributor",
	}

	reachable := map[string]bool{}
	for _, r := range e.rules {
		key := r.Method + " " + r.Path
		if !strings.Contains(r.Path, ":id") {
			continue
		}
		for _, k := range r.Access.Permissions {
			if strings.HasPrefix(k, "org.") || k == "task.submit" || k == "qualification.take" || k == "profile.manage" ||
				k == "earnings.read_own" || k == "review.read" || k == "review.decide" {
				reachable[key] = true
			}
		}
	}
	if len(reachable) == 0 {
		t.Fatal("no routes found: the rule format changed")
	}
	for key := range reachable {
		if _, ok := probes[key]; !ok && shared[key] == "" {
			t.Errorf("%s has no cross-tenant probe: add one here", key)
		}
	}
	for key, p := range probes {
		if !reachable[key] {
			t.Errorf("%s is probed but no longer reachable by these roles: update the test", key)
			continue
		}
		method, path, _ := strings.Cut(key, " ")
		url := strings.Replace(path, ":id", p.id, 1)
		w := e.do(method, url, p.body, p.caller, p.headers)
		if key == "POST /v1/me/feedback/:id/read" && w.Code == http.StatusNoContent {
			// Idempotent by design (204 whether or not anything changed); the
			// update is scoped to the caller, so someone else's note stays unread.
			if e.count(`SELECT count(*) FROM feedback WHERE id = $1 AND read_at IS NOT NULL`, p.id) != 0 {
				t.Errorf("%s as %s marked someone else's feedback", key, p.why)
			}
			continue
		}
		if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Errorf("%s as %s: %d %s", key, p.why, w.Code, w.Body)
			continue
		}
		if b := w.Body.String(); strings.Contains(b, "Sample sentence") || strings.Contains(b, "Tenancy") || strings.Contains(b, p.id) {
			t.Errorf("%s as %s leaks data: %s", key, p.why, b)
		}
		// The rightful caller is not refused, so the probe used a real ID.
		if p.rightful != nil && method == http.MethodGet && !strings.HasSuffix(path, "/download") {
			if w := e.do(method, url, nil, p.rightful, nil); w.Code == http.StatusForbidden || w.Code == http.StatusNotFound {
				t.Errorf("%s: the rightful caller got %d, so the probe proves nothing", key, w.Code)
			}
		}
	}
}

// Routes without an :id that take an organisation or project as a query or
// body parameter: a client passing another organisation's ID, or a reviewer
// another project's, gets a refusal or an answer without that data.
func TestCrossTenantParameters(t *testing.T) {
	e := newEnv(t)
	other := e.signIn("other.admin@sample.invalid")
	reviewer := e.signIn("reviewer@sample.invalid")
	c1, c2 := e.signIn("contributor001@sample.invalid"), e.signIn("contributor002@sample.invalid")
	sampleOrg := e.orgID("sample-client-ltd")
	disputed := e.reviewProject(1)
	e.workAll(c1, disputed, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, disputed, map[string]any{"labels": []string{"No"}})

	// Anything that names Sample Client Ltd or its projects is a leak.
	var sampleProjects []string
	e.staff(func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT p.id::text FROM projects p JOIN organisations o ON o.id = p.org_id WHERE o.slug = 'sample-client-ltd'`)
		if err != nil {
			return err
		}
		sampleProjects, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	leaks := func(body string) bool {
		if strings.Contains(body, sampleOrg) || strings.Contains(body, "Sample Client Ltd") {
			return true
		}
		for _, p := range sampleProjects {
			if strings.Contains(body, p) {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		method, path string
		body         any
		s            *session
	}{
		{http.MethodGet, "/v1/projects?org_id=" + sampleOrg, nil, other},
		{http.MethodGet, "/v1/activity?org_id=" + sampleOrg, nil, other},
		{http.MethodGet, "/v1/team?org_id=" + sampleOrg, nil, other},
		{http.MethodGet, "/v1/invoices?org_id=" + sampleOrg, nil, other},
		{http.MethodGet, "/v1/invitations?org_id=" + sampleOrg, nil, other},
		{http.MethodGet, "/v1/project-requests?org_id=" + sampleOrg, nil, other},
		{http.MethodPost, "/v1/invitations", map[string]string{"email": "x@sample.invalid", "role": "client_viewer", "org_id": sampleOrg}, other},
		{http.MethodPost, "/v1/project-requests", map[string]any{"org_id": sampleOrg, "title": "Cross-org (Sample)", "task_type": "classification",
			"brief_md": "Sample.", "contact_name": "Sample", "contact_email": "x@sample.invalid"}, other},
		{http.MethodGet, "/v1/review/items?project_id=" + disputed, nil, reviewer},
	} {
		w := e.do(c.method, c.path, c.body, c.s, nil)
		if w.Code >= 500 || leaks(w.Body.String()) {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}
	// A request created by the other admin belongs to their own organisation.
	if n := e.count(`SELECT count(*) FROM project_requests r JOIN organisations o ON o.id = r.org_id
		WHERE r.title = 'Cross-org (Sample)' AND o.slug <> 'other-sample-org'`); n != 0 {
		t.Fatalf("request filed under another organisation: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM invitations WHERE email = 'x@sample.invalid' AND org_id = $1`, sampleOrg); n != 0 {
		t.Fatalf("invitation into another organisation: %d", n)
	}
}

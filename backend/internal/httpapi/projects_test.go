package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/httpapi"
	"github.com/MboyaDan/mezai/backend/seed"
)

type projectResp struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	ItemCount int    `json:"item_count"`
	Staff     *struct {
		PayPerTaskCents int64    `json:"pay_per_task_cents"`
		GoldCount       int      `json:"gold_count"`
		Missing         []string `json:"missing"`
		ConfigLocked    bool     `json:"config_locked"`
	} `json:"staff"`
}

type importResp struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Kind      string `json:"kind"`
	TotalRows int    `json:"total_rows"`
	ValidRows int    `json:"valid_rows"`
	ErrorRows int    `json:"error_rows"`
	Errors    []struct {
		Row     int    `json:"row"`
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"errors"`
}

// multipart sends form fields and one file.
func (e *env) multipart(path string, fields map[string]string, fileField, fileName, content string, s *session) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if fileField != "" {
		fw, _ := mw.CreateFormFile(fileField, fileName)
		_, _ = fw.Write([]byte(content))
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", origin)
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: s.cookie})
	req.Header.Set(httpapi.HeaderCSRF, s.csrf)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *env) upload(s *session, projectID, kind, name, content string) (*httptest.ResponseRecorder, importResp) {
	e.t.Helper()
	w := e.multipart("/v1/projects/"+projectID+"/imports", map[string]string{"kind": kind}, "file", name, content, s)
	var out importResp
	if w.Code == http.StatusCreated {
		decode(e.t, w, &out)
	}
	return w, out
}

func (e *env) signIn(email string) *session {
	e.t.Helper()
	w, s := e.login(email, pw)
	if s == nil {
		e.t.Fatalf("login %s: %d %s", email, w.Code, w.Body)
	}
	return s
}

func classificationProject(orgID string) map[string]any {
	return map[string]any{
		"org_id": orgID, "task_type": "classification", "name": "Support ticket routing (Sample)",
		"config":    map[string]any{"labels": []string{"Billing", "Network", "Other"}},
		"overlap_k": 3, "gold_rate": 0.1, "pay_per_task_cents": 1500,
		"skills": []map[string]any{{"kind": "language", "value": "sw", "min_proficiency": "fluent"}},
	}
}

// TestProjectImportAC05 walks AC-05: Ops creates a project and imports CSV or
// JSONL with a row-level report; an invalid file imports nothing.
func TestProjectImportAC05(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")

	// Organisations.
	w := e.do(http.MethodPost, "/v1/organisations", map[string]string{"name": "Pilot Buyer Ltd (Sample)"}, ops, nil)
	var org struct{ ID, Slug string }
	if w.Code != http.StatusCreated {
		t.Fatalf("create org: %d %s", w.Code, w.Body)
	}
	decode(t, w, &org)
	if org.Slug != "pilot-buyer-ltd-sample" {
		t.Fatalf("slug = %q", org.Slug)
	}
	if w := e.do(http.MethodPost, "/v1/organisations", map[string]string{"name": "Pilot Buyer Ltd (Sample)"}, ops, nil); errorCode(w) != "slug_taken" {
		t.Fatalf("duplicate org: %d %s", w.Code, w.Body)
	}

	// Project settings are validated per task type.
	bad := classificationProject(org.ID)
	bad["config"] = map[string]any{"labels": []string{"Only"}}
	if w := e.do(http.MethodPost, "/v1/projects", bad, ops, nil); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "config.labels") {
		t.Fatalf("one label: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodPost, "/v1/projects", classificationProject(org.ID), ops, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", w.Code, w.Body)
	}
	var p projectResp
	decode(t, w, &p)
	if p.Status != "draft" || p.Staff == nil || !slices.Equal(p.Staff.Missing, []string{"instructions"}) {
		t.Fatalf("new project = %+v %+v", p, p.Staff)
	}
	status := func(action string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/v1/projects/"+p.ID+"/status", map[string]string{"action": action}, ops, nil)
	}
	if w := status("ready"); errorCode(w) != "not_ready" {
		t.Fatalf("ready without instructions: %d %s", w.Code, w.Body)
	}
	if w := status("pause"); errorCode(w) != "illegal_transition" {
		t.Fatalf("pause a draft: %d %s", w.Code, w.Body)
	}
	edit := classificationProject(org.ID)
	edit["instructions_md"] = "Choose the team that should answer the message."
	if w := e.do(http.MethodPatch, "/v1/projects/"+p.ID, edit, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	if w := status("ready"); w.Code != http.StatusOK {
		t.Fatalf("ready: %d %s", w.Code, w.Body)
	}
	w = status("publish")
	if errorCode(w) != "not_ready" || !strings.Contains(w.Body.String(), `"items","gold"`) {
		t.Fatalf("publish without items: %d %s", w.Code, w.Body)
	}

	// An invalid file gets a row-level report and imports nothing.
	items := func() int { return e.count(`SELECT count(*) FROM items WHERE project_id = $1`, p.ID) }
	w, rep := e.upload(ops, p.ID, "items", "batch.csv", "external_id,text\nm-1,Bundle ended early\nm-2,\nm-1,Repeat\n")
	if w.Code != http.StatusCreated || rep.Status != "failed" || rep.TotalRows != 3 || rep.ErrorRows != 2 || len(rep.Errors) != 2 {
		t.Fatalf("invalid file: %d %s", w.Code, w.Body)
	}
	if rep.Errors[0].Row != 3 || rep.Errors[0].Field != "text" || rep.Errors[1].Row != 4 || rep.Errors[1].Field != "external_id" {
		t.Fatalf("errors = %+v", rep.Errors)
	}
	if w := e.do(http.MethodPost, "/v1/imports/"+rep.ID+"/commit", nil, ops, nil); errorCode(w) != "import_not_committable" {
		t.Fatalf("commit invalid: %d %s", w.Code, w.Body)
	}
	if n := items(); n != 0 {
		t.Fatalf("invalid file imported %d items", n)
	}
	var got importResp
	decode(t, e.do(http.MethodGet, "/v1/imports/"+rep.ID, nil, ops, nil), &got)
	if len(got.Errors) != 2 {
		t.Fatalf("stored report = %+v", got)
	}

	// A valid CSV commits every row, once.
	_, rep = e.upload(ops, p.ID, "items", "batch.csv", "external_id,text\nm-1,Bundle ended early\nm-2,Calls drop in town\nm-3,\"Shop hours, please\"\n")
	if rep.Status != "validated" || rep.ValidRows != 3 {
		t.Fatalf("valid file: %+v", rep)
	}
	if n := items(); n != 0 {
		t.Fatalf("dry run imported %d items", n)
	}
	if w := e.do(http.MethodPost, "/v1/imports/"+rep.ID+"/commit", nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/imports/"+rep.ID+"/commit", nil, ops, nil); errorCode(w) != "import_not_committable" {
		t.Fatalf("second commit: %d %s", w.Code, w.Body)
	}
	if n := items(); n != 3 {
		t.Fatalf("items = %d", n)
	}
	// IDs already in the project are refused.
	if _, rep := e.upload(ops, p.ID, "items", "more.jsonl", `{"external_id":"m-2","text":"Again"}`+"\n"); rep.Status != "failed" || rep.Errors[0].Message != "is already used in this project" {
		t.Fatalf("reused ID: %+v", rep)
	}

	// Gold from JSONL, then the project can go live.
	gold := `{"external_id":"g-1","text":"My airtime vanished","expected":{"labels":["Billing"]}}` + "\n" +
		`{"external_id":"g-2","text":"No signal at home","expected":{"labels":["Network"]}}` + "\n"
	_, rep = e.upload(ops, p.ID, "gold", "gold.jsonl", gold)
	if rep.Status != "validated" {
		t.Fatalf("gold file: %+v", rep)
	}
	if w := e.do(http.MethodPost, "/v1/imports/"+rep.ID+"/commit", nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("commit gold: %d %s", w.Code, w.Body)
	}
	if w := status("publish"); w.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodGet, "/v1/projects/"+p.ID, nil, ops, nil), &p)
	if p.Status != "live" || p.ItemCount != 3 || p.Staff.GoldCount != 2 || !p.Staff.ConfigLocked {
		t.Fatalf("live project = %+v %+v", p, p.Staff)
	}
	// Labels are fixed once there is gold.
	edit["config"] = map[string]any{"labels": []string{"Billing", "Network"}}
	if w := e.do(http.MethodPatch, "/v1/projects/"+p.ID, edit, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("locked config: %d %s", w.Code, w.Body)
	}

	// Gold list, hand-made gold, retire.
	var gl struct {
		Gold []struct{ ID, Status string } `json:"gold"`
	}
	decode(t, e.do(http.MethodGet, "/v1/projects/"+p.ID+"/gold", nil, ops, nil), &gl)
	if len(gl.Gold) != 2 {
		t.Fatalf("gold = %+v", gl)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+p.ID+"/gold", map[string]any{
		"content": map[string]string{"text": "Change my account name"}, "expected": map[string]any{"labels": []string{"Account"}},
	}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("gold with unknown label: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+p.ID+"/gold", map[string]any{
		"content": map[string]string{"text": "Thanks for your help"}, "expected": map[string]any{"labels": []string{"Other"}},
	}, ops, nil); w.Code != http.StatusCreated {
		t.Fatalf("gold by hand: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/gold/"+gl.Gold[0].ID+"/retire", nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("retire: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/gold/"+gl.Gold[0].ID+"/retire", nil, ops, nil); errorCode(w) != "illegal_transition" {
		t.Fatalf("retire twice: %d %s", w.Code, w.Body)
	}

	// A file changed in storage after the check is refused.
	_, rep = e.upload(ops, p.ID, "items", "late.csv", "text\nOne more\n")
	var key string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT file_key FROM import_jobs WHERE id = $1`, rep.ID).Scan(&key)
	})
	e.files.Corrupt(key, []byte("text\nSwapped\nRows\n"))
	if w := e.do(http.MethodPost, "/v1/imports/"+rep.ID+"/commit", nil, ops, nil); errorCode(w) != "file_changed" {
		t.Fatalf("changed file: %d %s", w.Code, w.Body)
	}

	// Two checked files with the same ID: the second goes stale and imports nothing.
	_, a := e.upload(ops, p.ID, "items", "a.csv", "external_id,text\nx-1,First\n")
	_, b := e.upload(ops, p.ID, "items", "b.csv", "external_id,text\nx-2,Second\nx-1,Clash\n")
	if a.Status != "validated" || b.Status != "validated" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	e.do(http.MethodPost, "/v1/imports/"+a.ID+"/commit", nil, ops, nil)
	before := items()
	if w := e.do(http.MethodPost, "/v1/imports/"+b.ID+"/commit", nil, ops, nil); errorCode(w) != "import_stale" {
		t.Fatalf("stale: %d %s", w.Code, w.Body)
	}
	if items() != before {
		t.Fatal("a stale file imported rows")
	}

	// Closing stops imports.
	if w := status("close"); w.Code != http.StatusOK {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	if w, _ := e.upload(ops, p.ID, "items", "c.csv", "text\nLate\n"); errorCode(w) != "project_closed" {
		t.Fatalf("import into closed: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action IN ('project.created', 'import.checked', 'import.committed', 'project.status_changed', 'gold.retired')`); n < 10 {
		t.Fatalf("audit rows = %d", n)
	}
}

func TestImportPermissionsAndFiles(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")
	pid := e.projectID("Customer message routing (Sample)")

	reviewer := e.signIn("reviewer@sample.invalid")
	if w, _ := e.upload(reviewer, pid, "items", "a.csv", "text\nHi\n"); w.Code != http.StatusForbidden {
		t.Fatalf("reviewer import: %d", w.Code)
	}
	for name, c := range map[string]struct{ kind, file, code string }{
		"spreadsheet": {"items", "a.xlsx", "invalid_input"},
		"kind":        {"labels", "a.csv", "invalid_input"},
	} {
		if w, _ := e.upload(ops, pid, c.kind, c.file, "text\nHi\n"); errorCode(w) != c.code {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	big := "text\n" + strings.Repeat("x", 10<<20) + "\n"
	if w, _ := e.upload(ops, pid, "items", "big.csv", big); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("10 MB+: %d", w.Code)
	}
}

func TestClientProjectIsolation(t *testing.T) {
	e := newEnv(t)
	client := e.signIn("client.admin@sample.invalid")
	own := e.projectID("Customer message routing (Sample)")
	foreign := e.projectID("Other org message routing (Sample)")

	var list struct{ Projects []projectResp }
	decode(t, e.do(http.MethodGet, "/v1/projects", nil, client, nil), &list)
	if len(list.Projects) == 0 {
		t.Fatal("client sees no projects")
	}
	for _, p := range list.Projects {
		if p.OrgID != e.orgID(seed.SampleOrgSlug) || p.Staff != nil {
			t.Fatalf("client sees %+v", p)
		}
	}
	if w := e.do(http.MethodGet, "/v1/projects/"+foreign, nil, client, nil); w.Code != http.StatusNotFound {
		t.Fatalf("foreign project: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/projects/"+foreign+"/imports", nil, client, nil); w.Code != http.StatusNotFound {
		t.Fatalf("foreign imports: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/projects", classificationProject(e.orgID(seed.SampleOrgSlug)), client, nil); w.Code != http.StatusForbidden {
		t.Fatalf("client creates project: %d", w.Code)
	}

	// Clients see their item imports (dataset versions) but never gold files.
	ops := e.signIn("ops@sample.invalid")
	_, items := e.upload(ops, own, "items", "v2.csv", "text\nHello\n")
	_, gold := e.upload(ops, own, "gold", "g.csv", "text,expected_labels\nHello,Billing\n")
	var imps struct{ Imports []importResp }
	decode(t, e.do(http.MethodGet, "/v1/projects/"+own+"/imports", nil, client, nil), &imps)
	if len(imps.Imports) != 1 || imps.Imports[0].ID != items.ID {
		t.Fatalf("client imports = %+v", imps)
	}
	if w := e.do(http.MethodGet, "/v1/imports/"+gold.ID, nil, client, nil); w.Code != http.StatusNotFound {
		t.Fatalf("client reads gold import: %d", w.Code)
	}
	other := e.signIn("other.admin@sample.invalid")
	if w := e.do(http.MethodGet, "/v1/imports/"+items.ID, nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other org reads import: %d", w.Code)
	}
}

func TestProjectRequests(t *testing.T) {
	e := newEnv(t)
	client := e.signIn("client.admin@sample.invalid")
	req := map[string]any{
		"title": "Sentiment labels for support chats (Sample)", "task_type": "classification",
		"brief_md": "About 5,000 short chats in Swahili and English.", "languages": []string{"sw", "en", "sw"},
		"volume_estimate": 5000, "contact_name": "Sample Client Admin", "contact_email": "client.admin@sample.invalid",
	}
	body, _ := json.Marshal(req)
	w := e.multipart("/v1/project-requests", map[string]string{"request": string(body)}, "sample", "chats.csv", "text\nHabari\n", client)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var r struct {
		ID, Status, DecisionNote string
		Languages                []string
		SampleFileName           *string `json:"sample_file_name"`
		ProjectID                *string `json:"project_id"`
	}
	decode(t, w, &r)
	if r.Status != "submitted" || len(r.Languages) != 2 || r.SampleFileName == nil || *r.SampleFileName != "chats.csv" {
		t.Fatalf("request = %+v", r)
	}
	if n := e.count(`SELECT count(*) FROM email_outbox WHERE template = 'project_request_received'`); n != 1 {
		t.Fatalf("ops emails = %d", n)
	}

	// Who may submit: client admins of an organisation only.
	viewer := e.signIn("client.viewer@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/project-requests", req, viewer, nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer submits: %d", w.Code)
	}
	admin := e.signIn("superadmin@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/project-requests", req, admin, nil); w.Code != http.StatusForbidden {
		t.Fatalf("staff submits: %d %s", w.Code, w.Body)
	}
	bad := map[string]any{"title": "x", "task_type": "classification", "brief_md": "x", "contact_name": "x", "contact_email": "not-an-email"}
	if w := e.do(http.MethodPost, "/v1/project-requests", bad, client, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad email: %d", w.Code)
	}
	if w := e.multipart("/v1/project-requests", map[string]string{"request": string(body)}, "sample", "page.html", "<b>x</b>", client); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("html sample: %d", w.Code)
	}

	// Another organisation cannot see it.
	other := e.signIn("other.admin@sample.invalid")
	if w := e.do(http.MethodGet, "/v1/project-requests/"+r.ID, nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other org reads request: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/project-requests/"+r.ID+"/sample", nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other org downloads sample: %d", w.Code)
	}

	// Ops downloads the sample and converts the request.
	ops := e.signIn("ops@sample.invalid")
	w = e.do(http.MethodGet, "/v1/project-requests/"+r.ID+"/sample", nil, ops, nil)
	if w.Code != http.StatusOK || w.Body.String() != "text\nHabari\n" ||
		!strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("sample: %d %v", w.Code, w.Header())
	}
	project := classificationProject("")
	delete(project, "org_id")
	w = e.do(http.MethodPost, "/v1/project-requests/"+r.ID+"/convert", project, ops, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("convert: %d %s", w.Code, w.Body)
	}
	var p projectResp
	decode(t, w, &p)
	if p.OrgID != e.orgID(seed.SampleOrgSlug) || p.Status != "draft" {
		t.Fatalf("converted project = %+v", p)
	}
	if w := e.do(http.MethodPost, "/v1/project-requests/"+r.ID+"/convert", project, ops, nil); errorCode(w) != "already_decided" {
		t.Fatalf("convert twice: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodGet, "/v1/project-requests/"+r.ID, nil, client, nil), &r)
	if r.Status != "converted" || r.ProjectID == nil || *r.ProjectID != p.ID {
		t.Fatalf("client view = %+v", r)
	}

	// Declining needs a note the client then sees.
	w = e.do(http.MethodPost, "/v1/project-requests", req, client, nil)
	decode(t, w, &r)
	if w := e.do(http.MethodPost, "/v1/project-requests/"+r.ID+"/decline", map[string]string{"note": " "}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("decline without note: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/project-requests/"+r.ID+"/decline", map[string]string{"note": "We do not cover this language yet."}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("decline: %d %s", w.Code, w.Body)
	}
	var mine struct {
		Requests []struct {
			Status       string  `json:"status"`
			DecisionNote *string `json:"decision_note"`
		}
	}
	decode(t, e.do(http.MethodGet, "/v1/project-requests?status=declined", nil, client, nil), &mine)
	if len(mine.Requests) != 1 || *mine.Requests[0].DecisionNote != "We do not cover this language yet." {
		t.Fatalf("client declined list = %+v", mine)
	}
	decode(t, e.do(http.MethodGet, "/v1/project-requests", nil, other, nil), &mine)
	if len(mine.Requests) != 0 {
		t.Fatalf("other org sees %d requests", len(mine.Requests))
	}
}

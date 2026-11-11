package httpapi_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type progressResp struct {
	Total, Final, InReview int
	NotStarted             int `json:"not_started"`
	Throughput             []struct {
		Week  string `json:"week"`
		Final int    `json:"final"`
	} `json:"throughput"`
	Pace struct {
		Goal, Remaining int
		State           string `json:"state"`
	} `json:"pace"`
}

type workforceResp struct {
	Contributors int `json:"contributors"`
	Active       int `json:"active_last_7_days"`
	Workers      []struct {
		Code  string `json:"code"`
		Tasks int    `json:"tasks"`
	} `json:"workers"`
	Skills []struct {
		Kind, Value  string
		Contributors int
	} `json:"skills"`
}

type activityResp struct {
	Entries []struct {
		Action string `json:"action"`
		Actor  struct {
			Kind, Name string
		} `json:"actor"`
		ProjectName *string `json:"project_name"`
		Subject     string  `json:"subject"`
	} `json:"entries"`
}

func (a activityResp) actions() []string {
	var out []string
	for _, x := range a.Entries {
		out = append(out, x.Action)
	}
	return out
}

type invoiceResp struct {
	ID, Number, Status string
	OrgID              string  `json:"org_id"`
	PaidOn             *string `json:"paid_on"`
	VoidReason         *string `json:"void_reason"`
}

const invoicePDF = "%PDF-1.4\n% Sample invoice for tests only\n%%EOF\n"

var clientCode = regexp.MustCompile(`^C-[A-Z][0-9A-Z]{5}$`)

// TestClientPortal is AC-10 through the API for the Stage 13a endpoints: a
// client reads progress, workforce, activity, team and invoices of its own
// organisation only, and contributors appear only as per-client codes.
func TestClientPortal(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("ops@sample.invalid")
	admin := e.signIn("client.admin@sample.invalid")
	viewer := e.signIn("client.viewer@sample.invalid")
	other := e.signIn("other.admin@sample.invalid")
	sampleOrg, otherOrg := e.orgID("sample-client-ltd"), e.orgID("other-sample-org")

	// Two contributors finish a four-item project (k = 2, consensus).
	project := e.reviewProject(4)
	for _, who := range []string{"contributor001@sample.invalid", "contributor002@sample.invalid"} {
		e.workAll(e.signIn(who), project, map[string]any{"labels": []string{"Yes"}})
	}

	// Progress: the client and Ops read the same figures; gold is never counted.
	var pr progressResp
	e.get(admin, "/v1/projects/"+project+"/progress", &pr)
	if pr.Total != 4 || pr.Final != 4 || pr.Pace.State != "done" || len(pr.Throughput) != 8 || pr.Throughput[7].Final != 4 {
		t.Fatalf("progress: %+v", pr)
	}
	var opsPr progressResp
	e.get(ops, "/v1/projects/"+project+"/progress", &opsPr)
	if opsPr.Final != pr.Final || opsPr.Total != pr.Total {
		t.Fatalf("ops progress %+v differs from client %+v", opsPr, pr)
	}
	e.get(viewer, "/v1/projects/"+project+"/progress", &pr)

	// Workforce: codes only, stable for the organisation, never a staff-side
	// public code, name, email or contributor id.
	w := e.do(http.MethodGet, "/v1/projects/"+project+"/workforce", nil, admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("workforce: %d %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, leak := range []string{"contributor001", "Sample Contributor", e.contributorID("contributor001@sample.invalid"), `"C-0`} {
		if strings.Contains(body, leak) {
			t.Errorf("workforce leaks %q: %s", leak, body)
		}
	}
	var wf workforceResp
	decode(t, w, &wf)
	if wf.Contributors != 2 || wf.Active != 2 || len(wf.Workers) != 2 || wf.Workers[0].Tasks != 4 {
		t.Fatalf("workforce: %+v", wf)
	}
	for _, x := range wf.Workers {
		if !clientCode.MatchString(x.Code) {
			t.Errorf("code %q is not a client code", x.Code)
		}
	}
	var again workforceResp
	e.get(ops, "/v1/projects/"+project+"/workforce", &again)
	if again.Workers[0].Code != wf.Workers[0].Code {
		t.Errorf("codes are not stable: %s vs %s", again.Workers[0].Code, wf.Workers[0].Code)
	}

	// Another organisation's client: 404 on every project read.
	for _, path := range []string{"/progress", "/workforce", "/quality", ""} {
		if w := e.do(http.MethodGet, "/v1/projects/"+project+path, nil, other, nil); w.Code != http.StatusNotFound {
			t.Errorf("other org GET project%s: %d %s", path, w.Code, w.Body)
		}
	}

	// Invoices: Ops uploads PDFs only, numbers are unique per organisation,
	// and a project must belong to the invoice's organisation.
	inv := func(fields map[string]string, name, content string) *http.Response {
		w := e.multipart("/v1/invoices", fields, "file", name, content, ops)
		return w.Result()
	}
	base := map[string]string{"org_id": sampleOrg, "number": "MZ-2026-001", "issued_on": "2026-11-01",
		"due_on": "2026-11-30", "amount_cents": "1250000", "description": "Pilot work, October (Sample)", "project_id": project}
	if r := inv(base, "invoice.txt", "not a pdf"); r.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("non-PDF upload: %d", r.StatusCode)
	}
	if r := inv(map[string]string{"org_id": otherOrg, "number": "MZ-X", "issued_on": "2026-11-01", "amount_cents": "1", "project_id": project},
		"x.pdf", invoicePDF); r.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("project of another org: %d", r.StatusCode)
	}
	up := e.multipart("/v1/invoices", base, "file", "October.pdf", invoicePDF, ops)
	if up.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", up.Code, up.Body)
	}
	var invoice invoiceResp
	decode(t, up, &invoice)
	if r := inv(base, "again.pdf", invoicePDF); r.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate number: %d", r.StatusCode)
	}
	if r := inv(map[string]string{"org_id": otherOrg, "number": "MZ-2026-001", "issued_on": "2026-11-02", "amount_cents": "500"},
		"other.pdf", invoicePDF); r.StatusCode != http.StatusCreated {
		t.Fatalf("same number, other org: %d", r.StatusCode)
	}

	var list struct{ Invoices []invoiceResp }
	e.get(admin, "/v1/invoices", &list)
	if len(list.Invoices) != 1 || list.Invoices[0].ID != invoice.ID {
		t.Fatalf("admin invoices: %+v", list)
	}
	e.get(other, "/v1/invoices?org_id="+sampleOrg, &list)
	if len(list.Invoices) != 0 {
		t.Fatalf("other org reads sample invoices: %+v", list)
	}
	if w := e.do(http.MethodGet, "/v1/invoices", nil, viewer, nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer lists invoices: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/invoices/"+invoice.ID+"/download", nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other org download: %d", w.Code)
	}
	dl := e.do(http.MethodGet, "/v1/invoices/"+invoice.ID+"/download", nil, admin, nil)
	if dl.Code != http.StatusOK || dl.Body.String() != invoicePDF || dl.Header().Get("Content-Type") != "application/pdf" ||
		!strings.Contains(dl.Header().Get("Content-Disposition"), "invoice-MZ-2026-001.pdf") {
		t.Fatalf("download: %d %v", dl.Code, dl.Header())
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'invoice.downloaded' AND entity_id = $1`, invoice.ID); n != 1 {
		t.Fatalf("download audit rows: %d", n)
	}

	// Paid, then void; a void invoice cannot be paid again.
	status := func(body map[string]string) *http.Response {
		return e.do(http.MethodPost, "/v1/invoices/"+invoice.ID+"/status", body, ops, nil).Result()
	}
	if r := status(map[string]string{"status": "paid"}); r.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("paid without a day: %d", r.StatusCode)
	}
	if r := status(map[string]string{"status": "paid", "paid_on": "2026-11-20"}); r.StatusCode != http.StatusOK {
		t.Fatalf("paid: %d", r.StatusCode)
	}
	if r := status(map[string]string{"status": "void", "reason": "Replaced by MZ-2026-002 (Sample)"}); r.StatusCode != http.StatusOK {
		t.Fatalf("void: %d", r.StatusCode)
	}
	if r := status(map[string]string{"status": "paid", "paid_on": "2026-11-21"}); r.StatusCode != http.StatusConflict {
		t.Fatalf("paid after void: %d", r.StatusCode)
	}
	if w := e.do(http.MethodPost, "/v1/invoices/"+invoice.ID+"/status", map[string]string{"status": "void", "reason": "x"}, admin, nil); w.Code != http.StatusForbidden {
		t.Fatalf("client changes an invoice: %d", w.Code)
	}

	// Team: the admin invites a colleague, and sees only their own people.
	if w := e.do(http.MethodPost, "/v1/invitations", map[string]any{"email": "new.colleague@sample.invalid", "role": "client_viewer", "org_id": sampleOrg}, admin, nil); w.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", w.Code, w.Body)
	}
	var team struct {
		Members []struct{ Name, Email, Role string }
	}
	e.get(admin, "/v1/team", &team)
	names := map[string]string{}
	for _, m := range team.Members {
		names[m.Name] = m.Role
	}
	if names["Sample Client Admin"] != "client_admin" || names["Sample Client Viewer"] != "client_viewer" || len(team.Members) != 2 {
		t.Fatalf("team: %+v", team)
	}
	if w := e.do(http.MethodGet, "/v1/team", nil, viewer, nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer reads team: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/team?org_id="+otherOrg, nil, admin, nil); w.Code != http.StatusNotFound {
		t.Fatalf("admin reads other team: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/team", nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ops without org_id: %d", w.Code)
	}
	e.get(ops, "/v1/team?org_id="+otherOrg, &team)
	if len(team.Members) != 1 || team.Members[0].Name != "Other Sample Client Admin" {
		t.Fatalf("ops reads other team: %+v", team)
	}

	// Activity: a gold import and contributors' task events never appear.
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO audit_log (actor_kind, action, entity_type, org_id, after)
			VALUES ('system', 'import.committed', 'import_job', $1, '{"kind":"gold","rows":5}')`, sampleOrg)
		return err
	})
	var act activityResp
	e.get(admin, "/v1/activity", &act)
	got := strings.Join(act.actions(), " ")
	for _, want := range []string{"invoice.uploaded", "invoice.paid", "invoice.voided", "invoice.downloaded", "invitation.created"} {
		if !strings.Contains(got, want) {
			t.Errorf("admin activity lacks %s: %s", want, got)
		}
	}
	for _, x := range act.Entries {
		switch {
		case x.Action == "import.committed", strings.HasPrefix(x.Action, "task."), strings.HasPrefix(x.Action, "review."):
			t.Errorf("admin activity shows %s", x.Action)
		case x.Action == "invitation.created" && (x.Actor.Kind != "member" || x.Actor.Name != "Sample Client Admin" || x.Subject != "new.colleague@sample.invalid"):
			t.Errorf("invitation entry: %+v", x)
		case x.Action == "invoice.uploaded" && (x.Actor.Kind != "mezai" || x.Actor.Name != "MezAI team" || x.Subject != "MZ-2026-001"):
			t.Errorf("staff entry: %+v", x)
		}
	}
	e.get(viewer, "/v1/activity", &act)
	if strings.Contains(strings.Join(act.actions(), " "), "invoice.") {
		t.Errorf("viewer sees invoice activity: %v", act.actions())
	}
	e.get(other, "/v1/activity", &act)
	for _, x := range act.Entries {
		if x.Subject == "MZ-2026-001" && x.Action != "invoice.uploaded" || x.Subject == "new.colleague@sample.invalid" {
			t.Errorf("other org sees sample activity: %+v", x)
		}
	}
	if w := e.do(http.MethodGet, "/v1/activity?org_id="+sampleOrg, nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other org asks for sample activity: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/v1/activity", nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ops without org_id: %d", w.Code)
	}
}

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/seed"
)

type inboxList struct {
	Enquiries []struct {
		ID, Name, Status, Organisation string
		AssignedTo                     *string `json:"assigned_to"`
		NoteCount                      int     `json:"note_count"`
	}
	Counts map[string]int
}

type inboxDetail struct {
	ID, Status, Message string
	AssignedTo          *string `json:"assigned_to"`
	OrgID               *string `json:"org_id"`
	OrgName             *string `json:"org_name"`
	Moves               []string
	Notes               []struct {
		Body       string
		AuthorName string `json:"author_name"`
	}
	Assignees []struct{ ID, Name string }
}

// TestEnquiryInbox walks brief 6.3 and 9: Ops sees a new enquiry, assigns it,
// talks, notes and wins it with an organisation link; illegal moves, stale
// reads and other people are refused, and every write is audited without
// the enquirer's details.
func TestEnquiryInbox(t *testing.T) {
	e := newEnv(t)
	if w := e.do(http.MethodPost, "/v1/enquiries", validEnquiry("inbox.buyer@sample.invalid"), nil, nil); w.Code != http.StatusAccepted {
		t.Fatalf("enquiry: %d %s", w.Code, w.Body)
	}
	ops := e.signIn("ops@sample.invalid")

	var list inboxList
	w := e.do(http.MethodGet, "/v1/enquiries", nil, ops, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	decode(t, w, &list)
	if len(list.Enquiries) == 0 || list.Enquiries[0].Name != "Sample Buyer" || list.Enquiries[0].Status != "new" {
		t.Fatalf("newest first and new: %+v", list.Enquiries)
	}
	if list.Counts["new"] < 1 || list.Counts["won"] != 0 {
		t.Fatalf("counts: %v", list.Counts)
	}
	id := list.Enquiries[0].ID
	path := "/v1/enquiries/" + id

	var d inboxDetail
	w = e.do(http.MethodGet, path, nil, ops, nil)
	decode(t, w, &d)
	if !strings.Contains(d.Message, "Pilot first.") || strings.Join(d.Moves, ",") != "assigned,lost,spam" || len(d.Assignees) < 2 {
		t.Fatalf("detail: %+v", d)
	}
	opsID := e.userID("ops@sample.invalid")
	reviewerID := e.userID("reviewer@sample.invalid")

	patch := func(body map[string]any) *inboxDetail {
		t.Helper()
		w := e.do(http.MethodPatch, path, body, ops, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("patch %v: %d %s", body, w.Code, w.Body)
		}
		var out inboxDetail
		decode(t, w, &out)
		return &out
	}
	refused := func(body map[string]any, code int, want string) {
		t.Helper()
		w := e.do(http.MethodPatch, path, body, ops, nil)
		got := errorCode(w)
		if w.Code == http.StatusUnprocessableEntity {
			got = jsonField(t, w, "field")
		}
		if w.Code != code || got != want {
			t.Fatalf("%v: want %d %s, got %d %s", body, code, want, w.Code, w.Body)
		}
	}

	// Choosing someone for a new enquiry assigns it.
	d = *patch(map[string]any{"expected_status": "new", "status": "new", "assigned_to": opsID})
	if d.Status != "assigned" || d.AssignedTo == nil || *d.AssignedTo != opsID {
		t.Fatalf("assigned: %+v", d)
	}
	// Stale read: someone else moved it first.
	refused(map[string]any{"expected_status": "new", "status": "spam"}, http.StatusConflict, "enquiry_changed")
	// Only people who work enquiries can be assigned (the reviewer cannot).
	refused(map[string]any{"expected_status": "assigned", "status": "assigned", "assigned_to": reviewerID}, http.StatusUnprocessableEntity, "assigned_to")
	// Not straight to won.
	refused(map[string]any{"expected_status": "assigned", "status": "won", "assigned_to": opsID}, http.StatusUnprocessableEntity, "status")
	// In conversation needs someone.
	refused(map[string]any{"expected_status": "assigned", "status": "in_conversation"}, http.StatusUnprocessableEntity, "assigned_to")
	d = *patch(map[string]any{"expected_status": "assigned", "status": "in_conversation", "assigned_to": opsID})

	// Notes.
	if w := e.do(http.MethodPost, path+"/notes", map[string]string{"body": "  "}, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty note: %d", w.Code)
	}
	if w := e.do(http.MethodPost, path+"/notes", map[string]string{"body": "Call on Tuesday about the rubric."}, ops, nil); w.Code != http.StatusCreated {
		t.Fatalf("note: %d %s", w.Code, w.Body)
	}

	// Only a won enquiry links an organisation, and it must exist.
	org := e.orgID(seed.SampleOrgSlug)
	refused(map[string]any{"expected_status": "in_conversation", "status": "in_conversation", "assigned_to": opsID, "org_id": org}, http.StatusUnprocessableEntity, "org_id")
	refused(map[string]any{"expected_status": "in_conversation", "status": "won", "assigned_to": opsID, "org_id": reviewerID}, http.StatusUnprocessableEntity, "org_id")
	d = *patch(map[string]any{"expected_status": "in_conversation", "status": "won", "assigned_to": opsID, "org_id": org})
	if d.Status != "won" || d.OrgID == nil || *d.OrgID != org || d.OrgName == nil || len(d.Moves) != 0 {
		t.Fatalf("won: %+v", d)
	}
	if len(d.Notes) != 1 || d.Notes[0].AuthorName != "Sample Ops Manager" {
		t.Fatalf("notes: %+v", d.Notes)
	}
	// Won is final.
	refused(map[string]any{"expected_status": "won", "status": "lost", "assigned_to": opsID}, http.StatusUnprocessableEntity, "status")

	// Filters.
	decode(t, e.do(http.MethodGet, "/v1/enquiries?status=won", nil, ops, nil), &list)
	if len(list.Enquiries) != 1 || list.Enquiries[0].ID != id || list.Enquiries[0].NoteCount != 1 {
		t.Fatalf("won filter: %+v", list.Enquiries)
	}
	decode(t, e.do(http.MethodGet, "/v1/enquiries", nil, ops, nil), &list)
	for _, q := range list.Enquiries {
		if q.ID == id {
			t.Fatal("a won enquiry is not open")
		}
	}
	if w := e.do(http.MethodGet, "/v1/enquiries?status=maybe", nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad filter: %d", w.Code)
	}

	// Audit: three updates and a note, none carrying the enquirer's details or the note.
	if n := e.count(`SELECT count(*) FROM audit_log WHERE entity_id::text = $1 AND action IN ('enquiry.update', 'enquiry.note')
		AND actor_user_id::text = $2`, id, opsID); n != 4 {
		t.Fatalf("audit entries: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE entity_id::text = $1
		AND (coalesce(after::text, '') || coalesce(before::text, '')) ~ '(inbox\.buyer|Tuesday|Sample Buyer)'`, id); n != 0 {
		t.Fatalf("personal details or note text in the audit log: %d", n)
	}

	// Nobody else reads or writes the inbox.
	for _, who := range []string{"reviewer@sample.invalid", "approver@sample.invalid", "client.admin@sample.invalid", "contributor001@sample.invalid"} {
		s := e.signIn(who)
		for _, r := range []struct {
			method, path string
			body         any
		}{
			{http.MethodGet, "/v1/enquiries", nil},
			{http.MethodGet, path, nil},
			{http.MethodPatch, path, map[string]any{"expected_status": "won", "status": "won"}},
			{http.MethodPost, path + "/notes", map[string]string{"body": "x"}},
		} {
			if w := e.do(r.method, r.path, r.body, s, nil); w.Code != http.StatusForbidden {
				t.Errorf("%s %s %s: %d", who, r.method, r.path, w.Code)
			}
		}
	}
	if w := e.do(http.MethodGet, "/v1/enquiries/00000000-0000-7000-8000-000000000000", nil, ops, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", w.Code)
	}
}

// TestEnquiryUnassignAndReopen covers the ways back: unassigning a new
// enquiry, spam back to new, lost back to in conversation.
func TestEnquiryUnassignAndReopen(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("superadmin@sample.invalid")
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM enquiries WHERE status = 'new' ORDER BY created_at LIMIT 1`).Scan(&id)
	})
	path := "/v1/enquiries/" + id
	me := e.userID("superadmin@sample.invalid")
	for _, step := range []struct {
		from, to string
		who      any
		want     string
	}{
		{"new", "spam", nil, "spam"},
		{"spam", "new", nil, "new"},
		{"new", "assigned", me, "assigned"},
		{"assigned", "new", nil, "new"},
		{"new", "lost", nil, "lost"},
		{"lost", "in_conversation", me, "in_conversation"},
	} {
		w := e.do(http.MethodPatch, path, map[string]any{"expected_status": step.from, "status": step.to, "assigned_to": step.who}, ops, nil)
		if w.Code != http.StatusOK || jsonField(t, w, "status") != step.want {
			t.Fatalf("%s -> %s: %d %s", step.from, step.to, w.Code, w.Body)
		}
	}
}

// jsonField reads one top-level string field of a JSON answer.
func jsonField(t *testing.T, w *httptest.ResponseRecorder, field string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	v, _ := m[field].(string)
	return v
}

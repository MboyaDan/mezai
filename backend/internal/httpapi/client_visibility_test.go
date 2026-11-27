package httpapi_test

import (
	"net/http"
	"testing"
)

type visibilityResp struct {
	ClientVisibility struct {
		Quality   bool `json:"quality"`
		Workforce bool `json:"workforce"`
	} `json:"client_visibility"`
}

// TestClientVisibility: Ops holds the quality and workforce panels back from
// the client per project (brief 6.3, D-167). The API refuses the client, staff
// still read both, progress stays shared, and each change is audited once.
func TestClientVisibility(t *testing.T) {
	e := newEnv(t)
	ops := e.signIn("superadmin@sample.invalid")
	admin := e.signIn("client.admin@sample.invalid")
	viewer := e.signIn("client.viewer@sample.invalid")
	other := e.signIn("other.admin@sample.invalid")
	project := e.reviewProject(2)
	path := "/v1/projects/" + project

	// New projects share everything, and the client is told so.
	var pr visibilityResp
	e.get(admin, path, &pr)
	if !pr.ClientVisibility.Quality || !pr.ClientVisibility.Workforce {
		t.Fatalf("default visibility: %+v", pr.ClientVisibility)
	}

	set := func(s *session, body any) int {
		return e.do(http.MethodPut, path+"/client-visibility", body, s, nil).Code
	}
	// Only staff with project.manage, and the whole setting every time.
	if c := set(admin, map[string]bool{"quality": false, "workforce": false}); c != http.StatusForbidden {
		t.Fatalf("client admin set visibility: %d", c)
	}
	if c := set(ops, map[string]bool{"quality": false}); c != http.StatusUnprocessableEntity {
		t.Fatalf("partial setting: %d", c)
	}
	w := e.do(http.MethodPut, path+"/client-visibility", map[string]bool{"quality": false, "workforce": true}, ops, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("hide quality: %d %s", w.Code, w.Body)
	}
	decode(t, w, &pr)
	if pr.ClientVisibility.Quality || !pr.ClientVisibility.Workforce {
		t.Fatalf("after hiding quality: %+v", pr.ClientVisibility)
	}

	code := func(s *session, sub string) int { return e.do(http.MethodGet, path+sub, nil, s, nil).Code }
	for _, s := range []*session{admin, viewer} {
		if c := code(s, "/quality"); c != http.StatusForbidden {
			t.Errorf("client quality while hidden: %d", c)
		}
		if c := code(s, "/workforce"); c != http.StatusOK {
			t.Errorf("client workforce while shown: %d", c)
		}
		if c := code(s, "/progress"); c != http.StatusOK {
			t.Errorf("client progress: %d", c)
		}
	}
	if c := code(ops, "/quality"); c != http.StatusOK {
		t.Errorf("staff quality while hidden from the client: %d", c)
	}
	// Another organisation still gets not found, not a hint that it exists.
	if c := code(other, "/quality"); c != http.StatusNotFound {
		t.Errorf("other org quality: %d", c)
	}

	if c := set(ops, map[string]bool{"quality": true, "workforce": false}); c != http.StatusOK {
		t.Fatalf("swap: %d", c)
	}
	if c := code(admin, "/workforce"); c != http.StatusForbidden {
		t.Errorf("client workforce while hidden: %d", c)
	}
	if c := code(admin, "/quality"); c != http.StatusOK {
		t.Errorf("client quality after sharing again: %d", c)
	}
	if c := code(ops, "/workforce"); c != http.StatusOK {
		t.Errorf("staff workforce: %d", c)
	}

	// Saving the same setting again writes no audit entry.
	if c := set(ops, map[string]bool{"quality": true, "workforce": false}); c != http.StatusOK {
		t.Fatalf("same again: %d", c)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE entity_id::text = $1 AND action = 'project.client_visibility'`, project); n != 2 {
		t.Errorf("audit entries: %d, want 2", n)
	}
}

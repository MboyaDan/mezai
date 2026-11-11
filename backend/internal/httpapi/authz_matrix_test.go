package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/httpapi"
)

// The role table from brief section 4, written out independently of the
// migrations, so a wrong seed or migration fails here. AC-01.
var rolePermissions = map[string][]string{
	"super_admin": {"*"},
	"ops_manager": {"audit.read", "user.manage", "org.manage", "project.manage", "item.import", "gold.manage",
		"contributor.approve", "contributor.pause", "review.read", "review.decide", "review.adjudicate",
		"feedback.write", "deliverable.publish", "payout.create", "enquiry.manage"},
	"payout_approver": {"payout.approve"},
	"reviewer":        {"review.read", "review.decide", "feedback.write"},
	"contributor":     {"task.submit", "qualification.take", "profile.manage", "earnings.read_own"},
	"client_admin":    {"org.project.read", "org.project.request", "org.deliverable.download", "org.user.invite"},
	"client_viewer":   {"org.project.read", "org.deliverable.download"},
}

// The access each /v1 route must declare. Adding a route means adding it
// here; changing who may call a route means changing it here, in review.
var expectedAccess = map[string]string{
	"POST /v1/auth/login":                  "public",
	"POST /v1/auth/logout":                 "signed-in",
	"GET /v1/auth/me":                      "signed-in",
	"POST /v1/auth/password-reset/request": "public",
	"POST /v1/auth/password-reset/confirm": "public",
	"POST /v1/auth/verify-email":           "public",
	"POST /v1/auth/verify-email/resend":    "public",
	"POST /v1/invitations/accept":          "public",
	"GET /v1/invitations":                  "user.manage|org.user.invite",
	"POST /v1/invitations":                 "user.manage|org.user.invite",
	"DELETE /v1/invitations/:id":           "user.manage|org.user.invite",
	"GET /v1/users":                        "user.manage",
	"POST /v1/users/:id/roles":             "user.manage",
	"DELETE /v1/users/:id/roles/:role":     "user.manage",
	"POST /v1/users/:id/status":            "user.manage",
	"GET /v1/roles":                        "user.manage|role.manage",
	"GET /v1/permissions":                  "role.manage",
	"PUT /v1/roles/:key/permissions":       "role.manage",
	"GET /v1/stats":                        "public",
	"POST /v1/enquiries":                   "public",

	"POST /v1/contributors/signup":                        "public",
	"GET /v1/me/contributor":                              "profile.manage",
	"PUT /v1/me/contributor/profile":                      "profile.manage",
	"POST /v1/me/contributor/consents":                    "profile.manage",
	"GET /v1/me/qualification/tests":                      "qualification.take",
	"POST /v1/me/qualification/tests/:id/attempts":        "qualification.take",
	"GET /v1/me/qualification/attempts/:id":               "qualification.take",
	"PUT /v1/me/qualification/attempts/:id/answers":       "qualification.take",
	"POST /v1/me/qualification/attempts/:id/submit":       "qualification.take",
	"GET /v1/contributors":                                "contributor.approve|contributor.pause",
	"GET /v1/contributors/:id":                            "contributor.approve|contributor.pause",
	"POST /v1/contributors/:id/status":                    "contributor.pause",
	"POST /v1/contributors/:id/identity":                  "contributor.approve",
	"PUT /v1/me/contributor/cv":                           "profile.manage",
	"POST /v1/qualification/core-check":                   "gold.manage",
	"DELETE /v1/me/contributor/cv":                        "profile.manage",
	"GET /v1/contributors/:id/cv":                         "contributor.approve",
	"POST /v1/contributors/:id/screening":                 "contributor.approve",
	"POST /v1/contributors/:id/interviews":                "contributor.approve",
	"GET /v1/me/projects":                                 "task.submit",
	"GET /v1/me/projects/:id":                             "task.submit",
	"POST /v1/me/projects/:id/next":                       "task.submit",
	"GET /v1/me/tasks/:id":                                "task.submit",
	"PUT /v1/me/tasks/:id/draft":                          "task.submit",
	"POST /v1/me/tasks/:id/submit":                        "task.submit",
	"POST /v1/me/tasks/:id/skip":                          "task.submit",
	"POST /v1/me/tasks/:id/report":                        "task.submit",
	"GET /v1/content-reports":                             "project.manage",
	"GET /v1/me/feedback":                                 "task.submit",
	"POST /v1/me/feedback/:id/read":                       "task.submit",
	"GET /v1/me/earnings":                                 "earnings.read_own",
	"GET /v1/me/earnings/statement":                       "earnings.read_own",
	"GET /v1/me/payouts":                                  "earnings.read_own",
	"GET /v1/me/quality":                                  "earnings.read_own",
	"GET /v1/me/training":                                 "task.submit",
	"GET /v1/projects/:id/quality":                        "project.manage|org.project.read",
	"POST /v1/projects/:id/quality/snapshot":              "project.manage",
	"POST /v1/content-reports/:id/resolve":                "project.manage",
	"GET /v1/review/items":                                "review.read",
	"GET /v1/review/items/:id":                            "review.read",
	"POST /v1/review/items/:id/decision":                  "review.decide",
	"GET /v1/projects/:id/reviewers":                      "project.manage",
	"POST /v1/projects/:id/reviewers":                     "project.manage",
	"DELETE /v1/projects/:id/reviewers/:userId":           "project.manage",
	"GET /v1/projects/:id/deliverables":                   "deliverable.publish|org.deliverable.download",
	"POST /v1/projects/:id/deliverables":                  "deliverable.publish",
	"POST /v1/projects/:id/deliverables/:version/publish": "deliverable.publish",
	"POST /v1/deliverables/:id/links":                     "deliverable.publish|org.deliverable.download",
	"GET /v1/deliverables/:id/download":                   "deliverable.publish|org.deliverable.download",
	"POST /v1/contributors/:id/mpesa-name-check":          "payout.create",
	"GET /v1/qualification/attempts":                      "contributor.approve",
	"GET /v1/qualification/attempts/:id":                  "contributor.approve",
	"POST /v1/qualification/attempts/:id/decision":        "contributor.approve",
	"GET /v1/qualification/tests":                         "contributor.approve|gold.manage",
	"POST /v1/qualification/tests":                        "gold.manage",
	"POST /v1/qualification/tests/:id/status":             "gold.manage",

	"GET /v1/organisations":                 "org.manage|project.manage",
	"POST /v1/organisations":                "org.manage",
	"GET /v1/task-templates":                "project.manage|item.import|gold.manage",
	"GET /v1/projects":                      "project.manage|item.import|gold.manage|org.project.read",
	"POST /v1/projects":                     "project.manage",
	"GET /v1/projects/:id":                  "project.manage|item.import|gold.manage|org.project.read",
	"PATCH /v1/projects/:id":                "project.manage",
	"POST /v1/projects/:id/status":          "project.manage",
	"GET /v1/projects/:id/imports":          "item.import|gold.manage|org.project.read",
	"POST /v1/projects/:id/imports":         "item.import|gold.manage",
	"GET /v1/imports/:id":                   "item.import|gold.manage|org.project.read",
	"POST /v1/imports/:id/commit":           "item.import|gold.manage",
	"GET /v1/projects/:id/gold":             "gold.manage",
	"POST /v1/projects/:id/gold":            "gold.manage",
	"POST /v1/gold/:id/retire":              "gold.manage",
	"GET /v1/project-requests":              "project.manage|org.project.request|org.project.read",
	"POST /v1/project-requests":             "org.project.request",
	"GET /v1/project-requests/:id":          "project.manage|org.project.request|org.project.read",
	"GET /v1/project-requests/:id/sample":   "project.manage|org.project.request|org.project.read",
	"POST /v1/project-requests/:id/convert": "project.manage",
	"POST /v1/project-requests/:id/decline": "project.manage",
}

var roleUser = map[string]string{
	"super_admin":     "superadmin@sample.invalid",
	"ops_manager":     "ops@sample.invalid",
	"payout_approver": "approver@sample.invalid",
	"reviewer":        "reviewer@sample.invalid",
	"contributor":     "contributor001@sample.invalid",
	"client_admin":    "client.admin@sample.invalid",
	"client_viewer":   "client.viewer@sample.invalid",
}

func holds(role string, any []string) bool {
	for _, have := range rolePermissions[role] {
		if have == "*" {
			return true
		}
		for _, want := range any {
			if have == want {
				return true
			}
		}
	}
	return false
}

// TestAuthorisationMatrix calls every protected /v1 route as every role and
// checks allow or deny against the table above. Requests carry a malformed
// body or unknown ids, so an allowed call stops in the handler (400 or 404)
// and changes nothing, while a denied call stops at the access rule (403).
func TestAuthorisationMatrix(t *testing.T) {
	e := newEnv(t)
	sessions := map[string]*session{}
	for role, email := range roleUser {
		w, s := e.login(email, pw)
		if s == nil {
			t.Fatalf("%s login: %d %s", role, w.Code, w.Body)
		}
		sessions[role] = s
	}
	roles := make([]string, 0, len(roleUser))
	for r := range roleUser {
		roles = append(roles, r)
	}
	sort.Strings(roles)

	seen := map[string]bool{}
	for _, rule := range e.rules {
		key := rule.Method + " " + rule.Path
		seen[key] = true
		if want, ok := expectedAccess[key]; !ok {
			t.Errorf("%s is not in expectedAccess; add it with its permission", key)
		} else if got := rule.Access.String(); got != want {
			t.Errorf("%s declares %q, expected %q", key, got, want)
		}
	}
	for key := range expectedAccess {
		if !seen[key] {
			t.Errorf("%s is expected but not registered", key)
		}
	}

	unknown := "00000000-0000-7000-8000-000000000000"
	checked := 0
	for _, rule := range e.rules {
		expected := expectedAccess[rule.Method+" "+rule.Path]
		if expected == "public" || expected == "" {
			continue
		}
		path := strings.NewReplacer(":id", unknown, ":role", "no_such_role", ":key", "no_such_role").Replace(rule.Path)
		for _, role := range roles {
			want := expected == "signed-in" || holds(role, strings.Split(expected, "|"))
			if rule.Path == "/v1/auth/logout" {
				continue // would end the session the other checks use; covered in TestCSRF
			}
			w := e.raw(rule.Method, path, "{", sessions[role])
			denied := w.Code == http.StatusForbidden && errorCode(w) == "forbidden"
			if denied == want {
				t.Errorf("%s %s as %s: allowed=%v, got %d %s", rule.Method, rule.Path, role, want, w.Code, w.Body)
			}
			if w.Code == http.StatusInternalServerError {
				t.Errorf("%s %s as %s: 500", rule.Method, rule.Path, role)
			}
			checked++
		}
		// Anonymous callers are refused everywhere that is not public.
		if w := e.raw(rule.Method, path, "{", nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous: %d", rule.Method, rule.Path, w.Code)
		}
	}
	if checked < 7*10 {
		t.Fatalf("only %d role × route checks ran", checked)
	}
}

// raw sends a literal body (for malformed JSON).
func (e *env) raw(method, path, body string, s *session) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	if s != nil {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: s.cookie})
		req.Header.Set(httpapi.HeaderCSRF, s.csrf)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// lastLink returns the token from the newest queued email to `to`.
func (e *env) lastLink(to string) string {
	e.t.Helper()
	var data []byte
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT data FROM email_outbox WHERE to_address = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, to).Scan(&data)
	})
	var d map[string]string
	_ = json.Unmarshal(data, &d)
	u, err := url.Parse(d["Link"])
	if err != nil || !strings.HasPrefix(d["Link"], origin+"/") {
		e.t.Fatalf("link %q", d["Link"])
	}
	return u.Query().Get("token")
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	e.staff(func(tx pgx.Tx) error { return tx.QueryRow(context.Background(), sql, args...).Scan(&n) })
	return n
}

func (e *env) userID(email string) string {
	e.t.Helper()
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM users WHERE email = $1`, email).Scan(&id)
	})
	return id
}

func (e *env) orgID(slug string) string {
	e.t.Helper()
	var id string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM organisations WHERE slug = $1`, slug).Scan(&id)
	})
	return id
}

func TestPasswordReset(t *testing.T) {
	e := newEnv(t)
	_, old := e.login("ops@sample.invalid", pw)
	for i := 0; i < 5; i++ {
		e.login("ops@sample.invalid", "wrong password value")
	}

	a := e.do(http.MethodPost, "/v1/auth/password-reset/request", map[string]string{"email": "ops@sample.invalid"}, nil, nil)
	b := e.do(http.MethodPost, "/v1/auth/password-reset/request", map[string]string{"email": "nobody@sample.invalid"}, nil, nil)
	if a.Code != http.StatusAccepted || a.Body.String() != b.Body.String() || b.Code != a.Code {
		t.Fatalf("known and unknown emails must look the same: %d %s / %d %s", a.Code, a.Body, b.Code, b.Body)
	}
	token := e.lastLink("ops@sample.invalid")

	w := e.do(http.MethodPost, "/v1/auth/password-reset/confirm", map[string]string{"token": token, "password": "short"}, nil, nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password: %d %s", w.Code, w.Body)
	}
	const newPW = "a brand new passphrase"
	if w := e.do(http.MethodPost, "/v1/auth/password-reset/confirm", map[string]string{"token": token, "password": newPW}, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("confirm after a rejected attempt (token must survive): %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/auth/password-reset/confirm", map[string]string{"token": token, "password": newPW}, nil, nil); errorCode(w) != "invalid_token" {
		t.Fatalf("token reused: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, old, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("old session survived the reset: %d", w.Code)
	}
	// The reset also lifted the lockout.
	if w, s := e.login("ops@sample.invalid", newPW); s == nil {
		t.Fatalf("login with new password: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action IN ('auth.password_reset_requested', 'auth.password_reset')`); n != 2 {
		t.Errorf("reset audit entries: %d", n)
	}
}

func TestEmailVerification(t *testing.T) {
	e := newEnv(t)
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE users SET email_verified_at = NULL WHERE email = 'reviewer@sample.invalid'`)
		return err
	})
	if w := e.do(http.MethodPost, "/v1/auth/verify-email/resend", map[string]string{"email": "reviewer@sample.invalid"}, nil, nil); w.Code != http.StatusAccepted {
		t.Fatalf("resend: %d", w.Code)
	}
	token := e.lastLink("reviewer@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": "made-up"}, nil, nil); errorCode(w) != "invalid_token" {
		t.Fatalf("made-up token: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": token}, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if w, s := e.login("reviewer@sample.invalid", pw); s == nil {
		t.Fatalf("login after verifying: %d %s", w.Code, w.Body)
	}
}

func TestInvitations(t *testing.T) {
	e := newEnv(t)
	_, ops := e.login("ops@sample.invalid", pw)
	_, clientAdmin := e.login("client.admin@sample.invalid", pw)
	sample, other := e.orgID("sample-client-ltd"), e.orgID("other-sample-org")

	cases := []struct {
		name   string
		s      *session
		body   map[string]any
		status int
		code   string
	}{
		{"ops invites a reviewer", ops, map[string]any{"email": "New.Reviewer@sample.invalid", "role": "reviewer"}, 201, ""},
		{"client admin invites a viewer to own org", clientAdmin, map[string]any{"email": "viewer2@sample.invalid", "role": "client_viewer", "org_id": sample}, 201, ""},
		{"client admin, other org", clientAdmin, map[string]any{"email": "x1@sample.invalid", "role": "client_viewer", "org_id": other}, 403, "forbidden"},
		{"client admin, staff role", clientAdmin, map[string]any{"email": "x2@sample.invalid", "role": "ops_manager"}, 403, "forbidden"},
		{"super admin is never invitable", ops, map[string]any{"email": "x3@sample.invalid", "role": "super_admin"}, 403, "not_grantable"},
		{"contributors sign up themselves", ops, map[string]any{"email": "x4@sample.invalid", "role": "contributor"}, 403, "not_grantable"},
		{"existing account", ops, map[string]any{"email": "reviewer@sample.invalid", "role": "reviewer"}, 409, "email_in_use"},
		{"org role without org", ops, map[string]any{"email": "x5@sample.invalid", "role": "client_admin"}, 400, "bad_request"},
		{"staff role with org", ops, map[string]any{"email": "x6@sample.invalid", "role": "reviewer", "org_id": sample}, 400, "bad_request"},
	}
	for _, c := range cases {
		w := e.do(http.MethodPost, "/v1/invitations", c.body, c.s, nil)
		if w.Code != c.status || (c.code != "" && errorCode(w) != c.code) {
			t.Errorf("%s: got %d %s", c.name, w.Code, w.Body)
		}
	}

	// The client admin sees only their organisation's invitations.
	w := e.do(http.MethodGet, "/v1/invitations", nil, clientAdmin, nil)
	if !strings.Contains(w.Body.String(), "viewer2@sample.invalid") || strings.Contains(w.Body.String(), "new.reviewer") {
		t.Errorf("client admin list: %s", w.Body)
	}

	token := e.lastLink("new.reviewer@sample.invalid")
	if w := e.do(http.MethodPost, "/v1/invitations/accept", map[string]string{"token": token, "display_name": "Sample New Reviewer", "password": "short"}, nil, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/invitations/accept", map[string]string{"token": token, "display_name": "Sample New Reviewer", "password": "reviewer passphrase"}, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("accept: %d %s", w.Code, w.Body)
	}
	w, s := e.login("new.reviewer@sample.invalid", "reviewer passphrase")
	if s == nil || !strings.Contains(w.Body.String(), `"review.read"`) {
		t.Fatalf("new reviewer login: %d %s", w.Code, w.Body)
	}

	// A revoked invitation cannot be accepted.
	viewerToken := e.lastLink("viewer2@sample.invalid")
	var invID string
	e.staff(func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id::text FROM invitations WHERE email = 'viewer2@sample.invalid'`).Scan(&invID)
	})
	if w := e.do(http.MethodDelete, "/v1/invitations/"+invID, nil, clientAdmin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/invitations/accept", map[string]string{"token": viewerToken, "display_name": "X", "password": "viewer passphrase"}, nil, nil); errorCode(w) != "invalid_token" {
		t.Fatalf("accept revoked: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action LIKE 'invitation.%'`); n != 4 {
		t.Errorf("invitation audit entries: %d (want 2 created, 1 accepted, 1 revoked)", n)
	}
}

func TestSuperAdminProtections(t *testing.T) {
	e := newEnv(t)
	_, admin := e.login("superadmin@sample.invalid", pw)
	_, ops := e.login("ops@sample.invalid", pw)
	adminID, opsID := e.userID("superadmin@sample.invalid"), e.userID("ops@sample.invalid")

	steps := []struct {
		name   string
		s      *session
		method string
		path   string
		body   any
		status int
		code   string
	}{
		{"last super admin cannot be demoted", admin, http.MethodDelete, "/v1/users/" + adminID + "/roles/super_admin", nil, 409, "last_super_admin"},
		{"nobody disables themselves", admin, http.MethodPost, "/v1/users/" + adminID + "/status", map[string]string{"status": "disabled"}, 409, "cannot_change_self"},
		{"ops cannot demote a super admin", ops, http.MethodDelete, "/v1/users/" + adminID + "/roles/super_admin", nil, 403, "forbidden"},
		{"ops cannot disable a super admin", ops, http.MethodPost, "/v1/users/" + adminID + "/status", map[string]string{"status": "disabled"}, 403, "forbidden"},
		{"super admin role is never granted", admin, http.MethodPost, "/v1/users/" + opsID + "/roles", map[string]string{"role": "super_admin"}, 403, "not_grantable"},
		{"system.admin is never added to a role", admin, http.MethodPut, "/v1/roles/ops_manager/permissions", map[string]any{"permissions": []string{"system.admin"}}, 403, "not_grantable"},
		{"super admin role is fixed", admin, http.MethodPut, "/v1/roles/super_admin/permissions", map[string]any{"permissions": []string{"audit.read"}}, 403, "not_grantable"},
		{"staff cannot also be a client", admin, http.MethodPost, "/v1/users/" + opsID + "/roles", map[string]string{"role": "client_viewer", "org_id": e.orgID("sample-client-ltd")}, 409, "mixed_user_kinds"},
		{"ops cannot configure roles", ops, http.MethodPut, "/v1/roles/reviewer/permissions", map[string]any{"permissions": []string{}}, 403, "forbidden"},
	}
	for _, st := range steps {
		w := e.do(st.method, st.path, st.body, st.s, nil)
		if w.Code != st.status || errorCode(w) != st.code {
			t.Errorf("%s: got %d %s", st.name, w.Code, w.Body)
		}
	}
	if n := e.count(`SELECT count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.key = 'super_admin'`); n != 1 {
		t.Fatalf("super admins: %d", n)
	}
}

func TestRoleAndStatusAdministration(t *testing.T) {
	e := newEnv(t)
	_, admin := e.login("superadmin@sample.invalid", pw)
	_, reviewer := e.login("reviewer@sample.invalid", pw)
	reviewerID := e.userID("reviewer@sample.invalid")

	// Role configuration takes effect on the reviewer's next request.
	if w := e.do(http.MethodPut, "/v1/roles/reviewer/permissions", map[string]any{"permissions": []string{"review.read", "feedback.write"}}, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("set permissions: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, reviewer, nil); strings.Contains(w.Body.String(), "review.decide") {
		t.Errorf("review.decide still held: %s", w.Body)
	}
	if w := e.do(http.MethodPut, "/v1/roles/reviewer/permissions", map[string]any{"permissions": []string{"no.such_key"}}, admin, nil); w.Code != http.StatusBadRequest {
		t.Errorf("unknown key: %d", w.Code)
	}

	// Disabling signs the user out everywhere; enabling lets them back in.
	if w := e.do(http.MethodPost, "/v1/users/"+reviewerID+"/status", map[string]string{"status": "disabled"}, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/auth/me", nil, reviewer, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("disabled user's session still works: %d", w.Code)
	}
	if w, _ := e.login("reviewer@sample.invalid", pw); w.Code != http.StatusUnauthorized {
		t.Errorf("disabled user signed in: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/users/"+reviewerID+"/status", map[string]string{"status": "active"}, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("enable: %d", w.Code)
	}

	// Grant and remove a staff role.
	if w := e.do(http.MethodPost, "/v1/users/"+reviewerID+"/roles", map[string]string{"role": "payout_approver"}, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("grant: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodDelete, "/v1/users/"+reviewerID+"/roles/payout_approver", nil, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodDelete, "/v1/users/"+reviewerID+"/roles/payout_approver", nil, admin, nil); w.Code != http.StatusNotFound {
		t.Errorf("remove twice: %d", w.Code)
	}

	w := e.do(http.MethodGet, "/v1/users", nil, admin, nil)
	if !strings.Contains(w.Body.String(), "reviewer@sample.invalid") || strings.Contains(w.Body.String(), "contributor001") {
		t.Errorf("users list should hold staff and clients, not contributors: %.300s", w.Body)
	}
	for _, action := range []string{"role.permissions_changed", "user.status_changed", "user.role_granted", "user.role_removed"} {
		if n := e.count(`SELECT count(*) FROM audit_log WHERE action = $1`, action); n == 0 {
			t.Errorf("no %s audit entry", action)
		}
	}
}

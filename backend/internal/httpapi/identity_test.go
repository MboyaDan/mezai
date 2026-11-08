package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type detailResp struct {
	Profile  profileResp `json:"profile"`
	Identity struct {
		Status             string  `json:"status"`
		Note               *string `json:"note"`
		NationalIDLast3    *string `json:"national_id_last3"`
		MpesaNameCheckedAt *string `json:"mpesa_name_checked_at"`
		Flags              []struct {
			Kind         string `json:"kind"`
			RaisedByThis bool   `json:"raised_by_this"`
			Other        struct {
				PublicCode string `json:"public_code"`
			} `json:"other"`
		} `json:"flags"`
		SameName []struct {
			PublicCode string `json:"public_code"`
		} `json:"same_name"`
	} `json:"identity"`
}

func profileWith(changes map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range completeProfile {
		out[k] = v
	}
	for k, v := range changes {
		out[k] = v
	}
	return out
}

func (e *env) detail(ops *session, id string) detailResp {
	e.t.Helper()
	w := e.do(http.MethodGet, "/v1/contributors/"+id, nil, ops, nil)
	if w.Code != http.StatusOK {
		e.t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
	var d detailResp
	decode(e.t, w, &d)
	return d
}

// TestOneAccountPerPerson covers D-078 (Stage 7c): one account per M-Pesa
// number and National ID number, the number never stored, clashes recorded
// for Ops, verification before activation, and the M-Pesa name check.
func TestOneAccountPerPerson(t *testing.T) {
	e := newEnv(t)
	a := e.signUpAndSignIn("first.applicant@sample.invalid")
	b := e.signUpAndSignIn("second.applicant@sample.invalid")
	_, ops := e.login("ops@sample.invalid", pw)

	// The National ID number is part of a complete profile.
	var p profileResp
	w := e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"national_id": ""}), a, nil)
	decode(t, w, &p)
	if p.Status != "applied" || !strings.Contains(strings.Join(p.Missing, ","), "national_id") {
		t.Fatalf("profile without ID: %s", w.Body)
	}
	for _, bad := range []string{"12345", "A1234567", "2468135790"} {
		if w := e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"national_id": bad}), a, nil); !strings.Contains(w.Body.String(), `"field":"national_id"`) {
			t.Fatalf("ID %q: %d %s", bad, w.Code, w.Body)
		}
	}
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", completeProfile, a, nil)
	decode(t, w, &p)
	if w.Code != http.StatusOK || p.Status != "qualifying" || p.NationalIDLast3 == nil || *p.NationalIDLast3 != "357" || p.IdentityStatus != "pending" {
		t.Fatalf("complete profile: %d %s", w.Code, w.Body)
	}
	aID := p.ID

	// Only a 32-byte fingerprint and the last 3 digits are stored; the
	// number appears nowhere: not in responses, the profile row or the audit log.
	if strings.Contains(w.Body.String(), "24681357") {
		t.Fatal("ID number in the response")
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles WHERE id = $1 AND octet_length(national_id_fp) = 32 AND national_id_last3 = '357'`, aID); n != 1 {
		t.Fatal("fingerprint not stored")
	}
	if n := e.count(`SELECT count(*) FROM contributor_profiles cp WHERE cp::text LIKE '%24681357%'`); n != 0 {
		t.Fatal("ID number stored in contributor_profiles")
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE coalesce(after::text, '') LIKE '%24681357%' OR coalesce(before::text, '') LIKE '%24681357%'`); n != 0 {
		t.Fatal("ID number in the audit log")
	}

	// B cannot use A's M-Pesa number, nor A's ID number typed differently.
	// Each try is refused, saves nothing and is recorded for Ops.
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"national_id": "87654321"}), b, nil)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"field":"mpesa_phone"`) || !strings.Contains(w.Body.String(), "another MezAI account") {
		t.Fatalf("reused phone: %d %s", w.Code, w.Body)
	}
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{
		"national_id": "02468-1357", "mpesa_phone": "0712 345 679", "mpesa_phone_confirm": "0712345679",
	}), b, nil)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"field":"national_id"`) {
		t.Fatalf("reused ID: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodGet, "/v1/me/contributor", nil, b, nil), &p)
	bID := p.ID
	if p.Status != "applied" || p.MpesaPhoneMasked != "" || p.NationalIDLast3 != nil {
		t.Fatalf("a refused save changed B's profile: %+v", p)
	}
	if n := e.count(`SELECT count(*) FROM identity_flags WHERE contributor_id = $1 AND other_contributor_id = $2`, bID, aID); n != 2 {
		t.Fatalf("flags recorded: %d", n)
	}

	// Ops sees the clashes on both accounts and the shared name, by code only.
	d := e.detail(ops, aID)
	if len(d.Identity.Flags) != 2 || d.Identity.Flags[0].RaisedByThis || len(d.Identity.SameName) != 1 {
		t.Fatalf("A's identity checks: %+v", d.Identity)
	}
	w = e.do(http.MethodGet, "/v1/contributors?identity=pending", nil, ops, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), aID) || !strings.Contains(w.Body.String(), `"possible_duplicates":3`) {
		t.Fatalf("pending list: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodGet, "/v1/contributors?identity=maybe", nil, ops, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad identity filter: %d", w.Code)
	}

	// Verifying needs both numbers, and a note while there are possible duplicates.
	verify := func(id, decision, note string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/v1/contributors/"+id+"/identity", map[string]string{"decision": decision, "note": note}, ops, nil)
	}
	if w := verify(bID, "verify", "x"); errorCode(w) != "identity_incomplete" {
		t.Fatalf("verify without numbers: %d %s", w.Code, w.Body)
	}
	if w := verify(aID, "verify", ""); !strings.Contains(w.Body.String(), `"field":"note"`) {
		t.Fatalf("verify with duplicates and no note: %d %s", w.Code, w.Body)
	}
	if w := verify(aID, "reject", ""); !strings.Contains(w.Body.String(), `"field":"note"`) {
		t.Fatalf("reject without note: %d %s", w.Code, w.Body)
	}
	_, c1 := e.login("contributor001@sample.invalid", pw)
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/identity", map[string]string{"decision": "verify", "note": "x"}, c1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("contributor verifies: %d", w.Code)
	}
	note := "Called the applicant; the M-Pesa number belongs to a relative. Asked for their own number."
	if w := verify(aID, "reject", note); w.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	decode(t, e.do(http.MethodGet, "/v1/me/contributor", nil, a, nil), &p)
	if p.IdentityStatus != "rejected" || p.IdentityNote == nil || *p.IdentityNote != note {
		t.Fatalf("contributor sees the rejection: %+v", p)
	}

	// A corrected number goes back to pending.
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{
		"mpesa_phone": "0722 000 111", "mpesa_phone_confirm": "0722000111", "national_id": "",
	}), a, nil)
	decode(t, w, &p)
	if w.Code != http.StatusOK || p.IdentityStatus != "pending" || *p.NationalIDLast3 != "357" {
		t.Fatalf("after correcting the number: %d %s", w.Code, w.Body)
	}
	if w := verify(aID, "verify", "Flags came from a second applicant using this person's old number."); w.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE entity_id = $1 AND action IN ('contributor.identity_verified', 'contributor.identity_rejected')`, aID); n != 2 {
		t.Fatalf("audited identity decisions: %d", n)
	}

	// M-Pesa name check: payout.create only; cleared when the number or name changes.
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/mpesa-name-check", map[string]any{}, c1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("contributor records the name check: %d", w.Code)
	}
	if w := e.do(http.MethodPost, "/v1/contributors/"+aID+"/mpesa-name-check", map[string]any{}, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("name check: %d %s", w.Code, w.Body)
	}
	if d := e.detail(ops, aID); d.Identity.MpesaNameCheckedAt == nil || d.Identity.Status != "verified" {
		t.Fatalf("after the name check: %+v", d.Identity)
	}
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{
		"display_name": "Sample Applicant Renamed", "mpesa_phone": "", "mpesa_phone_confirm": "", "national_id": "",
	}), a, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body)
	}
	if d := e.detail(ops, aID); d.Identity.MpesaNameCheckedAt != nil || d.Identity.Status != "verified" {
		t.Fatalf("a new name needs the name check again, identity stays verified: %+v", d.Identity)
	}

	// The released number is free again for B.
	w = e.do(http.MethodPut, "/v1/me/contributor/profile", profileWith(map[string]any{"national_id": "87654321"}), b, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("B uses the number A gave up: %d %s", w.Code, w.Body)
	}
}

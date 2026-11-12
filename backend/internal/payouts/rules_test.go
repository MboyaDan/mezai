package payouts

import (
	"strings"
	"testing"
	"time"
)

func TestDecide(t *testing.T) {
	phone := "+254700000001"
	ok := Candidate{BalanceCents: 123456, CutoffCents: 123456, Phone: &phone, PhoneConfirmed: true, IdentityVerified: true, NameChecked: true}
	cases := []struct {
		name   string
		edit   func(c *Candidate)
		amount int64
		hold   string
	}{
		{"whole shillings, cents stay owed", func(*Candidate) {}, 123400, ""},
		{"later earnings wait for the next batch", func(c *Candidate) { c.BalanceCents = 200000 }, 123400, ""},
		{"a later correction is respected", func(c *Candidate) { c.BalanceCents = 60050 }, 60000, ""},
		{"exactly the minimum", func(c *Candidate) { c.BalanceCents, c.CutoffCents = 50000, 50000 }, 50000, ""},
		{"below the minimum", func(c *Candidate) { c.CutoffCents = 50099 - 100 }, 0, HoldBelowMinimum},
		{"no number", func(c *Candidate) { c.Phone = nil }, 0, HoldNoPhone},
		{"number not confirmed", func(c *Candidate) { c.PhoneConfirmed = false }, 0, HoldPhoneUnconfirmed},
		{"identity not verified", func(c *Candidate) { c.IdentityVerified = false }, 0, HoldIdentityUnverified},
		{"name not checked", func(c *Candidate) { c.NameChecked = false }, 0, HoldNameUnchecked},
	}
	for _, tc := range cases {
		c := ok
		tc.edit(&c)
		amount, hold := Decide(c, 50000)
		if amount != tc.amount || hold != tc.hold {
			t.Errorf("%s: got %d %q, want %d %q", tc.name, amount, hold, tc.amount, tc.hold)
		}
	}
}

func TestBuildCSV(t *testing.T) {
	f, err := ParseFormat([]byte(`{"confirmed": false, "columns": [
		{"header": "Phone Number", "field": "phone_254"}, {"header": "Amount", "field": "amount_kes"},
		{"header": "Name", "field": "name"}, {"header": "Reference", "field": "reference"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	end := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	got, err := BuildCSV(f, []Payee{
		{PhoneE164: "+254700000001", AmountCents: 150000, Name: "Sample Contributor 001", PublicCode: "C-0001"},
		{PhoneE164: "+254700000002", AmountCents: 50000, Name: "=HYPERLINK(\"x\")", PublicCode: "C-0002"},
	}, end)
	if err != nil {
		t.Fatal(err)
	}
	want := "Phone Number,Amount,Name,Reference\r\n" +
		"254700000001,1500,Sample Contributor 001,MEZAI C-0001 2026-11-08\r\n" +
		"254700000002,500,\"'=HYPERLINK(\"\"x\"\")\",MEZAI C-0002 2026-11-08\r\n"
	if string(got) != want {
		t.Fatalf("csv:\n%q\nwant\n%q", got, want)
	}
	if _, err := BuildCSV(f, []Payee{{PhoneE164: "+254700000001", AmountCents: 150050}}, end); err == nil {
		t.Fatal("cents in a KES amount were accepted")
	}
	for _, bad := range []string{`{"columns": []}`, `{"columns": [{"header": "X", "field": "balance"}]}`, `nope`} {
		if _, err := ParseFormat([]byte(bad)); err == nil || !strings.Contains(err.Error(), "payout.export_format") {
			t.Errorf("format %s accepted: %v", bad, err)
		}
	}
}

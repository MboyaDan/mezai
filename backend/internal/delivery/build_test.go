package delivery

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

func TestGoldSummary(t *testing.T) {
	g := goldSummary([]store.ExportGoldByContributorRow{
		{PublicCode: "C-0001", Total: 10, Correct: 9},
		{PublicCode: "C-0002", Total: 4, Correct: 2},
		{PublicCode: "C-0003", Total: 5, Correct: 4},
		{PublicCode: "C-0004", Total: 1, Correct: 1},
	})
	if g.Results != 20 || g.Correct != 16 || *g.Accuracy != 0.8 {
		t.Fatalf("project accuracy: %+v", g)
	}
	d := g.Distribution
	// Per contributor 0.5, 0.8, 0.9, 1: the median of four is the mean of the middle two.
	if d.Contributors != 4 || *d.Min != 0.5 || *d.Median != 0.85 || *d.Max != 1 {
		t.Fatalf("distribution: %v %v %v", *d.Min, *d.Median, *d.Max)
	}
	if g.ByContributor[1].Contributor != "C-0002" || *g.ByContributor[1].Accuracy != 0.5 {
		t.Fatalf("by contributor: %+v", g.ByContributor[1])
	}
	empty := goldSummary(nil)
	if empty.Accuracy != nil || empty.Distribution.Median != nil || empty.ByContributor == nil {
		t.Fatalf("no gold: %+v", empty)
	}
}

func TestGuardAndNames(t *testing.T) {
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+1": "'+1", "-2": "'-2", "@x": "'@x", "\tx": "'\tx",
		"Habari": "Habari", "": "", "4.5": "4.5"} {
		if got := guard(in); got != want {
			t.Errorf("guard(%q) = %q, want %q", in, got, want)
		}
	}
	if n := FileName("Swahili Intent (Sample) — v2!", 3, "csv"); n != "swahili-intent-sample-v2-v3.csv" {
		t.Errorf("file name %q", n)
	}
	if n := FileName("???", 1, "manifest"); n != "project-v1-manifest.json" {
		t.Errorf("file name %q", n)
	}
}

func TestLinkSignature(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s := &Service{linkKey: deriveLinkKey([]byte("test secret, 32 bytes long......")), now: func() time.Time { return now }}
	id, user := uuid.New(), uuid.New()
	exp := now.Add(LinkTTL).Unix()
	sig := s.sign(id, "data", user, exp)
	if sig != s.sign(id, "data", user, exp) || len(sig) != 64 {
		t.Fatal("signature not stable")
	}
	for name, other := range map[string]string{
		"file":    s.sign(id, "manifest", user, exp),
		"user":    s.sign(id, "data", uuid.New(), exp),
		"expires": s.sign(id, "data", user, exp+1),
		"key":     (&Service{linkKey: deriveLinkKey([]byte("another secret"))}).sign(id, "data", user, exp),
	} {
		if other == sig {
			t.Errorf("changing the %s keeps the signature", name)
		}
	}
	// Expired links fail before any database work, so a nil pool is safe here.
	s.now = func() time.Time { return now.Add(LinkTTL + time.Second) }
	if _, err := s.Download(t.Context(), &auth.Principal{UserID: user}, id, "data", strconv.FormatInt(exp, 10), sig, auth.Meta{}); err != ErrLinkInvalid {
		t.Fatalf("expired link: %v", err)
	}
}

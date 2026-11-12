package portal

import (
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/jobs"
)

func TestPace(t *testing.T) {
	today := time.Date(2026, 11, 11, 15, 0, 0, 0, jobs.Nairobi) // a Wednesday
	i32 := func(v int32) *int32 { return &v }
	s := func(v string) *string { return &v }
	cases := []struct {
		name     string
		pr       Progress
		recent   int32
		state    string
		finish   string
		perDay   float64
		goal     int32
		remained int32
	}{
		{"all done", Progress{Total: 10, Final: 10}, 0, "done", "", 0, 10, 0},
		// 28 final in 14 days = 2 a day; 20 left → 10 days → 21 Nov.
		{"on track", Progress{Total: 100, Final: 80, Deadline: s("2026-11-30")}, 28, "on_track", "2026-11-21", 2, 100, 20},
		{"finish on the deadline is on track", Progress{Total: 100, Final: 80, Deadline: s("2026-11-21")}, 28, "on_track", "2026-11-21", 2, 100, 20},
		{"projected after the deadline", Progress{Total: 100, Final: 80, Deadline: s("2026-11-20")}, 28, "at_risk", "2026-11-21", 2, 100, 20},
		// The target, not the items imported, is the goal.
		{"target is the goal", Progress{Total: 50, Final: 30, Target: i32(40)}, 14, "no_deadline", "2026-11-21", 1, 40, 10},
		{"no pace, deadline in a week", Progress{Total: 5, Deadline: s("2026-11-18")}, 0, "at_risk", "", 0, 5, 5},
		{"no pace, deadline far", Progress{Total: 5, Deadline: s("2026-12-31")}, 0, "no_pace", "", 0, 5, 5},
		{"no items, no deadline", Progress{}, 0, "no_deadline", "", 0, 0, 0},
	}
	for _, c := range cases {
		got := pace(c.pr, c.recent, today)
		finish := ""
		if got.ProjectedFinish != nil {
			finish = *got.ProjectedFinish
		}
		if got.State != c.state || finish != c.finish || got.PerDay != c.perDay || got.Goal != c.goal || got.Remaining != c.remained {
			t.Errorf("%s: got %+v (finish %q)", c.name, got, finish)
		}
	}
}

func TestCodeIsPerOrganisation(t *testing.T) {
	s := NewService(nil, nil, []byte("test-secret-only"), nil)
	orgA, orgB, c1, c2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	format := regexp.MustCompile(`^C-[A-HJ-NP-TV-Z][0-9A-HJKMNP-TV-Z]{5}$`)
	if s.Code(orgA, c1) != s.Code(orgA, c1) {
		t.Fatal("code is not stable")
	}
	if s.Code(orgA, c1) == s.Code(orgB, c1) {
		t.Fatal("one contributor has the same code in two organisations")
	}
	if s.Code(orgA, c1) == s.Code(orgA, c2) {
		t.Fatal("two contributors share a code")
	}
	other := NewService(nil, nil, []byte("another-secret"), nil)
	if other.Code(orgA, c1) == s.Code(orgA, c1) {
		t.Fatal("code does not depend on the key")
	}
	for range 200 {
		if c := s.Code(uuid.New(), uuid.New()); !format.MatchString(c) {
			t.Fatalf("code %q has the wrong form", c)
		}
	}
}

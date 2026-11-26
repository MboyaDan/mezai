package enquiries

import "testing"

func TestCanMove(t *testing.T) {
	for _, c := range []struct {
		from, to string
		ok       bool
	}{
		{"new", "new", true},
		{"new", "assigned", true},
		{"new", "spam", true},
		{"new", "lost", true},
		{"new", "in_conversation", false},
		{"new", "won", false},
		{"assigned", "new", true},
		{"assigned", "in_conversation", true},
		{"assigned", "won", false},
		{"in_conversation", "won", true},
		{"in_conversation", "lost", true},
		{"in_conversation", "assigned", false},
		{"won", "won", true},
		{"won", "lost", false},
		{"won", "in_conversation", false},
		{"lost", "in_conversation", true},
		{"lost", "won", false},
		{"spam", "new", true},
		{"spam", "assigned", false},
		{"unknown", "unknown", false},
		{"new", "unknown", false},
	} {
		if got := CanMove(c.from, c.to); got != c.ok {
			t.Errorf("%s -> %s: got %v, want %v", c.from, c.to, got, c.ok)
		}
	}
	// Every status in brief 9's list has a row, and every move target is a status.
	for _, s := range Statuses {
		for _, to := range Moves(s) {
			if !known(to) {
				t.Errorf("%s moves to unknown %s", s, to)
			}
		}
	}
	if len(next) != len(Statuses) {
		t.Errorf("next has %d rows, Statuses %d", len(next), len(Statuses))
	}
}

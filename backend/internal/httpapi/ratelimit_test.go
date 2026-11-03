package httpapi

import (
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := newLimiter(3, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if ok, _ := l.allow("ip"); !ok {
			t.Fatalf("hit %d refused", i+1)
		}
	}
	ok, wait := l.allow("ip")
	if ok || wait != time.Minute {
		t.Fatalf("4th hit: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allow("other"); !ok {
		t.Fatal("keys must be independent")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.allow("ip"); !ok {
		t.Fatal("new window refused")
	}
}

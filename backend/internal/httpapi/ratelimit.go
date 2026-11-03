package httpapi

import (
	"sync"
	"time"
)

// limiter is a fixed-window counter per key, held in memory. Enough for the
// single API instance of the pilot; a second instance would need a shared
// store (A-019).
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*windowCount
	now    func() time.Time
}

type windowCount struct {
	start time.Time
	n     int
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, hits: map[string]*windowCount{}, now: time.Now}
}

// allow counts a hit for key and reports whether it is within the limit,
// and if not, how long until the window resets.
func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) > 10_000 {
		for k, w := range l.hits {
			if now.Sub(w.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.hits[key] = &windowCount{start: now, n: 1}
		return true, 0
	}
	w.n++
	if w.n > l.limit {
		return false, w.start.Add(l.window).Sub(now)
	}
	return true, 0
}

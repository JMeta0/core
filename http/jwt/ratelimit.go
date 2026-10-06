package jwt

import (
	"sync"
	"time"
)

// attemptLimiter is a simple per-key sliding window rate limiter. It is used to
// throttle login attempts per client IP.
type attemptLimiter struct {
	lock   sync.Mutex
	events map[string][]time.Time
	window time.Duration
	max    int
}

func newAttemptLimiter(max int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{
		events: make(map[string][]time.Time),
		window: window,
		max:    max,
	}
}

// allow records an attempt for key and reports whether it is allowed. At most
// max attempts are allowed within the configured window.
func (l *attemptLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.lock.Lock()
	defer l.lock.Unlock()

	events := l.events[key]

	// Drop expired events.
	kept := events[:0]
	for _, e := range events {
		if e.After(cutoff) {
			kept = append(kept, e)
		}
	}

	if len(kept) >= l.max {
		l.events[key] = kept

		return false
	}

	l.events[key] = append(kept, now)

	// Opportunistically keep the map from growing without bound.
	if len(l.events) > 10000 {
		for k, v := range l.events {
			if len(v) == 0 || v[len(v)-1].Before(cutoff) {
				delete(l.events, k)
			}
		}
	}

	return true
}

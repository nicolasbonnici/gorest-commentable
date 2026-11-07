package antispam

import (
	"sync"
	"time"
)

// windowLimiter is a fixed-window counter keyed by an arbitrary string. Each key
// may be used `limit` times per window; the window advances lazily on access.
// It is safe for concurrent use.
type windowLimiter struct {
	window time.Duration
	mu     sync.Mutex
	counts map[string]*windowCount
	// now is injectable for deterministic tests; defaults to time.Now.
	now func() time.Time
}

type windowCount struct {
	start time.Time
	count int
}

func newWindowLimiter(window time.Duration) *windowLimiter {
	return &windowLimiter{
		window: window,
		counts: make(map[string]*windowCount),
		now:    time.Now,
	}
}

// allow records one hit for key and reports whether it stays within limit for the
// current window.
func (l *windowLimiter) allow(key string, limit int) bool {
	if limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	wc, ok := l.counts[key]
	if !ok || now.Sub(wc.start) >= l.window {
		l.counts[key] = &windowCount{start: now, count: 1}
		l.pruneLocked(now)
		return true
	}

	if wc.count >= limit {
		return false
	}
	wc.count++
	return true
}

// pruneLocked drops expired windows so the map does not grow without bound. The
// caller must hold l.mu.
func (l *windowLimiter) pruneLocked(now time.Time) {
	for k, wc := range l.counts {
		if now.Sub(wc.start) >= l.window {
			delete(l.counts, k)
		}
	}
}

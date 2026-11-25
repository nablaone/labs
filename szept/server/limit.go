package server

import (
	"sync"
	"time"
)

// limiter is a fixed-window counter keyed by string (IP, token hash, ...).
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	m      map[string]*window
}

type window struct {
	start time.Time
	n     int
}

func newLimiter(max int, per time.Duration) *limiter {
	return &limiter{max: max, window: per, m: map[string]*window{}}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := time.Now()
	w := l.m[key]
	if w == nil || t.Sub(w.start) >= l.window {
		if len(l.m) > 100000 { // crude bound on memory
			l.m = map[string]*window{}
		}
		w = &window{start: t}
		l.m[key] = w
	}
	w.n++
	return w.n <= l.max
}

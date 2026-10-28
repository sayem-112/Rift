// Package ratelimit provides keyed token-bucket limiters.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter keeps one token bucket per key (an IP, an email, a user ID).
// Buckets idle longer than the cleanup interval are dropped, so memory is
// bounded by the number of recently active keys.
//
// Limits are per instance. That is acceptable for abuse protection on a small
// deployment; a shared store is the upgrade path if instances multiply.
type Limiter struct {
	rate  rate.Limit
	burst int

	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
	now     func() time.Time
}

type bucket struct {
	limiter *rate.Limiter
	seen    time.Time
}

const idleAfter = 10 * time.Minute

// New allows `events` per `per`, with bursts up to `burst`.
func New(events int, per time.Duration, burst int) *Limiter {
	return &Limiter{
		rate:    rate.Limit(float64(events) / per.Seconds()),
		burst:   burst,
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// Allow reports whether an event for key may happen now. When it may not,
// retryAfter says how long until it will.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastGC) > idleAfter {
		for k, b := range l.buckets {
			if now.Sub(b.seen) > idleAfter {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}

	b, found := l.buckets[key]
	if !found {
		b = &bucket{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[key] = b
	}
	b.seen = now

	r := b.limiter.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// Package ratelimit provides a simple in-memory token-bucket rate
// limiter keyed by an arbitrary string (typically a client IP).
//
// This is a per-process limiter. It is not shared across instances
// and a restart resets the buckets. For a multi-instance deployment,
// the store should be replaced with a Redis-backed implementation.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastFill time.Time
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	rate  float64
	burst float64
	now   func() time.Time
}

// New creates a limiter that allows `burst` requests in a single
// burst and refills at `rate` tokens per second.
func New(rate float64, burst float64) *Limiter {
	return NewWithNow(rate, burst, time.Now)
}

// NewWithNow is like New but lets callers inject a clock for tests.
func NewWithNow(rate float64, burst float64, now func() time.Time) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		now:     now,
	}
}

// Allow returns true when the request identified by `key` is allowed
// under the current bucket state and decrements the bucket.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst - 1, lastFill: l.now()}
		l.buckets[key] = b
		return true
	}

	now := l.now()
	elapsed := now.Sub(b.lastFill).Seconds()
	if elapsed > 0 {
		b.tokens = minFloat(l.burst, b.tokens+elapsed*l.rate)
		b.lastFill = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

// RetryAfter returns the suggested wait before the next request would
// be allowed. It is only meaningful when Allow returns false.
func (l *Limiter) RetryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		return 0
	}
	now := l.now()
	elapsed := now.Sub(b.lastFill).Seconds()
	tokens := b.tokens
	if elapsed > 0 {
		tokens = minFloat(l.burst, b.tokens+elapsed*l.rate)
	}
	missing := 1 - tokens
	if missing <= 0 {
		return 0
	}
	return time.Duration(missing/l.rate*float64(time.Second)) + time.Millisecond
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Package ratelimit provides a token-bucket rate limiter. The package
// ships two implementations behind a single Limiter interface:
//
//   - Limiter (in-memory): a per-process token bucket. Lost on restart
//     and not shared across replicas. Use it for local development and
//     single-instance deployments.
//   - RedisLimiter: a Redis-backed implementation that shares buckets
//     across replicas. Use it whenever REDIS_URL is set.
//
// Both implementations honour the same Allow / RetryAfter contract
// and are safe for concurrent use.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter is the contract every rate limiter must satisfy. The HTTP
// middleware talks to this interface so callers can swap the
// implementation without changing the middleware.
type Limiter interface {
	// Allow returns true when the request identified by `key` is
	// allowed under the current bucket state and decrements the
	// bucket.
	Allow(key string) bool
	// RetryAfter returns the suggested wait before the next
	// request would be allowed. It is only meaningful when Allow
	// returns false.
	RetryAfter(key string) time.Duration
	// Close releases any resources held by the limiter.
	Close() error
}

// Ensure the in-memory Limiter satisfies the interface at compile time.
var _ Limiter = (*inMemoryLimiter)(nil)

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// Limiter is the in-memory token-bucket implementation. See the
// package comment for the trade-off.
type inMemoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	rate  float64
	burst float64
	now   func() time.Time
}

// New creates an in-memory limiter that allows `burst` requests in a
// single burst and refills at `rate` tokens per second.
func New(rate float64, burst float64) *inMemoryLimiter {
	return NewWithNow(rate, burst, time.Now)
}

// NewWithNow is like New but lets callers inject a clock for tests.
func NewWithNow(rate float64, burst float64, now func() time.Time) *inMemoryLimiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	if now == nil {
		now = time.Now
	}
	return &inMemoryLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		now:     now,
	}
}

// Allow returns true when the request identified by `key` is allowed
// under the current bucket state and decrements the bucket.
func (l *inMemoryLimiter) Allow(key string) bool {
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
func (l *inMemoryLimiter) RetryAfter(key string) time.Duration {
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

// Close is a no-op for the in-memory implementation.
func (l *inMemoryLimiter) Close() error { return nil }

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// --- RedisLimiter ---

// RedisLimiter is a Redis-backed token-bucket limiter. Each key has
// a Redis hash with two fields: `tokens` (float) and `last` (unix
// nanoseconds). The whole update is a single Lua script so the
// read-modify-write is atomic and safe across replicas.
type RedisLimiter struct {
	client *redis.Client
	script *redis.Script
	rate   float64
	burst  float64
}

// NewRedis builds a Redis-backed limiter. The client takes ownership
// of the underlying connection pool; call Close to release it.
func NewRedis(client *redis.Client, rate, burst float64) *RedisLimiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	script := redis.NewScript(`
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local now_ns = tonumber(ARGV[3])
		local data = redis.call("HMGET", KEYS[1], "tokens", "last")
		local tokens = tonumber(data[1])
		local last = tonumber(data[2])
		if tokens == nil then
			tokens = burst - 1
			last = now_ns
			redis.call("HMSET", KEYS[1], "tokens", tokens, "last", last)
			redis.call("PEXPIRE", KEYS[1], 60000)
			return 1
		end
		local elapsed = (now_ns - last) / 1e9
		if elapsed > 0 then
			tokens = math.min(burst, tokens + elapsed * rate)
			last = now_ns
		end
		if tokens < 1 then
			redis.call("HMSET", KEYS[1], "tokens", tokens, "last", last)
			redis.call("PEXPIRE", KEYS[1], 60000)
			return 0
		end
		tokens = tokens - 1
		redis.call("HMSET", KEYS[1], "tokens", tokens, "last", last)
		redis.call("PEXPIRE", KEYS[1], 60000)
		return 1
	`)
	return &RedisLimiter{client: client, script: script, rate: rate, burst: burst}
}

func (l *RedisLimiter) keyFor(name string) string {
	return "ratelimit:" + name + ":bucket"
}

func (l *RedisLimiter) Allow(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	res, err := l.script.Run(ctx, l.client,
		[]string{l.keyFor(name)},
		l.rate, l.burst, time.Now().UnixNano()).Result()
	if err != nil {
		return true // fail open — do not block traffic on Redis errors
	}
	switch v := res.(type) {
	case int64:
		return v == 1
	case string:
		return v == "1"
	default:
		return true
	}
}

func (l *RedisLimiter) RetryAfter(name string) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	tokens, err := l.client.HMGet(ctx, l.keyFor(name), "tokens", "last").Result()
	if err != nil || len(tokens) < 2 || tokens[0] == nil || tokens[1] == nil {
		return 0
	}
	t, err := strconv.ParseFloat(fmt.Sprint(tokens[0]), 64)
	if err != nil {
		return 0
	}
	last, err := strconv.ParseInt(fmt.Sprint(tokens[1]), 10, 64)
	if err != nil {
		return 0
	}
	elapsed := float64(time.Now().UnixNano()-last) / 1e9
	if elapsed > 0 {
		t = minFloat(l.burst, t+elapsed*l.rate)
	}
	missing := 1 - t
	if missing <= 0 {
		return 0
	}
	return time.Duration(missing/l.rate*float64(time.Second)) + time.Millisecond
}

func (l *RedisLimiter) Close() error {
	if l.client == nil {
		return nil
	}
	return l.client.Close()
}

var ErrLimiterClosed = errors.New("rate limiter closed")

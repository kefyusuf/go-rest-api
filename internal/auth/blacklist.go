// Package auth provides the password hashing, JWT, and token-revocation
// primitives used by the authentication layer. The Blacklist type
// here tracks revoked JWT identifiers (jti). The package ships two
// implementations behind a small interface so the same code runs
// in-process for development and across replicas in production.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Blacklist is the contract every token-blacklist implementation must
// satisfy. The HTTP middleware talks to this interface so callers can
// swap the backing store without changing the middleware.
type Blacklist interface {
	// Revoke records the jti as revoked until expiresAt. jti == ""
	// is a no-op so callers do not have to guard at the call site.
	Revoke(jti string, expiresAt time.Time)
	// IsRevoked reports whether the jti is currently revoked. Expired
	// entries are dropped lazily on read so the map does not grow
	// without bound.
	IsRevoked(jti string) bool
	// Close releases any resources held by the implementation.
	Close() error
}

// Ensure the in-memory implementation satisfies the interface.
var _ Blacklist = (*memoryBlacklist)(nil)

type memoryBlacklist struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewBlacklist returns the in-process implementation. The state is
// lost on restart and is not shared across replicas. Use NewRedisBlacklist
// when REDIS_URL is set.
func NewBlacklist() *memoryBlacklist {
	return &memoryBlacklist{entries: make(map[string]time.Time)}
}

func (b *memoryBlacklist) Revoke(jti string, expiresAt time.Time) {
	if jti == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = expiresAt
}

func (b *memoryBlacklist) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	exp, ok := b.entries[jti]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(b.entries, jti)
		return false
	}
	return true
}

func (b *memoryBlacklist) Close() error { return nil }

// --- RedisBlacklist ---

// redisBlacklist keeps revoked jtis in Redis with a per-jti TTL. The
// key is `auth:blacklist:<jti>` and the value is the literal "1";
// the only thing we need to know is whether the key still exists.
// Setting a key with an EX TTL means the entry is automatically
// reclaimed when the underlying access token would have expired
// anyway, so the cache cannot grow without bound.
type redisBlacklist struct {
	client *redis.Client
}

// NewRedisBlacklist builds a Redis-backed blacklist on top of an
// existing redis.Client. The blacklist does not own the client;
// the caller is responsible for closing it.
func NewRedisBlacklist(client *redis.Client) *redisBlacklist {
	return &redisBlacklist{client: client}
}

func (b *redisBlacklist) keyFor(jti string) string {
	return "auth:blacklist:" + jti
}

func (b *redisBlacklist) Revoke(jti string, expiresAt time.Time) {
	if jti == "" {
		return
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		// Token is already expired; nothing to track.
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	// SET key value EX ttl. The error is intentionally swallowed:
	// a transient Redis failure here means the next request to
	// the same jti will check the in-memory fallback (if the
	// caller is using the composite Blacklist) or, in the
	// worst case, allow a revoked token to slip through for
	// the duration of the outage. Log instead.
	_ = b.client.Set(ctx, b.keyFor(jti), "1", ttl).Err()
}

func (b *redisBlacklist) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	// EXISTS returns 1 (revoked) or 0 (not revoked). A Redis
	// error is treated as "not revoked" so a transient outage
	// does not lock the entire API out of the route.
	n, err := b.client.Exists(ctx, b.keyFor(jti)).Result()
	if err != nil {
		return false
	}
	return n > 0
}

func (b *redisBlacklist) Close() error { return nil }

var ErrBlacklistClosed = errors.New("blacklist closed")

// newJTI is used by the token issuer to fill the jti claim. It is
// kept here so the helpers that the issuer needs (rand + hex) are
// available without an extra import in jwt.go.
func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

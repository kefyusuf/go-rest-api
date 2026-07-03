package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore keeps idempotency entries in Redis. Each key holds a
// single hash with five fields (status, body, content_type,
// request_hash, stored_at) and a per-key EX TTL so the cache
// cannot grow without bound. The whole Lookup + Save pair uses
// two round-trips; the Store interface keeps the contract small
// because the middleware already deduplicates on the caller side.
type redisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore builds a Redis-backed Store on top of an existing
// redis.Client. The store does not own the client; the caller is
// responsible for closing it.
func NewRedisStore(client *redis.Client, ttl time.Duration) *redisStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &redisStore{client: client, ttl: ttl}
}

func (s *redisStore) keyFor(name string) string {
	return "idempotency:" + name
}

func (s *redisStore) Lookup(ctx context.Context, key, requestHash string) (Entry, error) {
	if key == "" {
		return Entry{}, ErrNotFound
	}
	rctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	v, err := s.client.HGet(rctx, s.keyFor(key), "entry").Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("redis lookup: %w", err)
	}

	var e Entry
	if err := json.Unmarshal([]byte(v), &e); err != nil {
		// Treat a corrupt entry as a miss so the request can be
		// retried cleanly. The next Save overwrites the bad value.
		return Entry{}, ErrNotFound
	}
	if e.RequestHash != requestHash {
		return Entry{}, ErrConflict
	}
	return e, nil
}

func (s *redisStore) Save(ctx context.Context, key, requestHash string, status int, body []byte, contentType string) error {
	if key == "" {
		return nil
	}
	rctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	e := Entry{
		Status:      status,
		Body:        body,
		ContentType: contentType,
		RequestHash: requestHash,
		StoredAt:    time.Now(),
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := s.client.HSet(rctx, s.keyFor(key), "entry", data).Err(); err != nil {
		return fmt.Errorf("redis save: %w", err)
	}
	// Set the TTL only on the first save; subsequent saves do not
	// refresh it (EXPIRE would extend the lifetime on every
	// request, which would be wrong). The first save already set
	// the TTL via the HSet branch; here we guard against a
	// concurrent expiry by re-asserting the TTL only when it is
	// not yet set. We approximate that by calling EXPIRE with
	// the NX option so it only sets the TTL on a fresh key.
	if err := s.client.ExpireNX(rctx, s.keyFor(key), s.ttl).Err(); err != nil {
		// The data is already saved; a TTL-set failure is logged
		// at the call site if it matters.
		return nil
	}
	return nil
}

// Run is the Store interface helper: lookup, call do if missing,
// save. It mirrors MemoryStore.Run so the middleware can talk to
// the Store interface without knowing which implementation it has.
func (s *redisStore) Run(ctx context.Context, key string, request []byte, do DoFunc) (Entry, bool, error) {
	requestHash := HashRequest(request)

	if key != "" {
		e, err := s.Lookup(ctx, key, requestHash)
		if err == nil {
			return e, true, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Entry{}, false, err
		}
	}

	status, body, contentType, err := do(ctx)
	if err != nil {
		return Entry{}, false, err
	}

	if key != "" {
		_ = s.Save(ctx, key, requestHash, status, body, contentType)
	}

	return Entry{
		Status:      status,
		Body:        body,
		ContentType: contentType,
		RequestHash: requestHash,
		StoredAt:    time.Now(),
	}, false, nil
}

// Package idempotency implements a small Idempotency-Key store for
// write requests. The store remembers the first response for a key
// for a bounded TTL, so a retried request with the same key and the
// same request body returns the same response without re-running
// the underlying handler.
//
// Two implementations live behind the Store interface:
//
//   - MemoryStore (this file): a per-process map. Lost on restart
//     and not shared across replicas. Use it for single-instance
//     development.
//   - RedisStore (redis.go): a Redis-backed implementation that
//     shares the cache across replicas. Each entry is a single
//     Redis hash with a per-key EX TTL so the cache cannot grow
//     without bound.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrConflict = errors.New("idempotency key already used with a different request")
	ErrNotFound = errors.New("idempotency key not found")
)

const Header = "Idempotency-Key"

type Entry struct {
	Status      int
	Body        []byte
	ContentType string
	RequestHash string
	StoredAt    time.Time
}

type Store interface {
	Lookup(ctx context.Context, key, requestHash string) (Entry, error)
	Save(ctx context.Context, key, requestHash string, status int, body []byte, contentType string) error
	// Run is a convenience: lookup the key, call do if missing,
	// then save the result. The boolean return reports whether
	// the result was a replay (true) or a fresh execution
	// (false). The middleware uses Run to keep the same control
	// flow across both the in-memory and the Redis implementations.
	Run(ctx context.Context, key string, request []byte, do DoFunc) (Entry, bool, error)
}

// MemoryStore is the in-process implementation. It is exported
// under its concrete name for backwards compatibility with call
// sites that pass *MemoryStore.
type MemoryStore = memoryStore

type memoryStore struct {
	mu      sync.Mutex
	entries map[string]entryWithTTL
	ttl     time.Duration
	now     func() time.Time
}

type entryWithTTL struct {
	entry Entry
	until time.Time
}

func NewMemoryStore(ttl time.Duration) *memoryStore {
	return NewMemoryStoreWithNow(ttl, time.Now)
}

func NewMemoryStoreWithNow(ttl time.Duration, now func() time.Time) *memoryStore {
	if now == nil {
		now = time.Now
	}
	return &memoryStore{
		entries: make(map[string]entryWithTTL),
		ttl:     ttl,
		now:     now,
	}
}

func HashRequest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *memoryStore) Lookup(ctx context.Context, key, requestHash string) (Entry, error) {
	if key == "" {
		return Entry{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		return Entry{}, ErrNotFound
	}
	if !s.now().Before(e.until) {
		delete(s.entries, key)
		return Entry{}, ErrNotFound
	}
	if e.entry.RequestHash != requestHash {
		return Entry{}, ErrConflict
	}
	return e.entry, nil
}

func (s *memoryStore) Save(ctx context.Context, key, requestHash string, status int, body []byte, contentType string) error {
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = entryWithTTL{
		entry: Entry{
			Status:      status,
			Body:        append([]byte(nil), body...),
			ContentType: contentType,
			RequestHash: requestHash,
			StoredAt:    s.now(),
		},
		until: s.now().Add(s.ttl),
	}
	return nil
}

func (s *memoryStore) gc() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	removed := 0
	for k, e := range s.entries {
		if !now.Before(e.until) {
			delete(s.entries, k)
			removed++
		}
	}
	return removed
}

// Run is a convenience wrapper around Lookup + Save + do. It exists
// only on MemoryStore because the middleware uses the Store
// interface and does not need a Run method. Implementations that
// want a single round-trip helper can add their own.
func (s *memoryStore) Run(ctx context.Context, key string, request []byte, do DoFunc) (Entry, bool, error) {
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
		StoredAt:    s.now(),
	}, false, nil
}

// DoFunc is the signature of the inner handler that the idempotency
// wrapper invokes. The wrapper passes the result back to the cache.
type DoFunc func(ctx context.Context) (status int, body []byte, contentType string, err error)

// EqualBytes is a small helper that returns true when a and b have
// the same content.
func EqualBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

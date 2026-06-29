// Package cache provides a small key-value cache abstraction with
// an in-process map implementation and a Redis adapter.
//
// The interface is intentionally tiny: Get, Set, Delete, Close. Both
// implementations are safe for concurrent use.
package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]entry
	now     func() time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]entry),
		now:     time.Now,
	}
}

func (c *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, ErrMiss
	}
	if !c.now().Before(e.expiresAt) {
		delete(c.entries, key)
		return nil, ErrMiss
	}
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, nil
}

func (c *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	stored := make([]byte, len(value))
	copy(stored, value)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{value: stored, expiresAt: c.now().Add(ttl)}
	return nil
}

func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

func (c *MemoryCache) Close() error {
	return nil
}

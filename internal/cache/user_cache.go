package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// UserCache is a typed cache wrapper for serialisable user payloads
// keyed by user id. It does not know about model.User directly so it
// stays a generic, transport-agnostic helper.
type UserCache struct {
	cache  Cache
	ttl    time.Duration
	keyFmt string
}

func NewUserCache(c Cache, ttl time.Duration) *UserCache {
	return &UserCache{cache: c, ttl: ttl, keyFmt: "user:%d"}
}

func (c *UserCache) Get(ctx context.Context, id int64) ([]byte, error) {
	if c == nil || c.cache == nil {
		return nil, ErrMiss
	}
	return c.cache.Get(ctx, c.keyFor(id))
}

func (c *UserCache) Set(ctx context.Context, id int64, value []byte) error {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.Set(ctx, c.keyFor(id), value, c.ttl)
}

// SetWithTTL is for tests that want a non-default TTL. Production
// callers should use Set so the cache enforces a single ttl.
func (c *UserCache) SetWithTTL(ctx context.Context, id int64, value []byte, ttl time.Duration) error {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.Set(ctx, c.keyFor(id), value, ttl)
}

func (c *UserCache) Delete(ctx context.Context, id int64) error {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.Delete(ctx, c.keyFor(id))
}

func (c *UserCache) keyFor(id int64) string {
	return fmt.Sprintf(c.keyFmt, id)
}

func JSONEncode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func JSONDecode(data []byte, v any) error {
	if len(data) == 0 {
		return errors.New("empty cache value")
	}
	return json.Unmarshal(data, v)
}

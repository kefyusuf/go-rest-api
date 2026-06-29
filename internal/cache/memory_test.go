package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-lang/internal/cache"
)

func TestMemoryCacheSetAndGet(t *testing.T) {
	c := cache.NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("expected v1, got %q", string(got))
	}
}

func TestMemoryCacheMiss(t *testing.T) {
	c := cache.NewMemoryCache()
	_, err := c.Get(context.Background(), "missing")
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}
}

func TestMemoryCacheExpiry(t *testing.T) {
	c := cache.NewMemoryCache()
	if err := c.Set(context.Background(), "k1", []byte("v1"), 5*time.Millisecond); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	_, err := c.Get(context.Background(), "k1")
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected ErrMiss after expiry, got %v", err)
	}
}

func TestMemoryCacheDelete(t *testing.T) {
	c := cache.NewMemoryCache()
	ctx := context.Background()
	_ = c.Set(ctx, "k1", []byte("v1"), time.Minute)
	if err := c.Delete(ctx, "k1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := c.Get(ctx, "k1")
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected miss after delete, got %v", err)
	}
}

func TestMemoryCacheSetDoesNotStoreZeroTTL(t *testing.T) {
	c := cache.NewMemoryCache()
	if err := c.Set(context.Background(), "k1", []byte("v1"), 0); err != nil {
		t.Fatalf("set with zero ttl: %v", err)
	}
	_, err := c.Get(context.Background(), "k1")
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected miss for zero-ttl entry, got %v", err)
	}
}

func TestMemoryCacheGetReturnsCopy(t *testing.T) {
	c := cache.NewMemoryCache()
	original := []byte("hello")
	_ = c.Set(context.Background(), "k1", original, time.Minute)
	got, _ := c.Get(context.Background(), "k1")
	got[0] = 'X'
	again, _ := c.Get(context.Background(), "k1")
	if string(again) != "hello" {
		t.Fatalf("expected first read not to mutate stored value, got %q", string(again))
	}
}

func TestUserCacheGetMissIsCacheMiss(t *testing.T) {
	uc := cache.NewUserCache(cache.NewMemoryCache(), time.Minute)
	_, err := uc.Get(context.Background(), 42)
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}
}

func TestUserCacheNilSafe(t *testing.T) {
	var uc *cache.UserCache
	if err := uc.Set(context.Background(), 1, []byte("v")); err != nil {
		t.Fatalf("nil UserCache Set should be no-op, got %v", err)
	}
	if err := uc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("nil UserCache Delete should be no-op, got %v", err)
	}
}

package store_test

import (
	"testing"

	"go-lang/internal/cache"
	"go-lang/internal/model"
	"go-lang/internal/store"
)

func TestCachedUserStoreServesFromCacheOnSecondRead(t *testing.T) {
	inner := store.NewMemoryUserStore()
	created, err := inner.Create(model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	memCache := cache.NewMemoryCache()
	cached := store.NewCachedUserStore(inner, cache.NewUserCache(memCache, 5_000_000_000), 5_000_000_000)

	if _, err := cached.GetByID(created.ID); err != nil {
		t.Fatalf("first read: %v", err)
	}

	if err := inner.Delete(created.ID); err != nil {
		t.Fatalf("delete on inner: %v", err)
	}

	user, err := cached.GetByID(created.ID)
	if err != nil {
		t.Fatalf("expected cached read to still return the user, got %v", err)
	}
	if user.Email != "ada@example.com" {
		t.Fatalf("expected cached user, got %+v", user)
	}
}

func TestCachedUserStoreInvalidatesOnUpdate(t *testing.T) {
	inner := store.NewMemoryUserStore()
	created, _ := inner.Create(model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})

	memCache := cache.NewMemoryCache()
	cached := store.NewCachedUserStore(inner, cache.NewUserCache(memCache, 5_000_000_000), 5_000_000_000)

	if _, err := cached.GetByID(created.ID); err != nil {
		t.Fatalf("prime cache: %v", err)
	}

	if _, err := cached.Update(created.ID, model.UpdateUserRequest{
		Name: "Ada B.", Email: "ada@example.com",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	raw, err := memCache.Get(t.Context(), "user:1")
	if err == nil && raw != nil {
		t.Fatalf("expected cache entry to be invalidated, got %s", string(raw))
	}
}

func TestCachedUserStoreInvalidatesOnDelete(t *testing.T) {
	inner := store.NewMemoryUserStore()
	created, _ := inner.Create(model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})

	memCache := cache.NewMemoryCache()
	cached := store.NewCachedUserStore(inner, cache.NewUserCache(memCache, 5_000_000_000), 5_000_000_000)

	if _, err := cached.GetByID(created.ID); err != nil {
		t.Fatalf("prime cache: %v", err)
	}

	if err := cached.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	raw, err := memCache.Get(t.Context(), "user:1")
	if err == nil && raw != nil {
		t.Fatalf("expected cache entry to be invalidated, got %s", string(raw))
	}
}

func TestCachedUserStoreFallsBackToStoreOnMiss(t *testing.T) {
	inner := store.NewMemoryUserStore()
	created, _ := inner.Create(model.CreateUserRequest{
		Name: "Ada", Email: "ada@example.com", Password: "pw",
	})

	memCache := cache.NewMemoryCache()
	cached := store.NewCachedUserStore(inner, cache.NewUserCache(memCache, 5_000_000_000), 5_000_000_000)

	user, err := cached.GetByID(created.ID)
	if err != nil {
		t.Fatalf("miss read: %v", err)
	}
	if user.ID != created.ID {
		t.Fatalf("expected id %d, got %d", created.ID, user.ID)
	}
}

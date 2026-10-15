# ADR-0002: The cache is an optimization — cache errors fall through to the database, invalidation is best-effort

Status: Accepted
Date: 2026-10-15

## Context

`CachedUserStore` sits in front of the real store on the read path. If a cache failure propagated,
Redis becoming unavailable would turn a performance feature into an availability incident — every user
read would 500 while the database was perfectly healthy. The alternative is to swallow cache errors,
which trades a guarantee for staleness: a value that fails to write back, or an invalidation that
fails, leaves the database as the only source of truth while the cache may still hold an older entry.

## Decision

Reads treat the cache as advisory (`internal/store/cached_user_store.go:29-39`):

```go
if data, err := s.cache.Get(ctx, int64(id)); err == nil {
        if jerr := cache.JSONDecode(data, &u); jerr == nil {
                return u, nil
        }
}
user, err := s.inner.GetByID(id)   // reached on miss, cache error, OR decode error
```

Only `GetByID` is cached; `List` (`:24`) and `GetByEmail` (`:57`) always hit the store. Population is
write-through-ish with a TTL of **5 minutes** by default (`config.go:68`, env `USER_CACHE_TTL`) and the
error from `SetWithTTL` is discarded (`:51`), as is the error from invalidating `Delete` (`:99`).
Writes (`Create`, `Update`, `UpdatePassword`, `Delete`) call `invalidate` at `:66,75,83,91`.

## Summary (EN)

A cache read that errors — connection failure, wrong payload, decode failure — silently falls back to
the database, so the user sees a slower correct response instead of an error. A cache write or
invalidation that errors is ignored for the same reason: the cache is allowed to be behind, never
allowed to be fatal. The bound on that staleness is the 5-minute TTL, plus the fact that only the
single-record read path is cached at all — listing users and email lookups never touch it. The cost is
that a failed invalidation is invisible in logs and metrics, and no test covers the backend-error path,
only the miss path.

## Consequences

- Redis downtime degrades latency, not correctness or availability, and no cache state can produce a
  wrong answer that the TTL will not eventually repair.
- Cost: an invalidation failure is indistinguishable from success (`_ =` at `:51` and `:99`), so a
  stale read can outlive its intended window silently; a metrics counter or a log line on cache-write
  failure is the cheap fix and is not implemented.

## Verification

- `internal/store/cached_user_store_test.go`: `TestCachedUserStoreServesFromCacheOnSecondRead`,
  `TestCachedUserStoreInvalidatesOnUpdate`, `TestCachedUserStoreInvalidatesOnDelete`,
  `TestCachedUserStoreFallsBackToStoreOnMiss` — all pure unit, using `MemoryCache`.
- `internal/cache/memory_test.go` — the backend contract.
- Unpinned today: a cache backend returning an error (only misses are tested), and `RedisCache`, which
  has no test file of its own.

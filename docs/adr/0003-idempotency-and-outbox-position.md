# ADR-0003: Idempotency keys replay the stored response; the outbox is durable but deliberately not transactional

Status: Accepted
Date: 2026-10-15

## Context

Two consistency features landed in separate layers and are easy to over-read. `Idempotency-Key` makes a
retried `POST` safe, and the outbox keeps an event alive until a broker accepts it. Neither one, alone,
gives exactly-once processing, and the README's layer table calls layer 11 "Outbox + dispatcher +
Publisher" — which a reader may take for a transactional outbox. It is not one, and this ADR is where
that gets said out loud.

## Decision

**Idempotency** (`internal/server/idempotency_middleware.go`, `internal/idempotency/`):
- The key is the `Idempotency-Key` header, scoped to `POST /users` and `POST /auth/register`.
- The request body is hashed with SHA-256 (`idempotency/memory.go:86`); the same key with a different
  body is `ErrConflict` → **409** (`middleware.go:68-73`) rather than a silent second answer.
- A matching replay returns the stored status, body and content type plus `Idempotent-Replay: true`
  (`middleware.go:94-102`). Records live in memory or Redis (`idempotency:<key>`, `redis.go:35`) with a
  **24-hour** TTL (`config.go:73`).
- The store is advisory: if it errors, the request proceeds (`middleware.go:76-78`).

**Events** (`internal/events/`):
- When a database is configured, `cmd/api/main.go:316-330` wires `DBOutbox` — a real `events_outbox`
  table. The dispatcher dequeues with `FOR UPDATE SKIP LOCKED` (`outbox.go:270`) so concurrent
  dispatchers cannot claim the same row, publishes, then deletes the row in `MarkPublished` (`:304`).
  A publish failure leaves the row in place for the next attempt, and Kafka is published synchronously
  with `RequireAll` acks (`kafka.go:50,66`). **Delivery is at-least-once.** Without a database the
  in-memory outbox is used and a failed publish drops the event (`outbox.go:100`, dispatcher `:165`).
- The business write and the event row are **not** in one transaction: `handler/user.go:115` calls
  `h.outbox.Enqueue(...)` after `store.Create` has already committed, and discards the error (`_ =`).

## Summary (EN)

Retrying a create with the same `Idempotency-Key` replays the original response instead of creating a
second row, and reusing that key with a different body is a 409 rather than a guess. Idempotency records
expire after 24 hours and the middleware lets the request through if the record store is unhealthy, so
duplicate protection can lapse exactly when Redis is down. Events go through a Postgres-backed outbox
claimed with `FOR UPDATE SKIP LOCKED`, published with all acks, and deleted only after success, which
makes delivery at-least-once. It is not a transactional outbox: the user row commits first and the event
is inserted afterwards in a separate statement whose error is ignored, so a crash between the two loses
the event. Consumers must stay idempotent on `event.id`.

## Consequences

- Both features fail in the same direction — availability over strictness — which is a coherent rule
  across this API rather than two accidents.
- Cost, named instead of hidden: (1) a crash or a failed INSERT between `Create` and `Enqueue` loses
  `user.created` with no trace; (2) with Redis down, two retries can both execute; (3) the in-memory
  outbox is dev-only because publish failures drop events. The fix for (1) is to move the enqueue
  inside the store's transaction — which needs `UserStore.Create` to accept a `*sql.Tx` — and that is a
  breaking interface change, so it stays out of a patch release.

## Verification

- `internal/idempotency/memory_test.go`: `TestMemoryStoreRunReplaysCachedResponse`,
  `TestMemoryStoreConflictOnDifferentBody`, `TestMemoryStoreExpiry`.
- `internal/server/idempotency_test.go`: `TestIdempotencyReplaysSameKeySameBody`,
  `TestIdempotencyRejectsSameKeyDifferentBody`, `TestIdempotencyReplaysOnRegister` — HTTP-level,
  no Redis required.
- `internal/events/events_test.go`: `TestOutboxEnqueueDequeue`,
  `TestDispatcherPublishesAndStopsOnContext`; `internal/events/kafka_test.go`:
  `TestKafkaPublisherRejectsEmptyBrokers`, `TestKafkaPublisherFailsOnUnreachableBroker`.
- `internal/events/db_test.go`: `TestDBOutboxEnqueueDequeueMarkPublished`,
  `TestDBOutboxLenCountsUnpublished` — these skip unless `DATABASE_URL` points at a live Postgres, so
  in practice they run in the Docker test job, not on a bare `go test ./...`.
- Unpinned today: the non-atomicity itself. No test asserts that a failed enqueue after a successful
  create is visible anywhere, because currently it is not.

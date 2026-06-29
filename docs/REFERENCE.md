# Reference

> This is the operations reference. The full endpoint catalogue, the
> hardening details (rate limit, CORS, security headers), the idempotency
> contract, the background-job model, the event outbox, the cache, ready
> curl examples, the Swagger surface, and the test layout all live here.

---

## Endpoints

### GET /health

Confirms that the application is running. Kept for backwards
compatibility; prefer `/health/live` for new probes.

Example response:

```json
{
  "message": "API is running",
  "status": "ok"
}
```

### GET /health/live

Liveness probe. Returns `200 OK` if the process is up. It does not
depend on any external service, so a transient database blip will
not restart the pod.

### GET /health/ready

Readiness probe. Pings the database with a short timeout. Returns
`200 OK` when the database is reachable, `503 Service Unavailable`
otherwise. The in-memory store path reports `ok` and skips the
database check.

### GET /metrics

Prometheus exposition. The registry includes:

- `http_requests_total{method,path,status}` (counter)
- `http_request_duration_seconds{method,path,status}` (histogram)
- `http_in_flight_requests` (gauge)
- `go_*` and `process_*` (Go runtime and process collectors)

All custom metrics are labelled with `service` and `environment`
const labels so multiple environments can share a Prometheus
instance.

Example:

```bash
curl http://localhost:8080/metrics
```

### GET /users

Returns all users.

Example response:

```json
{
  "data": [
    {
      "id": 1,
      "name": "Ada Lovelace",
      "email": "ada@example.com"
    }
  ],
  "meta": {
    "nextCursor": null,
    "limit": 20
  }
}
```

### GET /users/{id}

Returns a single user.

### POST /users

Creates a new user. The password is hashed with bcrypt before it is
stored; it is never returned in any response.

Example request body:

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "password": "correct-horse-battery-staple"
}
```

### POST /auth/register

Creates a new user account and returns a JWT access token and a refresh
token. The password is hashed with bcrypt before it is stored; it is
never returned in any response.

Example request body:

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "password": "correct-horse-battery-staple"
}
```

Successful response (`201 Created`):

```json
{
  "accessToken": "<jwt>",
  "refreshToken": "<jwt>",
  "tokenType": "Bearer",
  "expiresIn": 900,
  "expiresAt": "2026-06-29T13:00:00Z",
  "refreshExpiresAt": "2026-07-06T12:00:00Z",
  "user": {
    "id": 1,
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  }
}
```

If the email is already registered, the response is `409 Conflict`.

### POST /auth/login

Exchanges email and password for a JWT access token and a refresh
token.

Example request body:

```json
{
  "email": "ada@example.com",
  "password": "correct-horse-battery-staple"
}
```

Successful response (`200 OK`):

```json
{
  "accessToken": "<jwt>",
  "refreshToken": "<jwt>",
  "tokenType": "Bearer",
  "expiresIn": 900,
  "expiresAt": "2026-06-29T13:00:00Z",
  "refreshExpiresAt": "2026-07-06T12:00:00Z",
  "user": {
    "id": 1,
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  }
}
```

Invalid credentials return `401 Unauthorized` with a generic
`UNAUTHORIZED` code and the message `invalid email or password`. The
message is the same whether the email is unknown or the password is
wrong, so it does not leak which side failed.

### POST /auth/logout

Revokes the bearer token used in this request. After logout, the same
token cannot be used to call `/me` or any other protected endpoint.

Example request:

```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer <access-token>"
```

Successful response: `204 No Content`. The token blacklist lives in
process memory, so it is cleared on restart. A production deployment
would move it to Redis or another shared store (`layer/06`).

### POST /auth/refresh

Trades a valid refresh token for a new access token and a new refresh
token. The old refresh token is single-use and is rejected on replay.

Example request body:

```json
{
  "refreshToken": "<refresh-jwt>"
}
```

Successful response: `200 OK` with the same `LoginResponse` shape as
`/auth/login`.

### POST /auth/forgot-password

Starts the password reset flow. Always returns `202 Accepted` whether
or not the email is registered, to avoid leaking which emails exist on
the platform.

In this in-memory starter the response also includes the reset token
directly so you can complete the flow locally:

```json
{
  "accepted": true,
  "token": "<reset-token>",
  "expiresAt": "2026-06-29T14:00:00Z",
  "resetUrl": "/auth/reset-password"
}
```

A production deployment would email the token to the user and never
include it in the response.

### POST /auth/reset-password

Resets a user's password using a valid reset token from the
`/auth/forgot-password` response.

Example request body:

```json
{
  "token": "<reset-token>",
  "password": "new-correct-horse-battery-staple"
}
```

Successful response: `204 No Content`. The reset token is single-use.

### GET /me

Returns the user behind a valid bearer token.

Example request:

```bash
curl http://localhost:8080/me \
  -H "Authorization: Bearer <jwt>"
```

Successful response (`200 OK`):

```json
{
  "id": 1,
  "name": "Ada Lovelace",
  "email": "ada@example.com"
}
```

Missing, malformed, expired, or revoked tokens return `401 Unauthorized`.

### PUT /users/{id}

Updates an existing user.

Example request body:

```json
{
  "name": "Ada Byron",
  "email": "ada.byron@example.com"
}
```

### DELETE /users/{id}

Deletes a user. Returns `204 No Content` on success.

### Error behavior

If a second user is created with the same `email`, or an update uses
another user's email, the API returns `409 Conflict`.

If a request body is missing, empty, or unparseable, the API returns
`400 Bad Request` with a clear message.

If a body endpoint receives a `Content-Type` other than
`application/json`, the API returns `415 Unsupported Media Type`.

If a path parameter is invalid, the API returns `400 Bad Request`.

---

## Hardening

Three layers protect the API beyond the basic request/response
contract.

### Rate limiting

Two token-bucket limiters, in-process per instance, keyed by client
IP (or `X-Forwarded-For` / `X-Real-IP` if present):

- Global limiter applied to `/users`, `/users/{id}`, `/me`:
  default `RATE_LIMIT_PER_SECOND=20`, `RATE_LIMIT_BURST=40`
- Auth limiter applied to `/auth/login`, `/auth/register`,
  `/auth/refresh`, `/auth/forgot-password`, `/auth/reset-password`:
  default `AUTH_RATE_LIMIT_PER_SECOND=5`, `AUTH_RATE_LIMIT_BURST=10`

A request that exceeds the limit returns `429 Too Many Requests` with
a `RATE_LIMITED` code, the shared error envelope, and a
`Retry-After` header in seconds.

The limiter is per-process and lost on restart. For a multi-instance
deployment the limiter should be backed by Redis. A future
`layer/08` will replace this with a Redis-backed store.

### CORS

`CORS(opts.CORS)` adds a small, opt-in CORS layer. Origins are
configured with `CORS_ALLOWED_ORIGINS` as a comma-separated list. The
middleware echoes the request's `Origin` back as
`Access-Control-Allow-Origin` only when the origin is in the
allow-list, and sets `Vary: Origin` so caches do not serve the
wrong CORS headers to a different origin. A `Vary: *` shorthand is
deliberately not used. Preflight `OPTIONS` requests return `204 No
Content` with `Access-Control-Allow-Methods`,
`Access-Control-Allow-Headers`, and `Access-Control-Max-Age`.

When `CORS_ALLOWED_ORIGINS` is empty the middleware still runs but
does not set any `Access-Control-*` header, so a browser-based client
cannot call the API cross-origin. This is the right default for an
API used by server-to-server clients.

### Security headers

Every response carries a small set of safe-by-default headers:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`

These are set only if the handler has not already set them, so
specific endpoints can override individual values without
duplicating the rest.

---

## Idempotency

`POST /users` and `POST /auth/register` accept an optional
`Idempotency-Key` request header. The first call with a given key
runs the handler and caches the response (status, body, content
type) for `IDEMPOTENCY_TTL` (default `24h`).

A retry with the same key and the same request body returns the
cached response with an `Idempotent-Replay: true` header. The
handler does not run a second time, so the underlying store is not
re-mutated.

A retry with the same key but a different body returns `409
Conflict` with the shared error envelope. The error code is
`CONFLICT` and the message is `idempotency key reused with a
different request body`. This protects against an honest client
re-using an old key by accident.

When `Idempotency-Key` is not set, the handler runs normally and
the standard duplicate checks (for example, duplicate email) take
over.

The in-memory store is per process. A future `layer/11` will add a
Redis-backed store so retries across instances land on the same
cache. Until then, an instance restart drops the cache and the
first request after restart always runs.

---

## Background jobs

`internal/jobs` is a small in-process background-job system. It
ships with:

- A `Queue` interface and an in-memory implementation backed by a
  slice and a condition variable. The in-memory queue honours
  per-job `RunAfter` for retry backoff.
- A `Registry` that owns a worker pool. Each job runs with a
  30s timeout; failed jobs are retried with exponential backoff
  (1s, 2s, 4s, ... capped at 1 minute) up to `MaxRetries` times
  (default 2). After that the job lands in the dead-letter list.
- A `DeadLetter` interface and an in-memory implementation. The
  list is process-local; an instance restart drops it.

Production wiring would replace the in-memory queue with RabbitMQ
or another broker and the dead-letter list with a shared store.
Both keep the same `Handler` interface, so handlers do not need
to change.

The starter ships a single mock job, `welcome_email`, that simply
logs that it ran. Real handlers can be added by calling
`jobReg.Register("type", jobs.HandlerFunc(...))` in `main.go`.

---

## Event publishing (outbox)

`internal/events` is a small publish-subscribe layer that follows
the outbox pattern: handlers that want to emit an event call
`outbox.Enqueue(...)`; a background dispatcher reads the outbox
and forwards every event to the active `Publisher`.

The starter ships a `LoggingPublisher` that just logs the event
attributes. The `Publisher` interface is intentionally small so
the in-memory implementation can be replaced with Kafka (via
`segmentio/kafka-go` or `IBM/sarama`) or another broker without
changing the call sites. The dispatcher owns the outbox drain and
the publisher lifecycle, so swapping the publisher is a one-line
change in `main.go`.

Why an outbox? The transaction that creates the user and the
write that records the event happen in the same logical step,
even when the actual broker is a separate process. A handler that
calls the broker directly can lose the event if the broker is down
between the two writes; the outbox collapses that to a single
in-process write that the dispatcher flushes at-least-once.

In the starter, the outbox is in-process memory. Production should
back it with the same database the user store uses (a `outbox`
table is enough) so events survive a process restart.

The starter emits one event: `user.created`, published with the
key being the new user id and the payload being a small JSON
document with `id`, `name`, and `email`. Real applications add
more events (`user.updated`, `user.deleted`, ...).

---

## Caching

`GET /users/{id}` and `GET /me` are served through a read-through
cache (`internal/store/cached_user_store.go`) that wraps the
underlying `UserStore`. The cache keeps a serialised user payload
keyed by id with a configurable TTL (default `5m`, override with
`USER_CACHE_TTL`).

When `REDIS_URL` is set, the cache uses `go-redis` and is shared
across instances. When `REDIS_URL` is empty, an in-process map is
used; the warning is logged at startup and the cache is lost on
restart. This is the right behaviour for the in-memory starter and
fine for a single-instance deployment but should be replaced before
scaling out.

Cache invalidation rules:

- `Create`, `Update`, `UpdatePassword`, and `Delete` invalidate the
  cache entry for the affected user id
- The cache only caches `GetByID`. List and email lookup always go to
  the store so the underlying data cannot drift

Cache failures never bring the API down. A miss on the cache is
treated as a cache miss and the loader runs. A write failure logs at
debug level (not in this layer) and leaves the cache as-is.

---

## Quick curl examples

A few examples you can run directly from the terminal to try things
quickly.

### 1. Health check

```bash
curl http://localhost:8080/health
```

### 2. Create a user

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"correct-horse-battery-staple"}'
```

### 3. List all users

```bash
curl http://localhost:8080/users
```

### 4. Update a user

```bash
curl -X PUT http://localhost:8080/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Byron","email":"ada.byron@example.com"}'
```

Note: when using PowerShell, `curl` may behave like `Invoke-WebRequest`.
Use `curl.exe` if needed.

### 5. Delete a user

```bash
curl -X DELETE http://localhost:8080/users/1
```

### 5a. Register a new account

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"correct-horse-battery-staple"}'
```

### 5b. Log out

```bash
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer <access-token>"
```

### 5c. Refresh an access token

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":"<refresh-token>"}'
```

### 6. Trigger a duplicate email error

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"correct-horse-battery-staple"}'
```

The second call returns `409 Conflict` with a `CONFLICT` error code.

### 7. Trigger a validation error

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"","email":""}'
```

The response is `400 Bad Request` with a `VALIDATION_ERROR` code and
field-level `details`.

---

## Swagger

Swagger UI address:

```text
http://localhost:8080/swagger/index.html
```

Swagger documentation is generated from annotations on the handler files.

To generate the docs:

```bash
go generate ./cmd/api
```

### Swagger annotation template

When adding a new endpoint, copy and adapt the following template:

```go
// CreateThing godoc
// @Summary Create thing
// @Description Creates a new record
// @Tags things
// @Accept json
// @Produce json
// @Param thing body model.CreateThingRequest true "New thing"
// @Success 201 {object} model.Thing
// @Failure 400 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /things [post]
func (h ThingHandler) CreateThing(w http.ResponseWriter, r *http.Request) {
    // handler body
}
```

Example with a path parameter:

```go
// GetThingByID godoc
// @Summary Get thing by ID
// @Description Returns a record by id
// @Tags things
// @Produce json
// @Param id path int true "Thing ID"
// @Success 200 {object} model.Thing
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /things/{id} [get]
func (h ThingHandler) GetThingByID(w http.ResponseWriter, r *http.Request) {
    // handler body
}
```

---

## Tests

### Run the existing integration tests

In-memory tests:

```bash
go test ./...
```

### Run integration tests with PostgreSQL

First, start the test database:

Windows:

```bash
scripts\test-db-up.bat
```

Unix-like environments:

```bash
sh scripts/test-db-up.sh
```

Then run the tests:

```bash
set "DATABASE_URL=postgres://postgres:postgres@localhost:5433/go_lang_test?sslmode=disable"
go test ./...
```

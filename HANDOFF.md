# Handoff

## Session summary

This session started with an empty `main` branch containing a small Go
`net/http` starter and built it up to a 12-layer production-shaped REST
API with auth, caching, rate limiting, idempotency, background jobs,
event publishing, observability, CI/CD, and Kubernetes manifests. Every
layer landed on its own branch, was reviewed, and was merged into
`main`. The final state is on `origin/main` at commit `a36f43d`.

## Final state

### Repository

- **URL:** https://github.com/kefyusuf/go-rest-api
- **Default branch:** `main`
- **HEAD commit on `main`:** `a36f43d` — *docs: split the README into focused docs/ files*
- **All 12 layer branches** are still on the remote as a learning
  history (`origin/layer/02-api-consistency` …
  `origin/layer/12-k8s-cloud`).
- **No open pull requests** against `main`. Five Dependabot PRs are
  open; the `dependabot-auto-merge.yml` workflow handles patch and
  minor updates automatically.

### Branches and PRs

| # | Layer | Branch | PR | What it adds |
|---|---|---|---|---|
| 0 | Baseline | `main` | — | Health, users CRUD, Swagger, Postgres/memory store, Docker, integration tests, request logging |
| 1 | API consistency | `layer/02-api-consistency` | [#1](https://github.com/kefyusuf/go-rest-api/pull/1) | Standard error envelope, `204 No Content` for delete, list envelope with `data/meta`, content-type enforcement, `nextCursor` |
| 2 | Config + logging | `layer/02a-config-and-logger` | [#2](https://github.com/kefyusuf/go-rest-api/pull/2) | `internal/config` env-driven loader, `log/slog` JSON logger, `X-Request-Id` middleware |
| 3 | Server hardening | `layer/02b-server-hardening` | [#3](https://github.com/kefyusuf/go-rest-api/pull/3) | `http.Server` timeouts, graceful shutdown, panic recovery, body-size limit |
| 4 | JWT auth | `layer/03-auth-jwt` | [#4](https://github.com/kefyusuf/go-rest-api/pull/4) | `POST /auth/login`, JWT issuance (HS256, 32-byte secret), `GET /me`, `RequireAuth` middleware, bcrypt |
| 5 | Auth session | `layer/04-auth-session` | [#5](https://github.com/kefyusuf/go-rest-api/pull/5) | `POST /auth/register`, `POST /auth/refresh` with rotation, `POST /auth/logout` with jti blacklist, forgot/reset password |
| 6 | Observability | `layer/05-observability` | [#6](https://github.com/kefyusuf/go-rest-api/pull/6) | Prometheus metrics, `/health/live`, `/health/ready`, slog access log |
| 7 | User cache | `layer/06-performance-cache` | [#7](https://github.com/kefyusuf/go-rest-api/pull/7) | `internal/cache` (memory + Redis), `CachedUserStore` with read-through and write invalidation |
| 8 | API hardening | `layer/07-api-hardening` | [#8](https://github.com/kefyusuf/go-rest-api/pull/8) | Per-IP rate limiting (token bucket), opt-in CORS, security headers, `Allow` header on 405 |
| 9 | Idempotency | `layer/08-idempotency-consistency` | [#9](https://github.com/kefyusuf/go-rest-api/pull/9) | `Idempotency-Key` support on `POST /users` and `POST /auth/register`, replay with `Idempotent-Replay: true`, 409 on same-key-different-body |
| 10 | Background jobs | `layer/09-async-processing` | [#10](https://github.com/kefyusuf/go-rest-api/pull/10) | In-process job queue with worker pool, exponential backoff (1s, 2s, 4s, max 1 min), max-retry, dead-letter list |
| 11 | Event publishing | `layer/10-event-driven` | [#11](https://github.com/kefyusuf/go-rest-api/pull/11) | Outbox + dispatcher + `Publisher` interface, `user.created` event published with key=user id |
| 12 | CI/CD | `layer/11-delivery-platform` | [#12](https://github.com/kefyusuf/go-rest-api/pull/12) | `.github/workflows/{ci,release,dependabot-auto-merge}.yml`, `Makefile`, `golangci.yml`, Dependabot config |
| 13 | K8s + Helm | `layer/12-k8s-cloud` | [#13](https://github.com/kefyusuf/go-rest-api/pull/13) | `deploy/k8s/` Kustomize bundle and `deploy/helm/go-rest-api/` Helm chart |

### Documentation layout

The README was split into focused documents under `docs/`:

| Topic | File |
| --- | --- |
| Layer history with branch + PR links, dependency chain, reading order | [docs/LAYERS.md](docs/LAYERS.md) |
| Tech decisions, per-layer design notes, error contract, auth model, cache, jobs, events, idempotency | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| CI/CD, Kubernetes, Helm, env variables, Docker network, start-up commands | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| Branch lifecycle, naming, contribution rules, project structure | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |
| Endpoint reference, hardening, idempotency, jobs, events, caching, curl examples, swagger, tests | [docs/REFERENCE.md](docs/REFERENCE.md) |

The README is now a 141-line landing page that points at all five.

## What works (verified end-to-end)

- `go build ./...` clean
- `go vet ./...` clean
- `go test ./... -count=1` → **106 PASS, 2 SKIP, 0 FAIL**
- `docker compose up -d --build` → api + postgres 17, ready in
  under 5 seconds, all documented endpoints verified end-to-end
  against a real PostgreSQL with data surviving an API restart
  (see commit `312ed10`'s "fix: make docker stack actually
  buildable and runnable" message for the three bugs that surfaced
  during the first end-to-end run)

## Open work / known limitations

- **Per-process state.** The dead-letter list (jobs) is
  process-local. The blacklist, rate limiter, idempotency store,
  job queue, reset-token store, and outbox are now durable when
  the matching backing service is configured: blacklist / rate
  limiter / idempotency / job queue / reset tokens switch to
  Redis when `REDIS_URL` is set; the outbox switches to a
  database-backed implementation (PostgreSQL, using the same
  `*sql.DB` as the user store) when `DATABASE_URL` is set. The
  interface for the remaining in-memory state is intentionally
  small so a Redis-backed or RabbitMQ-backed implementation can
  drop in.
- **In-memory `Publisher`.** As of the most recent commit a
  `KafkaPublisher` is wired in. Set `KAFKA_BROKERS=host:9092` and
  events go to Kafka instead of the log. Leave it empty to keep the
  starter behaviour. `internal/events/kafka.go` is the production
  adapter; the switch is in `cmd/api/main.go` buildKafkaPublisher.
- **In-memory rate limiter.** As of the most recent commit a Redis-backed
  `RedisLimiter` is wired in. When `REDIS_URL` is set, both the
  global and the auth limiter share buckets across replicas. The
  Lua-script-based update in `internal/ratelimit/limiter.go`
  (`RedisLimiter.Allow`) keeps the read-modify-write atomic. The
  in-memory limiter remains the default for single-instance
  development.
- **In-memory token blacklist.** As of the most recent commit a
  Redis-backed `RedisBlacklist` is wired in. When `REDIS_URL` is
  set, revoked jti values live in `auth:blacklist:<jti>` keys with
  an EX TTL matching the access-token lifetime. The set can no
  longer grow without bound (TTL reclaims entries), and revocation
  is consistent across replicas. The in-memory fallback is
  retained for single-instance development. Implementation in
  `internal/auth/blacklist.go`.
- **In-memory idempotency store.** As of the most recent commit a
  Redis-backed `RedisStore` is wired in. When `REDIS_URL` is set,
  each entry lives at `idempotency:<key>` as a single Redis hash
  with the status, body, content type, request hash, and stored-at
  serialised as JSON. A per-key EXPIRE-NX sets the TTL only on
  the first save so subsequent retries do not extend the lifetime.
  The in-memory fallback is retained for single-instance
  development. Implementation in
  `internal/idempotency/redis.go`.
- **Five open Dependabot PRs** (`actions/upload-artifact`,
  `docker/setup-buildx-action`, `golangci/golangci-lint-action`,
  `docker/build-push-action`, `actions/setup-go`). The
  `dependabot-auto-merge.yml` workflow handles patch and minor
  updates. Major bumps need a human review.

## How to read the codebase

```bash
git clone https://github.com/kefyusuf/go-rest-api.git
cd go-rest-api
cat README.md
# then jump into any docs/ file from the table at the bottom
```

Or to read the code in the order it was written:

```bash
git log --oneline --reverse
git checkout layer/02-api-consistency
# read, git checkout layer/02a-config-and-logger, etc.
```

The dependency chain is documented in the `How the layers chain`
section of `README.md` and in `docs/LAYERS.md`.

## Environment

- Working tree clean on `main`
- No uncommitted changes
- `origin/main` is in sync with the local `main`
- All 13 `origin/layer/*` branches are present on the remote for
  reference

## Where to pick up

If you are a maintainer coming back to this repository:

1. The dead-letter list is process-local. For multi-replica
   deployments back it with Redis or the same database the outbox
   uses; the drop-in follows the same pattern as the outbox.
2. The `welcome_email` job is a mock. Replace the registration in
   `cmd/api/main.go` with a real handler (SendGrid / Resend / SES)
   once you have credentials.
3. CI is wired but the `ci.yml` only exists on the layer branches
   and on the most recent `main` commits. If you rebase an old
   layer branch, the workflow will be missing — re-add it from a
   later commit if you push that branch.
4. The Kubernetes manifests and Helm chart in `deploy/` have not
   been exercised against a real cluster. The compose stack on
   the local dev machine is the only end-to-end verification today.
5. The Swagger output is OpenAPI 2.0. A future layer could
   switch to `oapi-codegen` or `kin-openapi` for OpenAPI 3.1.

## Files of interest

- `cmd/api/main.go` — process bootstrap, wiring of every layer
- `internal/server/server.go` — the route table and middleware chain
- `internal/config/config.go` — every env var the process reads
- `internal/auth/jwt.go` — the token issuer, including the
  access/refresh audience split
- `internal/store/cached_user_store.go` — the read-through cache
  wrapper
- `internal/jobs/registry.go` — the worker pool with retry and DLQ
- `internal/events/outbox.go` — the outbox + dispatcher pattern
- `internal/idempotency/memory.go` — the Idempotency-Key store
- `internal/ratelimit/limiter.go` — the token-bucket limiter
- `deploy/k8s/20-deployment.yaml` — the hardened pod spec
- `deploy/helm/go-rest-api/values.yaml` — the chart defaults

End of session.

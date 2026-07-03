# Go REST API Starter

A production-shaped Go REST API built incrementally through 12 learning
layers. The repository starts as a 200-line `net/http` skeleton and grows,
layer by layer, into a backend with JWT auth, caching, rate limiting,
background jobs, an event outbox, Prometheus metrics, CI/CD, and Kubernetes
manifests. Every layer lands on its own branch with its own pull request,
its own test suite, and its own entry in the table below.

The goal is for a reader to learn one production concern at a time without
losing the working state of the previous one.

## What is in `main`?

`main` is the fully merged baseline. It includes every layer from the
table below; the per-layer branches exist only as a learning history.

- Standard `net/http` server with timeouts, graceful shutdown, recovery
- Standardised error envelope with `code`, `message`, and `details`
- JWT auth: `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`,
  `POST /auth/logout`, `POST /auth/forgot-password`, `POST /auth/reset-password`,
  `GET /me`
- bcrypt password hashing; Argon2 is a drop-in if you need it
- Read-through user cache (in-process map; switch to Redis via `REDIS_URL`)
- Per-IP rate limiting (global + auth limiters) with a `Retry-After` header
- Security headers (`X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`) and opt-in CORS
- `Idempotency-Key` support on `POST /users` and `POST /auth/register`
- Background-job queue with exponential-backoff retry and a dead-letter list
- Outbox + dispatcher for event publishing (in-memory publisher; Kafka
  is a drop-in via the `Publisher` interface)
- Prometheus `/metrics`, `/health/live`, `/health/ready`
- GitHub Actions CI (test + lint + build) and a release workflow that
  publishes a multi-stage Docker image to GHCR on `v*.*.*` tags
- Kubernetes manifests and a Helm chart under `deploy/`
- HTTP-level integration tests against both the in-memory store and
  PostgreSQL

Quick start:

```bash
cp .env.example .env
docker compose up -d --build
curl http://localhost:8080/health/live
```

## Learning Layers

Every layer was developed on its own branch and merged into `main`. The
branches are still in the repository for anyone who wants to read the
code in the order it was written. The table below links each branch to
its PR so you can read the conversation, the review, and the
side-by-side diff.

| # | Layer | Branch | Pull Request | What it adds | Status |
|---|---|---|---|---|---|
| 0 | Baseline | `main` | — | Health, users CRUD, Swagger, Postgres/memory store, Docker, integration tests, request logging | stable |
| 1 | API consistency | [`layer/02-api-consistency`](../../tree/layer/02-api-consistency) | [#1](../../pull/1) | Standard error envelope, field-level validation details, content-type enforcement, `204 No Content` for delete, list envelope with `data/meta`, `nextCursor` | stable |
| 2 | Config + logging | [`layer/02a-config-and-logger`](../../tree/layer/02a-config-and-logger) | [#2](../../pull/2) | `internal/config` (env-driven, validated), `log/slog` JSON logger, `X-Request-Id` middleware | stable |
| 3 | Server hardening | [`layer/02b-server-hardening`](../../tree/layer/02b-server-hardening) | [#3](../../pull/3) | `http.Server` timeouts, graceful shutdown on `SIGINT`/`SIGTERM`, panic recovery middleware, body-size limit, `/health/live` vs default `/health` | stable |
| 4 | JWT auth | [`layer/03-auth-jwt`](../../tree/layer/03-auth-jwt) | [#4](../../pull/4) | `POST /auth/login`, JWT issuance (HS256, 32-byte secret, short-lived), `GET /me`, `RequireAuth` middleware, bcrypt password hashing | stable |
| 5 | Auth session | [`layer/04-auth-session`](../../tree/layer/04-auth-session) | [#5](../../pull/5) | `POST /auth/register`, `POST /auth/refresh` with rotation, `POST /auth/logout` with jti blacklist, `POST /auth/forgot-password` and `POST /auth/reset-password` | stable |
| 6 | Observability | [`layer/05-observability`](../../tree/layer/05-observability) | [#6](../../pull/6) | Prometheus metrics (`http_requests_total`, `http_request_duration_seconds`, `http_in_flight_requests`), `/health/live`, `/health/ready` with Postgres ping, slog access log | stable |
| 7 | User cache | [`layer/06-performance-cache`](../../tree/layer/06-performance-cache) | [#7](../../pull/7) | `internal/cache` with `MemoryCache` and `RedisCache`, `CachedUserStore` wrapping `GetByID` with read-through + write invalidation, configurable TTL | stable |
| 8 | API hardening | [`layer/07-api-hardening`](../../tree/layer/07-api-hardening) | [#8](../../pull/8) | Per-IP rate limiting (token bucket, global + auth), opt-in CORS, security headers middleware, `Allow` header on 405 | stable |
| 9 | Idempotency | [`layer/08-idempotency-consistency`](../../tree/layer/08-idempotency-consistency) | [#9](../../pull/9) | `Idempotency-Key` support on `POST /users` and `POST /auth/register`, replay with `Idempotent-Replay: true`, 409 on same-key-different-body | stable |
| 10 | Background jobs | [`layer/09-async-processing`](../../tree/layer/09-async-processing) | [#10](../../pull/10) | In-process job queue with worker pool, exponential backoff (1s, 2s, 4s, max 1 min), max-retry, dead-letter list, single-use `RunAfter` semantics | stable |
| 11 | Event publishing | [`layer/10-event-driven`](../../tree/layer/10-event-driven) | [#11](../../pull/11) | Outbox + dispatcher + `Publisher` interface, `user.created` event published with key=user id, Kafka is a drop-in adapter | stable |
| 12 | CI/CD | [`layer/11-delivery-platform`](../../tree/layer/11-delivery-platform) | [#12](../../pull/12) | `.github/workflows/{ci,release,dependabot-auto-merge}.yml`, `Makefile` mirroring CI, `golangci.yml` curated ruleset, Dependabot config | stable |
| 13 | K8s + Helm | [`layer/12-k8s-cloud`](../../tree/layer/12-k8s-cloud) | [#13](../../pull/13) | `deploy/k8s/` Kustomize bundle (Namespace, ConfigMap, Secret, Deployment with hardened security context, Service, HPA, Ingress + NetworkPolicy, PDB, ServiceMonitor) and `deploy/helm/go-rest-api/` Helm chart | stable |

Every layer in the table is already merged into `main`. The per-layer
branches are kept in the repository so a reader can `git checkout` any
one of them and read the code in the order it was written. The
branch-only history is also the audit trail for the design decisions
captured in the per-layer PR conversations.

### How the layers chain

```
main
└─ layer/02-api-consistency  (#1)  error contract, 204 delete, data/meta envelope
   ├─ layer/02a-config-and-logger  (#2)  config + slog + X-Request-Id
   └─ layer/02b-server-hardening   (#3)  timeouts + graceful + recovery + body limit
      └─ layer/03-auth-jwt         (#4)  login + /me + JWT
         └─ layer/04-auth-session  (#5)  register + refresh + logout + forgot/reset
   ├─ layer/05-observability       (#6)  Prometheus + /health/live + /health/ready
   ├─ layer/06-performance-cache  (#7)  user cache (memory + Redis)
   ├─ layer/07-api-hardening      (#8)  rate limit + CORS + security headers
   │  └─ layer/08-idempotency-consistency  (#9)  Idempotency-Key
   │     └─ layer/09-async-processing  (#10)  background jobs + retry + DLQ
   │        └─ layer/10-event-driven  (#11)  outbox + dispatcher + publisher
   └─ layer/11-delivery-platform  (#12)  CI + release + Makefile + Dependabot
      └─ layer/12-k8s-cloud        (#13)  Kustomize bundle + Helm chart
```

`02a` and `02b` are independent refinements of `02`. From `03` onward
every layer assumes `02` and the auth foundation, but a layer does not
strictly require its predecessor's *changes* — the auth middleware
in `03` uses config that `02a` introduces, the metrics middleware in
`05` uses the slog logger from `02a`, and so on.

### Reading order

If you are new to the codebase, read in this order:

1. **`main`** (current branch) — start here. The README below
   documents the full surface.
2. **`layer/02-api-consistency`** — the error envelope and the
   contract cleanup. PR #1 has the design conversation.
3. **`layer/02a-config-and-logger`** and **`layer/02b-server-hardening`**
   in either order — together they make the server production-safe.
4. **`layer/03-auth-jwt`** — the first security boundary. Once you
   have JWT issuance, every later layer can decide whether a route
   is public, authenticated, or admin.
5. **`layer/04-auth-session`** — the session lifecycle.
6. **`layer/05-observability`** through **`layer/10-event-driven`**
   in any order — they are independent. The diagram above shows
   the dependency chain if you want to be strict.
7. **`layer/11-delivery-platform`** and **`layer/12-k8s-cloud`**
   last — they cover the pipeline that ships the code you have just
   read.

## Documentation map

The detailed content of this README has been split into focused documents
under `docs/`. The README stays short; everything else lives in a file
that does one job well.

| Topic | File |
| --- | --- |
| Every layer in the repository with branch + PR links, dependency chain, and reading order | [docs/LAYERS.md](docs/LAYERS.md) |
| Architecture, per-layer design notes, error contract, auth model, cache, jobs, events, idempotency | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Tech decisions, CI/CD pipelines, Kubernetes manifests, Helm chart, env variables, Docker network, start-up commands | [docs/OPERATIONS.md](docs/OPERATIONS.md) |
| Branch lifecycle, naming, contribution rules, project structure, first contribution checklist | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |
| Endpoint reference, hardening, idempotency, jobs, events, caching details, curl examples, swagger, tests | [docs/REFERENCE.md](docs/REFERENCE.md) |

Each `docs/` file is a normal markdown document that lives next to the
README. They are referenced from the GitHub UI tree view and from the
README's documentation map, so the full surface of the project is
still reachable from the front page — just no longer inline.

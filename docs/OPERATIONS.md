# Operations

> This document covers everything that touches the outside world: CI/CD
> pipelines, Kubernetes manifests, the Helm chart, environment variables,
> the Docker network, and the day-to-day commands. Read it when you
> deploy, when you change runtime config, or when you wire up the pipeline.

---

## CI/CD and delivery

`.github/workflows/` carries three pipelines:

- `ci.yml` runs on every push to `main` and on every pull request.
  Three jobs run in sequence: `test` (go build, go vet, `go test -race`
  with a Postgres service container, coverage summary), `lint`
  (golangci-lint with the curated ruleset in `.golangci.yml`), and
  `build` (final binary, uploaded as a build artifact).
- `release.yml` runs on `v*.*.*` tags and on manual dispatch. It
  builds a multi-stage Docker image and pushes it to GHCR with the
  tag as the version and `latest` for dispatch from `main`.
- `dependabot-auto-merge.yml` auto-merges Dependabot PRs for
  patch and minor updates so the project stays current without
  manual review on routine bumps.

Dependabot itself is configured in `.github/dependabot.yml` to scan
`go.mod` and GitHub Actions weekly.

The `Makefile` mirrors the CI surface for local use:

```bash
make test       # go test ./...
make test-race  # go test -race ./...
make vet        # go vet ./...
make lint       # golangci-lint run
make build      # compile to bin/api
make cover      # coverage summary
make swagger    # regenerate docs/
```

The `Dockerfile` is a multi-stage build. The `builder` stage
compiles the binary with `-trimpath` and `-ldflags="-s -w"` for
smaller images; the final stage is a non-root `alpine:3.22` with
the binary and a `HEALTHCHECK` that hits `/health/live`.

## Kubernetes and cloud

`deploy/k8s/` carries a Kustomize bundle:

```bash
kubectl apply -k deploy/k8s
```

It creates a Namespace, a ConfigMap with the non-secret runtime
config, a Secret with placeholders for `JWTSecret`, `DATABASE_URL`,
and `REDIS_URL`, a Deployment (2 replicas, non-root, hardened
security context, startup/liveness/readiness probes against
`/health/live` and `/health/ready`), a Service, a
HorizontalPodAutoscaler (2-10 replicas on CPU and memory), an
Ingress with cert-manager, a NetworkPolicy (default-deny with
allow-rules for the ingress controller and the data pods plus
DNS), a PodDisruptionBudget, and a ServiceMonitor for Prometheus.
See `deploy/k8s/README.md` for the per-file inventory and the
override pattern.

`deploy/helm/go-rest-api/` carries the same set of resources as a
minimal Helm chart. Render the chart with `helm template` and apply
the output, or install directly with `helm install`. Values mirror
the Kustomize fields 1:1; the chart template is intentionally
small so it is easy to read and override.

Production checklist:

- Replace the placeholder `JWTSecret` in the secret with
  `openssl rand -base64 48`
- Replace the placeholder `DATABASE_URL` and `REDIS_URL` with
  real connection strings, including `?sslmode=require` for
  Postgres and `rediss://` for Redis when TLS is in use
- Pin the image tag in `kustomization.yaml` to a release version;
  the default `v0.1.0` is a placeholder
- Apply the NetworkPolicy before exposing the service publicly;
  until then the cluster default (allow-all) applies
- Run a Postgres operator (e.g. CloudNativePG) and a Redis operator
  (e.g. Redis Operator) so the data pods match the labels the
  NetworkPolicy selects

---

## Start in 1 minute

To run the project:

Windows:

```bash
scripts\dev-up.bat
```

Unix-like environments:

```bash
sh scripts/dev-up.sh
```

Open in a browser:

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/index.html`

To stop:

Windows:

```bash
scripts\dev-down.bat
```

Unix-like environments:

```bash
sh scripts/dev-down.sh
```

## Common commands

### Start the application

Windows:

```bash
scripts\dev-up.bat
```

Unix-like environments:

```bash
sh scripts/dev-up.sh
```

### Follow the logs

```bash
docker compose logs -f
```

### Optional: hot reload with air

If you want the API to rebuild automatically inside the container on
code changes, use the following scripts.

Windows:

```bash
scripts\dev-up-air.bat
```

Unix-like environments:

```bash
sh scripts/dev-up-air.sh
```

Or run the same command directly:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```

In this mode, `air` is installed inside the container, watches file
changes, and restarts the API.

Stop it with:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml down
```

When to use it:

- You change handler or store code frequently
- You do not want to wait for an image rebuild on every change

When you might skip it:

- You are learning Docker for the first time
- You want a simpler startup flow

The main recommendation is still the normal `scripts\dev-up.bat` or
`sh scripts/dev-up.sh` flow. The `air` mode is entirely optional.

### Check container status

```bash
docker compose ps
```

If the `api` service shows as `healthy`, the application is running and
the `/health` endpoint is responding successfully.

### Start the test database

Windows:

```bash
scripts\test-db-up.bat
```

Unix-like environments:

```bash
sh scripts/test-db-up.sh
```

### Stop the test database

Windows:

```bash
scripts\test-db-down.bat
```

Unix-like environments:

```bash
sh scripts/test-db-down.sh
```

---

## Environment variables

| Variable | Where it is used | Example value | Description |
|---|---|---|---|
| `PORT` | inside the API container | `8080` | Port the API listens on |
| `DATABASE_URL` | API application and PostgreSQL integration tests | `postgres://postgres:postgres@postgres:5432/go_lang?sslmode=disable` | PostgreSQL connection URL for the application |
| `JWTSecret` | API application | at least 32 random bytes | Secret used to sign JWT access tokens. Required. |
| `ACCESS_TOKEN_TTL` | API application | `15m` | Lifetime of an access token |
| `REFRESH_TOKEN_TTL` | API application | `168h` | Lifetime of a refresh token |
| `BcryptCost` | API application | `10` | bcrypt cost factor used when hashing passwords |
| `APP_ENV` | API application | `development` | Environment name; emitted as the JWT `iss` claim |
| `READ_HEADER_TIMEOUT` | API application | `5s` | HTTP read header timeout |
| `READ_TIMEOUT` | API application | `15s` | HTTP read timeout |
| `WRITE_TIMEOUT` | API application | `15s` | HTTP write timeout |
| `IDLE_TIMEOUT` | API application | `60s` | HTTP idle timeout |
| `MAX_HEADER_BYTES` | API application | `1048576` | Maximum header size in bytes |
| `MAX_BODY_BYTES` | API application | `1048576` | Maximum request body size in bytes |
| `SHUTDOWN_TIMEOUT` | API application | `15s` | Maximum time to drain in-flight requests on shutdown |
| `REDIS_URL` | API application | empty | When set, the user cache uses Redis. Empty falls back to an in-process map. |
| `USER_CACHE_TTL` | API application | `5m` | TTL for cached user payloads read by `/users/{id}` and `/me`. |
| `RATE_LIMIT_PER_SECOND` | API application | `20` | Sustained rate of the global token-bucket limiter (requests per second). |
| `RATE_LIMIT_BURST` | API application | `40` | Burst size of the global limiter. |
| `AUTH_RATE_LIMIT_PER_SECOND` | API application | `5` | Sustained rate of the auth-endpoint limiter. |
| `AUTH_RATE_LIMIT_BURST` | API application | `10` | Burst size of the auth limiter. |
| `CORS_ALLOWED_ORIGINS` | API application | empty | Comma-separated list of origins allowed by CORS. Empty disables cross-origin browser access. |
| `IDEMPOTENCY_TTL` | API application | `24h` | How long a successful `Idempotency-Key` response is replayed. |

---

## Docker network logic

| Case | Hostname | Port | Description |
|---|---|---|---|
| API container -> main database | `postgres` | `5432` | Uses the Compose service name inside the Compose network |
| Host machine -> main database | `localhost` | `5432` | Used to connect to the database directly from the host |
| Host machine -> test database | `localhost` | `5433` | Used by the separate `docker-compose.test.yml` flow |

In short, use the Compose service name instead of `localhost` when
connecting from one container to another.

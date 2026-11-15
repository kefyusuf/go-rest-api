# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
the project versions with SemVer tags (`v*.*.*`).

## [Unreleased]

## [1.2.0] – 2026-11-15

### Added
- Application routes are versioned under `/v1`; `/health`, `/health/live`,
  `/health/ready`, `/metrics` and `/swagger` stay unversioned
- Keyset pagination on `GET /v1/users` (`?limit` 1-100, default 20;
  `?cursor` = last id; `meta.nextCursor` on a full page)
- gitleaks secrets scan in CI (full history, path allowlist for
  `*_test.go` and `*.example` fixtures in `.gitleaks.toml`)
- Community files: `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`,
  `CODEOWNERS`
- Helm: one template per resource (HPA, NetworkPolicy, ServiceMonitor
  moved out of `service.yaml`/`ingress.yaml`), `automountServiceAccountToken: false`,
  `secrets.existingSecret` support
- Password policy: minimum 8 characters, maximum 72 bytes (bcrypt limit)
  enforced on register, user create, and password reset

### Changed
- **Breaking:** module renamed `go-lang` → `github.com/kefyusuf/go-rest-api`;
  every import path changes
- **Breaking:** env vars `JWTSecret`/`BcryptCost` renamed to
  `JWT_SECRET`/`BCRYPT_COST`
- Password policy literal in validation details is a field-level detail;
  the shared error envelope is unchanged

### Fixed
- Global and auth rate limiters shared one Redis bucket per IP when
  `REDIS_URL` was set; the auth limiter now namespaces its keys
  (`AuthClientIPKey`)
- `BCRYPT_COST` sent by the Helm configmap was silently ignored because
  the app read `BcryptCost`
- Dead copy-paste block removed from the api bootstrap; the response
  encoder and outbox enqueue failures are now logged instead of
  swallowed
- gofmt drift cleaned across the tree; `gofmt` added to the lint gates

## [1.1.1] – 2026-10

- Demo image pinned to 1.1.1; `/health/ready` reports `APP_VERSION`

## [1.1.0] – 2026-10

- `APP_VERSION` environment variable reported by `/health/ready`
- Render deployment removed; the project runs locally with Docker only

## [1.0.0] – 2026-09

- Initial 15-layer baseline: health, users CRUD, JWT auth with rotation
  and revocation, rate limiting, CORS, idempotency, background jobs,
  outbox + events, Prometheus metrics, CI/CD, k8s manifests + Helm chart

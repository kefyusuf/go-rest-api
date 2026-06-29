# Learning Layers

> **Source of truth:** this document covers every layer in the repository,
> from the baseline `main` to the Kubernetes deployment. Each layer
> lives on its own branch and was merged into `main` via a pull
> request. The table below links to the branch, the pull request, and
> a short description of what the layer added.

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

### `layer/02-api-consistency` scope

This layer tightens existing endpoint behavior to be more predictable and
more standard, rather than adding new features.

Recommended concrete rules for this branch:

- **Consistent error body:** every error response must follow a single shape.

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email is required",
    "details": {
      "email": ["required"]
    }
  }
}
```

- **HTTP status rules:**
  - `GET /health` -> `200`
  - `GET /users` -> `200`
  - `GET /users/{id}` -> `200` when present, `404` when missing
  - `POST /users` -> `201` on success
  - `PUT /users/{id}` -> `200` on success, `404` when missing
  - `DELETE /users/{id}` -> `204` on success, `404` when missing
  - validation errors -> `400`
  - duplicate email -> `409`

- **List endpoint standard:** when `GET /users` later gains pagination, the
  response shape must be ready for it, not a flat array.

```json
{
  "data": [],
  "meta": {
    "nextCursor": null,
    "limit": 20
  }
}
```

- **Path parameter rule:** invalid path parameters such as `GET /users/abc`
  must return `400`, not `500`.

- **Empty / malformed body rule:** requests to `POST /users` and
  `PUT /users/{id}` with unparseable JSON must return `400` with a clear
  message.

- **Content-Type expectation:** endpoints that accept a body must require
  `Content-Type: application/json`. Pick either `400` or `415` for
  mismatches and apply it consistently everywhere.

- **Response shape consistency:** create / update / get endpoints must
  return the same user representation. The same resource must not be
  exposed under different field names in different endpoints.

- **Request-id preparation:** even before full tracing, prepare the
  response header and log fields to carry a request-id.

- **Swagger alignment:** handler behavior and Swagger documentation must
  stay aligned. Status codes and error response annotations must not
  drift from the real behavior.

Good candidate tasks for this layer:

- Add a shared `ErrorResponse` model
- Standardize validation error details
- Move list endpoints to a `data/meta` shape
- Bind `404`, `409`, `400` responses in every handler to the same contract
- Align Swagger examples with the real response shape

#### Suggested issue / checklist

**P0 — must be done first**

- [ ] [P0] Add a shared `ErrorResponse` model
- [ ] [P0] Create shared error helpers under `internal/response`
- [ ] [P0] Make `POST /users` return `201 Created` on success
- [ ] [P0] Make `DELETE /users/{id}` return `204 No Content` on success
- [ ] [P0] Make `GET /users/{id}` and `PUT /users/{id}` return a clean `404` for not found
- [ ] [P0] Standardize duplicate email create / update scenarios as a consistent `409 Conflict`
- [ ] [P0] Standardize `400 Bad Request` behavior for empty body and malformed JSON
- [ ] [P0] Add `400 Bad Request` behavior for invalid path parameters
- [ ] [P0] Update integration tests to match the new response contract and status codes

**P1 — right after the core contract**

- [ ] [P1] Apply the `Content-Type: application/json` requirement consistently on body endpoints
- [ ] [P1] Bind the validation `details` field to the contract
- [ ] [P1] Align Swagger annotations and example responses with the real behavior
- [ ] [P1] Prepare the `data/meta` response shape for `GET /users`

**P2 — preparation and improvements**

- [ ] [P2] Add middleware or helper infrastructure to carry a request-id

#### Acceptance criteria (PR template)

Pull requests opened for this layer should answer the following as clearly as possible.

**Scope**

- [ ] This change stays strictly within the `layer/02-api-consistency` goals
- [ ] No other layer concerns (auth, cache, queue, deployment) are pulled into this PR

**HTTP behavior**

- [ ] `POST /users` returns `201 Created` on success
- [ ] `DELETE /users/{id}` returns `204 No Content` on success
- [ ] `GET /users/{id}` and `PUT /users/{id}` return `404` for not found
- [ ] Duplicate email scenarios return `409 Conflict`
- [ ] Malformed JSON and invalid path parameters return `400 Bad Request`

**Response contract**

- [ ] Error responses follow the shared `ErrorResponse` contract
- [ ] Validation errors include the `details` field
- [ ] The same resource uses the same field names across endpoints
- [ ] If the list response changed, the `data/meta` shape is applied consistently

**Documentation and tests**

- [ ] Swagger annotations match the real behavior
- [ ] README examples are up to date
- [ ] Integration tests verify the new status codes and payload contract
- [ ] At least one failure-scenario test is added for new behavior

**Review notes**

- [ ] The PR description clearly lists which response contract changes were made
- [ ] Any backward-compatibility risk is noted

#### Gap analysis — current code vs target contract

The following items summarize the gap between the current code base and
the `layer/02-api-consistency` goals.

**Already aligned**

- `POST /users` returns `201 Created` on success
- Duplicate email flows return `409 Conflict`
- Invalid user id path parameter returns `400 Bad Request`
- Not found flows return `404 Not Found`
- Basic integration tests exist for users CRUD

**Missing or out of line**

- `ErrorResponse` still carries a single string field; the target shape
  needs an object with `code`, `message`, and optional `details`
- There is no shared error helper layer yet; handlers build error payloads
  directly
- `DELETE /users/{id}` currently returns `200 OK` with a message body; the
  target behavior is `204 No Content`
- `GET /users` returns a flat array; the target is the `data/meta` shape
- Validation errors do not return field-level `details`
- There is no `Content-Type` enforcement on body endpoints
- Request-id preparation is not in place yet
- Swagger annotations and integration tests follow the current behavior;
  they will need to be updated alongside the new contract

**Priority summary**

- **P0:** error contract, shared helpers, delete semantics, test updates
- **P1:** validation `details`, list `data/meta`, content-type strategy, Swagger alignment
- **P2:** request-id preparation

This analysis confirms that the `layer/02-api-consistency` branch is not a
small refactor but a controlled API contract cleanup.

#### Phase 1 implemented changes

The first small change set implemented the following:

- `ErrorResponse` moved from a string field to an object-based error contract
- `ErrorDetail` and error code constants were added
- Shared error helpers were added under `internal/response`
- User handlers were updated to use the new helpers
- `DELETE /users/{id}` behavior changed from `200 + message` to `204 No Content`
- In-memory store integration tests were updated to the new error shape
  and delete behavior
- PostgreSQL integration tests were updated to the new error shape and
  delete behavior
- The generated Swagger docs were regenerated and aligned with the new
  contract
- Phase 1 changes were verified with `go test ./...`

Pieces aligned to the same contract at the end of Phase 1:

- Model layer
- Response helper layer
- User handlers
- Route-level method-not-allowed responses
- Integration tests
- PostgreSQL tests
- Swagger output

Items deliberately left out of Phase 1:

- Field-level validation `details`
- `data/meta` response shape for `GET /users`
- `Content-Type` enforcement strategy
- Request-id preparation

#### Phase 2A implemented changes

Validation behavior was made more granular in the second small step.

Implemented changes:

- A shared `validateUserInput` helper was added for `CreateUser` and
  `UpdateUser`
- Validation errors now return field-level `details` in addition to a
  general message
- The validation message changed from `name and email are required` to
  the more contract-oriented `validation failed`
- Tests were updated to verify `details.name` and `details.email`
- Phase 2A changes were verified with `go test ./...`

Example structure expected at the end of Phase 2A:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": {
      "name": ["required"],
      "email": ["required"]
    }
  }
}
```

#### Phase 2B implemented changes

The `Content-Type` contract was also applied to body endpoints.

Implemented changes:

- The `UNSUPPORTED_MEDIA_TYPE` error code was added
- An `UnsupportedMediaType` helper was added under `internal/response`
- `CreateUser` and `UpdateUser` now require `application/json`
- The API returns `415 Unsupported Media Type` for the wrong content type
- Tests were added for unsupported media type on create and update
- Phase 2B changes were verified with `go test ./...`

Example structure expected at the end of Phase 2B:

```json
{
  "error": {
    "code": "UNSUPPORTED_MEDIA_TYPE",
    "message": "Content-Type must be application/json"
  }
}
```

Open items after Phase 2:

- Sharper separation between malformed JSON and missing field validation
- Request-id preparation
- Real cursor / limit parsing and pagination behavior

#### Phase 3 implemented changes

The list endpoint contract moved to the `data/meta` envelope in the third
small step.

Implemented changes:

- `ListUsersMeta` and `ListUsersResponse` models were added
- `GET /users` now returns `{ data, meta }` instead of a flat `[]User`
- `meta.nextCursor` is fixed to `null` for now; `meta.limit` is fixed to `20`
- In-memory store integration tests were updated to verify
  `listResponse.Data` and `listResponse.Meta`
- The generated Swagger docs were regenerated and the `/users` GET
  response was aligned with `model.ListUsersResponse`
- The `GET /users` example response in the README was updated to the new
  `data/meta` shape
- The list response contract was verified with `go test ./...`

Example structure expected at the end of Phase 3:

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

Open items after Phase 3:

- Real cursor / limit parsing and pagination behavior
- Filtering / sorting contract
- Sharper separation between malformed JSON and missing field validation
- Request-id preparation

#### Phase 2C implemented changes

The separation between request body parse errors and semantic validation
errors was clarified in the third small validation step.

Implemented changes:

- A shared `decodeJSONBody` helper was added for `CreateUser` and
  `UpdateUser`
- An empty body / `io.EOF` case now returns the message
  `request body is required`
- A malformed or unparseable JSON body returns the message
  `invalid request body`
- Semantic field gaps still return `VALIDATION_ERROR` with
  `validation failed` and `details`
- Tests were added for malformed JSON on create / update and empty body
  on create
- The parse / body separation was verified with `go test ./...`

Example separation expected at the end of Phase 2C:

#### Error matrix

| Case | HTTP | Code | Message | Details |
|---|---|---|---|---|
| Malformed JSON | `400` | `BAD_REQUEST` | `invalid request body` | none |
| Empty body | `400` | `BAD_REQUEST` | `request body is required` | none |
| Semantic validation failure | `400` | `VALIDATION_ERROR` | `validation failed` | yes |
| Unsupported media type | `415` | `UNSUPPORTED_MEDIA_TYPE` | `Content-Type must be application/json` | none |

**Malformed JSON**

```json
{
  "error": {
    "code": "BAD_REQUEST",
    "message": "invalid request body"
  }
}
```

**Empty body**

```json
{
  "error": {
    "code": "BAD_REQUEST",
    "message": "request body is required"
  }
}
```

**Semantic validation failure**

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": {
      "name": ["required"],
      "email": ["required"]
    }
  }
}
```

Open items after Phase 2C:

- Real cursor / limit parsing and pagination behavior
- Filtering / sorting contract
- Request-id preparation

### `layer/03-auth-jwt` draft

This layer adds a controlled authentication foundation instead of loading
a full auth system into `main`. The focus is to verify the user identity,
expose protected endpoints, and standardize a basic identity endpoint
such as `/me`.

#### Suggested endpoints

- `POST /auth/login`
  - Email / password login
  - Returns an access token on success
  - Returns `401 Unauthorized` for invalid credentials

- `GET /me`
  - Returns the user behind a valid bearer token
  - Returns `401 Unauthorized` when the token is missing or invalid

Endpoints that are **not recommended** in the first auth layer:

- `POST /auth/logout`
- `POST /auth/refresh`
- `POST /auth/register`
- `POST /auth/forgot-password`
- `POST /auth/reset-password`

These belong in the next layer, `layer/04-auth-session`.

#### Suggested token scope

The first JWT layer should keep the token model small:

- **Token type:** bearer access token
- **Transport:** `Authorization: Bearer <token>`
- **Purpose:** carries the user identity and basic auth state
- **Initial scope:** short-lived access token
- **Deferred:** refresh token, revoke list, session rotation, device / session tracking

Recommended minimum claim fields:

- `sub` -> user id
- `exp` -> expiration timestamp
- `iat` -> issued-at timestamp
- optional `iss` -> token issuer

The first JWT layer must not put unnecessary domain data inside the
token. For example, do not embed:

- The full user profile
- The full permission list
- Large payloads intended for caching

#### Example request / response

Sample `POST /auth/login` request:

```json
{
  "email": "ada@example.com",
  "password": "correct-horse-battery-staple"
}
```

Successful login response:

```json
{
  "accessToken": "<jwt>",
  "tokenType": "Bearer",
  "expiresIn": 3600,
  "user": {
    "id": 1,
    "name": "Ada Lovelace",
    "email": "ada@example.com"
  }
}
```

Invalid credentials response (`401 Unauthorized`):

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "invalid email or password"
  }
}
```

Login with a missing field response (`400 Bad Request`):

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email and password are required",
    "details": {
      "email": ["required"],
      "password": ["required"]
    }
  }
}
```

Successful `GET /me` response:

```json
{
  "id": 1,
  "name": "Ada Lovelace",
  "email": "ada@example.com"
}
```

Missing or invalid token response (`401 Unauthorized`):

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "missing or invalid bearer token"
  }
}
```

#### Draft Go struct list

A draft of the request / response and auth models that keep the first
layer small:

```go
type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    AccessToken string `json:"accessToken"`
    TokenType   string `json:"tokenType"`
    ExpiresIn   int    `json:"expiresIn"`
    User        User   `json:"user"`
}

type AuthErrorDetail struct {
    Code    string              `json:"code"`
    Message string              `json:"message"`
    Details map[string][]string `json:"details,omitempty"`
}

type AuthErrorResponse struct {
    Error AuthErrorDetail `json:"error"`
}

type TokenClaims struct {
    Subject   string `json:"sub"`
    IssuedAt  int64  `json:"iat"`
    ExpiresAt int64  `json:"exp"`
    Issuer    string `json:"iss,omitempty"`
}
```

Notes:

- `LoginRequest` must stay as small as possible. `email` and `password`
  are enough for the first layer
- `LoginResponse.User` can reuse the existing `model.User` shape
- `AuthErrorResponse` must align with the shared `ErrorResponse` contract
  introduced by `layer/02-api-consistency`
- `TokenClaims` must not carry role lists, profile data, or large domain
  payloads in the first layer

#### Middleware and behavior rules

- Add an auth middleware for protected routes
- Token parse / verify failures must map to a single `401` response
- The `GET /me` response shape must stay compatible with the existing
  user resource model
- Swagger documentation must clearly mark auth-required routes
- The raw token must never be written to logs

#### Swagger security scheme and annotation draft

The Swagger side must make the bearer auth explicit for this layer.

Recommended global security scheme:

```go
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT bearer token. Example: "Bearer <token>"
```

Example annotation for `POST /auth/login`:

```go
// Login godoc
// @Summary Login
// @Description Login with email and password, returns an access token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body model.LoginRequest true "Login credentials"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /auth/login [post]
func (h AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    // handler body
}
```

Example annotation for `GET /me`:

```go
// Me godoc
// @Summary Current user
// @Description Returns the user behind a valid bearer token
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.User
// @Failure 401 {object} model.ErrorResponse
// @Router /me [get]
func (h AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
    // handler body
}
```

Notes:

- Protected endpoints like `GET /me` must include `@Security BearerAuth`
- `POST /auth/login` is public and does not need a security annotation
- Swagger example payloads must stay aligned with the login and
  unauthorized response shapes defined in this README

#### Test scenarios and edge cases

**Basic test scenarios**

- [ ] `POST /auth/login` with the right email / password returns `200`
- [ ] The successful login response includes `accessToken`, `tokenType`,
      `expiresIn`, and `user`
- [ ] A login attempt with a wrong password returns `401 Unauthorized`
- [ ] A login attempt with an unknown user returns `401 Unauthorized`
- [ ] `GET /me` with a valid bearer token returns the user
- [ ] `GET /me` with a missing bearer token returns `401 Unauthorized`
- [ ] `GET /me` with an invalid bearer token returns `401 Unauthorized`

**Validation and parse scenarios**

- [ ] An empty body to `POST /auth/login` returns `400 Bad Request`
- [ ] A malformed JSON body returns `400 Bad Request`
- [ ] A missing `email` returns a validation error
- [ ] A missing `password` returns a validation error
- [ ] A wrong `Content-Type` returns a consistent `400` or `415`
      depending on the chosen strategy

**Token edge cases**

- [ ] An expired token returns `401 Unauthorized`
- [ ] A token with a broken signature returns `401 Unauthorized`
- [ ] A token without the `Bearer` prefix is rejected
- [ ] A malformed `Authorization` header is rejected safely
- [ ] When the `sub` inside the token cannot be resolved, the request
      ends with `401`

**Security and behavior notes**

- [ ] Error messages must not leak whether the user exists
- [ ] Access tokens must never be written to logs in raw form
- [ ] The `GET /me` response must stay compatible with the `user` shape
      returned by login
- [ ] Swagger auth examples must stay aligned with the real behavior

#### Good candidate tasks for this layer

- Add a password hash verification helper
- Add the `POST /auth/login` handler
- Add JWT issue and verify helpers
- Add the auth middleware and protect `/me`
- Add the Swagger security annotations
- Add auth failure tests

# Go API Starter

A clean, beginner-friendly Go REST API example that runs entirely through Docker.

## What is in this project?

- Minimal API written with Go `net/http`
- PostgreSQL connection
- One-command startup with Docker Compose
- Swagger UI documentation
- `/health` endpoint and container healthcheck
- Example CRUD endpoints for a `users` resource
- JSON request logs
- HTTP-level integration tests

## Main Scope

The `main` branch is the core API-first starting point of this repository.
The goal is to provide a small, understandable, and realistic Go REST API skeleton.

By default, `main` includes:

- A minimal API built on Go `net/http`
- A `/health` endpoint
- An example CRUD flow on the `users` resource
- Request validation and basic error handling
- Swagger / OpenAPI documentation
- A PostgreSQL store with a memory store fallback
- One-command startup with Docker Compose
- HTTP-level integration tests
- JSON request logging

`main` deliberately does **not** include:

- Authentication / authorization
- login, logout, `/me`
- JWT and refresh token flows
- Redis caching
- Rate limiting
- Prometheus metrics and distributed tracing
- Asynchronous or event-driven components such as RabbitMQ or Kafka
- Jenkins or advanced CI/CD pipeline definitions
- Kubernetes / Helm deployment layer
- Fintech-grade idempotency keys, outbox, saga, or audit trail patterns

In short, `main` is not a full enterprise platform. It is a clean API core that
can be extended with additional layers.

## Learning Layers

This repository is designed to grow on top of `main` through learning layers.
Advanced topics live in dedicated branches instead of being loaded directly
into `main`.

| Level | Branch | Main focus | Difference from `main` | Depends on | Status | Owner | Target | Risk |
|---|---|---|---|---|---|---|---|---|
| 0 | `main` | Core API-first starter | Health, users CRUD, Swagger, Postgres/memory store, Docker, integration tests, request logging | — | stable | core | current baseline | medium — scope creep; mitigation: lock the main scope through the README |
| 1 | `layer/02-api-consistency` | Tighten the API contract | Standard error shape, field-level validation details, pagination/filter/sort rules, request-id | `main` | planned | api-design | error model + contract cleanup | high — response contract breakage; mitigation: integration tests + Swagger alignment |
| 2 | `layer/03-auth-jwt` | Authentication foundation | login, JWT, auth middleware, protected routes, `/me` | `layer/02-api-consistency` | planned | auth | auth foundation | high — incorrect token handling; mitigation: short-lived tokens + auth tests |
| 3 | `layer/04-auth-session` | Mature the auth flow | logout, refresh token, revoke/blacklist, password reset / email verify | `layer/03-auth-jwt` | planned | auth | session lifecycle | high — session invalidation bugs; mitigation: cover revoke and refresh in separate tests |
| 4 | `layer/05-observability` | Observability | Prometheus metrics, health/readiness/liveness split, trace hooks | `layer/02-api-consistency` | planned | observability | metrics + tracing prep | medium — noisy or inconsistent telemetry; mitigation: fixed field names and an example log/metric contract |
| 5 | `layer/06-performance-cache` | Performance | Redis-based cache, cache invalidation, read-heavy endpoint optimizations | `layer/02-api-consistency` | planned | backend | read-path acceleration | high — stale cache; mitigation: explicit invalidation rules + read-after-write tests |
| 6 | `layer/07-api-hardening` | Hardening | rate limit, body size limits, timeout policy, CORS / security header improvements | `layer/02-api-consistency` | planned | security-reviewer | abuse protection | medium — false positives blocking clients; mitigation: calibrate limits with test fixtures |
| 7 | `layer/08-idempotency-consistency` | Stronger write semantics | Idempotency-Key, transaction boundary, retry-safe write behavior | `layer/07-api-hardening` | planned | api-design | safe write semantics | high — duplicate side effects; mitigation: same-key replay tests + transaction boundary checks |
| 8 | `layer/09-async-processing` | Background jobs | RabbitMQ / worker model, retry, dead-letter queue, async job examples | `layer/08-idempotency-consistency` | planned | backend | background jobs | high — retry and DLQ drift; mitigation: cover retry limits and poison message scenarios |
| 9 | `layer/10-event-driven` | Event-based integration | outbox pattern, Kafka producer/consumer, event contracts | `layer/09-async-processing` | planned | api | event publishing | high — event ordering and duplication; mitigation: outbox + idempotent consumer |
| 10 | `layer/11-delivery-platform` | CI/CD and delivery | GitHub Actions or Jenkins pipeline, image scan, release flow | `main` | planned | devops | delivery automation | medium — flaky pipeline behavior; mitigation: small jobs and deterministic cache |
| 11 | `layer/12-k8s-cloud` | Deployment platform | Kubernetes / Helm, secret/config separation, scaling and probe strategy | `layer/11-delivery-platform` | planned | devops | cloud deployment | high — config drift and rollout errors; mitigation: version the values files and probe settings |

Recommended usage:

- Use `main` to learn the basic API infrastructure
- Use `layer/02-api-consistency` for the contract and error model improvements
- Use `layer/03-auth-jwt` and `layer/04-auth-session` for auth flows
- Use the relevant advanced layer branches for cache, queue, event-driven, and deployment topics

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

## Tech Decisions

The technology choices in this repository follow a "what problem does it
solve?" reasoning.

### Conscious choices for `main`

- **HTTP layer:** Go `net/http` to keep things minimal and instructive
- **Database:** PostgreSQL for real-world closeness
- **Fallback store:** in-memory store for fast experimentation without a database
- **Documentation:** Swagger / OpenAPI for a visible API contract
- **Logging:** JSON request logging for basic observability
- **Container flow:** Docker Compose for a repeatable startup for everyone

### Deliberately deferred technologies for `main`

- **Redis:** for cache, rate limit, idempotency, or ephemeral state in later layers
- **RabbitMQ:** when async jobs or workers are required
- **Kafka:** after event-driven integration and outbox
- **Prometheus:** in the metrics layer
- **Jenkins:** in the delivery layer when enterprise CI/CD is needed
- **Kubernetes / Helm:** in the cloud / deployment layer

### Router and framework note

This starter is intentionally not built around `gorilla` for new projects.
The goal is to show the basic HTTP flow with minimum dependencies, so the
`net/http` line is preserved. More opinionated router or framework
choices can be evaluated in separate branches.

## Branch Lifecycle

The `Status` field in the Learning Layers table describes the maturity of
each branch. The same words are meant to mean the same thing across branches.

### `planned`

A branch that has not been implemented yet but has a defined scope and goal.

Expectations:

- The target branch name is known
- The README has a scope, owner, target, and risk note
- The implementation may not have started on the code side

### `in-progress`

A branch that is actively being worked on.

Expectations:

- The main skeleton is in place
- Some endpoints, tests, or documentation may be missing
- The contract and behavior may still change

### `stable`

A branch that is recommended for active use and has settled core behavior.

Expectations:

- The main tests pass
- README and Swagger are aligned with the behavior
- The branch scope is clear
- No major direction changes are expected

### `done`

A learning layer that is finished and kept as a reference.

Expectations:

- The target scope is closed
- The main items of the checklist or acceptance criteria are met
- The branch is no longer the place where new experiments accumulate; it
  is a finished example layer

Short summary:

- `planned` -> design is ready
- `in-progress` -> implementation is ongoing
- `stable` -> safe to read and learn from
- `done` -> finished educational / example layer

## Branch Naming and Contribution Rules

In this repository, branches are not just temporary workspaces. They are
also learning layers. Branch names and contributions should stay as
organized as possible.

### Branch naming

Recommended formats:

- `layer/NN-topic-name` -> for learning layers
- `feat/short-description` -> small feature additions on `main` or a layer
- `fix/short-description` -> bug fixes
- `docs/short-description` -> README, Swagger, or documentation updates
- `refactor/short-description` -> code simplification without behavior change

Examples:

- `layer/02-api-consistency`
- `layer/03-auth-jwt`
- `feat/user-search`
- `fix/duplicate-email-response`
- `docs/readme-branch-map`

### Branch opening rules

- `main` must always stay minimal and working
- Use the `layer/NN-...` format whenever a new learning layer is opened
- One branch must carry one main topic. Auth, cache, and queue must not
  pile up in the same branch
- An advanced layer must build on a previous layer when it truly needs it
- Prefer the problem-area name over the technology name when possible
  - example: `layer/06-performance-cache`
  - weaker example: `feat/redis`

### Contribution rules

- Contributions to `main` must not exceed the core API-first skeleton
- Topics such as auth, JWT, Redis, Kafka, RabbitMQ, Prometheus, Jenkins,
  and Kubernetes must not be added directly to `main`; they belong in
  their layer branches
- When a README update is required, document not only the technical
  change but also the branch map and the scope impact
- When a new endpoint is added, update the Swagger annotations and the
  example usage as well
- When behavior changes, tests must follow
- Contributions inside a layer branch must stay aligned with that
  layer's goal. For example, do not start auth work inside
  `layer/02-api-consistency`

### Commit and PR intent

Because this repository carries an educational and evolution map, commits
should be as small and single-focused as possible.

- good example: `docs: add learning layer status column`
- good example: `feat: standardize validation error response`
- weak example: `update stuff`

A pull request or branch description should answer the following three
questions:

1. Which layer does this change belong to?
2. Does it affect the `main` scope?
3. Does the README or Swagger need an update?

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

## Quick curl examples

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

## Docker network logic

| Case | Hostname | Port | Description |
|---|---|---|---|
| API container -> main database | `postgres` | `5432` | Uses the Compose service name inside the Compose network |
| Host machine -> main database | `localhost` | `5432` | Used to connect to the database directly from the host |
| Host machine -> test database | `localhost` | `5433` | Used by the separate `docker-compose.test.yml` flow |

In short, use the Compose service name instead of `localhost` when
connecting from one container to another.

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

## How to make your first contribution

To make a small first contribution to the project, follow this order:

1. Bring the application up
2. Check the endpoints from the Swagger UI
3. Make a small change in a handler or a response
4. Regenerate the Swagger docs if needed: `go generate ./cmd/api`
5. Run the tests: `go test ./...`

Good candidates for a first contribution:

- Add a new endpoint
- Improve an error message
- Add a new request / response model
- Add an example to the README

## Project structure

The request flow is simple:

```text
HTTP Request
   -> internal/server
   -> internal/handler
   -> internal/store through the UserStore interface
      -> memory store
      -> postgres store
   -> internal/response
   -> HTTP Response
```

This way, the handler layer works against the same interface regardless
of where the data is stored.

```text
.
├── cmd/api/main.go                 # Application entry point
├── internal/database/              # PostgreSQL connection and migration
├── internal/handler/               # HTTP handlers
├── internal/model/                 # Request / response models
├── internal/server/                # Router, middleware, tests
├── internal/store/                 # Memory and PostgreSQL stores
├── docs/                           # Generated Swagger files
├── docker-compose.yml              # Application + PostgreSQL
├── docker-compose.test.yml         # Test PostgreSQL service
├── Dockerfile                      # API image definition
└── README.md
```

## Notes

- The application is designed to run entirely through Docker.
- The main database service runs inside the Compose network under the
  name `postgres`.
- The test database is exposed on port `5433` through a separate Compose
  file.
- Request logs are written in JSON.
- Swagger documentation can be regenerated with `go generate ./cmd/api`.

## Why is this project kept minimal?

The goal is that a beginner can:

- Understand the folder layout quickly
- See the CRUD flow
- Bring the development environment up with Docker
- Learn the Swagger and testing approach

That is why unnecessary abstraction and complexity are intentionally
avoided.

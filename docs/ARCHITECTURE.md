# Architecture

> This document collects the tech decisions, per-layer design notes, and
> the design rationale for the error contract, auth model, cache, jobs,
> events, and idempotency. It is the *why* behind the codebase.

---

## Layer design notes

The following subsections are the design notes that drove each
layer. They were captured on the layer's branch at the time the
code was written and live on in the per-layer PR conversation;
they are reproduced here so a reader does not have to dig through
the PR list to understand why a particular shape was chosen.

## `layer/02-api-consistency` scope

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

## `layer/03-auth-jwt` draft

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

---

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

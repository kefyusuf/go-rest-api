# Development

> This document is for contributors. It covers the branch lifecycle, naming
> conventions, contribution rules, project structure, and the first
> contribution checklist.

---

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

---

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

---

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

---

## Notes

- The application is designed to run entirely through Docker.
- The main database service runs inside the Compose network under the
  name `postgres`.
- The test database is exposed on port `5433` through a separate Compose
  file.
- Request logs are written in JSON.
- Swagger documentation can be regenerated with `go generate ./cmd/api`.

---

## Why is this project kept minimal?

The goal is that a beginner can:

- Understand the folder layout quickly
- See the CRUD flow
- Bring the development environment up with Docker
- Learn the Swagger and testing approach

That is why unnecessary abstraction and complexity are intentionally
avoided.

# Contributing

## Ground rules

- `main` is trunk. Changes land as conventional commits
  (`fix:`, `feat:`, `docs:`, `ops:`, `chore:`, `style:`, `build(ci):`, `ci:`)
  and, for larger pieces of work, as a branch + PR in the style of the
  existing layer history.
- English everywhere in docs, commits, and code comments.

## Local gates (mirror CI)

```bash
go build ./...
go vet ./...
make lint        # golangci-lint v2.15.0; gofmt lives in the formatters section
make test        # full suite; postgres tests need DATABASE_URL
```

Formatting is enforced by the `gofmt` formatter in `.golangci.yml` —
run `gofmt -w` before committing. On Windows, the worktree is checked
out with CRLF (`core.autocrlf=true`), so `gofmt -l` flags every file;
format check against committed blobs instead
(`diff <(git show HEAD:$f) <(git show HEAD:$f | gofmt)`).

## Conventions to know before your first change

- **Module path** is `github.com/kefyusuf/go-rest-api`.
- **Routes** live under `/v1` (`/v1/users`, `/v1/auth/*`, `/v1/me`);
  `/health`, `/health/live`, `/health/ready`, `/metrics` and `/swagger`
  stay unversioned as operations endpoints.
- **Env vars** are `SCREAMING_SNAKE_CASE` (`JWT_SECRET`, `BCRYPT_COST`).
- **Secrets never enter git.** The k8s bundle expects a local
  `deploy/k8s/11-secret.yaml` copied from `11-secret.yaml.example`
  (git-ignored); the Helm chart supports `secrets.existingSecret`.
- **Test fixtures** in `*_test.go` carry synthetic literals on purpose;
  gitleaks allowlists them by path in `.gitleaks.toml`.

## Pull requests

Keep PRs single-purpose and small, and let CI be the gate: the pipeline
runs test (race + coverage + Postgres), lint, secrets-scan (gitleaks,
full history), and a build job. A PR is mergeable when all four are
green.

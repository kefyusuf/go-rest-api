# Security Policy

## Supported versions

The project follows trunk-based development: `main` is the only
supported line, and releases are cut from it as `v*.*.*` tags. Always
run the latest tag.

| Version | Supported |
| --- | --- |
| latest tag | yes |
| older tags | no |

## Reporting a vulnerability

Do **not** open a public issue for a security problem.

Report it privately through GitHub's [private vulnerability
reporting](https://github.com/kefyusuf/go-rest-api/security/advisories/new)
for this repository. Include the affected endpoint or file, the steps
to reproduce, and, if possible, a fix suggestion. You will get an
initial response within a few days, and a fix or an explicit
decision-not-to-fix will land on `main` together with a release tag.

## Scope

In scope: anything under `cmd/`, `internal/`, `deploy/` and the CI
workflows — auth and session handling, password and token flows, rate
limiting, the deployment manifests and the release pipeline.

Out of scope: the historical `layer/*` branches (they exist as a
learning record and do not receive fixes), and denial-of-service
reports that only demonstrate what the documented rate limits already
state.

# ADR-0001: Short-lived HS256 access tokens, rotating refresh tokens, and a blacklist that fails open

Status: Accepted
Date: 2026-10-15

## Context

Logout has to invalidate a token the server no longer issues and cannot recall. The stateless option —
let the access token expire on its own — leaves a signed token usable after the user walked away. A
central session store solves that but puts a synchronous dependency on the request path of every
authenticated call, and this API runs with Redis as an optional component. The decision is therefore not
"revoke or not" but "what does the API do when the thing holding revocations is unreachable?".

## Decision

- Access token: HS256, audiences `go-lang:access` / `go-lang:refresh`, default TTL **15 minutes**
  (`config.go:58`, env `ACCESS_TOKEN_TTL`); the parser rejects any other algorithm (`jwt.go:100,104`).
- Refresh token: default TTL **7 days** (`config.go:59`, env `REFRESH_TOKEN_TTL`). `POST /auth/refresh`
  rotates: an already-used refresh jti is rejected (`handler/auth.go:314`) and the old jti is revoked
  before the new pair is issued (`auth.go:320`).
- Logout revokes the access token's jti (`auth.go:267`). Revocations live in Redis under
  `auth:blacklist:<jti>` with value `"1"` and TTL equal to the token's remaining life
  (`auth/blacklist.go:97,104,117`), so the key space self-cleans; every Redis call is capped at 250 ms.
- **Fail open.** `IsRevoked` returns `false` when Redis errors (`blacklist.go:129-132`), because a
  transient outage must not lock every authenticated request out of the API. `Revoke` swallows its own
  error (`blacklist.go:117`).

## Summary (EN)

Auth uses a 15-minute HS256 access token and a 7-day refresh token that rotates on use, with the old
token's jti blacklisted in Redis for exactly as long as the token would have lived. Logout is therefore
real, not cosmetic. The trade-off is written into the code comment: if Redis is unreachable, the
blacklist read returns "not revoked" and the request proceeds, so a logged-out token can survive its
owner's logout for up to its remaining 15 minutes. The team chose bounded staleness over a total
auth outage, and capped every Redis call at 250 ms so the worst case is latency, not a hang.

## Consequences

- Availability: a Redis outage degrades to "logout stops working for up to 15 minutes" instead of
  "nobody can use the API".
- Cost and gaps, stated plainly: (1) no test pins the fail-open branch — `blacklist_redis_test.go`
  covers revoke/check and expiry, not an unreachable server; (2) the in-memory fallback blacklist is
  per-process, so it is only correct for single-instance dev and is lost on restart. Both are listed
  here so they are not mistaken for solved; (1) is a one-case test, (2) needs a durable store.

## Verification

- `internal/server/session_test.go`: `TestRefreshIssuesNewTokens`, `TestRefreshRejectsAlreadyUsedToken`,
  `TestLogoutRevokesAccessToken` — rotation and revocation semantics, pure unit (no Redis).
- `internal/auth/blacklist_redis_test.go`: `TestRedisBlacklistRevokeAndCheck`,
  `TestRedisBlacklistExpiredJtiIsNotRevoked` — skips when `127.0.0.1:6379` is unreachable.
- `internal/server/auth_test.go`, `internal/server/users_auth_test.go` — the middleware gate.
- Unpinned today: the `err != nil → false` branch in `blacklist.go:129`.

# AGENTS.md

This file provides guidance to Claude Code, Codex, GitHub Copilot, and other AI coding agents
working in this repository.

## About This Project

`admin-api` is the HTTP microservice for the SweetRPG platform's cross-cutting admin concerns.
The first capability is banner messages - platform/service/page-scoped notices with severity and
expiration, consumed by every frontend that wants to show them. It's a thin Gin-based layer:
`server/*.go` wires routes directly to MongoDB via `mongodb.go`'s generic
`Get`/`Query`/`Insert`/`Update`/`Delete` functions (no separate `admin-data.go` library - this
service is small enough that a data-access layer split isn't justified yet).

## Dependencies

Depends on `api-core.go` (tracing, health checks, shared VOs/constants), `common.go` (logging),
and `mongodb.go` (database connection lifecycle and generic CRUD). Nothing in the platform
depends on this repo yet - `admin-web` (a separate repo) and consuming frontends
(`main-web`, `catalog-web`) call it over HTTP, not as a Go dependency.

## Write-route auth

Every `POST`/`PUT`/`DELETE` route requires either a forwarded user bearer token carrying the
`admin` role (verified against `auth-api`'s `/authz/check`, via the `authz` package) or, as a
legacy fallback during migration, the shared `X-Internal-Service-Token` header (matching
`INTERNAL_SERVICE_TOKEN`) plus `X-Acting-User-Sub` - see `server/middleware/writeauth.go` and
`sweetrpg/platform`'s `api-client-auth` OpenSpec change. `admin-web` forwards the acting admin's
own Auth0 access token from its shared session as the bearer credential; the legacy header path
exists only for callers not yet migrated and will be removed once none remain.

## Logging

HTTP access logs output in JSON format via `slog-gin` middleware, configured in `cmd/admin-api/main.go`.
Application logs remain under `common.go/logging` control. This provides structured logs suitable for
log aggregation systems while keeping HTTP and application concerns separate.

## Known deviations from `docs/service-conventions.md`

- No `gin-contrib/cache` per-route caching. Banner reads need to reflect admin edits promptly
  (an "expire this now" action should take effect immediately), and consuming frontends already
  do their own bounded-TTL caching client-side - a second cache layer here would only add
  staleness without a clear benefit for this service's read volume.

## Committing Code

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

## Branches and Workflow

* `develop` - integration branch, default branch, target for all PRs.
* `master` - latest released state, nothing committed directly.
* `feature/*`, `fix/*` branched from `develop`; `hotfix/*` branched from `master`.

See `CONTRIBUTING.md` for the full workflow.

## Running Checks Locally

```bash
go build -v ./...
go vet ./...
go test -v -coverprofile coverage.out ./...
```

Regenerate Swagger docs after changing handler annotations:

```bash
go run github.com/swaggo/swag/cmd/swag@latest init -d cmd/admin-api/,server/,models/ --parseDependency --parseInternal
```

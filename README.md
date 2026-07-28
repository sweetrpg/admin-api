# SweetRPG Admin API

[![CI](https://github.com/sweetrpg/admin-api/actions/workflows/ci.yaml/badge.svg)](https://github.com/sweetrpg/admin-api/actions/workflows/ci.yaml)
[![License](https://img.shields.io/github/license/sweetrpg/admin-api.svg)](https://img.shields.io/github/license/sweetrpg/admin-api.svg)
[![Issues](https://img.shields.io/github/issues/sweetrpg/admin-api.svg)](https://img.shields.io/github/issues/sweetrpg/admin-api.svg)
[![PRs](https://img.shields.io/github/issues-pr/sweetrpg/admin-api.svg)](https://img.shields.io/github/issues-pr/sweetrpg/admin-api.svg)
[![Dependabot](https://badgen.net/github/dependabot/sweetrpg/admin-api)](https://badgen.net/github/dependabot/sweetrpg/admin-api)

HTTP microservice for the SweetRPG platform's cross-cutting admin concerns. The first capability
is **banner messages**: platform-, service-, or page-scoped notices with a severity and a
required expiration, served to any frontend that wants to render them. A thin Gin-based layer:
`server/*.go` wires routes directly to MongoDB via [mongodb.go](https://github.com/sweetrpg/mongodb.go)'s
generic CRUD functions.

## API

- `POST /banners` - create a banner message.
- `GET /banners?scope=platform&scope=service:catalog&scope=page:/catalog/browse` - list active
  banners matching any of the given scopes, most-severe-first. Platform-scoped banners are
  always included regardless of the requested scopes.
- `PUT /banners/{id}` - update a banner message.
- `DELETE /banners/{id}` - delete a banner message.

`expires_at` is required on create; a banner without one is rejected. Expired documents are also
physically removed by a MongoDB TTL index on `expires_at`.

## Run locally

```bash
scripts/run-docker-local.sh
```

Brings up the service plus its MongoDB dependency via `docker/docker-compose.yml`. Swagger UI is
served at `/swagger/index.html` once running.

## Documentation

Package documentation: [pkg.go.dev/github.com/sweetrpg/admin-api](https://pkg.go.dev/github.com/sweetrpg/admin-api).

Swagger UI (`swaggo/swag` + `gin-swagger`, generated from handler annotations, not
hand-written): `/swagger/index.html` against whatever host you're running against.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow.

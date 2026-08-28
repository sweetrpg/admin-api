
## 0.8.3 - 2026-08-28

### Fixed
- Exclude expired records from active filter



## 0.8.2 - 2026-08-22

### Changed
- Remove legacy X-Internal-Service-Token fallback from write routes



## 0.8.1 - 2026-08-21

### Fixed
- Fix Deployment fields that never matched ArgoCD's applied manifest



## 0.8.0 - 2026-08-18

### Added
- Enable Stakater reloader for Prometheus metrics
- Add MongoDB connection config to local overlay
- Authorize write routes on forwarded user token, not shared secret



## 0.7.0 - 2026-08-11

### Added
- Gate continuous profiling behind the profiling-enabled feature flag



## 0.6.0 - 2026-08-07

### Added
- Add slog-gin for JSON HTTP access logging
- Add slog-gin JSON HTTP access logging


### Documentation
- Document slog-gin JSON HTTP logging


### Fixed
- Scope AtlasDatabaseUser role to sweetrpg-admin
- Authenticate AtlasDatabaseUser against admin, not app db



## 0.5.4 - 2026-08-06

### Fixed
- Version-prefixed Ingress paths with a latest-alias Service



## 0.5.3 - 2026-08-06

### Fixed
- Point readinessProbe at /status/ping, not /status/health



## 0.5.2 - 2026-08-06

### Fixed
- Widen readinessProbe timeout margin above Mongo's worst case



## 0.5.1 - 2026-08-06

### Fixed
- Target DB_PASSWORD explicitly in the admin-api-db rewrite
- Remove startup log dump of the full process environment



## 0.5.0 - 2026-08-05


## 0.5.0 - 2026-08-05

### Added
- Add Atlas user manifest


### Fixed
- Point atlas-db-password ExternalSecrets at new Akeyless path/key



## 0.4.0 - 2026-08-05

### Added
- Connect via admin-api's own Atlas database user



## 0.3.1 - 2026-08-05


## 0.3.0 - 2026-08-02


## 0.3.0 - 2026-08-02

### Added
- Add maintenance-mode resource


### Fixed
- Point local overlay at the shared local MongoDB



## 0.2.0 - 2026-08-01

### Added
- Require internal-service write auth and audit writes (#12)


### Fixed
- Point local overlay at the shared local MongoDB


## 0.1.0 - 2026-08-01

### Added
- Wire api-core.go tracing, health checks, and logging
- Add BannerMessage model with validation and TTL index
- Add banner CRUD and scoped query API
- Add include_inactive param for admin listing (#2)
- Add overlays/local for the Tailscale operator (#6)


### Fixed
- Include models/ in swag doc generation
- Don't fail go-docs when generated docs are unchanged
- Add missing dev ingress (#3)
- Fold overlays/local into the shared path-based scheme (#7)


## Unreleased

### Added

- Release automation via git-cliff and the shared Go release workflows.

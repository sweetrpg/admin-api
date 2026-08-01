
## 0.1.0 - 2026-08-01

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

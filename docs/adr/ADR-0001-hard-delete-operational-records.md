---
id: ADR-0001
title: banners and maintenance_modes hard-delete (audit-fields carve-out)
status: accepted
date: 2026-09-03
scope: admin-api
supersedes:
superseded_by:
tags: [data, persistence, audit]
---

# ADR-0001: banners and maintenance_modes hard-delete

Records `admin-api`'s hard-delete carve-out under the platform audit-fields convention, as
`PADR-0027` requires each hard-deleting service to do in its own repo.

## Context

The platform convention (`sweetrpg/platform` `PADR-0001` / `docs/data-conventions.md`) is soft
delete by default: a delete sets `deleted_at` / `deleted_by` and every read filters
`{deleted_at: nil}`. `PADR-0027` carves out classes where hard delete is correct instead, one of
which is **non-user operational / configuration records** — not owned by any end user, cheap to
recreate, with no recovery value in a tombstone.

`admin-api` persists two collections, both of that class:

- `banners` — short-lived admin banner messages, already removed by a MongoDB TTL index on
  `expires_at`, plus an explicit `DELETE /banners/:id`.
- `maintenance_modes` — one durable on/off record per scope, edited in place; `DELETE
  /maintenance-modes/:id` removes it.

Neither is addressed by a user-supplied resource id that must be owner-checked, and neither
carries history worth keeping past its own lifetime.

## Options

- **Option A — soft delete, matching the platform default.** Rejected: adds a `deleted_at` /
  `deleted_by` pair and a `{deleted_at: nil}` filter obligation on every read for records that
  gain nothing from recoverability. A banner past its `expires_at` is already gone via TTL; a
  maintenance record is one short document an admin retypes in seconds.
- **Option B — hard delete, no `deleted_*` (chosen).** See Decision.

## Decision

`DELETE` on `banners` and `maintenance_modes` removes the document. The models carry the
`created_at` / `created_by` / `updated_at` / `updated_by` audit fields (still required by
`PADR-0001`) but **no `deleted_at` / `deleted_by` pair**, and there is no `{deleted_at: nil}`
read filter and no deletion log (a deletion log is only required for security-control records
under `PADR-0027`, which these are not).

`created_by` / `updated_by` are stamped from the verified acting subject
(`middleware.ActingUserSubKey`) on every create and update.

## Consequences

- A deleted banner or maintenance record is unrecoverable. Accepted — re-creating either is
  trivial and there is no audit value in retaining a removed operational message.
- The platform-wide "always filter `{deleted_at: nil}`" review checkpoint does not apply to this
  service. A reviewer confirms the delete class from this ADR.
- Adding a third `admin-api` collection that *is* user-owned domain data would need soft delete
  (the platform default) and would not be covered by this ADR.

## Links

- Platform: `sweetrpg/platform` `PADR-0027` (delete strategy by data class), `PADR-0001` (audit
  fields), `openspec/changes/platform-audit-fields-convention` (task 9.5).

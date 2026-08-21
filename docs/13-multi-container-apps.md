# Multi-container apps (sidecar services) — design spec

Status: proposed (target v0.8.0)

## Problem

Some apps need companion services — a database and/or Redis — that the current
one-container-per-blueprint model can't provide. They crash-loop if installed, so
they are marked `coming_soon`: **immich** (Postgres + Redis), **paperless**
(Redis + DB), **ghost** (MySQL), **umami** (Postgres), **outline**
(Postgres + Redis).

We want a blueprint to declare private sidecar services that are created,
networked, and torn down together with the app — without changing single-container
blueprints and without exposing the sidecars.

## Goals

- A blueprint can declare one or more sidecar services (e.g. Postgres, Redis).
- The app reaches a sidecar by a stable hostname (the service name).
- Only the main app is ever reachable through Caddy (single-exposed-service
  invariant preserved). Sidecars are not routable and not shared between apps.
- Full lifecycle works on the whole group: install, start, stop, restart,
  uninstall, status, health, backup.
- Existing single-container blueprints are unchanged (backward compatible).

## Non-goals

- Sharing one database across multiple apps.
- Letting users supply arbitrary Docker Compose.
- Cross-engine backup dumps (pg_dump etc.) — see Backup below.

## Approaches considered

1. **Sidecars declared in the blueprint (chosen).** Each blueprint carries its
   own private services. Matches how these apps' official Compose files work;
   clean per-app isolation; deterministic hostnames.
2. **`depends_on` a separate shared DB blueprint (rejected).** Apps expect a DB
   at a known host (often `localhost`), sharing one Postgres needs per-app
   databases/users, and lifecycle coupling across separately-installed apps is
   fragile.
3. **Blueprint = raw Compose file (rejected).** Departs from the model, makes
   validation/hardening/health harder, and ties us to the Compose schema.

## Design (approach 1)

### Schema

Add an optional `services:` list. The main app stays under `container:`.

```yaml
services:
  - name: db                 # hostname on the app network; container pcg-<app>-db
    image: postgres:16-alpine
    environment:
      - POSTGRES_PASSWORD=... # generated/nudged like other secrets
    volumes:
      - pcg-<app>-db:/var/lib/postgresql/data
  - name: redis
    image: redis:7-alpine
```

The app references sidecars by service name, e.g. `DB_HOSTNAME=db`,
`REDIS_HOSTNAME=redis`. (immich's `localhost` values become `db`/`redis`.)

### Networking

- Create a per-app bridge network `pcg-net-<app>`.
- **Sidecars** join only `pcg-net-<app>`, with a network alias = service name.
  They are not on the shared network, so Caddy and other apps cannot reach them.
- **Main app** joins both `pcg-net-<app>` (to reach its sidecars by alias) and the
  shared `cloud_core_private` network (so Caddy can proxy to it). Unchanged for
  single-container apps (no per-app network created when `services` is empty).

### Lifecycle (docker manager)

Every container is labelled `pcg.app=<id>` and `pcg.role=app|service` so the whole
group is discoverable.

- **Install:** create `pcg-net-<app>` -> for each service: pull, create (per-app
  net, alias), start -> create + start the main app. Services start before the
  app; the app's own connection retry handles brief DB-not-ready windows (all
  five target apps retry).
- **Uninstall:** stop + remove the app and all services, then remove
  `pcg-net-<app>`. Named volumes are removed with the app (documented) so a
  reinstall is clean.
- **Start / Stop / Restart:** act on all containers with `pcg.app=<id>`
  (services first on start, app first on stop).
- **Status:** the app's status is the main container's status (unchanged).
- **Health:** unchanged — the blueprint's `health` check still targets the main
  app; sidecar readiness is internal.

`buildCreateBody` is reused for services (same hardening: `no-new-privileges`,
memory limits, named-volume-only). Validation (image charset, named volumes)
applies to services too.

### Backup

Backup already enumerates an app's volumes; extend it to include sidecar volumes.
For v0.8 this is a **volume snapshot** (filesystem copy of the DB volume), which
can be slightly inconsistent for a live database — documented, with the
recommendation to snapshot during low activity. Engine-aware dumps (pg_dump,
mysqldump) are a later enhancement.

### Migrating the coming-soon apps

Rewrite the five blueprints to declare their sidecars, point env at the service
hostnames, drop `coming_soon`, and verify each end-to-end with the lifecycle
test (install -> services + app running -> healthy -> uninstall -> group + network
gone).

## Testing

- Unit: group create body + per-app network wiring; label-based group lookup.
- E2E: a multi-container lifecycle test (start with **umami + Postgres** — the
  simplest) asserting sidecars come up, the app is healthy, and uninstall removes
  every container, volume, and the network.

## Rollout

- Ships as **v0.8.0** (minor: new, backward-compatible capability).
- Land in reviewable PRs: (1) schema + docker-manager group lifecycle + tests;
  (2) backup of sidecar volumes; (3) migrate the 5 apps off `coming_soon`, each
  verified. Cut v0.8.0 once all five pass.

## Open decisions

1. **DB backup fidelity:** volume snapshot (simple, minor inconsistency risk) vs
   engine dumps (pg_dump/mysqldump — better, more work). Proposed: volume
   snapshot for v0.8, dumps later.
2. **Sidecar volumes on uninstall:** remove with the app (clean reinstalls) vs
   keep (data survives an accidental uninstall). Proposed: remove with the app,
   matching how the app container is removed today; rely on the backup feature
   for safety.

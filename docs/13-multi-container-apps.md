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

### Backup (engine-aware dumps)

Databases are backed up with **engine-native dumps**, not raw volume copies, so
backups are consistent. A service declares how to dump/restore itself:

```yaml
services:
  - name: db
    image: postgres:16-alpine
    environment: [POSTGRES_USER=umami, POSTGRES_PASSWORD=..., POSTGRES_DB=umami]
    backup:
      dump:    ["pg_dump", "-U", "umami", "umami"]      # stdout -> services/db/dump.sql
      restore: ["psql", "-U", "umami", "-d", "umami"]   # dump.sql piped to stdin
```

- **Backup:** `docker exec` the `dump` command and stream stdout into the archive
  at `services/<name>/dump.sql` (encrypted with the rest of the archive).
- **Restore:** after the app + services are (re)created and the DB is accepting
  connections, `docker exec -i` the `restore` command with `dump.sql` on stdin.
  This makes restore multi-step (create group -> wait for DB ready -> load dumps
  -> start app), so restore grows a readiness wait and a load phase.
- Redis and other caches declare no `backup` block and are simply recreated
  empty (their data is disposable).
- Non-DB app volumes still use the existing volume-snapshot path.

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

## Decisions (resolved)

1. **Approach:** sidecar services declared in the blueprint (approach 1).
2. **DB backup fidelity:** engine-native dumps (pg_dump / mysqldump) via a
   per-service `backup.dump`/`backup.restore` command — consistent backups.
   Restore therefore becomes multi-step (see Backup).
3. **Sidecar volumes on uninstall:** removed with the app, for clean reinstalls;
   the backup feature is the safety net.

# App lifecycle coverage

Each blueprint is a different upstream image and can fail in ways a single app
can't reveal (wrong port, a health path that never returns the expected status,
missing env/volumes, needing an external database, slow or crash-looping
startup). So every app is verified individually with the data-driven lifecycle
test, and its status is tracked here.

## How to verify an app

The lifecycle test (`apps/core/e2e/lifecycle_test.go`) installs an app, waits for
its container to reach **running**, confirms Caddy **routes** to it, waits for it
to report **healthy** against the blueprint's own `health.path` / `expected_status`,
then **uninstalls** it and confirms the route is gone.

Run it against a live stack for any app(s):

```sh
E2E_BASE_URL=http://home.localtest.me \
E2E_EMAIL=you@example.com E2E_PASSWORD=… \
E2E_APPS=gitea,vaultwarden \
go test -v -tags e2e -timeout 1800s -run TestE2E_AppLifecycle ./e2e/...
```

**Add an app → verify it → update the table below.** This is a per-release /
per-app check, not a per-PR gate (it pulls images and starts containers).

## Status

Legend: ✅ verified (full lifecycle: install → running → routed → healthy →
uninstalled) · ❌ does not work as a single container

Last verified: 2026-08-21 (local dev stack).

**16 of 19 verified working.** The 3 failures all need external databases the
single-container blueprint model can't provide (see below).

| App | Subdomain | Health check | Status | Notes |
|---|---|---|---|---|
| excalidraw | `draw` | `/` 200 | ✅ | static |
| memos | `memos` | `/` 200 | ✅ | sqlite |
| uptime-kuma | `status` | `/` 200 | ✅ | sqlite |
| vaultwarden | `vault` | `/alive` 200 | ✅ | sqlite |
| gitea | `git` | `/api/healthz` 200 | ✅ | sqlite |
| shiori | `bookmarks` | `/` 200 | ✅ | sqlite |
| filebrowser | `files` | `/` 200 | ✅ | fixed: internal_port 8080→80 |
| actual-budget | `budget` | `/` 200 | ✅ | |
| silverbullet | `notes` | `/` 200 | ✅ | |
| couchdb | `couchdb` | `/` 200 | ✅ | set a real password before exposing |
| freshrss | `rss` | `/` 200 | ✅ | |
| n8n | `n8n` | `/healthz` 200 | ✅ | fixed: memory 512m→1024m (was OOM crash-loop) |
| stirling-pdf | `pdf` | `/api/v1/info/status` 200 | ✅ | fixed: JVM metaspace + memory 2g + health path (`/` returns 401 via its own login) |
| jellyfin | `media` | `/health` 200 | ✅ | |
| nextcloud | `cloud` | `/status.php` 200 | ✅ | |
| homeassistant | `ha` | `/api/` 401 | ✅ | 401 is the healthy signal (no token) |
| ghost | `blog` | `/ghost/api/v4/admin/site/` 200 | ❌ | crash-loops — Ghost needs an external **MySQL** |
| paperless | `docs` | `/api/` 200 | ❌ | needs **Redis** (+ a database) |
| immich | `photos` | `/api/server-info/ping` 200 | ❌ | needs **Postgres + Redis** (`DB_HOSTNAME=localhost` in its own container) |

## Not supported as single containers (ghost, paperless, immich)

These three are **multi-container apps**: they expect an external database (and
Redis) that the current one-container-per-blueprint model doesn't provision.
`ghost` crash-loops connecting to MySQL at `127.0.0.1:3306`; `immich`/`paperless`
similarly expect Postgres/Redis. They cannot work as bundled today.

Options (a v1 decision):
1. **Quarantine for v1** — remove them from the bundled set (or hide behind an
   "experimental/unsupported" flag) so users don't install a broken app.
2. **Add multi-container blueprint support post-v1** — let a blueprint declare
   sidecar services (Postgres/Redis) on the app's private network and point the
   app's env at them. The blueprint schema already has a `depends_on` field to
   build on.

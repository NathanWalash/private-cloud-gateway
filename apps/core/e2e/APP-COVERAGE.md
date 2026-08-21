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

Status column: **yes** = verified (full lifecycle: install -> running -> routed
-> healthy -> uninstalled) · **soon** = coming soon (listed, install blocked).

Last verified: 2026-08-21 (local dev stack).

**22 of 24 verified working; 2 coming soon.** Every "yes" app passed the full
lifecycle. Multi-container support (v0.8) let umami, ghost, and paperless
graduate with sidecar databases. The 2 remaining "soon" apps need more than
plain sidecars (see below).

| App | Subdomain | Health check | Status | Notes |
|---|---|---|---|---|
| excalidraw | `draw` | `/` 200 | yes | static |
| memos | `memos` | `/` 200 | yes | sqlite |
| uptime-kuma | `status` | `/` 200 | yes | sqlite |
| vaultwarden | `vault` | `/alive` 200 | yes | sqlite |
| gitea | `git` | `/api/healthz` 200 | yes | sqlite |
| filebrowser | `files` | `/` 200 | yes | fixed: internal_port 8080→80 |
| actual-budget | `budget` | `/` 200 | yes | |
| silverbullet | `notes` | `/` 200 | yes | |
| couchdb | `couchdb` | `/` 200 | yes | set a real password before exposing |
| freshrss | `rss` | `/` 200 | yes | |
| n8n | `n8n` | `/healthz` 200 | yes | fixed: memory 512m→1024m (was OOM crash-loop) |
| stirling-pdf | `pdf` | `/api/v1/info/status` 200 | yes | fixed: JVM metaspace + memory 2g + health path (`/` returns 401 via its own login) |
| jellyfin | `media` | `/health` 200 | yes | |
| nextcloud | `cloud` | `/status.php` 200 | yes | |
| homeassistant | `ha` | `/api/` 401 | yes | 401 is the healthy signal (no token) |
| it-tools | `tools` | `/` 200 | yes | dev utilities (static) |
| cyberchef | `cyberchef` | `/` 200 | yes | dev utilities (static) |
| adminer | `db` | `/` 200 | yes | DB web UI |
| gatus | `gatus` | `/health` 200 | yes | status page; add checks in the config volume |
| umami | `analytics` | `/api/heartbeat` 200 | yes | + Postgres sidecar (multi-container) |
| ghost | `blog` | `/ghost/api/v4/admin/site/` 200 | yes | + MySQL sidecar (multi-container) |
| paperless | `docs` | `/` 200 | yes | + Redis sidecar (multi-container) |
| outline | `wiki` | `/_health` 200 | soon | Postgres + Redis AND an external auth provider |
| immich | `photos` | `/api/server-info/ping` 200 | soon | vector-enabled Postgres + Redis + ML service |

(Removed: `shiori` — bookmarks, not wanted.)

## Coming soon — need more than plain sidecars

Multi-container support (v0.8, see `docs/13-multi-container-apps.md`) lets a
blueprint declare private sidecar databases, which is how umami, ghost, and
paperless now work. Two apps need more than that and stay `coming_soon: true`
(listed but install blocked):

- **outline** — beyond Postgres + Redis it requires an external authentication
  provider (OIDC/Google/Slack); there is no built-in login. It graduates once
  the gateway can act as an OIDC provider for it.
- **immich** — needs a *vector-enabled* Postgres (the `immich-app/postgres`
  image with the VectorChord extension), Redis, and a separate machine-learning
  service. Achievable with the current sidecar model but not yet verified.

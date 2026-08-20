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

Legend: ✅ verified · ⏳ not yet verified · ⚠️ needs external services/config

Last verified batch: 2026-08-20 (local dev stack).

| App | Subdomain | Health check | Status | Notes |
|---|---|---|---|---|
| excalidraw | `draw` | `/` 200 | ✅ | static, single container |
| memos | `memos` | `/` 200 | ✅ | single container + sqlite |
| uptime-kuma | `status` | `/` 200 | ✅ | single container + sqlite |
| vaultwarden | `vault` | `/alive` 200 | ✅ | single container + sqlite |
| gitea | `git` | `/api/healthz` 200 | ✅ | single container + sqlite |
| shiori | `bookmarks` | `/` 200 | ✅ | single container + sqlite |
| filebrowser | `files` | `/` 200 | ⏳ | single container |
| actual-budget | `budget` | `/` 200 | ⏳ | single container |
| silverbullet | `notes` | `/` 200 | ⏳ | single container |
| couchdb | `couchdb` | `/` 200 | ⏳ | database; set a real password before exposing |
| freshrss | `rss` | `/` 200 | ⏳ | single container |
| n8n | `n8n` | `/healthz` 200 | ⏳ | single container + sqlite |
| stirling-pdf | `pdf` | `/` 200 | ⏳ | large image, slow first pull |
| jellyfin | `media` | `/health` 200 | ⏳ | large image |
| nextcloud | `cloud` | `/status.php` 200 | ⏳ | slow first-boot init |
| ghost | `blog` | `/ghost/api/v4/admin/site/` 200 | ⏳ | verify it comes up without an external DB |
| homeassistant | `ha` | `/api/` 401 | ⏳ | slow first-boot; 401 is the healthy signal |
| paperless | `docs` | `/api/` 200 | ⚠️ | normally needs Redis + a database — verify standalone |
| immich | `photos` | `/api/server-info/ping` 200 | ⚠️ | normally needs Postgres + Redis — verify standalone |

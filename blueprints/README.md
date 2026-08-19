# Blueprints

Blueprints define installable apps. Place a `.yaml` file here and it appears in the dashboard Install dialog.

A blueprint tells the gateway how to create the container, route a subdomain, health check, back up, and show the app on the dashboard. See `docs/06-docker-and-app-blueprints.md` for the full schema.

## Placeholders

Environment values may use these placeholders, substituted at install time for
the running instance's values (so one blueprint works in both dev and production):

| Placeholder | Substituted with | Example |
|---|---|---|
| `${DOMAIN}` | The configured root domain | `cloud.${DOMAIN}` → `cloud.example.com` |
| `${SCHEME}` | `https` in production, `http` in local dev | `${SCHEME}://n8n.${DOMAIN}/` |

Never hard-code `localtest.me` or `http://` in an app that needs its own public
URL (e.g. Nextcloud trusted domains, n8n host, Ghost URL) — use the placeholders.

## Available blueprints

| File | App | Subdomain | Category |
|---|---|---|---|
| `actual-budget.yaml` | Actual Budget | `budget` | Finance |
| `couchdb.yaml` | CouchDB | `couchdb` | Storage |
| `excalidraw.yaml` | Excalidraw | `draw` | Utilities |
| `filebrowser.yaml` | File Browser | `files` | Storage |
| `freshrss.yaml` | FreshRSS | `rss` | Productivity |
| `ghost.yaml` | Ghost | `blog` | Productivity |
| `gitea.yaml` | Gitea | `git` | Development |
| `homeassistant.yaml` | Home Assistant | `home` | Automation |
| `immich.yaml` | Immich | `photos` | Storage |
| `jellyfin.yaml` | Jellyfin | `media` | Media |
| `memos.yaml` | Memos | `memos` | Productivity |
| `n8n.yaml` | n8n | `n8n` | Automation |
| `nextcloud.yaml` | Nextcloud | `cloud` | Storage |
| `paperless.yaml` | Paperless-ngx | `docs` | Productivity |
| `shiori.yaml` | Shiori | `bookmarks` | Productivity |
| `silverbullet.yaml` | SilverBullet | `notes` | Productivity |
| `stirling-pdf.yaml` | Stirling PDF | `pdf` | Utilities |
| `uptime-kuma.yaml` | Uptime Kuma | `status` | Monitoring |
| `vaultwarden.yaml` | Vaultwarden | `vault` | Security |

## Notes

**Obsidian sync:** Obsidian is a desktop app — there is no Docker server for it.
Install `couchdb` and use the [Self-hosted LiveSync](https://github.com/vrtmrz/obsidian-livesync) plugin
in Obsidian to sync your vault to your server — see the step-by-step
[Obsidian sync guide](../docs/obsidian-sync.md). Or install `silverbullet` for an
Obsidian-like markdown notes experience that runs entirely in the browser.

**Vaultwarden:** `SIGNUPS_ALLOWED=false` is set by default. Create your account immediately
after first install before anyone else can.

**CouchDB:** Change `COUCHDB_PASSWORD=changeme` before installing.

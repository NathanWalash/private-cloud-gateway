# Blueprints

Blueprints define installable apps. Place a `.yaml` file here and it appears in the dashboard Install dialog.

A blueprint tells the gateway how to create the container, route a subdomain, health check, back up, and show the app on the dashboard. See `docs/06-docker-and-app-blueprints.md` for the full schema.

## Available blueprints

| File | App | Subdomain | Category |
|---|---|---|---|
| `filebrowser.yaml` | File Browser | `files` | Storage |
| `stirling-pdf.yaml` | Stirling PDF | `pdf` | Utilities |
| `shiori.yaml` | Shiori | `bookmarks` | Productivity |
| `silverbullet.yaml` | SilverBullet | `notes` | Productivity |
| `memos.yaml` | Memos | `memos` | Productivity |
| `uptime-kuma.yaml` | Uptime Kuma | `status` | Monitoring |
| `vaultwarden.yaml` | Vaultwarden | `vault` | Security |
| `actual-budget.yaml` | Actual Budget | `budget` | Finance |
| `n8n.yaml` | n8n | `n8n` | Automation |
| `excalidraw.yaml` | Excalidraw | `draw` | Utilities |
| `couchdb.yaml` | CouchDB | `couchdb` | Storage |

## Notes

**Obsidian sync:** Obsidian is a desktop app — there is no Docker server for it.
Install `couchdb` and use the [Self-hosted LiveSync](https://github.com/vrtmrz/obsidian-livesync) plugin
in Obsidian to sync your vault to your server. Or install `silverbullet` for an Obsidian-like
markdown notes experience that runs entirely in the browser.

**Vaultwarden:** `SIGNUPS_ALLOWED=false` is set by default. Create your account immediately
after first install before anyone else can.

**CouchDB:** Change `COUCHDB_PASSWORD=changeme` before installing.

# Technical Stack

## Recommended stack

| Layer | Choice |
|---|---|
| Backend/control plane | Go |
| Frontend/dashboard | Vite + TypeScript + Tailwind CSS |
| Control database | SQLite in WAL mode |
| Reverse proxy | Caddy |
| Runtime | Docker Compose |
| Production host | Ubuntu Server on Oracle Cloud |
| Service manager | systemd |
| App definitions | YAML blueprints |
| Backups | Encrypted archives, later restic/Borg/S3-compatible storage |
| Testing | Go tests, Testcontainers, Playwright, k6 later |

## Why Go for the core engine

Go is a good fit because the core engine needs to:

- run as a small service
- talk to Docker
- expose HTTP APIs
- run background workers
- perform health checks
- manage backups
- deploy as a simple binary/container

## Why SQLite

SQLite is used for Cloud Core's control-plane state only.

Good fit for:

- one node
- one owner
- low operational overhead
- settings/state/audit logs

Do not use SQLite as the forced database for every hosted app.

## Why Caddy

Caddy is used for:

- HTTPS
- automatic certificates
- reverse proxying
- subdomain routing
- forward-auth integration

## Why Docker Compose first

Docker Compose is simpler than Kubernetes for a single-node personal cloud.

Kubernetes/K3s is not required for v1.

## Why subdomains over iframes

Default app access should be:

```text
app-card -> app.example.com
```

This avoids browser embedding limitations and keeps apps running naturally.

Inline embedding may be added later only for apps that support it safely.

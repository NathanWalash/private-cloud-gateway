# Architecture

## High-level architecture

```text
Oracle Cloud Ubuntu VM
  ├── Caddy
  │     └── public HTTPS entrypoint
  │
  ├── Cloud Core Go Engine
  │     ├── auth/session verification
  │     ├── app catalogue
  │     ├── Docker lifecycle manager
  │     ├── Caddy route manager
  │     ├── widget/status API
  │     ├── backup/restore engine
  │     └── SQLite database
  │
  ├── Cloud Core Web UI
  │     └── login, dashboard, widgets, settings
  │
  └── Private Docker Network
        ├── filebrowser
        ├── stirling-pdf
        ├── shiori
        ├── uptime-kuma
        └── custom apps
```

## Component responsibilities

| Component | Responsibility |
|---|---|
| Caddy | Public HTTPS gateway, subdomain routing, auth check |
| Go Core | Auth, Docker control, blueprints, backups, metrics, API |
| SQLite | Control-plane state |
| Web UI | Login, dashboard, widgets, app controls |
| Docker | Runs hosted apps |
| Blueprints | Define installable apps |
| Oracle VM | Production host |

## Request flow

```text
User requests files.nathan.me
  ↓
Caddy receives HTTPS request
  ↓
Caddy asks Go Core /api/auth/verify
  ↓
Go Core validates session
  ↓
If valid: Caddy proxies to filebrowser container
  ↓
If invalid: redirect to nathan.me/login
```

## Internal networking

Apps run on a private Docker network.

Only Caddy should be publicly reachable.

## State

Cloud Core stores its own control-plane state in SQLite:

- users
- sessions
- apps
- routes
- blueprints
- widgets
- backup jobs
- backup records
- health checks
- audit events
- settings

Hosted apps keep their own data in mounted volumes.

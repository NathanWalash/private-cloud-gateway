# Cloud Core — Go Backend

The Cloud Core service is the Go control plane for Private Cloud Gateway.

## Responsibilities

- Login page and session management
- `/api/auth/verify` — Caddy forward-auth endpoint
- Docker lifecycle management (Milestone 3)
- App blueprint parsing (Milestone 3)
- Backup orchestration (Milestone 4)
- Health and status APIs (Milestone 3+)

## Running locally

The service is built and run via Docker Compose from the repo root.
See [docs/dev-local.md](../../docs/dev-local.md) for full setup.

```bash
# From repo root
docker compose up core
```

## Developing without Docker

Requires Go 1.22+. On first clone, generate `go.sum`:

```bash
cd apps/core
go mod tidy
go run .
```

The service reads configuration from environment variables. Copy `.env.example`
to `.env` at the repo root before running.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `CLOUD_CORE_SESSION_SECRET` | yes | — | Min 32-char random string. Generate: `openssl rand -hex 32` |
| `CLOUD_CORE_DATABASE_PATH` | no | `./data/cloud-core.db` | SQLite file path |
| `CLOUD_CORE_PORT` | no | `8080` | Listen port |
| `CLOUD_CORE_LOGIN_URL` | no | `/login` | Where Caddy redirects unauthenticated users |
| `CLOUD_CORE_BOOTSTRAP_EMAIL` | no | — | Admin email — used only on first startup |
| `CLOUD_CORE_BOOTSTRAP_PASSWORD` | no | — | Admin password — used only on first startup |

## Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/login` | Login page (HTML form) |
| `POST` | `/api/auth/login` | Submit login credentials |
| `POST` | `/api/auth/logout` | Clear session and redirect to login |
| `GET` | `/api/auth/verify` | Caddy forward-auth endpoint (returns 200 or 401) |
| `GET` | `/healthz` | Health check |
| `GET` | `/` | Dashboard placeholder (Milestone 2) |

## Package structure

```text
apps/core/
├── main.go                 Entry point — reads config, opens DB, starts server
└── internal/
    ├── db/
    │   ├── db.go           SQLite connection (WAL mode, foreign keys on)
    │   ├── migrate.go      Schema migrations (users, sessions, audit_log)
    │   └── bootstrap.go    Creates first admin user on fresh install
    ├── auth/
    │   ├── handler.go      Login, logout, verify, login page handlers
    │   └── session.go      Session create/validate/delete (DB-backed)
    └── server/
        └── server.go       HTTP mux and route registration
```

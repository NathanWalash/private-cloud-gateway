# Project Memory

Instructions here apply to this project and are shared with team members.

## What this project is

Private Cloud Gateway — a personal, self-hosted cloud dashboard and Docker app gateway.
One login protects all hosted apps. Caddy is the public HTTPS entrypoint. A Go service
(internally "Cloud Core") handles auth, Docker lifecycle, blueprints, and backups.
SQLite stores control-plane state. Dashboard is Vite + TypeScript + Tailwind (Milestone 2).
Production target: single Oracle Cloud Ubuntu VM.

## Current state — Milestone 1 COMPLETE

All PRs merged. Local dev stack fully working and tested.

Start: `docker compose -f infra/docker/docker-compose.yml up --build`

## Local dev URLs

`*.localtest.me` resolves to 127.0.0.1 via public DNS. No hosts file needed.
Do NOT use `*.localhost` — Chrome blocks Domain=localhost cookies (Public Suffix List).

| URL | What |
|---|---|
| `http://home.localtest.me` | Login page and dashboard |
| `http://files.localtest.me` | Protected test app (whoami) |

## Git workflow

Branch per feature. PRs required before merging to main. No direct commits to main.
Conventional Commits: `feat(scope):`, `fix(scope):`, `docs(scope):`, `chore(scope):`, `test(scope):`
See `docs/git-workflow.md`.

## Key env vars

| Var | Default | Notes |
|---|---|---|
| `CLOUD_CORE_SESSION_SECRET` | required | Min 32 chars. `openssl rand -hex 32` |
| `CLOUD_CORE_LOGIN_URL` | `http://home.localtest.me/login` | Must be absolute URL |
| `CLOUD_CORE_COOKIE_DOMAIN` | `localtest.me` | Root domain, not subdomain |
| `CLOUD_CORE_ENV` | `production` | Set to `development` for text logs |
| `CLOUD_CORE_BOOTSTRAP_EMAIL` | — | First-run admin creation only |
| `CLOUD_CORE_BOOTSTRAP_PASSWORD` | — | First-run admin creation only |

## Architecture invariants (never break these)

1. Caddy is the ONLY service with host `ports:` — everything else uses `expose:`
2. All protected routes use `forward_auth core:8080 { uri /api/auth/verify }`
3. Session cookie: HttpOnly, SameSite=Lax, Domain=<root domain>
4. Login rate limited: 10 attempts / IP / minute
5. HTTP server timeouts: ReadTimeout 15s, WriteTimeout 30s, IdleTimeout 120s

## CI pipeline

- `repo-check` — required files present
- `markdown-lint` — markdownlint-cli2
- `shellcheck` — shell scripts
- `yaml-lint` — YAML files
- `go-lint` — golangci-lint (govet, errcheck, staticcheck, gosec, ...)
- `go-vuln` — govulncheck (CVE scan)
- `go-test` — full test suite with -race detector
- `go-build` — Docker image (requires lint + tests first)

## Next milestone

**Milestone 2: Dashboard foundation**
- `apps/web/` — Vite + TypeScript + Tailwind
- Proper login page (replace Go Core inline HTML)
- Dashboard layout: app cards, server status widgets
- Responsive mobile layout

# Project Memory

Instructions here apply to this project and are shared with team members.

## What this project is

Private Cloud Gateway — a personal, self-hosted cloud dashboard and Docker app gateway.
One login protects all hosted apps. Caddy is the public HTTPS entrypoint. A Go service
(internally called "Cloud Core") handles auth, Docker lifecycle, blueprints, and backups.
SQLite stores control-plane state. Dashboard is Vite + TypeScript + Tailwind (not yet built).
Production target is a single Oracle Cloud Ubuntu VM.

## Current state — Milestone 1 COMPLETE

All PRs merged to main. The local dev stack is working:
- `home.localtest.me` → login page and dashboard placeholder
- `files.localtest.me` → protected by forward-auth → whoami test app

Start with: `docker compose -f infra/docker/docker-compose.yml up --build`

## Local dev URLs

Uses `*.localtest.me` (public wildcard DNS → 127.0.0.1). No hosts file needed.
Do NOT use `*.localhost` — Chrome blocks `Domain=localhost` cookies (Public Suffix List).

## Git workflow

- Branch per feature. PRs required before merging to main. No direct commits to main.
- Conventional Commits: `feat(scope):`, `fix(scope):`, `docs(scope):`, `chore(scope):`, `test(scope):`
- See `docs/git-workflow.md` for full conventions.

## Repo structure

```
apps/core/          Go backend (auth, sessions, SQLite, /api/auth/verify)
apps/web/           Vite dashboard (not yet implemented — Milestone 2)
infra/caddy/        Caddyfile (local dev uses localtest.me, HTTP only)
infra/docker/       docker-compose.yml
blueprints/         YAML app definitions
scripts/            dev-up.sh, dev-down.sh, test.sh, lint.sh
docs/               All project documentation
```

## Key env vars (apps/core)

| Var | Local default | Notes |
|---|---|---|
| `CLOUD_CORE_SESSION_SECRET` | — | Required. Min 32 chars. `openssl rand -hex 32` |
| `CLOUD_CORE_LOGIN_URL` | `http://home.localtest.me/login` | Must be absolute URL |
| `CLOUD_CORE_COOKIE_DOMAIN` | `localtest.me` | Use root domain, not subdomain |
| `CLOUD_CORE_BOOTSTRAP_EMAIL` | — | First-run admin creation |
| `CLOUD_CORE_BOOTSTRAP_PASSWORD` | — | First-run admin creation |

## CI

GitHub Actions runs on every PR:
- `repo-check` — required files present
- `markdown-lint` — markdownlint-cli2
- `shellcheck` — shell scripts
- `yaml-lint` — blueprints/ and workflows/
- `go-test` — `docker build --target tester` (runs all Go tests)
- `go-build` — final runtime image (requires go-test to pass)

## Architecture rules (never break these)

1. Caddy is the only public entrypoint — only service with host `ports:`
2. Apps use `expose:` not `ports:` — never directly reachable from outside Docker
3. All protected routes go through `forward_auth core:8080 { uri /api/auth/verify }`
4. Session cookie: `HttpOnly`, `SameSite=Lax`, `Domain=<root domain>`

## Next milestone

**Milestone 2: Dashboard foundation**
- Vite + TypeScript + Tailwind app in `apps/web/`
- Login page (replace Go Core's inline HTML)
- Dashboard layout with app cards and status widgets
- Responsive mobile layout

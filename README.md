# Private Cloud Gateway

[![CI](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml)
[![Go 1.22](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A private, login-protected cloud dashboard and Docker app gateway for a single owner.

Every hosted app sits behind one login. Caddy is the only public entry point. A Go service verifies sessions, manages Docker app lifecycles, and handles backups. Apps run on a private Docker network and are never directly reachable from the internet.

```
Browser
  │
  ▼
Caddy (HTTPS, port 80/443)
  │  forward_auth: is session valid?
  ▼
Cloud Core (Go service, private)
  │  valid → proxy to app
  │  invalid → redirect to login
  ▼
App containers (private Docker network, no host ports)
```

## Status

| Milestone | Status |
|---|---|
| 1 — Local secured routing | ✅ Complete |
| 2 — Dashboard (Vite + Tailwind) | 🔜 Next |
| 3 — App blueprints | Planned |
| 4 — Backup and restore | Planned |
| 5 — Oracle Cloud deployment | Planned |

## Quick start

**Prerequisites:** Docker Desktop, a `.env` file.

```bash
git clone https://github.com/NathanWalash/private-cloud-gateway.git
cd private-cloud-gateway
cp .env.example .env
# Edit .env — set CLOUD_CORE_SESSION_SECRET, BOOTSTRAP_EMAIL, BOOTSTRAP_PASSWORD
docker compose -f infra/docker/docker-compose.yml up --build
```

Open Chrome and visit `http://home.localtest.me`.

> `*.localtest.me` resolves to `127.0.0.1` via public DNS — no hosts file changes needed.

## Local dev URLs

| URL | What |
|---|---|
| `http://home.localtest.me` | Login page and dashboard |
| `http://files.localtest.me` | Protected test app (whoami) |

## Required `.env` values

| Variable | Description |
|---|---|
| `CLOUD_CORE_SESSION_SECRET` | Min 32 random chars. Generate: `openssl rand -hex 32` |
| `CLOUD_CORE_BOOTSTRAP_EMAIL` | Admin email created on first startup |
| `CLOUD_CORE_BOOTSTRAP_PASSWORD` | Admin password created on first startup |

See `.env.example` for the full list with defaults.

## Architecture

```
private-cloud-gateway/
├── apps/
│   ├── core/        Go backend — auth, sessions, Docker lifecycle, backups
│   └── web/         Vite + TypeScript + Tailwind dashboard (Milestone 2)
├── infra/
│   ├── caddy/       Caddyfile — subdomain routing + forward-auth
│   └── docker/      docker-compose.yml
├── blueprints/      YAML app definitions (Milestone 3)
├── docs/            Full project documentation
└── scripts/         dev-up, dev-down, test, lint
```

**Key invariants:**

- Caddy is the **only** service with exposed host ports (`80`, `443`).
- All other services use `expose:` only — never reachable from outside Docker.
- Every protected subdomain uses `forward_auth` to verify the session before proxying.
- Session cookies are `HttpOnly`, `SameSite=Lax`, scoped to the root domain.

## Development

```bash
# Start the stack
./scripts/dev-up.sh          # or: make dev-up

# Run tests
./scripts/test.sh            # or: make test

# Run linters
./scripts/lint.sh            # or: make lint

# Stop the stack
./scripts/dev-down.sh        # or: make dev-down
```

See [docs/dev-local.md](docs/dev-local.md) for full setup, prerequisite versions, and the Milestone 1 test checklist.

## CI

Every pull request runs:

- `repo-check` — required files present
- `markdown-lint` — markdownlint-cli2
- `shellcheck` — shell scripts
- `yaml-lint` — blueprints and workflow YAML
- `go-lint` — golangci-lint (govet, errcheck, staticcheck, gosec, …)
- `go-vuln` — govulncheck dependency vulnerability scan
- `go-test` — full test suite with race detector (`-race`)
- `go-build` — Docker image build (requires lint + tests to pass first)

## Documentation

| Doc | Description |
|---|---|
| [docs/dev-local.md](docs/dev-local.md) | Local setup, prerequisites, test checklist |
| [docs/git-workflow.md](docs/git-workflow.md) | Branch naming, commit style, PR process |
| [docs/02-architecture.md](docs/02-architecture.md) | Architecture overview |
| [docs/04-security-model.md](docs/04-security-model.md) | Security model and requirements |
| [docs/decisions/](docs/decisions/) | Architecture decision records |
| [ROADMAP.md](ROADMAP.md) | Milestone definitions and progress |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Security policy |

## Contributing

PRs are required before merging to `main`. See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/git-workflow.md](docs/git-workflow.md).

## License

MIT

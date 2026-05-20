# Private Cloud Gateway

[![CI](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml)
[![Go 1.24](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A self-hosted cloud dashboard that protects every app behind a single login.
Each installed app gets its own subdomain, a Docker container on a private network,
and automatic HTTPS in production. Everything runs on one Oracle Cloud VM.

## How it works

```text
Browser
  |
  v  HTTPS
Caddy  (public gateway — only service with host ports)
  |
  v  forward_auth: verify session
Go Core  (auth, Docker lifecycle, blueprints, backup)
  |
  v  private Docker network
App containers  (File Browser, Vaultwarden, n8n, etc.)
```

When a request arrives for `pdf.yourdomain.com`, Caddy calls Go Core to
verify the session cookie. Valid session: request proxied to the PDF container.
No session: browser redirected to the login page.

## Features

| Category | What ships |
|---|---|
| **Authentication** | bcrypt passwords, TOTP two-factor auth, session cookies, rate limiting |
| **App management** | Install, start, stop, restart, update, remove Docker apps |
| **Blueprints** | YAML app definitions — 11 built-in (File Browser, Vaultwarden, n8n, Uptime Kuma, ...) |
| **Dashboard** | React 18 + Tailwind — marketplace, live status polling, logs viewer |
| **Backup** | AES-256-GCM encrypted archives, Safe Escape download, scheduled backups |
| **Monitoring** | URL health monitors, server status widget |
| **Production** | Let's Encrypt HTTPS, systemd service, one-command Oracle Cloud installer |
| **Developer** | Comprehensive test suite, CI pipeline, E2E tests, Docker dev tools |

## Quick start (local development)

Requirements: Docker Desktop, Git.

```bash
git clone https://github.com/NathanWalash/private-cloud-gateway.git
cd private-cloud-gateway
cp .env.example .env
# Edit .env — set CLOUD_CORE_SESSION_SECRET (openssl rand -hex 32)
./scripts/dev-up.sh            # or: make dev-up
```

Open `http://home.localtest.me` in Chrome. The setup wizard appears on first run.

> `*.localtest.me` resolves to `127.0.0.1` via public DNS. No hosts file needed
> in Chrome/Edge. Other browsers may need entries in `/etc/hosts`.

## Development commands

```bash
make dev-up      # start stack (builds on first run)
make dev-down    # stop stack
make nuke        # wipe everything — containers, volumes, images — and rebuild
make test        # run unit + integration tests
make lint        # run all linters
```

## Production deployment (Oracle Cloud)

One command on a fresh Ubuntu 22.04+ VM:

```bash
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

The installer prompts for your domain and admin email, generates secrets, configures
systemd and UFW, and starts the service. Point `A *.yourdomain.com` at the server IP
and visit `https://home.yourdomain.com` to complete setup.

See [infra/oracle/README.md](infra/oracle/README.md) for full deployment documentation.

## Architecture

```text
private-cloud-gateway/
  apps/
    core/        Go service — auth, Docker lifecycle, blueprints, backup, API
    web/         Vite + React 18 + TypeScript + Tailwind dashboard
  blueprints/    YAML app definitions
  infra/
    caddy/       Caddyfile (local dev + production)
    docker/      docker-compose files
    oracle/      systemd service, firewall, installer
  scripts/       dev-up, dev-down, dev-nuke, e2e, lint
```

**Architecture invariants:**

- Caddy is the **only** service with exposed host ports (80, 443).
- All app containers use `expose:` only — never reachable from outside Docker.
- Every protected subdomain uses `forward_auth` before proxying.
- Session cookies are `HttpOnly`, `SameSite=Lax`, scoped to the root domain.
- Login rate-limited to 10 attempts per IP per minute.
- Backup archives are AES-256-GCM encrypted when a passphrase is set.

## CI pipeline

Every pull request runs:

| Job | Tool |
|---|---|
| `repo-check` | Verifies required files exist |
| `markdown-lint` | markdownlint-cli2 |
| `shellcheck` | ShellCheck |
| `yaml-lint` | yamllint |
| `go-lint` | golangci-lint — govet, errcheck, staticcheck, gosec, bodyclose, noctx |
| `go-vuln` | govulncheck — CVE scan against Go vulnerability database |
| `go-test` | Full test suite with race detector |
| `node-build` | TypeScript type-check + Vite production build |
| `go-build` | Docker image build (requires all other jobs to pass) |

On version tags, `release.yml` builds and pushes the Docker image to `ghcr.io`.

## Running tests

```bash
# Unit and integration tests
cd apps/core && go test -v -race ./...

# End-to-end tests against a running stack
E2E_EMAIL=you@example.com E2E_PASSWORD=yourpass ./scripts/e2e.sh

# Go E2E with typed assertions
cd apps/core && E2E_EMAIL=x E2E_PASSWORD=y go test -v -tags e2e ./e2e/...
```

## Built-in blueprints

| App | Subdomain | Purpose |
|---|---|---|
| File Browser | `files.*` | Browser-based file manager |
| Stirling PDF | `pdf.*` | PDF merge, split, compress, OCR |
| Shiori | `bookmarks.*` | Read-later and bookmark manager |
| Uptime Kuma | `status.*` | Uptime monitoring for URLs, APIs, TCP |
| Vaultwarden | `vault.*` | Self-hosted Bitwarden password manager |
| SilverBullet | `notes.*` | Markdown wiki with backlinks |
| Memos | `memos.*` | Quick-capture notes and journal |
| n8n | `n8n.*` | Workflow automation |
| Actual Budget | `budget.*` | Personal finance and budgeting |
| Excalidraw | `draw.*` | Collaborative virtual whiteboard |
| CouchDB | `couchdb.*` | Database for Obsidian LiveSync |

## Documentation

| Document | Description |
|---|---|
| [docs/dev-local.md](docs/dev-local.md) | Local setup, prerequisites, test checklist |
| [docs/git-workflow.md](docs/git-workflow.md) | Branch naming, commit style, PR process |
| [docs/02-architecture.md](docs/02-architecture.md) | Architecture overview |
| [docs/04-security-model.md](docs/04-security-model.md) | Security model |
| [docs/decisions/](docs/decisions/) | Architecture decision records |
| [infra/oracle/README.md](infra/oracle/README.md) | Production deployment |
| [ROADMAP.md](ROADMAP.md) | Milestone progress |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Security policy |

## Contributing

PRs are required before merging to `main`. See [CONTRIBUTING.md](CONTRIBUTING.md)
and [docs/git-workflow.md](docs/git-workflow.md).

Use Conventional Commit messages: `feat(scope):`, `fix(scope):`, `chore(scope):`, etc.

## License

MIT — see [LICENSE](LICENSE).

<div align="center">

# Private Cloud Gateway

**Your own cloud — one login in front of everything.**

Self-host Nextcloud, Immich, Vaultwarden, Jellyfin and more. Each app runs in its
own private container on its own subdomain, behind a single sign-in with automatic
HTTPS. One small server, entirely yours — no per-app logins, no exposed ports, no
fragile proxy configs to hand-maintain.

[![CI](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/NathanWalash/private-cloud-gateway?sort=semver)](https://github.com/NathanWalash/private-cloud-gateway/releases)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[Quick start](#quick-start) · [How it works](#how-it-works) · [Apps](#built-in-apps) · [Deploy](#deploy-to-production) · [Security](#security) · [Docs](#documentation)

</div>

---

## Why

Self-hosting a handful of apps usually means a pile of exposed ports, a different
password for each service, and a reverse-proxy config you're afraid to touch.
Private Cloud Gateway replaces all of that with **one gateway**: every app sits
behind a single login on a private network, gets its own `app.yourdomain.com`
with an automatic certificate, and is installed from a small YAML "blueprint" in
a couple of clicks. It runs comfortably on one cheap VM — an Oracle Cloud Always
Free box is plenty.

## Highlights

- **One login in front of everything.** Caddy verifies your session before any
  app is reachable; app containers are never exposed directly to the internet.
- **Real authentication.** bcrypt password hashing, TOTP two-factor with backup
  codes, hardened session cookies, and per-IP login rate limiting.
- **Apps in a couple of clicks.** 19 built-in blueprints (Nextcloud, Immich,
  Jellyfin, Vaultwarden, Paperless-ngx, n8n…); adding one is a single YAML file.
- **Hardened by default.** A one-time setup token closes the first-run window,
  HSTS + a full set of security headers, least-privilege containers, and an
  SSRF-guarded uptime monitor.
- **Encrypted, consistent backups.** AES-256-GCM archives (WAL-checkpointed so
  no recent writes are lost) with optional off-site S3/R2 upload.
- **One binary, no runtime deps.** A Go core (pure-Go SQLite, no CGO) with the
  React dashboard embedded — plus Caddy and Docker.
- **Deliberate, versioned deploys.** Production runs a pinned release image and
  only moves when you deploy one, with a pre-deploy backup and auto-rollback.

## Quick start

Requires a Docker runtime (Docker Desktop **or** Rancher Desktop) and Git.

```sh
git clone https://github.com/NathanWalash/private-cloud-gateway.git
cd private-cloud-gateway
make dev-up          # creates .env, builds images, starts the stack
```

Open **`http://home.localtest.me`** — the first-run wizard creates your admin
account. (`*.localtest.me` resolves to `127.0.0.1` via public DNS, so there's no
hosts-file editing; use it rather than `localhost`, which breaks cross-subdomain
cookies.)

```sh
make dev-logs        # tail logs         make dev-ps    # container status
make dev-down        # stop              make dev-nuke  # wipe + rebuild
```

## How it works

```text
Browser
  │  HTTPS (prod) / HTTP (dev)
  ▼
Caddy            ← the only service with host ports (80/443)
  │  forward_auth → core:8080/api/auth/verify   (session valid?)
  ▼  yes → proxy · no → 302 to login
Go Core (:8080)  ← auth, Docker lifecycle, blueprints, backups, API, dashboard
  │  Docker API over the socket
  ▼  private network (never publicly routable)
App containers   (Nextcloud, Immich, Vaultwarden, …)
```

When a request hits `photos.yourdomain.com`, Caddy asks the core to verify the
session cookie. Valid → the request is proxied to the Immich container; invalid →
the browser is redirected to the login page. App containers only ever listen on
the private Docker network — Caddy is the single service bound to host ports.

## Built-in apps

**19 apps, each verified end-to-end** (install → running → routed → healthy → uninstalled):

| App | Subdomain | App | Subdomain |
|---|---|---|---|
| Nextcloud | `cloud.*` | Jellyfin | `media.*` |
| Vaultwarden | `vault.*` | Gitea | `git.*` |
| FreshRSS | `rss.*` | n8n | `n8n.*` |
| Home Assistant | `ha.*` | File Browser | `files.*` |
| Stirling PDF | `pdf.*` | Actual Budget | `budget.*` |
| Uptime Kuma | `status.*` | Memos | `memos.*` |
| SilverBullet | `notes.*` | Excalidraw | `draw.*` |
| CouchDB | `couchdb.*` | IT-Tools | `tools.*` |
| CyberChef | `cyberchef.*` | Adminer | `db.*` |
| Gatus | `gatus.*` | | |

**Coming soon** (listed in the marketplace, not yet installable — they need
multi-container support for their external databases): **Immich**, **Paperless-ngx**,
**Ghost**, **Umami**, **Outline**.

Adding an app is one YAML file in [`blueprints/`](blueprints/) — see
[`blueprints/README.md`](blueprints/README.md) or run the `/new-blueprint`
scaffold. Blueprints support per-app resource limits and container hardening
(`no-new-privileges` is on by default; read-only rootfs and capability drops are
opt-in).

## Deploy to production

One command on a fresh Ubuntu 22.04+ VM (an Oracle Cloud Always Free ARM box
works well):

```sh
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

The installer prompts for your domain and admin email, generates secrets
(including a one-time **setup token** it prints once), and configures systemd and
the firewall. Point `A yourdomain.com` and `A *.yourdomain.com` at the server,
then visit `https://home.yourdomain.com` and complete setup with that token.

Updates are **deliberate and versioned** — production runs a pinned release tag
and only moves when you deploy one (with a pre-deploy backup and automatic
rollback on a failed health check). See [`docs/deployment.md`](docs/deployment.md)
for the dev → prod workflow and [`infra/oracle/README.md`](infra/oracle/README.md)
for VM specifics.

## Security

Designed so a single small VM can face the internet safely.

- **Single exposed service.** Only Caddy binds host ports; every app sits behind
  `forward_auth` on a private Docker network and is never directly reachable.
- **No first-run land-grab.** In production the setup wizard requires a one-time
  token (generated by the installer), so nobody can claim the admin account in
  the window before you finish setup. The core refuses to start if the token is
  missing before first-run.
- **Strong auth.** bcrypt passwords, TOTP 2FA with recovery codes, and per-IP
  login rate limiting. Session cookies are `HttpOnly` / `SameSite=Lax`, and
  `Secure` in production.
- **Defensive HTTP.** HSTS, `X-Frame-Options`, `X-Content-Type-Options`, a strict
  `Referrer-Policy` / `Permissions-Policy`, a same-origin CSP, and a request-body
  size limit — applied to every response.
- **Least-privilege containers.** Core, Caddy, and every app run with
  `no-new-privileges`; Caddy drops all capabilities except `NET_BIND_SERVICE`.
- **No SSRF pivots.** The built-in uptime monitor refuses to fetch loopback,
  private, link-local, or cloud-metadata addresses — checked at creation and
  again at connect time (defeating DNS rebinding).
- **Encrypted backups.** AES-256-GCM archives with PBKDF2, WAL-checkpointed for
  consistency, plus a weak-default-secret nudge on install.
- **Supply chain.** `gosec` and `govulncheck` run in CI alongside the
  race-enabled test suite; Dependabot keeps dependencies current.

> **Single-operator by design.** Every account is full-trust — login is
> authentication, not per-app authorization, so anyone who can sign in can reach
> every app and the dashboard. Don't create an account for someone you wouldn't
> give full control of the server. See
> [`docs/04-security-model.md`](docs/04-security-model.md#user-model-and-trust-boundary).

See [`docs/04-security-model.md`](docs/04-security-model.md). Report
vulnerabilities via [`SECURITY.md`](SECURITY.md).

## Development

```sh
make dev-up          # full stack (rebuilds core + web)
make test            # unit + integration tests
make lint            # markdown / shell / yaml linters
```

For a fast frontend loop, run the core alone and Vite with hot reload — see
[`CLAUDE.md`](CLAUDE.md) for that plus the full architecture, conventions, and
gotchas. The UI follows [`design.md`](design.md). Every PR runs golangci-lint
(incl. `gosec`), `govulncheck`, the race-enabled test suite, the web build, and a
production Docker build.

## Tech stack

Go (standard library, `modernc.org/sqlite`) · React 18 + TypeScript + Vite +
Tailwind · Caddy · Docker · SQLite.

## Documentation

| Doc | About |
|---|---|
| [CLAUDE.md](CLAUDE.md) | Architecture, dev workflow, conventions (start here) |
| [docs/](docs/) | Numbered design docs and decision records |
| [docs/deployment.md](docs/deployment.md) | Dev → prod deploy workflow |
| [design.md](design.md) | Design system and UI direction |
| [ROADMAP.md](ROADMAP.md) | Milestones and progress |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |

## Contributing

Changes land on `main` via pull request with Conventional Commit titles
(`feat(scope):`, `fix(scope):`, …) and green CI. See
[CONTRIBUTING.md](CONTRIBUTING.md) and [docs/git-workflow.md](docs/git-workflow.md).

## License

[MIT](LICENSE)

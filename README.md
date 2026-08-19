<div align="center">

# Private Cloud Gateway

**Your own cloud — one login in front of everything.**

Self-host Nextcloud, Immich, Vaultwarden, Jellyfin and more. Each app gets its
own subdomain and a private container; a single sign-in and automatic HTTPS sit
in front of all of them. One small server, entirely yours.

[![CI](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/NathanWalash/private-cloud-gateway/actions/workflows/ci.yml)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[Quick start](#quick-start) · [How it works](#how-it-works) · [Apps](#built-in-apps) · [Deploy](#deploy-to-production) · [Security](#security)

</div>

---

## Why

Self-hosting usually means a pile of exposed ports, a different login per app,
and fragile reverse-proxy configs. Private Cloud Gateway replaces that with one
gateway: **every app sits behind a single login on a private network**, gets its
own `app.yourdomain.com`, and is installed from a small YAML "blueprint" in a
couple of clicks. It runs comfortably on one cheap VM (an Oracle Cloud Always
Free box is plenty).

## Highlights

- **One login, everything behind it.** Caddy verifies your session before any
  app is reachable — apps are never exposed directly.
- **Real auth.** bcrypt passwords, TOTP two-factor with backup codes, session
  cookies (`HttpOnly`, `SameSite`, `Secure` in production), login rate limiting.
- **Install apps in clicks.** 19 built-in blueprints (Nextcloud, Immich,
  Jellyfin, Vaultwarden, n8n, Paperless-ngx…); adding one is a single YAML file.
- **Encrypted backups.** AES-256-GCM archives with optional off-site S3/R2 upload
  and a one-file "safe escape" download.
- **One binary, no runtime deps.** Go core (pure-Go SQLite, no CGO) with the
  React dashboard embedded — plus Caddy and Docker.
- **Boring on purpose.** Comprehensive tests, `gosec` + `govulncheck` in CI, and
  a one-command installer.

## Screenshots

<!-- Add real screenshots to docs/screenshots/ and reference them here. -->
Run the [quick start](#quick-start) and open `http://home.localtest.me` to see
the dashboard, or the marketplace, app cards, and live status widgets in action.

## Quick start

Requires a Docker runtime (Docker Desktop **or** Rancher Desktop) and Git.

```sh
git clone https://github.com/NathanWalash/private-cloud-gateway.git
cd private-cloud-gateway
make dev-up          # creates .env, builds images, starts the stack
```

Open **`http://home.localtest.me`** in Chrome — the first-run setup wizard
creates your admin account. (`*.localtest.me` resolves to `127.0.0.1` via public
DNS, so no hosts-file editing; use it rather than `localhost`, which breaks
cross-subdomain cookies.)

```sh
make dev-logs        # tail logs        make dev-ps    # container status
make dev-down        # stop             make dev-nuke  # wipe + rebuild
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
session cookie. Valid → the request is proxied to the Immich container. Invalid
→ the browser is redirected to the login page. App containers only ever listen
on the private Docker network.

## Built-in apps

| App | Subdomain | App | Subdomain |
|---|---|---|---|
| Nextcloud | `cloud.*` | Immich | `photos.*` |
| Jellyfin | `media.*` | Vaultwarden | `vault.*` |
| Paperless-ngx | `docs.*` | Gitea | `git.*` |
| Ghost | `blog.*` | FreshRSS | `rss.*` |
| n8n | `n8n.*` | Home Assistant | `home.*` |
| File Browser | `files.*` | Stirling PDF | `pdf.*` |
| Actual Budget | `budget.*` | Uptime Kuma | `status.*` |
| Memos | `memos.*` | SilverBullet | `notes.*` |
| Shiori | `bookmarks.*` | Excalidraw | `draw.*` |
| CouchDB | `couchdb.*` | | |

Adding an app is one YAML file in [`blueprints/`](blueprints/) — see
[`blueprints/README.md`](blueprints/README.md) or run the `/new-blueprint`
scaffold.

## Deploy to production

One command on a fresh Ubuntu 22.04+ VM (Oracle Cloud Always Free ARM works well):

```sh
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

The installer prompts for your domain and admin email, generates secrets, and
configures systemd + the firewall. Point `A yourdomain.com` and
`A *.yourdomain.com` at the server, then visit `https://home.yourdomain.com`.

Updates are **deliberate and versioned** — production runs a pinned release tag
and only moves when you deploy one (with backup + auto-rollback). See
[`docs/deployment.md`](docs/deployment.md) for the dev → prod workflow, and
[`infra/oracle/README.md`](infra/oracle/README.md) for VM specifics.

## Security

- Caddy is the only service with host ports; every app is behind `forward_auth`
  on a private network.
- Passwords hashed with bcrypt; optional TOTP 2FA with recovery codes.
- Session cookies are `HttpOnly` / `SameSite=Lax`, and `Secure` in production;
  logins are rate-limited per IP.
- Backups are AES-256-GCM encrypted; `gosec` and `govulncheck` run in CI.

See [`docs/04-security-model.md`](docs/04-security-model.md). Report
vulnerabilities via [`SECURITY.md`](SECURITY.md).

## Development

```sh
make dev-up          # full stack (rebuilds core + web)
make test            # unit + integration tests
make lint            # markdown / shell / yaml linters
```

For a fast frontend loop, run the core alone and Vite with hot reload — see
[`CLAUDE.md`](CLAUDE.md) for that and the full architecture, conventions, and
gotchas. The UI follows [`design.md`](design.md). CI runs golangci-lint,
`govulncheck`, the race-enabled test suite, the web build, and a Docker build on
every PR.

## Tech stack

Go (standard library, `modernc.org/sqlite`) · React 18 + TypeScript + Vite +
Tailwind · Caddy · Docker · SQLite.

## Documentation

| Doc | About |
|---|---|
| [CLAUDE.md](CLAUDE.md) | Architecture, dev workflow, conventions (start here) |
| [design.md](design.md) | Design system and UI direction |
| [docs/](docs/) | Numbered design docs + decision records |
| [ROADMAP.md](ROADMAP.md) | Milestones and progress |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |

## Contributing

PRs are required to `main`, with Conventional Commit titles
(`feat(scope):`, `fix(scope):`, …). See [CONTRIBUTING.md](CONTRIBUTING.md) and
[docs/git-workflow.md](docs/git-workflow.md).

## License

[MIT](LICENSE)

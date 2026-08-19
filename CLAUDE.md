# CLAUDE.md — Private Cloud Gateway

Guidance for AI agents and humans working in this repo. Keep it accurate; update
it when architecture, invariants, or workflows change.

## What this is

A security-first, self-hosted app platform. One Go binary ("Cloud Core") sits
behind Caddy and puts **every** app behind a single login. Each installed app
gets its own subdomain, a Docker container on a private network, and (in prod)
automatic HTTPS. Everything runs on one VM.

Think "a lean, opinionated Cloudron/Umbrel you actually own."

## Architecture (how a request flows)

```text
Browser
  │  HTTPS (prod) / HTTP (dev)
  ▼
Caddy            ← the ONLY service with host ports (80/443). Public gateway.
  │  forward_auth → GET core:8080/api/auth/verify   (verifies the session cookie)
  ▼  (200 = allowed, proxy on; else 302 → login)
Go Core (:8080) ← auth, Docker lifecycle, blueprints, backups, monitors, API, SPA
  │  Docker REST API over /var/run/docker.sock
  ▼  private bridge network (cloud_core_private) — app containers never public
App containers   (Nextcloud, Immich, Vaultwarden, n8n, …)
```

Caddy config is **regenerated wholesale** on every install/uninstall
(`internal/caddy/manager.go` → `buildCaddyfile` → POST to Caddy admin `/load`),
not patched route-by-route. `home.<domain>` → core; each app subdomain →
`forward_auth` then `reverse_proxy` to the container; a catch-all redirects
unknown subdomains to home.

## Repo layout

```text
apps/core/          Go service (stdlib http, no framework)
  main.go           config from env, wiring, background goroutines
  internal/
    server/         router (net/http mux) + security headers/middleware
    auth/           bcrypt, sessions, TOTP + backup codes, rate limiting
    api/            app lifecycle, backups, monitors, settings, SSE events
    blueprint/      YAML parse/validate + Render(domain, scheme) substitution
    docker/         container lifecycle via Docker REST API over the socket
    caddy/          Caddyfile generation + admin /load reload
    db/             SQLite (modernc, pure-Go, no CGO) schema + migrations
    backup/         AES-256-GCM archives, S3/R2 off-site upload
    notify/         SMTP + webhook notifications
  web/              embedded built SPA (static.go serves web/dist)
apps/web/           Vite + React 18 + TS + Tailwind dashboard (source)
blueprints/         *.yaml app definitions (one file per app)
infra/caddy/        Caddyfile (dev + prod)
infra/docker/       docker-compose.yml (+ .dev.yml override, .prod.yml)
infra/oracle/       systemd unit, firewall, deploy notes
scripts/            dev-up / dev-down / dev-nuke / test / e2e / lint
docs/               numbered design docs + docs/decisions (ADRs)
```

## Running it locally

Requires a Docker runtime (Docker Desktop **or** Rancher Desktop) + Git.

```sh
make dev-up          # creates .env if missing, builds images, starts the stack
# open http://home.localtest.me  → first-run setup wizard
make dev-logs        # tail logs      make dev-ps    # status
make dev-down        # stop           make dev-nuke  # wipe volumes+images, rebuild
```

`*.localtest.me` resolves to `127.0.0.1` via public DNS — **use it, not
`localhost`** (Chrome blocks `Domain=localhost` cookies, which breaks
cross-subdomain sessions). Chrome/Edge need no hosts file.

### Two dev loops
- **Full stack** (`make dev-up`): core + web are baked into the image, so a code
  change means a rebuild. Correct for backend/integration work.
- **Fast frontend loop**: run only the core with its port exposed, then Vite dev
  server with hot reload —
  ```sh
  docker compose -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up core
  cd apps/web && pnpm dev      # http://localhost:5173, proxies /api → core
  ```

## Conventions

- **Git**: branch per change; **PRs required to `main`** (no direct pushes).
  Conventional Commits (`feat(scope):`, `fix(scope):`, `chore(scope):`, …).
- **Go**: standard library first; keep dependencies minimal (currently just
  `x/crypto`, `yaml.v3`, `modernc/sqlite`). `gofmt` is enforced in CI.
- **No CGO** — SQLite is pure-Go so the binary stays statically linked.
- **Tests** live beside code (`_test.go`), run with `-race`. Prefer testing pure
  functions (scoring, rendering, validation) over mocking the world.

## Blueprints (adding an app)

Drop a `blueprints/<id>.yaml` file; it appears in the Install dialog. Schema is
in `docs/06-docker-and-app-blueprints.md`; the struct is in
`internal/blueprint/blueprint.go`. Rules enforced by
`TestBundledBlueprintsAreValid`:

1. **Pin the image to a version** — never `:latest` or a mutable rolling tag
   (`stable`/`release`/`edge`). Reproducible deploys; Dependabot/Renovate bump it.
2. **Never hard-code a domain or scheme.** Use the `${DOMAIN}` and `${SCHEME}`
   placeholders for any env var that embeds the app's own public URL
   (e.g. `NEXTCLOUD_TRUSTED_DOMAINS=cloud.${DOMAIN}`,
   `WEBHOOK_URL=${SCHEME}://n8n.${DOMAIN}/`). They're substituted at install time
   in `Blueprint.Render` using the deployment's real domain and scheme.
3. Required fields: `id`, `name`, `container.image`, `route.subdomain`,
   `route.internal_port`.

## Architecture invariants (do not break)

1. Caddy is the **only** service with host `ports:` — all app containers use
   `expose:` and live on the private network.
2. Every protected subdomain goes through
   `forward_auth core:8080 { uri /api/auth/verify }` before `reverse_proxy`.
3. Session cookie is `HttpOnly`, `SameSite=Lax`, `Domain=<root domain>`.
4. Login is rate-limited (10 attempts / IP / minute).
5. Blueprint env is domain/scheme-agnostic via `${DOMAIN}`/`${SCHEME}` (invariant #2 of blueprints).

## Config (env vars, `CLOUD_CORE_*`)

| Var | Default | Notes |
|---|---|---|
| `CLOUD_CORE_SESSION_SECRET` | required | ≥32 bytes; `openssl rand -hex 32` |
| `CLOUD_CORE_ENV` | `production` | `development` = text logs, HTTP, whoami test route |
| `CLOUD_CORE_COOKIE_DOMAIN` | `localtest.me` | root domain; used for cookies **and** Caddy routes |
| `CLOUD_CORE_LOGIN_URL` | `http://home.localtest.me/login` | absolute URL |
| `CLOUD_CORE_ADMIN_EMAIL` | — | set (with `ENV=production`) to enable Let's Encrypt HTTPS |
| `CLOUD_CORE_BOOTSTRAP_EMAIL` / `_PASSWORD` | — | optional first-run admin; the setup wizard is preferred (leave blank) |
| `CLOUD_CORE_BACKUP_SCHEDULE` | — | e.g. `24h`; empty disables scheduled backups |

Prod vs dev is decided by `ENV=production` **and** `ADMIN_EMAIL` set → HTTPS +
`scheme=https`; otherwise HTTP + `scheme=http`.

## CI (`.github/workflows/ci.yml`, required to merge)

`repo-check` · `markdown-lint` · `shellcheck` · `yaml-lint` ·
`go-lint` (golangci: gofmt, govet, errcheck, staticcheck, gosec, bodyclose, noctx) ·
`go-vuln` (govulncheck) · `go-test` (`-race`) · `node-build` · `go-build` (Docker,
uses the `tester` stage). Tags trigger `release.yml` → image to `ghcr.io`.

## Gotchas

- **Ports 80/443 busy?** Rancher Desktop's built-in Kubernetes/Traefik grabs
  them. Disable it: `rdctl set --kubernetes.enabled=false` (frees the ports; your
  Docker containers are unaffected).
- **Cold-start**: Caddy `depends_on` the core being *healthy*. The core's health
  check has generous slack, but on a very first slow build if Caddy ever aborts,
  a second `docker compose up -d` starts it once the core is healthy.
- **Docker socket**: the core talks to `/var/run/docker.sock`. Docker Desktop and
  Rancher Desktop both provide it inside their VM, so the mount works.
- Use `localtest.me`, never `localhost` (cookie domain, see above).

## Deploying (production)

One command on a fresh Ubuntu 22.04+ VM (Oracle Cloud Always Free ARM works well):

```sh
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

Prompts for domain + admin email, generates secrets, configures systemd + UFW.
Point `A *.yourdomain.com` and `A yourdomain.com` at the VM, then visit
`https://home.yourdomain.com`. See `infra/oracle/README.md`.

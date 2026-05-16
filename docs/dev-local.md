# Local Development

This guide covers everything you need to run Private Cloud Gateway on your local machine.

No application code exists yet. The current focus is Milestone 1: local secured routing.

## Prerequisites checklist

Install all of the following before starting. Run the version commands to verify each tool.

### Required

| Tool | Minimum | Check |
|---|---|---|
| Git | any recent | `git --version` |
| Docker Engine or Docker Desktop | 24+ | `docker --version` |
| Docker Compose (plugin) | 2+ | `docker compose version` |
| Go | 1.22+ | `go version` |
| Node.js LTS | 20+ | `node --version` |
| pnpm or npm | any | `pnpm --version` or `npm --version` |

### Strongly recommended

| Tool | Why | Check |
|---|---|---|
| SQLite CLI | Inspect the control database | `sqlite3 --version` |
| curl | Test API endpoints manually | `curl --version` |
| jq | Format JSON output | `jq --version` |
| Make | Run Makefile targets | `make --version` |

### Optional

| Tool | Why |
|---|---|
| mkcert | Local trusted HTTPS certificates for Caddy |
| Playwright browsers | End-to-end tests (added in a later milestone) |

## First-time setup

```bash
git clone <repo-url>
cd private-cloud-gateway
cp .env.example .env
```

Edit `.env` and at minimum change:

- `CLOUD_CORE_SESSION_SECRET` — generate a safe value with `openssl rand -hex 32`
- `CLOUD_CORE_BOOTSTRAP_PASSWORD` — set a real local dev password

Do not commit `.env`. It is in `.gitignore`.

## Local subdomain routing

Milestone 1 uses localhost subdomains such as `files.localhost` and `home.localhost`.

Some browsers and systems handle `.localhost` subdomains automatically. If yours does not, add entries to your hosts file:

**macOS / Linux** — edit `/etc/hosts`:

```text
127.0.0.1 home.localhost
127.0.0.1 files.localhost
```

**Windows** — edit `C:\Windows\System32\drivers\etc\hosts` as Administrator:

```text
127.0.0.1 home.localhost
127.0.0.1 files.localhost
```

## Local HTTPS (optional for Milestone 1)

Caddy can serve HTTPS locally using a locally trusted certificate from mkcert.

Install mkcert and generate a wildcard cert:

```bash
mkcert -install
mkcert "*.localhost" localhost 127.0.0.1
```

The generated files can be referenced in the local Caddy config. This is optional for Milestone 1, which can run over HTTP.

## Running the stack

The dev stack is managed with Docker Compose. Scripts are stubs until Milestone 1 implementation:

```bash
# When implemented:
./scripts/dev-up.sh     # start all services
./scripts/dev-down.sh   # stop all services
```

Until scripts are implemented, you can use:

```bash
docker compose up       # start all services
docker compose down     # stop all services
```

## Milestone 1 goal

Milestone 1 is the first working version. Success means:

```text
1. Docker Compose starts Caddy, the Go Core service, and one test app.
2. Visiting files.localhost while logged out redirects to the login page.
3. Logging in creates a session cookie.
4. Visiting files.localhost while logged in proxies to the test app container.
5. The test app container has no exposed public host port.
6. Caddy is the only public-facing service.
```

See [ROADMAP.md](../ROADMAP.md) for the full milestone list.

## Project structure

```text
private-cloud-gateway/
├── apps/
│   ├── core/       Go Core backend (auth, Docker, blueprints, backups)
│   └── web/        Vite + TypeScript + Tailwind dashboard
├── blueprints/     YAML app definitions
├── docs/           All project documentation
├── infra/
│   ├── caddy/      Caddy configuration
│   ├── docker/     Docker Compose files
│   └── oracle/     Oracle Cloud production setup
└── scripts/        Developer helper scripts
```

## Useful commands

```bash
# Check all prerequisites at once
git --version && docker --version && docker compose version && go version && node --version && sqlite3 --version && jq --version

# Generate a session secret
openssl rand -hex 32

# Inspect the SQLite control database (when it exists)
sqlite3 ./data/cloud-core.db .tables
```

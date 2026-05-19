# Docker Infrastructure

Docker Compose is the runtime for local development and v1 production.

## Starting the stack

```bash
# From repo root
cp .env.example .env      # first time only — then edit CLOUD_CORE_SESSION_SECRET
docker compose -f infra/docker/docker-compose.yml up --build

# Or use the make target (once dev scripts are implemented):
make dev-up
```

## Services

| Service | Image | Role | Public port |
|---|---|---|---|
| `caddy` | `caddy:2-alpine` | HTTPS gateway, forward-auth | 80, 443 |
| `core` | built from `apps/core/` | Auth, session, API | none (internal 8080) |
| `whoami` | `traefik/whoami` | Milestone 1 test app | none (internal 80) |

## Networking rules

- `caddy` is the **only** service with exposed host ports.
- All other services use `expose:` — reachable inside Docker only.
- The private network is named `cloud_core_private`.
- `core` is on both `public` and `private` networks so Caddy can reach it for forward-auth.
- App containers (`whoami`, future apps) are on `private` only.

## Volumes

| Volume | Purpose |
|---|---|
| `caddy_data` | TLS certificates (empty in local HTTP dev) |
| `caddy_config` | Caddy runtime config |
| `core_data` | SQLite database (`/data/cloud-core.db`) |
| `core_backups` | Encrypted backup archives |

## Milestone 1 verification

After `docker compose up`:

1. Check all services are healthy: `docker compose ps`
2. Visit `http://home.localhost` — you should see the login page.
3. Visit `http://files.localhost` — you should be redirected to login.
4. Log in with your bootstrap credentials.
5. Visit `http://files.localhost` again — you should see the whoami response including `X-Auth-User-Id`.
6. Confirm `whoami` has no exposed host port: `docker compose port whoami 80` should fail.

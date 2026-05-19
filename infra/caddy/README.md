# Caddy Infrastructure

Caddy is the public HTTPS gateway for Private Cloud Gateway.

## Local development

`Caddyfile` is the local development configuration. It uses HTTP only
(`auto_https off`) so no certificates are needed for Milestone 1.

Caddy runs as a container in Docker Compose. It is the only service that
exposes a public port (80).

## How forward-auth works

```text
Browser → files.localhost:80
  ↓
Caddy receives request
  ↓
Caddy calls GET core:8080/api/auth/verify (forwards session cookie)
  ↓
200 OK  → Caddy proxies request to the app container
401     → handle_errors block redirects browser to http://home.localhost/login
```

## Adding a new protected subdomain

1. Add a new entry to `/etc/hosts` (see docs/dev-local.md).
2. Copy the `http://files.localhost` block in `Caddyfile`.
3. Change the subdomain and `reverse_proxy` target.
4. Restart Caddy: `docker compose restart caddy`

## Security rules

- Caddy is the only service that publishes host ports (`80`, `443`).
- App containers use `expose:` (container-internal only) — never `ports:`.
- The Caddy admin API (`localhost:2019`) must never be publicly reachable.
- In production, Caddy handles TLS termination and wildcard certificates.

## Production

The production Caddyfile will be added in Milestone 5. It will use:

- Automatic HTTPS via Let's Encrypt
- Wildcard subdomain (`*.yourdomain.com`)
- HTTPS redirect for all HTTP traffic

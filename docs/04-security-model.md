# Security Model

## Security principle

Cloud Core protects private apps by placing them behind a single authenticated HTTPS ingress.

## Public surface

Only Caddy should be public.

Allowed public ports:

```text
80/tcp
443/tcp
```

SSH should be restricted where possible.

## App exposure rule

Hosted apps must not publish public host ports.

Correct:

```yaml
expose:
  - "8080"
networks:
  - cloud_core_private
```

Incorrect:

```yaml
ports:
  - "8080:8080"
```

## Authentication flow

```text
Request to protected subdomain
  ↓
Caddy calls Cloud Core /api/auth/verify
  ↓
Cloud Core validates session cookie
  ↓
Valid: allow proxy
  ↓
Invalid: redirect to login
```

## Session requirements

- HTTP-only cookies.
- Secure cookies in production.
- SameSite=Lax or Strict.
- Short-lived session with refresh strategy.
- Server-side session invalidation.
- Password hashing with Argon2id or bcrypt.

## Sensitive operations

Require authenticated admin session:

- install app
- remove app
- start/stop/restart app
- change route
- change domain
- create backup
- restore backup
- change backup settings
- view logs

## Audit log

Cloud Core should record:

- login success/failure
- app lifecycle actions
- route changes
- backup actions
- restore actions
- settings changes

## Backup security

Backups must be encrypted before leaving the server.

The backup password/key must not be stored in plaintext.

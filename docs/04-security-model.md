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

## User model and trust boundary

**Every account is full-trust. This is a single-operator system.**

`forward_auth` performs *authentication*, not *authorization*: `/api/auth/verify`
returns success for any valid session, regardless of which app subdomain is being
requested. There are no per-user or per-app access controls. Consequently:

- **Any user who can log in can reach every installed app and the full dashboard**
  (install/remove apps, backups, settings, monitors).
- The intended deployment is a single owner (or a small group who all fully trust
  each other and are all effectively admins).

Do **not** create an account for someone you would not give complete control of
the server and every hosted app. Per-user authorization / multi-tenancy is out of
scope for v1; if it is ever added, `Verify` must take the target host into an
access decision and the `X-Auth-User-ID` header must be consumed for authz.

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

# Security Policy

## Core security rule

Caddy is the only public entrypoint.

Hosted apps must not publish public host ports directly.

Use private Docker networking:

```yaml
expose:
  - "8080"
networks:
  - cloud_core_private
```

Avoid:

```yaml
ports:
  - "8080:8080"
```

## Authentication model

All protected app routes use the same auth gate.

```text
Request to files.example.com
  ↓
Caddy receives request
  ↓
Caddy calls Cloud Core /api/auth/verify
  ↓
Cloud Core validates session cookie
  ↓
Valid: proxy to private container
  ↓
Invalid: redirect to login
```

## Minimum security requirements

- HTTP-only secure session cookies.
- SameSite cookie policy.
- Password hashing with Argon2id or bcrypt.
- Private Docker network for apps.
- No direct app host ports.
- Encrypted backups.
- Audit log for sensitive actions.
- Minimal firewall rules.
- Admin setup token for first install.

## Sensitive actions

These should be logged:

- Login success/failure.
- App install/remove.
- App start/stop/restart.
- Backup created.
- Restore started.
- Settings changed.
- Domain/routing changed.

## Secrets

Do not commit:

- `.env`
- private keys
- backup passwords
- Oracle credentials
- Cloudflare tokens
- production Caddy data

# Testing Strategy

## Testing goals

Cloud Core must prove:

- auth protects apps
- app routes work
- Docker lifecycle works
- blueprints parse correctly
- backups are valid
- restore is possible
- dashboard flows work

## Unit tests

Test small logic:

- auth/session logic
- password hashing
- blueprint parser
- route generator
- lifecycle state machine
- backup planner
- settings validation

## Integration tests

Test real components together:

- Go Core + SQLite
- Go Core + Docker API
- Go Core + Caddy forward-auth
- blueprint install flow
- app health checks
- backup archive creation

Recommended tool:

```text
Testcontainers
```

## End-to-end tests

Test browser flows:

- login
- logout
- dashboard loads
- app card opens protected route
- logged-out user is redirected
- backup now action starts
- settings save correctly

Recommended tool:

```text
Playwright
```

## Load/performance tests

Not required immediately, but useful later:

- dashboard API load
- Caddy route performance
- wake-on-demand behaviour
- backup performance

Recommended tool:

```text
k6
```

## Minimum v1 test checklist

- [ ] Unauthenticated app route redirects to login.
- [ ] Authenticated app route proxies correctly.
- [ ] App install from blueprint works.
- [ ] App start/stop works.
- [ ] SQLite migrations run.
- [ ] Backup archive is created.
- [ ] Backup archive passes checksum validation.
- [ ] Dashboard login flow works in browser.

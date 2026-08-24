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

## Coverage

Two layers of tests cover different kinds of code, and they must be read
together:

- **Unit tests** cover pure logic cheaply: blueprint validation, TOTP and
  backup-code math, DB queries, Caddy config generation, server middleware.
  These packages sit at 75–90%.
- **The E2E suite** (real Docker + a live stack) covers the wiring that unit
  tests can't reach without a daemon and a running server: the HTTP handlers,
  the Docker lifecycle, backup create/restore. Measured in isolation with
  `go test ./...`, these packages look thin (`api`, `auth`, `docker` ~15–25%)
  because a plain unit-test run never counts what the E2E exercises.

`scripts/coverage.sh` produces the honest combined figure. It runs the unit
tests and a coverage-instrumented build of the core (`Dockerfile` `COVERPKG`
arg + `infra/docker/docker-compose.coverage.yml`) through the E2E suite, then
merges both covdata sets:

```bash
scripts/coverage.sh                 # unit + Go E2E (default: excalidraw)
E2E_APPS=excalidraw,umami scripts/coverage.sh
COVERAGE_UNIT_ONLY=1 scripts/coverage.sh
```

The instrumented binary flushes coverage to `$GOCOVERDIR` on graceful shutdown
(the core already handles SIGTERM), which `compose down` triggers. Output lands
in `.coverage/coverage.txt` (open with `go tool cover -html`).

Counting the E2E run tells a very different story from unit-only — for example
`api` 17%→38%, `auth` 17%→49%, `docker` 12%→44% with a single app installed;
more apps in `E2E_APPS` raise it further. The number is intentionally
concentrated: logic gets unit tests, wiring gets E2E.

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

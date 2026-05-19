# Roadmap

## Milestone 1: Local secured routing ✅

Goal: prove that one login protects both the dashboard and app subdomains.

Tasks:

- [x] Create Docker Compose dev environment.
- [x] Run Caddy locally.
- [x] Run Go Core (Go, SQLite, bcrypt, sessions).
- [x] Add SQLite database and migrations.
- [x] Add login endpoint with bcrypt password hashing.
- [x] Add session cookie (HttpOnly, SameSite=Lax, cross-subdomain domain).
- [x] Add Caddy forward-auth check.
- [x] Protect one test app behind auth.
- [x] Confirm unauthenticated requests redirect to login.
- [x] Unit and integration test suite (13 tests, race detector).
- [x] CI pipeline (lint, vuln scan, tests, Docker build).

Success criteria:

```text
files.localtest.me redirects to login when logged out. ✅
files.localtest.me opens the app when logged in. ✅
```

## Milestone 2: Dashboard foundation

Goal: create the first useful private dashboard.

Tasks:

- [ ] Build login page.
- [ ] Build dashboard layout.
- [ ] Add app cards.
- [ ] Add custom links.
- [ ] Add server status widgets.
- [ ] Add backup status widget placeholder.
- [ ] Add theme settings.
- [ ] Add responsive mobile layout.

Success criteria:

```text
User logs in and sees a polished dashboard with apps, links, and status cards.
```

## Milestone 3: App blueprints

Goal: install and manage apps from YAML blueprints.

Tasks:

- [ ] Define blueprint schema.
- [ ] Parse blueprint YAML.
- [ ] Validate blueprint fields.
- [ ] Create Docker container/network/volumes from blueprint.
- [ ] Register app route in Caddy.
- [ ] Show installed app on dashboard.
- [ ] Add start/stop/restart controls.
- [ ] Add app health checks.

Success criteria:

```text
A new app can be added by placing a YAML file in /blueprints and clicking Install.
```

## Milestone 4: Backup and restore

Goal: make data loss unlikely and recovery realistic.

Tasks:

- [ ] Define backup manifest format.
- [ ] Back up SQLite database.
- [ ] Back up app blueprints.
- [ ] Back up app volumes.
- [ ] Back up dashboard settings.
- [ ] Encrypt backup archive.
- [ ] Add "Backup now" action.
- [ ] Add "Safe Escape" local download.
- [ ] Add restore command.
- [ ] Add restore test/check command.

Success criteria:

```text
A fresh server can be restored from a backup archive.
```

## Milestone 5: Oracle Cloud deployment

Goal: deploy to a fresh Oracle Cloud Ubuntu VM.

Tasks:

- [ ] Write install.sh.
- [ ] Create production Docker Compose file.
- [ ] Create production Caddy config.
- [ ] Create systemd service.
- [ ] Create firewall setup.
- [ ] Document DNS setup.
- [ ] Document persistent volume layout.
- [ ] Test on fresh Oracle VM.

Success criteria:

```text
Fresh Oracle Ubuntu VM + one install command + DNS setup = working Private Cloud Gateway dashboard.
```

## Milestone 6: Polish and hardening

Goal: make it reliable and pleasant.

Tasks:

- [ ] Improve UI/UX.
- [ ] Add logs viewer.
- [ ] Add app update flow.
- [ ] Add alerting.
- [ ] Add API status widgets.
- [ ] Add better error states.
- [ ] Add audit log.
- [ ] Add backup health warnings.
- [ ] Add app catalogue.

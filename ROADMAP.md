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

## Milestone 2: Dashboard foundation ✅

Goal: create the first useful private dashboard.

Tasks:

- [x] Login page (dark theme, Lucide icons).
- [x] Dashboard layout with app cards grid and sidebar widgets.
- [x] Server status widget (uptime, version).
- [x] Responsive layout.
- [x] First-run setup wizard (name, email, password — no .env bootstrap required).

Success criteria:

```text
User logs in and sees a polished dashboard with apps, links, and status cards. ✅
```

## Milestone 3: App blueprints ✅

Goal: install and manage apps from YAML blueprints.

Tasks:

- [x] Blueprint YAML schema and parser.
- [x] Docker lifecycle manager (install, start, stop, restart, remove).
- [x] Caddy dynamic route registration via /load API.
- [x] Install dialog showing available blueprints.
- [x] App cards with status badges and lifecycle controls.
- [x] 11-app blueprint catalogue (Stirling PDF, Vaultwarden, SilverBullet, Uptime Kuma, etc).
- [x] Route re-registration on Core startup.

Success criteria:

```text
A new app can be added by clicking Install and selecting a blueprint. ✅
```

## Milestone 4: Backup and restore 🔜

Goal: make data loss unlikely and recovery realistic.

Tasks:

- [x] Backup manifest format (JSON).
- [x] Back up SQLite database and blueprints into an encrypted zip archive.
- [x] AES-256-GCM encryption with PBKDF2 key derivation.
- [x] "Backup now" button on dashboard.
- [x] "Safe Escape" direct browser download.
- [x] Backup list with timestamps and file sizes.
- [ ] App volume backup (Docker volume snapshot).
- [ ] Restore from backup archive.
- [ ] Scheduled automatic backups.

Success criteria:

```text
A fresh server can be restored from a backup archive.
```

## Milestone 5: Oracle Cloud deployment 🔜

Goal: deploy to a fresh Oracle Cloud Ubuntu VM.

Tasks:

- [x] Write `install.sh` — one-command installer for Ubuntu 22.04+.
- [x] Create production Docker Compose (`docker-compose.prod.yml`).
- [x] Create production Caddyfile with HTTPS + Let's Encrypt.
- [x] Create systemd service (`pcg.service`).
- [x] Create firewall setup script (UFW, ports 22/80/443 only).
- [x] Document DNS setup (wildcard A records).
- [x] Production HTTPS mode in Go Core (caddy.NewProduction).
- [ ] End-to-end test on fresh Oracle VM.
- [ ] GitHub release with versioned image.

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

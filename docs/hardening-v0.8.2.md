# Hardening plan — v0.8.2 (pre-internet-deployment)

Findings from the final code + security + deployment review (2026-08), verified
against the code. Three tiers by urgency. Tier 1 blocks the first internet
deploy; Tier 2 is before relying on features / real exposure; Tier 3 is polish.

Each item: **problem**, **fix**, **acceptance check**.

---

## Tier 1 — fix before any internet deploy

### T1.1 (H1) Isolate the Caddy admin API from app containers

- **Problem:** `infra/caddy/Caddyfile.prod` sets `admin :2019` (all interfaces),
  Caddy sits on `cloud_core_private`, and every app container joins that same
  network. Any compromised/malicious app image can `POST caddy:2019/load` and
  rewrite the gateway (strip `forward_auth`, expose the dashboard, proxy
  anywhere). Full gateway takeover from a single bad app image.
- **Fix:** Move the Caddy admin endpoint off any network apps can reach. Preferred:
  bind admin to a Unix socket on a volume shared only between `core` and `caddy`,
  and have the Go caddy manager dial that socket instead of `caddy:2019`. Fallback
  if socket is impractical: give Caddy a static IP on the `public` (core↔caddy)
  network and bind `admin <that-ip>:2019` so the private (apps) subnet can't route
  to it.
- **Acceptance:** From an app container on `cloud_core_private`, `caddy:2019` is
  unreachable; Core can still push config; installing/routing an app still works
  (E2E green).

### T1.2 (B1) Installer health check must probe the container, not the host

- **Problem:** `install.sh:167,173` curl `http://localhost:8080/healthz`, but core
  only `expose`s 8080 on the Docker network (never published to the host). The
  check can never pass — always prints "may still be starting" and masks real
  boot failures.
- **Fix:** Check the core container's health (`docker inspect --format
  '{{.State.Health.Status}}'`, as `deploy.sh` already does) or curl Caddy on `:80`.
- **Acceptance:** On a healthy stack the installer prints success; on a dead core
  it reports failure.

### T1.3 (B2) `deploy.sh` must un-shallow before checking out a release tag

- **Problem:** `install.sh:90` clones `--depth 1`; `deploy.sh` does `git fetch
  --tags` then `git checkout <tag>` with no `--unshallow`, which can fail for lack
  of objects on the first real deploy.
- **Fix:** Add `git -C "$INSTALL_DIR" fetch --unshallow 2>/dev/null || true`
  before the tag fetch in deploy.sh.
- **Acceptance:** `deploy.sh v0.8.2` checks out cleanly on a `--depth 1` clone.

### T1.4 (C2) Extend the write deadline on backup handlers

- **Problem:** `BackupCreate` and `SafeEscape` (`internal/api/backup.go`) do not
  call `extendWriteDeadline(w)` (unlike `Install`/`UpdateApp`). Large backups
  (volumes + pg_dump) exceed the 30s `WriteTimeout` → client sees failure though
  the backup was written.
- **Fix:** Call `extendWriteDeadline(w)` at the top of both handlers.
- **Acceptance:** A long backup returns 2xx to the client.

### T1.5 (C1) Restore must not overwrite the live SQLite DB underneath the server

- **Problem:** `backup.Restore` truncates and rewrites `cloud-core.db` in place
  while the server holds it open in WAL mode; a background write mid-copy can
  corrupt the DB.
- **Fix:** Write the restored DB to a sidecar path and atomically stage it so it
  is adopted on the next start (the handler already tells the user to restart), or
  quiesce writers + checkpoint + close before overwriting. Treat a missing/failed
  `cloud-core.db` archive entry as a hard error (also fixes C-review #5).
- **Acceptance:** Restore never writes the live DB file in place; a partial
  archive returns an error, not a false success; existing restore round-trip
  tests still pass.

---

## Tier 2 — before relying on features / real exposure

### T2.1 (H2) Re-authenticate before 2FA enrol/disable and password change

- **Problem:** `TOTPSetup`/`TOTPConfirm`/`TOTPDisable` require only a session; a
  stolen shared-domain cookie can enrol the attacker's authenticator.
- **Fix (adopted):** `TOTPConfirm` (the step that persists a new secret) now
  requires the account password, verified with bcrypt, before enrolling. Disable
  already requires a current TOTP code; password change already requires the
  current password — both left as-is. Frontend 2FA setup collects the password.
- **Acceptance:** enrol fails (401) without the correct account password, even
  with a valid code.

### T2.2 (M3) SSRF guard on webhook (and Telegram/SMTP host)

- **Problem:** `WEBHOOK_URL` is POSTed with no scheme/host validation or blocked-IP
  dialer; an authed user can hit `caddy:2019`/metadata. Compounds with H1.
- **Fix:** Route webhook sends through the monitor's blocked-IP-guarded dialer, or
  validate `WEBHOOK_URL` on `PutSetting` the way monitor URLs are validated.
- **Acceptance:** A webhook URL resolving to loopback/private/link-local/metadata
  is rejected or refused at dial time.

### T2.3 (C3) Render `${DOMAIN}`/`${SCHEME}` in sidecar service env

- **Problem:** `Blueprint.Render` substitutes only `Container.Environment`, not
  `Services[].Environment` → sidecars get literal placeholders.
- **Fix:** Apply the same replacer over each rendered service's environment.
- **Acceptance:** A sidecar env using `${DOMAIN}` is substituted; unit test added.

### T2.4 (B3) `deploy.sh` re-syncs the production Caddyfile

- **Problem:** deploy.sh never copies `Caddyfile.prod` to the live mount path, so
  bootstrap-Caddyfile fixes in new releases never land.
- **Fix:** After checkout, `cp infra/caddy/Caddyfile.prod caddy/Caddyfile`.
- **Acceptance:** A changed Caddyfile.prod reaches the running Caddy after deploy.

### T2.5 (M2) Restore: drop form-supplied passphrase override; audit-log restores

- **Problem:** Restore accepts a form passphrase, letting an authed user restore
  an attacker-authored unencrypted archive that replaces the auth DB.
- **Fix (adopted):** write a prominent audit-log entry (`backup.restored`) and a
  WARN log on every restore. Kept the form-supplied passphrase: dropping it would
  break the legitimate case of restoring an archive encrypted with a passphrase
  different from the server's current env value. In the single-trusted-user model
  a restore is already an authenticated admin action; traceability is the win.
- **Acceptance:** every restore writes a `backup.restored` audit row.

### T2.6 (C4) SSE handlers respect shutdown

- **Problem:** `AppEvents`/`LogsStream` only return on request-context done, so
  `srv.Shutdown` hangs the full 30s on every restart.
- **Fix:** Select on a server-wide shutdown signal (derive from `bgCtx`) in the
  stream loops.
- **Acceptance:** Restart with an open SSE stream shuts down promptly.

---

## Tier 3 — polish / post-launch

- **T3.1** PBKDF2 iterations 100k → 600k (store count in the archive header for
  forward-compat); or migrate to argon2id. `internal/backup/backup.go`.
- **T3.2** Backup codes: `bcrypt.MinCost` → `DefaultCost`. `auth/backup_codes.go`.
- **T3.3** Tighten `X-Forwarded-For` trust from `172.16.0.0/12` to the
  caddy↔core subnet. `auth/ratelimit.go`.
- **T3.4** Per-account login-failure lockout + wire the `login.fail` notification.
- **T3.5** Send a baseline CSP unconditionally on HTML responses. `server/server.go`.
- **T3.6** `install.sh` preflight: verify DNS resolves to this host and warn
  loudly (blocking gate) about the Oracle VCN 80/443 ingress before starting.
- **T3.7** `install.sh` optional swapfile as OOM insurance on small VMs.
- **T3.8** `install.sh` resolve the release tag with `jq`, and fail loudly instead
  of falling back to `:latest`.
- **T3.9** Document the memory cost of large restores (`io.ReadAll` of the whole
  archive); consider streaming if it becomes a problem.
- **T3.10** `adoptRestoredDB`: run `PRAGMA integrity_check` on the staged DB
  before deleting the live file, so a corrupt staged copy can't leave the service
  unbootable. (Staged writes are already atomic as of v0.8.2, so a *partial*
  `.restored` can't occur; this guards against a semantically-corrupt archive.)

---

## Delivery

- Tier 1 → **shipped in v0.8.2** (#79).
- Tier 2 → **shipped in v0.8.2** (#80).
- Tier 3 → **shipped in v0.8.3** (this batch): T3.1–T3.10 all done.

Recommended before the real domain: a dry-run deploy on a scratch VM with Let's
Encrypt **staging**.

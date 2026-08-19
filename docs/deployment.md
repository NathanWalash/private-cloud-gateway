# Deployment (dev → production)

A lightweight but real dev/prod split for a solo developer. Nothing auto-deploys;
production only moves when you deliberately deploy a pinned release.

## The model

```text
Local (laptop)     main branch        Releases (tags)     Production (VM)
make dev-up    →   CI-gated PRs   →   vX.Y.Z image   →    runs a PINNED tag
localtest.me       "ready" code       in ghcr.io          moves only on deploy
```

- **Local** is your dev + test environment (`make dev-up` → `http://home.localtest.me`).
- **`main`** is CI-gated; merges are your "ready" line.
- **Releases** (`vX.Y.Z` tags) build a versioned image to `ghcr.io` via `release.yml`.
- **Production** runs a **pinned release tag**, never `:latest` or `main`. It only
  changes when you run a deploy, which backs up, switches, health-checks, and
  auto-rolls-back on failure.

There is no separate staging box — for one developer, local *is* staging.

## One-time production setup

1. **Provision a VM** (Oracle Cloud Always Free ARM, or any Ubuntu 22.04+ VPS).
2. **DNS**: point `A yourdomain.com` and `A *.yourdomain.com` at the VM's IP.
3. **Install**:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
   ```

   It prompts for your domain + admin email, generates secrets, and sets up
   Caddy (Let's Encrypt HTTPS), systemd, and the firewall.
4. **First deploy** to pin a version:

   ```sh
   sudo /opt/pcg/scripts/deploy.sh v0.5.0
   ```

## Deploying updates

**Preferred — the one-click Action:** GitHub → **Actions → Deploy to production →
Run workflow**, enter the tag (e.g. `v0.6.0`). It SSHes to the VM and runs the
deploy for you.

**Or by hand on the VM:**

```sh
sudo /opt/pcg/scripts/deploy.sh v0.6.0
```

Either way the deploy:

1. Snapshots `cloud-core.db` into `backups/`.
2. Checks out the tag (syncs blueprints/Caddyfile) and pins the image.
3. Pulls the release image and recreates the stack.
4. Health-checks the core; **rolls back to the previous version if it fails**.

## Rolling back

```sh
sudo /opt/pcg/scripts/rollback.sh          # last known-good
sudo /opt/pcg/scripts/rollback.sh v0.5.0   # a specific tag
```

## Wiring the Action (one-time secrets)

Add these repo secrets (Settings → Secrets and variables → Actions) once the VM
exists:

| Secret | Value |
|---|---|
| `DEPLOY_SSH_HOST` | VM IP or hostname |
| `DEPLOY_SSH_USER` | SSH user (with `sudo` rights to run the deploy script) |
| `DEPLOY_SSH_KEY` | Private key whose public half is in the VM's `authorized_keys` |

## Typical loop

1. Develop locally (`make dev-up`), commit on a branch, open a PR — CI gates it.
2. Merge to `main`.
3. When confident, cut a release: tag `vX.Y.Z` and push it (→ image built to ghcr).
4. Deploy that tag via the Action (or `deploy.sh`) when *you* choose.
5. If something's wrong, `rollback.sh`.

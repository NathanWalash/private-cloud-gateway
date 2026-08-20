# Scripts

Operational scripts for local development and production deploys.

## Development

- `dev-up.sh` — start the local development stack.
- `dev-down.sh` — stop the local development stack.
- `test.sh` — run tests.
- `lint.sh` — run format/lint checks.
- `e2e-local.sh` — run the full end-to-end suite (Go container-lifecycle +
  Playwright browser click-through) against a running dev stack. Slow; this is
  the per-release confidence check, not a per-PR gate.

## Production deploy

Run on the VM; see [`docs/deployment.md`](../docs/deployment.md).

- `deploy.sh` — deploy a pinned release tag with a pre-deploy backup, health
  check, and automatic rollback on failure.
- `rollback.sh` — roll back to the previously deployed tag.

## Backups

Backup and restore are handled by the running app, not by shell scripts —
archives are AES-256-GCM encrypted and (de)encrypted by the Go core.

- Create/restore from the dashboard: **Settings → Backup**.
- Scheduled backups run automatically (`CLOUD_CORE_BACKUP_SCHEDULE`, default 24h).
- `backup-now.sh` / `restore.sh` print these instructions.

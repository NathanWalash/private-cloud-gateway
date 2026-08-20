#!/usr/bin/env bash
set -euo pipefail

# On-demand backups are created by the Go core (it archives the database,
# blueprints, and app volumes, then encrypts with AES-256-GCM).
#
# Create one from the dashboard:  Settings → Backup → Create backup
# Scheduled backups run automatically per CLOUD_CORE_BACKUP_SCHEDULE (default 24h).

cat <<'EOF'
Backups are created from the dashboard, not this script.

  • On demand:  Settings → Backup → Create backup
  • Scheduled:  automatic, controlled by CLOUD_CORE_BACKUP_SCHEDULE in .env
                (default 24h; set CLOUD_CORE_BACKUP_PASSPHRASE to encrypt).

Backups are written to the backups/ volume (/backups in the container).
EOF

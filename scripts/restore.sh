#!/usr/bin/env bash
set -euo pipefail

# Backup archives are AES-256-GCM encrypted and are restored by the Go core
# (which holds the decryption logic), not by this shell script.
#
# Restore from the dashboard:  Settings → Backup → Restore
# (upload the .pcg-backup archive and enter its passphrase).

cat <<'EOF'
Restore is performed from the dashboard, not this script.

  1. Open  https://home.<your-domain>/  and sign in.
  2. Go to Settings → Backup → Restore.
  3. Upload your .pcg-backup archive and enter its passphrase.
  4. The server restores the database + blueprints, then restart it:
       sudo systemctl restart pcg      # (or: docker compose ... restart core)

Archives are AES-256-GCM encrypted, so a plain shell restore is not possible.
EOF

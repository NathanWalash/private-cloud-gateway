#!/usr/bin/env bash
# Roll production back to the last known-good release, or to an explicit tag.
#
#   sudo ./scripts/rollback.sh            # last known-good deploy
#   sudo ./scripts/rollback.sh v0.5.0     # a specific tag
set -euo pipefail

INSTALL_DIR="${PCG_INSTALL_DIR:-/opt/pcg}"
target="${1:-}"
if [ -z "$target" ]; then
  target="$(cat "$INSTALL_DIR/.last-good-version" 2>/dev/null || true)"
fi
if [ -z "$target" ]; then
  echo "no tag given and no .last-good-version recorded" >&2
  exit 2
fi

echo "==> Rolling back to $target"
exec "$INSTALL_DIR/scripts/deploy.sh" "$target"

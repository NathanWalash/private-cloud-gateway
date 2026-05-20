#!/usr/bin/env bash
# shellcheck disable=SC2015
# Completely wipe the local dev environment and rebuild from scratch.
# Removes:
#   - All compose-managed containers (caddy, core, whoami)
#   - All compose-managed volumes (SQLite DB, Caddy certs, backups)
#   - All app containers installed via Go Core (pcg-*)
#   - All named Docker volumes created by Go Core for apps (pcg-*)
#   - The built Docker image so it always rebuilds from source
#
# Use after a PR merge to start completely fresh with no leftover state.
#
# Usage:
#   ./scripts/dev-nuke.sh           # wipe + rebuild + start
#   ./scripts/dev-nuke.sh --wipe    # wipe only (don't rebuild)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/infra/docker/docker-compose.yml"
WIPE_ONLY=false

for arg in "$@"; do
  [ "$arg" = "--wipe" ] && WIPE_ONLY=true
done

echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║   Private Cloud Gateway — Dev Nuke               ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""

# ── 1. Stop and remove compose-managed services ───────────────────────────────
echo "→ Stopping compose services..."
docker compose -f "$COMPOSE_FILE" down --volumes --remove-orphans 2>/dev/null || true

# ── 2. Remove app containers installed by Go Core ─────────────────────────────
echo "→ Removing pcg-* app containers..."
docker ps -aq --filter "label=pcg.managed=true" 2>/dev/null \
  | xargs -r docker stop 2>/dev/null || true
docker ps -aq --filter "label=pcg.managed=true" 2>/dev/null \
  | xargs -r docker rm  2>/dev/null || true
# Also catch any pcg-* containers not labelled (e.g. from older runs)
docker ps -aq --filter "name=pcg-" 2>/dev/null \
  | xargs -r docker stop 2>/dev/null || true
docker ps -aq --filter "name=pcg-" 2>/dev/null \
  | xargs -r docker rm  2>/dev/null || true

# ── 3. Remove named volumes created for app data ──────────────────────────────
echo "→ Removing pcg-* named volumes..."
VOLUMES=$(docker volume ls -q --filter "name=pcg-" 2>/dev/null || true)
if [ -n "$VOLUMES" ]; then
  echo "$VOLUMES" | xargs -r docker volume rm 2>/dev/null || true
  echo "  Removed: $(echo "$VOLUMES" | wc -w | tr -d ' ') volume(s)"
else
  echo "  None found"
fi

# ── 4. Remove the built images so Docker rebuilds from source ─────────────────
echo "→ Removing built images..."
docker rmi private-cloud-gateway-core 2>/dev/null || true
docker rmi private-cloud-gateway_core 2>/dev/null || true

# ── 5. Check .env exists ──────────────────────────────────────────────────────
if [ ! -f "$REPO_ROOT/.env" ]; then
  echo ""
  echo "  ⚠  No .env found. Creating from .env.example..."
  cp "$REPO_ROOT/.env.example" "$REPO_ROOT/.env"
  echo "  Edit $REPO_ROOT/.env — set CLOUD_CORE_SESSION_SECRET and bootstrap credentials."
  WIPE_ONLY=true  # Don't start without a proper .env
fi

echo ""
echo "✓ Wipe complete."
echo ""

[ "$WIPE_ONLY" = true ] && exit 0

# ── 6. Rebuild and start ──────────────────────────────────────────────────────
echo "→ Rebuilding and starting fresh stack..."
docker compose -f "$COMPOSE_FILE" up --build -d

echo ""
echo "✓ Stack is running. Open http://home.localtest.me"
echo ""

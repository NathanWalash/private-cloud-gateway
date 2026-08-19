#!/usr/bin/env bash
# Deploy a pinned release to production.
#
# Backs up the control-plane DB, switches prod to the given release tag,
# health-checks the core, and rolls back automatically if it fails to come up.
# Run on the production VM (usually via the "Deploy to production" GitHub Action):
#
#   sudo ./scripts/deploy.sh v0.6.0
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: deploy.sh <release-tag>   e.g. deploy.sh v0.6.0" >&2
  exit 2
fi

INSTALL_DIR="${PCG_INSTALL_DIR:-/opt/pcg}"
COMPOSE_FILE="$INSTALL_DIR/infra/docker/docker-compose.prod.yml"
ENV_FILE="$INSTALL_DIR/.env"

compose() { docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"; }

cd "$INSTALL_DIR"

previous="$(grep -E '^PCG_VERSION=' "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)"
echo "==> Deploying $VERSION (current: ${previous:-unset})"

git fetch --tags --force --quiet origin
if ! git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
  echo "error: release tag '$VERSION' not found" >&2
  exit 1
fi

# Safety net: snapshot the control-plane database before touching prod.
if [ -f "$INSTALL_DIR/data/cloud-core.db" ]; then
  ts="$(date +%Y%m%d-%H%M%S)"
  cp "$INSTALL_DIR/data/cloud-core.db" "$INSTALL_DIR/backups/pre-deploy-$ts-cloud-core.db"
  echo "==> Backed up cloud-core.db"
fi

set_version() {
  if grep -qE '^PCG_VERSION=' "$ENV_FILE"; then
    sed -i "s|^PCG_VERSION=.*|PCG_VERSION=$1|" "$ENV_FILE"
  else
    echo "PCG_VERSION=$1" >>"$ENV_FILE"
  fi
}

apply() {
  git checkout -q "$1"
  set_version "$1"
  compose pull core
  compose up -d
}

apply "$VERSION"

echo "==> Waiting for core to become healthy..."
cid="$(compose ps -q core)"
healthy=false
for _ in $(seq 1 30); do
  status="$(docker inspect --format '{{.State.Health.Status}}' "$cid" 2>/dev/null || echo starting)"
  if [ "$status" = "healthy" ]; then
    healthy=true
    break
  fi
  sleep 3
done

if [ "$healthy" = true ]; then
  echo "$VERSION" >"$INSTALL_DIR/.last-good-version"
  echo "==> Deployed $VERSION successfully."
else
  echo "!! $VERSION did not become healthy within 90s." >&2
  if [ -n "$previous" ] && git rev-parse -q --verify "refs/tags/$previous" >/dev/null 2>&1; then
    echo "==> Rolling back to $previous..." >&2
    apply "$previous"
    echo "==> Rolled back to $previous." >&2
  fi
  exit 1
fi

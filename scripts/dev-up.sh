#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/infra/docker/docker-compose.yml"
ENV_FILE="$REPO_ROOT/.env"

# First run: create .env from the example with a freshly generated session
# secret so the stack starts with a single command. Admin is created via the
# in-app setup wizard, so bootstrap creds are left blank.
if [ ! -f "$ENV_FILE" ]; then
  echo "No .env found — creating one from .env.example with a generated session secret."
  cp "$REPO_ROOT/.env.example" "$ENV_FILE"
  secret="$(openssl rand -hex 32)"
  tmp="$(mktemp)"
  sed \
    -e "s|^CLOUD_CORE_SESSION_SECRET=.*|CLOUD_CORE_SESSION_SECRET=${secret}|" \
    -e "s|^CLOUD_CORE_BOOTSTRAP_EMAIL=.*|CLOUD_CORE_BOOTSTRAP_EMAIL=|" \
    -e "s|^CLOUD_CORE_BOOTSTRAP_PASSWORD=.*|CLOUD_CORE_BOOTSTRAP_PASSWORD=|" \
    "$ENV_FILE" >"$tmp" && mv "$tmp" "$ENV_FILE"
  echo ".env created — session secret set; create your admin on first visit."
fi

echo "Starting Private Cloud Gateway dev stack..."
echo "Tip: run ./scripts/dev-nuke.sh to wipe everything and start fresh after a PR merge."
docker compose -f "$COMPOSE_FILE" up --build "$@"

echo
echo "Stack up. Open http://home.localtest.me"

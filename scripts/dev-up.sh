#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/infra/docker/docker-compose.yml"
ENV_FILE="$REPO_ROOT/.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: .env not found. Run: cp .env.example .env"
  echo "Then set CLOUD_CORE_SESSION_SECRET before starting."
  exit 1
fi

echo "Starting Private Cloud Gateway dev stack..."
docker compose -f "$COMPOSE_FILE" up --build "$@"

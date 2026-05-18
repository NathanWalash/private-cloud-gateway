#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/infra/docker/docker-compose.yml"

echo "Stopping Private Cloud Gateway dev stack..."
docker compose -f "$COMPOSE_FILE" down "$@"

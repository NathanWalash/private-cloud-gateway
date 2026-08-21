#!/usr/bin/env bash
set -euo pipefail

# Run the full end-to-end suite (Go container-lifecycle + Playwright browser
# click-through) against a locally running dev stack.
#
# Prereqs: the stack is up (`make dev-up`) and an admin account exists. Pass its
# credentials via env, or accept the defaults below.
#
#   E2E_EMAIL=you@example.com E2E_PASSWORD=secret ./scripts/e2e-local.sh
#
# These tests actually pull images and start/stop containers, so they take a
# while — this is the per-release confidence check, not a per-PR gate.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export E2E_BASE_URL="${E2E_BASE_URL:-http://home.localtest.me}"
export E2E_EMAIL="${E2E_EMAIL:-e2e@test.local}"
export E2E_PASSWORD="${E2E_PASSWORD:-e2e-test-pass-123}"

echo "▸ Target: $E2E_BASE_URL (user: $E2E_EMAIL)"

if ! curl -sf "$E2E_BASE_URL/healthz" >/dev/null; then
  echo "[x] Stack not reachable at $E2E_BASE_URL — run 'make dev-up' first." >&2
  exit 1
fi

echo "▸ Go container-lifecycle E2E…"
( cd "$REPO_ROOT/apps/core" && go test -v -tags e2e -timeout 360s ./e2e/... )

echo "▸ Playwright browser E2E…"
( cd "$REPO_ROOT/e2e" && npm install --silent && npx playwright install chromium >/dev/null 2>&1 && npx playwright test )

echo "[ok] End-to-end suite passed."

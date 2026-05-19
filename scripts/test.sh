#!/usr/bin/env bash
set -euo pipefail

# Runs tests for all components that have them.
# Go tests are added in Milestone 1; E2E tests (Playwright) in Milestone 2.

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Running Go Core tests..."
if [ -f "$REPO_ROOT/apps/core/go.mod" ]; then
  (cd "$REPO_ROOT/apps/core" && go test ./...)
else
  echo "  No go.mod found — skipping (not yet implemented)"
fi

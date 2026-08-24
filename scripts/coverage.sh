#!/usr/bin/env bash
set -euo pipefail

# Honest, combined coverage: unit tests + the end-to-end suite.
#
# `go test -cover` only measures what unit tests touch. The HTTP handlers, Docker
# lifecycle, and backup code are exercised by the E2E suite (real Docker), whose
# coverage a plain `go test ./...` never counts — so those packages look far
# thinner than they actually are. This script closes that gap: it runs the unit
# tests and a coverage-instrumented E2E stack, then merges both into one report.
#
# Usage:
#   scripts/coverage.sh            # unit + Go E2E (default apps), merged report
#   COVERAGE_UNIT_ONLY=1 …         # skip the Docker stack; unit coverage only
#   E2E_APPS=excalidraw,umami …    # which blueprints the E2E lifecycle installs
#
# Output: .coverage/coverage.txt (merged profile) + a printed per-package table.
# Requires: Go 1.20+ (integration coverage), Docker (unless COVERAGE_UNIT_ONLY=1).

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE="$REPO_ROOT/apps/core"
COVDIR="$REPO_ROOT/.coverage"
UNIT="$COVDIR/unit"
E2E="$COVDIR/e2e"
MERGED="$COVDIR/merged"
COMPOSE=(docker compose -f "$REPO_ROOT/infra/docker/docker-compose.yml" -f "$REPO_ROOT/infra/docker/docker-compose.coverage.yml")

E2E_APPS="${E2E_APPS:-excalidraw}"

echo "▸ Resetting $COVDIR"
rm -rf "$COVDIR"
mkdir -p "$UNIT" "$E2E" "$MERGED"

# ── 1. Unit coverage (binary covdata format, so it merges with the E2E data) ──
echo "▸ Unit tests (coverage → $UNIT)"
# atomic mode must match the instrumented E2E binary (-covermode=atomic) so the
# two covdata sets merge without a counter-mode clash. -count=1 defeats the test
# cache, which would otherwise skip execution and write no coverage.
( cd "$CORE" && go test -count=1 -cover -covermode=atomic -coverpkg=./... ./... -args -test.gocoverdir="$UNIT" >/dev/null )

INPUTS="$UNIT"

if [ "${COVERAGE_UNIT_ONLY:-0}" != "1" ]; then
  # ── 2. E2E coverage: instrumented stack, real Docker ──────────────────────
  # A throwaway .env for the fresh stack (restored on exit).
  ENV_FILE="$REPO_ROOT/.env"
  ENV_BAK=""
  if [ -f "$ENV_FILE" ]; then
    ENV_BAK="$REPO_ROOT/.env.coverage-bak"
    cp "$ENV_FILE" "$ENV_BAK"
  fi
  TEST_EMAIL="e2e@test.local"
  TEST_PASSWORD="e2e-test-pass-1234"
  {
    cat "$REPO_ROOT/.env.example"
    echo "CLOUD_CORE_SESSION_SECRET=$(openssl rand -hex 32)"
    echo "CLOUD_CORE_BOOTSTRAP_EMAIL=$TEST_EMAIL"
    echo "CLOUD_CORE_BOOTSTRAP_PASSWORD=$TEST_PASSWORD"
    echo "CLOUD_CORE_LOGIN_URL=http://home.localtest.me/login"
    echo "CLOUD_CORE_COOKIE_DOMAIN=localtest.me"
  } > "$ENV_FILE"

  cleanup() {
    echo "▸ Tearing down coverage stack (flushes coverage on graceful shutdown)"
    "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
    if [ -n "$ENV_BAK" ]; then mv "$ENV_BAK" "$ENV_FILE"; else rm -f "$ENV_FILE"; fi
  }
  trap cleanup EXIT

  echo "▸ Building + starting instrumented stack"
  # Wipe any persisted volume first: a stale admin from an earlier run would make
  # bootstrap skip, and the E2E login would 401 against the old password.
  "${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
  "${COMPOSE[@]}" up -d --build

  echo "▸ Waiting for core to become healthy"
  for i in $(seq 1 60); do
    if curl -sf -H "Host: home.localtest.me" http://127.0.0.1/healthz >/dev/null 2>&1; then
      echo "  healthy after ${i}s"; break
    fi
    if [ "$i" -eq 60 ]; then
      echo "  [x] stack did not become healthy"; "${COMPOSE[@]}" logs core; exit 1
    fi
    sleep 2
  done

  echo "▸ Go E2E lifecycle (apps: $E2E_APPS)"
  ( cd "$CORE" && \
    E2E_BASE_URL="http://home.localtest.me" \
    E2E_FILES_URL="http://files.localtest.me" \
    E2E_RESOLVE="127.0.0.1" \
    E2E_EMAIL="$TEST_EMAIL" \
    E2E_PASSWORD="$TEST_PASSWORD" \
    E2E_APPS="$E2E_APPS" \
    go test -count=1 -tags e2e -timeout 900s ./e2e/... ) || {
    echo "  [x] E2E failed"; "${COMPOSE[@]}" logs core; exit 1
  }

  # `compose down` (in the trap) sends SIGTERM → the core shuts down gracefully
  # → the instrumented runtime writes counters to /coverage (mounted at $E2E).
  cleanup
  trap - EXIT

  if ! compgen -G "$E2E/covcounters.*" >/dev/null; then
    echo "  [x] no E2E coverage counters were written to $E2E" >&2
    echo "      (the instrumented binary must exit gracefully to flush)" >&2
    exit 1
  fi
  INPUTS="$UNIT,$E2E"
fi

# ── 3. Merge + report ──────────────────────────────────────────────────────
echo "▸ Merging coverage ($INPUTS)"
go tool covdata merge -i="$INPUTS" -o="$MERGED"
go tool covdata textfmt -i="$MERGED" -o="$COVDIR/coverage.txt"

echo ""
echo "=== Combined coverage by package ==="
go tool covdata percent -i="$MERGED" | sort
echo ""
echo -n "=== Total: "
( cd "$CORE" && go tool cover -func="$COVDIR/coverage.txt" | tail -1 | awk '{print $NF}' )
echo "Merged profile: $COVDIR/coverage.txt (open with: go tool cover -html=$COVDIR/coverage.txt)"

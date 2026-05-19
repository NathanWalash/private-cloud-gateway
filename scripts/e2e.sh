#!/usr/bin/env bash
# shellcheck disable=SC2015  # A && pass || fail is intentional — pass always returns 0
# End-to-end tests for Private Cloud Gateway.
# Runs against a live stack — not part of the standard unit test suite.
#
# Usage:
#   ./scripts/e2e.sh                          # defaults to http://home.localtest.me
#   ./scripts/e2e.sh http://home.localtest.me
#   E2E_PASSWORD=yourpassword ./scripts/e2e.sh
#
# Prerequisites:
#   - Stack running: make dev-up
#   - jq installed: apt install jq / brew install jq

set -uo pipefail

BASE="${1:-http://home.localtest.me}"
FILES_BASE="${FILES_BASE:-http://files.localtest.me}"
EMAIL="${E2E_EMAIL:-}"
PASSWORD="${E2E_PASSWORD:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0

pass()  { echo -e "${GREEN}✓${NC} $1"; PASS=$((PASS+1)); }
fail()  { echo -e "${RED}✗${NC} $1 — $2"; FAIL=$((FAIL+1)); }
skip()  { echo -e "${YELLOW}⊘${NC} $1 (skipped: $2)"; SKIP=$((SKIP+1)); }
section() { echo -e "\n${BLUE}── $1 ──${NC}"; }

http_code() { curl -s -o /dev/null -w "%{http_code}" "$@" 2>/dev/null; }
http_body() { curl -s "$@" 2>/dev/null; }

echo "Running E2E tests against $BASE"
echo "Set E2E_EMAIL and E2E_PASSWORD to test authenticated flows."

# ── Health & setup ─────────────────────────────────────────────────────────────
section "Health & setup"

code=$(http_code "$BASE/healthz")
[ "$code" = "200" ] && pass "healthz returns 200" || fail "healthz" "got $code"

needs=$(http_body "$BASE/api/auth/setup" -H "Accept: application/json" | jq -r '.needs_setup' 2>/dev/null)
if [ "$needs" = "true" ] || [ "$needs" = "false" ]; then
  pass "/api/auth/setup responds (needs_setup=$needs)"
else
  fail "/api/auth/setup" "unexpected response: $needs"
fi

# ── Unauthenticated redirects ──────────────────────────────────────────────────
section "Unauthenticated access"

code=$(http_code "$BASE/" -H "Accept: text/html")
[ "$code" = "200" ] && pass "SPA served at /" || fail "SPA at /" "got $code"

code=$(http_code "$BASE/api/auth/me" -H "Accept: application/json")
[ "$code" = "401" ] && pass "/api/auth/me returns 401 when not logged in" || fail "/api/auth/me unauthed" "got $code"

code=$(http_code "$BASE/api/apps" -H "Accept: application/json")
[ "$code" = "401" ] && pass "/api/apps returns 401 when not logged in" || fail "/api/apps unauthed" "got $code"

code=$(http_code "$BASE/api/auth/verify" -H "Accept: application/json")
[ "$code" = "302" ] && pass "/api/auth/verify returns 302 (Caddy forward-auth redirect)" || fail "/api/auth/verify" "got $code, want 302"

# ── files.* subdomain ─────────────────────────────────────────────────────────
section "Protected subdomain (files)"

code=$(http_code "$FILES_BASE/")
if [ "$code" = "302" ]; then
  pass "files.* redirects to login when not authenticated"
elif [ "$code" = "000" ]; then
  skip "files.* connectivity" "cannot reach $FILES_BASE — add to /etc/hosts"
else
  fail "files.* auth redirect" "got $code, want 302"
fi

# ── Authenticated flows (optional) ────────────────────────────────────────────
if [ -z "$EMAIL" ] || [ -z "$PASSWORD" ]; then
  echo ""
  echo "  Set E2E_EMAIL and E2E_PASSWORD to run authenticated tests."
  echo ""
  echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
  [ $FAIL -eq 0 ] || exit 1
  exit 0
fi

section "Authenticated flows (email=$EMAIL)"

# Login
response=$(curl -si -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" 2>/dev/null)

code=$(echo "$response" | head -1 | grep -o "[0-9][0-9][0-9]" | head -1)
SESSION=$(echo "$response" | grep -i "set-cookie" | grep -o "pcg_session=[^;]*" | head -1)

if [ "$code" = "200" ] && [ -n "$SESSION" ]; then
  pass "Login succeeds and returns session cookie"
else
  fail "Login" "got HTTP $code, session=${SESSION:-empty}"
  echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
  exit 1
fi

AUTH_HEADER="Cookie: $SESSION"

# Me endpoint
me=$(http_body "$BASE/api/auth/me" -H "$AUTH_HEADER" -H "Accept: application/json")
email_got=$(echo "$me" | jq -r '.email' 2>/dev/null)
[ "$email_got" = "$EMAIL" ] && pass "/api/auth/me returns correct email" || fail "/api/auth/me email" "got $email_got"

first=$(echo "$me" | jq -r '.first_name' 2>/dev/null)
[ -n "$first" ] && pass "/api/auth/me has first_name: $first" || skip "/api/auth/me first_name" "empty (user may not have set it)"

# Status
status_body=$(http_body "$BASE/api/status" -H "$AUTH_HEADER" -H "Accept: application/json")
uptime=$(echo "$status_body" | jq -r '.uptime' 2>/dev/null)
[ -n "$uptime" ] && pass "/api/status returns uptime: $uptime" || fail "/api/status" "missing uptime"

# Apps list
apps_body=$(http_body "$BASE/api/apps" -H "$AUTH_HEADER" -H "Accept: application/json")
is_array=$(echo "$apps_body" | jq 'type == "array"' 2>/dev/null)
[ "$is_array" = "true" ] && pass "/api/apps returns an array" || fail "/api/apps" "not an array: $apps_body"

# Blueprints
bps_body=$(http_body "$BASE/api/blueprints" -H "$AUTH_HEADER" -H "Accept: application/json")
bp_count=$(echo "$bps_body" | jq 'length' 2>/dev/null)
[ "${bp_count:-0}" -gt 0 ] && pass "/api/blueprints returns $bp_count blueprints" || fail "/api/blueprints" "empty or error: $bps_body"

# Backup list
backup_body=$(http_body "$BASE/api/backup/list" -H "$AUTH_HEADER" -H "Accept: application/json")
is_array=$(echo "$backup_body" | jq 'type == "array"' 2>/dev/null)
[ "$is_array" = "true" ] && pass "/api/backup/list returns an array" || fail "/api/backup/list" "not an array"

# Files subdomain WITH session
if [ "$FILES_BASE" != "http://files.localtest.me" ] || [ "$(http_code "$FILES_BASE/")" != "000" ]; then
  code=$(http_code "$FILES_BASE/" --cookie "$SESSION")
  [ "$code" = "200" ] && pass "files.* accessible when authenticated" || skip "files.* authenticated" "got $code"
fi

# Logout
code=$(http_code -X POST "$BASE/api/auth/logout" -H "$AUTH_HEADER" -H "Accept: application/json")
[ "$code" = "200" ] && pass "Logout succeeds" || fail "Logout" "got $code"

# Verify session is gone after logout
code=$(http_code "$BASE/api/auth/me" -H "$AUTH_HEADER" -H "Accept: application/json")
[ "$code" = "401" ] && pass "Session invalid after logout" || fail "Post-logout session" "got $code, want 401"

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}Passed: $PASS${NC}  ${RED}Failed: $FAIL${NC}  ${YELLOW}Skipped: $SKIP${NC}"
[ $FAIL -eq 0 ] || exit 1

#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Linting markdown..."
markdownlint-cli2 "$REPO_ROOT/**/*.md"

echo "Linting shell scripts..."
shellcheck "$REPO_ROOT/scripts"/*.sh

echo "Linting YAML..."
yamllint -c "$REPO_ROOT/.yamllint.yml" "$REPO_ROOT/blueprints/" "$REPO_ROOT/.github/workflows/" "$REPO_ROOT/infra/docker/"

echo "All lint checks passed."

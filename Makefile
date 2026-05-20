.DEFAULT_GOAL := help

COMPOSE_FILE := infra/docker/docker-compose.yml

# ── Help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' | sort

# ── Environment ───────────────────────────────────────────────────────────────

.PHONY: env
env: ## Copy .env.example to .env if .env does not exist
	@if [ -f .env ]; then \
		echo ".env already exists — skipping."; \
	else \
		cp .env.example .env; \
		echo ".env created. Edit CLOUD_CORE_SESSION_SECRET before starting."; \
	fi

# ── Development stack ─────────────────────────────────────────────────────────

.PHONY: nuke
nuke: ## Wipe ALL state (containers, volumes, images) and rebuild from scratch — use after PR merges
	./scripts/dev-nuke.sh

.PHONY: wipe
wipe: ## Wipe ALL state without rebuilding
	./scripts/dev-nuke.sh --wipe

.PHONY: dev-up
dev-up: ## Start the local development stack (builds images)
	./scripts/dev-up.sh

.PHONY: dev-up-d
dev-up-d: ## Start the local development stack in detached mode
	./scripts/dev-up.sh -d

.PHONY: dev-down
dev-down: ## Stop and remove the local development stack
	./scripts/dev-down.sh

.PHONY: dev-logs
dev-logs: ## Tail logs from all services
	docker compose -f $(COMPOSE_FILE) logs -f

.PHONY: dev-ps
dev-ps: ## Show service status
	docker compose -f $(COMPOSE_FILE) ps

# ── Testing ───────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests
	./scripts/test.sh

# ── Linting ───────────────────────────────────────────────────────────────────

.PHONY: lint
lint: lint-md lint-sh lint-yaml ## Run all lint checks

.PHONY: lint-md
lint-md: ## Lint markdown files (requires: npm i -g markdownlint-cli2)
	markdownlint-cli2 "**/*.md"

.PHONY: lint-sh
lint-sh: ## Lint shell scripts (requires: shellcheck)
	shellcheck scripts/*.sh

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files (requires: pip install yamllint)
	yamllint -c .yamllint.yml blueprints/ .github/workflows/ infra/docker/

# ── Docs ──────────────────────────────────────────────────────────────────────

.PHONY: docs
docs: ## List all documentation files
	@find docs -name "*.md" | sort

.DEFAULT_GOAL := help

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
		echo ".env created. Edit it before starting the stack."; \
	fi

# ── Development stack ─────────────────────────────────────────────────────────

.PHONY: dev-up
dev-up: ## Start local development stack (not yet implemented)
	@echo "dev-up is not yet implemented. See docs/dev-local.md."
	@exit 1

.PHONY: dev-down
dev-down: ## Stop local development stack (not yet implemented)
	@echo "dev-down is not yet implemented. See docs/dev-local.md."
	@exit 1

# ── Testing ───────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests (not yet implemented)
	@echo "test is not yet implemented. Tests will be added in Milestone 1."
	@exit 1

.PHONY: lint
lint: lint-md lint-sh lint-yaml ## Run all lint checks

.PHONY: lint-md
lint-md: ## Lint markdown files (requires markdownlint-cli2: npm i -g markdownlint-cli2)
	markdownlint-cli2 "**/*.md"

.PHONY: lint-sh
lint-sh: ## Lint shell scripts (requires shellcheck)
	shellcheck scripts/*.sh

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files (requires yamllint: pip install yamllint)
	yamllint -c .yamllint.yml blueprints/ .github/workflows/

# ── Docs ──────────────────────────────────────────────────────────────────────

.PHONY: docs
docs: ## List all documentation files
	@find docs -name "*.md" | sort

---
description: Run the full local verification suite before opening a PR
---

Run the project's checks and report a concise pass/fail summary. Fix anything
that fails, then re-run.

Backend (`apps/core`):

- `gofmt -l .` — must print nothing (formatting clean).
- `go vet ./...`
- `golangci-lint run ./...` (v2; install with
  `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
  if missing).
- `go test -race -count=1 ./...`

Frontend (`apps/web`):

- `pnpm install` then `pnpm build` (type-check + production build).

Repo:

- `make lint` (markdownlint, shellcheck, yamllint).

Do not claim success unless every command above actually passed — show the
failing output if not.

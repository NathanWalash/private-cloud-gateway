# AI Coding Agent Brief

This file is the starting context for an AI coding agent working in this repository.

## What we are building

**Private Cloud Gateway** is a private, login-protected personal cloud dashboard and Docker app orchestrator.

It runs on a single Oracle Cloud Ubuntu VM, protects all apps behind one login, gives each app a custom subdomain, manages Docker apps from YAML blueprints, shows dashboard widgets, and provides encrypted backup/restore.

> The internal Go backend service is called **Cloud Core**. You will see this name in environment variables (`CLOUD_CORE_*`), Docker network names, and service references in the codebase.

## Current state

The repository contains planning documentation and scaffolding only. No application code has been written yet.

The immediate next task is **Milestone 1: local secured routing**.

See [ROADMAP.md](../ROADMAP.md) and [docs/dev-local.md](dev-local.md) for context.

## Do not build

- AI agent features.
- Kubernetes.
- Public SaaS multi-tenancy.
- Oracle Database migration tooling.
- Iframe-first app embedding.
- Complex enterprise RBAC for v1.

## Core architecture

```text
Caddy (public HTTPS gateway)
  ↓
Cloud Core Go service — auth verification at /api/auth/verify
  ↓
Private Docker app network
```

## Tech stack

| Layer | Choice |
|---|---|
| Backend/control plane | Go |
| Frontend/dashboard | Vite + TypeScript + Tailwind CSS |
| Control database | SQLite in WAL mode |
| Reverse proxy | Caddy |
| Runtime | Docker Compose |
| Production host | Ubuntu Server on Oracle Cloud |
| App definitions | YAML blueprints |
| Backups | Encrypted archives |

## Milestone 1 goal

Build local secured routing.

Required outcome:

```text
files.localhost redirects to login when logged out.
files.localhost opens a test app when logged in.
```

## Milestone 1 implementation tasks

1. Create Go HTTP server in `apps/core/`.
2. Add SQLite connection and migrations.
3. Add login endpoint with Argon2id password hashing.
4. Add session cookie (HTTP-only, SameSite=Lax).
5. Add `/api/auth/verify` endpoint.
6. Configure Caddy forward-auth to call `/api/auth/verify`.
7. Start one test app container on the private Docker network.
8. Verify that unauthenticated requests redirect to login.
9. Verify that authenticated requests reach the test app.

## Core rules

- **Caddy is the only public entrypoint.** All traffic enters through Caddy.
- **Apps do not publish public host ports.** Use `expose:` not `ports:`.
- **All protected subdomains require auth.** No exceptions for v1.
- **Backups are a v1 feature.** Do not defer them.
- **Keep docs updated** with every architectural change.

## Git workflow

- Branch from `main`. Do not commit directly to `main`.
- PRs are required before merging.
- Use Conventional Commit style.

See [docs/git-workflow.md](git-workflow.md).

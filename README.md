# Private Cloud Gateway

Private Cloud Gateway is a private, login-protected personal cloud dashboard and Docker app gateway.

It runs on a single Oracle Cloud Ubuntu VM, protects all hosted apps behind one login, routes each app through a custom subdomain, manages apps from YAML blueprints, shows server and app status widgets, and makes encrypted backup and restore simple enough to trust.

> **Naming note:** The internal Go backend service is called the **Cloud Core** engine. You will see this name in environment variable prefixes (`CLOUD_CORE_*`), Docker network names, and internal service references.

## How it works

```text
User
  ↓
Custom domain (e.g. nathan.me)
  ↓
Caddy HTTPS gateway
  ↓
Cloud Core auth check
  ↓
Private Docker network
  ↓
Self-hosted apps
```

Example routes:

```text
nathan.me              → login + dashboard
files.nathan.me        → protected file browser
pdf.nathan.me          → protected PDF tools
bookmarks.nathan.me    → protected bookmarks app
status.nathan.me       → protected monitoring
```

## Main goals

- One polished private login page.
- One customisable dashboard.
- All apps locked behind the same authentication layer.
- Clean subdomain routing for each app.
- Docker-based app installs from YAML blueprints.
- App status, server status, API status, and custom widgets.
- Reliable encrypted backup and restore.
- Local-first development, then one-command Oracle Cloud deployment.

## Not in scope for v1

- Kubernetes or multi-node clustering.
- AI agent support.
- Public SaaS multi-tenancy.
- Iframe-first app embedding.
- Oracle Database migration.

## Local development

```bash
git clone <repo-url>
cd private-cloud-gateway
cp .env.example .env
# edit .env — at minimum set CLOUD_CORE_SESSION_SECRET
./scripts/dev-up.sh
```

Then open:

```text
http://home.localhost
```

See [docs/dev-local.md](docs/dev-local.md) for full setup instructions, prerequisite versions, and hosts file configuration.

## Production deployment

Develop and test locally first.

Eventually deploy to a fresh Oracle Cloud Ubuntu VM with:

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/private-cloud-gateway/main/install.sh | sudo bash
```

The installer will prepare the server, install Docker, start the gateway, and guide you through domain, admin, and backup setup.

## Documentation

| Document | Description |
|---|---|
| [docs/dev-local.md](docs/dev-local.md) | Local development setup |
| [docs/git-workflow.md](docs/git-workflow.md) | Branch and PR workflow |
| [ROADMAP.md](ROADMAP.md) | Milestones and current status |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | Security model |
| [docs/02-architecture.md](docs/02-architecture.md) | Architecture overview |
| [docs/decisions/](docs/decisions/) | Architecture decision records |

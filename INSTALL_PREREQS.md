# Prerequisites

See [docs/dev-local.md](docs/dev-local.md) for the full local development setup guide, including hosts file configuration and Milestone 1 goals.

## Local development

Install these on your development machine:

| Tool | Check |
|---|---|
| Git | `git --version` |
| Docker Desktop or Docker Engine | `docker --version` |
| Docker Compose plugin | `docker compose version` |
| Go 1.22+ | `go version` |
| Node.js LTS (20+) | `node --version` |
| pnpm or npm | `pnpm --version` or `npm --version` |
| Make (optional) | `make --version` |
| mkcert (optional, local HTTPS) | `mkcert --version` |

## Recommended developer tools

| Tool | Check |
|---|---|
| curl | `curl --version` |
| jq | `jq --version` |
| sqlite3 | `sqlite3 --version` |
| openssl | `openssl version` |

## Verify everything at once

```bash
git --version
docker --version
docker compose version
go version
node --version
pnpm --version
sqlite3 --version
jq --version
```

## Oracle Cloud production

The production installer handles most server-side dependencies automatically. You need:

- Oracle Cloud account with an Ubuntu VM
- SSH access and a public IPv4 address
- A custom domain with DNS access
- Optionally, a Cloudflare account for DNS management

The installer is responsible for:

- Docker and Docker Compose plugin
- Caddy (as a container or system package)
- systemd service files
- Firewall rules (ports 80, 443 public; 22 restricted)

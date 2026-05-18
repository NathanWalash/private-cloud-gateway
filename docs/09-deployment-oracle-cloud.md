# Oracle Cloud Deployment

## Target environment

Private Cloud Gateway targets a single Oracle Cloud Ubuntu VM.

Recommended starting host:

- Ubuntu Server (ARM64 or AMD64)
- Docker and Docker Compose plugin
- Caddy (as a container or system package)
- Persistent block volume for data
- Public IPv4
- Custom domain pointed to the VM

## Production flow

```text
Create Oracle VM
  ↓
SSH into server
  ↓
Run install command
  ↓
Point DNS to server IP
  ↓
Open setup page
  ↓
Create admin account
  ↓
Configure backup
  ↓
Open dashboard
```

## One-command install goal

Eventually:

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/private-cloud-gateway/main/install.sh | sudo bash
```

## Installer responsibilities

The installer should:

- Install Docker and Docker Compose plugin
- Install required system packages
- Create a `cloudgw` service user
- Create storage directories under `/var/lib/cloud-core/`
- Configure firewall rules
- Pull and start containers
- Install systemd service
- Prepare Caddy configuration
- Start the gateway
- Print the setup URL

## DNS

Recommended wildcard DNS so new app subdomains work without adding per-app records:

```text
A     example.com       → ORACLE_SERVER_IP
A     *.example.com     → ORACLE_SERVER_IP
```

## Storage layout

Recommended production data root:

```text
/var/lib/cloud-core/
  database/       Cloud Core SQLite database
  blueprints/     Installed app blueprints
  apps/           App data volumes
  backups/        Encrypted backup archives
  caddy/          Caddy data and config
  logs/           Service logs
```

## Firewall

Public (exposed by Caddy):

```text
80/tcp
443/tcp
```

Restricted (SSH — limit by IP where possible):

```text
22/tcp
```

No app container ports should be publicly accessible. All app traffic routes through Caddy.

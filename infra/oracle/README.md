# Oracle Cloud Deployment

Deploy Private Cloud Gateway to a fresh Oracle Cloud Ubuntu VM.

## Prerequisites

- Oracle Cloud Always Free tier VM (ARM or AMD), Ubuntu 22.04+
- SSH access and public IPv4
- Custom domain with DNS access

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

## DNS setup

Point your domain to the server IP with a wildcard — this means every new
app subdomain works automatically without per-app DNS changes:

```text
A     yourdomain.com      → SERVER_IP
A     *.yourdomain.com    → SERVER_IP
```

## First run

Visit `https://home.yourdomain.com` → setup wizard → create admin account.

## Service management

```bash
sudo systemctl status pcg
sudo systemctl restart pcg
sudo journalctl -u pcg -f
```

## TLS certificates

**Default (individual certs per subdomain):** Caddy automatically gets a Let's Encrypt cert for each subdomain as you install apps. No extra config needed. Works with any DNS provider.

**Optional (wildcard cert):** One cert covers `*.yourdomain.com`. Requires DNS validation — Cloudflare is the easiest provider. Install the Caddy Cloudflare plugin and add to `.env`:

```bash
# Add to /opt/pcg/.env
CLOUDFLARE_API_TOKEN=your-token-here
```

Then replace `Caddyfile.prod` with:

```text
{
    email {$CLOUD_CORE_ADMIN_EMAIL}
    admin :2019
}

*.{$CLOUD_CORE_COOKIE_DOMAIN}, {$CLOUD_CORE_COOKIE_DOMAIN} {
    tls {
        dns cloudflare {$CLOUDFLARE_API_TOKEN}
    }
    @home host home.{$CLOUD_CORE_COOKIE_DOMAIN}
    handle @home {
        reverse_proxy core:8080
    }
    # App routes added dynamically
}
```

The official Caddy image does not include the Cloudflare plugin. Use `caddy:2-alpine` as a base and add the plugin, or use the [xcaddy builder](https://github.com/caddyserver/xcaddy).

## Backup

Backups are stored at `/opt/pcg/backups/`. The service backs up daily by default (`CLOUD_CORE_BACKUP_SCHEDULE=24h`).

For offsite backup, set `CLOUD_CORE_BACKUP_PASSPHRASE` and use the **Safe Escape download** button in the dashboard — it creates an encrypted archive and downloads it to your browser.

## Files

| File | Purpose |
|---|---|
| `install.sh` | One-command installer |
| `pcg.service` | systemd service unit |
| `setup-firewall.sh` | UFW rules (ports 22, 80, 443 only) |

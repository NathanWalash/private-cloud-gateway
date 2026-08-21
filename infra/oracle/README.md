# Oracle Cloud Deployment

Deploy Private Cloud Gateway to a fresh Oracle Cloud Ubuntu VM.

## Prerequisites

- Oracle Cloud Always Free tier VM, Ubuntu 22.04+. **Recommended: the ARM
  `VM.Standard.A1.Flex` shape with ~4 OCPU / 24 GB RAM** (also free). The 1 GB
  `E2.1.Micro` shape is too small once you run the multi-container apps
  (Ghost+MySQL, Paperless+Redis) — they will OOM.
- SSH access and public IPv4
- Custom domain with DNS access

## Open ports in the Oracle console (do this first)

**This is the #1 first-deploy gotcha.** A stock Oracle VM blocks inbound 80/443
at the cloud level, which no command on the box can fix. In the Oracle Cloud
console, open your VCN's **Security List** (or the instance's NSG) and add
**ingress rules for TCP 80 and TCP 443** (source `0.0.0.0/0`). Only port 22 is
allowed by default. Without this the site is unreachable and Let's Encrypt
cannot issue certificates. (`setup-firewall.sh`, run by the installer, handles
the host-level UFW + iptables rules — but not the cloud Security List.)

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash
```

The installer pins the latest release, generates all secrets (session, setup
token, backup passphrase — it prints the setup token and passphrase; save them),
and starts the service.

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

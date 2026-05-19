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

## Files

| File | Purpose |
|---|---|
| `install.sh` | One-command installer |
| `pcg.service` | systemd service unit |
| `setup-firewall.sh` | UFW rules (ports 22, 80, 443 only) |

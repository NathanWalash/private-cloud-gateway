#!/usr/bin/env bash
# Configure UFW firewall for Private Cloud Gateway on Oracle Cloud Ubuntu.
# Run as root after initial VM setup.
set -euo pipefail

echo "Setting up UFW firewall rules..."

# Reset to defaults
ufw --force reset

# Default policies
ufw default deny incoming
ufw default allow outgoing

# SSH — restrict to your IP if possible: ufw allow from YOUR_IP to any port 22
ufw allow 22/tcp comment "SSH"

# Web traffic — Caddy handles HTTP and HTTPS
ufw allow 80/tcp  comment "HTTP (Caddy redirect to HTTPS)"
ufw allow 443/tcp comment "HTTPS"
ufw allow 443/udp comment "HTTP/3 (QUIC)"

# Never expose:
#   Port 8080 (Go Core)      — internal Docker network only
#   Port 2019 (Caddy admin)  — internal Docker network only
#   Port 5984 (CouchDB)      — proxied through Caddy

ufw --force enable
ufw status verbose

echo ""
echo "Firewall configured. Public ports: 22 (SSH), 80 (HTTP), 443 (HTTPS)."
echo "All other ports are blocked."

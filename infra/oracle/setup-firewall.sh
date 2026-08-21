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

# Oracle Cloud Ubuntu images ship iptables rules (netfilter-persistent) that
# REJECT inbound 80/443 in the INPUT chain regardless of UFW. Insert explicit
# ACCEPT rules ahead of that REJECT and persist them, or the site stays
# unreachable and Let's Encrypt HTTP-01 fails.
if command -v iptables >/dev/null 2>&1; then
  echo "Opening 80/443 in iptables (Oracle default INPUT REJECT)..."
  iptables -I INPUT -p tcp --dport 80 -j ACCEPT
  iptables -I INPUT -p tcp --dport 443 -j ACCEPT
  iptables -I INPUT -p udp --dport 443 -j ACCEPT
  if command -v netfilter-persistent >/dev/null 2>&1; then
    netfilter-persistent save || true
  fi
fi

echo ""
echo "Host firewall configured. Public ports: 22 (SSH), 80 (HTTP), 443 (HTTPS)."
echo ""
echo "IMPORTANT: the host firewall is only half of it. In the Oracle Cloud"
echo "console you MUST also add ingress rules for TCP 80 and 443 to your VCN's"
echo "Security List (or the instance NSG) — only 22 is allowed by default."
echo "Without that, the site is unreachable from the internet and HTTPS can't"
echo "be issued."

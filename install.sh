#!/usr/bin/env bash
# Private Cloud Gateway — one-command installer for Oracle Cloud Ubuntu VM
# Usage: curl -fsSL https://raw.githubusercontent.com/NathanWalash/private-cloud-gateway/main/install.sh | sudo bash

set -euo pipefail

REPO="https://github.com/NathanWalash/private-cloud-gateway.git"
INSTALL_DIR="/opt/pcg"
SERVICE_NAME="pcg"
MIN_UBUNTU="22"

# ── Colour helpers ────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[PCG]${NC} $*"; }
success() { echo -e "${GREEN}[PCG]${NC} $*"; }
warn()    { echo -e "${YELLOW}[PCG]${NC} $*"; }
error()   { echo -e "${RED}[PCG]${NC} $*" >&2; exit 1; }

# ── Checks ────────────────────────────────────────────────────────────────────
[ "$(id -u)" -eq 0 ] || error "Run as root: curl ... | sudo bash"

OS=$(lsb_release -si 2>/dev/null || echo "Unknown")
VER=$(lsb_release -sr 2>/dev/null | cut -d. -f1 || echo "0")
[ "$OS" = "Ubuntu" ] && [ "$VER" -ge "$MIN_UBUNTU" ] || \
  error "Requires Ubuntu ${MIN_UBUNTU}+. Got: ${OS} ${VER}"

info "Starting Private Cloud Gateway installation on Ubuntu ${VER}"

# ── Install dependencies ──────────────────────────────────────────────────────
info "Installing system dependencies..."
apt-get update -qq
apt-get install -y -qq curl git ca-certificates gnupg lsb-release jq openssl ufw

# Docker
if ! command -v docker &>/dev/null; then
  info "Installing Docker..."
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
  success "Docker installed"
else
  success "Docker already installed ($(docker --version))"
fi

# Docker Compose (plugin)
if ! docker compose version &>/dev/null; then
  info "Installing Docker Compose plugin..."
  apt-get install -y -qq docker-compose-plugin
fi
success "Docker Compose: $(docker compose version --short)"

# ── Ensure swap (OOM insurance on small VMs) ──────────────────────────────────
# Multi-container apps can OOM on a low-RAM instance. If there's little memory and
# no swap, add a 2GB swapfile so a spike degrades gracefully instead of OOM-killing.
mem_mb=$(free -m 2>/dev/null | awk '/^Mem:/{print $2}')
swap_mb=$(free -m 2>/dev/null | awk '/^Swap:/{print $2}')
if [ "${swap_mb:-0}" -eq 0 ] && [ "${mem_mb:-9999}" -lt 4096 ]; then
  info "Low memory (${mem_mb}MB) and no swap — creating a 2GB swapfile..."
  if fallocate -l 2G /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=2048 2>/dev/null; then
    chmod 600 /swapfile
    mkswap /swapfile >/dev/null 2>&1
    swapon /swapfile
    grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
    success "2GB swapfile enabled"
  else
    warn "Could not create a swapfile; continuing without swap"
  fi
fi

# ── Collect configuration ────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Private Cloud Gateway Configuration"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

read -rp "  Your root domain (e.g. nathan.me): " DOMAIN
read -rp "  Admin email for Let's Encrypt (cert expiry notices): " ADMIN_EMAIL

# Validate inputs
[[ "$DOMAIN" =~ ^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)+$ ]] \
  || error "Invalid domain: $DOMAIN"
[[ "$ADMIN_EMAIL" =~ ^[^@]+@[^@]+\.[^@]+$ ]] \
  || error "Invalid email: $ADMIN_EMAIL"

SESSION_SECRET=$(openssl rand -hex 32)
SETUP_TOKEN=$(openssl rand -hex 16)
BACKUP_PASSPHRASE=$(openssl rand -hex 24)

# ── Preflight: DNS points here? ───────────────────────────────────────────────
# Caddy issues HTTPS via Let's Encrypt HTTP-01, which needs home.$DOMAIN (and app
# subdomains) resolving to THIS host with ports 80/443 reachable. Warn early —
# certs silently fail to issue otherwise. (getent uses the system resolver, so no
# extra package is required.)
PUBLIC_IP=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || echo "")
RESOLVED=$(getent ahostsv4 "home.$DOMAIN" 2>/dev/null | awk '{print $1; exit}')
if [ -z "$RESOLVED" ]; then
  warn "home.$DOMAIN does not resolve yet — create DNS 'A $DOMAIN → ${PUBLIC_IP:-<this IP>}' and 'A *.$DOMAIN → same' before certs can issue."
elif [ -n "$PUBLIC_IP" ] && [ "$RESOLVED" != "$PUBLIC_IP" ]; then
  warn "home.$DOMAIN resolves to $RESOLVED, not this host ($PUBLIC_IP) — HTTPS will fail until DNS points here."
fi

echo ""

# ── Set up directory layout ──────────────────────────────────────────────────
info "Creating directory layout at $INSTALL_DIR..."
mkdir -p \
  "$INSTALL_DIR/data" \
  "$INSTALL_DIR/backups" \
  "$INSTALL_DIR/blueprints" \
  "$INSTALL_DIR/caddy/data" \
  "$INSTALL_DIR/caddy/config"

chmod 700 "$INSTALL_DIR/data" "$INSTALL_DIR/backups"

# ── Clone or update repository ───────────────────────────────────────────────
if [ -d "$INSTALL_DIR/.git" ]; then
  info "Updating existing installation..."
  git -C "$INSTALL_DIR" pull --ff-only
else
  info "Cloning repository..."
  git clone --depth 1 "$REPO" "$INSTALL_DIR"
fi

# ── Resolve the release image to pin ──────────────────────────────────────────
# Pin PCG_VERSION to the latest published release so the first boot pulls a real,
# immutable image instead of a floating :latest.
info "Resolving latest release..."
PCG_VERSION=$(curl -fsSL https://api.github.com/repos/NathanWalash/private-cloud-gateway/releases/latest 2>/dev/null \
  | jq -r '.tag_name // empty')
if [ -z "$PCG_VERSION" ]; then
  error "Could not resolve the latest release tag (GitHub API rate-limited or offline). Re-run shortly — refusing to fall back to a floating :latest image, which would defeat pinning."
fi
info "Deploying version $PCG_VERSION"

# ── Write .env ───────────────────────────────────────────────────────────────
info "Writing configuration..."
cat > "$INSTALL_DIR/.env" <<EOF
# Private Cloud Gateway — production configuration
# Generated by install.sh on $(date -u +%Y-%m-%dT%H:%M:%SZ)

CLOUD_CORE_ENV=production
CLOUD_CORE_PORT=8080

# Pinned release image tag (deploy.sh updates this on deploy/rollback).
PCG_VERSION=$PCG_VERSION
CLOUD_CORE_COOKIE_DOMAIN=$DOMAIN
CLOUD_CORE_LOGIN_URL=https://home.$DOMAIN/login
CLOUD_CORE_ADMIN_EMAIL=$ADMIN_EMAIL

CLOUD_CORE_DATABASE_PATH=/data/cloud-core.db
CLOUD_CORE_SESSION_SECRET=$SESSION_SECRET
# Caddy's admin API is bound to a Unix socket shared only with core (see
# docker-compose.prod.yml), so app containers can't reach it. Point core at the
# socket path (an absolute path selects the socket transport).
CLOUD_CORE_CADDY_ADMIN=/run/pcg/caddy-admin.sock

# Setup token — required to claim the admin account on first run.
# This closes the first-run race: without it, anyone reaching the exposed
# instance before you could create the admin account. Printed once below.
CLOUD_CORE_SETUP_TOKEN=$SETUP_TOKEN
CLOUD_CORE_BLUEPRINT_DIR=/blueprints

# Backup passphrase — encrypts every backup. Generated below; SAVE IT (you need
# it to restore). Losing it means losing access to your backups.
CLOUD_CORE_BACKUP_PASSPHRASE=$BACKUP_PASSPHRASE

# Scheduled backups — e.g. "24h" for daily, empty to disable.
CLOUD_CORE_BACKUP_SCHEDULE=24h
# How many backup archives to keep (older ones are pruned). 0 keeps all.
CLOUD_CORE_BACKUP_KEEP=7

# Bootstrap (optional — use the setup wizard instead)
# CLOUD_CORE_BOOTSTRAP_EMAIL=
# CLOUD_CORE_BOOTSTRAP_PASSWORD=
EOF
chmod 600 "$INSTALL_DIR/.env"
success ".env written"

# ── Write production Caddyfile ───────────────────────────────────────────────
cp "$INSTALL_DIR/infra/caddy/Caddyfile.prod" "$INSTALL_DIR/caddy/Caddyfile"
info "Production Caddyfile written"

# ── Install systemd service ──────────────────────────────────────────────────
info "Installing systemd service..."
sed "s|/opt/pcg|$INSTALL_DIR|g" \
  "$INSTALL_DIR/infra/oracle/pcg.service" \
  > "/etc/systemd/system/${SERVICE_NAME}.service"

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
success "systemd service installed and enabled"

# ── Configure firewall ───────────────────────────────────────────────────────
info "Configuring firewall..."
bash "$INSTALL_DIR/infra/oracle/setup-firewall.sh"

# ── Start service ─────────────────────────────────────────────────────────────
info "Starting Private Cloud Gateway..."
systemctl start "$SERVICE_NAME"

# Wait for health. Core only exposes :8080 on the Docker network (never on the
# host — only Caddy binds host ports), so probe the container's health status
# directly, the same way deploy.sh does.
COMPOSE_FILE="$INSTALL_DIR/infra/docker/docker-compose.prod.yml"
healthy=false
for _ in $(seq 1 30); do
  cid="$(docker compose -f "$COMPOSE_FILE" --env-file "$INSTALL_DIR/.env" ps -q core 2>/dev/null)"
  if [ -n "$cid" ]; then
    status="$(docker inspect --format '{{.State.Health.Status}}' "$cid" 2>/dev/null || echo starting)"
    if [ "$status" = "healthy" ]; then
      healthy=true
      break
    fi
  fi
  sleep 3
done

if [ "$healthy" = true ]; then
  success "Service is healthy"
else
  warn "Service may still be starting. Check: sudo journalctl -u $SERVICE_NAME -f"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "  ${GREEN}Private Cloud Gateway installed successfully!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Dashboard: https://home.$DOMAIN"
echo ""
echo -e "  ${YELLOW}Setup token (needed to create your admin account):${NC}"
echo -e "      ${GREEN}$SETUP_TOKEN${NC}"
echo "  Enter this on the setup screen. It's also in $INSTALL_DIR/.env"
echo "  (CLOUD_CORE_SETUP_TOKEN). Keep it secret until setup is complete."
echo ""
echo -e "  ${YELLOW}Backup passphrase (SAVE THIS — required to restore backups):${NC}"
echo -e "      ${GREEN}$BACKUP_PASSPHRASE${NC}"
echo "  Stored in $INSTALL_DIR/.env (CLOUD_CORE_BACKUP_PASSPHRASE)."
echo ""
echo "  Next steps:"
echo -e "  ${YELLOW}1. REQUIRED — In the Oracle Cloud console, add VCN/subnet Security List"
echo "     ingress rules for TCP 80 and 443 (only 22 is open by default). Until"
echo -e "     you do, the site is unreachable and HTTPS cannot be issued.${NC}"
echo "  2. Point DNS: A $DOMAIN → ${PUBLIC_IP:-YOUR_IP}"
echo "     and:        A *.$DOMAIN → same IP"
echo "  3. Visit https://home.$DOMAIN to complete setup (enter the token above)"
echo ""
echo "  Manage:"
echo "  sudo systemctl status $SERVICE_NAME"
echo "  sudo journalctl -u $SERVICE_NAME -f"
echo ""

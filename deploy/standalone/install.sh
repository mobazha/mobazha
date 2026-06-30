#!/usr/bin/env bash
set -euo pipefail

# Mobazha Standalone Store — Zero-Config Installer
#
# Usage:
#   curl -sSL https://get.mobazha.org/standalone | sudo bash
#
# Optional flags:
#   --domain <domain>        Pre-configure a domain (enables auto-TLS via Let's Encrypt)
#   --overlay <tor|lokinet>  Enable privacy overlay network
#   --saas-url <url>         Override SaaS API URL (default: https://app.mobazha.org)
#   --testnet                Use cryptocurrency testnets (no real money)
#   --help                   Show this help message
#
# Tested on: Ubuntu 22.04+, Debian 12+

INSTALL_DIR="${INSTALL_DIR:-/opt/mobazha}"
COMPOSE_URL="https://get.mobazha.org/standalone/docker-compose.yml"
COMPOSE_OVERLAY_URL="https://get.mobazha.org/standalone/docker-compose.overlay.yml"
CTL_URL="https://get.mobazha.org/standalone/mobazha-ctl"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

STORE_DOMAIN=""
STANDALONE_API_KEY=""
SAAS_API_URL="${SAAS_API_URL:-https://app.mobazha.org}"
CONNECTIVITY="public"
OVERLAY_TYPE=""
PUBLIC_IP=""
TESTNET=""

log()  { echo -e "${GREEN}[mobazha]${NC} $*"; }
warn() { echo -e "${YELLOW}[mobazha]${NC} $*"; }
err()  { echo -e "${RED}[mobazha]${NC} $*" >&2; }

# --- Prerequisites ---

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "This script must be run as root (or with sudo)."
        exit 1
    fi
}

install_docker() {
    if command -v docker &>/dev/null; then
        log "Docker is already installed: $(docker --version)"
        return
    fi

    log "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
    log "Docker installed successfully."
}

install_docker_compose() {
    if docker compose version &>/dev/null 2>&1; then
        log "Docker Compose plugin is available."
        return
    fi

    log "Installing Docker Compose plugin..."
    apt-get update -qq && apt-get install -qq -y docker-compose-plugin
    log "Docker Compose plugin installed."
}

generate_api_key() {
    openssl rand -hex 32
}

detect_public_ip() {
    curl -4 -sf --connect-timeout 5 --max-time 10 https://ifconfig.me 2>/dev/null \
        || curl -4 -sf --connect-timeout 5 --max-time 10 https://api.ipify.org 2>/dev/null \
        || hostname -I 2>/dev/null | awk '{print $1}' \
        || echo "unknown"
}

# --- File Setup ---

write_env() {
    cat > .env <<EOF
STORE_DOMAIN=${STORE_DOMAIN}
SAAS_API_URL=${SAAS_API_URL}
STANDALONE_API_KEY=${STANDALONE_API_KEY}
ADMIN_PASSWORD=
SAAS_PEER_ID=
CONNECTIVITY=${CONNECTIVITY}
OVERLAY_TYPE=${OVERLAY_TYPE}
OVERLAY_DOMAIN=
TAG=stable
TESTNET=${TESTNET}
EOF
    chmod 600 .env
}

setup_files() {
    log "Setting up in $INSTALL_DIR ..."
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"

    if [ -f docker-compose.yml ]; then
        warn "Existing installation found. Backing up..."
        cp docker-compose.yml "docker-compose.yml.bak.$(date +%s)"
        [ -f .env ] && cp .env ".env.bak.$(date +%s)"
    fi

    curl -sSL "$COMPOSE_URL" -o docker-compose.yml
    log "Downloaded docker-compose.yml"

    if [ -n "$OVERLAY_TYPE" ]; then
        curl -sSL "$COMPOSE_OVERLAY_URL" -o docker-compose.overlay.yml
        log "Downloaded docker-compose.overlay.yml (overlay: ${OVERLAY_TYPE})"
    fi

    write_env
    log "Created .env configuration"

    curl -sSL "$CTL_URL" -o /usr/local/bin/mobazha-ctl
    chmod +x /usr/local/bin/mobazha-ctl
    log "Installed mobazha-ctl to /usr/local/bin/"
}

setup_auto_update() {
    log "Setting up automatic updates (systemd timer)..."

    cat > /etc/systemd/system/mobazha-update.service <<UNIT
[Unit]
Description=Mobazha Store Auto-Update
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory=${INSTALL_DIR}
ExecStart=/usr/bin/docker compose pull --quiet
ExecStart=/usr/bin/docker compose up -d --remove-orphans
ExecStartPost=/usr/bin/docker image prune -f --filter "until=168h"
UNIT

    cat > /etc/systemd/system/mobazha-update.timer <<UNIT
[Unit]
Description=Check for Mobazha Store updates hourly

[Timer]
OnCalendar=hourly
RandomizedDelaySec=300
Persistent=true

[Install]
WantedBy=timers.target
UNIT

    systemctl daemon-reload
    systemctl enable --now mobazha-update.timer

    log "Auto-update timer enabled (hourly)."
}

# --- Services ---

compose_cmd() {
    if [ -n "$OVERLAY_TYPE" ] && [ -f docker-compose.overlay.yml ]; then
        docker compose -f docker-compose.yml -f docker-compose.overlay.yml --profile "$OVERLAY_TYPE" "$@"
    else
        docker compose "$@"
    fi
}

start_services() {
    log "Pulling images..."
    compose_cmd pull

    log "Starting services..."
    compose_cmd up -d

    log "Waiting for health check..."
    local retries=0
    while ! compose_cmd ps --format json 2>/dev/null | grep -q '"healthy"'; do
        retries=$((retries + 1))
        if [ $retries -gt 60 ]; then
            warn "Health check timeout. Check logs with: docker compose logs -f"
            return
        fi
        sleep 2
    done
    log "Services are healthy!"
}

# --- Summary ---

print_summary() {
    local store_url

    if [ -n "$STORE_DOMAIN" ]; then
        store_url="https://${STORE_DOMAIN}"
    else
        store_url="http://${PUBLIC_IP}"
    fi

    echo ""
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Mobazha Standalone Store is running!${NC}"
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  Store URL:   ${BOLD}${store_url}${NC}"

    if [ -z "$STORE_DOMAIN" ]; then
        echo ""
        echo -e "  ${YELLOW}No domain configured — running on HTTP.${NC}"
        echo -e "  ${YELLOW}Add a domain with: mobazha-ctl set-domain <your-domain>${NC}"
    fi

    echo ""
    echo -e "  Install dir:  ${BOLD}${INSTALL_DIR}${NC}"
    echo -e "  Auto-update:  ${BOLD}systemctl status mobazha-update.timer${NC}"
    echo -e "  Manage:       ${BOLD}mobazha-ctl status${NC}"
    echo -e "  Logs:         ${BOLD}mobazha-ctl logs${NC}"
    echo -e "  Stop:         cd ${INSTALL_DIR} && docker compose down"
    echo ""
    echo -e "  ${BOLD}Next: open ${store_url}/admin to set up your store.${NC}"
    echo ""
}

# --- CLI ---

show_help() {
    cat <<'HELP'
Mobazha Standalone Store — Zero-Config Installer

Usage:
  curl -sSL https://get.mobazha.org/standalone | sudo bash
  sudo bash install.sh [options]

Options:
  --domain <domain>        Pre-configure a domain name for auto-TLS.
                           Without this, the store runs on IP with HTTP.
  --overlay <tor|lokinet>  Enable a privacy overlay network.
  --saas-url <url>         Override SaaS API URL.
  --testnet                Use cryptocurrency testnets (no real money).
  --help                   Show this help message and exit.

Examples:
  # Zero-config (most common)
  curl -sSL https://get.mobazha.org/standalone | sudo bash

  # With a pre-configured domain
  curl ... | sudo bash -s -- --domain shop.example.com

  # Privacy mode with Tor
  curl ... | sudo bash -s -- --overlay tor

Environment:
  INSTALL_DIR     Installation directory (default: /opt/mobazha)
  SAAS_API_URL    SaaS API URL (default: https://app.mobazha.org)
HELP
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --domain)
                STORE_DOMAIN="${2:-}"
                if [ -z "$STORE_DOMAIN" ]; then
                    err "--domain requires a value."
                    exit 1
                fi
                shift 2
                ;;
            --overlay)
                OVERLAY_TYPE="${2:-}"
                if [ -z "$OVERLAY_TYPE" ] || { [ "$OVERLAY_TYPE" != "tor" ] && [ "$OVERLAY_TYPE" != "lokinet" ]; }; then
                    err "--overlay requires 'tor' or 'lokinet'."
                    exit 1
                fi
                CONNECTIVITY="overlay"
                shift 2
                ;;
            --saas-url)
                SAAS_API_URL="${2:-}"
                if [ -z "$SAAS_API_URL" ]; then
                    err "--saas-url requires a value."
                    exit 1
                fi
                shift 2
                ;;
            --testnet)
                TESTNET="true"
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                err "Unknown argument: $1"
                echo "Run 'install.sh --help' for usage."
                exit 1
                ;;
        esac
    done
}

# --- Main ---

main() {
    parse_args "$@"

    log "Mobazha Standalone Store Installer"
    echo ""

    check_root
    install_docker
    install_docker_compose

    STANDALONE_API_KEY="$(generate_api_key)"
    PUBLIC_IP="$(detect_public_ip)"

    log "Detected public IP: ${PUBLIC_IP}"

    setup_files
    setup_auto_update
    start_services
    print_summary
}

main "$@"

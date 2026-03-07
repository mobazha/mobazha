#!/usr/bin/env bash
set -euo pipefail

# Mobazha Standalone Store — One-Click Installer
#
# Usage:
#   curl -sSL https://get.mobazha.org/standalone | bash
#   # or
#   bash install.sh
#
# Tested on: Ubuntu 22.04+, Debian 12+

INSTALL_DIR="${INSTALL_DIR:-/opt/mobazha}"
COMPOSE_URL="https://raw.githubusercontent.com/mobazha/mobazha3.0/main/deploy/standalone/docker-compose.yml"
ENV_URL="https://raw.githubusercontent.com/mobazha/mobazha3.0/main/deploy/standalone/.env.example"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[mobazha]${NC} $*"; }
warn() { echo -e "${YELLOW}[mobazha]${NC} $*"; }
err()  { echo -e "${RED}[mobazha]${NC} $*" >&2; }

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

prompt_config() {
    echo ""
    echo -e "${BOLD}=== Mobazha Standalone Store Setup ===${NC}"
    echo ""

    read -rp "Domain name (leave empty for IP mode): " STORE_DOMAIN
    STORE_DOMAIN="${STORE_DOMAIN:-}"

    STANDALONE_API_KEY="$(generate_api_key)"
    log "Generated API Key: $STANDALONE_API_KEY"
    log "(Save this key — you'll need it when registering with the SaaS platform)"

    echo ""
    read -rsp "Admin password (leave empty to auto-generate): " ADMIN_PASSWORD
    echo ""
    ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"

    read -rp "SaaS API URL [https://store.mobazha.org]: " SAAS_API_URL
    SAAS_API_URL="${SAAS_API_URL:-https://store.mobazha.org}"

    echo ""
    echo "Connectivity mode:"
    echo "  public  — Store has a public IP/domain (direct HTTPS)"
    echo "  tunnel  — Behind NAT with Cloudflare Tunnel"
    echo "  nat     — Behind NAT, managed via SaaS P2P proxy"
    read -rp "Connectivity mode [nat]: " CONNECTIVITY
    CONNECTIVITY="${CONNECTIVITY:-nat}"

    if [ "$CONNECTIVITY" = "nat" ] || [ "$CONNECTIVITY" = "tunnel" ]; then
        read -rp "SaaS Default Node Peer ID (for remote management): " SAAS_PEER_ID
        SAAS_PEER_ID="${SAAS_PEER_ID:-}"
    fi
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

    cat > .env <<EOF
STORE_DOMAIN=${STORE_DOMAIN}
SAAS_API_URL=${SAAS_API_URL}
STANDALONE_API_KEY=${STANDALONE_API_KEY}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
SAAS_PEER_ID=${SAAS_PEER_ID:-}
CONNECTIVITY=${CONNECTIVITY:-nat}
TAG=stable
EOF
    chmod 600 .env
    log "Created .env configuration"
}

start_services() {
    log "Pulling images..."
    docker compose pull

    log "Starting services..."
    docker compose up -d

    log "Waiting for health check..."
    local retries=0
    while ! docker compose ps --format json 2>/dev/null | grep -q '"healthy"'; do
        retries=$((retries + 1))
        if [ $retries -gt 60 ]; then
            warn "Health check timeout. Check logs with: docker compose logs -f"
            return
        fi
        sleep 2
    done
    log "Services are healthy!"
}

print_summary() {
    echo ""
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Mobazha Standalone Store is running!${NC}"
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo ""

    if [ -n "$STORE_DOMAIN" ]; then
        echo -e "  Store URL:   ${BOLD}https://${STORE_DOMAIN}${NC}"
    else
        local ip
        ip=$(curl -4 -sf https://ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
        echo -e "  Store URL:   ${BOLD}https://${ip}${NC} (self-signed TLS)"
    fi

    echo -e "  Admin:       ${BOLD}https://${STORE_DOMAIN:-<your-ip>}/admin/${NC}"
    echo -e "  Username:    ${BOLD}admin${NC}"
    if [ -n "$ADMIN_PASSWORD" ]; then
        echo -e "  Password:    ${BOLD}(as configured)${NC}"
    else
        echo -e "  Password:    ${BOLD}(check logs: docker compose logs store | grep password)${NC}"
    fi
    echo ""
    echo -e "  API Key:     ${BOLD}${STANDALONE_API_KEY}${NC}"
    echo ""
    echo -e "  Install dir: ${BOLD}${INSTALL_DIR}${NC}"
    echo -e "  Logs:        docker compose -f ${INSTALL_DIR}/docker-compose.yml logs -f"
    echo -e "  Stop:        docker compose -f ${INSTALL_DIR}/docker-compose.yml down"
    echo ""
    echo -e "  ${YELLOW}Next steps:${NC}"
    echo -e "    1. Open the admin panel and change the default password"
    echo -e "    2. Set up your store profile"
    echo -e "    3. Register with the SaaS platform (to enable buyer features)"
    echo -e "    4. Publish your first listing"
    echo ""
}

main() {
    log "Mobazha Standalone Store Installer"
    echo ""

    check_root
    install_docker
    install_docker_compose
    prompt_config
    setup_files
    start_services
    print_summary
}

main "$@"

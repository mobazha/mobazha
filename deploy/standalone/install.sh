#!/usr/bin/env bash
set -euo pipefail

# Mobazha Standalone Store — One-Click Installer
#
# Usage:
#   Interactive mode (manual prompts):
#     curl -sSL https://get.mobazha.org/standalone | sudo bash
#
#   Token mode (pre-configured via SaaS Deploy Wizard, zero interaction):
#     curl -sSL https://get.mobazha.org/i/<token> | sudo bash
#     # or
#     sudo bash install.sh <token>
#
#   Options:
#     --dry-run    Print generated .env without installing
#     --help       Show this help message
#
# Tested on: Ubuntu 22.04+, Debian 12+

INSTALL_DIR="${INSTALL_DIR:-/opt/mobazha}"
COMPOSE_URL="https://raw.githubusercontent.com/mobazha/mobazha3.0/main/deploy/standalone/docker-compose.yml"
SAAS_BASE_URL="${SAAS_BASE_URL:-https://app.mobazha.org}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

DEPLOY_TOKEN=""
DRY_RUN=false

STORE_DOMAIN=""
STANDALONE_API_KEY=""
ADMIN_PASSWORD=""
SAAS_API_URL=""
SAAS_PEER_ID=""
CONNECTIVITY="nat"

log()  { echo -e "${GREEN}[mobazha]${NC} $*"; }
warn() { echo -e "${YELLOW}[mobazha]${NC} $*"; }
err()  { echo -e "${RED}[mobazha]${NC} $*" >&2; }

# --- Token Detection ---

detect_token() {
    local arg="${1:-}"
    if [[ "$arg" =~ ^[0-9a-f]{64}$ ]]; then
        DEPLOY_TOKEN="$arg"
        return 0
    fi
    return 1
}

# --- SaaS API Integration ---

fetch_config() {
    local token="$1"
    local url="${SAAS_BASE_URL}/platform/v1/deploy/config/${token}"
    local response

    log "Fetching deployment configuration..."
    if ! response=$(curl -sSf --connect-timeout 15 --max-time 30 "$url" 2>&1); then
        err "Failed to fetch config from SaaS. The token may be expired or invalid."
        err "Response: $response"
        exit 1
    fi

    echo "$response"
}

parse_config() {
    local json="$1"

    STORE_DOMAIN=$(echo "$json" | jq -r '.data.domain // empty')
    STANDALONE_API_KEY=$(echo "$json" | jq -r '.data.apiKey // empty')
    SAAS_API_URL=$(echo "$json" | jq -r '.data.saasUrl // empty')
    ADMIN_PASSWORD=$(echo "$json" | jq -r '.data.adminPassword // empty')
    CONNECTIVITY=$(echo "$json" | jq -r '.data.connectivity // "nat"')
    SAAS_PEER_ID=$(echo "$json" | jq -r '.data.saasPeerID // empty')

    if [ -z "$STANDALONE_API_KEY" ]; then
        err "Invalid config response: missing apiKey."
        exit 1
    fi

    log "Configuration loaded: domain=${STORE_DOMAIN:-<ip-mode>}, connectivity=${CONNECTIVITY}"
}

report_progress() {
    if [ -z "$DEPLOY_TOKEN" ]; then
        return 0
    fi

    local stage="$1"
    local status="$2"
    local url="${SAAS_BASE_URL}/platform/v1/deploy/progress/${DEPLOY_TOKEN}"

    curl -sS --connect-timeout 10 --max-time 15 -X POST "$url" \
        -H 'Content-Type: application/json' \
        -d "{\"stage\":\"${stage}\",\"status\":\"${status}\"}" \
        >/dev/null 2>&1 || warn "Progress report failed for stage=$stage (non-fatal)"
}

# --- Prerequisites ---

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "This script must be run as root (or with sudo)."
        exit 1
    fi
}

check_jq() {
    if ! command -v jq &>/dev/null; then
        log "Installing jq..."
        apt-get update -qq && apt-get install -qq -y jq
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

# --- Interactive Configuration ---

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

# --- File Setup ---

write_env() {
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

    write_env
    log "Created .env configuration"
}

# --- Services ---

start_services() {
    log "Pulling images..."
    report_progress "image_pulling" "in_progress"
    docker compose pull
    report_progress "image_pulled" "completed"

    log "Starting services..."
    report_progress "services_starting" "in_progress"
    docker compose up -d
    report_progress "services_started" "completed"

    log "Waiting for health check..."
    report_progress "health_check" "in_progress"
    local retries=0
    while ! docker compose ps --format json 2>/dev/null | grep -q '"healthy"'; do
        retries=$((retries + 1))
        if [ $retries -gt 60 ]; then
            report_progress "health_check" "timeout"
            warn "Health check timeout. Check logs with: docker compose logs -f"
            return
        fi
        sleep 2
    done
    report_progress "health_check" "completed"
    log "Services are healthy!"
}

# --- Summary ---

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

# --- CLI ---

show_help() {
    cat <<'HELP'
Mobazha Standalone Store — One-Click Installer

Usage:
  sudo bash install.sh [options] [token]

Arguments:
  token       64-char hex deploy token from the SaaS Deploy Wizard.
              When provided, all configuration is fetched from the
              SaaS API automatically (zero interaction).

Options:
  --dry-run   Print the generated .env file without installing.
              Useful for inspecting configuration before committing.
  --help      Show this help message and exit.

Modes:
  Interactive (no token):
    Prompts for domain, connectivity, password, etc.

  Token (with token):
    Fetches pre-configured settings from SaaS Deploy Wizard.
    No prompts. Ideal for automated / one-click deployment.

Examples:
  # Interactive mode
  sudo bash install.sh

  # Token mode (from Deploy Wizard)
  sudo bash install.sh a3f8c1d2e4...

  # Inspect config without installing
  sudo bash install.sh --dry-run a3f8c1d2e4...

Environment:
  INSTALL_DIR     Installation directory (default: /opt/mobazha)
  SAAS_BASE_URL   SaaS platform base URL (default: https://app.mobazha.org)
HELP
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                if detect_token "$1"; then
                    shift
                else
                    err "Unknown argument: $1"
                    echo "Run 'install.sh --help' for usage."
                    exit 1
                fi
                ;;
        esac
    done
}

# --- Main ---

main() {
    parse_args "$@"

    log "Mobazha Standalone Store Installer"
    echo ""

    if [ -n "$DEPLOY_TOKEN" ]; then
        log "Token mode: configuration will be fetched from SaaS."

        if ! command -v jq &>/dev/null && [ "$DRY_RUN" = true ]; then
            err "jq is required for token mode. Install it with: sudo apt-get install jq"
            exit 1
        fi

        if [ "$DRY_RUN" != true ]; then
            check_root
            check_jq
        fi

        local config_json
        config_json=$(fetch_config "$DEPLOY_TOKEN")
        parse_config "$config_json"

        if [ "$DRY_RUN" = true ]; then
            log "Dry-run mode — generated .env contents:"
            echo "---"
            echo "STORE_DOMAIN=${STORE_DOMAIN}"
            echo "SAAS_API_URL=${SAAS_API_URL}"
            echo "STANDALONE_API_KEY=${STANDALONE_API_KEY}"
            echo "ADMIN_PASSWORD=${ADMIN_PASSWORD}"
            echo "SAAS_PEER_ID=${SAAS_PEER_ID:-}"
            echo "CONNECTIVITY=${CONNECTIVITY:-nat}"
            echo "TAG=stable"
            echo "---"
            exit 0
        fi

        report_progress "docker_installing" "in_progress"
        install_docker
        report_progress "docker_installed" "completed"

        install_docker_compose
        setup_files
        start_services
        report_progress "completed" "completed"
        print_summary
    else
        if [ "$DRY_RUN" = true ]; then
            err "--dry-run requires a deploy token."
            exit 1
        fi

        check_root
        install_docker
        install_docker_compose
        prompt_config
        setup_files
        start_services
        print_summary
    fi
}

main "$@"

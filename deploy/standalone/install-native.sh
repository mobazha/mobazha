#!/usr/bin/env bash
set -euo pipefail

# Mobazha Standalone Store — Native Binary Installer (no Docker)
#
# Usage:
#   curl -sSL https://get.mobazha.org/native | sudo bash
#
# Installs mobazha binary + Caddy + systemd services.
# For users who prefer not to use Docker.
#
# Tested on: Ubuntu 22.04+, Debian 12+

INSTALL_DIR="/var/lib/mobazha"
BIN_DIR="/usr/local/bin"
CONF_DIR="/etc/mobazha"
LOG_DIR="/var/log/mobazha"
SAAS_BASE_URL="${SAAS_BASE_URL:-https://app.mobazha.org}"

MOBAZHA_VERSION="${MOBAZHA_VERSION:-latest}"
GITHUB_BASE="https://github.com/mobazha/mobazha/releases"

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

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) err "Unsupported architecture: $arch"; exit 1 ;;
    esac
}

detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux) echo "linux" ;;
        *) err "Unsupported OS: $os"; exit 1 ;;
    esac
}

install_caddy() {
    if command -v caddy &>/dev/null; then
        log "Caddy is already installed: $(caddy version)"
        return
    fi

    log "Installing Caddy..."
    apt-get update -qq
    apt-get install -qq -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -qq
    apt-get install -qq -y caddy
    log "Caddy installed."
}

download_binary() {
    local os arch url
    os=$(detect_os)
    arch=$(detect_arch)

    if [ "$MOBAZHA_VERSION" = "latest" ]; then
        url="${GITHUB_BASE}/latest/download/mobazha-${os}-${arch}"
    else
        url="${GITHUB_BASE}/download/${MOBAZHA_VERSION}/mobazha-${os}-${arch}"
    fi

    log "Downloading mobazha binary (${os}/${arch})..."
    curl -sSL -o "${BIN_DIR}/mobazha" "$url"
    chmod +x "${BIN_DIR}/mobazha"
    log "Binary installed to ${BIN_DIR}/mobazha"
}

create_user() {
    if id -u mobazha &>/dev/null; then
        log "User 'mobazha' already exists."
        return
    fi

    log "Creating system user 'mobazha'..."
    useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin mobazha
}

setup_directories() {
    log "Setting up directories..."
    mkdir -p "$INSTALL_DIR" "$CONF_DIR" "$LOG_DIR"
    chown -R mobazha:mobazha "$INSTALL_DIR" "$LOG_DIR"
    chmod 750 "$INSTALL_DIR" "$LOG_DIR"
}

prompt_config() {
    echo ""
    echo -e "${BOLD}=== Mobazha Standalone Store Setup (Native) ===${NC}"
    echo ""

    read -rp "Domain name (leave empty for IP mode): " STORE_DOMAIN
    STORE_DOMAIN="${STORE_DOMAIN:-}"

    read -rsp "Admin password (leave empty to auto-generate): " ADMIN_PASSWORD
    echo ""
    ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"

    cat > "${CONF_DIR}/env" <<EOF
# Reference configuration — the binary auto-registers with SaaS on first boot.
STORE_DOMAIN=${STORE_DOMAIN}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
EOF
    chmod 600 "${CONF_DIR}/env"
    log "Configuration written to ${CONF_DIR}/env"
}

install_systemd() {
    local service_url="https://raw.githubusercontent.com/mobazha/mobazha/main/deploy/standalone/systemd/mobazha.service"

    log "Installing systemd service..."
    curl -sSL "$service_url" -o /etc/systemd/system/mobazha.service
    systemctl daemon-reload
    systemctl enable mobazha
    log "Service installed and enabled."
}

configure_caddy() {
    local domain="${STORE_DOMAIN:-localhost}"

    log "Configuring Caddy for ${domain}..."
    cat > /etc/caddy/Caddyfile <<EOF
${domain} {
    reverse_proxy localhost:5102
    encode gzip
    log {
        output file /var/log/mobazha/caddy.log
    }
}
EOF

    if [ -z "$STORE_DOMAIN" ]; then
        cat > /etc/caddy/Caddyfile <<'EOF'
:443 {
    tls internal
    reverse_proxy localhost:5102
    encode gzip
    log {
        output file /var/log/mobazha/caddy.log
    }
}
EOF
    fi

    systemctl restart caddy
    log "Caddy configured and restarted."
}

start_services() {
    log "Starting Mobazha node..."
    systemctl start mobazha

    log "Waiting for health check..."
    local retries=0
    while ! curl -sf http://localhost:5102/healthz &>/dev/null; do
        retries=$((retries + 1))
        if [ $retries -gt 30 ]; then
            warn "Health check timeout. Check: journalctl -u mobazha -f"
            return
        fi
        sleep 2
    done
    log "Mobazha node is healthy!"
}

print_summary() {
    echo ""
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Mobazha Standalone Store is running (native)!${NC}"
    echo -e "${BOLD}════════════════════════════════════════════════${NC}"
    echo ""

    if [ -n "${STORE_DOMAIN:-}" ]; then
        echo -e "  Store URL:   ${BOLD}https://${STORE_DOMAIN}${NC}"
    else
        local ip
        ip=$(curl -4 -sf https://ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
        echo -e "  Store URL:   ${BOLD}https://${ip}${NC} (self-signed TLS)"
    fi

    echo -e "  Data dir:    ${BOLD}${INSTALL_DIR}${NC}"
    echo -e "  Config:      ${BOLD}${CONF_DIR}/env${NC}"
    echo -e "  Logs:        journalctl -u mobazha -f"
    echo ""
    echo -e "  Manage:"
    echo -e "    systemctl status mobazha    # check status"
    echo -e "    systemctl restart mobazha   # restart"
    echo -e "    systemctl stop mobazha      # stop"
    echo -e "    mobazha doctor              # diagnose issues"
    echo ""
}

main() {
    log "Mobazha Standalone Store — Native Installer"
    echo ""

    check_root
    download_binary
    create_user
    setup_directories
    install_caddy
    prompt_config
    install_systemd
    configure_caddy
    start_services
    print_summary
}

main "$@"

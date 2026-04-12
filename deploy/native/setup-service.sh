#!/bin/bash
set -euo pipefail

# Sets up Mobazha as a system service (systemd on Linux, launchd on macOS).
# Usage:
#   sudo ./setup-service.sh install   # Install and enable the service
#   sudo ./setup-service.sh uninstall # Remove the service

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ACTION="${1:-install}"

setup_systemd() {
    case "$ACTION" in
        install)
            if ! id -u mobazha &>/dev/null; then
                useradd --system --home-dir /var/lib/mobazha --shell /usr/sbin/nologin mobazha
            fi
            mkdir -p /var/lib/mobazha
            chown mobazha:mobazha /var/lib/mobazha

            cp "$SCRIPT_DIR/systemd/mobazha.service" /etc/systemd/system/
            systemctl daemon-reload
            systemctl enable mobazha
            systemctl start mobazha
            echo "Mobazha service installed and started."
            echo "  systemctl status mobazha    # Check status"
            echo "  journalctl -u mobazha -f    # View logs"
            ;;
        uninstall)
            systemctl stop mobazha 2>/dev/null || true
            systemctl disable mobazha 2>/dev/null || true
            rm -f /etc/systemd/system/mobazha.service
            systemctl daemon-reload
            echo "Mobazha service removed. Data directory preserved at /var/lib/mobazha"
            ;;
        *)
            echo "Usage: $0 {install|uninstall}"
            exit 1
            ;;
    esac
}

setup_launchd() {
    local plist_src="$SCRIPT_DIR/launchd/org.mobazha.node.plist"
    local plist_dst="$HOME/Library/LaunchAgents/org.mobazha.node.plist"

    case "$ACTION" in
        install)
            mkdir -p /usr/local/var/lib/mobazha /usr/local/var/log
            mkdir -p "$HOME/Library/LaunchAgents"
            cp "$plist_src" "$plist_dst"
            launchctl load "$plist_dst"
            echo "Mobazha service installed and started."
            echo "  launchctl list org.mobazha.node     # Check status"
            echo "  tail -f /usr/local/var/log/mobazha.log  # View logs"
            ;;
        uninstall)
            launchctl unload "$plist_dst" 2>/dev/null || true
            rm -f "$plist_dst"
            echo "Mobazha service removed. Data preserved at /usr/local/var/lib/mobazha"
            ;;
        *)
            echo "Usage: $0 {install|uninstall}"
            exit 1
            ;;
    esac
}

case "$(uname -s)" in
    Linux)  setup_systemd ;;
    Darwin) setup_launchd ;;
    *)      echo "Unsupported OS: $(uname -s)"; exit 1 ;;
esac

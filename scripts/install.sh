#!/bin/bash
set -euo pipefail

# Mobazha native binary installer / uninstaller
# Usage:
#   curl -sSL https://get.mobazha.org/install | bash                        # install latest
#   curl -sSL https://get.mobazha.org/install | bash -s -- --version v0.1.0 # install specific
#   curl -sSL https://get.mobazha.org/install | bash -s -- --uninstall      # uninstall
#
# Binaries are published to the public mobazha.org repo on GitHub as
# releases with tag prefix "native-".

REPO="mobazha/mobazha.org"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="mobazha"
TAG_PREFIX="native-"
DATA_DIR="${HOME}/.mobazha"
GATEWAY_PORT="5102"

ACTION="install"
VERSION=""
PURGE=false
AUTO_START=true

while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall)  ACTION="uninstall"; shift ;;
        --purge)      PURGE=true; shift ;;
        --version)    VERSION="$2"; shift 2 ;;
        --dir)        INSTALL_DIR="$2"; shift 2 ;;
        --no-start)   AUTO_START=false; shift ;;
        --help|-h)    ACTION="help"; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)  os="linux" ;;
        darwin) os="darwin" ;;
        *)      echo "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)             echo "Unsupported architecture: $arch"; exit 1 ;;
    esac

    echo "${os}-${arch}"
}

get_latest_version() {
    local tag
    tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
        | grep '"tag_name"' \
        | grep "\"${TAG_PREFIX}" \
        | head -1 \
        | sed -E 's/.*"([^"]+)".*/\1/')"
    if [ -z "$tag" ]; then
        echo "Error: no native releases found in ${REPO}" >&2
        exit 1
    fi
    echo "$tag"
}

detect_public_ip() {
    local ip
    ip="$(curl -fsSL --connect-timeout 3 https://ifconfig.me 2>/dev/null)" && echo "$ip" && return
    ip="$(curl -fsSL --connect-timeout 3 https://api.ipify.org 2>/dev/null)" && echo "$ip" && return
    echo "localhost"
}

ensure_in_path() {
    if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        return 0
    fi

    local shell_rc="" path_line='export PATH="$HOME/.local/bin:$PATH"'

    case "$(basename "${SHELL:-/bin/bash}")" in
        zsh)
            shell_rc="$HOME/.zshrc"
            ;;
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                shell_rc="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                shell_rc="$HOME/.bash_profile"
            fi
            ;;
        fish)
            shell_rc="$HOME/.config/fish/config.fish"
            path_line="fish_add_path $HOME/.local/bin"
            ;;
    esac

    if [ -n "$shell_rc" ]; then
        if ! grep -qF '.local/bin' "$shell_rc" 2>/dev/null; then
            echo "$path_line" >> "$shell_rc"
            echo "   Added ${INSTALL_DIR} to PATH in ${shell_rc}"
        fi
        export PATH="$INSTALL_DIR:$PATH"
    else
        echo "   ⚠️  Add ${INSTALL_DIR} to your PATH manually."
    fi
}

do_remove() {
    local target="$1"
    if [ -w "$(dirname "$target")" ]; then
        rm -f "$target"
    else
        sudo rm -f "$target"
    fi
}

do_install() {
    echo "🔍 Detecting platform..."
    local platform
    platform="$(detect_platform)"
    echo "   Platform: $platform"

    if [ -z "$VERSION" ]; then
        echo "🔍 Fetching latest release..."
        VERSION="$(get_latest_version)"
    elif [[ "$VERSION" != ${TAG_PREFIX}* ]]; then
        VERSION="${TAG_PREFIX}${VERSION}"
    fi
    echo "   Version:  $VERSION"

    local url="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${platform}"
    local checksum_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums-sha256.txt"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "${tmpdir:-}"' EXIT

    # --- Download node binary ---
    echo "⬇️  Downloading ${BINARY_NAME}..."
    curl -fL# -o "${tmpdir}/${BINARY_NAME}" "$url"

    echo "🔐 Verifying checksum..."
    if curl -fsSL -o "${tmpdir}/checksums-sha256.txt" "$checksum_url" 2>/dev/null; then
        local expected actual
        expected="$(grep "${BINARY_NAME}-${platform}" "${tmpdir}/checksums-sha256.txt" | awk '{print $1}')"
        if [ -z "$expected" ]; then
            echo "⚠️  No checksum entry for ${BINARY_NAME}-${platform}, skipping."
        else
            if command -v sha256sum &>/dev/null; then
                actual="$(sha256sum "${tmpdir}/${BINARY_NAME}" | awk '{print $1}')"
            else
                actual="$(shasum -a 256 "${tmpdir}/${BINARY_NAME}" | awk '{print $1}')"
            fi
            if [ "$expected" != "$actual" ]; then
                echo "❌ Checksum verification failed! Aborting."
                echo "   Expected: $expected"
                echo "   Actual:   $actual"
                exit 1
            fi
        fi
    else
        echo "⚠️  Checksum file not available, skipping verification."
    fi

    chmod +x "${tmpdir}/${BINARY_NAME}"

    # --- Download launcher binary ---
    local launcher_available=false
    local launcher_name="${BINARY_NAME}-launcher-${platform}"
    local launcher_url="https://github.com/${REPO}/releases/download/${VERSION}/${launcher_name}"
    echo "⬇️  Downloading launcher..."
    if curl -fL# -o "${tmpdir}/${BINARY_NAME}-launcher" "$launcher_url" 2>/dev/null; then
        chmod +x "${tmpdir}/${BINARY_NAME}-launcher"
        launcher_available=true
    else
        echo "⚠️  Launcher binary not available for this platform, skipping."
    fi

    # --- Install binaries ---
    mkdir -p "$INSTALL_DIR"
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        if $launcher_available; then
            mv "${tmpdir}/${BINARY_NAME}-launcher" "${INSTALL_DIR}/${BINARY_NAME}-launcher"
        fi
    else
        echo "📦 Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        if $launcher_available; then
            sudo mv "${tmpdir}/${BINARY_NAME}-launcher" "${INSTALL_DIR}/${BINARY_NAME}-launcher"
        fi
    fi

    # --- macOS: clear quarantine attribute ---
    if [[ "$(uname -s)" == "Darwin" ]]; then
        xattr -d com.apple.quarantine "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null || true
        if $launcher_available; then
            xattr -d com.apple.quarantine "${INSTALL_DIR}/${BINARY_NAME}-launcher" 2>/dev/null || true
        fi
    fi

    # --- Ensure INSTALL_DIR is in PATH ---
    ensure_in_path

    echo ""
    echo "📦 Mobazha ${VERSION} installed to ${INSTALL_DIR}/"

    # --- Auto-start service ---
    if $AUTO_START; then
        echo ""
        echo "🚀 Starting Mobazha service..."
        if "${INSTALL_DIR}/${BINARY_NAME}" service install > /dev/null 2>&1; then
            local public_ip
            public_ip="$(detect_public_ip)"

            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            if $launcher_available; then
                echo "✅ Mobazha is running! (with auto-update)"
            else
                echo "✅ Mobazha is running!"
            fi
            echo ""
            echo "   🌐 Your store:  http://${public_ip}:${GATEWAY_PORT}"
            echo ""
            echo "   It may take a minute for the first startup."
            echo "   Open the URL above in your browser to set up your store."
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "Commands:"
            echo "  mobazha service status    Check service status"
            echo "  mobazha service stop      Stop the node"
            echo "  mobazha service start     Start the node"
            echo "  mobazha doctor            System health check"
            echo "  mobazha backup            Back up data"
            echo ""
            echo "Uninstall:"
            echo "  curl -sSL https://get.mobazha.org/install | bash -s -- --uninstall"
        else
            echo "⚠️  Service registration failed. Start manually:"
            print_manual_start "$launcher_available"
        fi
    else
        echo ""
        print_manual_start "$launcher_available"
    fi
}

print_manual_start() {
    local launcher_available="${1:-false}"

    if [ "$launcher_available" = "true" ]; then
        echo "Start your store:"
        echo "  ${BINARY_NAME}-launcher           # Recommended: with auto-update"
        echo "  ${BINARY_NAME} service install    # Run as background service"
    else
        echo "Start your store:"
        echo "  ${BINARY_NAME} service install    # Run as background service"
        echo "  ${BINARY_NAME} start              # Start in foreground"
    fi
    echo ""
    echo "After starting, open your browser:"
    echo "  http://localhost:${GATEWAY_PORT}"
    echo ""
    echo "Uninstall:"
    echo "  curl -sSL https://get.mobazha.org/install | bash -s -- --uninstall"
}

do_uninstall() {
    echo "🗑️  Uninstalling Mobazha..."

    # Stop the service if running
    if [[ "$(uname -s)" == "Linux" ]] && command -v systemctl &>/dev/null; then
        if systemctl is-active --quiet mobazha 2>/dev/null; then
            echo "   Stopping systemd service..."
            sudo systemctl stop mobazha
        fi
        if systemctl is-enabled --quiet mobazha 2>/dev/null; then
            echo "   Disabling systemd service..."
            sudo systemctl disable mobazha
            sudo rm -f /etc/systemd/system/mobazha.service
            sudo systemctl daemon-reload
        fi
        # Also check user-level systemd
        if systemctl --user is-active --quiet mobazha 2>/dev/null; then
            systemctl --user stop mobazha
        fi
        if systemctl --user is-enabled --quiet mobazha 2>/dev/null; then
            systemctl --user disable mobazha
            local user_unit
            user_unit="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/mobazha.service"
            rm -f "$user_unit"
            systemctl --user daemon-reload
        fi
    elif [[ "$(uname -s)" == "Darwin" ]]; then
        local plist="$HOME/Library/LaunchAgents/org.mobazha.node.plist"
        if [ -f "$plist" ]; then
            echo "   Unloading launchd service..."
            launchctl unload "$plist" 2>/dev/null || true
            rm -f "$plist"
        fi
    fi

    # Remove binaries (check both new and legacy install dirs)
    local found=false
    for dir in "$INSTALL_DIR" "/usr/local/bin"; do
        for name in "${BINARY_NAME}" "${BINARY_NAME}-launcher" "${BINARY_NAME}-tray"; do
            local binary="${dir}/${name}"
            if [ -f "$binary" ]; then
                echo "   Removing ${binary}..."
                do_remove "$binary"
                found=true
            fi
        done
    done
    if ! $found; then
        echo "   Binary not found, skipping."
    fi

    # Purge data if requested
    if $PURGE; then
        echo "   ⚠️  Removing data directory ${DATA_DIR}..."
        rm -rf "$DATA_DIR"
        if [[ "$(uname -s)" == "Linux" ]]; then
            sudo rm -rf /var/lib/mobazha 2>/dev/null || true
        elif [[ "$(uname -s)" == "Darwin" ]]; then
            rm -rf /usr/local/var/lib/mobazha 2>/dev/null || true
        fi
    fi

    echo ""
    echo "✅ Mobazha uninstalled."
    if ! $PURGE; then
        echo "   Data directory preserved at ${DATA_DIR}"
        echo "   To also remove data: add --purge flag"
    fi
}

show_help() {
    cat <<'HELP'
Mobazha Installer

INSTALL:
  curl -sSL https://get.mobazha.org/install | bash
  curl -sSL https://get.mobazha.org/install | bash -s -- --version v0.1.0
  curl -sSL https://get.mobazha.org/install | bash -s -- --dir /opt/bin

UNINSTALL:
  curl -sSL https://get.mobazha.org/install | bash -s -- --uninstall
  curl -sSL https://get.mobazha.org/install | bash -s -- --uninstall --purge

OPTIONS:
  --version <tag>   Install a specific version (e.g. v0.1.0)
  --dir <path>      Install directory (default: ~/.local/bin)
  --no-start        Don't register/start the background service after install
  --uninstall       Remove Mobazha binary and system service
  --purge           Also remove data directory (use with --uninstall)
  --help            Show this help message
HELP
}

case "$ACTION" in
    install)   do_install ;;
    uninstall) do_uninstall ;;
    help)      show_help ;;
esac

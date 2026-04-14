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

ACTION="install"
VERSION=""
PURGE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall)  ACTION="uninstall"; shift ;;
        --purge)      PURGE=true; shift ;;
        --version)    VERSION="$2"; shift 2 ;;
        --dir)        INSTALL_DIR="$2"; shift 2 ;;
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

    echo "⬇️  Downloading ${BINARY_NAME}-${platform}..."
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

    # On macOS, also download the desktop tray binary (system tray icon + auto-open browser)
    local tray_available=false
    if [[ "$platform" == darwin-* ]]; then
        local tray_name="${BINARY_NAME}-tray-${platform}"
        local tray_url="https://github.com/${REPO}/releases/download/${VERSION}/${tray_name}"
        echo "⬇️  Downloading desktop tray (${tray_name})..."
        if curl -fL# -o "${tmpdir}/${BINARY_NAME}-tray" "$tray_url" 2>/dev/null; then
            chmod +x "${tmpdir}/${BINARY_NAME}-tray"
            tray_available=true
        else
            echo "⚠️  Tray binary not available for this version, skipping."
        fi
    fi

    mkdir -p "$INSTALL_DIR"
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        if $tray_available; then
            mv "${tmpdir}/${BINARY_NAME}-tray" "${INSTALL_DIR}/${BINARY_NAME}-tray"
        fi
    else
        echo "📦 Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        if $tray_available; then
            sudo mv "${tmpdir}/${BINARY_NAME}-tray" "${INSTALL_DIR}/${BINARY_NAME}-tray"
        fi
    fi

    echo ""
    echo "✅ Mobazha ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}"
    if $tray_available; then
        echo "   Desktop tray also installed: ${INSTALL_DIR}/${BINARY_NAME}-tray"
    fi

    # Check if INSTALL_DIR is in PATH; if not, advise the user.
    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        echo ""
        echo "⚠️  ${INSTALL_DIR} is not in your PATH."
        local shell_rc=""
        case "$(basename "${SHELL:-/bin/bash}")" in
            zsh)  shell_rc="~/.zshrc" ;;
            bash) shell_rc="~/.bashrc" ;;
            fish) shell_rc="~/.config/fish/config.fish" ;;
            *)    shell_rc="your shell profile" ;;
        esac
        echo "   Add it by running:"
        echo ""
        echo "     echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ${shell_rc}"
        echo "     source ${shell_rc}"
    fi

    echo ""
    if $tray_available; then
        echo "Quick start (Desktop — recommended for macOS):"
        echo "  ${BINARY_NAME}-tray              # Launch tray icon, auto-opens browser"
        echo ""
        echo "Or use the CLI:"
    else
        echo "Quick start:"
    fi
    echo "  ${BINARY_NAME} start             # Start the node (foreground)"
    echo "  ${BINARY_NAME} service install   # Run as background service"
    echo ""
    echo "After starting, open your browser:"
    echo "  http://localhost:4002"
    echo ""
    echo "Other commands:"
    echo "  ${BINARY_NAME} service status    # Check service status"
    echo "  ${BINARY_NAME} doctor            # Check system health"
    echo "  ${BINARY_NAME} backup            # Back up data"
    echo ""
    echo "To uninstall later:"
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
        for name in "${BINARY_NAME}" "${BINARY_NAME}-tray"; do
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

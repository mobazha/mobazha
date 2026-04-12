#!/bin/bash
set -euo pipefail

# Mobazha native binary installer
# Usage: curl -sSL https://get.mobazha.org/install | bash
#   or:  curl -sSL https://get.mobazha.org/install | bash -s -- --version v0.1.0
#
# Binaries are published to the public mobazha.org repo on GitHub as
# releases with tag prefix "native-".

REPO="mobazha/mobazha.org"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="mobazha"
TAG_PREFIX="native-"

VERSION=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --version) VERSION="$2"; shift 2 ;;
        --dir)     INSTALL_DIR="$2"; shift 2 ;;
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
    # Find the latest release with "native-" tag prefix
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

main() {
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

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    echo "⬇️  Downloading ${BINARY_NAME}-${platform}..."
    curl -fsSL -o "${tmpdir}/${BINARY_NAME}" "$url"

    echo "🔐 Verifying checksum..."
    if curl -fsSL -o "${tmpdir}/checksums-sha256.txt" "$checksum_url" 2>/dev/null; then
        if ! (cd "$tmpdir" && grep "${BINARY_NAME}-${platform}" checksums-sha256.txt | sed "s/${BINARY_NAME}-${platform}/${BINARY_NAME}/" | sha256sum -c --quiet 2>/dev/null); then
            echo "❌ Checksum verification failed! Aborting."
            exit 1
        fi
    else
        echo "⚠️  Checksum file not available, skipping verification."
    fi

    chmod +x "${tmpdir}/${BINARY_NAME}"

    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo "📦 Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    echo ""
    echo "✅ Mobazha ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}"
    echo ""
    echo "Quick start:"
    echo "  ${BINARY_NAME} init      # Initialize data directory"
    echo "  ${BINARY_NAME} start     # Start the node"
    echo "  ${BINARY_NAME} doctor    # Check system health"
    echo "  ${BINARY_NAME} backup    # Back up data"
}

main "$@"

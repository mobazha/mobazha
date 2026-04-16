#!/usr/bin/env bash
set -euo pipefail

# Build Mobazha Windows distribution zip.
#
# Usage:
#   ./scripts/build-windows-zip.sh [--version v0.1.0]
#
# The zip contains:
#   mobazha.exe           — CLI binary (CGO_ENABLED=0, cross-compiled)
#   mobazha-launcher.exe  — Desktop launcher with supervisor (CGO_ENABLED=1, native compile on Windows)
#   README.txt            — Quick start instructions
#
# On non-Windows hosts, only the CLI binary is included (launcher desktop mode
# requires native Windows compilation). The CI workflow builds the launcher
# binary on a windows-latest runner and merges them.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="${PROJECT_ROOT}/dist"
VERSION="${VERSION:-dev}"

while [[ $# -gt 0 ]]; do
    case $1 in
        --version) VERSION="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

STAGE_DIR="${DIST_DIR}/mobazha-windows"
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"

echo "==> Building Windows distribution (${VERSION})"

# --- CLI binary ---
MAIN_BINARY="${DIST_DIR}/mobazha-windows-amd64.exe"
if [ -f "$MAIN_BINARY" ]; then
    echo "==> Using pre-built CLI binary"
    cp "$MAIN_BINARY" "${STAGE_DIR}/mobazha.exe"
else
    echo "==> Building CLI binary (cross-compile)..."
    BUILD_TAGS="${BUILD_TAGS:-goolm purego_sqlite embed_frontend}"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
        -tags "${BUILD_TAGS}" \
        -ldflags="-s -w -X github.com/mobazha/mobazha3.0/internal/api.Version=${VERSION}" \
        -o "${STAGE_DIR}/mobazha.exe" \
        "${PROJECT_ROOT}"
fi

# --- Launcher binary (only on Windows, requires CGO for systray) ---
if [[ "$(uname -s)" == MINGW* ]] || [[ "$(uname -s)" == MSYS* ]] || [[ "${GOOS:-}" == "windows" ]]; then
    echo "==> Building launcher binary (native Windows, desktop mode)..."
    CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build \
        -tags "desktop" \
        -ldflags="-s -w -H windowsgui -X github.com/mobazha/mobazha3.0/internal/supervisor.Version=${VERSION}" \
        -o "${STAGE_DIR}/mobazha-launcher.exe" \
        "${PROJECT_ROOT}/cmd/mobazha-launcher"
else
    echo "==> Skipping launcher binary (not on Windows; CI will build this natively)"
fi

# --- README ---
cat > "${STAGE_DIR}/README.txt" <<'README'
Mobazha Standalone Store
========================

Quick Start
-----------

Option 1: Double-click mobazha-launcher.exe
   The launcher manages the node lifecycle with auto-updates and crash
   recovery. The system tray icon appears, and your browser opens to the
   setup wizard.

Option 2: Command line
   Open PowerShell or Command Prompt, navigate to this folder, then:

     .\mobazha.exe start

   Your store will be available at http://localhost:5102

Commands
--------
   .\mobazha.exe start             Start the node (foreground)
   .\mobazha.exe start --open      Start and open browser automatically
   .\mobazha.exe doctor            Check system health
   .\mobazha.exe backup            Back up store data
   .\mobazha.exe --help            Show all commands

More Info
---------
   https://mobazha.org/self-host
README

# --- Create zip ---
ZIP_NAME="Mobazha-${VERSION}-windows-amd64.zip"
echo "==> Creating ${ZIP_NAME}"
(cd "$DIST_DIR" && zip -r "$ZIP_NAME" mobazha-windows/)
echo "==> Created ${DIST_DIR}/${ZIP_NAME}"

rm -rf "$STAGE_DIR"
echo "==> Done!"

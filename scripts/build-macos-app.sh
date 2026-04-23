#!/usr/bin/env bash
set -euo pipefail

# Build Mobazha.app bundle for macOS.
#
# Usage:
#   ./scripts/build-macos-app.sh [--arch arm64|amd64] [--version v0.1.0]
#
# Prerequisites:
#   - macOS (cannot cross-compile the launcher desktop binary)
#   - Go toolchain
#   - The main `mobazha` binary already built for the target arch
#
# Output:
#   dist/Mobazha.app          — macOS application bundle
#   dist/Mobazha-{version}-{arch}.dmg  — disk image (if create-dmg is installed)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="${PROJECT_ROOT}/dist"
APP_DIR="${DIST_DIR}/Mobazha.app"

ARCH="${ARCH:-$(uname -m)}"
VERSION="${VERSION:-dev}"

while [[ $# -gt 0 ]]; do
    case $1 in
        --arch)    ARCH="$2"; shift 2 ;;
        --version) VERSION="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

case "$ARCH" in
    arm64|aarch64) GOARCH="arm64"; ARCH_LABEL="arm64" ;;
    x86_64|amd64)  GOARCH="amd64"; ARCH_LABEL="amd64" ;;
    *) echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

echo "==> Building Mobazha.app (${ARCH_LABEL}, ${VERSION})"

rm -rf "$APP_DIR"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

# --- Info.plist ---
cat > "${APP_DIR}/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Mobazha</string>
    <key>CFBundleDisplayName</key>
    <string>Mobazha</string>
    <key>CFBundleIdentifier</key>
    <string>org.mobazha.desktop</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>mobazha-launcher</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
</dict>
</plist>
PLIST

# --- Build launcher binary (requires CGO for systray desktop mode) ---
echo "==> Building mobazha-launcher (desktop)..."
CGO_ENABLED=1 GOARCH="${GOARCH}" go build \
    -tags "desktop" \
    -ldflags="-s -w -X github.com/mobazha/mobazha3.0/internal/supervisor.Version=${VERSION}" \
    -o "${APP_DIR}/Contents/MacOS/mobazha-launcher" \
    "${PROJECT_ROOT}/cmd/mobazha-launcher"

# --- Copy or build the main CLI binary ---
MAIN_BINARY="${DIST_DIR}/mobazha-darwin-${ARCH_LABEL}"
if [ -f "$MAIN_BINARY" ]; then
    echo "==> Using pre-built mobazha binary: ${MAIN_BINARY}"
    cp "$MAIN_BINARY" "${APP_DIR}/Contents/MacOS/mobazha"
else
    echo "==> Building mobazha CLI binary..."
    BUILD_TAGS="${BUILD_TAGS:-goolm purego_sqlite embed_frontend}"
    CGO_ENABLED=0 GOARCH="${GOARCH}" go build \
        -tags "${BUILD_TAGS}" \
        -ldflags="-s -w -X github.com/mobazha/mobazha3.0/internal/version.buildVersion=${VERSION}" \
        -o "${APP_DIR}/Contents/MacOS/mobazha" \
        "${PROJECT_ROOT}"
fi

chmod +x "${APP_DIR}/Contents/MacOS/mobazha"
chmod +x "${APP_DIR}/Contents/MacOS/mobazha-launcher"

# --- Icon ---
cp "${PROJECT_ROOT}/cmd/mobazha-launcher/assets/Mobazha.icns" "${APP_DIR}/Contents/Resources/AppIcon.icns"

echo "==> Mobazha.app created at ${APP_DIR}"

# --- Create DMG (if create-dmg is available) ---
if command -v create-dmg &>/dev/null; then
    DMG_NAME="Mobazha-${VERSION}-macOS-${ARCH_LABEL}.dmg"
    echo "==> Creating DMG: ${DMG_NAME}"
    create-dmg \
        --volname "Mobazha" \
        --window-pos 200 120 \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "Mobazha.app" 175 190 \
        --app-drop-link 425 190 \
        --hide-extension "Mobazha.app" \
        "${DIST_DIR}/${DMG_NAME}" \
        "${APP_DIR}" \
        || true
    echo "==> DMG created at ${DIST_DIR}/${DMG_NAME}"
else
    echo "==> Skipping DMG creation (install create-dmg for .dmg output)"
    echo "   brew install create-dmg"
fi

echo "==> Done!"

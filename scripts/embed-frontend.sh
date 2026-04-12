#!/bin/bash
set -euo pipefail

# Copies the Vite SPA build output into the go:embed directory and
# optionally pre-compresses assets with Brotli for zero-overhead serving.
#
# Usage:
#   ./scripts/embed-frontend.sh [SPA_DIST_DIR]
#
# Defaults:
#   SPA_DIST_DIR = ../../mobazha-unified/apps/web/dist  (relative to repo root)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
EMBED_DIR="$REPO_ROOT/internal/embedded/frontend/dist"

SPA_DIST="${1:-$(cd "$REPO_ROOT/../.." && pwd)/dev/openbazaar/mobazha-unified/apps/web/dist}"

if [ ! -d "$SPA_DIST" ]; then
    echo "ERROR: SPA dist directory not found: $SPA_DIST"
    echo "Run 'pnpm --filter @mobazha/web build' first."
    exit 1
fi

if [ ! -f "$SPA_DIST/index.html" ]; then
    echo "ERROR: index.html not found in $SPA_DIST"
    exit 1
fi

echo "==> Cleaning embed directory..."
rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR"

echo "==> Copying SPA dist from $SPA_DIST ..."
cp -r "$SPA_DIST"/* "$EMBED_DIR/"

BROTLI_CMD=""
if command -v brotli &>/dev/null; then
    BROTLI_CMD="brotli"
elif command -v br &>/dev/null; then
    BROTLI_CMD="br"
fi

if [ -n "$BROTLI_CMD" ]; then
    echo "==> Pre-compressing with Brotli ($BROTLI_CMD)..."
    find "$EMBED_DIR" -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.json' -o -name '*.svg' \) | while read -r f; do
        $BROTLI_CMD --best --keep "$f"
    done
    BR_COUNT=$(find "$EMBED_DIR" -name '*.br' | wc -l | tr -d ' ')
    echo "    Compressed $BR_COUNT files"
else
    echo "==> Brotli not found, skipping pre-compression."
    echo "    Install with: brew install brotli  (macOS) or apt install brotli (Linux)"
fi

FILE_COUNT=$(find "$EMBED_DIR" -type f | wc -l | tr -d ' ')
DIR_SIZE=$(du -sh "$EMBED_DIR" | cut -f1)
echo "==> Done. Embedded $FILE_COUNT files ($DIR_SIZE)"
echo ""
echo "Now rebuild the binary:"
echo "  CGO_ENABLED=0 go build -tags 'goolm purego_sqlite' -o mobazha ."

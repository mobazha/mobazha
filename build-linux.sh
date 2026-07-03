#!/bin/bash

# Build script for mobazha on Linux x64

set -e

echo "Building mobazha_hosting for Linux x64..."

# Set build variables
OUTPUT="mobazha"
GOOS="linux"
GOARCH="amd64"

# Build with CGO and static linking
# This version supports go-sqlite3 (requires CGO)
echo "Building with CGO enabled (supports SQLite)..."
GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1 \
    CC=/opt/homebrew/bin/x86_64-linux-musl-gcc \
    go build \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -trimpath \
    -o $OUTPUT

# Check if build was successful
if [ -f "$OUTPUT" ]; then
    echo "✓ Build successful!"
    echo "---"
    ls -lh $OUTPUT
    file $OUTPUT
    echo "---"
    echo "Output: $OUTPUT"
    echo ""
    echo "Note: This is a statically linked binary with SQLite support"
else
    echo "✗ Build failed!"
    exit 1
fi


#!/usr/bin/env bash
set -euo pipefail

# build-standalone.sh — Build the standalone store Docker image
#
# This script:
#   1. Builds the Vite SPA from the mobazha-unified monorepo
#   2. Copies the dist/ output into this repo's build context
#   3. Builds the multi-stage Docker image with the real frontend
#
# Usage:
#   ./deploy/standalone/build-standalone.sh [OPTIONS]
#
# Options:
#   -t TAG         Docker image tag (default: mobazha/standalone:dev)
#   -f FRONTEND    Path to mobazha-unified repo (default: auto-detect)
#   -s             Skip frontend build (use existing dist in FRONTEND_DIR)
#   -p             Push image after build
#   --platform P   Docker buildx platform (e.g. linux/amd64,linux/arm64)
#
# Requirements:
#   - Node.js 20+, pnpm 9+
#   - Docker with BuildKit support

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

IMAGE_TAG="mobazha/standalone:dev"
FRONTEND_REPO=""
SKIP_FRONTEND=false
PUSH=false
PLATFORM=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -t) IMAGE_TAG="$2"; shift 2 ;;
        -f) FRONTEND_REPO="$2"; shift 2 ;;
        -s) SKIP_FRONTEND=true; shift ;;
        -p) PUSH=true; shift ;;
        --platform) PLATFORM="$2"; shift 2 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# Auto-detect mobazha-unified repo location
if [[ -z "$FRONTEND_REPO" ]]; then
    for candidate in \
        "$REPO_ROOT/../mobazha-unified" \
        "$HOME/dev/openbazaar/mobazha-unified" \
        "$HOME/dev/mobazha/mobazha-unified"; do
        if [[ -f "$candidate/apps/web/vite.config.ts" ]]; then
            FRONTEND_REPO="$(cd "$candidate" && pwd)"
            break
        fi
    done
fi

if [[ -z "$FRONTEND_REPO" ]]; then
    echo "ERROR: Cannot find mobazha-unified repo. Use -f to specify path." >&2
    exit 1
fi

echo "==> Config"
echo "    Backend repo:  $REPO_ROOT"
echo "    Frontend repo: $FRONTEND_REPO"
echo "    Image tag:     $IMAGE_TAG"
echo ""

DIST_DIR="$REPO_ROOT/deploy/standalone/.frontend-dist"

if [[ "$SKIP_FRONTEND" == "false" ]]; then
    echo "==> Building frontend SPA (standalone mode)..."

    ENV_FILE="$SCRIPT_DIR/env.standalone.production"
    if [[ ! -f "$ENV_FILE" ]]; then
        echo "ERROR: Missing $ENV_FILE" >&2
        exit 1
    fi

    # Copy env file to apps/web/.env.production.local for Vite to pick up
    cp "$ENV_FILE" "$FRONTEND_REPO/apps/web/.env.production.local"

    (
        cd "$FRONTEND_REPO"
        echo "    Installing dependencies..."
        pnpm install --frozen-lockfile 2>&1 | tail -1
        echo "    Running vite build..."
        pnpm --filter @mobazha/web build 2>&1 | tail -5
    )

    # Clean up the injected env file
    rm -f "$FRONTEND_REPO/apps/web/.env.production.local"

    # Copy dist to build context
    rm -rf "$DIST_DIR"
    cp -r "$FRONTEND_REPO/apps/web/dist" "$DIST_DIR"

    FILE_COUNT=$(find "$DIST_DIR" -type f | wc -l)
    echo "    Frontend built: $FILE_COUNT files in .frontend-dist/"
else
    echo "==> Skipping frontend build (-s flag)"
    if [[ ! -d "$DIST_DIR" ]]; then
        echo "ERROR: No pre-built frontend at $DIST_DIR. Run without -s first." >&2
        exit 1
    fi
fi

echo ""
echo "==> Building Docker image..."

BUILD_ARGS=(
    -f "$SCRIPT_DIR/Dockerfile.standalone"
    --build-arg "FRONTEND_DIR=deploy/standalone/.frontend-dist"
    -t "$IMAGE_TAG"
)

if [[ -n "$PLATFORM" ]]; then
    BUILD_ARGS+=(--platform "$PLATFORM")
fi

if [[ "$PUSH" == "true" ]]; then
    BUILD_ARGS+=(--push)
fi

DOCKER_BUILDKIT=1 docker build "${BUILD_ARGS[@]}" "$REPO_ROOT"

echo ""
echo "==> Done! Image: $IMAGE_TAG"
if [[ "$PUSH" == "false" ]]; then
    echo "    Use -p to push, or run:"
    echo "    docker push $IMAGE_TAG"
fi

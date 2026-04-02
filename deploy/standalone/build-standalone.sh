#!/usr/bin/env bash
set -euo pipefail

# build-standalone.sh — Build the standalone store Docker image
#
# This script:
#   1. Builds the Vite SPA from the mobazha-unified monorepo
#   2. Copies the dist/ output into this repo's build context
#   3. Detects mobazha-core path for multi-repo Docker build
#   4. Builds the multi-stage Docker image with the real frontend
#
# Usage:
#   ./deploy/standalone/build-standalone.sh [OPTIONS]
#
# Options:
#   -t TAG         Docker image tag (default: mobazha/standalone:dev)
#   -f FRONTEND    Path to mobazha-unified repo (default: auto-detect)
#   -c CORE        Path to mobazha-core repo (default: auto-detect from go.work)
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
CORE_REPO=""
SKIP_FRONTEND=false
PUSH=false
PLATFORM=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -t) IMAGE_TAG="$2"; shift 2 ;;
        -f) FRONTEND_REPO="$2"; shift 2 ;;
        -c) CORE_REPO="$2"; shift 2 ;;
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

# Auto-detect mobazha-core repo location
if [[ -z "$CORE_REPO" ]]; then
    # Try go.work first (most reliable)
    if [[ -f "$REPO_ROOT/go.work" ]]; then
        CORE_PATH=$(grep 'replace github.com/mobazha/mobazha-core =>' "$REPO_ROOT/go.work" | sed 's/.*=> //' | xargs)
        if [[ -n "$CORE_PATH" && -f "$CORE_PATH/go.mod" ]]; then
            CORE_REPO="$(cd "$CORE_PATH" && pwd)"
        fi
    fi
    # Fallback: common locations
    if [[ -z "$CORE_REPO" ]]; then
        for candidate in \
            "$REPO_ROOT/../mobazha-core" \
            "$HOME/dev/mobazha/core" \
            "$HOME/go/src/github.com/mobazha/mobazha-core"; do
            if [[ -f "$candidate/go.mod" ]]; then
                CORE_REPO="$(cd "$candidate" && pwd)"
                break
            fi
        done
    fi
fi

if [[ -z "$CORE_REPO" ]]; then
    echo "ERROR: Cannot find mobazha-core repo. Use -c to specify path." >&2
    exit 1
fi

echo "==> Config"
echo "    Backend repo:  $REPO_ROOT"
echo "    Core repo:     $CORE_REPO"
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
    --build-context "core=$CORE_REPO"
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
echo "    Image size: $(docker image inspect "$IMAGE_TAG" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1f MB", $1/1024/1024}')"
if [[ "$PUSH" == "false" ]]; then
    echo "    Use -p to push, or run:"
    echo "    docker push $IMAGE_TAG"
fi

#!/usr/bin/env bash
set -euo pipefail

# build-standalone-v2.sh — Standard approach: build frontend INSIDE Docker
#
# No external frontend build, no fixup scripts, no prune scripts.
# Everything happens in Docker multi-stage build.
#
# Usage:
#   ./deploy/standalone/build-standalone-v2.sh [OPTIONS]
#
# Options:
#   -t TAG         Docker image tag (default: ghcr.io/mobazha/standalone:v2)
#   -f FRONTEND    Path to mobazha-unified repo (default: auto-detect)
#   -c CORE        Path to mobazha-core repo (default: auto-detect from go.work)
#   -n NODE_IMAGE  Pre-built node image (skip Go compilation)
#   -p             Push image after build
#   --platform P   Docker buildx platform (e.g. linux/amd64,linux/arm64)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

IMAGE_TAG="ghcr.io/mobazha/standalone:v2"
FRONTEND_REPO=""
CORE_REPO=""
NODE_IMAGE=""
PUSH=false
PLATFORM=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -t) IMAGE_TAG="$2"; shift 2 ;;
        -f) FRONTEND_REPO="$2"; shift 2 ;;
        -c) CORE_REPO="$2"; shift 2 ;;
        -n) NODE_IMAGE="$2"; shift 2 ;;
        -p) PUSH=true; shift ;;
        --platform) PLATFORM="$2"; shift 2 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# Auto-detect mobazha-unified repo
if [[ -z "$FRONTEND_REPO" ]]; then
    for candidate in \
        "$REPO_ROOT/../mobazha-unified" \
        "$HOME/dev/openbazaar/mobazha-unified" \
        "$HOME/dev/mobazha/mobazha-unified"; do
        if [[ -f "$candidate/apps/web/package.json" ]]; then
            FRONTEND_REPO="$(cd "$candidate" && pwd)"
            break
        fi
    done
fi
[[ -n "$FRONTEND_REPO" ]] || { echo "ERROR: Cannot find mobazha-unified repo. Use -f to specify." >&2; exit 1; }


echo "==> Config (v2 — standard in-Docker build)"
echo "    Backend repo:  $REPO_ROOT"
if [[ -n "$NODE_IMAGE" ]]; then
    echo "    Node image:    $NODE_IMAGE (pre-built)"
fi
echo "    Frontend repo: $FRONTEND_REPO"
echo "    Image tag:     $IMAGE_TAG"
echo ""

echo "==> Building Docker image (frontend builds inside Docker)..."

BUILD_ARGS=(
    -f "$SCRIPT_DIR/Dockerfile.standalone-v2"
    --build-context "frontend=$FRONTEND_REPO"
    -t "$IMAGE_TAG"
)

if [[ -n "$NODE_IMAGE" ]]; then
    BUILD_ARGS+=(--build-arg "NODE_IMAGE=$NODE_IMAGE")
fi

if [[ -n "$PLATFORM" ]]; then
    BUILD_ARGS+=(--platform "$PLATFORM")
fi

if [[ "$PUSH" == "true" ]]; then
    BUILD_ARGS+=(--push)
fi

DOCKER_BUILDKIT=1 docker build "${BUILD_ARGS[@]}" "$REPO_ROOT"

echo ""
echo "==> Done! Image: $IMAGE_TAG"
IMAGE_SIZE=$(docker image inspect "$IMAGE_TAG" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1f MB", $1/1024/1024}')
echo "    Image size: $IMAGE_SIZE"
if [[ "$PUSH" == "false" ]]; then
    echo "    Use -p to push, or run: docker push $IMAGE_TAG"
fi

#!/usr/bin/env bash
set -euo pipefail

# release-standalone.sh — Build, tag, and push standalone store image to GHCR
#
# Usage:
#   ./deploy/standalone/release-standalone.sh v1.0.0              # release version
#   ./deploy/standalone/release-standalone.sh v1.0.0 --stable     # also tag as :stable
#   ./deploy/standalone/release-standalone.sh --edge               # build :edge (nightly)
#   ./deploy/standalone/release-standalone.sh --dry-run v1.0.0    # build only, no push
#
# Prerequisites:
#   - Docker with BuildKit support
#   - Authenticated to GHCR: echo $GHCR_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
#   - Node.js 20+, pnpm 9+ (for frontend build)
#
# Tag strategy:
#   Release: ghcr.io/mobazha/standalone:v1.0.0 + :stable (if --stable)
#   Edge:    ghcr.io/mobazha/standalone:edge
#   All:     Also tagged with short SHA for traceability

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

REGISTRY="ghcr.io/mobazha"
IMAGE_NAME="standalone"
IMAGE_BASE="${REGISTRY}/${IMAGE_NAME}"

VERSION=""
TAG_STABLE=false
EDGE=false
DRY_RUN=false
PLATFORM="${PLATFORM:-linux/amd64}"
EXTRA_BUILD_ARGS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        --stable)   TAG_STABLE=true; shift ;;
        --edge)     EDGE=true; shift ;;
        --dry-run)  DRY_RUN=true; shift ;;
        --platform) PLATFORM="$2"; shift 2 ;;
        -s)         EXTRA_BUILD_ARGS+=("-s"); shift ;;
        -f)         EXTRA_BUILD_ARGS+=("-f" "$2"); shift 2 ;;
        -c)         EXTRA_BUILD_ARGS+=("-c" "$2"); shift 2 ;;
        v*)         VERSION="$1"; shift ;;
        *)          echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [[ "$EDGE" == "false" && -z "$VERSION" ]]; then
    echo "Usage: release-standalone.sh <version> [--stable] [--dry-run]" >&2
    echo "       release-standalone.sh --edge [--dry-run]" >&2
    echo "" >&2
    echo "Examples:" >&2
    echo "  release-standalone.sh v1.0.0              # release v1.0.0" >&2
    echo "  release-standalone.sh v1.0.0 --stable     # release v1.0.0 + tag :stable" >&2
    echo "  release-standalone.sh --edge               # build edge (nightly)" >&2
    exit 1
fi

GIT_SHA="$(cd "$REPO_ROOT" && git rev-parse --short HEAD)"
GIT_SHA_LONG="$(cd "$REPO_ROOT" && git rev-parse HEAD)"

TAGS=()
if [[ "$EDGE" == "true" ]]; then
    TAGS+=("${IMAGE_BASE}:edge")
    TAGS+=("${IMAGE_BASE}:sha-${GIT_SHA}")
    echo "==> Release: edge (sha: ${GIT_SHA})"
else
    if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.+)?$ ]]; then
        echo "ERROR: Version must be in format vX.Y.Z[-prerelease] (got: $VERSION)" >&2
        exit 1
    fi
    TAGS+=("${IMAGE_BASE}:${VERSION}")
    TAGS+=("${IMAGE_BASE}:sha-${GIT_SHA}")
    if [[ "$TAG_STABLE" == "true" ]]; then
        TAGS+=("${IMAGE_BASE}:stable")
    fi
    echo "==> Release: ${VERSION} (sha: ${GIT_SHA})"
fi

echo "    Tags:"
for tag in "${TAGS[@]}"; do
    echo "      - $tag"
done
echo ""

if [[ "$DRY_RUN" == "false" ]]; then
    if ! docker info 2>/dev/null | grep -q "ghcr.io"; then
        if ! docker pull ghcr.io/library/alpine:3.20 >/dev/null 2>&1; then
            echo "WARNING: Docker may not be authenticated to GHCR." >&2
            echo "  Run: echo \$GHCR_TOKEN | docker login ghcr.io -u <username> --password-stdin" >&2
        fi
    fi
fi

PRIMARY_TAG="${TAGS[0]}"

echo "==> Building image..."
"$SCRIPT_DIR/build-standalone.sh" \
    -t "$PRIMARY_TAG" \
    --platform "$PLATFORM" \
    "${EXTRA_BUILD_ARGS[@]}"

for tag in "${TAGS[@]:1}"; do
    echo "    Tagging: $tag"
    docker tag "$PRIMARY_TAG" "$tag"
done

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    echo "==> Dry run — skipping push."
    echo "    Built: $PRIMARY_TAG"
    echo "    To push manually: docker push $PRIMARY_TAG"
    exit 0
fi

echo ""
echo "==> Pushing to GHCR..."
for tag in "${TAGS[@]}"; do
    echo "    Pushing $tag ..."
    docker push "$tag"
done

echo ""
echo "==> Release complete!"
echo "    Image:   $PRIMARY_TAG"
echo "    SHA:     ${GIT_SHA_LONG}"
echo ""
echo "    Users pull via:"
if [[ "$TAG_STABLE" == "true" ]]; then
    echo "      docker pull ${IMAGE_BASE}:stable"
fi
if [[ -n "$VERSION" ]]; then
    echo "      docker pull ${IMAGE_BASE}:${VERSION}"
fi
echo ""
echo "    install.sh users get it automatically (TAG=stable in .env)."

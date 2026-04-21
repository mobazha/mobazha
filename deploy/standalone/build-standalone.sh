#!/usr/bin/env bash
set -euo pipefail

# Load nvm if available (pnpm is managed via nvm on dev machines)
if [[ -s "$HOME/.nvm/nvm.sh" ]]; then
    source "$HOME/.nvm/nvm.sh"
fi

# build-standalone.sh — Build the standalone store Docker image
#
# This script:
#   1. Builds the Next.js app (standalone output) from the mobazha-unified monorepo
#   2. Copies the standalone output into this repo's build context
#   3. Detects mobazha-core path for multi-repo Docker build
#   4. Builds the multi-stage Docker image with the real frontend
#
# Usage:
#   ./deploy/standalone/build-standalone.sh [OPTIONS]
#
# Options:
#   -t TAG         Docker image tag (default: ghcr.io/mobazha/standalone:dev)
#   -f FRONTEND    Path to mobazha-unified repo (default: auto-detect)
#   -c CORE        Path to mobazha-core repo (default: auto-detect from go.work)
#   -n NODE_IMAGE  Pre-built node image (skip Go compilation, e.g. ghcr.io/mobazha/standalone-node:v1.0)
#   -s             Skip frontend build (use existing dist in FRONTEND_DIR)
#   -p             Push image after build
#   --platform P   Docker buildx platform (e.g. linux/amd64,linux/arm64)
#
# Requirements:
#   - Node.js 20+, pnpm 9+
#   - Docker with BuildKit support

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

IMAGE_TAG="ghcr.io/mobazha/standalone:dev"
FRONTEND_REPO=""
CORE_REPO=""
NODE_IMAGE=""
SKIP_FRONTEND=false
PUSH=false
PLATFORM=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -t) IMAGE_TAG="$2"; shift 2 ;;
        -f) FRONTEND_REPO="$2"; shift 2 ;;
        -c) CORE_REPO="$2"; shift 2 ;;
        -n) NODE_IMAGE="$2"; shift 2 ;;
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

if [[ -z "$NODE_IMAGE" && -z "$CORE_REPO" ]]; then
    echo "ERROR: Cannot find mobazha-core repo. Use -c to specify path, or -n to use a pre-built node image." >&2
    exit 1
fi

echo "==> Config"
echo "    Backend repo:  $REPO_ROOT"
if [[ -n "$NODE_IMAGE" ]]; then
    echo "    Node image:    $NODE_IMAGE (pre-built, skipping Go compilation)"
else
    echo "    Core repo:     $CORE_REPO"
fi
echo "    Frontend repo: $FRONTEND_REPO"
echo "    Image tag:     $IMAGE_TAG"
echo ""

DIST_DIR="$REPO_ROOT/deploy/standalone/.frontend-dist"

if [[ "$SKIP_FRONTEND" == "false" ]]; then
    echo "==> Building Next.js frontend (standalone mode)..."

    ENV_FILE="$SCRIPT_DIR/env.standalone.production"
    if [[ ! -f "$ENV_FILE" ]]; then
        echo "ERROR: Missing $ENV_FILE" >&2
        exit 1
    fi

    cp "$ENV_FILE" "$FRONTEND_REPO/apps/web/.env.production.local"

    (
        cd "$FRONTEND_REPO"
        echo "    Installing dependencies..."
        pnpm install --frozen-lockfile 2>&1 | tail -1
        echo "    Running next build..."
        pnpm --filter @mobazha/web build:next 2>&1 | tail -10
    )

    rm -f "$FRONTEND_REPO/apps/web/.env.production.local"

    WEB_DIR="$FRONTEND_REPO/apps/web"

    rm -rf "$DIST_DIR"
    mkdir -p "$DIST_DIR/standalone"

    # Next.js standalone output: self-contained server + minimal node_modules.
    # pnpm uses symlinks extensively; some may be broken in standalone output.
    # Use tar with -L (dereference) to resolve valid symlinks and skip broken ones.
    (cd "$WEB_DIR/.next/standalone" && tar -cLf - . 2>/dev/null) | (cd "$DIST_DIR/standalone" && tar xf -)

    # Fix incomplete pnpm packages in standalone output.
    # Next.js file tracing sometimes copies only package.json without
    # actual code files. We selectively copy ONLY missing files from the
    # monorepo pnpm store — never overwrite existing content.
    MONO_PNPM="$FRONTEND_REPO/node_modules/.pnpm"
    STANDALONE_PNPM="$DIST_DIR/standalone/node_modules/.pnpm"
    if [[ -d "$STANDALONE_PNPM" && -d "$MONO_PNPM" ]]; then
        echo "    Fixing incomplete pnpm packages..."
        FIXED=0

        # Phase 1: fix versioned packages (e.g. .pnpm/tough-cookie@5.1.2/node_modules/tough-cookie)
        while IFS= read -r pkg_json; do
            pkg_dir="$(dirname "$pkg_json")"
            rel="${pkg_dir#$STANDALONE_PNPM/}"
            mono_dir="$MONO_PNPM/$rel"
            [[ -d "$mono_dir" ]] || continue

            mono_count=$(find "$mono_dir" -type f | wc -l | tr -d ' ')
            standalone_count=$(find "$pkg_dir" -type f | wc -l | tr -d ' ')
            if (( mono_count > standalone_count )); then
                # Use rsync-like approach: copy only missing files (--ignore-existing)
                # tar overlay: extract only files that don't already exist
                (cd "$mono_dir" && find . -type f) | while read -r f; do
                    if [[ ! -f "$pkg_dir/$f" ]]; then
                        src="$mono_dir/$f"
                        [[ -L "$src" ]] && src="$(readlink -f "$src" 2>/dev/null)" && [[ ! -f "$src" ]] && continue
                        mkdir -p "$(dirname "$pkg_dir/$f")"
                        cp -L "$src" "$pkg_dir/$f" 2>/dev/null || true
                    fi
                done
                FIXED=$((FIXED + 1))
            fi
        done < <(find "$STANDALONE_PNPM" -mindepth 3 -maxdepth 5 -name "package.json" -not -path "*/node_modules/.pnpm/node_modules/*")

        # Phase 2: re-sync hoisted packages (.pnpm/node_modules/<name>).
        # tar -cL pre-resolved these from the (then-incomplete) versioned dirs.
        # Multiple versions may exist; pick the one with the most files.
        HOISTED_DIR="$STANDALONE_PNPM/node_modules"
        if [[ -d "$HOISTED_DIR" ]]; then
            for hoisted_pkg in "$HOISTED_DIR"/*/; do
                [[ -d "$hoisted_pkg" ]] || continue
                pkg_name=$(basename "$hoisted_pkg")
                hoisted_count=$(find "$hoisted_pkg" -type f | wc -l | tr -d ' ')
                # Find the versioned directory with the most files
                best_versioned=""
                best_count=0
                while IFS= read -r candidate; do
                    c=$(find "$candidate" -type f | wc -l | tr -d ' ')
                    if (( c > best_count )); then
                        best_count=$c
                        best_versioned="$candidate"
                    fi
                done < <(find "$STANDALONE_PNPM" -maxdepth 3 -path "*/${pkg_name}@*/node_modules/${pkg_name}" -type d 2>/dev/null)
                if [[ -n "$best_versioned" ]] && (( best_count > hoisted_count )); then
                    (cd "$best_versioned" && find . -type f) | while read -r f; do
                        if [[ ! -f "$hoisted_pkg/$f" ]]; then
                            mkdir -p "$(dirname "$hoisted_pkg/$f")"
                            cp "$best_versioned/$f" "$hoisted_pkg/$f" 2>/dev/null || true
                        fi
                    done
                    FIXED=$((FIXED + 1))
                fi
            done
        fi
        echo "    Fixed $FIXED incomplete packages"
    fi

    # Prune files not needed at runtime to reduce image size.
    echo "    Pruning dev artifacts..."
    BEFORE=$(du -sm "$DIST_DIR/standalone" | awk '{print $1}')
    # Platform-specific sharp native binaries (nested inside sharp@*/node_modules/@img/)
    find "$DIST_DIR/standalone/node_modules/.pnpm" -path "*/sharp@*/node_modules/@img/*darwin*" -type d -exec rm -rf {} + 2>/dev/null
    find "$DIST_DIR/standalone/node_modules/.pnpm" -path "*/sharp@*/node_modules/@img/*win32*" -type d -exec rm -rf {} + 2>/dev/null
    # Hoisted sharp platform copies
    rm -rf "$DIST_DIR"/standalone/node_modules/.pnpm/node_modules/@img/sharp-darwin-* \
           "$DIST_DIR"/standalone/node_modules/.pnpm/node_modules/@img/sharp-libvips-darwin-* \
           "$DIST_DIR"/standalone/node_modules/.pnpm/node_modules/@img/sharp-win32-* \
           "$DIST_DIR"/standalone/node_modules/.pnpm/node_modules/@img/sharp-libvips-win32-*
    # Dev-only packages
    rm -rf "$DIST_DIR"/standalone/node_modules/.pnpm/typescript@*/
    # File tracing metadata
    find "$DIST_DIR/standalone" -name "*.nft.json" -delete 2>/dev/null
    # Source maps and TypeScript declarations
    find "$DIST_DIR/standalone/node_modules" -name "*.map" -delete 2>/dev/null
    find "$DIST_DIR/standalone/node_modules" -name "*.d.ts" -delete 2>/dev/null
    find "$DIST_DIR/standalone/node_modules" -name "*.d.mts" -delete 2>/dev/null
    # Markdown/docs in node_modules
    find "$DIST_DIR/standalone/node_modules" \( -name "README.md" -o -name "CHANGELOG.md" -o -name "LICENSE" -o -name "HISTORY.md" \) -delete 2>/dev/null
    AFTER=$(du -sm "$DIST_DIR/standalone" | awk '{print $1}')
    echo "    Pruned $((BEFORE - AFTER))MB (${BEFORE}MB → ${AFTER}MB)"

    # Static assets: must be alongside standalone server under .next/static/
    if [[ -d "$WEB_DIR/.next/static" ]]; then
        mkdir -p "$DIST_DIR/static"
        cp -r "$WEB_DIR/.next/static/." "$DIST_DIR/static/"
    fi

    # Public assets: icons, manifest, etc.
    if [[ -d "$WEB_DIR/public" ]]; then
        mkdir -p "$DIST_DIR/public"
        cp -r "$WEB_DIR/public/." "$DIST_DIR/public/"
    fi

    FILE_COUNT=$(find "$DIST_DIR" -type f | wc -l)
    echo "    Next.js standalone built: $FILE_COUNT files in .frontend-dist/"
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

if [[ -n "$NODE_IMAGE" ]]; then
    BUILD_ARGS+=(--build-arg "NODE_IMAGE=$NODE_IMAGE")
else
    BUILD_ARGS+=(--build-context "core=$CORE_REPO")
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
echo "    Image size: $(docker image inspect "$IMAGE_TAG" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1f MB", $1/1024/1024}')"
if [[ "$PUSH" == "false" ]]; then
    echo "    Use -p to push, or run:"
    echo "    docker push $IMAGE_TAG"
fi

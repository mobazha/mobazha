#!/usr/bin/env bash
#
# release-standalone-smoke.sh — Post-push smoke test for GHCR standalone images.
#
# Pulls the image, starts a throwaway container, and verifies node health,
# setup API, and Caddy-fronted HTTP.
#
# Usage:
#   ./scripts/release-standalone-smoke.sh ghcr.io/mobazha/standalone:v0.3.0-beta.29
#
# Environment:
#   STARTUP_TIMEOUT  — seconds to wait (default 300)

set -euo pipefail

IMAGE="${1:?usage: release-standalone-smoke.sh <image>}"
CONTAINER="mobazha-standalone-smoke-$$"
NODE_PORT="${NODE_PORT:-15102}"
HTTP_PORT="${HTTP_PORT:-18080}"
STARTUP_TIMEOUT="${STARTUP_TIMEOUT:-300}"

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "=== Standalone Release Smoke ==="
echo "Image: $IMAGE"
echo ""

docker pull "$IMAGE"

docker run -d --name "$CONTAINER" \
    -e TESTNET=1 \
    -e ADMIN_PASSWORD=smoke-test-pass \
    -p "${NODE_PORT}:5102" \
    -p "${HTTP_PORT}:80" \
    "$IMAGE" >/dev/null

deadline=$((SECONDS + STARTUP_TIMEOUT))
until curl -sf "http://127.0.0.1:${NODE_PORT}/healthz" >/dev/null; do
    if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
        echo "FAIL: container exited before /healthz became ready"
        docker logs "$CONTAINER" 2>&1 | tail -40 || true
        exit 1
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
        echo "FAIL: /healthz not ready within ${STARTUP_TIMEOUT}s"
        docker logs "$CONTAINER" 2>&1 | tail -40 || true
        exit 1
    fi
    sleep 3
done
echo "PASS: GET :${NODE_PORT}/healthz"

setup_code=$(curl -s -o /tmp/standalone-setup.json -w "%{http_code}" \
    "http://127.0.0.1:${NODE_PORT}/v1/system/setup")
if [ "$setup_code" != "200" ]; then
    echo "FAIL: GET /v1/system/setup returned HTTP $setup_code"
    cat /tmp/standalone-setup.json 2>/dev/null || true
    exit 1
fi
echo "PASS: GET /v1/system/setup"

# Caddy may take a few extra seconds after the node is up.
http_deadline=$((SECONDS + 60))
until curl -sf "http://127.0.0.1:${HTTP_PORT}/healthz" >/dev/null 2>&1 || \
      curl -sf "http://127.0.0.1:${HTTP_PORT}/" >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$http_deadline" ]; then
        echo "FAIL: Caddy HTTP on :${HTTP_PORT} not reachable"
        docker logs "$CONTAINER" 2>&1 | tail -40 || true
        exit 1
    fi
    sleep 2
done
echo "PASS: Caddy HTTP on :${HTTP_PORT}"

echo ""
echo "=== Standalone Release Smoke: ALL PASS ==="

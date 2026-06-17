#!/usr/bin/env bash
#
# release-native-smoke.sh — Post-build smoke test for release native binaries.
#
# Verifies the linux-amd64 (or host) artifact can start, serve /healthz,
# expose setup API, and serve the embedded SPA root.
#
# Usage:
#   ./scripts/release-native-smoke.sh /path/to/mobazha-linux-amd64
#
# Environment:
#   GATEWAY_PORT     — default 15202
#   STARTUP_TIMEOUT  — seconds to wait for /healthz (default 180)

set -euo pipefail

MOBAZHA_BIN="${1:?usage: release-native-smoke.sh <binary>}"
DATA_DIR="$(mktemp -d)/mobazha-smoke"
GATEWAY_PORT="${GATEWAY_PORT:-15202}"
GATEWAY_ADDR="/ip4/127.0.0.1/tcp/${GATEWAY_PORT}"
STARTUP_TIMEOUT="${STARTUP_TIMEOUT:-180}"

cleanup() {
    if [ -n "${MOBAZHA_PID:-}" ] && kill -0 "$MOBAZHA_PID" 2>/dev/null; then
        kill "$MOBAZHA_PID" 2>/dev/null || true
        wait "$MOBAZHA_PID" 2>/dev/null || true
    fi
    rm -rf "$DATA_DIR" 2>/dev/null || true
}
trap cleanup EXIT

if [ ! -f "$MOBAZHA_BIN" ]; then
    echo "FAIL: binary not found: $MOBAZHA_BIN"
    exit 1
fi
chmod +x "$MOBAZHA_BIN"

echo "=== Native Release Smoke ==="
echo "Binary:  $MOBAZHA_BIN"
echo "Data:    $DATA_DIR"
echo "Gateway: $GATEWAY_ADDR"
echo ""

"$MOBAZHA_BIN" start \
    --datadir="$DATA_DIR" \
    --gatewayaddr="$GATEWAY_ADDR" \
    --testnet \
    --wallettestnet &
MOBAZHA_PID=$!
echo "Started mobazha PID=$MOBAZHA_PID"

deadline=$((SECONDS + STARTUP_TIMEOUT))
until curl -sf "http://127.0.0.1:${GATEWAY_PORT}/healthz" >/dev/null; do
    if ! kill -0 "$MOBAZHA_PID" 2>/dev/null; then
        echo "FAIL: process exited before /healthz became ready"
        exit 1
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
        echo "FAIL: /healthz not ready within ${STARTUP_TIMEOUT}s"
        exit 1
    fi
    sleep 2
done
echo "PASS: GET /healthz"

setup_code=$(curl -s -o /tmp/mobazha-setup.json -w "%{http_code}" \
    "http://127.0.0.1:${GATEWAY_PORT}/v1/system/setup")
if [ "$setup_code" != "200" ]; then
    echo "FAIL: GET /v1/system/setup returned HTTP $setup_code"
    cat /tmp/mobazha-setup.json 2>/dev/null || true
    exit 1
fi
echo "PASS: GET /v1/system/setup"

root_code=$(curl -s -o /tmp/mobazha-root.html -w "%{http_code}" \
    "http://127.0.0.1:${GATEWAY_PORT}/")
if [ "$root_code" != "200" ]; then
    echo "FAIL: GET / returned HTTP $root_code"
    exit 1
fi
if ! grep -Eqi '<html|mobazha' /tmp/mobazha-root.html; then
    echo "FAIL: GET / does not look like embedded frontend HTML"
    head -5 /tmp/mobazha-root.html || true
    exit 1
fi
echo "PASS: embedded frontend GET /"

echo ""
echo "=== Native Release Smoke: ALL PASS ==="

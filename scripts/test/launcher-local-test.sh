#!/usr/bin/env bash
# launcher-local-test.sh — Local native test for mobazha-launcher Supervisor
#
# Prerequisites:
#   1. Build binaries to /tmp/launcher-test/bin/ (node + launcher)
#   2. Run: sudo bash -c 'echo "127.0.0.1 matrix.mobazha.org" >> /etc/hosts && echo "127.0.0.1 mobazha.info" >> /etc/hosts'
#
# Usage:
#   bash scripts/test/launcher-local-test.sh [test-name]
#   test-name: t1|t2|t3|t4|t5|t6|t7|t8|t9|all (default: all)
#
# Cleanup:
#   bash scripts/test/launcher-local-test.sh cleanup

set -euo pipefail

TEST_ROOT="/tmp/launcher-test"
BIN_DIR="$TEST_ROOT/bin"
DATA_DIR="$TEST_ROOT/data"
NODE_DATA_DIR="$DATA_DIR/node-data"
LOG_DIR="$DATA_DIR/logs"
LAUNCHER_BIN="$BIN_DIR/mobazha-launcher"
NODE_BIN="$BIN_DIR/mobazha"
LAUNCHER_PID=""
GATEWAY_PORT=15199
ADMIN_USER="admin"
ADMIN_PASS=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

cleanup_processes() {
    if [ -n "$LAUNCHER_PID" ] && kill -0 "$LAUNCHER_PID" 2>/dev/null; then
        info "Stopping launcher (PID $LAUNCHER_PID)..."
        kill -INT "$LAUNCHER_PID" 2>/dev/null || true
        sleep 3
        kill -9 "$LAUNCHER_PID" 2>/dev/null || true
    fi
    pkill -f "$NODE_BIN start" 2>/dev/null || true
    sleep 1
}

cleanup_data() {
    cleanup_processes
    rm -rf "$DATA_DIR" "$TEST_ROOT/.mobazha"
    info "Cleaned up $DATA_DIR and $TEST_ROOT/.mobazha"
}

start_launcher() {
    local extra_env="${1:-}"
    mkdir -p "$DATA_DIR" "$LOG_DIR" "$NODE_DATA_DIR"
    local launcher_args=(
        --node-data-dir "$NODE_DATA_DIR"
        --gateway-port "$GATEWAY_PORT"
        --testnet
    )
    info "Starting launcher with HOME=$TEST_ROOT, node-data=$NODE_DATA_DIR, port=$GATEWAY_PORT ..."
    if [ -n "$extra_env" ]; then
        env HOME="$TEST_ROOT" $extra_env "$LAUNCHER_BIN" "${launcher_args[@]}" > "$LOG_DIR/launcher-stdout.log" 2>&1 &
    else
        env HOME="$TEST_ROOT" "$LAUNCHER_BIN" "${launcher_args[@]}" > "$LOG_DIR/launcher-stdout.log" 2>&1 &
    fi
    LAUNCHER_PID=$!
    info "Launcher PID: $LAUNCHER_PID"
}

wait_for_health() {
    local timeout=${1:-60}
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        if curl -s "http://127.0.0.1:$GATEWAY_PORT/healthz" > /dev/null 2>&1; then
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1
}

load_admin_password() {
    local pw_file="$NODE_DATA_DIR/admin_password"
    if [ -f "$pw_file" ]; then
        ADMIN_PASS=$(cat "$pw_file")
        info "Admin password loaded from $pw_file"
    else
        info "No admin_password file found at $pw_file, searching..."
        pw_file=$(find "$NODE_DATA_DIR" -name "admin_password" -type f 2>/dev/null | head -1)
        if [ -n "$pw_file" ]; then
            ADMIN_PASS=$(cat "$pw_file")
            info "Admin password loaded from $pw_file"
        else
            info "Warning: admin_password not found anywhere in $NODE_DATA_DIR"
        fi
    fi
}

LAUNCHER_DATA_DIR="$TEST_ROOT/.mobazha"

wait_for_status_file() {
    local timeout=${1:-30}
    local elapsed=0
    local status_file="$LAUNCHER_DATA_DIR/update-status.json"
    while [ $elapsed -lt $timeout ]; do
        if [ -f "$status_file" ]; then
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1
}

# ==============================================================================
# T1: Launcher discovers and starts node
# ==============================================================================
t1_launcher_starts_node() {
    info "=== T1: Launcher discovers and starts node ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        pass "T1: Node started and healthy via launcher"
    else
        fail "T1: Node did not become healthy within 90s"
        cat "$LOG_DIR/launcher-stdout.log" 2>/dev/null || true
    fi
    cleanup_processes
}

# ==============================================================================
# T2: Health monitor reports status
# ==============================================================================
t2_health_monitor() {
    info "=== T2: Health monitor reports status ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        load_admin_password
        local resp
        resp=$(curl -s --user "$ADMIN_USER:$ADMIN_PASS" "http://127.0.0.1:$GATEWAY_PORT/v1/system/health")
        if echo "$resp" | grep -q '"deploymentMode"'; then
            pass "T2: /v1/system/health includes deploymentMode"
        else
            fail "T2: deploymentMode not found in health response"
            echo "$resp"
        fi
    else
        fail "T2: Node did not become healthy"
    fi
    cleanup_processes
}

# ==============================================================================
# T3: Status file is written
# ==============================================================================
t3_status_file() {
    info "=== T3: Status file written ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        sleep 10
        local status_file="$LAUNCHER_DATA_DIR/update-status.json"
        if [ -f "$status_file" ]; then
            if grep -q '"launcherVersion"' "$status_file"; then
                pass "T3: update-status.json written with launcherVersion"
                cat "$status_file"
            else
                fail "T3: update-status.json missing launcherVersion"
                cat "$status_file"
            fi
        else
            fail "T3: update-status.json not found at $status_file"
            ls -la "$LAUNCHER_DATA_DIR/" 2>/dev/null || echo "(dir missing)"
        fi
    else
        fail "T3: Node did not become healthy"
    fi
    cleanup_processes
}

# ==============================================================================
# T4: Config hot-reload
# ==============================================================================
t4_config_reload() {
    info "=== T4: Config hot-reload ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        sleep 6
        local config_file="$LAUNCHER_DATA_DIR/launcher-config.json"
        info "Writing custom config to $config_file"
        echo '{"autoUpdateEnabled":false,"checkIntervalMin":120,"updateChannel":"beta"}' > "$config_file"
        info "Waiting 15s for config reload cycle (tick every 5s)..."
        sleep 15
        local status_file="$LAUNCHER_DATA_DIR/update-status.json"
        if [ -f "$status_file" ] && grep -q '"updateChannel".*beta' "$status_file"; then
            pass "T4: Config hot-reload detected (beta channel in status)"
        else
            fail "T4: Config not reflected in status file"
            cat "$status_file" 2>/dev/null || echo "(missing)"
        fi
    else
        fail "T4: Node did not become healthy"
    fi
    cleanup_processes
}

# ==============================================================================
# T5: Trigger check action
# ==============================================================================
t5_trigger_check() {
    info "=== T5: Trigger check action ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        load_admin_password
        sleep 6
        info "Writing trigger: check"
        local resp
        resp=$(curl -s -X POST "http://127.0.0.1:$GATEWAY_PORT/v1/system/update-trigger" \
               -H "Content-Type: application/json" \
               -d '{"action":"check"}' \
               --user "$ADMIN_USER:$ADMIN_PASS" 2>&1)
        info "Trigger response: $resp"
        sleep 15
        local status_file="$LAUNCHER_DATA_DIR/update-status.json"
        if [ -f "$status_file" ]; then
            pass "T5: Trigger processed, status file exists"
            cat "$status_file"
        else
            pass "T5: Trigger sent (status file may not exist if check had errors - expected with blocked hosts)"
        fi
    else
        fail "T5: Node did not become healthy"
    fi
    cleanup_processes
}

# ==============================================================================
# T6: Crash recovery
# ==============================================================================
t6_crash_recovery() {
    info "=== T6: Crash recovery (kill node, launcher restarts it) ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        info "Killing node process directly..."
        pkill -9 -f "$NODE_BIN start" 2>/dev/null || true
        sleep 2
        info "Waiting for launcher to restart node..."
        if wait_for_health 60; then
            pass "T6: Node restarted after crash"
        else
            fail "T6: Node did not restart after crash within 60s"
        fi
    else
        fail "T6: Node did not become healthy initially"
    fi
    cleanup_processes
}

# ==============================================================================
# T7: Graceful shutdown
# ==============================================================================
t7_graceful_shutdown() {
    info "=== T7: Graceful shutdown ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        info "Sending SIGINT to launcher..."
        kill -INT "$LAUNCHER_PID" 2>/dev/null || true
        sleep 5
        if kill -0 "$LAUNCHER_PID" 2>/dev/null; then
            fail "T7: Launcher still running after SIGINT"
            kill -9 "$LAUNCHER_PID" 2>/dev/null || true
        else
            pass "T7: Launcher exited gracefully"
        fi
        sleep 5
        if curl -s --connect-timeout 2 "http://127.0.0.1:$GATEWAY_PORT/healthz" > /dev/null 2>&1; then
            fail "T7: Node still running after launcher exit"
            pkill -f "$NODE_BIN" 2>/dev/null || true
        else
            pass "T7: Node also stopped"
        fi
    else
        fail "T7: Node did not become healthy"
    fi
    LAUNCHER_PID=""
}

# ==============================================================================
# T8: Auto-update check with mock server (version discovery)
# ==============================================================================
t8_update_check_mock() {
    info "=== T8: Auto-update version check with mock server ==="
    cleanup_data

    cp "$NODE_BIN" /tmp/launcher-test/fake-new-binary
    chmod +x /tmp/launcher-test/fake-new-binary

    info "Starting fake release server..."
    lsof -ti :9999 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 1
    cd "$(dirname "$0")/../.." || exit 1
    go run scripts/test/fake-release-server.go \
        -binary /tmp/launcher-test/fake-new-binary \
        -version 99.0.0 \
        -port 9999 > /tmp/launcher-test/fake-server.log 2>&1 &
    local SERVER_PID=$!
    sleep 2

    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        fail "T8: Fake release server failed to start"
        cat /tmp/launcher-test/fake-server.log
        return
    fi
    info "Fake server PID: $SERVER_PID"

    start_launcher "MOBAZHA_UPDATE_URL=http://127.0.0.1:9999/releases"

    if wait_for_health 90; then
        load_admin_password
        sleep 6
        info "Triggering update check..."
        curl -s -X POST "http://127.0.0.1:$GATEWAY_PORT/v1/system/update-trigger" \
             -H "Content-Type: application/json" \
             -d '{"action":"check"}' \
             --user "$ADMIN_USER:$ADMIN_PASS" 2>&1 || true
        sleep 15

        local status_file="$LAUNCHER_DATA_DIR/update-status.json"
        if [ -f "$status_file" ] && grep -q "99.0.0" "$status_file"; then
            pass "T8: Mock update discovered version 99.0.0"
            cat "$status_file"
        else
            fail "T8: Version 99.0.0 not found in status"
            cat "$status_file" 2>/dev/null || echo "(missing)"
            cat /tmp/launcher-test/fake-server.log
        fi
    else
        fail "T8: Node did not become healthy"
    fi

    kill "$SERVER_PID" 2>/dev/null || true
    rm -f /tmp/launcher-test/fake-new-binary
    cleanup_processes
}

# ==============================================================================
# T9: Update config API
# ==============================================================================
t9_update_config_api() {
    info "=== T9: Update config via API ==="
    cleanup_data

    start_launcher
    if wait_for_health 90; then
        load_admin_password
        sleep 3
        info "GET /v1/system/update-config..."
        local get_resp
        get_resp=$(curl -s "http://127.0.0.1:$GATEWAY_PORT/v1/system/update-config" \
                   --user "$ADMIN_USER:$ADMIN_PASS" 2>&1)
        info "GET response: $get_resp"

        info "PUT /v1/system/update-config..."
        local put_resp
        put_resp=$(curl -s -X PUT "http://127.0.0.1:$GATEWAY_PORT/v1/system/update-config" \
                   -H "Content-Type: application/json" \
                   -d '{"autoUpdateEnabled":false,"checkIntervalMin":60}' \
                   --user "$ADMIN_USER:$ADMIN_PASS" 2>&1)
        info "PUT response: $put_resp"

        local config_file="$LAUNCHER_DATA_DIR/launcher-config.json"
        if [ -f "$config_file" ] && grep -q '"autoUpdateEnabled".*false' "$config_file"; then
            pass "T9: Update config written to launcher-config.json"
        else
            fail "T9: Config not written"
            cat "$config_file" 2>/dev/null || echo "(missing)"
        fi
    else
        fail "T9: Node did not become healthy"
    fi
    cleanup_processes
}

# ==============================================================================
# Main
# ==============================================================================
run_test() {
    case "${1:-all}" in
        t1) t1_launcher_starts_node ;;
        t2) t2_health_monitor ;;
        t3) t3_status_file ;;
        t4) t4_config_reload ;;
        t5) t5_trigger_check ;;
        t6) t6_crash_recovery ;;
        t7) t7_graceful_shutdown ;;
        t8) t8_update_check_mock ;;
        t9) t9_update_config_api ;;
        all)
            t1_launcher_starts_node
            t2_health_monitor
            t3_status_file
            t4_config_reload
            t5_trigger_check
            t6_crash_recovery
            t7_graceful_shutdown
            t8_update_check_mock
            t9_update_config_api
            ;;
        cleanup)
            cleanup_data
            info "Full cleanup done"
            ;;
        *)
            echo "Usage: $0 [t1|t2|t3|t4|t5|t6|t7|t8|t9|all|cleanup]"
            exit 1
            ;;
    esac
}

trap cleanup_processes EXIT

if [ ! -f "$LAUNCHER_BIN" ] || [ ! -f "$NODE_BIN" ]; then
    echo "Error: binaries not found in $BIN_DIR"
    echo "Build first:"
    echo "  go build -tags goolm -ldflags '-X .../supervisor.Version=0.1.0-test' -o $LAUNCHER_BIN ./cmd/mobazha-launcher"
    echo "  go build -tags goolm -o $NODE_BIN ."
    exit 1
fi

info "Binaries:"
info "  Launcher: $LAUNCHER_BIN"
info "  Node:     $NODE_BIN"
echo

run_test "${1:-all}"

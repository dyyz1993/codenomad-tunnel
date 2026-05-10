#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Integration Test Suite for CodeNomad Tunnel
#
# Runs tunnel-hub + tunnel-client locally and exercises:
#   1. HTTP service forwarding
#   2. TCP service forwarding
#   3. No-service (unreachable) probe & 502
#   4. Service recovery (stop → 502, restart → 200)
#   5. Invalid subdomain rejection (400)
#   6. Duplicate subdomain rejection (409)
# ============================================================

HUB_HTTP_PORT=18080
HUB_API_PORT=18081
DOMAIN="tunnel.test"

HTTP_PORT=18888
TCP_PORT=18999
NO_SVC_PORT=19999
RECOVERY_PORT=18777

PASS=0
FAIL=0
TOTAL=0

SCENARIO_PIDS=()
HUB_PID=""

# ===================== Helpers =====================

assert_status() {
    local name="$1" expected="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$expected" = "$actual" ]; then
        echo "  PASS: $name"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name (expected=$expected, actual=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local name="$1" needle="$2" haystack="$3"
    TOTAL=$((TOTAL + 1))
    if [[ "$haystack" == *"$needle"* ]]; then
        echo "  PASS: $name"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name (expected to contain '$needle')"
        FAIL=$((FAIL + 1))
    fi
}

assert_http_status() {
    local name="$1" expected="$2" shift_amount=2
    shift 2
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$@")
    assert_status "$name" "$expected" "$code"
}

get_probe() {
    local name="$1"
    grep "Protocol:" "/tmp/client-${name}.log" 2>/dev/null \
        | head -1 | awk '{print $NF}' | tr -d '[:space:]'
}

wait_for() {
    local desc="$1" cmd="$2" max="${3:-30}"
    local waited=0
    while [ $waited -lt $max ]; do
        if eval "$cmd" > /dev/null 2>&1; then
            return 0
        fi
        sleep 0.5
        waited=$((waited + 1))
    done
    echo "  ERROR: $desc did not become ready within ${max}s"
    return 1
}

start_http_service() {
    local port="$1"
    local dir="${2:-/tmp}"
    python3 -m http.server "$port" --directory "$dir" > /dev/null 2>&1 &
    local pid=$!
    SCENARIO_PIDS+=($pid)
    wait_for "HTTP service :$port" "curl -sf http://127.0.0.1:$port/ -o /dev/null" 10
    echo "$pid"
}

start_tcp_echo() {
    local port="$1"
    python3 -c "
import socket, threading, time
def echo_server(p):
    s = socket.socket()
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    s.bind(('0.0.0.0', p))
    s.listen(5)
    while True:
        conn, _ = s.accept()
        data = conn.recv(4096)
        conn.sendall(b'ECHO:' + data)
        conn.close()
t = threading.Thread(target=echo_server, args=($port,), daemon=True)
t.start()
while True:
    time.sleep(86400)
" > /dev/null 2>&1 &
    local pid=$!
    SCENARIO_PIDS+=($pid)
    sleep 1
    echo "$pid"
}

start_client() {
    local name="$1" local_url="$2" subdomain="$3"
    ./tunnel-client \
        --hub-url "http://127.0.0.1:${HUB_API_PORT}" \
        --local "$local_url" \
        --subdomain "$subdomain" \
        --insecure \
        > "/tmp/client-${name}.log" 2>&1 &
    local pid=$!
    SCENARIO_PIDS+=($pid)

    if wait_for "client '$name'" "grep -q Protocol: /tmp/client-${name}.log" 15; then
        echo "  Client '$name' ready (PID=$pid)"
    else
        echo "  WARN: client '$name' log:"
        cat "/tmp/client-${name}.log" 2>/dev/null || true
    fi
}

cleanup_scenario() {
    for pid in "${SCENARIO_PIDS[@]:-}"; do
        kill "$pid" 2>/dev/null || true
    done
    for pid in "${SCENARIO_PIDS[@]:-}"; do
        wait "$pid" 2>/dev/null || true
    done
    SCENARIO_PIDS=()

    for port in "$HTTP_PORT" "$TCP_PORT" "$NO_SVC_PORT" "$RECOVERY_PORT"; do
        local pids
        pids=$(lsof -ti :"$port" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            for p in $pids; do
                kill "$p" 2>/dev/null || true
            done
        fi
    done
    sleep 0.5
}

cleanup_all() {
    echo ""
    echo "Cleaning up..."
    cleanup_scenario
    if [ -n "${HUB_PID:-}" ]; then
        kill "$HUB_PID" 2>/dev/null || true
        wait "$HUB_PID" 2>/dev/null || true
    fi
}

# ===================== Main =====================

echo "=========================================="
echo " Integration Test Suite"
echo "=========================================="

trap cleanup_all EXIT

# Build
echo ""
echo "Building binaries..."
go build -o tunnel-hub .
go build -o tunnel-client ./cmd/tunnel-client
echo "Build complete"

# Start hub
echo ""
echo "Starting tunnel-hub..."
./tunnel-hub \
    --domain "$DOMAIN" \
    --http-port "$HUB_HTTP_PORT" \
    --api-port "$HUB_API_PORT" \
    --public-url "http://127.0.0.1:${HUB_HTTP_PORT}" \
    > /tmp/hub.log 2>&1 &
HUB_PID=$!

if ! wait_for "hub health" "curl -sf http://127.0.0.1:${HUB_API_PORT}/api/health" 30; then
    echo "Hub log:"
    cat /tmp/hub.log
    exit 1
fi
echo "Hub ready (PID=$HUB_PID)"

# ===================== Scenario 1: HTTP Service =====================
echo ""
echo "=== Scenario 1: HTTP Service ==="

start_http_service "$HTTP_PORT"
start_client "s1" "http://127.0.0.1:${HTTP_PORT}" "http1"

PROBE=$(get_probe "s1")
assert_status "probe = http" "http" "${PROBE:-<empty>}"

assert_http_status "HTTP 200 via tunnel" "200" \
    -H "Host: http1.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/"

BODY=$(curl -s -H "Host: http1.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/")
assert_contains "response body is HTML directory listing" "Directory listing" "$BODY"

cleanup_scenario

# ===================== Scenario 2: TCP Service =====================
echo ""
echo "=== Scenario 2: TCP Service ==="

start_tcp_echo "$TCP_PORT"
start_client "s2" "tcp://127.0.0.1:${TCP_PORT}" "tcp1"

PROBE=$(get_probe "s2")
assert_status "probe = tcp" "tcp" "${PROBE:-<empty>}"

TCP_RESP=$(curl -s -H "Host: tcp1.${DOMAIN}" -d "hello world" \
    "http://127.0.0.1:${HUB_HTTP_PORT}/")
assert_contains "TCP echo response contains sent data" "ECHO:hello world" "$TCP_RESP"

cleanup_scenario

# ===================== Scenario 3: No Service =====================
echo ""
echo "=== Scenario 3: No Service (unreachable port) ==="

start_client "s3" "http://127.0.0.1:${NO_SVC_PORT}" "nosvc"

PROBE=$(get_probe "s3")
assert_status "probe = unknown" "unknown" "${PROBE:-<empty>}"

assert_http_status "HTTP 502 for unreachable service" "502" \
    -H "Host: nosvc.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/"

cleanup_scenario

# ===================== Scenario 4: Service Recovery =====================
echo ""
echo "=== Scenario 4: Service Recovery ==="

HTTP_PID=$(start_http_service "$RECOVERY_PORT")
start_client "s4" "http://127.0.0.1:${RECOVERY_PORT}" "recovery"

assert_http_status "initial HTTP 200" "200" \
    -H "Host: recovery.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/"

echo "  Stopping HTTP service (PID=$HTTP_PID)..."
kill -9 "$HTTP_PID" 2>/dev/null || true
wait "$HTTP_PID" 2>/dev/null || true

STALE_PIDS=$(lsof -ti :${RECOVERY_PORT} 2>/dev/null || true)
if [ -n "$STALE_PIDS" ]; then
    for p in $STALE_PIDS; do kill "$p" 2>/dev/null || true; done
fi
sleep 0.5

wait_for_502() {
    local waited=0
    while [ $waited -lt 20 ]; do
        local code
        code=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: recovery.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/")
        if [ "$code" = "502" ]; then
            return 0
        fi
        sleep 0.5
        waited=$((waited + 1))
    done
    return 1
}

if wait_for_502; then
    echo "  PASS: HTTP 502 after service stop"
    PASS=$((PASS + 1))
else
    local_code=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: recovery.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/")
    echo "  FAIL: HTTP 502 after service stop (expected=502, actual=${local_code})"
    FAIL=$((FAIL + 1))
fi

echo "  Restarting HTTP service..."
HTTP_PID2=$(start_http_service "$RECOVERY_PORT")
sleep 1

assert_http_status "HTTP 200 after service restart" "200" \
    -H "Host: recovery.${DOMAIN}" "http://127.0.0.1:${HUB_HTTP_PORT}/"

cleanup_scenario

# ===================== Scenario 5: Invalid Subdomain =====================
echo ""
echo "=== Scenario 5: Invalid Subdomain ==="

RESP=$(curl -s -w "\n%{http_code}" -X POST \
    "http://127.0.0.1:${HUB_API_PORT}/api/tunnels" \
    -H "Content-Type: application/json" \
    -d '{"subdomain":"-bad","targetHost":"127.0.0.1","targetPort":80}')
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')

assert_status "reject invalid subdomain with 400" "400" "$HTTP_CODE"
assert_contains "error mentions invalid" "invalid" "$BODY"

# ===================== Scenario 6: Duplicate Subdomain =====================
echo ""
echo "=== Scenario 6: Duplicate Subdomain ==="

RESP1=$(curl -s -w "\n%{http_code}" -X POST \
    "http://127.0.0.1:${HUB_API_PORT}/api/tunnels" \
    -H "Content-Type: application/json" \
    -d '{"subdomain":"dup","targetHost":"127.0.0.1","targetPort":80}')
CODE1=$(echo "$RESP1" | tail -1)
assert_status "first tunnel created with 201" "201" "$CODE1"

RESP2=$(curl -s -w "\n%{http_code}" -X POST \
    "http://127.0.0.1:${HUB_API_PORT}/api/tunnels" \
    -H "Content-Type: application/json" \
    -d '{"subdomain":"dup","targetHost":"127.0.0.1","targetPort":80}')
CODE2=$(echo "$RESP2" | tail -1)
BODY2=$(echo "$RESP2" | sed '$d')

assert_status "duplicate subdomain rejected with 409" "409" "$CODE2"
assert_contains "error mentions already exists" "already exists" "$BODY2"

# ===================== Summary =====================
echo ""
echo "=========================================="
echo " Results: $PASS passed, $FAIL failed (total: $TOTAL)"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0

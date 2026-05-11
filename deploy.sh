#!/usr/bin/env bash
set -euo pipefail

REPO="dyyz1993/codenomad-tunnel"
INSTALL_DIR="/opt/codenomad-tunnel"
MODE=""
DOMAIN=""
API_DOMAIN=""
API_PORT=8080
HTTP_PORT=80
PUBLIC_URL=""
HUB_URL=""
LOCAL=""
SUBDOMAIN=""
NAME=""
INSECURE=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; }

usage() {
  cat <<EOF

Usage:
  Deploy hub (server):   $(basename "$0") hub --domain tunnel.yourdomain.com [options]
  Connect client:        $(basename "$0") client --hub-url https://api.tunnel.yourdomain.com --local http://localhost:3000 [options]

Hub options:
  --domain DOMAIN        Base domain for tunnels (required, supports *.domain.com)
  --api-domain DOMAIN    API domain (default: api.{domain})
  --http-port PORT       HTTP proxy port (default: 80)
  --api-port PORT        API port (default: 8080)
  --public-url URL       Full public base URL
  --dir PATH             Install directory (default: /opt/codenomad-tunnel)

Client options:
  --hub-url URL          Tunnel hub API URL (required)
  --local URL            Local service URL, e.g. http://localhost:3000 (required)
  --subdomain SUB        Requested subdomain (auto-generated if empty)
  --name NAME            Tunnel name
  --insecure             Skip TLS verification

EOF
  exit 0
}

[ $# -eq 0 ] && usage

MODE="$1"
shift

case "$MODE" in
  hub|server) MODE=hub ;;
  client) ;;
  -h|--help) usage ;;
  *) err "Unknown mode: $MODE (use 'hub' or 'client')"; exit 1 ;;
esac

while [[ $# -gt 0 ]]; do
  case $1 in
    --domain)      DOMAIN="$2"; shift 2 ;;
    --api-domain)  API_DOMAIN="$2"; shift 2 ;;
    --http-port)   HTTP_PORT="$2"; shift 2 ;;
    --api-port)    API_PORT="$2"; shift 2 ;;
    --public-url)  PUBLIC_URL="$2"; shift 2 ;;
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    --hub-url)     HUB_URL="$2"; shift 2 ;;
    --local)       LOCAL="$2"; shift 2 ;;
    --subdomain)   SUBDOMAIN="$2"; shift 2 ;;
    --name)        NAME="$2"; shift 2 ;;
    --insecure)    INSECURE=true; shift ;;
    -h|--help)     usage ;;
    *) err "Unknown option: $1"; exit 1 ;;
  esac
done

detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case $ARCH in
    x86_64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) err "Unsupported architecture: $ARCH"; exit 1 ;;
  esac
}

download() {
  local name="$1"
  local dest="$2"
  local URL="https://github.com/${REPO}/releases/latest/download/${name}-${OS}-${ARCH}"
  [ "$OS" = "windows" ] && URL="${URL}.exe"

  info "Downloading ${name}..."
  if curl -fsSL --max-time 120 "$URL" -o "$dest"; then
    chmod +x "$dest"
    ok "${name} downloaded"
    return 0
  fi
  return 1
}

download_or_build() {
  local name="$1"
  local dest="$2"
  local build_target="$3"

  if download "$name" "$dest"; then
    return 0
  fi

  warn "Pre-built binary not available, building from source..."
  if ! command -v go &> /dev/null; then
    err "Go not installed. Install Go or check GitHub Releases: https://github.com/${REPO}/releases"
    exit 1
  fi

  local tmp_src
  tmp_src=$(mktemp -d)
  git clone "https://github.com/${REPO}.git" "$tmp_src"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$dest" "$tmp_src/$build_target"
  chmod +x "$dest"
  rm -rf "$tmp_src"
  ok "${name} built from source"
}

# ─── HUB MODE ───
run_hub() {
  if [ -z "$DOMAIN" ]; then
    err "--domain is required for hub mode"
    exit 1
  fi

  DOMAIN="${DOMAIN#\*.}"
  DOMAIN="${DOMAIN#\*}"
  [ -z "$API_DOMAIN" ] && API_DOMAIN="api.${DOMAIN}"

  echo ""
  echo "🚀 Deploying CodeNomad Tunnel Hub"
  echo "   Domain:     $DOMAIN"
  echo "   API Domain: $API_DOMAIN"
  echo "   HTTP Port:  $HTTP_PORT"
  echo "   API Port:   $API_PORT"
  echo ""

  detect_platform

  if [ "$(id -u)" -eq 0 ] && [ "$OS" = "linux" ]; then
    mkdir -p "$INSTALL_DIR"
    download_or_build "tunnel-hub" "$INSTALL_DIR/tunnel-hub" "."

    local PUBLIC_URL_FLAG=""
    [ -n "$PUBLIC_URL" ] && PUBLIC_URL_FLAG="--public-url $PUBLIC_URL"

    cat > /etc/systemd/system/codenomad-tunnel.service <<EOF
[Unit]
Description=CodeNomad Tunnel Hub
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/tunnel-hub --domain ${DOMAIN} --api-domain ${API_DOMAIN} --http-port ${HTTP_PORT} --api-port ${API_PORT} ${PUBLIC_URL_FLAG}
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=5
LimitNOFILE=65535
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable codenomad-tunnel
    systemctl restart codenomad-tunnel

    sleep 3
    if systemctl is-active --quiet codenomad-tunnel; then
      ok "Tunnel Hub is running!"
      echo ""
      echo "  Public wildcard:  *.${DOMAIN}"
      echo "  API domain:       ${API_DOMAIN}"
      echo "  Health:           http://127.0.0.1:${API_PORT}/api/health"
    else
      err "Service failed to start"
      journalctl -u codenomad-tunnel -n 30 --no-pager
      exit 1
    fi
  else
    local TMPDIR
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT
    download_or_build "tunnel-hub" "$TMPDIR/tunnel-hub" "."

    local PUBLIC_URL_FLAG=""
    [ -n "$PUBLIC_URL" ] && PUBLIC_URL_FLAG="--public-url $PUBLIC_URL"

    ok "Starting tunnel-hub..."
    exec "$TMPDIR/tunnel-hub" \
      --domain "$DOMAIN" \
      --api-domain "$API_DOMAIN" \
      --http-port "$HTTP_PORT" \
      --api-port "$API_PORT" \
      $PUBLIC_URL_FLAG
  fi
}

# ─── CLIENT MODE ───
run_client() {
  if [ -z "$HUB_URL" ] || [ -z "$LOCAL" ]; then
    err "--hub-url and --local are required for client mode"
    exit 1
  fi

  echo ""
  echo "🔗 Connecting tunnel client"
  echo "   Hub:   $HUB_URL"
  echo "   Local: $LOCAL"
  echo ""

  detect_platform

  local TMPDIR
  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT
  download_or_build "tunnel-client" "$TMPDIR/tunnel-client" "cmd/tunnel-client"

  local FLAGS="--hub-url $HUB_URL --local $LOCAL"
  [ -n "$SUBDOMAIN" ] && FLAGS="$FLAGS --subdomain $SUBDOMAIN"
  [ -n "$NAME" ] && FLAGS="$FLAGS --name $NAME"
  $INSECURE && FLAGS="$FLAGS --insecure"

  ok "Starting tunnel-client..."
  exec "$TMPDIR/tunnel-client" $FLAGS
}

# ─── MAIN ───
case "$MODE" in
  hub)    run_hub ;;
  client) run_client ;;
esac

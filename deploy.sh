#!/usr/bin/env bash
set -euo pipefail

# CodeNomad Tunnel Hub - One-Click VPS Deployment
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dyyz1993/codenomad-tunnel/main/deploy.sh | bash -s -- --domain tunnel.yourdomain.com
#
#   Or clone and run:
#   ./deploy.sh --domain tunnel.yourdomain.com [--api-port 8080] [--http-port 80] [--dir /opt/codenomad-tunnel]

DOMAIN=""
API_PORT=8080
HTTP_PORT=80
INSTALL_DIR="/opt/codenomad-tunnel"
REPO="dyyz1993/codenomad-tunnel"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}ℹ️  $*${NC}"; }
ok()    { echo -e "${GREEN}✅ $*${NC}"; }
warn()  { echo -e "${YELLOW}⚠️  $*${NC}"; }
err()   { echo -e "${RED}❌ $*${NC}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --domain)    DOMAIN="$2"; shift 2 ;;
    --api-port)  API_PORT="$2"; shift 2 ;;
    --http-port) HTTP_PORT="$2"; shift 2 ;;
    --dir)       INSTALL_DIR="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: ./deploy.sh --domain tunnel.yourdomain.com [--api-port 8080] [--http-port 80] [--dir /opt/codenomad-tunnel]"
      exit 0
      ;;
    *) err "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$DOMAIN" ]; then
  err "--domain is required"
  echo ""
  echo "Usage: ./deploy.sh --domain tunnel.yourdomain.com"
  echo ""
  echo "Options:"
  echo "  --domain DOMAIN      Base domain for tunnels (required)"
  echo "  --http-port PORT     HTTP tunnel proxy port (default: 80)"
  echo "  --api-port PORT      Management API port (default: 8080)"
  echo "  --dir PATH           Installation directory (default: /opt/codenomad-tunnel)"
  exit 1
fi

echo ""
echo "🚀 Deploying CodeNomad Tunnel Hub..."
echo "   Domain:    $DOMAIN"
echo "   HTTP Port: $HTTP_PORT"
echo "   API Port:  $API_PORT"
echo "   Install:   $INSTALL_DIR"
echo ""

if [ "$(id -u)" -ne 0 ]; then
  err "This script must be run as root (use sudo)"
  exit 1
fi

ARCH=$(uname -m)
case $ARCH in
  x86_64)         ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) err "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
  err "This script is designed for Linux. On macOS, use: go build -o tunnel-hub ."
  exit 1
fi

info "System: $OS/$ARCH"

ensure_deps() {
  local missing=()
  for cmd in curl git; do
    if ! command -v "$cmd" &> /dev/null; then
      missing+=("$cmd")
    fi
  done

  if [ ${#missing[@]} -gt 0 ]; then
    info "Installing missing dependencies: ${missing[*]}"
    if command -v apt-get &> /dev/null; then
      apt-get update -qq && apt-get install -y -qq "${missing[@]}"
    elif command -v yum &> /dev/null; then
      yum install -y "${missing[@]}"
    elif command -v apk &> /dev/null; then
      apk add "${missing[@]}"
    elif command -v dnf &> /dev/null; then
      dnf install -y "${missing[@]}"
    else
      err "Cannot install dependencies automatically. Please install: ${missing[*]}"
      exit 1
    fi
  fi
}

ensure_deps

mkdir -p "$INSTALL_DIR"

build_from_source() {
  info "Building from source..."
  if [ ! -d "$INSTALL_DIR/src" ]; then
    git clone "https://github.com/${REPO}.git" "$INSTALL_DIR/src"
  else
    git -C "$INSTALL_DIR/src" pull || true
  fi
  cd "$INSTALL_DIR/src"
  CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -ldflags="-s -w" -o "$INSTALL_DIR/tunnel-hub" .
  ok "Built from source"
}

download_binary() {
  local BINARY_URL="https://github.com/${REPO}/releases/latest/download/tunnel-hub-${OS}-${ARCH}"
  info "Downloading pre-built binary from $BINARY_URL ..."
  if curl -fsSL --max-time 60 "$BINARY_URL" -o "$INSTALL_DIR/tunnel-hub"; then
    ok "Downloaded pre-built binary"
    return 0
  fi
  return 1
}

install_go() {
  local GO_VERSION="1.23.6"
  local GO_TARBALL="go${GO_VERSION}.${OS}-${ARCH}.tar.gz"
  local GO_URL="https://go.dev/dl/${GO_TARBALL}"

  info "Installing Go ${GO_VERSION}..."
  if [ ! -d /usr/local/go ]; then
    curl -fsSL "$GO_URL" | tar -C /usr/local -xzf -
    export PATH="/usr/local/go/bin:$PATH"
    echo 'export PATH="/usr/local/go/bin:$PATH"' > /etc/profile.d/go.sh
    ok "Go ${GO_VERSION} installed"
  else
    ok "Go already installed at /usr/local/go"
  fi
}

if command -v go &> /dev/null; then
  info "Go $(go version | awk '{print $3}') detected"
  build_from_source
else
  if ! download_binary; then
    warn "Pre-built binary not available for this platform"
    install_go
    export PATH="/usr/local/go/bin:${PATH:-}"
    build_from_source
  fi
fi

chmod +x "$INSTALL_DIR/tunnel-hub"

ok "Binary installed to $INSTALL_DIR/tunnel-hub"

cat > /etc/systemd/system/codenomad-tunnel.service << EOF
[Unit]
Description=CodeNomad Tunnel Hub
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/tunnel-hub --domain ${DOMAIN} --http-port ${HTTP_PORT} --api-port ${API_PORT}
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

info "Waiting for service to start..."
sleep 3

if systemctl is-active --quiet codenomad-tunnel; then
  HEALTH_OK=false
  for i in 1 2 3; do
    if curl -sf "http://127.0.0.1:${API_PORT}/api/health" > /dev/null 2>&1; then
      HEALTH_OK=true
      break
    fi
    sleep 2
  done

  if $HEALTH_OK; then
    echo ""
    ok "Tunnel Hub is running!"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  🌐 Public wildcard:  *.${DOMAIN}"
    echo "  📊 API health:       http://127.0.0.1:${API_PORT}/api/health"
    echo ""
    echo "  📝 CodeNomad CLI flag:"
    echo "     --tunnel-hub-url http://${DOMAIN}"
    echo ""
    echo "  📝 Or in CodeNomad settings (config.json):"
    echo "     \"tunnelHubUrl\": \"http://${DOMAIN}\""
    echo ""
    echo "  🔧 Management commands:"
    echo "     systemctl status  codenomad-tunnel"
    echo "     systemctl restart  codenomad-tunnel"
    echo "     systemctl stop     codenomad-tunnel"
    echo "     journalctl -u codenomad-tunnel -f"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
  else
    warn "Service is running but health check failed."
    info "Check logs: journalctl -u codenomad-tunnel -n 50"
  fi
else
  err "Service failed to start"
  echo ""
  journalctl -u codenomad-tunnel -n 30 --no-pager
  exit 1
fi

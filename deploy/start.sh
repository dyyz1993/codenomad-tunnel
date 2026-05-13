#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

mkdir -p ssh data

if [ ! -f ssh/authorized_keys ]; then
    echo "[1/3] Injecting SSH public key from ~/.ssh/id_rsa.pub ..."
    cp ~/.ssh/id_rsa.pub ssh/authorized_keys
    chmod 644 ssh/authorized_keys
fi

echo "[2/3] Pulling latest image ..."
docker compose pull

echo "[3/3] Starting tunnel ..."
docker compose up -d

sleep 2
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
docker compose ps
echo ""
echo "  Proxy:  http://localhost:${HOST_PROXY_PORT:-19090}/"
echo "  API:    http://localhost:${HOST_API_PORT:-19091}/api/health"
echo "  SSH:    ssh -p ${HOST_SSH_PORT:-2223} tunnel@localhost"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

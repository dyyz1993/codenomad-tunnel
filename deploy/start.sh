#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f authorized_keys ]; then
    echo "Generating authorized_keys from local ~/.ssh/id_rsa.pub ..."
    cp ~/.ssh/id_rsa.pub authorized_keys
    echo "Done. You can edit authorized_keys to add more keys."
fi

mkdir -p data

echo "Pulling latest image..."
docker compose pull

echo "Starting tunnel..."
docker compose up -d

echo ""
echo "Status:"
docker compose ps
echo ""
echo "SSH:    ssh -p ${HOST_SSH_PORT:-2222} tunnel@localhost"
echo "API:    http://localhost:${HOST_API_PORT:-18081}/api/health"
echo "Proxy:  http://localhost:${HOST_PROXY_PORT:-18080}/"
echo ""
echo "Logs: docker compose logs -f"

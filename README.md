# CodeNomad Tunnel

[![Website](https://img.shields.io/badge/Website-dyyz1993.github.io-blue?style=flat-square)](https://dyyz1993.github.io/codenomad-tunnel/)
[![Release](https://img.shields.io/github/v/tag/dyyz1993/codenomad-tunnel?style=flat-square&label=Release)](https://github.com/dyyz1993/codenomad-tunnel/releases)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

HTTP-over-WebSocket reverse proxy tunnel service. Expose local services to the internet via wildcard subdomains.

**[dyyz1993.github.io/codenomad-tunnel](https://dyyz1993.github.io/codenomad-tunnel/)** &mdash; One-command deployment guide.

## Quick Start

### One-Click VPS Deployment

Deploy to any Linux VPS with a single command:

```bash
# Simple: wildcard domain input (auto-strips *. prefix)
curl -fsSL https://raw.githubusercontent.com/dyyz1993/codenomad-tunnel/main/deploy.sh | sudo bash -s -- --domain "*.tunnel.yourdomain.com"

# With custom API domain:
curl -fsSL https://raw.githubusercontent.com/dyyz1993/codenomad-tunnel/main/deploy.sh | sudo bash -s -- --domain "*.tunnel.yourdomain.com" --api-domain "api.tunnel.yourdomain.com"
```

This auto-generates:
- **API domain**: `api.tunnel.yourdomain.com`
- **Tunnel wildcard**: `*.tunnel.yourdomain.com`
- **Public URL**: `https://k8f2x1.tunnel.yourdomain.com`
- **API URL**: `https://api.tunnel.yourdomain.com`

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `--domain` | *(required)* | Base domain for tunnels (supports `*.domain.com` format) |
| `--api-domain` | `api.{domain}` | Domain for management API |
| `--http-port` | `80` | HTTP tunnel proxy port |
| `--api-port` | `8080` | Management API port |
| `--dir` | `/opt/codenomad-tunnel` | Installation directory |

The script auto-detects your architecture, downloads a pre-built binary (or builds from source), and sets up a systemd service.

### Docker

```bash
docker compose up -d
```

### Build from Source

```bash
go build -o tunnel-hub .
./tunnel-hub --domain "*.tunnel.yourdomain.com" --http-port 80 --api-port 8080
```

## DNS & Domain Configuration

### Standard setup (port 80)

```
*.tunnel.yourdomain.com  →  A record  →  your VPS IP
```

No extra flags needed — public URLs default to `http://subdomain.tunnel.yourdomain.com`.

### Custom port (e.g., 8080)

```
*.tunnel.yourdomain.com  →  A record  →  your VPS IP

./tunnel-hub --domain tunnel.yourdomain.com --http-port 8080 \
  --public-url http://tunnel.yourdomain.com:8080
```

### Behind reverse proxy (recommended for HTTPS)

```
DNS: *.tunnel.yourdomain.com → VPS IP

Caddy/Nginx:
  *.tunnel.yourdomain.com {
    reverse_proxy localhost:8080
  }

tunnel-hub:
  ./tunnel-hub --domain tunnel.yourdomain.com --http-port 8080 \
    --public-url https://tunnel.yourdomain.com
```

## DNS Setup

Create a wildcard DNS record pointing to your server:

```
*.tunnel.yourdomain.com.  IN  A  YOUR_VPS_IP
```

## API Documentation

The management API is available on the dedicated API port (8080) and also via the `--api-domain` on the HTTP port:

```bash
# Via API domain (recommended, works through reverse proxy/HTTPS):
curl https://api.tunnel.yourdomain.com/api/health

# Via dedicated API port:
curl http://localhost:8080/api/health
```

### Create a Tunnel

```bash
curl -X POST http://localhost:8080/api/tunnels \
  -H "Content-Type: application/json" \
  -d '{"name":"my-service","targetHost":"localhost","targetPort":3000}'
```

Response:
```json
{
  "id": "t_k8f2x1",
  "subdomain": "k8f2x1",
  "publicUrl": "https://tunnel.example.com/k8f2x1",
  "relayUrl": "wss://tunnel.example.com/relay/t_k8f2x1",
  "status": "waiting",
  "name": "my-service",
  "createdAt": "2026-05-10T12:00:00Z"
}
```

### List Tunnels

```bash
curl http://localhost:8080/api/tunnels
```

### Get Tunnel Details

```bash
curl http://localhost:8080/api/tunnels/t_k8f2x1
```

### Delete a Tunnel

```bash
curl -X DELETE http://localhost:8080/api/tunnels/t_k8f2x1
```

### Stream Request Logs (SSE)

```bash
curl -N http://localhost:8080/api/tunnels/t_k8f2x1/logs
```

### Health Check

```bash
curl http://localhost:8080/api/health
```

## Client Integration

After creating a tunnel, connect a WebSocket client to the `relayUrl`. The relay protocol sends JSON frames:

### Incoming Request (server → client)

```json
{
  "type": "request",
  "id": "r_abc12345",
  "method": "GET",
  "path": "/api/data",
  "query": "x=1",
  "headers": {"Accept": "application/json"},
  "body": ""
}
```

### Response (client → server)

Text response:
```json
{
  "type": "response",
  "id": "r_abc12345",
  "status": 200,
  "headers": {"Content-Type": "application/json"},
  "body": "{\"ok\":true}"
}
```

Binary response (use base64):
```json
{
  "type": "response",
  "id": "r_abc12345",
  "status": 200,
  "headers": {"Content-Type": "image/png"},
  "bodyBase64": "iVBORw0KGgo..."
}
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--domain` | `tunnel.example.com` | Base domain for tunnels (supports `*.domain.com`) |
| `--api-domain` | `api.{domain}` | Domain for management API on HTTP port |
| `--http-port` | `80` | HTTP tunnel proxy port |
| `--api-port` | `8080` | Management API port |
| `--tls-cert` | | TLS certificate path |
| `--tls-key` | | TLS key path |
| `--public-url` | | Full public base URL (overrides derived URL) |

Environment variables: `TUNNEL_DOMAIN`, `API_DOMAIN`, `HTTP_PORT`, `API_PORT`, `TLS_CERT`, `TLS_KEY`, `TUNNEL_PUBLIC_URL`

## TLS

Mount certificates and set env vars:

```yaml
environment:
  - TLS_CERT=/certs/fullchain.pem
  - TLS_KEY=/certs/privkey.pem
volumes:
  - ./certs:/certs:ro
```

## Architecture

- **HTTP proxy** on port 80: catches `*.tunnel.domain.com` requests, extracts subdomain, forwards through WebSocket. If the Host matches `--api-domain`, routes to the management API instead.
- **WebSocket relay** at `/relay/{tunnelId}`: local clients connect here, relay HTTP requests/responses
- **Management API** on port 8080: CRUD for tunnels, stats, health. Also accessible via `--api-domain` on the HTTP port.

Each tunnel handles multiple concurrent requests via unique request IDs. Requests time out after 30s if the relay client doesn't respond.

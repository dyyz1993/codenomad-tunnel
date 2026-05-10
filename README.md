# CodeNomad Tunnel Hub

HTTP-over-WebSocket reverse proxy tunnel service. Expose local services to the internet via wildcard subdomains.

## Quick Start

```bash
docker compose up -d
```

Or run directly:

```bash
go build -o tunnel-hub .
./tunnel-hub --domain tunnel.yourdomain.com --http-port 80 --api-port 8080
```

## DNS Setup

Create a wildcard DNS record pointing to your server:

```
*.tunnel.yourdomain.com.  IN  A  YOUR_VPS_IP
```

## API Documentation

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
  "publicUrl": "https://k8f2x1.tunnel.yourdomain.com",
  "relayUrl": "wss://tunnel.yourdomain.com/relay/t_k8f2x1",
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
| `--domain` | `tunnel.example.com` | Base domain for tunnels |
| `--http-port` | `80` | HTTP tunnel proxy port |
| `--api-port` | `8080` | Management API port |
| `--tls-cert` | | TLS certificate path |
| `--tls-key` | | TLS key path |

Environment variables: `TUNNEL_DOMAIN`, `HTTP_PORT`, `API_PORT`, `TLS_CERT`, `TLS_KEY`

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

- **HTTP proxy** on port 80: catches `*.tunnel.domain.com` requests, extracts subdomain, forwards through WebSocket
- **WebSocket relay** at `/relay/{tunnelId}`: local clients connect here, relay HTTP requests/responses
- **Management API** on port 8080: CRUD for tunnels, stats, health

Each tunnel handles multiple concurrent requests via unique request IDs. Requests time out after 30s if the relay client doesn't respond.

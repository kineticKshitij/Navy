# Docker Deployment Guide

## Quick Start

### Build and Run with Docker Compose

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f server
docker-compose logs -f agent-linux

# Stop services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Access Dashboard

- **Web Dashboard**: http://localhost:8080
- **API Endpoint**: http://localhost:8080/api
- **Health Check**: http://localhost:8080/api/health

## Individual Container Build

### Server

```bash
# Build
docker build -f Dockerfile.server -t ipsec-server:latest .

# Run
docker run -d \
  --name ipsec-server \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ipsec-server:latest

# View logs
docker logs -f ipsec-server
```

### Agent

```bash
# Build
docker build -f Dockerfile.agent -t ipsec-agent:latest .

# Run (requires privileged mode for IPsec)
docker run -d \
  --name ipsec-agent \
  --privileged \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  -e SERVER_URL=http://ipsec-server:8080 \
  -e PEER_ID=agent-001 \
  ipsec-agent:latest \
  start --server http://ipsec-server:8080 --peer-id agent-001

# View logs
docker logs -f ipsec-agent
```

## Architecture

```
┌─────────────────────┐
│  Web Browser        │
│  :8080              │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│  ipsec-server       │
│  Port: 8080         │
│  Network: bridge    │
└──────────┬──────────┘
           │
    ───────┴────────
    │              │
┌───▼────────┐ ┌───▼────────┐
│ agent-     │ │ agent-     │
│ linux      │ │ linux-2    │
│ (Ubuntu)   │ │ (Ubuntu)   │
└────────────┘ └────────────┘
```

## Environment Variables

### Server

- `LOG_LEVEL`: Log level (debug, info, warn, error) - default: info
- `DB_PATH`: SQLite database path - default: /app/data/ipsec.db
- `LISTEN_ADDR`: Server listen address - default: :8080

### Agent

- `SERVER_URL`: Management server URL (required)
- `PEER_ID`: Unique peer identifier (required)
- `LOG_LEVEL`: Log level - default: info
- `SYNC_INTERVAL`: Policy sync interval in seconds - default: 60

## Testing the Deployment

### 1. Check Services

```bash
# Check if containers are running
docker-compose ps

# Check server health
curl http://localhost:8080/api/health

# List registered peers
curl http://localhost:8080/api/peers
```

### 2. Create Test Policy

```bash
# Create policy
curl -X POST http://localhost:8080/api/policies \
  -H "Content-Type: application/json" \
  -d @test-policy.json

# List policies
curl http://localhost:8080/api/policies
```

### 3. Monitor Agents

```bash
# Check agent logs
docker-compose logs -f agent-linux

# Execute commands in agent container
docker exec -it ipsec-agent-linux ./ipsec-agent status

# Check strongSwan status
docker exec -it ipsec-agent-linux swanctl --list-conns
docker exec -it ipsec-agent-linux swanctl --list-sas
```

## Troubleshooting

### Agent Can't Connect to Server

```bash
# Check network connectivity
docker exec ipsec-agent-linux ping -c 3 ipsec-server

# Check server logs
docker-compose logs server

# Restart agent
docker-compose restart agent-linux
```

### Tunnels Not Establishing

```bash
# Check strongSwan status
docker exec -it ipsec-agent-linux swanctl --list-sas

# Check system logs
docker exec -it ipsec-agent-linux journalctl -u strongswan-swanctl

# Verify IPsec kernel modules
docker exec -it ipsec-agent-linux lsmod | grep ip_
```

### Permission Issues

```bash
# Ensure agent runs with --privileged flag
docker-compose down
docker-compose up -d

# Check capabilities
docker exec -it ipsec-agent-linux capsh --print
```

## Production Considerations

### Security

1. **Change default secrets**: Update PSKs in policies
2. **Use TLS**: Deploy server behind reverse proxy with HTTPS
3. **Network isolation**: Use separate networks for management and data plane
4. **Secrets management**: Use Docker secrets or external vault

### Scaling

1. **Multiple agents**: Agents can be scaled horizontally
2. **Load balancing**: Use multiple server replicas with shared database
3. **High availability**: Deploy server in HA configuration

### Monitoring

1. **Prometheus metrics**: Add metrics endpoint
2. **Log aggregation**: Configure centralized logging
3. **Alerting**: Set up alerts for tunnel failures

## Cleanup

```bash
# Stop all services
docker-compose down

# Remove all data
docker-compose down -v

# Remove images
docker rmi ipsec-server:latest ipsec-agent:latest

# Prune everything
docker system prune -a --volumes
```

## Next Steps

1. Customize configs in `configs/` directory
2. Add certificate-based authentication
3. Implement policy templates
4. Add more agent platforms (Windows, macOS)
5. Set up CI/CD pipeline

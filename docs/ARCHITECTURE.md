# Technical Architecture

## System Overview

The Unified Cross-Platform IPsec Manager is a distributed system consisting of:

1. **Policy Management Server**: Centralized REST API server for policy management
2. **Agent Daemons**: Lightweight agents running on each managed node
3. **Web Dashboard**: Real-time monitoring and management interface
4. **IPsec Abstraction Layer**: Platform-specific implementations for Linux, Windows, and macOS

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Management Server                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  REST API    │  │  Web UI      │  │  WebSocket   │      │
│  │  (Echo)      │  │  (Svelte)    │  │  (Real-time) │      │
│  └──────┬───────┘  └──────────────┘  └──────────────┘      │
│         │                                                     │
│  ┌──────▼───────┐  ┌──────────────┐                        │
│  │  Policy      │  │  Peer        │                        │
│  │  Storage     │◄─┤  Registry    │                        │
│  │  (SQLite)    │  └──────────────┘                        │
│  └──────────────┘                                           │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ HTTPS/REST
                 │
    ┌────────────┼────────────┬────────────┐
    │            │            │            │
┌───▼───┐    ┌──▼────┐   ┌──▼────┐   ┌──▼────┐
│ Agent │    │ Agent │   │ Agent │   │ Agent │
│Linux  │    │Windows│   │ macOS │   │ BOSS  │
└───┬───┘    └───┬───┘   └───┬───┘   └───┬───┘
    │            │           │           │
┌───▼───┐    ┌──▼────┐   ┌──▼────┐   ┌──▼────┐
│strong │    │Windows│   │ macOS │   │strong │
│Swan   │    │IPsec  │   │ IPsec │   │Swan   │
└───────┘    └───────┘   └───────┘   └───────┘
```

## Component Design

### 1. Policy Management Server

**Technology Stack:**
- Language: Go 1.21+
- Web Framework: Echo v4
- Database: SQLite (modernc.org/sqlite)
- Frontend: Svelte + Vite + Tailwind CSS

**Key Responsibilities:**
- Store and distribute IPsec policies
- Maintain peer registry and status
- Provide REST API for policy management
- Serve web dashboard for monitoring
- Audit logging of all changes

**API Endpoints:**

```
GET    /api/policies          - List all policies
POST   /api/policies          - Create new policy
GET    /api/policies/:id      - Get policy details
PUT    /api/policies/:id      - Update policy
DELETE /api/policies/:id      - Delete policy

POST   /api/peers/register    - Register new peer
GET    /api/peers             - List all peers
GET    /api/peers/:id         - Get peer details
PUT    /api/peers/:id/status  - Update peer status

GET    /api/tunnels           - List all tunnels (aggregated)
GET    /api/tunnels/:name     - Get tunnel details

GET    /api/health            - Health check
```

### 2. Agent Daemon

**Technology Stack:**
- Language: Go 1.21+
- CLI Framework: Cobra
- Configuration: Viper (YAML)
- Service Management: kardianos/service

**Key Responsibilities:**
- Fetch policies from server periodically
- Apply policies to local IPsec implementation
- Monitor tunnel health and status
- Auto-restart failed tunnels (watchdog)
- Report status back to server
- Start on boot as system service

**Agent State Machine:**

```
    ┌──────────┐
    │  Start   │
    └────┬─────┘
         │
    ┌────▼──────┐
    │ Register  │
    │ with      │
    │ Server    │
    └────┬──────┘
         │
    ┌────▼──────┐
    │   Sync    │◄──────┐
    │ Policies  │       │
    └────┬──────┘       │
         │              │
    ┌────▼──────┐       │
    │   Apply   │       │
    │  Tunnels  │       │
    └────┬──────┘       │
         │              │
    ┌────▼──────┐       │
    │  Monitor  │───────┘
    │  & Watch  │  (every 60s)
    └───────────┘
```

### 3. IPsec Abstraction Layer

**Interface Design:**

```go
type IPsecManager interface {
    CreateTunnel(ctx context.Context, config TunnelConfig) error
    DeleteTunnel(ctx context.Context, name string) error
    UpdateTunnel(ctx context.Context, config TunnelConfig) error
    StartTunnel(ctx context.Context, name string) error
    StopTunnel(ctx context.Context, name string) error
    GetTunnelStatus(ctx context.Context, name string) (*TunnelStatus, error)
    ListTunnels(ctx context.Context) ([]TunnelStatus, error)
    GetStatistics(ctx context.Context, name string) (*TrafficStats, error)
    GetSAInfo(ctx context.Context, name string) ([]SAInfo, error)
    ValidateConfig(config TunnelConfig) error
    Initialize(ctx context.Context) error
    Cleanup(ctx context.Context) error
}
```

**Platform Implementations:**

#### Linux (strongSwan)
- **Method**: VICI protocol (govici library)
- **Configuration**: swanctl.conf generation
- **Commands**: `swanctl --load-all`, `swanctl --initiate`
- **Status**: Parse VICI session data
- **Features**: Full support for all IPsec modes

#### Windows (NetIPsec)
- **Method**: PowerShell cmdlets
- **Configuration**: NetIPsec rules via PowerShell
- **Commands**: `New-NetIPsecRule`, `Get-NetIPsecQuickModeSA`
- **Status**: Parse JSON output from PowerShell
- **Features**: ESP and AH modes, certificate and PSK auth

#### macOS (Native VPN)
- **Method**: scutil commands + configuration profiles
- **Configuration**: racoon.conf / IKEv2 profiles
- **Commands**: `scutil --nc start`, `scutil --nc status`
- **Status**: Parse text output
- **Features**: IKEv2 preferred, limited AH support

#### BOSS OS
- **Method**: Same as Debian (strongSwan)
- **Configuration**: Identical to Linux implementation
- **Note**: BOSS OS is Debian-based, fully compatible

## Policy Engine

**Policy Structure:**

```yaml
id: string              # Unique policy ID
name: string            # Human-readable name
description: string     # Optional description
version: int            # Policy version
enabled: bool           # Is policy active?
priority: int           # Higher = applied first
applies_to: []string    # Peer IDs or tags
tunnels: []TunnelConfig # List of tunnel configurations
```

**Policy Validation:**

1. **Basic Validation**:
   - Required fields present
   - Valid IP addresses and subnets
   - At least one traffic selector

2. **Security Validation**:
   - PSK minimum length (8 characters)
   - Certificate paths exist (if using certs)
   - Valid crypto algorithms
   - SA lifetime constraints (5 min - 24 hours)

3. **Platform Compatibility**:
   - GCM modes require IKEv2
   - Warn about limited AH support
   - Check algorithm support per platform

## Data Flow

### Policy Distribution

```
1. Admin creates/updates policy via API or Web UI
2. Server validates policy using PolicyEngine
3. Server stores policy in SQLite database
4. Server logs audit event
5. Agent polls server every 60s (configurable)
6. Server filters policies for requesting agent (by peer ID/tags)
7. Agent receives applicable policies
8. Agent reconciles: creates/updates/deletes tunnels
9. Agent starts watchdog monitoring
```

### Tunnel Monitoring

```
1. Agent health check loop (every 10s)
2. Query IPsec manager for tunnel status
3. Collect metrics: state, bytes in/out, packets, uptime
4. Log status changes
5. If tunnel down and auto-start=true:
   a. Watchdog detects failure
   b. Attempt restart (max 3 retries)
   c. Log restart event
6. Optionally push metrics to server (future)
```

## Security Considerations

1. **Authentication**:
   - PSK: Minimum 8 characters, stored in config files with 0600 permissions
   - Certificate: Full PKI support, validate certificate chains
   - Server API: JWT tokens (future implementation)

2. **Transport Security**:
   - HTTPS for server-agent communication (recommended)
   - TLS certificate validation
   - Option to disable for testing (not recommended for production)

3. **Secret Management**:
   - PSKs never logged in plaintext
   - Configuration files protected (0600 permissions)
   - Audit logging for all policy changes
   - Consider external secret management (HashiCorp Vault, etc.)

4. **Access Control**:
   - Server API authentication (JWT - future)
   - Role-based access control (future)
   - Policy-to-peer targeting via tags

## Performance Characteristics

### Latency Impact

IPsec encryption adds minimal latency:
- **Software encryption**: ~10-50 microseconds per packet
- **Hardware offload (AES-NI)**: ~1-5 microseconds
- **Recommendation**: Use AES-GCM with IKEv2 for best performance

### Throughput

Depends on hardware and algorithms:
- **AES-128-GCM**: 10-40 Gbps (with AES-NI)
- **AES-256-CBC**: 5-20 Gbps
- **3DES**: 500-1000 Mbps (avoid if possible)

### Agent Resource Usage

- **Memory**: 20-50 MB per agent
- **CPU**: <1% idle, 2-5% during rekey
- **Network**: Minimal (policy sync every 60s, <1 KB)
- **Disk**: <100 MB (including logs)

## Scalability

- **Server capacity**: 1000+ agents per server instance
- **Agent capacity**: 100+ tunnels per agent
- **Database**: SQLite suitable for <10K policies
- **Future**: Add PostgreSQL support for larger deployments

## Deployment Patterns

### 1. Site-to-Site VPN

```
HQ (10.0.0.0/16) ←→ Branch 1 (10.1.0.0/16)
                  ←→ Branch 2 (10.2.0.0/16)
                  ←→ Branch 3 (10.3.0.0/16)
```

- Server at HQ or cloud
- Agent on each site gateway
- Multiple tunnels from HQ to each branch
- Policies target specific peer IDs

### 2. Hub-and-Spoke

```
        ┌── Spoke 1
        │
Hub ────┼── Spoke 2
        │
        └── Spoke 3
```

- Hub has policy with multiple tunnels
- Each spoke has single tunnel to hub
- Traffic between spokes routes through hub

### 3. Mesh Topology

```
Node 1 ←→ Node 2
  ↕         ↕
Node 3 ←→ Node 4
```

- Each node has tunnels to all others
- N nodes = N*(N-1)/2 tunnels
- Suitable for small clusters (<10 nodes)

## Monitoring and Observability

### Metrics Collected

- Tunnel state (up/down/connecting/error)
- Traffic statistics (bytes/packets in/out)
- Rekey events
- DPD failures
- Policy sync success/failure
- Agent uptime

### Logging

- **Structured logging**: JSON format with zerolog
- **Log levels**: debug, info, warn, error
- **Destinations**: 
  - Console (development)
  - File rotation (production)
  - Syslog (Linux/macOS)
  - Windows Event Log (Windows)

### Future: Prometheus Integration

```
ipsec_tunnel_state{name="tunnel-1"} 1
ipsec_tunnel_bytes_in{name="tunnel-1"} 1048576
ipsec_tunnel_bytes_out{name="tunnel-1"} 2097152
ipsec_rekey_events_total{name="tunnel-1"} 5
ipsec_policy_sync_errors_total 0
```

## Failure Scenarios and Recovery

1. **Agent loses connection to server**:
   - Continue using last known policies
   - Retry connection with exponential backoff
   - Log connectivity errors

2. **Tunnel fails to establish**:
   - Watchdog detects failure
   - Retry up to 3 times with 30s intervals
   - Log detailed error (key mismatch, peer unavailable, etc.)

3. **Peer unreachable**:
   - DPD detects unresponsive peer
   - Take action based on dpd.action (restart/clear/hold)
   - Update tunnel state

4. **Configuration error**:
   - Validate before applying
   - Rollback on failure
   - Keep previous working configuration
   - Log error details for troubleshooting

5. **Server database corruption**:
   - Use SQLite's WAL mode for durability
   - Implement backup/restore procedures
   - Validate database integrity on startup

## Future Enhancements

1. **Certificate Management**:
   - Integrated CA (cfssl library)
   - Automatic certificate renewal
   - PKCS#12 export for Windows

2. **Advanced Monitoring**:
   - Prometheus metrics endpoint
   - Grafana dashboards
   - Alertmanager integration

3. **High Availability**:
   - Server clustering
   - PostgreSQL backend
   - State synchronization

4. **Advanced Policies**:
   - Time-based policies (active hours)
   - Bandwidth limits
   - QoS integration

5. **Compliance**:
   - FIPS 140-2 mode
   - Audit report generation
   - Compliance policy templates

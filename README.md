# Unified Cross-Platform IPsec Manager

**SWAVLAMBAN 2025 HACKATHON - CHALLENGE 2**

A unified, cross-platform IPsec management solution that provides centralized policy enforcement and automated tunnel management across Windows, Linux, macOS, and BOSS OS.

## 🎯 Project Overview

This solution addresses the challenge of securing network traffic across heterogeneous enterprise environments. It provides:

- **Unified Policy Management**: Single source of truth for IPsec policies across all platforms
- **Automated Configuration**: Platform-specific IPsec setup (strongSwan, Windows NetIPsec, macOS VPN) from central policies
- **Hybrid Management**: CLI tools for automation + Web dashboard for monitoring
- **All IPsec Modes**: ESP/AH in tunnel/transport mode with all combinations
- **Auto-Recovery**: Automatic tunnel re-establishment and service persistence
- **Real-time Monitoring**: Live tunnel status, traffic statistics, and logging

## 🏗️ Architecture

### Components

1. **Policy Server** (`cmd/server/`)
   - REST API for policy management
   - Peer registration and inventory
   - SQLite storage for policies and audit logs
   - WebSocket server for real-time updates
   - Embedded Svelte dashboard

2. **Agent Daemon** (`cmd/agent/`)
   - Cross-platform service (systemd/Windows Service/launchd)
   - Policy synchronization from server
   - Platform-specific IPsec configuration
   - Tunnel health monitoring and recovery
   - Structured logging and metrics

3. **IPsec Abstraction Layer** (`internal/ipsec/`)
   - Unified interface for all platforms
   - Linux: strongSwan via VICI protocol (govici)
   - Windows: PowerShell NetIPsec cmdlets
   - macOS: scutil VPN management
   - BOSS: Debian-based strongSwan variant

4. **Web Dashboard** (`web/`)
   - Svelte + Vite + Tailwind CSS
   - Real-time tunnel visualization
   - Traffic graphs and statistics
   - Log viewer and search
   - Policy editor

## 🚀 Features

### Core Capabilities

- ✅ **All IPsec Modes**: ESP Tunnel, ESP Transport, AH Tunnel, AH Transport, Combined ESP+AH
- ✅ **Flexible Encryption**: AES-128/256, 3DES, Camellia
- ✅ **Integrity Algorithms**: SHA-1, SHA-256, SHA-384, SHA-512
- ✅ **Key Exchange**: IKEv1, IKEv2, DH groups 2/5/14/15/16/19/20/21
- ✅ **Authentication**: Pre-Shared Keys (PSK) and X.509 Certificates
- ✅ **Traffic Selectors**: Selective encryption by subnet, protocol, or port
- ✅ **Multi-Tunnel**: Simultaneous tunnels to multiple peers
- ✅ **Auto-Start**: Service starts on boot, survives reboots
- ✅ **Auto-Recovery**: Reconnects dropped tunnels automatically

### Monitoring & Management

- 📊 Real-time tunnel status dashboard
- 📈 Traffic statistics (bytes/packets in/out)
- 📝 Structured logging (JSON, syslog, Event Log)
- 🔔 Alerts for tunnel failures and misconfigurations
- 📉 Prometheus metrics export
- 🔍 Event log search and filtering

## 📋 Requirements

### Server Requirements

- Go 1.21+ (for building from source)
- SQLite3
- 512MB RAM minimum
- Linux, Windows, or macOS

### Agent Requirements

**Linux (Debian/Ubuntu/RHEL/Fedora/BOSS):**
- strongSwan 5.9+
- systemd (for service management)
- Root/sudo privileges for IPsec configuration

**Windows:**
- Windows 10/11 or Server 2019+
- PowerShell 5.1+
- Administrator privileges

**macOS:**
- macOS 11 (Big Sur) or later
- Command Line Tools installed
- Administrator privileges

## 🛠️ Installation

### Quick Start with Pre-built Binaries

Download the latest release for your platform from [Releases](https://github.com/swavlamban/ipsec-manager/releases).

**1. Install the Server:**

```bash
# Linux/macOS
sudo ./ipsec-server install
sudo systemctl start ipsec-server

# Windows (PowerShell as Administrator)
.\ipsec-server.exe install
Start-Service ipsec-server
```

**2. Install the Agent on each node:**

```bash
# Linux
sudo ./ipsec-agent install --server https://your-server:8443
sudo systemctl start ipsec-agent

# Windows (PowerShell as Administrator)
.\ipsec-agent.exe install --server https://your-server:8443
Start-Service ipsec-agent

# macOS
sudo ./ipsec-agent install --server https://your-server:8443
sudo launchctl load /Library/LaunchDaemons/com.swavlamban.ipsec-agent.plist
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/swavlamban/ipsec-manager.git
cd ipsec-manager

# Install dependencies
go mod download

# Build all components
make build

# Or build specific components
make build-server  # Builds cmd/server
make build-agent   # Builds cmd/agent

# Build for specific platform
GOOS=linux GOARCH=amd64 make build-agent
GOOS=windows GOARCH=amd64 make build-agent
GOOS=darwin GOARCH=arm64 make build-agent
```

### Platform-Specific Setup

**Linux (strongSwan):**

```bash
# Install strongSwan
sudo apt install strongswan strongswan-swanctl  # Debian/Ubuntu
sudo yum install strongswan                      # RHEL/Fedora

# Enable IP forwarding (if acting as gateway)
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
```

**Windows:**

```powershell
# Enable IPsec services
Set-Service -Name "IKEEXT" -StartupType Automatic
Start-Service IKEEXT
```

**macOS:**

```bash
# Install Command Line Tools if not present
xcode-select --install
```

## 📖 Usage

### Server Management

```bash
# Start server
ipsec-server start

# Access web dashboard
open http://localhost:8080

# View logs
ipsec-server logs

# Create a policy
ipsec-server policy create --file policy.yaml
```

### Agent Management

```bash
# Check agent status
ipsec-agent status

# View active tunnels
ipsec-agent tunnels list

# Force policy sync
ipsec-agent sync

# View logs
ipsec-agent logs --tail 50

# Restart specific tunnel
ipsec-agent tunnel restart tunnel-name
```

### Policy Configuration

Create a policy file `policy.yaml`:

```yaml
policies:
  - name: site-to-site-hq-branch
    mode: esp-tunnel
    local:
      address: 10.0.1.1
      subnet: 10.0.1.0/24
      id: hq@company.com
    remote:
      address: 10.0.2.1
      subnet: 10.0.2.0/24
      id: branch@company.com
    crypto:
      encryption: aes256
      integrity: sha256
      dhgroup: modp2048
      ikeversion: 2
    auth:
      type: psk
      secret: "SuperSecretKey123!"
    options:
      autostart: true
      dpdaction: restart
      dpddelay: 30s
```

Apply the policy:

```bash
ipsec-server policy apply -f policy.yaml
```

## 🧪 Testing

### Run Unit Tests

```bash
go test ./...
```

### Run Integration Tests

```bash
# Start test environment with Docker
cd test/integration
docker-compose up -d

# Run tests
go test -tags=integration ./test/integration/...

# Cleanup
docker-compose down
```

### Performance Testing

```bash
# Latency benchmark
./scripts/benchmark-latency.sh

# Throughput test
./scripts/benchmark-throughput.sh
```

## 📚 Documentation

- [Architecture Guide](docs/architecture.md)
- [API Reference](docs/api.md)
- [Policy Configuration Guide](docs/policies.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Development Guide](docs/development.md)

## 🎥 Demo Video

[Demo Video Link - 5 minutes showing installation, configuration, and multi-platform operation]

## 📊 Project Structure

```
ipsec-manager/
├── cmd/
│   ├── server/          # Policy management server
│   └── agent/           # Agent daemon
├── internal/
│   ├── ipsec/           # Platform abstraction layer
│   ├── policy/          # Policy engine
│   ├── monitor/         # Monitoring and metrics
│   ├── server/          # Server implementation
│   └── agent/           # Agent implementation
├── web/                 # Svelte dashboard
│   ├── src/
│   └── dist/            # Built assets (embedded)
├── configs/             # Configuration templates
├── test/
│   ├── integration/     # Integration tests
│   └── docker/          # Test containers
├── docs/                # Documentation
├── scripts/             # Build and utility scripts
└── deployments/         # Deployment configurations
```

## 🔒 Security Considerations

- All policies stored with encryption at rest
- JWT authentication for API access
- TLS for server-agent communication
- Audit logging for all policy changes
- PSK/certificates never logged in plaintext
- File permissions enforced (0600 for secrets)

## 🤝 Contributing

This project was developed for the SWAVLAMBAN 2025 Hackathon. 

## 📄 License

[To be determined based on hackathon requirements]

## 👥 Team

[Team information]

## 🏆 Hackathon Challenge

This project addresses **Challenge 2: Development of a Unified Cross-Platform IPsec Solution** from SWAVLAMBAN 2025.

### Challenge Compliance

✅ Unified cross-platform solution (Windows, Linux, macOS, BOSS)  
✅ All IPsec modes supported  
✅ Complete or selective traffic encryption  
✅ Minimal latency impact  
✅ Flexible cryptographic controls  
✅ Persistent and automated operation  
✅ Automation and interoperability  
✅ Multi-tunnel capacity  
✅ Monitoring and logs  
✅ Error handling  
✅ Network device operability  

### Bonus Features

✅ Tunnel visualization  
✅ Real-time traffic monitoring  
✅ Event logs and alerts  
✅ Configuration dashboard  

## 📞 Support

For issues and questions during the hackathon evaluation, please refer to the technical documentation or create an issue in the repository.

---

**Built with ❤️ for SWAVLAMBAN 2025**

# Quick Start Guide

## Prerequisites

### Server Requirements
- Operating System: Linux, Windows, or macOS
- Go 1.21+ (if building from source)
- 512 MB RAM minimum
- 1 GB disk space

### Agent Requirements

**Linux (Ubuntu/Debian/BOSS):**
```bash
sudo apt-get update
sudo apt-get install strongswan strongswan-swanctl
```

**Linux (RHEL/Rocky/Fedora):**
```bash
sudo yum install strongswan
```

**Windows:**
- Windows 10/11 or Server 2019+
- PowerShell 5.1+ (pre-installed)
- Administrator privileges

**macOS:**
- macOS 11 (Big Sur) or later
- Command Line Tools: `xcode-select --install`

## Installation

### Option 1: Pre-built Binaries (Recommended)

1. Download the latest release from [Releases](https://github.com/swavlamban/ipsec-manager/releases)

2. **Install Server:**

```bash
# Linux
tar -xzf ipsec-manager-server_v0.1.0_linux_amd64.tar.gz
sudo mv ipsec-server /usr/local/bin/
sudo mkdir -p /etc/ipsec-server /var/lib/ipsec-server

# Windows (PowerShell)
Expand-Archive ipsec-manager-server_v0.1.0_windows_amd64.zip
Move-Item ipsec-server.exe C:\Program Files\IPsec-Manager\
New-Item -ItemType Directory -Path "C:\ProgramData\IPsec-Manager"

# macOS
tar -xzf ipsec-manager-server_v0.1.0_darwin_amd64.tar.gz
sudo mv ipsec-server /usr/local/bin/
sudo mkdir -p /etc/ipsec-server /var/lib/ipsec-server
```

3. **Install Agent:**

```bash
# Linux
tar -xzf ipsec-manager-agent_v0.1.0_linux_amd64.tar.gz
sudo mv ipsec-agent /usr/local/bin/
sudo mkdir -p /etc/ipsec-agent

# Windows (PowerShell)
Expand-Archive ipsec-manager-agent_v0.1.0_windows_amd64.zip
Move-Item ipsec-agent.exe C:\Program Files\IPsec-Manager\
New-Item -ItemType Directory -Path "C:\ProgramData\IPsec-Manager\agent"

# macOS
tar -xzf ipsec-manager-agent_v0.1.0_darwin_amd64.tar.gz
sudo mv ipsec-agent /usr/local/bin/
sudo mkdir -p /etc/ipsec-agent
```

### Option 2: Build from Source

```bash
# Clone repository
git clone https://github.com/swavlamban/ipsec-manager.git
cd ipsec-manager

# Install dependencies
go mod download

# Build server and agent
make build

# Binaries will be in bin/
ls bin/
# ipsec-server  ipsec-agent
```

## Configuration

### 1. Configure Server

Create `/etc/ipsec-server/config.yaml`:

```yaml
server:
  listen: ":8080"
  db_path: "/var/lib/ipsec-server/ipsec.db"

log:
  level: "info"
```

### 2. Start Server

```bash
# Linux - foreground
sudo ipsec-server start

# Linux - as service
sudo ipsec-server install
sudo systemctl start ipsec-server
sudo systemctl enable ipsec-server

# View logs
sudo journalctl -u ipsec-server -f

# Windows (PowerShell as Administrator)
.\ipsec-server.exe install
Start-Service ipsec-server
Set-Service ipsec-server -StartupType Automatic

# macOS
sudo ipsec-server install
sudo launchctl load /Library/LaunchDaemons/com.swavlamban.ipsec-server.plist
```

Access web dashboard: `http://localhost:8080`

### 3. Configure Agent

Create `/etc/ipsec-agent/config.yaml`:

```yaml
server:
  url: "http://YOUR_SERVER_IP:8080"

agent:
  sync_interval: "60s"

peer:
  tags:
    - "production"

log:
  level: "info"
```

### 4. Install and Start Agent

```bash
# Linux
sudo ipsec-agent install --server http://SERVER_IP:8080
sudo systemctl start ipsec-agent
sudo systemctl enable ipsec-agent

# Check status
sudo ipsec-agent status

# View logs
sudo journalctl -u ipsec-agent -f

# Windows (PowerShell as Administrator)
.\ipsec-agent.exe install --server http://SERVER_IP:8080
Start-Service ipsec-agent
Set-Service ipsec-agent -StartupType Automatic

# Check status
.\ipsec-agent.exe status

# macOS
sudo ipsec-agent install --server http://SERVER_IP:8080
sudo launchctl load /Library/LaunchDaemons/com.swavlamban.ipsec-agent.plist

# Check status
sudo ipsec-agent status
```

## Create Your First Tunnel

### Method 1: Web Dashboard

1. Open `http://SERVER_IP:8080` in your browser
2. Click "Policies" → "Create Policy"
3. Fill in the form:
   - Name: `my-first-tunnel`
   - Mode: `ESP Tunnel`
   - Local Address: Your local IP
   - Remote Address: Remote peer IP
   - Encryption: `AES-256`
   - Auth Type: `PSK`
   - PSK Secret: Strong password
4. Click "Create"

### Method 2: API

```bash
curl -X POST http://SERVER_IP:8080/api/policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-first-tunnel",
    "enabled": true,
    "tunnels": [{
      "name": "site-to-site",
      "mode": "esp-tunnel",
      "local_address": "203.0.113.10",
      "remote_address": "203.0.113.20",
      "crypto": {
        "encryption": "aes256",
        "integrity": "sha256",
        "dhgroup": "modp2048",
        "ikeversion": "ikev2",
        "lifetime": "3600s"
      },
      "auth": {
        "type": "psk",
        "secret": "YourStrongPasswordHere"
      },
      "traffic_selectors": [{
        "local_subnet": "10.0.1.0/24",
        "remote_subnet": "10.0.2.0/24"
      }],
      "dpd": {
        "delay": "30s",
        "action": "restart"
      },
      "autostart": true
    }]
  }'
```

### Method 3: YAML Policy File

Create `my-policy.yaml`:

```yaml
name: "my-first-tunnel"
enabled: true
priority: 100

tunnels:
  - name: "site-to-site"
    mode: "esp-tunnel"
    local_address: "203.0.113.10"
    remote_address: "203.0.113.20"
    crypto:
      encryption: "aes256"
      integrity: "sha256"
      dhgroup: "modp2048"
      ikeversion: "ikev2"
      lifetime: "3600s"
    auth:
      type: "psk"
      secret: "YourStrongPasswordHere"
    traffic_selectors:
      - local_subnet: "10.0.1.0/24"
        remote_subnet: "10.0.2.0/24"
    dpd:
      delay: "30s"
      action: "restart"
    autostart: true
```

Apply it:

```bash
curl -X POST http://SERVER_IP:8080/api/policies \
  -H "Content-Type: application/json" \
  -d @my-policy.yaml
```

## Verification

### 1. Check Agent Status

```bash
# Linux/macOS
sudo ipsec-agent status

# Should show:
# IPsec Agent Status
# ==================
# Version:  v0.1.0
# Platform: linux
# Tunnels:  1
#
# Tunnel Status:
# --------------
#   site-to-site          State: established  In: 1048576 bytes  Out: 2097152 bytes
```

### 2. Check Tunnel on Linux (strongSwan)

```bash
sudo swanctl --list-sas

# Should show active SAs
```

### 3. Check Tunnel on Windows

```powershell
Get-NetIPsecQuickModeSA

# Should show active SAs
```

### 4. Test Connectivity

```bash
# Ping remote subnet through tunnel
ping 10.0.2.1

# Check traffic is encrypted
sudo tcpdump -i eth0 esp
# Should see ESP packets
```

## Common Issues

### Tunnel Won't Establish

1. **Check logs:**
   ```bash
   sudo journalctl -u ipsec-agent -f
   ```

2. **Verify connectivity:**
   ```bash
   ping REMOTE_IP
   telnet REMOTE_IP 500  # IKE
   telnet REMOTE_IP 4500 # NAT-T
   ```

3. **Check PSK matches** on both sides

4. **Firewall rules:**
   ```bash
   # Linux
   sudo iptables -I INPUT -p udp --dport 500 -j ACCEPT
   sudo iptables -I INPUT -p udp --dport 4500 -j ACCEPT
   sudo iptables -I INPUT -p esp -j ACCEPT
   
   # Windows
   New-NetFirewallRule -DisplayName "IKE" -Direction Inbound -Protocol UDP -LocalPort 500 -Action Allow
   New-NetFirewallRule -DisplayName "NAT-T" -Direction Inbound -Protocol UDP -LocalPort 4500 -Action Allow
   ```

### Agent Can't Connect to Server

1. **Check server is running:**
   ```bash
   curl http://SERVER_IP:8080/api/health
   ```

2. **Check agent config:**
   ```bash
   cat /etc/ipsec-agent/config.yaml
   ```

3. **Test network connectivity:**
   ```bash
   ping SERVER_IP
   ```

### Permission Denied Errors

Agents need root/administrator privileges:

```bash
# Linux - run with sudo
sudo ipsec-agent start

# Windows - run as Administrator
Right-click → "Run as Administrator"
```

## Next Steps

- Read the [User Manual](USER_MANUAL.md) for detailed features
- Check [ARCHITECTURE.md](ARCHITECTURE.md) for technical details
- See [example-policies.yaml](../configs/example-policies.yaml) for more examples
- Join discussions on GitHub for support

## Quick Reference

### Server Commands

```bash
ipsec-server start              # Start in foreground
ipsec-server install            # Install as service
ipsec-server uninstall          # Remove service
```

### Agent Commands

```bash
ipsec-agent start               # Start in foreground
ipsec-agent install             # Install as service
ipsec-agent uninstall           # Remove service
ipsec-agent status              # Show status
ipsec-agent tunnels list        # List tunnels
ipsec-agent sync                # Force policy sync
```

### API Endpoints

```
GET    /api/health              # Health check
GET    /api/policies            # List policies
POST   /api/policies            # Create policy
GET    /api/policies/:id        # Get policy
PUT    /api/policies/:id        # Update policy
DELETE /api/policies/:id        # Delete policy
GET    /api/peers               # List peers
POST   /api/peers/register      # Register peer
GET    /api/tunnels             # List all tunnels
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/swavlamban/ipsec-manager/issues
- Documentation: https://github.com/swavlamban/ipsec-manager/docs

---

**SWAVLAMBAN 2025 - Unified Cross-Platform IPsec Solution**

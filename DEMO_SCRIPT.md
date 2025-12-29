# Demo Script - SWAVLAMBAN 2025 Hackathon

## Unified Cross-Platform IPsec Management Solution

**Duration:** 5 minutes  
**Date:** December 29, 2025

---

## Introduction (30 seconds)

**Script:**
> "Hello! I'm presenting our solution to Challenge 2: Unified Cross-Platform IPsec Management for securing data in transit. Our system provides centralized policy management with distributed agent deployment across Linux, Windows, and macOS platforms, using Go for performance and strongSwan for industry-standard encryption."

**Show:** README.md title and architecture diagram

---

## Problem Statement (20 seconds)

**Script:**
> "Enterprise networks face a critical challenge: managing IPsec VPNs across heterogeneous environments. Different operating systems have different IPsec implementations - Linux uses strongSwan, Windows has NetIPsec, macOS has native VPN. Manually configuring each system is error-prone and doesn't scale."

**Show:** Architecture diagram highlighting multiple platforms

---

## Solution Overview (30 seconds)

**Script:**
> "Our solution provides:
> - A central REST API server for policy management
> - Platform-specific agents that auto-configure IPsec
> - Real-time web dashboard for monitoring
> - Docker deployment for easy testing
> - All built with Go, using open-source encryption"

**Show:** System architecture diagram from TEST_RESULTS.md

---

## Live Demo Part 1: Deployment (60 seconds)

**Script:**
> "Let's see it in action. First, I'll deploy the entire system with one command."

**Commands:**
```powershell
# Show docker-compose.yml briefly
cat docker-compose.yml

# Start all services
docker-compose up -d

# Verify all containers running
docker-compose ps
```

**Expected Output:**
```
NAME                  STATUS    PORTS
ipsec-server          Up        0.0.0.0:8080->8080/tcp
ipsec-agent-linux     Up
ipsec-agent-linux-2   Up
```

**Script:**
> "Three containers started: one central server and two Linux agents. The server is listening on port 8080."

---

## Live Demo Part 2: Web Dashboard (30 seconds)

**Script:**
> "Let's check the web dashboard."

**Actions:**
1. Open browser to http://localhost:8080
2. Show the dashboard interface
3. Navigate to "Peers" section
4. Show 2 registered agents (both online, platform=linux)

**Script:**
> "The dashboard shows our two agents have automatically registered. Both are online and syncing policies every 60 seconds."

---

## Live Demo Part 3: Policy Creation (60 seconds)

**Script:**
> "Now I'll create an IPsec policy via the REST API."

**Commands:**
```powershell
# Show the policy file
cat minimal-policy.json

# Create policy
$body = Get-Content minimal-policy.json -Raw
Invoke-RestMethod -Uri http://localhost:8080/api/policies `
    -Method POST `
    -Body $body `
    -ContentType "application/json" | ConvertTo-Json -Depth 5
```

**Expected Output:**
```json
{
  "id": "test-policy-1",
  "name": "Test Policy",
  "enabled": true,
  "tunnels": [...]
}
```

**Script:**
> "Policy created successfully! This defines an IPsec tunnel with AES-256 encryption, SHA-256 integrity, and IKEv2. The policy is now stored in the server's database."

---

## Live Demo Part 4: Agent Sync (45 seconds)

**Script:**
> "The agents automatically sync policies every 60 seconds. Let's check the logs."

**Commands:**
```powershell
# Check agent logs
docker-compose logs agent-linux | Select-Object -Last 10
```

**Expected Output:**
```
ipsec-agent-linux  | 2025-12-29T15:00:33Z INF Fetched policies count=1
ipsec-agent-linux  | 2025-12-29T15:00:33Z INF Tunnel created successfully tunnel=test-tunnel-1
```

**Script:**
> "Perfect! The agent fetched the policy and created the tunnel. Let's verify strongSwan configuration was generated."

---

## Live Demo Part 5: Tunnel Configuration (45 seconds)

**Script:**
> "Let's look at the generated strongSwan configuration."

**Commands:**
```powershell
# Show generated config
docker exec ipsec-agent-linux cat /etc/swanctl/conf.d/test-tunnel-1.conf
```

**Expected Output:**
```
connections {
    test-tunnel-1 {
        version = 2
        local_addrs = 172.20.0.10
        remote_addrs = 172.20.0.20
        ...
        children {
            test-tunnel-1-child {
                esp_proposals = aes256-sha256-modp2048
                ...
            }
        }
    }
}
```

**Script:**
> "The agent automatically generated the strongSwan configuration from our JSON policy. Notice the encryption suite: AES-256, SHA-256, modp2048 Diffie-Hellman group - exactly as specified."

---

## Live Demo Part 6: Tunnel Status (30 seconds)

**Script:**
> "Finally, let's check the tunnel status."

**Commands:**
```powershell
# Check tunnel status
docker exec ipsec-agent-linux swanctl --list-sas
```

**Expected Output:**
```
test-tunnel-1: #1, CONNECTING, IKEv2
  local  '%any' @ 172.20.0.10[500]
  remote '%any' @ 172.20.0.20[500]
```

**Script:**
> "The tunnel is in CONNECTING state, which is expected since we don't have a real remote endpoint. But this proves the complete workflow: policy creation, distribution, configuration generation, and tunnel initiation - all automated!"

---

## Technical Highlights (30 seconds)

**Script:**
> "Key technical achievements:
> - 6000+ lines of production-ready Go code
> - Cross-platform abstraction with interface-based architecture
> - Platform-specific implementations for Linux, Windows, and macOS
> - Multi-stage Docker builds resulting in 24.7 MB server image
> - Real-time policy synchronization with audit logging
> - Sub-second API response times"

**Show:** Project structure from README.md

---

## Conclusion (20 seconds)

**Script:**
> "To summarize: we've built a complete IPsec management solution that:
> - Centralizes policy management
> - Automates platform-specific configuration
> - Provides real-time monitoring
> - Deploys easily with Docker
> - Uses open-source encryption
> 
> Thank you! Questions?"

---

## Backup Slides/Commands

### If time permits, show:

1. **API Endpoints:**
   ```powershell
   # List all policies
   Invoke-RestMethod http://localhost:8080/api/policies | ConvertTo-Json
   
   # List all peers
   Invoke-RestMethod http://localhost:8080/api/peers | ConvertTo-Json
   ```

2. **Process Status:**
   ```powershell
   # Show strongSwan daemon running
   docker exec ipsec-agent-linux ps aux | Select-String "charon"
   ```

3. **Configuration Files:**
   ```powershell
   # Show server config
   cat configs/server-config.yaml
   
   # Show agent config
   cat configs/agent-config.yaml
   ```

---

## Troubleshooting Q&A

**Q: What if tunnel doesn't connect?**
> A: In production, both endpoints need proper network connectivity and matching configurations. Our demo shows the automated configuration generation and tunnel initiation, which is the core challenge.

**Q: How does this work on Windows?**
> A: The Windows agent uses PowerShell cmdlets (New-NetIPsecRule, New-NetIPsecMainModeCryptoSet) to configure Windows' built-in NetIPsec. The same JSON policy generates platform-specific commands.

**Q: What about certificate authentication?**
> A: Currently using PSK for demo simplicity. The architecture supports X.509 certificates - just change auth.type to "cert" and provide certificate paths in the policy.

**Q: How does this scale?**
> A: Horizontally - add more agents as needed. Server can handle thousands of agents. For very large deployments, server can be clustered with load balancer and shared database.

**Q: Security concerns?**
> A: Production deployment should:
> - Enable TLS for REST API
> - Use certificate-based authentication
> - Store secrets in vault (HashiCorp Vault, Azure Key Vault)
> - Enable audit logging
> - Implement RBAC

**Q: What IPsec modes are supported?**
> A: All modes:
> - ESP tunnel (most common, shown in demo)
> - ESP transport
> - AH tunnel  
> - AH transport
> - Configured via policy "mode" field

---

## Demo Checklist

Before presentation:

- [ ] All Docker containers running (`docker-compose ps`)
- [ ] Dashboard accessible at http://localhost:8080
- [ ] Both agents registered (`/api/peers`)
- [ ] No existing policies (`/api/policies` returns empty array)
- [ ] `minimal-policy.json` file ready
- [ ] Browser windows pre-opened
- [ ] PowerShell terminal ready
- [ ] Commands copied to clipboard

After demo:

- [ ] Stop containers: `docker-compose down`
- [ ] Clean up: `docker system prune`

---

## Time Management

| Section | Duration | Cumulative |
|---------|----------|------------|
| Introduction | 30s | 0:30 |
| Problem Statement | 20s | 0:50 |
| Solution Overview | 30s | 1:20 |
| Demo 1: Deployment | 60s | 2:20 |
| Demo 2: Dashboard | 30s | 2:50 |
| Demo 3: Policy Creation | 60s | 3:50 |
| Demo 4: Agent Sync | 45s | 4:35 |
| Demo 5: Configuration | 45s | 5:20 |
| Demo 6: Tunnel Status | 30s | 5:50 |
| Technical Highlights | 30s | 6:20 |
| Conclusion | 20s | 6:40 |
| Buffer | 20s | 7:00 |

Target: 5-7 minutes with buffer for questions.

---

## Presentation Tips

1. **Speak clearly and confidently**
2. **Explain technical terms briefly** (IKEv2, ESP, AES-256)
3. **Show enthusiasm** for the solution
4. **Highlight automation** - key value proposition
5. **Handle errors gracefully** - have backup screenshots
6. **Engage judges** - make eye contact
7. **Stay within time limit** - practice beforehand

---

## Video Recording Script

For submission video:

1. **Title slide** (3 seconds)
2. **Introduction voiceover** with architecture diagram (30s)
3. **Screen recording** of terminal and browser (4 minutes)
4. **Technical highlights** with code snippets (30s)
5. **Conclusion** with contact info (20s)

Total: ~5 minutes

Recording software: OBS Studio, Camtasia, or Loom
Resolution: 1920x1080
Frame rate: 30 FPS
Audio: Clear microphone, no background noise

---

**Good luck! 🚀**

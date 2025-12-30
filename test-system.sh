#!/bin/bash
# IPsec Manager System Test Suite (Bash version for Linux/macOS)
# Tests the complete end-to-end functionality

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Flags
SKIP_BUILD=false
SKIP_DOCKER=false
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-docker)
            SKIP_DOCKER=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Helper functions
success() {
    echo -e "${GREEN}✓ $*${NC}"
}

failure() {
    echo -e "${RED}✗ $*${NC}"
}

info() {
    echo -e "${CYAN}ℹ $*${NC}"
}

test_header() {
    echo -e "${YELLOW}→ $*${NC}"
}

test_assertion() {
    local test_name="$1"
    shift
    
    test_header "Testing: $test_name"
    
    if output=$("$@" 2>&1); then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        success "$test_name - PASSED"
        if [ "$VERBOSE" = true ]; then
            echo "$output"
        fi
        echo "$output"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        failure "$test_name - FAILED"
        echo "$output"
        return 1
    fi
}

wait_for_service() {
    local url=$1
    local max_attempts=${2:-30}
    local delay=${3:-2}
    
    info "Waiting for service at $url..."
    for ((i=0; i<max_attempts; i++)); do
        if curl -sf "$url" > /dev/null 2>&1; then
            success "Service is ready"
            return 0
        fi
        sleep "$delay"
    done
    
    failure "Service did not become ready after $max_attempts attempts"
    return 1
}

cat << "EOF"
╔═══════════════════════════════════════════════════════════╗
║  IPsec Manager - Automated Test Suite                    ║
║  SWAVLAMBAN 2025 Hackathon - Challenge 2                 ║
╚═══════════════════════════════════════════════════════════╝

EOF

# Test 1: Environment Check
echo -e "\n${MAGENTA}═══ Phase 1: Environment Checks ═══${NC}\n"

test_assertion "Go is installed" bash -c "go version"
test_assertion "Docker is installed and running" bash -c "docker --version && docker ps > /dev/null"
test_assertion "Docker Compose is available" bash -c "docker-compose --version"
test_assertion "Project structure is valid" bash -c '
    for file in go.mod cmd/server/main.go cmd/agent/main.go internal/ipsec/manager.go docker-compose.yml; do
        [ -f "$file" ] || exit 1
    done
    echo "All required files present"
'

# Test 2: Build Tests
if [ "$SKIP_BUILD" = false ]; then
    echo -e "\n${MAGENTA}═══ Phase 2: Build Tests ═══${NC}\n"
    
    test_assertion "Go modules download" bash -c "go mod download"
    test_assertion "Server builds successfully" bash -c "CGO_ENABLED=1 go build -o ipsec-server ./cmd/server && [ -f ipsec-server ]"
    test_assertion "Agent builds successfully" bash -c "CGO_ENABLED=0 go build -o ipsec-agent ./cmd/agent && [ -f ipsec-agent ]"
    test_assertion "Unit tests pass" bash -c "go test ./internal/... -v -short"
fi

# Test 3: Docker Tests
if [ "$SKIP_DOCKER" = false ]; then
    echo -e "\n${MAGENTA}═══ Phase 3: Docker Deployment Tests ═══${NC}\n"
    
    test_assertion "Clean up existing containers" bash -c "docker-compose down -v 2>/dev/null || true"
    test_assertion "Docker images build" bash -c "docker-compose build"
    test_assertion "Containers start successfully" bash -c "docker-compose up -d && sleep 5"
    test_assertion "All containers are running" bash -c '
        running=$(docker-compose ps --format json | jq -r ".State" | grep -c "running" || true)
        [ "$running" -ge 3 ] || exit 1
        echo "3 containers running"
    '
    test_assertion "Server health endpoint responds" bash -c "sleep 10 && curl -sf http://localhost:8080/api/health | jq -e '.status == \"ok\"'"
fi

# Test 4: API Tests
echo -e "\n${MAGENTA}═══ Phase 4: REST API Tests ═══${NC}\n"

test_assertion "GET /api/health returns 200" bash -c "curl -sf -o /dev/null -w '%{http_code}' http://localhost:8080/api/health | grep -q 200"

test_assertion "GET /api/peers lists registered agents" bash -c '
    sleep 10
    peers=$(curl -sf http://localhost:8080/api/peers | jq "length")
    [ "$peers" -ge 2 ] || exit 1
    echo "Registered peers: $peers"
'

test_assertion "POST /api/policies creates new policy" bash -c '
    POLICY_ID="test-policy-$RANDOM"
    cat > /tmp/test-policy.json << JSON
{
  "id": "$POLICY_ID",
  "name": "Automated Test Policy",
  "description": "Created by test script",
  "version": 1,
  "enabled": true,
  "priority": 100,
  "applies_to": ["docker-linux-agent"],
  "tunnels": [{
    "name": "test-tunnel-auto",
    "mode": "esp-tunnel",
    "local_address": "172.20.0.10",
    "remote_address": "172.20.0.20",
    "crypto": {
      "encryption": "aes256",
      "integrity": "sha256",
      "dhgroup": "modp2048",
      "ikeversion": "ikev2",
      "lifetime": 3600000000000
    },
    "auth": {
      "type": "psk",
      "secret": "test-secret-$RANDOM"
    },
    "traffic_selectors": [{
      "local_subnet": "10.10.0.0/24",
      "remote_subnet": "10.20.0.0/24"
    }],
    "dpd": {
      "delay": 30000000000,
      "action": "restart"
    },
    "autostart": true
  }]
}
JSON
    response=$(curl -sf -X POST http://localhost:8080/api/policies \
        -H "Content-Type: application/json" \
        -d @/tmp/test-policy.json)
    echo "$response" | jq -e ".id" > /dev/null
    echo "$response" | jq -r ".id" > /tmp/test-policy-id.txt
    echo "Policy created: $(cat /tmp/test-policy-id.txt)"
'

# Test 5: Agent Integration Tests
echo -e "\n${MAGENTA}═══ Phase 5: Agent Integration Tests ═══${NC}\n"

test_assertion "Agent syncs policies" bash -c '
    info "Waiting 70 seconds for agent sync cycle..."
    sleep 70
    docker-compose logs agent-linux 2>&1 | grep -q "Fetched policies"
'

test_assertion "Agent creates tunnel configuration" bash -c '
    docker exec ipsec-agent-linux find /etc/swanctl/conf.d/ -name "*.conf" | grep -q ".conf"
'

test_assertion "strongSwan daemon is running" bash -c '
    docker exec ipsec-agent-linux ps aux | grep -q "[c]haron"
'

test_assertion "VICI socket exists" bash -c '
    docker exec ipsec-agent-linux test -S /var/run/charon.vici
'

test_assertion "Tunnel is initiated" bash -c '
    docker exec ipsec-agent-linux swanctl --list-sas | grep -q "test-tunnel"
'

# Test 6: Cleanup
echo -e "\n${MAGENTA}═══ Phase 6: Cleanup & Verification ═══${NC}\n"

if [ -f /tmp/test-policy-id.txt ]; then
    test_assertion "DELETE /api/policies/:id removes policy" bash -c '
        POLICY_ID=$(cat /tmp/test-policy-id.txt)
        curl -sf -X DELETE "http://localhost:8080/api/policies/$POLICY_ID" > /dev/null
        ! curl -sf "http://localhost:8080/api/policies/$POLICY_ID" > /dev/null 2>&1
    '
fi

if [ "$SKIP_DOCKER" = false ]; then
    test_assertion "Stop containers gracefully" bash -c "docker-compose stop"
fi

# Test Results Summary
cat << EOF

╔═══════════════════════════════════════════════════════════╗
║              TEST RESULTS SUMMARY                         ║
╚═══════════════════════════════════════════════════════════╝

EOF

TOTAL=$((TESTS_PASSED + TESTS_FAILED))
if [ $TOTAL -gt 0 ]; then
    PASS_RATE=$(echo "scale=2; ($TESTS_PASSED * 100) / $TOTAL" | bc)
else
    PASS_RATE=0
fi

echo "Total Tests:   $TOTAL"
echo -e "Passed:        ${GREEN}$TESTS_PASSED${NC}"
if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "Failed:        ${GREEN}$TESTS_FAILED${NC}"
else
    echo -e "Failed:        ${RED}$TESTS_FAILED${NC}"
fi
echo "Pass Rate:     $PASS_RATE%"

# Generate JSON report
REPORT_FILE="test-results-$(date +%Y%m%d-%H%M%S).json"
cat > "$REPORT_FILE" << JSON
{
  "timestamp": "$(date -Iseconds)",
  "total": $TOTAL,
  "passed": $TESTS_PASSED,
  "failed": $TESTS_FAILED,
  "pass_rate": $PASS_RATE
}
JSON

info "Test report saved to: $REPORT_FILE"

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "\n${RED}❌ TESTS FAILED${NC}"
    exit 1
else
    echo -e "\n${GREEN}✅ ALL TESTS PASSED${NC}"
    exit 0
fi

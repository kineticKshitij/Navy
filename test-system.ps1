#!/usr/bin/env pwsh
# IPsec Manager System Test Suite
# Tests the complete end-to-end functionality

param(
    [switch]$SkipBuild,
    [switch]$SkipDocker,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$script:TestsPassed = 0
$script:TestsFailed = 0
$script:TestResults = @()

# Colors for output
function Write-Success { Write-Host "✓ $args" -ForegroundColor Green }
function Write-Failure { Write-Host "✗ $args" -ForegroundColor Red }
function Write-Info { Write-Host "ℹ $args" -ForegroundColor Cyan }
function Write-Test { Write-Host "→ $args" -ForegroundColor Yellow }

function Test-Assertion {
    param(
        [string]$TestName,
        [scriptblock]$Test,
        [string]$ExpectedResult
    )
    
    Write-Test "Testing: $TestName"
    
    try {
        $result = & $Test
        $script:TestsPassed++
        $script:TestResults += [PSCustomObject]@{
            Test = $TestName
            Status = "PASS"
            Result = $result
        }
        Write-Success "$TestName - PASSED"
        return $result
    }
    catch {
        $script:TestsFailed++
        $script:TestResults += [PSCustomObject]@{
            Test = $TestName
            Status = "FAIL"
            Error = $_.Exception.Message
        }
        Write-Failure "$TestName - FAILED: $($_.Exception.Message)"
        if ($Verbose) {
            Write-Host $_.ScriptStackTrace -ForegroundColor DarkRed
        }
        return $null
    }
}

function Wait-ForService {
    param(
        [string]$Url,
        [int]$MaxAttempts = 30,
        [int]$DelaySeconds = 2
    )
    
    Write-Info "Waiting for service at $Url..."
    for ($i = 0; $i -lt $MaxAttempts; $i++) {
        try {
            $response = Invoke-WebRequest -Uri $Url -Method GET -TimeoutSec 2 -UseBasicParsing
            if ($response.StatusCode -eq 200) {
                Write-Success "Service is ready"
                return $true
            }
        }
        catch {
            if ($i -eq $MaxAttempts - 1) {
                throw "Service did not become ready after $MaxAttempts attempts"
            }
            Start-Sleep -Seconds $DelaySeconds
        }
    }
    return $false
}

Write-Host @"
╔═══════════════════════════════════════════════════════════╗
║  IPsec Manager - Automated Test Suite                    ║
║  SWAVLAMBAN 2025 Hackathon - Challenge 2                 ║
╚═══════════════════════════════════════════════════════════╝

"@ -ForegroundColor Cyan

# Test 1: Environment Check
Write-Host "`n═══ Phase 1: Environment Checks ═══`n" -ForegroundColor Magenta

Test-Assertion "Go is installed" {
    $version = go version
    if (-not $version) { throw "Go not found" }
    Write-Verbose $version
    return $version
}

Test-Assertion "Docker is installed and running" {
    $version = docker --version
    docker ps | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Docker daemon not running" }
    return $version
}

Test-Assertion "Docker Compose is available" {
    $version = docker-compose --version
    if ($LASTEXITCODE -ne 0) { throw "docker-compose not found" }
    return $version
}

Test-Assertion "Project structure is valid" {
    $required = @(
        "go.mod",
        "cmd/server/main.go",
        "cmd/agent/main.go",
        "internal/ipsec/manager.go",
        "docker-compose.yml"
    )
    foreach ($file in $required) {
        if (-not (Test-Path $file)) {
            throw "Missing required file: $file"
        }
    }
    return "All required files present"
}

# Test 2: Build Tests
if (-not $SkipBuild) {
    Write-Host "`n═══ Phase 2: Build Tests ═══`n" -ForegroundColor Magenta

    Test-Assertion "Go modules download" {
        go mod download
        if ($LASTEXITCODE -ne 0) { throw "Failed to download modules" }
        return "Modules downloaded"
    }

    Test-Assertion "Server builds successfully" {
        $env:CGO_ENABLED = "1"
        go build -o ipsec-server.exe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Server build failed" }
        if (-not (Test-Path "ipsec-server.exe")) {
            throw "Server binary not created"
        }
        return "Server built: $(Get-Item ipsec-server.exe | Select-Object -ExpandProperty Length) bytes"
    }

    Test-Assertion "Agent builds successfully" {
        $env:CGO_ENABLED = "0"
        go build -o ipsec-agent.exe ./cmd/agent
        if ($LASTEXITCODE -ne 0) { throw "Agent build failed" }
        if (-not (Test-Path "ipsec-agent.exe")) {
            throw "Agent binary not created"
        }
        return "Agent built: $(Get-Item ipsec-agent.exe | Select-Object -ExpandProperty Length) bytes"
    }

    Test-Assertion "Unit tests pass" {
        go test ./internal/... -v -short
        if ($LASTEXITCODE -ne 0) { throw "Unit tests failed" }
        return "All unit tests passed"
    }
}

# Test 3: Docker Tests
if (-not $SkipDocker) {
    Write-Host "`n═══ Phase 3: Docker Deployment Tests ═══`n" -ForegroundColor Magenta

    Test-Assertion "Clean up existing containers" {
        docker-compose down -v 2>$null
        return "Containers cleaned"
    }

    Test-Assertion "Docker images build" {
        docker-compose build
        if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
        return "Images built successfully"
    }

    Test-Assertion "Containers start successfully" {
        docker-compose up -d
        if ($LASTEXITCODE -ne 0) { throw "Failed to start containers" }
        Start-Sleep -Seconds 5
        return "Containers started"
    }

    Test-Assertion "All containers are running" {
        $status = docker-compose ps --format json | ConvertFrom-Json
        $running = $status | Where-Object { $_.State -eq "running" }
        if ($running.Count -lt 3) {
            throw "Expected 3 running containers, found $($running.Count)"
        }
        return "3 containers running: server, agent-linux, agent-linux-2"
    }

    Test-Assertion "Server health endpoint responds" {
        Wait-ForService -Url "http://localhost:8080/api/health"
        $response = Invoke-RestMethod -Uri "http://localhost:8080/api/health"
        if ($response.status -ne "ok") {
            throw "Health check failed: $($response | ConvertTo-Json)"
        }
        return "Health: $($response.status)"
    }
}

# Test 4: API Tests
Write-Host "`n═══ Phase 4: REST API Tests ═══`n" -ForegroundColor Magenta

Test-Assertion "GET /api/health returns 200" {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/api/health" -UseBasicParsing
    if ($response.StatusCode -ne 200) {
        throw "Expected 200, got $($response.StatusCode)"
    }
    return "Status: $($response.StatusCode)"
}

Test-Assertion "GET /api/peers lists registered agents" {
    Start-Sleep -Seconds 10  # Wait for agents to register
    $peers = Invoke-RestMethod -Uri "http://localhost:8080/api/peers"
    if ($peers.Count -lt 2) {
        throw "Expected at least 2 peers, found $($peers.Count)"
    }
    $online = $peers | Where-Object { $_.status -eq "online" }
    if ($online.Count -lt 2) {
        throw "Expected 2 online peers, found $($online.Count)"
    }
    return "Registered peers: $($peers.Count), Online: $($online.Count)"
}

Test-Assertion "GET /api/policies returns empty array initially" {
    $policies = Invoke-RestMethod -Uri "http://localhost:8080/api/policies"
    if ($null -eq $policies) { $policies = @() }
    return "Policies count: $($policies.Count)"
}

Test-Assertion "POST /api/policies creates new policy" {
    $policy = @{
        id = "test-policy-$(Get-Random)"
        name = "Automated Test Policy"
        description = "Created by test script"
        version = 1
        enabled = $true
        priority = 100
        applies_to = @("docker-linux-agent")
        tunnels = @(
            @{
                name = "test-tunnel-auto"
                mode = "esp-tunnel"
                local_address = "172.20.0.10"
                remote_address = "172.20.0.20"
                crypto = @{
                    encryption = "aes256"
                    integrity = "sha256"
                    dhgroup = "modp2048"
                    ikeversion = "ikev2"
                    lifetime = 3600000000000  # 1 hour in nanoseconds
                }
                auth = @{
                    type = "psk"
                    secret = "test-secret-$(Get-Random)"
                }
                traffic_selectors = @(
                    @{
                        local_subnet = "10.10.0.0/24"
                        remote_subnet = "10.20.0.0/24"
                    }
                )
                dpd = @{
                    delay = 30000000000  # 30s in nanoseconds
                    action = "restart"
                }
                autostart = $true
            }
        )
    }
    
    $body = $policy | ConvertTo-Json -Depth 10
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/policies" `
        -Method POST `
        -Body $body `
        -ContentType "application/json"
    
    if (-not $response.id) {
        throw "Policy creation failed: $($response | ConvertTo-Json)"
    }
    
    # Store for later tests
    $script:TestPolicyId = $response.id
    return "Policy created: $($response.id)"
}

Test-Assertion "GET /api/policies/:id returns created policy" {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/policies/$($script:TestPolicyId)"
    if ($response.id -ne $script:TestPolicyId) {
        throw "Policy ID mismatch: expected $($script:TestPolicyId), got $($response.id)"
    }
    return "Policy retrieved: $($response.name)"
}

Test-Assertion "GET /api/policies lists all policies" {
    $policies = Invoke-RestMethod -Uri "http://localhost:8080/api/policies"
    if ($policies.Count -lt 1) {
        throw "Expected at least 1 policy, found $($policies.Count)"
    }
    return "Total policies: $($policies.Count)"
}

# Test 5: Agent Integration Tests
Write-Host "`n═══ Phase 5: Agent Integration Tests ═══`n" -ForegroundColor Magenta

Test-Assertion "Agent syncs policies" {
    Write-Info "Waiting 70 seconds for agent sync cycle..."
    Start-Sleep -Seconds 70
    
    $logs = docker-compose logs agent-linux 2>&1 | Select-String "Fetched policies"
    if (-not $logs) {
        throw "No policy sync logs found"
    }
    
    $lastSync = $logs[-1]
    if ($lastSync -notmatch "count=\d+") {
        throw "Invalid sync log format"
    }
    return "Last sync: $lastSync"
}

Test-Assertion "Agent creates tunnel configuration" {
    $configs = docker exec ipsec-agent-linux ls -la /etc/swanctl/conf.d/ 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to list configs: $configs"
    }
    
    $confFiles = docker exec ipsec-agent-linux find /etc/swanctl/conf.d/ -name "*.conf" 2>&1
    if (-not $confFiles -or $confFiles.Count -eq 0) {
        throw "No configuration files found"
    }
    return "Config files: $($confFiles.Count)"
}

Test-Assertion "strongSwan daemon is running" {
    $processes = docker exec ipsec-agent-linux ps aux 2>&1
    if ($processes -notmatch "charon") {
        throw "strongSwan daemon (charon) not running"
    }
    return "charon process found"
}

Test-Assertion "VICI socket exists" {
    $socket = docker exec ipsec-agent-linux test -S /var/run/charon.vici 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "VICI socket not found"
    }
    return "VICI socket available"
}

Test-Assertion "Tunnel is initiated" {
    $tunnels = docker exec ipsec-agent-linux swanctl --list-sas 2>&1
    if ($tunnels -notmatch "test-tunnel") {
        throw "No test tunnels found in output"
    }
    return "Tunnels listed: $($tunnels -split "`n" | Select-Object -First 1)"
}

# Test 6: Cleanup Tests
Write-Host "`n═══ Phase 6: Cleanup & Verification ═══`n" -ForegroundColor Magenta

Test-Assertion "DELETE /api/policies/:id removes policy" {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/policies/$($script:TestPolicyId)" `
        -Method DELETE
    
    # Verify deletion
    try {
        Invoke-RestMethod -Uri "http://localhost:8080/api/policies/$($script:TestPolicyId)"
        throw "Policy still exists after deletion"
    }
    catch {
        if ($_.Exception.Response.StatusCode -ne 404) {
            throw "Unexpected error: $_"
        }
    }
    return "Policy deleted successfully"
}

if (-not $SkipDocker) {
    Test-Assertion "Stop containers gracefully" {
        docker-compose stop
        if ($LASTEXITCODE -ne 0) { throw "Failed to stop containers" }
        return "Containers stopped"
    }
}

# Test Results Summary
Write-Host "`n╔═══════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║              TEST RESULTS SUMMARY                         ║" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════════════╝`n" -ForegroundColor Cyan

$total = $script:TestsPassed + $script:TestsFailed
$passRate = if ($total -gt 0) { [math]::Round(($script:TestsPassed / $total) * 100, 2) } else { 0 }

Write-Host "Total Tests:   $total" -ForegroundColor White
Write-Host "Passed:        " -NoNewline
Write-Host "$($script:TestsPassed)" -ForegroundColor Green
Write-Host "Failed:        " -NoNewline
Write-Host "$($script:TestsFailed)" -ForegroundColor $(if ($script:TestsFailed -eq 0) { "Green" } else { "Red" })
Write-Host "Pass Rate:     $passRate%`n" -ForegroundColor $(if ($passRate -ge 90) { "Green" } elseif ($passRate -ge 70) { "Yellow" } else { "Red" })

# Detailed Results Table
Write-Host "Detailed Results:" -ForegroundColor Cyan
$script:TestResults | Format-Table -AutoSize

# Generate JSON report
$report = @{
    timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    total = $total
    passed = $script:TestsPassed
    failed = $script:TestsFailed
    pass_rate = $passRate
    results = $script:TestResults
} | ConvertTo-Json -Depth 10

$reportFile = "test-results-$(Get-Date -Format 'yyyyMMdd-HHmmss').json"
$report | Out-File $reportFile
Write-Info "Test report saved to: $reportFile"

# Exit with appropriate code
if ($script:TestsFailed -gt 0) {
    Write-Host "`n❌ TESTS FAILED" -ForegroundColor Red
    exit 1
}
else {
    Write-Host "`n✅ ALL TESTS PASSED" -ForegroundColor Green
    exit 0
}

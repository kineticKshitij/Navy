#!/usr/bin/env pwsh
# Quick smoke test for IPsec Manager
# Runs essential tests only (fast execution)

param([switch]$Verbose)

$ErrorActionPreference = "Continue"

Write-Host @"
╔═══════════════════════════════════════════════════════════╗
║         IPsec Manager - Quick Smoke Test                 ║
╚═══════════════════════════════════════════════════════════╝

"@ -ForegroundColor Cyan

function Test-Quick {
    param([string]$Name, [scriptblock]$Test)
    Write-Host "→ $Name ... " -NoNewline -ForegroundColor Yellow
    try {
        $result = & $Test
        Write-Host "✓" -ForegroundColor Green
        if ($Verbose -and $result) { Write-Host "  $result" -ForegroundColor DarkGray }
        return $true
    }
    catch {
        Write-Host "✗" -ForegroundColor Red
        Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
}

$passed = 0
$failed = 0

# Quick Tests
if (Test-Quick "Docker containers running" { 
    $containers = docker-compose ps --format json 2>&1 | ConvertFrom-Json
    $running = ($containers | Where-Object { $_.State -eq "running" }).Count
    if ($running -lt 3) { throw "Expected 3 containers, found $running" }
    "$running containers active"
}) { $passed++ } else { $failed++ }

if (Test-Quick "Server health check" {
    $response = Invoke-RestMethod http://localhost:8080/api/health -TimeoutSec 5
    if ($response.status -notin @("ok", "healthy")) { throw "Health check failed: $($response.status)" }
    $response.status
}) { $passed++ } else { $failed++ }

if (Test-Quick "Agents registered" {
    $peers = Invoke-RestMethod http://localhost:8080/api/peers -TimeoutSec 5
    if ($peers.Count -lt 2) { throw "Expected 2 peers, found $($peers.Count)" }
    "$($peers.Count) peers online"
}) { $passed++ } else { $failed++ }

if (Test-Quick "Policies accessible" {
    $policies = Invoke-RestMethod http://localhost:8080/api/policies -TimeoutSec 5
    "$($policies.Count) policies loaded"
}) { $passed++ } else { $failed++ }

if (Test-Quick "strongSwan daemon active" {
    $ps = docker exec ipsec-agent-linux ps aux 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { throw "Failed to check processes" }
    if ($ps -notmatch "charon|ipsec") { throw "strongSwan daemon not running" }
    "strongSwan daemon active"
}) { $passed++ } else { $failed++ }

# Summary
Write-Host "`n═══ Results ═══" -ForegroundColor Cyan
Write-Host "Passed: $passed" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })

if ($failed -eq 0) {
    Write-Host "`n✅ System operational" -ForegroundColor Green
    exit 0
}
else {
    Write-Host "`n❌ System has issues" -ForegroundColor Red
    exit 1
}

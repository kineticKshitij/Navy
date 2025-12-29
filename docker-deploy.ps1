# Docker Build and Deploy Script
# Usage: .\docker-deploy.ps1 [command]
# Commands: build, up, down, logs, clean

param(
    [string]$Command = "up"
)

$ErrorActionPreference = "Stop"

function Show-Help {
    Write-Host "Docker Deployment for IPsec Manager" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\docker-deploy.ps1 [command]" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor White
    Write-Host "  build  - Build Docker images" -ForegroundColor White
    Write-Host "  up     - Start all services (default)" -ForegroundColor White
    Write-Host "  down   - Stop all services" -ForegroundColor White
    Write-Host "  logs   - View service logs" -ForegroundColor White
    Write-Host "  clean  - Stop and remove all data" -ForegroundColor White
    Write-Host "  status - Show running containers" -ForegroundColor White
}

function Build-Images {
    Write-Host "Building Docker images..." -ForegroundColor Cyan
    docker-compose build --no-cache
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Images built successfully" -ForegroundColor Green
    } else {
        Write-Host "✗ Build failed" -ForegroundColor Red
        exit 1
    }
}

function Start-Services {
    Write-Host "Starting services..." -ForegroundColor Cyan
    docker-compose up -d
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Services started" -ForegroundColor Green
        Write-Host ""
        Write-Host "Dashboard: http://localhost:8080" -ForegroundColor Yellow
        Write-Host "API: http://localhost:8080/api" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "View logs: docker-compose logs -f" -ForegroundColor White
        Write-Host "Check status: docker-compose ps" -ForegroundColor White
    } else {
        Write-Host "✗ Failed to start services" -ForegroundColor Red
        exit 1
    }
}

function Stop-Services {
    Write-Host "Stopping services..." -ForegroundColor Cyan
    docker-compose down
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Services stopped" -ForegroundColor Green
    }
}

function Show-Logs {
    Write-Host "Showing logs (Ctrl+C to exit)..." -ForegroundColor Cyan
    docker-compose logs -f
}

function Clean-All {
    Write-Host "Stopping services and removing data..." -ForegroundColor Yellow
    docker-compose down -v
    Write-Host "✓ Cleanup complete" -ForegroundColor Green
}

function Show-Status {
    Write-Host "Service Status:" -ForegroundColor Cyan
    docker-compose ps
    Write-Host ""
    Write-Host "Health Check:" -ForegroundColor Cyan
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/api/health" -UseBasicParsing -TimeoutSec 5
        Write-Host "✓ Server is healthy" -ForegroundColor Green
    } catch {
        Write-Host "✗ Server not responding" -ForegroundColor Red
    }
}

# Main execution
switch ($Command.ToLower()) {
    "build" { Build-Images }
    "up" { Start-Services }
    "down" { Stop-Services }
    "logs" { Show-Logs }
    "clean" { Clean-All }
    "status" { Show-Status }
    "help" { Show-Help }
    default {
        Write-Host "Unknown command: $Command" -ForegroundColor Red
        Write-Host ""
        Show-Help
        exit 1
    }
}

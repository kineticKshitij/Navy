# Build script for Windows PowerShell
# Usage: .\build.ps1 [command]
# Commands: build, test, clean, run-server, run-agent, web-build, help

param(
    [string]$Command = "build"
)

$ErrorActionPreference = "Stop"
$VERSION = "v0.1.0"
$BUILD_DIR = "bin"

function Show-Help {
    Write-Host "Usage: .\build.ps1 [command]" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Available commands:" -ForegroundColor Cyan
    Write-Host "  build        - Build both server and agent (default)" -ForegroundColor White
    Write-Host "  test         - Run unit tests" -ForegroundColor White
    Write-Host "  clean        - Remove build artifacts" -ForegroundColor White
    Write-Host "  run-server   - Run the server locally" -ForegroundColor White
    Write-Host "  run-agent    - Run the agent locally" -ForegroundColor White
    Write-Host "  web-build    - Build web dashboard" -ForegroundColor White
    Write-Host "  web-dev      - Start web dev server" -ForegroundColor White
    Write-Host "  install-deps - Install Go dependencies" -ForegroundColor White
    Write-Host "  help         - Show this help message" -ForegroundColor White
}

function Install-Dependencies {
    Write-Host "Installing Go dependencies..." -ForegroundColor Cyan
    go mod download
    go mod tidy
    Write-Host "✓ Dependencies installed" -ForegroundColor Green
}

function Build-Project {
    Write-Host "Building IPsec Manager..." -ForegroundColor Green
    
    # Create bin directory
    if (!(Test-Path $BUILD_DIR)) {
        New-Item -ItemType Directory -Path $BUILD_DIR | Out-Null
    }
    
    # Build server
    Write-Host "Building server..." -ForegroundColor Cyan
    go build -ldflags "-w -s -X main.Version=$VERSION" -o "$BUILD_DIR/ipsec-server.exe" ./cmd/server
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Server built: $BUILD_DIR/ipsec-server.exe" -ForegroundColor Green
    } else {
        Write-Host "✗ Server build failed" -ForegroundColor Red
        exit 1
    }
    
    # Build agent
    Write-Host "Building agent..." -ForegroundColor Cyan
    go build -ldflags "-w -s -X main.Version=$VERSION" -o "$BUILD_DIR/ipsec-agent.exe" ./cmd/agent
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Agent built: $BUILD_DIR/ipsec-agent.exe" -ForegroundColor Green
    } else {
        Write-Host "✗ Agent build failed" -ForegroundColor Red
        exit 1
    }
    
    Write-Host ""
    Write-Host "Build complete! 🚀" -ForegroundColor Green
}

function Run-Tests {
    Write-Host "Running tests..." -ForegroundColor Cyan
    go test -v ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Tests passed" -ForegroundColor Green
    } else {
        Write-Host "✗ Tests failed" -ForegroundColor Red
        exit 1
    }
}

function Clean-Build {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Cyan
    
    if (Test-Path $BUILD_DIR) {
        Remove-Item -Recurse -Force $BUILD_DIR
        Write-Host "✓ Removed $BUILD_DIR" -ForegroundColor Green
    }
    
    if (Test-Path "web/dist") {
        Remove-Item -Recurse -Force "web/dist"
        Write-Host "✓ Removed web/dist" -ForegroundColor Green
    }
    
    if (Test-Path "dist") {
        Remove-Item -Recurse -Force "dist"
        Write-Host "✓ Removed dist" -ForegroundColor Green
    }
    
    Get-ChildItem -Filter "*.log" | Remove-Item -Force
    Get-ChildItem -Filter "*.db" | Remove-Item -Force
    
    Write-Host "Clean complete!" -ForegroundColor Green
}

function Run-Server {
    Write-Host "Starting server..." -ForegroundColor Cyan
    go run ./cmd/server
}

function Run-Agent {
    Write-Host "Starting agent..." -ForegroundColor Cyan
    go run ./cmd/agent start
}

function Build-Web {
    Write-Host "Building web dashboard..." -ForegroundColor Cyan
    Push-Location web
    try {
        npm install
        npm run build
        Write-Host "✓ Web dashboard built" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Run-WebDev {
    Write-Host "Starting web development server..." -ForegroundColor Cyan
    Push-Location web
    try {
        npm install
        npm run dev
    } finally {
        Pop-Location
    }
}

# Main execution
switch ($Command.ToLower()) {
    "build" { Build-Project }
    "test" { Run-Tests }
    "clean" { Clean-Build }
    "run-server" { Run-Server }
    "run-agent" { Run-Agent }
    "web-build" { Build-Web }
    "web-dev" { Run-WebDev }
    "install-deps" { Install-Dependencies }
    "help" { Show-Help }
    default {
        Write-Host "Unknown command: $Command" -ForegroundColor Red
        Write-Host ""
        Show-Help
        exit 1
    }
}

# Build script for ClaraCore Universal Container (Windows)

Write-Host "🏗️  Building ClaraCore Universal Container..." -ForegroundColor Cyan
Write-Host "This image will work on CUDA, ROCm, Vulkan, and CPU!" -ForegroundColor Cyan
Write-Host ""

# Check if we're in the docker directory
if (-not (Test-Path "Dockerfile")) {
    Write-Host "❌ Error: Must run from the docker/ directory" -ForegroundColor Red
    Write-Host "   cd docker; .\build-universal.ps1" -ForegroundColor Yellow
    exit 1
}

# Check if the Linux binary exists
if (-not (Test-Path "..\dist\claracore-linux-amd64")) {
    Write-Host "❌ Error: Linux binary not found at ..\dist\claracore-linux-amd64" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please build the Linux binary first:" -ForegroundColor Yellow
    Write-Host "  cd .. && make build-linux" -ForegroundColor Yellow
    Write-Host "Or on Windows with Go installed:" -ForegroundColor Yellow
    Write-Host "  cd .." -ForegroundColor Yellow
    Write-Host '  $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o dist/claracore-linux-amd64 .' -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

Write-Host "✅ Found Linux binary" -ForegroundColor Green
Write-Host ""

# Build the Docker image
Write-Host "🐳 Building Docker image..." -ForegroundColor Cyan
docker build `
    -f Dockerfile `
    -t claracore:universal `
    -t claracore:latest `
    ..

if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "✅ Build complete!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host "📋 Quick Start:" -ForegroundColor Cyan
Write-Host ""
Write-Host "  # Auto-detect hardware (recommended)" -ForegroundColor Yellow
Write-Host "  docker compose up" -ForegroundColor White
Write-Host ""
Write-Host "  # Force CPU mode" -ForegroundColor Yellow
Write-Host "  docker compose -f docker-compose.cpu-only.yml up" -ForegroundColor White
Write-Host ""
Write-Host "  # Force CUDA (NVIDIA)" -ForegroundColor Yellow
Write-Host "  docker compose -f docker-compose.cuda-explicit.yml up" -ForegroundColor White
Write-Host ""
Write-Host "  # Force ROCm (AMD)" -ForegroundColor Yellow
Write-Host "  docker compose -f docker-compose.rocm-explicit.yml up" -ForegroundColor White
Write-Host ""
Write-Host "  # Force Vulkan (Universal GPU)" -ForegroundColor Yellow
Write-Host "  docker compose -f docker-compose.vulkan-explicit.yml up" -ForegroundColor White
Write-Host ""
Write-Host "📖 See DEPLOYMENT.md for complete documentation" -ForegroundColor Cyan
Write-Host ""

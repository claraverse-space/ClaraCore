# Build script for ClaraCore Docker containers (Windows PowerShell)

$ErrorActionPreference = "Stop"

Write-Host "🐳 Building ClaraCore Docker containers..." -ForegroundColor Cyan

# Check if dist/claracore-linux-amd64 exists
if (-not (Test-Path "dist/claracore-linux-amd64")) {
    Write-Host "❌ Error: dist/claracore-linux-amd64 not found!" -ForegroundColor Red
    Write-Host "Please build the Linux binary first with: python build.py" -ForegroundColor Yellow
    exit 1
}

# Parse arguments
$BuildCuda = $false
$BuildRocm = $false
$Push = $false
$Tag = "latest"

for ($i = 0; $i -lt $args.Count; $i++) {
    switch ($args[$i]) {
        "--cuda" { $BuildCuda = $true }
        "--rocm" { $BuildRocm = $true }
        "--all" { $BuildCuda = $true; $BuildRocm = $true }
        "--push" { $Push = $true }
        "--tag" { 
            $i++
            $Tag = $args[$i]
        }
        default {
            Write-Host "Unknown option: $($args[$i])" -ForegroundColor Red
            Write-Host "Usage: .\build-containers.ps1 [-cuda] [-rocm] [-all] [-push] [-tag TAG]" -ForegroundColor Yellow
            exit 1
        }
    }
}

# If no specific option, build both
if (-not $BuildCuda -and -not $BuildRocm) {
    $BuildCuda = $true
    $BuildRocm = $true
}

# Build CUDA container
if ($BuildCuda) {
    Write-Host "🔨 Building CUDA container..." -ForegroundColor Yellow
    docker build -f Dockerfile.cuda -t "claracore:cuda-${Tag}" -t claracore:cuda .
    
    if ($LASTEXITCODE -eq 0) {
        $size = docker images claracore:cuda --format "{{.Size}}"
        Write-Host "✅ CUDA container built successfully! Size: $size" -ForegroundColor Green
        
        if ($Push) {
            Write-Host "📤 Pushing CUDA container..." -ForegroundColor Yellow
            docker push "claracore:cuda-${Tag}"
            docker push "claracore:cuda"
        }
    } else {
        Write-Host "❌ CUDA container build failed!" -ForegroundColor Red
        exit 1
    }
}

# Build ROCm container
if ($BuildRocm) {
    Write-Host "🔨 Building ROCm container..." -ForegroundColor Yellow
    docker build -f Dockerfile.rocm -t "claracore:rocm-${Tag}" -t claracore:rocm .
    
    if ($LASTEXITCODE -eq 0) {
        $size = docker images claracore:rocm --format "{{.Size}}"
        Write-Host "✅ ROCm container built successfully! Size: $size" -ForegroundColor Green
        
        if ($Push) {
            Write-Host "📤 Pushing ROCm container..." -ForegroundColor Yellow
            docker push "claracore:rocm-${Tag}"
            docker push "claracore:rocm"
        }
    } else {
        Write-Host "❌ ROCm container build failed!" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "🎉 Build complete!" -ForegroundColor Green
Write-Host ""
Write-Host "To run CUDA container:" -ForegroundColor Cyan
Write-Host "  docker-compose -f docker-compose.cuda.yml up -d"
Write-Host "  or"
Write-Host "  docker run -d --gpus all -p 5800:5800 -v ./models:/models claracore:cuda"
Write-Host ""
Write-Host "To run ROCm container:" -ForegroundColor Cyan
Write-Host "  docker-compose -f docker-compose.rocm.yml up -d"
Write-Host "  or"
Write-Host "  docker run -d --device=/dev/kfd --device=/dev/dri -p 5800:5800 -v ./models:/models claracore:rocm"
Write-Host ""
Write-Host "Web UI: http://localhost:5800/ui/" -ForegroundColor Yellow

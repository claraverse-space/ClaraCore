# Build the custom CUDA container with copy entrypoint
# This container will copy specific models to a volume on first boot

Write-Host "🔨 Building ClaraCore CUDA container with model copy feature..." -ForegroundColor Cyan

# Check if dist/claracore-linux-amd64 exists
if (-not (Test-Path "dist/claracore-linux-amd64")) {
    Write-Host "❌ Error: dist/claracore-linux-amd64 not found!" -ForegroundColor Red
    Write-Host "Please build the Linux binary first with: python build.py" -ForegroundColor Yellow
    exit 1
}

# Build the container
docker build -f docker/Dockerfile.cuda-with-copy -t claracore:cuda-copy .

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Container built successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "🚀 To run with your specific models:" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "docker run -d ``" -ForegroundColor Yellow
    Write-Host "  --name claracore ``" -ForegroundColor Yellow
    Write-Host "  --restart unless-stopped ``" -ForegroundColor Yellow
    Write-Host "  --gpus all ``" -ForegroundColor Yellow
    Write-Host "  -p 5800:5800 ``" -ForegroundColor Yellow
    Write-Host "  -v `"C:\BackUP\llama-modelsss:/source:ro`" ``" -ForegroundColor Yellow
    Write-Host "  -v claracore-models:/models ``" -ForegroundColor Yellow
    Write-Host "  -v claracore-config:/app ``" -ForegroundColor Yellow
    Write-Host "  claracore:cuda-copy" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "📝 Notes:" -ForegroundColor Cyan
    Write-Host "  - First boot will copy selected models (one-time, takes ~5-10 min)" -ForegroundColor White
    Write-Host "  - Subsequent boots will be instant and use fast volume I/O" -ForegroundColor White
    Write-Host "  - Models are stored in Docker volume for native speed" -ForegroundColor White
    Write-Host ""
} else {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}

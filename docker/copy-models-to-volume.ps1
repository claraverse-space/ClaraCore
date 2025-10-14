# Copy models from host to Docker volume
# Usage: .\copy-models-to-volume.ps1 "C:\path\to\models"

param(
    [Parameter(Mandatory=$true)]
    [string]$SourcePath
)

Write-Host "📦 Copying models to Docker volume..." -ForegroundColor Cyan
Write-Host ""

# Validate source path
if (-not (Test-Path $SourcePath)) {
    Write-Host "❌ Error: Source path not found: $SourcePath" -ForegroundColor Red
    exit 1
}

# Count GGUF files
$ggufFiles = Get-ChildItem -Path $SourcePath -Filter "*.gguf" -Recurse
if ($ggufFiles.Count -eq 0) {
    Write-Host "⚠️  Warning: No .gguf files found in $SourcePath" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Found $($ggufFiles.Count) GGUF model(s)" -ForegroundColor Green
Write-Host ""

# Create a temporary container to copy files
Write-Host "🚀 Creating temporary container..." -ForegroundColor Cyan
docker run -d --name claracore-temp -v claracore_models:/models alpine sleep 60

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to create temporary container" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Temporary container created" -ForegroundColor Green
Write-Host ""

# Copy files
Write-Host "📁 Copying files..." -ForegroundColor Cyan
foreach ($file in $ggufFiles) {
    Write-Host "   Copying: $($file.Name) ($([math]::Round($file.Length / 1GB, 2)) GB)" -ForegroundColor White
    docker cp $file.FullName claracore-temp:/models/

    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Failed to copy $($file.Name)" -ForegroundColor Red
        docker rm -f claracore-temp | Out-Null
        exit 1
    }
}

Write-Host ""
Write-Host "✅ All files copied successfully!" -ForegroundColor Green
Write-Host ""

# List files in volume
Write-Host "📋 Files in Docker volume:" -ForegroundColor Cyan
docker exec claracore-temp ls -lh /models

# Cleanup
Write-Host ""
Write-Host "🧹 Cleaning up..." -ForegroundColor Cyan
docker rm -f claracore-temp | Out-Null

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "✅ Models copied to Docker volume!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Start ClaraCore: docker compose up -d" -ForegroundColor White
Write-Host "  2. View logs: docker compose logs -f" -ForegroundColor White
Write-Host "  3. Access UI: http://localhost:5800/ui/" -ForegroundColor White
Write-Host ""

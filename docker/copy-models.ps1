# Quick script to copy your specific models to Docker volume

$models = @(
    "C:\BackUP\llama-modelsss\GLM-4.5-Air-IQ4_XS",
    "C:\BackUP\llama-modelsss\Jan",
    "C:\BackUP\llama-modelsss\GLM-4.5-Air-UD-Q2_K_XL.gguf",
    "C:\BackUP\llama-modelsss\ByteDance-Seed_Seed-OSS-36B-Instruct-Q4_K_M.gguf"
)

Write-Host "📦 Copying models to Docker volume..." -ForegroundColor Cyan
Write-Host "This will take a few minutes depending on model sizes..." -ForegroundColor Yellow
Write-Host ""

foreach ($model in $models) {
    if (Test-Path $model) {
        $name = Split-Path $model -Leaf
        Write-Host "  📁 Copying: $name" -ForegroundColor Yellow
        docker cp $model claracore-docker:/models/
        if ($LASTEXITCODE -eq 0) {
            Write-Host "    ✅ Done" -ForegroundColor Green
        } else {
            Write-Host "    ❌ Failed" -ForegroundColor Red
        }
    } else {
        Write-Host "  ⚠️  Not found: $model" -ForegroundColor Red
    }
}

# Copy Qwen models
Write-Host ""
Write-Host "  📁 Creating Qwen directory..." -ForegroundColor Yellow
docker exec claracore-docker mkdir -p /models/Qwen

$qwenModels = @(
    "C:\BackUP\llama-modelsss\Qwen\Qwen_Qwen3-30B-A3B-Instruct-2507-Q5_K_M.gguf",
    "C:\BackUP\llama-modelsss\Qwen\Qwen3-4B-Thinking-2507-Q8_0.gguf"
)

foreach ($model in $qwenModels) {
    if (Test-Path $model) {
        $name = Split-Path $model -Leaf
        Write-Host "  📁 Copying: Qwen/$name" -ForegroundColor Yellow
        docker cp $model "claracore-docker:/models/Qwen/"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "    ✅ Done" -ForegroundColor Green
        } else {
            Write-Host "    ❌ Failed" -ForegroundColor Red
        }
    } else {
        Write-Host "  ⚠️  Not found: $model" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "📊 Checking models in container..." -ForegroundColor Cyan
$count = docker exec claracore-docker sh -c 'find /models -name "*.gguf" | wc -l'
Write-Host "  Found $count GGUF files" -ForegroundColor Green

Write-Host ""
Write-Host "🔄 Restarting container to scan models..." -ForegroundColor Cyan
docker restart claracore-docker

Write-Host ""
Write-Host "✅ Done!" -ForegroundColor Green
Write-Host ""
Write-Host "🌐 Access ClaraCore at: http://localhost:5801/ui/" -ForegroundColor Cyan
Write-Host "📝 Or add /models folder via the setup UI" -ForegroundColor Yellow

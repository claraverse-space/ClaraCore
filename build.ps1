# ClaraCore Build Script (PowerShell)
param(
    [switch]$Clean,
    [switch]$Verbose
)

# Set error handling
$ErrorActionPreference = "Stop"

function Write-Banner {
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "🚀 ClaraCore Build Script (PowerShell)" -ForegroundColor Yellow
    Write-Host "============================================================" -ForegroundColor Cyan
}

function Write-Step {
    param([string]$StepName)
    Write-Host ""
    Write-Host "📦 $StepName" -ForegroundColor Green
    Write-Host "----------------------------------------" -ForegroundColor Gray
}

function Test-Command {
    param([string]$Command)
    try {
        Get-Command $Command -ErrorAction Stop | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

function Invoke-BuildCommand {
    param(
        [string]$Command,
        [string]$WorkingDirectory = $null,
        [string]$Description = ""
    )
    
    if ($Description) {
        Write-Host "💻 $Description" -ForegroundColor Blue
    }
    
    Write-Host "💻 Running: $Command" -ForegroundColor Blue
    
    if ($WorkingDirectory) {
        Write-Host "📁 Directory: $WorkingDirectory" -ForegroundColor Gray
        $oldLocation = Get-Location
        Set-Location $WorkingDirectory
    }
    
    try {
        Invoke-Expression $Command
        if ($LASTEXITCODE -ne 0) {
            throw "Command failed with exit code: $LASTEXITCODE"
        }
        Write-Host "✅ Command completed successfully" -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host "❌ Command failed: $_" -ForegroundColor Red
        return $false
    }
    finally {
        if ($WorkingDirectory) {
            Set-Location $oldLocation
        }
    }
}

function Build-UI {
    Write-Step "Building UI (React/TypeScript)"
    
    $uiDir = "ui"
    if (-not (Test-Path $uiDir)) {
        Write-Host "❌ UI directory not found!" -ForegroundColor Red
        return $false
    }
    
    # Check if package.json exists
    $packageJson = Join-Path $uiDir "package.json"
    if (-not (Test-Path $packageJson)) {
        Write-Host "❌ package.json not found in ui directory!" -ForegroundColor Red
        return $false
    }
    
    # Install dependencies if node_modules doesn't exist
    $nodeModules = Join-Path $uiDir "node_modules"
    if (-not (Test-Path $nodeModules)) {
        Write-Host "📦 Installing npm dependencies..." -ForegroundColor Yellow
        if (-not (Invoke-BuildCommand "npm install" $uiDir "Installing npm dependencies")) {
            return $false
        }
    }
    
    # Build the UI
    if (-not (Invoke-BuildCommand "npm run build" $uiDir "Building UI")) {
        return $false
    }
    
    # Check if build output exists
    $buildOutput = "proxy\ui_dist"
    if (Test-Path $buildOutput) {
        $fullPath = Resolve-Path $buildOutput
        Write-Host "✅ UI build output created at: $fullPath" -ForegroundColor Green
    } else {
        Write-Host "⚠️ UI build completed but output directory not found" -ForegroundColor Yellow
    }
    
    return $true
}

function Build-Go {
    Write-Step "Building ClaraCore (Go Backend)"
    
    # Check if go.mod exists
    if (-not (Test-Path "go.mod")) {
        Write-Host "❌ go.mod not found! Are you in the ClaraCore root directory?" -ForegroundColor Red
        return $false
    }
    
    # Clean previous build
    $executable = "claracore.exe"
    if (Test-Path $executable) {
        Write-Host "🗑️ Removing previous build..." -ForegroundColor Yellow
        try {
            Remove-Item $executable -Force
        }
        catch {
            Write-Host "⚠️ Could not remove previous build: $_" -ForegroundColor Yellow
        }
    }
    
    # Build Go application
    if (-not (Invoke-BuildCommand "go build -o claracore.exe ." $null "Building Go application")) {
        return $false
    }
    
    # Check if executable was created
    if (Test-Path $executable) {
        Write-Host "✅ ClaraCore executable created successfully" -ForegroundColor Green
        
        # Get file size
        $fileInfo = Get-Item $executable
        $sizeMB = [math]::Round($fileInfo.Length / 1MB, 1)
        Write-Host "📊 Executable size: $sizeMB MB" -ForegroundColor Cyan
    } else {
        Write-Host "❌ ClaraCore executable not found after build!" -ForegroundColor Red
        return $false
    }
    
    return $true
}

function Test-Dependencies {
    Write-Step "Checking Dependencies"
    
    # Check Node.js/npm
    if (Test-Command "npm") {
        $npmVersion = (npm --version).Trim()
        Write-Host "✅ npm v$npmVersion found" -ForegroundColor Green
    } else {
        Write-Host "❌ npm not found! Please install Node.js" -ForegroundColor Red
        return $false
    }
    
    # Check Go
    if (Test-Command "go") {
        $goVersion = (go version)
        Write-Host "✅ $goVersion" -ForegroundColor Green
    } else {
        Write-Host "❌ Go not found! Please install Go" -ForegroundColor Red
        return $false
    }
    
    return $true
}

# Main execution
try {
    $startTime = Get-Date
    
    Write-Banner
    
    # Check if we're in the right directory
    if (-not (Test-Path "go.mod")) {
        Write-Host "❌ Please run this script from the ClaraCore root directory" -ForegroundColor Red
        exit 1
    }
    
    # Check dependencies
    if (-not (Test-Dependencies)) {
        Write-Host "`n❌ Build failed: Missing dependencies" -ForegroundColor Red
        exit 1
    }
    
    # Clean build if requested
    if ($Clean) {
        Write-Step "Cleaning Previous Builds"
        if (Test-Path "claracore.exe") { Remove-Item "claracore.exe" -Force }
        if (Test-Path "ui\build") { Remove-Item "ui\build" -Recurse -Force }
        if (Test-Path "proxy\ui_dist") { Remove-Item "proxy\ui_dist" -Recurse -Force }
        Write-Host "✅ Cleaned previous builds" -ForegroundColor Green
    }
    
    # Build UI
    if (-not (Build-UI)) {
        Write-Host "`n❌ Build failed: UI build error" -ForegroundColor Red
        exit 1
    }
    
    # Build Go backend
    if (-not (Build-Go)) {
        Write-Host "`n❌ Build failed: Go build error" -ForegroundColor Red
        exit 1
    }
    
    # Success!
    $endTime = Get-Date
    $buildTime = ($endTime - $startTime).TotalSeconds
    
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "🎉 BUILD SUCCESSFUL!" -ForegroundColor Green
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "⏱️ Total build time: $([math]::Round($buildTime, 2)) seconds" -ForegroundColor Yellow
    Write-Host "🚀 Ready to run: .\claracore.exe" -ForegroundColor Green
    Write-Host "🌐 UI will be served at: http://localhost:5800" -ForegroundColor Cyan
    Write-Host "============================================================" -ForegroundColor Cyan
}
catch {
    Write-Host "`n❌ Build failed: $_" -ForegroundColor Red
    exit 1
}
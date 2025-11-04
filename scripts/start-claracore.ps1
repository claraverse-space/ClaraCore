# Quick Start Script for ClaraCore
# Use this if ClaraCore didn't start automatically after installation

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    $colorMap = @{
        Red = "Red"; Green = "Green"; Yellow = "Yellow"; Blue = "Cyan"
    }
    $fgColor = if ($colorMap.ContainsKey($Color)) { $colorMap[$Color] } else { "White" }
    Write-Host $Message -ForegroundColor $fgColor
}

Write-Host ""
Write-ColorOutput "ClaraCore Quick Start" "Blue"
Write-ColorOutput "=====================" "Blue"
Write-Host ""

# Find ClaraCore installation
$binaryPath = $null
$configPath = $null

# Check common locations
$locations = @(
    @{Binary = "$env:ProgramFiles\ClaraCore\claracore.exe"; Config = "$env:ProgramData\ClaraCore\config.yaml"},
    @{Binary = "$env:LOCALAPPDATA\ClaraCore\claracore.exe"; Config = "$env:APPDATA\ClaraCore\config.yaml"}
)

foreach ($loc in $locations) {
    if (Test-Path $loc.Binary) {
        $binaryPath = $loc.Binary
        $configPath = $loc.Config
        break
    }
}

if (-not $binaryPath) {
    Write-ColorOutput "❌ ClaraCore installation not found!" "Red"
    Write-Host ""
    Write-ColorOutput "Please run the installer first:" "Yellow"
    Write-ColorOutput "  .\install.ps1" "White"
    exit 1
}

Write-ColorOutput "Found ClaraCore at: $binaryPath" "Green"
Write-Host ""

# Check and fix config files
Write-ColorOutput "Checking configuration files..." "Blue"

# Fix models: [] to models: {} in config.yaml
if (Test-Path $configPath) {
    $configContent = Get-Content $configPath -Raw
    if ($configContent -match "models:\s*\[\]") {
        Write-ColorOutput "Fixing config.yaml (models should be {} not [])..." "Yellow"
        $configContent = $configContent -replace "models:\s*\[\]", "models: {}"
        $configContent | Out-File -FilePath $configPath -Encoding UTF8 -NoNewline
        Write-ColorOutput "✓ Fixed config.yaml" "Green"
    }
}

# Create model_folders.json if missing
$modelFoldersPath = Join-Path (Split-Path $configPath) "model_folders.json"
if (-not (Test-Path $modelFoldersPath)) {
    Write-ColorOutput "Creating missing model_folders.json..." "Yellow"
    @"
{
  "folders": []
}
"@ | Out-File -FilePath $modelFoldersPath -Encoding UTF8
    Write-ColorOutput "✓ Created model_folders.json" "Green"
}

Write-Host ""

# Check if already running
try {
    $response = Invoke-WebRequest -Uri "http://localhost:5800/" -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
    if ($response.StatusCode -eq 200) {
        Write-ColorOutput "✅ ClaraCore is already running!" "Green"
        Write-Host ""
        Write-ColorOutput "Access the web interface at:" "Blue"
        Write-ColorOutput "  http://localhost:5800/ui/" "Cyan"
        Write-Host ""
        exit 0
    }
} catch {
    # Not running, continue
}

Write-ColorOutput "Starting ClaraCore..." "Blue"
Write-Host ""

# Try to unblock the binary first
try {
    Unblock-File $binaryPath -ErrorAction SilentlyContinue
    Write-ColorOutput "✓ Unblocked executable" "Green"
} catch {
    Write-ColorOutput "⚠ Could not unblock file (may require admin)" "Yellow"
}

# Try starting via service/task
$serviceStarted = $false

# Try Windows Service first (requires admin)
$service = Get-Service -Name "ClaraCore" -ErrorAction SilentlyContinue
if ($service) {
    try {
        Start-Service -Name "ClaraCore"
        Write-ColorOutput "✓ Started Windows Service" "Green"
        $serviceStarted = $true
    } catch {
        Write-ColorOutput "⚠ Could not start Windows Service: $($_.Exception.Message)" "Yellow"
    }
}

# Try Scheduled Task
if (-not $serviceStarted) {
    $task = Get-ScheduledTask -TaskName "ClaraCore" -ErrorAction SilentlyContinue
    if ($task) {
        try {
            Start-ScheduledTask -TaskName "ClaraCore"
            Write-ColorOutput "✓ Started Scheduled Task" "Green"
            $serviceStarted = $true
        } catch {
            Write-ColorOutput "⚠ Could not start Scheduled Task: $($_.Exception.Message)" "Yellow"
        }
    }
}

# If no service, start manually
if (-not $serviceStarted) {
    Write-ColorOutput "Starting ClaraCore manually..." "Yellow"
    Write-Host ""
    
    try {
        # Start in a new hidden window
        $processInfo = New-Object System.Diagnostics.ProcessStartInfo
        $processInfo.FileName = $binaryPath
        $processInfo.Arguments = "--config `"$configPath`""
        $processInfo.UseShellExecute = $false
        $processInfo.CreateNoWindow = $true
        $processInfo.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
        
        $process = [System.Diagnostics.Process]::Start($processInfo)
        
        Write-ColorOutput "✓ Started ClaraCore process (PID: $($process.Id))" "Green"
        Write-ColorOutput "  Note: This is a manual start. Use the installer to set up auto-start." "Yellow"
    } catch {
        Write-ColorOutput "❌ Failed to start ClaraCore: $($_.Exception.Message)" "Red"
        Write-Host ""
        Write-ColorOutput "Troubleshooting:" "Yellow"
        Write-ColorOutput "1. Unblock the binary:" "White"
        Write-ColorOutput "   Unblock-File `"$binaryPath`"" "Cyan"
        Write-Host ""
        Write-ColorOutput "2. Add Windows Defender exclusion:" "White"
        Write-ColorOutput "   .\scripts\add-defender-exclusion.bat" "Cyan"
        Write-Host ""
        Write-ColorOutput "3. Run as Administrator and try again" "White"
        exit 1
    }
}

# Wait for service to start
Write-Host ""
Write-ColorOutput "Waiting for ClaraCore to initialize..." "Blue"
Start-Sleep -Seconds 5

# Check if accessible
$maxAttempts = 10
$attempt = 0

Write-Host -NoNewline "  Checking"
while ($attempt -lt $maxAttempts) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:5800/" -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
        if ($response.StatusCode -eq 200) {
            Write-Host ""
            Write-Host ""
            Write-ColorOutput "✅ ClaraCore is running and accessible!" "Green"
            Write-Host ""
            Write-ColorOutput "┌─────────────────────────────────────────┐" "Blue"
            Write-ColorOutput "│  Open your browser and visit:          │" "Blue"
            Write-ColorOutput "│                                         │" "Blue"
            Write-ColorOutput "│  http://localhost:5800/ui/              │" "Cyan"
            Write-ColorOutput "│                                         │" "Blue"
            Write-ColorOutput "└─────────────────────────────────────────┘" "Blue"
            Write-Host ""
            exit 0
        }
    } catch {
        Write-Host -NoNewline "."
        Start-Sleep -Seconds 1
    }
    $attempt++
}

Write-Host ""
Write-Host ""
Write-ColorOutput "⚠ ClaraCore may still be starting up..." "Yellow"
Write-Host ""
Write-ColorOutput "Try accessing the web interface in a moment:" "White"
Write-ColorOutput "  http://localhost:5800/ui/" "Cyan"
Write-Host ""
Write-ColorOutput "To check status:" "White"
Write-ColorOutput "  .\scripts\claracore-service.ps1 status" "Cyan"
Write-Host ""


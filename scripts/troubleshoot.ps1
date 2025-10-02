# ClaraCore Windows Troubleshooting Script
# Fixes common Windows security and service issues

param(
    [switch]$UnblockFile = $false,
    [switch]$FixService = $false,
    [switch]$ShowStatus = $false,
    [switch]$All = $false
)

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    $colorMap = @{
        Red = "Red"; Green = "Green"; Yellow = "Yellow"; Blue = "Blue"
    }
    if ($colorMap.ContainsKey($Color)) {
        Write-Host $Message -ForegroundColor $colorMap[$Color]
    } else {
        Write-Host $Message
    }
}

function Find-ClaraInstallation {
    $paths = @(
        "$env:LOCALAPPDATA\ClaraCore\claracore.exe",
        "$env:ProgramFiles\ClaraCore\claracore.exe",
        ".\claracore.exe"
    )
    
    foreach ($path in $paths) {
        if (Test-Path $path) { return $path }
    }
    return $null
}

function Test-Binary {
    param([string]$Path)
    try {
        $result = Start-Process -FilePath $Path -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop
        return $result.ExitCode -eq 0
    } catch {
        if ($_.Exception.Message -like "*Application Control*") {
            Write-ColorOutput "Windows Application Control is blocking this file" "Red"
        }
        return $false
    }
}

function Invoke-UnblockFile {
    param([string]$Path)
    Write-ColorOutput "Unblocking file..." "Blue"
    try {
        Unblock-File $Path
        Write-ColorOutput "File unblocked successfully" "Green"
        
        # Check if it's Application Control blocking
        try {
            $result = Start-Process -FilePath $Path -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop
            if ($result.ExitCode -eq 0) {
                Write-ColorOutput "Binary now works!" "Green"
                return $true
            }
        } catch {
            if ($_.Exception.Message -like "*Application Control*") {
                Write-ColorOutput "Windows Application Control is still blocking execution" "Red"
                Write-ColorOutput "This requires administrator action in Windows Security settings" "Yellow"
                Write-ColorOutput "Go to: Windows Security > App & browser control > Reputation-based protection" "Yellow"
                Write-ColorOutput "Then disable 'Check apps and files'" "Yellow"
            }
        }
    } catch {
        Write-ColorOutput "Failed to unblock: $($_.Exception.Message)" "Red"
    }
    return $false
}

function Show-Status {
    param([string]$Path)
    Write-ColorOutput "=== ClaraCore Status ===" "Blue"
    
    if ($Path) {
        Write-ColorOutput "Binary: $Path" "Green"
        $file = Get-Item $Path
        Write-ColorOutput "Size: $([math]::Round($file.Length/1MB,1)) MB" "White"
        Write-ColorOutput "Date: $($file.LastWriteTime)" "White"
        
        if (Test-Binary $Path) {
            Write-ColorOutput "Execution: OK" "Green"
        } else {
            Write-ColorOutput "Execution: BLOCKED" "Red"
        }
    } else {
        Write-ColorOutput "Binary: NOT FOUND" "Red"
    }
    
    $service = Get-Service -Name "ClaraCore" -ErrorAction SilentlyContinue
    if ($service) {
        Write-ColorOutput "Service: $($service.Status)" "White"
    } else {
        Write-ColorOutput "Service: NOT INSTALLED" "Red"
    }
}

# Main execution
Write-ColorOutput "ClaraCore Troubleshooter" "Blue"
Write-Host ""

if (-not ($UnblockFile -or $FixService -or $ShowStatus -or $All)) {
    Write-ColorOutput "Usage:" "Yellow"
    Write-ColorOutput "  .\troubleshoot.ps1 -UnblockFile" "White"
    Write-ColorOutput "  .\troubleshoot.ps1 -ShowStatus" "White"
    Write-ColorOutput "  .\troubleshoot.ps1 -All" "White"
    exit
}

$binaryPath = Find-ClaraInstallation
if (-not $binaryPath) {
    Write-ColorOutput "ClaraCore not found. Install it first." "Red"
    exit 1
}

if ($ShowStatus -or $All) {
    Show-Status $binaryPath
    Write-Host ""
}

if ($UnblockFile -or $All) {
    if (-not (Test-Binary $binaryPath)) {
        Invoke-UnblockFile $binaryPath
    } else {
        Write-ColorOutput "Binary already works!" "Green"
    }
}

Write-ColorOutput "Done!" "Green"
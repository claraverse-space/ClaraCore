# Fix JarvisCore Windows Service
# This script fixes common issues with the JarvisCore Windows Service

param(
    [switch]$RemoveService = $false,
    [switch]$RecreateService = $false,
    [switch]$ShowStatus = $false
)

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    $colorMap = @{ Red = "Red"; Green = "Green"; Yellow = "Yellow"; Blue = "Blue" }
    if ($colorMap.ContainsKey($Color)) {
        Write-Host $Message -ForegroundColor $colorMap[$Color]
    } else {
        Write-Host $Message
    }
}

function Test-ServiceStatus {
    $service = Get-Service -Name "JarvisCore" -ErrorAction SilentlyContinue
    if ($service) {
        Write-ColorOutput "Service Status: $($service.Status)" "Blue"
        Write-ColorOutput "Service Start Type: $($service.StartType)" "Blue"
        
        # Get detailed service config
        $config = sc.exe qc JarvisCore 2>$null
        if ($config) {
            Write-ColorOutput "Service Configuration:" "Blue"
            $config | ForEach-Object { Write-Host "  $_" }
        }
        return $true
    } else {
        Write-ColorOutput "JarvisCore service not found" "Red"
        return $false
    }
}

function Remove-JarvisCoreService {
    Write-ColorOutput "Removing JarvisCore service..." "Blue"
    
    # Stop service if running
    $service = Get-Service -Name "JarvisCore" -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq "Running") {
        Write-ColorOutput "Stopping service..." "Yellow"
        Stop-Service -Name "JarvisCore" -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
    }
    
    # Remove service
    sc.exe delete JarvisCore | Out-Null
    Write-ColorOutput "Service removed" "Green"
}

function Install-JarvisCoreServiceFixed {
    Write-ColorOutput "Installing JarvisCore service with fixed configuration..." "Blue"
    
    # Paths
    $binaryPath = "$env:LOCALAPPDATA\JarvisCore\jarviscore.exe"
    
    if (-not (Test-Path $binaryPath)) {
        Write-ColorOutput "Binary not found at $binaryPath" "Red"
        return $false
    }
    
    # Test binary first
    Write-ColorOutput "Testing binary execution..." "Blue"
    try {
        $result = Start-Process -FilePath $binaryPath -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop
        if ($result.ExitCode -ne 0) {
            Write-ColorOutput "Binary test failed - Windows security may be blocking" "Red"
            Write-ColorOutput "Try: Unblock-File '$binaryPath'" "Yellow"
            return $false
        }
        Write-ColorOutput "Binary test successful" "Green"
    } catch {
        Write-ColorOutput "Binary execution failed: $($_.Exception.Message)" "Red"
        return $false
    }
    
    # Create service command without config file - let JarvisCore create its own
    $serviceCommand = "`"$binaryPath`""
    $workingDir = Split-Path $binaryPath -Parent
    
    try {
        # Create the service
        New-Service -Name "JarvisCore" -BinaryPathName $serviceCommand -DisplayName "JarvisCore AI Inference Server" -Description "JarvisCore AI model inference server - config auto-created" -StartupType Manual | Out-Null
        Write-ColorOutput "Service created successfully (Manual start)" "Green"
        Write-ColorOutput "JarvisCore will create config.yaml automatically in: $workingDir" "Blue"
        
        # Try to start the service
        Write-ColorOutput "Attempting to start service..." "Blue"
        try {
            Start-Service -Name "JarvisCore" -ErrorAction Stop
            Write-ColorOutput "Service started successfully!" "Green"
            
            # If it starts successfully, change to automatic
            Set-Service -Name "JarvisCore" -StartupType Automatic
            Write-ColorOutput "Changed to automatic startup" "Green"
            
        } catch {
            Write-ColorOutput "Service created but failed to start: $($_.Exception.Message)" "Yellow"
            Write-ColorOutput "" "White"
            Write-ColorOutput "Possible solutions:" "Yellow"
            Write-ColorOutput "1. Check Windows Event Log for details" "Blue"
            Write-ColorOutput "2. Try manual start: $binaryPath" "Blue"
            Write-ColorOutput "3. Check Windows security blocking: Unblock-File '$binaryPath'" "Blue"
        }
        
        return $true
        
    } catch {
        Write-ColorOutput "Failed to create service: $($_.Exception.Message)" "Red"
        return $false
    }
}

# Main execution
Write-ColorOutput "JarvisCore Service Fix Tool" "Blue"
Write-ColorOutput "=========================" "Blue"
Write-Host ""

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-ColorOutput "This script requires Administrator privileges" "Red"
    Write-ColorOutput "Please run PowerShell as Administrator and try again" "Yellow"
    exit 1
}

if ($ShowStatus) {
    Test-ServiceStatus
    exit 0
}

if ($RemoveService) {
    Remove-JarvisCoreService
    exit 0
}

if ($RecreateService) {
    Write-ColorOutput "Recreating JarvisCore service..." "Blue"
    
    # Remove existing service
    if (Test-ServiceStatus) {
        Remove-JarvisCoreService
        Start-Sleep -Seconds 2
    }
    
    # Install fixed service
    $success = Install-JarvisCoreServiceFixed
    
    if ($success) {
        Write-ColorOutput "" "White"
        Write-ColorOutput "Service recreation completed!" "Green"
        Write-ColorOutput "" "White"
        Write-ColorOutput "Test the service:" "Blue"
        Write-ColorOutput "  Get-Service JarvisCore" "White"
        Write-ColorOutput "  Start-Service JarvisCore" "White"
    }
    
    exit 0
}

# Default: Show help
Write-ColorOutput "Usage:" "Yellow"
Write-ColorOutput "  .\fix-service.ps1 -ShowStatus      # Show current service status" "White"
Write-ColorOutput "  .\fix-service.ps1 -RecreateService # Remove and recreate service" "White"
Write-ColorOutput "  .\fix-service.ps1 -RemoveService   # Remove service completely" "White"
Write-Host ""
Write-ColorOutput "Common fixes:" "Yellow"
Write-ColorOutput "1. Recreate service:  .\fix-service.ps1 -RecreateService" "Blue"
Write-ColorOutput "2. Remove service:    .\fix-service.ps1 -RemoveService" "Blue"
Write-ColorOutput "3. Manual startup:    C:\Users\$env:USERNAME\AppData\Local\JarvisCore\jarviscore.exe" "Blue"
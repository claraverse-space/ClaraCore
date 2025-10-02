# ClaraCore Windows Uninstall Script
# Removes ClaraCore installation and Windows Service

param(
    [switch]$RemoveConfig = $false,
    [switch]$Force = $false
)

# Colors for output
$colors = @{
    Red = "Red"
    Green = "Green"
    Yellow = "Yellow"
    Blue = "Blue"
}

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    if ($colors.ContainsKey($Color)) {
        Write-Host $Message -ForegroundColor $colors[$Color]
    } else {
        Write-Host $Message -ForegroundColor White
    }
}

function Write-Header {
    param([string]$Title)
    Write-Host ""
    Write-ColorOutput "========================================" "Blue"
    Write-ColorOutput "  $Title" "Blue"
    Write-ColorOutput "========================================" "Blue"
    Write-Host ""
}

function Test-AdminRights {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Find-ClaraInstallations {
    Write-ColorOutput "Finding ClaraCore installations..." "Blue"
    
    $installations = @()
    
    # Check system-wide installation
    $systemPath = "$env:ProgramFiles\ClaraCore"
    if (Test-Path (Join-Path $systemPath "claracore.exe")) {
        $installations += @{
            Type = "System"
            Path = $systemPath
            ConfigPath = "$env:ProgramData\ClaraCore"
            Binary = Join-Path $systemPath "claracore.exe"
        }
    }
    
    # Check user installation
    $userPath = "$env:LOCALAPPDATA\ClaraCore"
    if (Test-Path (Join-Path $userPath "claracore.exe")) {
        $installations += @{
            Type = "User"
            Path = $userPath
            ConfigPath = "$env:APPDATA\ClaraCore"
            Binary = Join-Path $userPath "claracore.exe"
        }
    }
    
    # Check PATH for other installations
    $pathDirs = $env:PATH -split ";"
    foreach ($dir in $pathDirs) {
        $claraPath = Join-Path $dir "claracore.exe"
        if ((Test-Path $claraPath) -and ($claraPath -notin $installations.Binary)) {
            $installations += @{
                Type = "Custom"
                Path = $dir
                ConfigPath = ""
                Binary = $claraPath
            }
        }
    }
    
    return $installations
}

function Stop-ClaraProcesses {
    Write-ColorOutput "Stopping ClaraCore processes..." "Blue"
    
    $processes = Get-Process -Name "claracore" -ErrorAction SilentlyContinue
    if ($processes) {
        foreach ($process in $processes) {
            try {
                $process.Kill()
                Write-ColorOutput "Stopped process: $($process.Id)" "Green"
            }
            catch {
                Write-ColorOutput "Warning: Could not stop process $($process.Id): $($_.Exception.Message)" "Yellow"
            }
        }
        Start-Sleep -Seconds 2
    }
}

function Remove-WindowsService {
    if (-not (Test-AdminRights)) {
        Write-ColorOutput "Warning: Cannot remove Windows Service without administrator privileges" "Yellow"
        return
    }
    
    Write-ColorOutput "Removing Windows Service..." "Blue"
    
    $serviceName = "ClaraCore"
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    
    if ($service) {
        try {
            # Stop service if running
            if ($service.Status -eq "Running") {
                Stop-Service -Name $serviceName -Force
                Write-ColorOutput "Service stopped" "Green"
            }
            
            # Remove service
            sc.exe delete $serviceName | Out-Null
            Write-ColorOutput "Windows Service removed" "Green"
        }
        catch {
            Write-ColorOutput "Error removing service: $($_.Exception.Message)" "Red"
        }
    }
    else {
        Write-ColorOutput "No Windows Service found" "Yellow"
    }
}

function Remove-Installation {
    param([hashtable]$Installation)
    
    Write-ColorOutput "Removing $($Installation.Type) installation..." "Blue"
    Write-ColorOutput "Path: $($Installation.Path)" "Yellow"
    
    try {
        # Remove binary directory
        if (Test-Path $Installation.Path) {
            Remove-Item $Installation.Path -Recurse -Force
            Write-ColorOutput "Removed installation directory" "Green"
        }
        
        # Remove from PATH if user installation
        if ($Installation.Type -eq "User") {
            $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
            if ($userPath -like "*$($Installation.Path)*") {
                $pathParts = $userPath -split ";"
                $newPathParts = $pathParts | Where-Object { $_ -ne $Installation.Path }
                $newPath = $newPathParts -join ";"
                [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
                Write-ColorOutput "Removed from user PATH" "Green"
            }
        }
        
        # Remove configuration if requested
        if ($RemoveConfig -and $Installation.ConfigPath -and (Test-Path $Installation.ConfigPath)) {
            Remove-Item $Installation.ConfigPath -Recurse -Force
            Write-ColorOutput "Removed configuration directory" "Green"
        }
    }
    catch {
        Write-ColorOutput "Error removing installation: $($_.Exception.Message)" "Red"
    }
}

function Remove-DesktopShortcut {
    $shortcutPath = Join-Path $env:USERPROFILE "Desktop\ClaraCore.lnk"
    if (Test-Path $shortcutPath) {
        Remove-Item $shortcutPath -Force
        Write-ColorOutput "Removed desktop shortcut" "Green"
    }
}

function Confirm-Uninstall {
    param([array]$Installations)
    
    if ($Force) {
        return $true
    }
    
    Write-Host ""
    Write-ColorOutput "The following ClaraCore installations will be removed:" "Yellow"
    foreach ($install in $Installations) {
        Write-ColorOutput "  - $($install.Type): $($install.Path)" "White"
    }
    
    if ($RemoveConfig) {
        Write-Host ""
        Write-ColorOutput "Configuration data will also be removed:" "Yellow"
        foreach ($install in $Installations) {
            if ($install.ConfigPath) {
                Write-ColorOutput "  - $($install.ConfigPath)" "White"
            }
        }
    }
    
    Write-Host ""
    $response = Read-Host "Continue with uninstallation? (y/N)"
    return $response -match "^[Yy]"
}

function Main {
    Write-Header "ClaraCore Windows Uninstaller"
    
    # Find installations
    $installations = Find-ClaraInstallations
    
    if ($installations.Count -eq 0) {
        Write-ColorOutput "No ClaraCore installations found" "Yellow"
        exit 0
    }
    
    # Confirm uninstall
    if (-not (Confirm-Uninstall $installations)) {
        Write-ColorOutput "Uninstallation cancelled" "Yellow"
        exit 0
    }
    
    try {
        # Stop processes
        Stop-ClaraProcesses
        
        # Remove Windows Service
        Remove-WindowsService
        
        # Remove installations
        foreach ($installation in $installations) {
            Remove-Installation $installation
        }
        
        # Remove desktop shortcut
        Remove-DesktopShortcut
        
        Write-Host ""
        Write-ColorOutput "Uninstallation completed successfully!" "Green"
        
        if (-not $RemoveConfig) {
            Write-Host ""
            Write-ColorOutput "Configuration files were preserved." "Yellow"
            Write-ColorOutput "Use -RemoveConfig flag to remove them." "Yellow"
        }
    }
    catch {
        Write-ColorOutput "Uninstallation failed: $($_.Exception.Message)" "Red"
        exit 1
    }
}

# Run main uninstaller
Main
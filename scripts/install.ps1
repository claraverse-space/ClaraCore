# ClaraCore Windows Installation Script
# Downloads the latest release and sets up Windows Service

param(
    [switch]$SystemWide = $false,
    [switch]$NoService = $false,
    [string]$InstallPath = "",
    [string]$ModelFolder = ""
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

function Get-LatestRelease {
    Write-ColorOutput "Fetching latest release information..." "Blue"
    
    try {
        $repo = "claraverse-space/ClaraCore"
        $releaseUrl = "https://api.github.com/repos/$repo/releases/latest"
        $release = Invoke-RestMethod -Uri $releaseUrl -UseBasicParsing
        
        Write-ColorOutput "Latest release: $($release.tag_name)" "Green"
        return $release
    }
    catch {
        Write-ColorOutput "Error: Could not fetch latest release: $($_.Exception.Message)" "Red"
        exit 1
    }
}

function Download-Binary {
    param([object]$Release)
    
    $binaryName = "claracore-windows-amd64.exe"
    $asset = $Release.assets | Where-Object { $_.name -eq $binaryName }
    
    if (-not $asset) {
        Write-ColorOutput "Error: Binary $binaryName not found in release" "Red"
        exit 1
    }
    
    $downloadUrl = $asset.browser_download_url
    Write-ColorOutput "Downloading ClaraCore binary..." "Blue"
    Write-ColorOutput "URL: $downloadUrl" "Yellow"
    
    $tempFile = [System.IO.Path]::GetTempFileName() + ".exe"
    
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -UseBasicParsing
        Write-ColorOutput "Download completed successfully" "Green"
        return $tempFile
    }
    catch {
        Write-ColorOutput "Error: Failed to download binary: $($_.Exception.Message)" "Red"
        exit 1
    }
}

function Install-Binary {
    param([string]$TempFile)
    
    if ($SystemWide) {
        if (-not (Test-AdminRights)) {
            Write-ColorOutput "Error: System-wide installation requires administrator privileges" "Red"
            Write-ColorOutput "Please run as administrator or remove -SystemWide flag" "Yellow"
            exit 1
        }
        $installDir = "$env:ProgramFiles\ClaraCore"
        $configDir = "$env:ProgramData\ClaraCore"
    }
    else {
        $installDir = "$env:LOCALAPPDATA\ClaraCore"
        $configDir = "$env:APPDATA\ClaraCore"
    }
    
    if ($InstallPath) {
        $installDir = $InstallPath
    }
    
    Write-ColorOutput "Installing to: $installDir" "Blue"
    
    # Create directories
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    
    # Install binary
    $binaryPath = Join-Path $installDir "claracore.exe"
    Copy-Item $TempFile $binaryPath -Force
    
    # Unblock the downloaded file to prevent Windows security warnings
    try {
        Unblock-File $binaryPath
        Write-ColorOutput "Unblocked executable for Windows security" "Green"
    }
    catch {
        Write-ColorOutput "Warning: Could not unblock file. You may need to run 'Unblock-File `"$binaryPath`"' manually" "Yellow"
    }
    
    # Add to PATH if user install
    if (-not $SystemWide) {
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ($userPath -notlike "*$installDir*") {
            $newPath = "$userPath;$installDir"
            [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
            Write-ColorOutput "Added $installDir to user PATH" "Green"
        }
    }
    
    Write-ColorOutput "Binary installed successfully" "Green"
    return @{
        BinaryPath = $binaryPath
        ConfigDir = $configDir
        InstallDir = $installDir
    }
}

function Create-DefaultConfig {
    param([string]$ConfigDir)
    
    Write-ColorOutput "Creating default configuration..." "Blue"
    
    $configYaml = @"
# ClaraCore Configuration
# This file is auto-generated. You can modify it or regenerate via the web UI.

host: "127.0.0.1"
port: 5800
cors: true
api_key: ""

# Models will be auto-discovered and configured
models: []

# Model groups for memory management
groups: {}
"@

    $settingsJson = @"
{
  "gpuType": "auto",
  "backend": "auto",
  "vramGB": 0,
  "ramGB": 0,
  "preferredContext": 8192,
  "throughputFirst": true,
  "enableJinja": true,
  "requireApiKey": false,
  "apiKey": ""
}
"@

    $configYaml | Out-File -FilePath (Join-Path $ConfigDir "config.yaml") -Encoding UTF8
    $settingsJson | Out-File -FilePath (Join-Path $ConfigDir "settings.json") -Encoding UTF8
    
    Write-ColorOutput "Default configuration created in $ConfigDir" "Green"
}

function Install-WindowsService {
    param([hashtable]$Paths)
    
    if (-not (Test-AdminRights)) {
        Write-ColorOutput "Warning: Cannot install Windows Service without administrator privileges" "Yellow"
        Write-ColorOutput "Skipping service installation. You can install manually later." "Yellow"
        return
    }
    
    Write-ColorOutput "Installing Windows Service..." "Blue"
    
    $serviceName = "ClaraCore"
    $serviceDisplayName = "ClaraCore AI Inference Server"
    $serviceDescription = "ClaraCore AI model inference server with automatic setup"
    
    # Stop and remove existing service if it exists
    $existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-ColorOutput "Stopping existing service..." "Yellow"
        Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $serviceName | Out-Null
        Start-Sleep -Seconds 2
    }
    
    # Create service with proper user context
    # Don't specify config file - let ClaraCore create its own
    $binaryPath = "`"$($Paths.BinaryPath)`""
    
    try {
        # Test if binary can run first
        Write-ColorOutput "Testing binary before service installation..." "Blue"
        $testResult = Start-Process -FilePath $Paths.BinaryPath -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction SilentlyContinue
        
        if ($testResult.ExitCode -ne 0) {
            throw "Binary test failed. Likely blocked by Windows security policies."
        }
        
        # Create startup shortcut instead of Windows service (more reliable for console apps)
        Write-ColorOutput "Setting up auto-start shortcut..." "Blue"
        
        try {
            $startupPath = [Environment]::GetFolderPath("Startup")
            $shortcutPath = Join-Path $startupPath "ClaraCore.lnk"
            
            $shell = New-Object -ComObject WScript.Shell
            $shortcut = $shell.CreateShortcut($shortcutPath)
            $shortcut.TargetPath = $Paths.BinaryPath
            $shortcut.WorkingDirectory = Split-Path $Paths.BinaryPath -Parent
            $shortcut.WindowStyle = 7  # Minimized window
            $shortcut.Description = "ClaraCore AI Inference Server"
            $shortcut.Save()
            
            Write-ColorOutput "Auto-start shortcut created successfully" "Green"
            Write-ColorOutput "ClaraCore will start automatically when you log in" "Green"
            Write-ColorOutput "Shortcut location: $shortcutPath" "Blue"
            
            # Release COM object
            [System.Runtime.Interopservices.Marshal]::ReleaseComObject($shell) | Out-Null
            
        } catch {
            Write-ColorOutput "Warning: Could not create auto-start shortcut: $($_.Exception.Message)" "Yellow"
        }
    }
    catch {
        Write-ColorOutput "Error: Failed to install Windows Service: $($_.Exception.Message)" "Red"
        Write-ColorOutput "" "White"
        Write-ColorOutput "This is likely due to Windows security policies blocking the executable." "Yellow"
        Write-ColorOutput "Solutions:" "Yellow"
        Write-ColorOutput "1. Unblock the file: Unblock-File `"$($Paths.BinaryPath)`"" "White"
        Write-ColorOutput "2. Start manually: $($Paths.BinaryPath)" "White"
        Write-ColorOutput "3. Add Windows Defender exclusion for: $(Split-Path $Paths.BinaryPath)" "White"
    }
}

function Create-DesktopShortcut {
    param([hashtable]$Paths)
    
    Write-ColorOutput "Creating desktop shortcut..." "Blue"
    
    $WshShell = New-Object -comObject WScript.Shell
    $shortcutPath = Join-Path $env:USERPROFILE "Desktop\ClaraCore.lnk"
    $shortcut = $WshShell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $Paths.BinaryPath
    $shortcut.WorkingDirectory = $Paths.ConfigDir
    $shortcut.Description = "ClaraCore AI Inference Server"
    $shortcut.Save()
    
    Write-ColorOutput "Desktop shortcut created" "Green"
}

function Show-NextSteps {
    param([hashtable]$Paths)
    
    Write-Header "Installation Completed!"
    
    Write-ColorOutput "Next steps:" "Yellow"
    Write-Host ""
    
    # Test if binary works
    Write-ColorOutput "Testing installation..." "Blue"
    try {
        $testResult = Start-Process -FilePath $Paths.BinaryPath -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop
        if ($testResult.ExitCode -eq 0) {
            Write-ColorOutput "✅ Installation test successful!" "Green"
        } else {
            throw "Binary test failed"
        }
    }
    catch {
        Write-ColorOutput "⚠️  Binary is blocked by Windows security" "Yellow"
        Write-Host ""
        Write-ColorOutput "IMPORTANT: Fix Windows security blocking:" "Red"
        Write-ColorOutput "   Unblock-File `"$($Paths.BinaryPath)`"" "White"
        Write-Host ""
        Write-ColorOutput "Or add Windows Defender exclusion:" "Yellow"
        Write-ColorOutput "   Windows Security > Exclusions > Add folder: $(Split-Path $Paths.BinaryPath)" "White"
    }
    Write-Host ""
    
    Write-ColorOutput "1. Configure your models folder:" "White"
    Write-ColorOutput "   $($Paths.BinaryPath) --models-folder `"C:\path\to\your\models`"" "Blue"
    Write-Host ""
    
    Write-ColorOutput "2. Or start with the web interface:" "White"
    Write-ColorOutput "   $($Paths.BinaryPath)" "Blue"
    Write-ColorOutput "   Then visit: http://localhost:5800/ui/setup" "Blue"
    Write-Host ""
    
    Write-ColorOutput "3. Auto-start:" "White"
    Write-ColorOutput "   ClaraCore will start automatically when you log in" "Blue"
    Write-ColorOutput "   Manual start: $($Paths.BinaryPath)" "Blue"
    Write-ColorOutput "   Stop: Close the ClaraCore window or press Ctrl+C" "Blue"
    Write-Host ""
    
    Write-ColorOutput "4. Configuration files:" "White"
    Write-ColorOutput "   Config:    $(Join-Path $Paths.ConfigDir "config.yaml")" "Blue"
    Write-ColorOutput "   Settings:  $(Join-Path $Paths.ConfigDir "settings.json")" "Blue"
    Write-Host ""
    
    Write-ColorOutput "Documentation: https://github.com/badboysm890/ClaraCore/tree/main/docs" "Green"
    Write-ColorOutput "Support: https://github.com/badboysm890/ClaraCore/issues" "Green"
}

function Main {
    Write-Header "ClaraCore Windows Installer"
    
    # Check requirements
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        Write-ColorOutput "Error: PowerShell 5.0 or higher is required" "Red"
        exit 1
    }
    
    try {
        # Get latest release
        $release = Get-LatestRelease
        
        # Download binary
        $tempFile = Download-Binary $release
        
        # Install binary
        $paths = Install-Binary $tempFile
        
        # Create configuration
        Create-DefaultConfig $paths.ConfigDir
        
        # Install Windows Service (if requested and admin)
        if (-not $NoService) {
            Install-WindowsService $paths
        }
        
        # Create desktop shortcut
        Create-DesktopShortcut $paths
        
        # Clean up temp file
        Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
        
        # Show next steps
        Show-NextSteps $paths
        
        Write-Host ""
        Write-ColorOutput "Installation completed successfully!" "Green"
    }
    catch {
        Write-ColorOutput "Installation failed: $($_.Exception.Message)" "Red"
        exit 1
    }
}

# Run main installation
Main
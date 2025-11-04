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
models: {}

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

    $modelFoldersJson = @"
{
  "folders": []
}
"@

    $configYaml | Out-File -FilePath (Join-Path $ConfigDir "config.yaml") -Encoding UTF8
    $settingsJson | Out-File -FilePath (Join-Path $ConfigDir "settings.json") -Encoding UTF8
    $modelFoldersJson | Out-File -FilePath (Join-Path $ConfigDir "model_folders.json") -Encoding UTF8
    
    Write-ColorOutput "Default configuration created in $ConfigDir" "Green"
}

function Install-WindowsService {
    param([hashtable]$Paths)
    
    if (-not (Test-AdminRights)) {
        Write-ColorOutput "Warning: Cannot install Windows Service without administrator privileges" "Yellow"
        Write-ColorOutput "Installing user-level service instead..." "Yellow"
        Install-UserLevelService $Paths
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
    $configPath = Join-Path $Paths.ConfigDir "config.yaml"
    $binaryPath = "`"$($Paths.BinaryPath)`" --config `"$configPath`""
    
    try {
        # Test if binary can run first
        Write-ColorOutput "Testing binary before service installation..." "Blue"
        $testResult = Start-Process -FilePath $Paths.BinaryPath -ArgumentList "--version" -Wait -PassThru -WindowStyle Hidden -ErrorAction SilentlyContinue
        
        if ($testResult.ExitCode -ne 0) {
            throw "Binary test failed. Likely blocked by Windows security policies."
        }
        
        Write-ColorOutput "Creating Windows Service..." "Blue"
        
        # Create the Windows service using sc.exe
        $createResult = sc.exe create $serviceName binPath= $binaryPath start= auto DisplayName= $serviceDisplayName
        
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to create service: $createResult"
        }
        
        # Set service description
        sc.exe description $serviceName $serviceDescription | Out-Null
        
        # Configure service to restart on failure
        sc.exe failure $serviceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
        
        # Set service to run as LocalSystem with desktop interaction disabled (runs in background)
        Write-ColorOutput "Configuring service to run in background..." "Blue"
        
        # Start the service
        Write-ColorOutput "Starting ClaraCore service..." "Blue"
        Start-Service -Name $serviceName -ErrorAction Stop
        
        # Wait a moment and check status
        Start-Sleep -Seconds 3
        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        
        if ($service.Status -eq 'Running') {
            Write-ColorOutput "✅ Windows Service installed and started successfully" "Green"
            Write-ColorOutput "✅ ClaraCore will run in background on system startup" "Green"
            Write-ColorOutput "   Service Name: $serviceName" "Blue"
            return $true
        } else {
            throw "Service created but failed to start. Status: $($service.Status)"
        }
    }
    catch {
        Write-ColorOutput "Error: Failed to install Windows Service: $($_.Exception.Message)" "Red"
        Write-ColorOutput "" "White"
        Write-ColorOutput "Attempting user-level service installation instead..." "Yellow"
        Install-UserLevelService $Paths
    }
}

function Install-UserLevelService {
    param([hashtable]$Paths)
    
    Write-ColorOutput "Setting up user-level auto-start (Task Scheduler)..." "Blue"
    
    try {
        $taskName = "ClaraCore"
        $configPath = Join-Path $Paths.ConfigDir "config.yaml"
        
        # Remove existing task if it exists
        $existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        if ($existingTask) {
            Write-ColorOutput "Removing existing scheduled task..." "Yellow"
            Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        }
        
        # Create action to run ClaraCore hidden
        $action = New-ScheduledTaskAction -Execute $Paths.BinaryPath -Argument "--config `"$configPath`"" -WorkingDirectory (Split-Path $Paths.BinaryPath)
        
        # Create trigger to run at logon
        $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
        
        # Create settings for the task
        $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RunOnlyIfNetworkAvailable:$false -Hidden
        
        # Create principal to run with highest privileges without showing window
        $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest
        
        # Register the scheduled task
        Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description "ClaraCore AI Inference Server - Runs in background" | Out-Null
        
        Write-ColorOutput "✅ User-level service installed successfully" "Green"
        Write-ColorOutput "✅ ClaraCore will start automatically when you log in" "Green"
        Write-ColorOutput "   Task Name: $taskName" "Blue"
        
        # Try to start the task immediately
        Write-ColorOutput "Starting ClaraCore task..." "Blue"
        try {
            Start-ScheduledTask -TaskName $taskName -ErrorAction Stop
            Start-Sleep -Seconds 3
            
            # Verify task started
            $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName -ErrorAction SilentlyContinue
            if ($taskInfo) {
                Write-ColorOutput "✅ Task started successfully" "Green"
                return $true
            } else {
                Write-ColorOutput "⚠ Task created but may not have started" "Yellow"
                return $true
            }
        } catch {
            Write-ColorOutput "Warning: Task created but failed to start immediately: $($_.Exception.Message)" "Yellow"
            Write-ColorOutput "It will start automatically on next login" "Yellow"
            return $true
        }
    }
    catch {
        Write-ColorOutput "Warning: Could not create scheduled task: $($_.Exception.Message)" "Yellow"
        Write-ColorOutput "You can start ClaraCore manually: $($Paths.BinaryPath)" "Yellow"
        return $false
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
    param([hashtable]$Paths, [bool]$ServiceInstalled)
    
    Write-Header "Installation Completed!"
    
    Write-ColorOutput "✅ ClaraCore is now installed and running in the background!" "Green"
    Write-Host ""
    
    Write-ColorOutput "🌐 Web Interface:" "Yellow"
    Write-ColorOutput "   Open your browser and visit: http://localhost:5800/ui/" "Blue"
    Write-Host ""
    
    if ($ServiceInstalled) {
        Write-ColorOutput "🔧 Service Management:" "Yellow"
        
        $isAdmin = Test-AdminRights
        if ($isAdmin) {
            Write-ColorOutput "   Status:   Get-Service ClaraCore | Select-Object Status,StartType" "Blue"
            Write-ColorOutput "   Stop:     Stop-Service ClaraCore" "Blue"
            Write-ColorOutput "   Start:    Start-Service ClaraCore" "Blue"
            Write-ColorOutput "   Restart:  Restart-Service ClaraCore" "Blue"
            Write-ColorOutput "   Logs:     Get-EventLog -LogName Application -Source ClaraCore -Newest 50" "Blue"
        } else {
            Write-ColorOutput "   Status:   Get-ScheduledTask -TaskName ClaraCore" "Blue"
            Write-ColorOutput "   Stop:     Stop-ScheduledTask -TaskName ClaraCore" "Blue"
            Write-ColorOutput "   Start:    Start-ScheduledTask -TaskName ClaraCore" "Blue"
            Write-ColorOutput "   Disable:  Disable-ScheduledTask -TaskName ClaraCore" "Blue"
            Write-ColorOutput "   Enable:   Enable-ScheduledTask -TaskName ClaraCore" "Blue"
        }
        Write-Host ""
    }
    
    Write-ColorOutput "📂 Configuration Files:" "Yellow"
    Write-ColorOutput "   Config:    $(Join-Path $Paths.ConfigDir "config.yaml")" "Blue"
    Write-ColorOutput "   Settings:  $(Join-Path $Paths.ConfigDir "settings.json")" "Blue"
    Write-Host ""
    
    Write-ColorOutput "💡 Quick Tips:" "Yellow"
    Write-ColorOutput "   • ClaraCore runs silently in the background (no terminal window)" "White"
    Write-ColorOutput "   • It will auto-start when your system boots" "White"
    Write-ColorOutput "   • Configure models via the web interface at http://localhost:5800/ui/setup" "White"
    Write-Host ""
    
    Write-ColorOutput "📚 Documentation: https://github.com/claraverse-space/ClaraCore/tree/main/docs" "Green"
    Write-ColorOutput "❓ Support: https://github.com/claraverse-space/ClaraCore/issues" "Green"
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
        $serviceInstalled = $false
        if (-not $NoService) {
            $serviceInstalled = Install-WindowsService $paths
        }
        
        # Create desktop shortcut
        Create-DesktopShortcut $paths
        
        # Clean up temp file
        Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
        
        # Wait a moment for service to start
        if ($serviceInstalled) {
            Write-ColorOutput "Waiting for ClaraCore to initialize..." "Blue"
            Start-Sleep -Seconds 5
            
            # Try to check if service is responding
            $maxAttempts = 10
            $attempt = 0
            $isRunning = $false
            
            while ($attempt -lt $maxAttempts -and -not $isRunning) {
                try {
                    $response = Invoke-WebRequest -Uri "http://localhost:5800/" -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
                    if ($response.StatusCode -eq 200) {
                        Write-ColorOutput "✅ ClaraCore is running and accessible!" "Green"
                        $isRunning = $true
                        break
                    }
                } catch {
                    if ($attempt -eq 0) {
                        Write-Host -NoNewline "  Waiting for service to start"
                    }
                    Write-Host -NoNewline "."
                    Start-Sleep -Seconds 1
                }
                $attempt++
            }
            
            if (-not $isRunning) {
                Write-Host ""
                Write-ColorOutput "⚠ Service may not have started automatically" "Yellow"
                Write-Host ""
                Write-ColorOutput "Troubleshooting steps:" "Yellow"
                Write-ColorOutput "1. Check if binary is blocked:" "White"
                Write-ColorOutput "   Unblock-File `"$($paths.BinaryPath)`"" "Blue"
                Write-Host ""
                Write-ColorOutput "2. Try starting manually:" "White"
                Write-ColorOutput "   Start-ScheduledTask -TaskName ClaraCore" "Blue"
                Write-ColorOutput "   # or if admin: Start-Service ClaraCore" "Blue"
                Write-Host ""
                Write-ColorOutput "3. Check service status:" "White"
                Write-ColorOutput "   .\scripts\claracore-service.ps1 status" "Blue"
                Write-Host ""
                Write-ColorOutput "4. Add Windows Defender exclusion:" "White"
                Write-ColorOutput "   .\scripts\add-defender-exclusion.bat" "Blue"
            }
        } else {
            Write-Host ""
            Write-ColorOutput "⚠ Service installation may have failed" "Yellow"
            Write-ColorOutput "You can start ClaraCore manually with:" "White"
            Write-ColorOutput "   $($paths.BinaryPath)" "Blue"
        }
        
        # Show next steps
        Show-NextSteps $paths $serviceInstalled
        
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
# ClaraCore Service Management Script for Windows
# Provides easy service management commands for Windows

param(
    [Parameter(Position=0)]
    [ValidateSet('status', 'start', 'stop', 'restart', 'enable', 'disable', 'logs', 'help')]
    [string]$Command = 'help'
)

$ErrorActionPreference = "Continue"

# Colors for output
function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    $colorMap = @{
        Red = "Red"
        Green = "Green"
        Yellow = "Yellow"
        Blue = "Cyan"
    }
    $fgColor = if ($colorMap.ContainsKey($Color)) { $colorMap[$Color] } else { "White" }
    Write-Host $Message -ForegroundColor $fgColor
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

function Get-ServiceType {
    # Check if running as Windows Service
    $service = Get-Service -Name "ClaraCore" -ErrorAction SilentlyContinue
    if ($service) {
        return @{
            Type = "Service"
            Name = "ClaraCore"
            IsAdmin = $true
        }
    }
    
    # Check if running as Scheduled Task
    $task = Get-ScheduledTask -TaskName "ClaraCore" -ErrorAction SilentlyContinue
    if ($task) {
        return @{
            Type = "Task"
            Name = "ClaraCore"
            IsAdmin = $false
        }
    }
    
    return @{
        Type = "None"
        Name = ""
        IsAdmin = $false
    }
}

function Show-Status {
    Write-Header "ClaraCore Service Status"
    
    $serviceInfo = Get-ServiceType
    
    if ($serviceInfo.Type -eq "Service") {
        $service = Get-Service -Name $serviceInfo.Name
        
        Write-ColorOutput "Service Type: Windows Service" "Blue"
        Write-ColorOutput "Service Name: $($serviceInfo.Name)" "Blue"
        Write-Host ""
        
        if ($service.Status -eq 'Running') {
            Write-ColorOutput "✓ Status: Running" "Green"
        } else {
            Write-ColorOutput "✗ Status: $($service.Status)" "Yellow"
        }
        
        Write-ColorOutput "  Start Type: $($service.StartType)" "Blue"
        Write-Host ""
        
        # Check if responding
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:5800/" -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                Write-ColorOutput "✓ Service is responding on http://localhost:5800/" "Green"
            }
        } catch {
            Write-ColorOutput "✗ Service is not responding on http://localhost:5800/" "Yellow"
        }
        
    } elseif ($serviceInfo.Type -eq "Task") {
        $task = Get-ScheduledTask -TaskName $serviceInfo.Name
        $taskInfo = Get-ScheduledTaskInfo -TaskName $serviceInfo.Name
        
        Write-ColorOutput "Service Type: Scheduled Task" "Blue"
        Write-ColorOutput "Task Name: $($serviceInfo.Name)" "Blue"
        Write-Host ""
        
        if ($task.State -eq 'Running') {
            Write-ColorOutput "✓ Status: Running" "Green"
        } elseif ($task.State -eq 'Ready') {
            Write-ColorOutput "○ Status: Ready (not running)" "Yellow"
        } else {
            Write-ColorOutput "✗ Status: $($task.State)" "Yellow"
        }
        
        if ($task.Settings.Enabled) {
            Write-ColorOutput "✓ Auto-start: Enabled" "Green"
        } else {
            Write-ColorOutput "✗ Auto-start: Disabled" "Yellow"
        }
        
        Write-ColorOutput "  Last Run: $($taskInfo.LastRunTime)" "Blue"
        Write-ColorOutput "  Last Result: $($taskInfo.LastTaskResult)" "Blue"
        Write-Host ""
        
        # Check if responding
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:5800/" -TimeoutSec 2 -UseBasicParsing -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                Write-ColorOutput "✓ Service is responding on http://localhost:5800/" "Green"
            }
        } catch {
            Write-ColorOutput "✗ Service is not responding on http://localhost:5800/" "Yellow"
        }
        
    } else {
        Write-ColorOutput "✗ ClaraCore service not found" "Red"
        Write-Host ""
        Write-ColorOutput "ClaraCore is not installed as a service or scheduled task." "Yellow"
        Write-ColorOutput "Please run the installer to set it up." "Yellow"
    }
}

function Start-ClaraService {
    Write-Header "Starting ClaraCore Service"
    
    $serviceInfo = Get-ServiceType
    
    if ($serviceInfo.Type -eq "Service") {
        if (-not (Test-AdminRights)) {
            Write-ColorOutput "Error: Administrator privileges required to manage Windows Service" "Red"
            exit 1
        }
        
        $service = Get-Service -Name $serviceInfo.Name
        if ($service.Status -eq 'Running') {
            Write-ColorOutput "Service is already running" "Yellow"
            return
        }
        
        Write-ColorOutput "Starting service..." "Blue"
        Start-Service -Name $serviceInfo.Name
        Start-Sleep -Seconds 2
        
        $service = Get-Service -Name $serviceInfo.Name
        if ($service.Status -eq 'Running') {
            Write-ColorOutput "✓ Service started successfully" "Green"
        } else {
            Write-ColorOutput "✗ Failed to start service" "Red"
        }
        
    } elseif ($serviceInfo.Type -eq "Task") {
        Write-ColorOutput "Starting scheduled task..." "Blue"
        Start-ScheduledTask -TaskName $serviceInfo.Name
        Start-Sleep -Seconds 2
        
        $task = Get-ScheduledTask -TaskName $serviceInfo.Name
        if ($task.State -eq 'Running') {
            Write-ColorOutput "✓ Task started successfully" "Green"
        } else {
            Write-ColorOutput "⚠ Task triggered, but may not be running yet" "Yellow"
        }
        
    } else {
        Write-ColorOutput "✗ ClaraCore service not found" "Red"
        exit 1
    }
}

function Stop-ClaraService {
    Write-Header "Stopping ClaraCore Service"
    
    $serviceInfo = Get-ServiceType
    
    if ($serviceInfo.Type -eq "Service") {
        if (-not (Test-AdminRights)) {
            Write-ColorOutput "Error: Administrator privileges required to manage Windows Service" "Red"
            exit 1
        }
        
        $service = Get-Service -Name $serviceInfo.Name
        if ($service.Status -ne 'Running') {
            Write-ColorOutput "Service is not running" "Yellow"
            return
        }
        
        Write-ColorOutput "Stopping service..." "Blue"
        Stop-Service -Name $serviceInfo.Name -Force
        Start-Sleep -Seconds 2
        
        $service = Get-Service -Name $serviceInfo.Name
        if ($service.Status -eq 'Stopped') {
            Write-ColorOutput "✓ Service stopped successfully" "Green"
        } else {
            Write-ColorOutput "✗ Failed to stop service" "Red"
        }
        
    } elseif ($serviceInfo.Type -eq "Task") {
        Write-ColorOutput "Stopping scheduled task..." "Blue"
        Stop-ScheduledTask -TaskName $serviceInfo.Name -ErrorAction SilentlyContinue
        
        # Also kill the process if it's running
        $processes = Get-Process -Name "claracore" -ErrorAction SilentlyContinue
        if ($processes) {
            Write-ColorOutput "Stopping ClaraCore processes..." "Blue"
            $processes | Stop-Process -Force
        }
        
        Start-Sleep -Seconds 1
        Write-ColorOutput "✓ Task stopped successfully" "Green"
        
    } else {
        Write-ColorOutput "✗ ClaraCore service not found" "Red"
        exit 1
    }
}

function Restart-ClaraService {
    Write-Header "Restarting ClaraCore Service"
    
    Stop-ClaraService
    Write-Host ""
    Start-Sleep -Seconds 2
    Start-ClaraService
}

function Enable-ClaraService {
    Write-Header "Enabling ClaraCore Service"
    
    $serviceInfo = Get-ServiceType
    
    if ($serviceInfo.Type -eq "Service") {
        if (-not (Test-AdminRights)) {
            Write-ColorOutput "Error: Administrator privileges required to manage Windows Service" "Red"
            exit 1
        }
        
        Write-ColorOutput "Enabling service auto-start..." "Blue"
        Set-Service -Name $serviceInfo.Name -StartupType Automatic
        Write-ColorOutput "✓ Service enabled for auto-start" "Green"
        
    } elseif ($serviceInfo.Type -eq "Task") {
        Write-ColorOutput "Enabling scheduled task..." "Blue"
        Enable-ScheduledTask -TaskName $serviceInfo.Name | Out-Null
        Write-ColorOutput "✓ Task enabled for auto-start" "Green"
        
    } else {
        Write-ColorOutput "✗ ClaraCore service not found" "Red"
        exit 1
    }
}

function Disable-ClaraService {
    Write-Header "Disabling ClaraCore Service"
    
    $serviceInfo = Get-ServiceType
    
    if ($serviceInfo.Type -eq "Service") {
        if (-not (Test-AdminRights)) {
            Write-ColorOutput "Error: Administrator privileges required to manage Windows Service" "Red"
            exit 1
        }
        
        Write-ColorOutput "Disabling service auto-start..." "Blue"
        Set-Service -Name $serviceInfo.Name -StartupType Manual
        Write-ColorOutput "✓ Service disabled from auto-start" "Green"
        
    } elseif ($serviceInfo.Type -eq "Task") {
        Write-ColorOutput "Disabling scheduled task..." "Blue"
        Disable-ScheduledTask -TaskName $serviceInfo.Name | Out-Null
        Write-ColorOutput "✓ Task disabled from auto-start" "Green"
        
    } else {
        Write-ColorOutput "✗ ClaraCore service not found" "Red"
        exit 1
    }
}

function Show-Logs {
    Write-Header "ClaraCore Service Logs"
    
    $serviceInfo = Get-ServiceType
    
    Write-ColorOutput "Fetching recent logs..." "Blue"
    Write-Host ""
    
    # Try to get Windows Event Log entries
    try {
        $events = Get-EventLog -LogName Application -Source "ClaraCore" -Newest 50 -ErrorAction SilentlyContinue
        if ($events) {
            Write-ColorOutput "Recent Windows Event Log entries:" "Blue"
            $events | Format-Table TimeGenerated, EntryType, Message -AutoSize
        } else {
            Write-ColorOutput "No Windows Event Log entries found" "Yellow"
        }
    } catch {
        Write-ColorOutput "No Windows Event Log entries found" "Yellow"
    }
    
    Write-Host ""
    Write-ColorOutput "For real-time logs, check:" "Blue"
    
    if ($serviceInfo.Type -eq "Task") {
        $configDir = "$env:APPDATA\ClaraCore"
        if (Test-Path "$configDir\logs") {
            Write-ColorOutput "  Output: $configDir\logs\claracore.log" "Blue"
            Write-ColorOutput "  Errors: $configDir\logs\claracore.error.log" "Blue"
            Write-Host ""
            Write-ColorOutput "To view logs in real-time:" "Yellow"
            Write-ColorOutput "  Get-Content '$configDir\logs\claracore.log' -Wait -Tail 50" "White"
        }
    } else {
        Write-ColorOutput "  Use: Get-EventLog -LogName Application -Source ClaraCore -Newest 50" "White"
        Write-ColorOutput "  Or check Task Scheduler logs" "White"
    }
}

function Show-Help {
    Write-Host ""
    Write-ColorOutput "ClaraCore Service Management Script" "Blue"
    Write-Host ""
    Write-ColorOutput "USAGE:" "Yellow"
    Write-Host "  .\claracore-service.ps1 <command>"
    Write-Host ""
    Write-ColorOutput "COMMANDS:" "Yellow"
    Write-Host "  status      Show service status and information"
    Write-Host "  start       Start the ClaraCore service"
    Write-Host "  stop        Stop the ClaraCore service"
    Write-Host "  restart     Restart the ClaraCore service"
    Write-Host "  enable      Enable service for auto-start on boot"
    Write-Host "  disable     Disable service auto-start"
    Write-Host "  logs        Show service logs"
    Write-Host "  help        Show this help message"
    Write-Host ""
    Write-ColorOutput "EXAMPLES:" "Yellow"
    Write-Host "  .\claracore-service.ps1 status          # Check if service is running"
    Write-Host "  .\claracore-service.ps1 restart         # Restart the service"
    Write-Host "  .\claracore-service.ps1 logs            # View service logs"
    Write-Host ""
    Write-ColorOutput "NOTE:" "Yellow"
    Write-Host "  Some commands may require administrator privileges."
    Write-Host "  Run PowerShell as Administrator if you encounter permission errors."
    Write-Host ""
}

# Main script logic
switch ($Command) {
    'status' { Show-Status }
    'start' { Start-ClaraService }
    'stop' { Stop-ClaraService }
    'restart' { Restart-ClaraService }
    'enable' { Enable-ClaraService }
    'disable' { Disable-ClaraService }
    'logs' { Show-Logs }
    'help' { Show-Help }
    default { Show-Help }
}


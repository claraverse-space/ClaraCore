@echo off
REM JarvisCore Windows Defender Exclusion Helper
REM Adds JarvisCore to Windows Defender exclusions to prevent false positives

echo.
echo ========================================
echo  JarvisCore Antivirus Helper
echo ========================================
echo.

REM Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script must be run as Administrator
    echo.
    echo Right-click this file and select "Run as administrator"
    echo.
    pause
    exit /b 1
)

echo Detected JarvisCore in current directory...
echo.

REM Get current directory
set "CURRENT_DIR=%cd%"

echo Adding Windows Defender exclusion for:
echo %CURRENT_DIR%
echo.

REM Add exclusion
powershell -Command "Add-MpPreference -ExclusionPath '%CURRENT_DIR%'" >nul 2>&1

if %errorLevel% equ 0 (
    echo [SUCCESS] Exclusion added successfully!
    echo.
    echo JarvisCore is now excluded from Windows Defender scans.
    echo You can now run jarviscore.exe without false positive warnings.
    echo.
) else (
    echo [ERROR] Failed to add exclusion.
    echo.
    echo Troubleshooting:
    echo   1. Make sure Windows Defender is your active antivirus
    echo   2. Check if antivirus is managed by organization policy
    echo   3. Try adding exclusion manually via Windows Security settings
    echo.
)

echo ========================================
echo.
echo To verify JarvisCore is safe:
echo   1. Check SHA256 hash against GitHub release
echo   2. Review source code at github.com/jarvisverse-space/JarvisCore
echo   3. Build from source yourself
echo.
echo For more info, see docs/ANTIVIRUS_FALSE_POSITIVES.md
echo.

pause

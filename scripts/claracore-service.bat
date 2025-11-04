@echo off
REM ClaraCore Service Management Script for Windows (Batch Wrapper)
REM This is a simple wrapper around the PowerShell service management script

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PS_SCRIPT=%SCRIPT_DIR%claracore-service.ps1

REM Check if PowerShell script exists
if not exist "%PS_SCRIPT%" (
    echo Error: PowerShell script not found at %PS_SCRIPT%
    exit /b 1
)

REM Get the command argument
set COMMAND=%1

REM If no command provided, show help
if "%COMMAND%"=="" (
    set COMMAND=help
)

REM Validate command
if not "%COMMAND%"=="status" (
    if not "%COMMAND%"=="start" (
        if not "%COMMAND%"=="stop" (
            if not "%COMMAND%"=="restart" (
                if not "%COMMAND%"=="enable" (
                    if not "%COMMAND%"=="disable" (
                        if not "%COMMAND%"=="logs" (
                            if not "%COMMAND%"=="help" (
                                echo Invalid command: %COMMAND%
                                echo Use 'help' to see available commands
                                exit /b 1
                            )
                        )
                    )
                )
            )
        )
    )
)

REM Run PowerShell script with the command
powershell -ExecutionPolicy Bypass -File "%PS_SCRIPT%" -Command %COMMAND%

endlocal


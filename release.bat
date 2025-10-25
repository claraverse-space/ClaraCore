@echo off
REM ClaraCore Release Helper for Windows

echo ==========================================
echo  ClaraCore Release Manager
echo ==========================================
echo.

REM Check if Python is available
python --version >nul 2>&1
if errorlevel 1 (
    echo Error: Python is not installed or not in PATH
    echo Please install Python 3.7+ from https://python.org
    pause
    exit /b 1
)

REM Check if pip is available
pip --version >nul 2>&1
if errorlevel 1 (
    echo Error: pip is not available
    pause
    exit /b 1
)

REM Install dependencies if needed
echo Checking Python dependencies...
pip show requests >nul 2>&1
if errorlevel 1 (
    echo Installing Python dependencies...
    pip install -r requirements-release.txt
    if errorlevel 1 (
        echo Error: Failed to install dependencies
        pause
        exit /b 1
    )
) else (
    pip show PyGithub >nul 2>&1
    if errorlevel 1 (
        echo Installing Python dependencies...
        pip install -r requirements-release.txt
        if errorlevel 1 (
            echo Error: Failed to install dependencies
            pause
            exit /b 1
        )
    )
)

echo.
echo Dependencies OK!
echo.

REM Get version from user
set /p VERSION="Enter release version (e.g., v0.1.0): "
if "%VERSION%"=="" (
    echo Error: Version cannot be empty
    pause
    exit /b 1
)

REM Get GitHub token
set /p TOKEN_CHOICE="Use token file? (y/N): "
if /i "%TOKEN_CHOICE%"=="y" (
    set /p TOKEN_FILE="Enter token file path (.github_token): "
    if "%TOKEN_FILE%"=="" set TOKEN_FILE=.github_token
    
    if not exist "%TOKEN_FILE%" (
        echo Error: Token file not found: %TOKEN_FILE%
        echo.
        echo Create a GitHub Personal Access Token with 'repo' scope at:
        echo https://github.com/settings/tokens
        echo.
        echo Save it to %TOKEN_FILE% file
        pause
        exit /b 1
    )
    
    set TOKEN_ARG=--token-file "%TOKEN_FILE%"
) else (
    set /p GITHUB_TOKEN="Enter GitHub token (will be hidden): "
    if "%GITHUB_TOKEN%"=="" (
        echo Error: GitHub token cannot be empty
        pause
        exit /b 1
    )
    
    set TOKEN_ARG=--token "%GITHUB_TOKEN%"
)

REM Ask about draft
set /p DRAFT_CHOICE="Create as draft? (y/N): "
if /i "%DRAFT_CHOICE%"=="y" (
    set DRAFT_ARG=--draft
) else (
    set DRAFT_ARG=
)

echo.
echo ==========================================
echo  Creating Release %VERSION%
echo ==========================================
echo.

REM Run the release script
python release.py --version "%VERSION%" %TOKEN_ARG% %DRAFT_ARG%

if errorlevel 1 (
    echo.
    echo ==========================================
    echo  Release Failed!
    echo ==========================================
    pause
    exit /b 1
) else (
    echo.
    echo ==========================================
    echo  Release Created Successfully!
    echo ==========================================
    echo.
    echo Visit: https://github.com/badboysm890/ClaraCore/releases
    pause
)
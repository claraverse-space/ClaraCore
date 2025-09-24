@echo off
setlocal enabledelayedexpansion

echo ============================================================
echo 🚀 ClaraCore Build Script (Windows)
echo ============================================================

:: Check if we're in the right directory
if not exist "go.mod" (
    echo ❌ Please run this script from the ClaraCore root directory
    pause
    exit /b 1
)

:: Build UI
echo.
echo 📦 Building UI (React/TypeScript)
echo ----------------------------------------
cd ui
if not exist "package.json" (
    echo ❌ package.json not found in ui directory!
    cd ..
    pause
    exit /b 1
)

:: Install dependencies if needed
if not exist "node_modules" (
    echo 📦 Installing npm dependencies...
    npm install
    if !errorlevel! neq 0 (
        echo ❌ npm install failed!
        cd ..
        pause
        exit /b 1
    )
)

:: Build UI
echo 🔨 Building UI...
npm run build
if !errorlevel! neq 0 (
    echo ❌ UI build failed!
    cd ..
    pause
    exit /b 1
)

echo ✅ UI build completed successfully
cd ..

:: Build Go backend
echo.
echo 📦 Building ClaraCore (Go Backend)
echo ----------------------------------------

:: Clean previous build
if exist "claracore.exe" (
    echo 🗑️ Removing previous build...
    del "claracore.exe"
)

:: Build Go application
echo 🔨 Building Go application...
go build -o claracore.exe .
if !errorlevel! neq 0 (
    echo ❌ Go build failed!
    pause
    exit /b 1
)

if exist "claracore.exe" (
    echo ✅ ClaraCore executable created successfully
) else (
    echo ❌ ClaraCore executable not found after build!
    pause
    exit /b 1
)

echo.
echo ============================================================
echo 🎉 BUILD SUCCESSFUL!
echo ============================================================
echo 🚀 Ready to run: claracore.exe
echo 🌐 UI will be served at: http://localhost:5800
echo ============================================================

pause
# ClaraCore Build Scripts

This directory contains automated build scripts to build both the UI and Go backend in one command.

## Available Scripts

### 1. Python Script (Recommended)
```bash
python build.py
```
**Features:**
- ✅ Cross-platform (Windows, macOS, Linux)
- ✅ Real-time output display
- ✅ Dependency checking
- ✅ Build time reporting
- ✅ File size reporting
- ✅ Error handling with detailed messages

### 2. PowerShell Script (Windows)
```powershell
.\build.ps1
```
**Features:**
- ✅ Windows PowerShell native
- ✅ Colored output
- ✅ Clean build option: `.\build.ps1 -Clean`
- ✅ Verbose mode: `.\build.ps1 -Verbose`

### 3. Batch File (Windows Legacy)
```batch
build.bat
```
**Features:**
- ✅ Works on any Windows system
- ✅ No external dependencies
- ✅ Simple double-click execution

## What the Scripts Do

1. **Dependency Check**: Verifies npm and Go are installed
2. **UI Build**: 
   - Installs npm dependencies (if needed)
   - Runs `npm run build` in the `ui/` directory
   - Outputs to `proxy/ui_dist/`
3. **Go Build**:
   - Cleans previous `claracore.exe` (if exists)
   - Runs `go build -o claracore.exe .`
   - Reports executable size

## Requirements

- **Node.js** (v16+ recommended)
- **Go** (v1.19+ recommended)
- **Python 3.6+** (for Python script only)

## Usage Examples

### Quick Build
```bash
# Using Python (recommended)
python build.py

# Using PowerShell
.\build.ps1

# Using Batch
build.bat
```

### Clean Build (PowerShell only)
```powershell
.\build.ps1 -Clean
```

### After Build
```bash
# Run ClaraCore
.\claracore.exe

# Access UI at: http://localhost:5800
```

## Troubleshooting

### Common Issues

1. **"npm not found"**
   - Install Node.js from https://nodejs.org/

2. **"go not found"**
   - Install Go from https://golang.org/dl/

3. **"Please run from ClaraCore root directory"**
   - Make sure you're in the directory containing `go.mod`

4. **Permission denied (PowerShell)**
   ```powershell
   Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
   ```

### Build Logs

All scripts show real-time output, so you can see exactly what's happening during the build process.

## Performance

Typical build times:
- **UI Build**: ~2-4 seconds
- **Go Build**: ~1-3 seconds
- **Total**: ~3-7 seconds

The Python script also reports exact build times for monitoring performance.
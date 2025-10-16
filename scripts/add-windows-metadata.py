#!/usr/bin/env python3
"""
Add Windows metadata to ClaraCore executable
This helps reduce false positives from antivirus software
"""

import os
import sys
import subprocess
from pathlib import Path

def compile_resource_file():
    """Compile the .rc file to .syso for embedding in Go binary"""
    print("🔨 Compiling Windows resource file...")
    
    # Check if windres is available (part of mingw)
    try:
        subprocess.run(["windres", "--version"], capture_output=True, check=True)
    except (subprocess.CalledProcessError, FileNotFoundError):
        print("⚠️  windres not found. Install mingw-w64:")
        print("   - Windows: choco install mingw")
        print("   - Or download from: https://www.mingw-w64.org/")
        return False
    
    rc_file = Path("claracore.rc")
    syso_file = Path("claracore_windows.syso")
    
    if not rc_file.exists():
        print(f"❌ Resource file not found: {rc_file}")
        return False
    
    # Compile .rc to .syso
    cmd = [
        "windres",
        "-i", str(rc_file),
        "-o", str(syso_file),
        "-O", "coff"
    ]
    
    try:
        subprocess.run(cmd, check=True)
        print(f"✅ Created: {syso_file}")
        print("   This file will be automatically embedded when building on Windows")
        return True
    except subprocess.CalledProcessError as e:
        print(f"❌ Failed to compile resource file: {e}")
        return False

def main():
    print("=" * 60)
    print("🛡️  ClaraCore Windows Metadata Builder")
    print("=" * 60)
    print()
    
    if sys.platform != "win32":
        print("⚠️  This script is designed for Windows")
        print("   Resource files are only needed for Windows builds")
        sys.exit(0)
    
    success = compile_resource_file()
    
    print()
    print("=" * 60)
    if success:
        print("✅ Metadata compilation successful!")
        print()
        print("Next steps:")
        print("1. Build your binary normally: go build")
        print("2. The .syso file will be automatically embedded")
        print("3. Your exe will now have proper metadata visible in Windows Explorer")
    else:
        print("⚠️  Metadata compilation failed")
        print("   Your binary will still work, but without Windows metadata")
    print("=" * 60)
    
    return 0 if success else 1

if __name__ == "__main__":
    sys.exit(main())

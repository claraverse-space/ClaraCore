#!/usr/bin/env python3
"""
ClaraCore Build Script
Builds both UI and Go backend in sequence
"""

import os
import sys
import subprocess
import time
from pathlib import Path

def print_banner():
    """Print build script banner"""
    print("=" * 60)
    print("🚀 ClaraCore Build Script")
    print("=" * 60)

def print_step(step_name):
    """Print build step header"""
    print(f"\n📦 {step_name}")
    print("-" * 40)

def run_command(command, cwd=None, shell=True):
    """Run a command and return success status"""
    try:
        print(f"💻 Running: {command}")
        if cwd:
            print(f"📁 Directory: {cwd}")
        
        # Use shell=True on Windows for proper command execution
        result = subprocess.run(
            command, 
            cwd=cwd, 
            shell=shell, 
            check=True,
            capture_output=False,  # Show output in real-time
            text=True
        )
        
        print(f"✅ Command completed successfully")
        return True
        
    except subprocess.CalledProcessError as e:
        print(f"❌ Command failed with exit code: {e.returncode}")
        return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

def build_ui():
    """Build the UI using npm"""
    print_step("Building UI (React/TypeScript)")
    
    ui_dir = Path("ui")
    if not ui_dir.exists():
        print("❌ UI directory not found!")
        return False
    
    # Check if package.json exists
    package_json = ui_dir / "package.json"
    if not package_json.exists():
        print("❌ package.json not found in ui directory!")
        return False
    
    # Install dependencies if node_modules doesn't exist
    node_modules = ui_dir / "node_modules"
    if not node_modules.exists():
        print("📦 Installing npm dependencies...")
        if not run_command("npm install", cwd=ui_dir):
            return False
    
    # Build the UI
    print("🔨 Building UI...")
    if not run_command("npm run build", cwd=ui_dir):
        return False
    
    # Check if build output exists
    build_output = Path("proxy/ui_dist")
    if build_output.exists():
        print(f"✅ UI build output created at: {build_output.absolute()}")
    else:
        print("⚠️  UI build completed but output directory not found")
    
    return True

def build_go():
    """Build the Go backend"""
    print_step("Building ClaraCore (Go Backend)")
    
    # Check if go.mod exists
    if not Path("go.mod").exists():
        print("❌ go.mod not found! Are you in the ClaraCore root directory?")
        return False
    
    # Clean previous build
    if Path("claracore.exe").exists():
        print("🗑️  Removing previous build...")
        try:
            os.remove("claracore.exe")
        except Exception as e:
            print(f"⚠️  Could not remove previous build: {e}")
    
    # Build Go application
    print("🔨 Building Go application...")
    if not run_command("go build -o claracore.exe ."):
        return False
    
    # Check if executable was created
    if Path("claracore.exe").exists():
        print("✅ ClaraCore executable created successfully")
        
        # Get file size
        size = Path("claracore.exe").stat().st_size
        size_mb = size / (1024 * 1024)
        print(f"📊 Executable size: {size_mb:.1f} MB")
    else:
        print("❌ ClaraCore executable not found after build!")
        return False
    
    return True

def check_dependencies():
    """Check if required tools are available"""
    print_step("Checking Dependencies")
    
    # Check Node.js/npm
    try:
        result = subprocess.run(["npm", "--version"], capture_output=True, text=True, shell=True)
        if result.returncode == 0:
            print(f"✅ npm v{result.stdout.strip()} found")
        else:
            print("❌ npm not found! Please install Node.js")
            return False
    except Exception:
        print("❌ npm not found! Please install Node.js")
        return False
    
    # Check Go
    try:
        result = subprocess.run(["go", "version"], capture_output=True, text=True, shell=True)
        if result.returncode == 0:
            version_line = result.stdout.strip()
            print(f"✅ {version_line}")
        else:
            print("❌ Go not found! Please install Go")
            return False
    except Exception:
        print("❌ Go not found! Please install Go")
        return False
    
    return True

def main():
    """Main build function"""
    start_time = time.time()
    
    print_banner()
    
    # Check if we're in the right directory
    if not Path("claracore.go").exists() and not Path("go.mod").exists():
        print("❌ Please run this script from the ClaraCore root directory")
        sys.exit(1)
    
    # Check dependencies
    if not check_dependencies():
        print("\n❌ Build failed: Missing dependencies")
        sys.exit(1)
    
    # Build UI
    if not build_ui():
        print("\n❌ Build failed: UI build error")
        sys.exit(1)
    
    # Build Go backend
    if not build_go():
        print("\n❌ Build failed: Go build error")
        sys.exit(1)
    
    # Success!
    end_time = time.time()
    build_time = end_time - start_time
    
    print("\n" + "=" * 60)
    print("🎉 BUILD SUCCESSFUL!")
    print("=" * 60)
    print(f"⏱️  Total build time: {build_time:.2f} seconds")
    print("🚀 Ready to run: ./claracore.exe")
    print("🌐 UI will be served at: http://localhost:5800")
    print("=" * 60)

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n⚠️  Build interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}")
        sys.exit(1)
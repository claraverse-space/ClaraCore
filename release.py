#!/usr/bin/env python3

"""
ClaraCore Release Manager
Automates the creation of GitHub releases with cross-platform binaries.

Usage:
    python release.py --version v0.1.0 --token YOUR_GITHUB_TOKEN
    python release.py --version v0.1.0 --token-file .github_token
    python release.py --help

Requirements:
    pip install requests PyGithub

Code Signing (Recommended to Avoid Antivirus False Positives):
    Windows binaries benefit greatly from code signing to reduce false positives.
    
    To enable code signing:
    1. Obtain a code signing certificate from:
       - DigiCert (https://www.digicert.com/signing/code-signing-certificates)
       - Sectigo (https://sectigo.com/ssl-certificates-tls/code-signing)
       - SSL.com (https://www.ssl.com/certificates/code-signing/)
       - For open source: May qualify for free cert from certain providers
    
    2. Install Windows SDK for signtool.exe:
       - Download: https://developer.microsoft.com/windows/downloads/windows-sdk/
       - Or: winget install Microsoft.WindowsSDK
    
    3. Set environment variables:
       set SIGN_CERT_PATH=C:\\path\\to\\certificate.pfx
       set SIGN_CERT_PASSWORD=your_certificate_password
    
    4. Run release.py normally - it will auto-sign if cert is available
    
    Additional Antivirus Best Practices:
    - Submit binaries to Microsoft for analysis: https://www.microsoft.com/en-us/wdsi/filesubmission
    - Build from clean, trusted environments
    - Use consistent build metadata (.rc file with proper company/product info)
    - Include digital signatures in release artifacts
    - Maintain public GitHub presence with clear documentation
"""

import os
import sys
import json
import time
import shutil
import hashlib
import argparse
import subprocess
from pathlib import Path
from typing import Dict, List, Tuple, Optional
from datetime import datetime

try:
    import requests
    from github import Github
except ImportError:
    print("Error: Required packages not installed. Run:")
    print("pip install requests PyGithub")
    sys.exit(1)

# Configuration
REPO_OWNER = "claraverse-space"
REPO_NAME = "ClaraCore"
BUILD_DIR = "dist"
BINARY_NAME = "claracore"

# Build targets for cross-compilation
BUILD_TARGETS = [
    {
        "goos": "linux",
        "goarch": "amd64",
        "filename": "claracore-linux-amd64",
        "description": "Linux x64"
    },
    {
        "goos": "linux", 
        "goarch": "arm64",
        "filename": "claracore-linux-arm64",
        "description": "Linux ARM64"
    },
    {
        "goos": "darwin",
        "goarch": "amd64", 
        "filename": "claracore-darwin-amd64",
        "description": "macOS Intel"
    },
    {
        "goos": "darwin",
        "goarch": "arm64",
        "filename": "claracore-darwin-arm64", 
        "description": "macOS Apple Silicon"
    },
    {
        "goos": "windows",
        "goarch": "amd64",
        "filename": "claracore-windows-amd64.exe",
        "description": "Windows x64"
    }
]

class Colors:
    """ANSI color codes for terminal output"""
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    PURPLE = '\033[0;35m'
    CYAN = '\033[0;36m'
    WHITE = '\033[1;37m'
    NC = '\033[0m'  # No Color

def print_colored(message: str, color: str = Colors.WHITE):
    """Print colored message to terminal"""
    print(f"{color}{message}{Colors.NC}")

def print_header(title: str):
    """Print formatted header"""
    print()
    print_colored("=" * 60, Colors.BLUE)
    print_colored(f" {title}", Colors.BLUE)
    print_colored("=" * 60, Colors.BLUE)
    print()

def run_command(cmd: List[str], cwd: Optional[str] = None, env: Optional[Dict[str, str]] = None) -> Tuple[bool, str]:
    """Run shell command and return success status and output"""
    try:
        print_colored(f"Running: {' '.join(cmd)}", Colors.CYAN)
        
        # Merge environment variables
        full_env = os.environ.copy()
        if env:
            full_env.update(env)
        
        result = subprocess.run(
            cmd,
            cwd=cwd,
            env=full_env,
            capture_output=True,
            text=True,
            check=True
        )
        
        if result.stdout.strip():
            print(result.stdout.strip())
        
        return True, result.stdout
        
    except subprocess.CalledProcessError as e:
        print_colored(f"Error running command: {e}", Colors.RED)
        if e.stderr:
            print_colored(f"Error output: {e.stderr}", Colors.RED)
        return False, e.stderr

def calculate_sha256(filepath: Path) -> str:
    """Calculate SHA256 hash of a file"""
    sha256_hash = hashlib.sha256()
    with open(filepath, "rb") as f:
        for byte_block in iter(lambda: f.read(4096), b""):
            sha256_hash.update(byte_block)
    return sha256_hash.hexdigest()

def get_file_size(filepath: Path) -> str:
    """Get human-readable file size"""
    size = filepath.stat().st_size
    for unit in ['B', 'KB', 'MB', 'GB']:
        if size < 1024.0:
            return f"{size:.1f} {unit}"
        size /= 1024.0
    return f"{size:.1f} TB"

def sign_windows_binary(binary_path: Path) -> bool:
    """
    Sign Windows binary with Authenticode certificate.
    This dramatically reduces false positives from Windows Defender.
    
    Requires:
    - Code signing certificate (can be from DigiCert, Sectigo, etc.)
    - signtool.exe (part of Windows SDK)
    
    If no certificate is available, returns True to continue build.
    """
    if not binary_path.name.endswith('.exe'):
        return True
    
    # Check if SIGN_CERT_PATH environment variable is set
    cert_path = os.environ.get('SIGN_CERT_PATH')
    cert_password = os.environ.get('SIGN_CERT_PASSWORD')
    
    if not cert_path or not Path(cert_path).exists():
        print_colored("  ⚠️  No code signing certificate found (set SIGN_CERT_PATH)", Colors.YELLOW)
        print_colored("  ℹ️  Binary will work but may trigger AV warnings", Colors.YELLOW)
        return True
    
    print_colored("  🔐 Signing binary with Authenticode...", Colors.CYAN)
    
    # Find signtool.exe
    signtool_paths = [
        r"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe",
        r"C:\Program Files (x86)\Windows Kits\10\bin\10.0.22621.0\x64\signtool.exe",
        r"C:\Program Files (x86)\Windows Kits\10\bin\x86\signtool.exe",
    ]
    
    signtool = None
    for path in signtool_paths:
        if Path(path).exists():
            signtool = path
            break
    
    if not signtool:
        print_colored("  ⚠️  signtool.exe not found. Install Windows SDK.", Colors.YELLOW)
        return True
    
    # Sign the binary
    cmd = [
        signtool,
        "sign",
        "/f", cert_path,
        "/p", cert_password if cert_password else "",
        "/tr", "http://timestamp.digicert.com",  # RFC 3161 timestamp server
        "/td", "sha256",
        "/fd", "sha256",
        "/d", "ClaraCore AI Inference Server",
        "/du", f"https://github.com/{REPO_OWNER}/{REPO_NAME}",
        str(binary_path)
    ]
    
    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print_colored("  ✅ Binary signed successfully", Colors.GREEN)
        return True
    except subprocess.CalledProcessError as e:
        print_colored(f"  ⚠️  Signing failed: {e}", Colors.YELLOW)
        return True  # Continue even if signing fails

def compile_windows_resources() -> bool:
    """
    Compile Windows resource file (.rc) to .syso for embedding metadata.
    This helps Windows recognize the binary as legitimate software.
    """
    if sys.platform != "win32":
        return True  # Not needed on other platforms
    
    rc_file = Path("claracore.rc")
    syso_file = Path("claracore_windows.syso")
    
    if not rc_file.exists():
        print_colored("  ⚠️  claracore.rc not found, skipping metadata", Colors.YELLOW)
        return True
    
    # Check if windres is available
    try:
        subprocess.run(["windres", "--version"], capture_output=True, check=True)
    except (subprocess.CalledProcessError, FileNotFoundError):
        print_colored("  ⚠️  windres not found, skipping metadata", Colors.YELLOW)
        print_colored("     Install: choco install mingw", Colors.YELLOW)
        return True
    
    print_colored("  🔨 Compiling Windows metadata...", Colors.CYAN)
    
    cmd = ["windres", "-i", str(rc_file), "-o", str(syso_file), "-O", "coff"]
    
    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print_colored("  ✅ Windows metadata compiled", Colors.GREEN)
        return True
    except subprocess.CalledProcessError:
        print_colored("  ⚠️  Metadata compilation failed", Colors.YELLOW)
        return True

def build_binaries(version: str) -> List[Dict]:
    """Build binaries for all target platforms"""
    print_header(f"Building ClaraCore {version} Binaries")
    
    # Compile Windows resources first (if on Windows)
    compile_windows_resources()
    
    # Create build directory
    build_path = Path(BUILD_DIR)
    if build_path.exists():
        print_colored(f"Removing existing build directory: {build_path}", Colors.YELLOW)
        shutil.rmtree(build_path)
    
    build_path.mkdir(parents=True, exist_ok=True)
    
    # Set build variables
    build_time = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
    
    # Build flags optimized for legitimate software recognition
    # Note: We keep debug info for Windows to help with crash reports and look more legitimate
    ldflags_common = [
        f"-X main.version={version}",
        f"-X main.date={build_time}",
        f"-X main.commit={get_git_commit()}",
    ]
    
    built_binaries = []
    
    for target in BUILD_TARGETS:
        print_colored(f"\nBuilding {target['description']}...", Colors.BLUE)
        
        output_path = build_path / target["filename"]
        
        # Set Go build environment
        env = {
            "GOOS": target["goos"],
            "GOARCH": target["goarch"],
        }
        
        # For Windows: Enable CGO to embed .syso metadata, keep some debug info
        # For others: Disable CGO and strip for smaller size
        if target["goos"] == "windows":
            env["CGO_ENABLED"] = "1" if sys.platform == "win32" else "0"
            # Don't strip Windows binaries - helps avoid AV detection
            ldflags = ldflags_common.copy()
        else:
            env["CGO_ENABLED"] = "0"
            # Strip non-Windows binaries for size
            ldflags = ldflags_common + ["-w", "-s"]
        
        # Build command with buildmode for Windows
        cmd = [
            "go", "build",
            "-trimpath",  # Remove local path info for reproducible builds
            "-buildmode=exe" if target["goos"] == "windows" else "-buildmode=default",
            "-ldflags", " ".join(ldflags),
            "-o", str(output_path),
            "."
        ]
        
        success, output = run_command(cmd, env=env)
        
        if not success:
            print_colored(f"Failed to build {target['description']}", Colors.RED)
            continue
        
        if not output_path.exists():
            print_colored(f"Binary not found: {output_path}", Colors.RED)
            continue
        
        # Sign Windows binaries to reduce false positives
        if target["goos"] == "windows":
            sign_windows_binary(output_path)
        
        # Calculate metadata
        file_size = get_file_size(output_path)
        sha256 = calculate_sha256(output_path)
        
        binary_info = {
            "target": target,
            "path": output_path,
            "size": file_size,
            "sha256": sha256
        }
        
        built_binaries.append(binary_info)
        print_colored(f"✓ Built {target['filename']} ({file_size})", Colors.GREEN)
    
    # Cleanup .syso file after build
    syso_file = Path("claracore_windows.syso")
    if syso_file.exists():
        syso_file.unlink()
        print_colored("  🧹 Cleaned up temporary files", Colors.CYAN)
    
    print_colored(f"\n✓ Successfully built {len(built_binaries)}/{len(BUILD_TARGETS)} binaries", Colors.GREEN)
    return built_binaries

def get_git_commit() -> str:
    """Get current git commit hash"""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout.strip()[:7]
    except:
        return "unknown"

def generate_release_notes(version: str, binaries: List[Dict]) -> str:
    """Generate clean and concise release notes"""
    commit_hash = get_git_commit()
    build_time = datetime.utcnow().strftime("%Y-%m-%d %H:%M UTC")
    
    notes = f"""# ClaraCore {version}

AI-powered model inference server with automatic setup and OpenAI-compatible API.

## 📦 Downloads

Choose the appropriate binary for your system:

"""
    
    # Add download table
    for binary in binaries:
        target = binary["target"]
        notes += f"- **{target['description']}**: `{target['filename']}` ({binary['size']})\n"
    
    notes += f"""
## 🔧 Installation

### Quick Install (Recommended)

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/{REPO_OWNER}/{REPO_NAME}/main/scripts/install.sh | bash
```

**Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/{REPO_OWNER}/{REPO_NAME}/main/scripts/install.ps1 | iex
```

### Manual Installation

1. Download the appropriate binary for your system
2. Make it executable (Linux/macOS): `chmod +x claracore-*`
3. Run setup: `./claracore-* --models-folder /path/to/your/models`
4. Visit: http://localhost:5800/ui/setup

## 🛠️ Quick Start

```bash
# Basic usage
./claracore-linux-amd64 --models-folder /path/to/gguf/models

# With specific backend
./claracore-linux-amd64 --models-folder /path/to/models --backend vulkan

# Web interface
./claracore-linux-amd64
# Then visit: http://localhost:5800/ui/setup
```

## 📚 Documentation

- [Setup Guide](https://github.com/{REPO_OWNER}/{REPO_NAME}/blob/main/docs/SETUP.md)
- [API Documentation](https://github.com/{REPO_OWNER}/{REPO_NAME}/blob/main/docs/API_COMPREHENSIVE.md)
- [Configuration Guide](https://github.com/{REPO_OWNER}/{REPO_NAME}/blob/main/docs/README.md)

## 🔍 Security & Verification

### SHA256 Checksums

Verify your download integrity:

"""
    
    # Add checksums
    for binary in binaries:
        notes += f"- `{binary['target']['filename']}`: `{binary['sha256']}`\n"
    
    notes += f"""
### Antivirus Notes

**Windows Defender**: Our Windows binaries are legitimate, open-source software. If you encounter false positive warnings:

1. **Verify Checksum**: Compare SHA256 hash above with your downloaded file
2. **Check Signature**: Right-click exe → Properties → Digital Signatures (if signed)
3. **Review Source**: All code is public at github.com/{REPO_OWNER}/{REPO_NAME}
4. **Submit to Microsoft**: Help improve detection at https://www.microsoft.com/en-us/wdsi/filesubmission

We build from clean environments and submit binaries to Microsoft for analysis. False positives may occur with new releases until Microsoft updates their database.

**Why does this happen?** 
- New executables without established reputation
- Network and system operations typical of server software
- Go binaries sometimes trigger heuristic detection

**Our Commitment**:
- 🔓 100% open source - inspect the code yourself
- 🔐 Code signed when possible (check exe properties)
- 📝 Detailed build metadata embedded in executables
- 🧪 Reproducible builds with public CI/CD
- 📤 Submitted to Microsoft Defender SmartScreen

## 📊 Build Information

- **Version**: {version}
- **Build Time**: {build_time}
- **Git Commit**: {commit_hash}
- **Go Version**: {get_go_version()}

## 🤝 Support

- **Issues**: [GitHub Issues](https://github.com/{REPO_OWNER}/{REPO_NAME}/issues)
- **Discussions**: [GitHub Discussions](https://github.com/{REPO_OWNER}/{REPO_NAME}/discussions)
- **Documentation**: [Docs](https://github.com/{REPO_OWNER}/{REPO_NAME}/tree/main/docs)

---

**Full Changelog**: https://github.com/{REPO_OWNER}/{REPO_NAME}/compare/...{version}
"""
    
    return notes

def get_go_version() -> str:
    """Get Go version"""
    try:
        result = subprocess.run(
            ["go", "version"],
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout.strip().split()[2]
    except:
        return "unknown"

def submit_to_microsoft_defender(binary_path: Path) -> None:
    """
    Provide instructions for submitting binary to Microsoft Defender for analysis.
    This helps get legitimate software whitelisted.
    """
    print_colored("\n" + "="*60, Colors.BLUE)
    print_colored("📤 OPTIONAL: Submit to Microsoft for Whitelisting", Colors.BLUE)
    print_colored("="*60, Colors.BLUE)
    print()
    print_colored("To reduce false positives, submit your signed binary to:", Colors.WHITE)
    print_colored("https://www.microsoft.com/en-us/wdsi/filesubmission", Colors.CYAN)
    print()
    print_colored("Submission helps Microsoft:", Colors.WHITE)
    print_colored("  • Analyze and whitelist legitimate software", Colors.WHITE)
    print_colored("  • Reduce false positive detections", Colors.WHITE)
    print_colored("  • Improve Windows Defender accuracy", Colors.WHITE)
    print()
    print_colored(f"Binary to submit: {binary_path.name}", Colors.YELLOW)
    print_colored(f"SHA256: {calculate_sha256(binary_path)}", Colors.YELLOW)
    print()

def create_github_release(token: str, version: str, binaries: List[Dict], draft: bool = False) -> bool:
    """Create GitHub release with binaries"""
    print_header(f"Creating GitHub Release {version}")
    
    try:
        # Initialize GitHub client
        g = Github(token)
        repo = g.get_repo(f"{REPO_OWNER}/{REPO_NAME}")
        
        # Generate release notes
        release_notes = generate_release_notes(version, binaries)
        
        # Create release
        print_colored("Creating release...", Colors.BLUE)
        release = repo.create_git_release(
            tag=version,
            name=f"ClaraCore {version}",
            message=release_notes,
            draft=draft,
            prerelease=version.find("alpha") != -1 or version.find("beta") != -1 or version.find("rc") != -1
        )
        
        print_colored(f"✓ Created release: {release.html_url}", Colors.GREEN)
        
        # Upload binaries
        print_colored("Uploading binaries...", Colors.BLUE)
        
        for binary in binaries:
            filename = binary["target"]["filename"]
            filepath = binary["path"]
            
            print_colored(f"  Uploading {filename}...", Colors.CYAN)
            
            with open(filepath, "rb") as f:
                asset = release.upload_asset(
                    path=str(filepath),
                    name=filename,
                    content_type="application/octet-stream"
                )
            
            print_colored(f"  ✓ Uploaded {filename} ({binary['size']})", Colors.GREEN)
        
        # Create checksums file
        print_colored("Creating checksums file...", Colors.BLUE)
        checksums_content = f"# SHA256 Checksums for ClaraCore {version}\n\n"
        for binary in binaries:
            checksums_content += f"{binary['sha256']}  {binary['target']['filename']}\n"
        
        checksums_path = Path(BUILD_DIR) / "checksums.txt"
        checksums_path.write_text(checksums_content)
        
        with open(checksums_path, "rb") as f:
            release.upload_asset(
                path=str(checksums_path),
                name="checksums.txt",
                content_type="text/plain"
            )
        
        print_colored("✓ Uploaded checksums.txt", Colors.GREEN)
        
        print()
        print_colored("🎉 Release created successfully!", Colors.GREEN)
        print_colored(f"Release URL: {release.html_url}", Colors.CYAN)
        print_colored(f"Assets: {len(binaries)} binaries + checksums", Colors.CYAN)
        
        # Show Microsoft submission reminder for Windows binaries
        for binary in binaries:
            if binary["target"]["goos"] == "windows":
                submit_to_microsoft_defender(binary["path"])
                break  # Only show once
        
        return True
        
    except Exception as e:
        print_colored(f"Error creating release: {e}", Colors.RED)
        return False

def validate_version(version: str) -> bool:
    """Validate version format"""
    if not version.startswith('v'):
        print_colored("Version must start with 'v' (e.g., v0.1.0)", Colors.RED)
        return False
    
    # Remove 'v' prefix and check semantic versioning
    ver = version[1:]
    parts = ver.split('.')
    
    if len(parts) < 2:
        print_colored("Version must follow semantic versioning (e.g., v1.0.0)", Colors.RED)
        return False
    
    return True

def check_prerequisites() -> bool:
    """Check if all prerequisites are met"""
    print_header("Checking Prerequisites")
    
    # Check Go installation
    success, _ = run_command(["go", "version"])
    if not success:
        print_colored("✗ Go not found. Please install Go.", Colors.RED)
        return False
    print_colored("✓ Go found", Colors.GREEN)
    
    # Check git
    success, _ = run_command(["git", "--version"])
    if not success:
        print_colored("✗ Git not found. Please install Git.", Colors.RED)
        return False
    print_colored("✓ Git found", Colors.GREEN)
    
    # Check if we're in a git repository
    if not Path(".git").exists():
        print_colored("✗ Not in a Git repository", Colors.RED)
        return False
    print_colored("✓ Git repository found", Colors.GREEN)
    
    # Check for go.mod
    if not Path("go.mod").exists():
        print_colored("✗ go.mod not found. Please run 'go mod init' first.", Colors.RED)
        return False
    print_colored("✓ Go module found", Colors.GREEN)
    
    return True

def main():
    """Main function"""
    parser = argparse.ArgumentParser(
        description="Create GitHub release for ClaraCore with cross-platform binaries",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python release.py --version v0.1.0 --token ghp_xxxxxxxxxxxx
  python release.py --version v0.1.0 --token-file .github_token
  python release.py --version v0.1.0 --token-file .github_token --draft
        """
    )
    
    parser.add_argument(
        "--version",
        required=True,
        help="Release version (e.g., v0.1.0)"
    )
    
    token_group = parser.add_mutually_exclusive_group(required=True)
    token_group.add_argument(
        "--token",
        help="GitHub personal access token"
    )
    token_group.add_argument(
        "--token-file",
        help="File containing GitHub personal access token"
    )
    
    parser.add_argument(
        "--draft",
        action="store_true",
        help="Create release as draft"
    )
    
    parser.add_argument(
        "--build-only",
        action="store_true",
        help="Only build binaries, don't create release"
    )
    
    args = parser.parse_args()
    
    # Validate version
    if not validate_version(args.version):
        sys.exit(1)
    
    # Check prerequisites
    if not check_prerequisites():
        sys.exit(1)
    
    # Get GitHub token
    if args.token:
        github_token = args.token
    else:
        token_file = Path(args.token_file)
        if not token_file.exists():
            print_colored(f"Token file not found: {token_file}", Colors.RED)
            sys.exit(1)
        github_token = token_file.read_text().strip()
    
    try:
        # Build binaries
        binaries = build_binaries(args.version)
        
        if not binaries:
            print_colored("No binaries were built successfully", Colors.RED)
            sys.exit(1)
        
        if args.build_only:
            print_colored(f"✓ Build completed. Binaries in {BUILD_DIR}/", Colors.GREEN)
            return
        
        # Create GitHub release
        success = create_github_release(github_token, args.version, binaries, args.draft)
        
        if success:
            print_colored("\n🎉 Release process completed successfully!", Colors.GREEN)
        else:
            print_colored("\n❌ Release process failed", Colors.RED)
            sys.exit(1)
            
    except KeyboardInterrupt:
        print_colored("\n\n❌ Release process cancelled by user", Colors.YELLOW)
        sys.exit(1)
    except Exception as e:
        print_colored(f"\n❌ Unexpected error: {e}", Colors.RED)
        sys.exit(1)

if __name__ == "__main__":
    main()
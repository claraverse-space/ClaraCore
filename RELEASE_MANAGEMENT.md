# ClaraCore Release Management

This directory contains automated release management tools for ClaraCore.

## 🚀 Quick Release

### Windows
```cmd
.\release.bat
```

### Linux/macOS
```bash
chmod +x release.sh
./release.sh
```

### Manual
```bash
# Install dependencies
pip install -r requirements-release.txt

# Create release
python release.py --version v0.1.0 --token-file .github_token
```

## 📁 Files

- **`release.py`** - Main release automation script
- **`release.bat`** - Windows helper script with interactive prompts
- **`release.sh`** - Linux/macOS helper script with interactive prompts
- **`requirements-release.txt`** - Python dependencies
- **`GITHUB_TOKEN_SETUP.md`** - GitHub token creation guide

## 🔧 Features

### Cross-Platform Builds
Automatically builds binaries for:
- Linux x64 (`claracore-linux-amd64`)
- Linux ARM64 (`claracore-linux-arm64`)
- macOS Intel (`claracore-darwin-amd64`)
- macOS Apple Silicon (`claracore-darwin-arm64`)
- Windows x64 (`claracore-windows-amd64.exe`)

### Release Automation
- Creates GitHub release with proper versioning
- Uploads all platform binaries
- Generates comprehensive release notes
- Creates SHA256 checksums file
- Supports draft releases for testing

### Build Information
Each binary includes:
- Version information
- Build timestamp
- Git commit hash
- Optimized binary size (stripped debug info)

## 📋 Prerequisites

1. **Go 1.19+** - For cross-compilation
2. **Python 3.7+** - For the release script
3. **Git** - For version information
4. **GitHub Token** - With `repo` scope access

## 🛠️ Usage Examples

### Create v0.1.0 Release
```bash
python release.py --version v0.1.0 --token-file .github_token
```

### Create Draft Release
```bash
python release.py --version v0.1.0 --token-file .github_token --draft
```

### Build Only (No Release)
```bash
python release.py --version v0.1.0 --token dummy --build-only
```

### Using Direct Token
```bash
python release.py --version v0.1.0 --token ghp_your_token_here
```

## 🔐 GitHub Token Setup

1. Go to [GitHub Settings > Personal Access Tokens](https://github.com/settings/tokens)
2. Create new token with `repo` scope
3. Save to `.github_token` file:
   ```bash
   echo "ghp_your_token_here" > .github_token
   ```

See [GITHUB_TOKEN_SETUP.md](GITHUB_TOKEN_SETUP.md) for detailed instructions.

## 📊 Release Process

1. **Prerequisites Check** - Verifies Go, Git, and repository state
2. **Cross-Platform Build** - Compiles binaries for all target platforms
3. **Checksum Generation** - Creates SHA256 hashes for verification
4. **Release Creation** - Creates GitHub release with generated notes
5. **Asset Upload** - Uploads all binaries and checksums
6. **Verification** - Confirms successful upload

## 🎯 Release Notes

The script automatically generates comprehensive release notes including:
- Download links for all platforms
- Installation instructions
- Quick start guide
- SHA256 checksums
- Build information
- Recent git changes
- Documentation links

## 🔄 Version Management

### Semantic Versioning
- **v1.0.0** - Major release
- **v1.1.0** - Minor release with new features
- **v1.1.1** - Patch release with bug fixes

### Pre-release Versions
- **v1.0.0-alpha.1** - Alpha release
- **v1.0.0-beta.1** - Beta release
- **v1.0.0-rc.1** - Release candidate

## 🐛 Troubleshooting

### Build Failures
```bash
# Check Go installation
go version

# Verify in git repository
git status

# Test build manually
go build .
```

### GitHub API Issues
```bash
# Verify token permissions
curl -H "Authorization: token ghp_your_token" https://api.github.com/user

# Check rate limits
curl -H "Authorization: token ghp_your_token" https://api.github.com/rate_limit
```

### Network Issues
```bash
# Test GitHub connectivity
curl https://api.github.com/repos/badboysm890/ClaraCore

# Use proxy if needed
export HTTPS_PROXY=http://proxy:port
```

## 📈 CI/CD Integration

The release script can be integrated into GitHub Actions:

```yaml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - uses: actions/setup-python@v3
      - run: pip install -r requirements-release.txt
      - run: python release.py --version ${{ github.ref_name }} --token ${{ secrets.GITHUB_TOKEN }}
```

## 🔗 Related Documentation

- [Setup Guide](docs/SETUP.md) - Installation and configuration
- [API Documentation](docs/API_COMPREHENSIVE.md) - Complete API reference
- [Contributing Guide](CONTRIBUTING.md) - Development guidelines
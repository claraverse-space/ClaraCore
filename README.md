<div align="center">
  <img src="banner.png" alt="JarvisCore Banner" width="100%">
</div>

# JarvisCore 🚀

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)
```bash
# Add to PATH and reload shell:
export PATH="$HOME/.local/bin:$PATH"
source ~/.bashrc
```

**🛡️ Windows Security Notice**
JarvisCore may be flagged by antivirus software - **this is a false positive**. [Complete guide to antivirus warnings](docs/ANTIVIRUS_FALSE_POSITIVES.md) | [Why is this flagged?](SECURITY_VERIFICATION.md)

```powershell
# Quick fix - Add Windows Defender exclusion:
Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\JarvisCore"

# Or unblock the file:
Unblock-File "$env:LOCALAPPDATA\JarvisCore\jarviscore.exe"

# Or run troubleshooter:
curl -fsSL https://raw.githubusercontent.com/jarvisverse-space/JarvisCore/main/scripts/troubleshoot.ps1 | powershell
```

**✅ Verify it's safe:** Check SHA256 hash against [official releases](https://github.com/jarvisverse-space/JarvisCore/releases) or [build from source](SECURITY_VERIFICATION.md).

**Need help?** See our [Setup Guide](docs/SETUP.md) or [Container Guide](docker/CONTAINER_SETUP.md)

## 🙏 Credits & Acknowledgments)
[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/prave/JarvisCore)

> **Auto-setup for llama.cpp** - Point it at your GGUF models folder and get a complete AI inference server in seconds.

JarvisCore extends [llama-swap](https://github.com/mostlygeek/llama-swap) with intelligent automation, bringing zero-configuration setup to llama.cpp deployments.

## 🔥 Quick Install

### Native Installation

**Linux/macOS (Recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/jarvisverse-space/JarvisCore/main/scripts/install.sh | bash
```

**Windows:**
```powershell
irm https://raw.githubusercontent.com/jarvisverse-space/JarvisCore/main/scripts/install.ps1 | iex
```

**Then start immediately:**
```bash
jarviscore --models-folder /path/to/your/gguf/models
# Visit: http://localhost:5800/ui/setup
```

### 🐳 Docker (CUDA/ROCm)

**CUDA (NVIDIA):**
```bash
docker run -d --gpus all -p 5800:5800 -v ./models:/models jarviscore:cuda --models-folder /models
```

**ROCm (AMD):**
```bash
docker run -d --device=/dev/kfd --device=/dev/dri -p 5800:5800 -v ./models:/models jarviscore:rocm --models-folder /models
```

📦 **Optimized containers**: 2-3GB vs 8-12GB full SDKs. See [Container Guide](docker/CONTAINER_SETUP.md)

✨ **Features include:** Auto-setup, hardware detection, binary management, and production configs!

## ✨ What's New in JarvisCore

While maintaining 100% compatibility with llama-swap, JarvisCore adds:

- 🎯 **Auto-Setup Engine** - Automatic GGUF detection and configuration
- 🔍 **Smart Hardware Detection** - CUDA/ROCm/Vulkan/Metal optimization
- ⬇️ **Binary Management** - Automatic llama-server downloads
- ⚙️ **Intelligent Configs** - Production-ready settings out of the box
- 🚀 **Speculative Decoding** - Automatic draft model pairing

## 🚀 Quick Start

```bash
# One command setup - that's it!
./jarviscore --models-folder /path/to/your/gguf/models --backend vulkan

# JarvisCore will:
# 1. Scan for GGUF files
# 2. Detect your hardware (GPUs, CUDA, etc.)
# 3. Download optimal binaries
# 4. Generate intelligent configuration
# 5. Start serving immediately
```

## 📦 Installation

### Automated Installer (Recommended)

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/jarvisverse-space/JarvisCore/main/scripts/install.sh | bash
```

**Windows:**
```powershell
irm https://raw.githubusercontent.com/jarvisverse-space/JarvisCore/main/scripts/install.ps1 | iex
```

The installer will:
- Download the latest binary for your platform
- Set up configuration files
- Add to system PATH
- Configure auto-start service (systemd/launchd/Windows Service)

### Manual Download

```bash
# Windows
curl -L -o jarviscore.exe https://github.com/jarvisverse-space/JarvisCore/releases/latest/download/jarviscore-windows-amd64.exe

# Linux
curl -L -o jarviscore https://github.com/jarvisverse-space/JarvisCore/releases/latest/download/jarviscore-linux-amd64
chmod +x jarviscore

# macOS Intel
curl -L -o jarviscore https://github.com/jarvisverse-space/JarvisCore/releases/latest/download/jarviscore-darwin-amd64
chmod +x jarviscore

# macOS Apple Silicon
curl -L -o jarviscore https://github.com/jarvisverse-space/JarvisCore/releases/latest/download/jarviscore-darwin-arm64
chmod +x jarviscore
```

### Build from Source

```bash
git clone https://github.com/jarvisverse-space/JarvisCore.git
cd JarvisCore
python build.py  # Builds UI + Go backend with version info
# or: go build -o jarviscore .
```

## 🎛️ Core Features

All the power of llama-swap, plus intelligent automation:

### From llama-swap (unchanged)
- ✅ OpenAI API compatible endpoints
- ✅ Automatic model swapping on-demand
- ✅ Multiple models with `groups`
- ✅ Auto-unload with `ttl`
- ✅ Web UI for monitoring
- ✅ Docker/Podman support
- ✅ Full llama.cpp server control

### JarvisCore Enhancements
- ✅ Zero-configuration setup
- ✅ Automatic binary downloads
- ✅ Hardware capability detection
- ✅ Intelligent resource allocation
- ✅ Speculative decoding setup
- ✅ Model metadata parsing

## 📖 Usage Examples

### Automatic Setup
```bash
# Just point to your models
./jarviscore --models-folder ~/models
```
### Manual Setup - for devices like strix halo and others who want to customize or setup without relying on auto-detection
```bash
# Create a config file
./jarviscore -ram 64 -vram 24 -backend cuda
# it will download the binaries and create a config file automatically and UI will have all the features needed to manage models
```

### API Usage
```bash
# List models
curl http://localhost:8080/v1/models

# Chat completion
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-8b",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 🔧 Configuration

JarvisCore generates configurations automatically, but you can customize everything:

```yaml
# Auto-generated by JarvisCore
models:
  "llama-3-70b":
    cmd: |
      binaries/llama-server/llama-server
      --model models/llama-3-70b-q4.gguf
      --host 127.0.0.1 --port ${PORT}
      --flash-attn auto -ngl 99
    draft: "llama-3-8b"  # Automatic speculative decoding
    proxy: "http://127.0.0.1:${PORT}"
    
groups:
  "large-models":
    swap: true
    exclusive: true
    members: ["llama-3-70b", "qwen-72b"]
```

## � Documentation

### API Reference
- **[Complete API Documentation](docs/API_COMPREHENSIVE.md)** - Full API reference with examples
- **[Quick API Reference](docs/API.md)** - Concise API overview

### Key Features
- **OpenAI-Compatible Endpoints**: `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`
- **Configuration Management**: `/api/config/*` - Manage models and settings
- **Model Downloads**: `/api/models/download` - Download from Hugging Face
- **System Detection**: `/api/system/detection` - Hardware and backend detection
- **Real-time Events**: `/api/events` - SSE stream for live updates

### Common Operations

```bash
# Get all available models
curl http://localhost:5800/v1/models

# Update model parameters with restart prompt
curl -X POST http://localhost:5800/api/config/model/llama-3-8b \
  -H "Content-Type: application/json" \
  -d '{"temperature": 0.8, "max_tokens": 1024}'

# Smart configuration regeneration
curl -X POST http://localhost:5800/api/config/regenerate-from-db \
  -H "Content-Type: application/json" \
  -d '{"options": {"forceBackend": "vulkan", "preferredContext": 8192}}'

# Monitor setup progress
curl http://localhost:5800/api/setup/progress
```

### Web Interface
- **Setup Wizard**: `http://localhost:5800/ui/setup` - Initial configuration
- **Model Management**: `http://localhost:5800/ui/models` - Chat and model controls  
- **Configuration**: `http://localhost:5800/ui/configuration` - Edit settings
- **Downloads**: `http://localhost:5800/ui/downloads` - Model download manager

## �🙏 Credits & Acknowledgments

**JarvisCore is built on [llama-swap](https://github.com/mostlygeek/llama-swap) by [@mostlygeek](https://github.com/mostlygeek)** 

This project extends llama-swap's excellent proxy architecture with automatic setup capabilities. Approximately 90% of the core functionality comes from the original llama-swap codebase. We're deeply grateful for @mostlygeek's work in creating such a solid foundation.

### Special Thanks To:
- **[@mostlygeek](https://github.com/mostlygeek)** - Creator of llama-swap
- **[llama.cpp team](https://github.com/ggerganov/llama.cpp)** - The inference engine
- **[Georgi Gerganov](https://github.com/ggerganov)** - Creator of llama.cpp

## 🤝 Contributing

We welcome contributions! Whether it's bug fixes, new features, or documentation improvements.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## � Release Management

For maintainers creating releases:

```bash
# Quick release (interactive)
./release.bat        # Windows
./release.sh         # Linux/macOS

# Manual release
python release.py --version v0.1.1 --token-file .github_token

# Draft release for testing
python release.py --version v0.1.1 --token-file .github_token --draft
```

See [RELEASE_MANAGEMENT.md](RELEASE_MANAGEMENT.md) for detailed release procedures.

## �📄 License

MIT License - Same as llama-swap. See [LICENSE](LICENSE) for details.

## 🔗 Links

- [JarvisCore Issues](https://github.com/prave/JarvisCore/issues)
- [Original llama-swap](https://github.com/mostlygeek/llama-swap)
- [llama.cpp](https://github.com/ggerganov/llama.cpp)
- [Documentation Wiki](https://github.com/prave/JarvisCore/wiki)

---

<div align="center">
  
**Built with ❤️ by the community, for the community**

*Standing on the shoulders of giants*

</div>

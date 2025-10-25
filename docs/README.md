# ClaraCore Documentation

Welcome to the ClaraCore documentation! This directory contains comprehensive guides and references for using ClaraCore.

## 📚 Documentation Overview

### 🚀 Getting Started
- **[Setup Guide](SETUP.md)** - Complete installation and configuration guide
  - Quick start instructions
  - Backend selection (CUDA/Vulkan/ROCm/Metal/CPU)
  - Model organization and folder structure
  - Web interface walkthrough
  - Performance optimization tips
  - Troubleshooting common issues

### 📖 API Reference
- **[Complete API Documentation](API_COMPREHENSIVE.md)** - Full API reference with examples
  - OpenAI-compatible endpoints (`/v1/*`)
  - Configuration management (`/api/config/*`)
  - Model downloads (`/api/models/*`)
  - System information (`/api/system/*`)
  - Settings management (`/api/settings/*`)
  - Real-time events and monitoring
  - Authentication and security

- **[Quick API Reference](API.md)** - Concise API overview for developers
  - Essential endpoints
  - Common workflows
  - Quick examples

## 🎯 Quick Navigation

### For New Users
1. **[Setup Guide](SETUP.md)** - Start here for installation
2. **[Main README](../README.md)** - Project overview and features

### For Developers
1. **[API Documentation](API_COMPREHENSIVE.md)** - Complete API reference
2. **[Quick API Reference](API.md)** - Essential endpoints

### For Integration
1. **API Examples** - See API_COMPREHENSIVE.md for:
   - Python integration examples
   - JavaScript/Node.js examples
   - cURL command examples
   - OpenAI SDK compatibility

## 🌟 Key Features Documentation

### OpenAI Compatibility
ClaraCore provides full OpenAI API compatibility:
```bash
# Chat completions
POST /v1/chat/completions

# Text completions  
POST /v1/completions

# Embeddings
POST /v1/embeddings

# Model listing
GET /v1/models
```

### Smart Configuration
Automatic setup and optimization:
```bash
# Auto-detect and configure
./claracore --models-folder /path/to/models

# Backend-specific optimization
./claracore --models-folder /path/to/models --backend vulkan
```

### Web Interface
Modern React-based UI:
- **Setup Wizard**: `http://localhost:5800/ui/setup`
- **Model Chat**: `http://localhost:5800/ui/models`
- **Configuration**: `http://localhost:5800/ui/configuration`
- **Downloads**: `http://localhost:5800/ui/downloads`

### Real-time Features
- **Progress Tracking**: Live setup progress with polling
- **Restart Prompts**: Automatic prompts when config changes
- **Event Streaming**: Real-time logs and metrics via SSE
- **Download Manager**: Queue and monitor model downloads

## 🔧 Configuration Files

ClaraCore uses several configuration files:

### `config.yaml` - Main Configuration
```yaml
host: "127.0.0.1"
port: 8080
models:
  - name: "llama-3.2-3b-instruct"
    backend: "cuda"
    model: "/path/to/model.gguf"
    context_length: 8192
```

### `settings.json` - System Settings
```json
{
  "gpuType": "nvidia",
  "backend": "cuda", 
  "vramGB": 10.0,
  "ramGB": 32.0,
  "preferredContext": 8192,
  "throughputFirst": true,
  "enableJinja": true
}
```

### `model_folders.json` - Tracked Folders
```json
{
  "folders": [
    {
      "path": "/home/user/models",
      "enabled": true,
      "recursive": true
    }
  ]
}
```

## 🎨 Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React UI      │    │   Go Backend    │    │  llama-server   │
│   (Frontend)    │◄──►│   (Proxy)       │◄──►│   (Inference)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐              │
         └─────────────►│  Configuration  │◄─────────────┘
                        │   Management    │
                        └─────────────────┘
```

### Components
- **React UI**: Modern web interface for setup and management
- **Go Backend**: Proxy server with intelligent automation
- **llama-server**: High-performance inference engine
- **Configuration Management**: Automatic setup and optimization

## 🚀 Common Workflows

### 1. First-Time Setup
```bash
# 1. Download ClaraCore
curl -L -o claracore https://github.com/badboysm890/ClaraCore/releases/latest/download/claracore-linux-amd64

# 2. Run auto-setup
./claracore --models-folder /path/to/models

# 3. Access web interface
open http://localhost:5800/ui/setup
```

### 2. API Integration
```python
import openai

# Configure client
client = openai.OpenAI(
    base_url="http://localhost:5800/v1",
    api_key="not-required"  # unless auth enabled
)

# Chat completion
response = client.chat.completions.create(
    model="llama-3.2-3b-instruct",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### 3. Model Management
```bash
# Add model folders
curl -X POST http://localhost:5800/api/config/folders \
  -H "Content-Type: application/json" \
  -d '{"folderPaths": ["/path/to/models"]}'

# Regenerate configuration
curl -X POST http://localhost:5800/api/config/regenerate-from-db

# Soft restart
curl -X POST http://localhost:5800/api/server/restart
```

## 🛠️ Advanced Topics

### Custom Binary Management
ClaraCore automatically downloads optimal binaries, but you can customize:
```bash
# Use custom binary
./claracore --binary-path /custom/llama-server

# Force binary re-download
rm -rf binaries/
./claracore --models-folder /path/to/models
```

### Performance Tuning
Key parameters for optimization:
- **Context Length**: Balance memory vs capability
- **GPU Layers**: Maximize GPU utilization
- **Batch Size**: Optimize throughput
- **Backend Selection**: Match hardware capabilities

### Multi-Model Configuration
ClaraCore supports complex multi-model setups:
- **Speculative Decoding**: Automatic draft model pairing
- **Model Groups**: Exclusive loading for memory management
- **Smart Swapping**: On-demand model loading/unloading

## 📞 Support & Community

### Getting Help
- **GitHub Issues**: [Report bugs and request features](https://github.com/badboysm890/ClaraCore/issues)
- **GitHub Discussions**: [Community support and questions](https://github.com/badboysm890/ClaraCore/discussions)
- **Documentation**: Check this documentation for common solutions

### Contributing
- **Bug Reports**: Use GitHub Issues with detailed reproduction steps
- **Feature Requests**: Describe use cases and expected behavior
- **Pull Requests**: Follow the contributing guidelines
- **Documentation**: Help improve these docs!

### Compatibility
- **llama-swap**: 100% compatible with existing llama-swap configurations
- **OpenAI API**: Full compatibility with OpenAI SDKs and tools
- **GGUF Models**: Support for all standard GGUF quantizations

---

## 📋 Document Status

| Document | Status | Last Updated |
|----------|--------|--------------|
| [SETUP.md](SETUP.md) | ✅ Complete | Oct 2025 |
| [API_COMPREHENSIVE.md](API_COMPREHENSIVE.md) | ✅ Complete | Oct 2025 |
| [API.md](API.md) | ✅ Complete | Oct 2025 |

---

**Happy building with ClaraCore! 🚀**
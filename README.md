# 🚀 ClaraCore# ClaraCore



[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)ClaraCore is a light weight, transparent proxy server that provides automatic model swapping to llama.cpp's server with enhanced auto-setup capabilities.

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org/)

[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/prave/ClaraCore)Written in golang, it is very easy to install (single binary with no dependencies) and configure (single yaml file or automatic setup). To get started, download a pre-built binary or use the new auto-setup feature.



> **The missing auto-setup solution for llama.cpp** - Point it at your GGUF models folder and get a complete AI inference server in seconds.## Features:



## ✨ What is ClaraCore?- ✅ **Auto-Setup**: Automatically detect GGUF models and download binaries with `--models-folder`

- ✅ **Smart GPU Detection**: Automatically detects CUDA/ROCm/Vulkan capabilities and GPU count

ClaraCore is an enhanced AI model management platform that brings **automatic setup capabilities** to llama.cpp. While platforms like vLLM, Ollama, and others have streamlined deployment, **llama.cpp was missing this crucial piece** - until now.- ✅ **Intelligent Configuration**: Generates optimized configs with speculative decoding and resource management

- ✅ Easy to deploy: single binary with no dependencies

**Built on the solid foundation of [llama-swap](https://github.com/mostlygeek/llama-swap)** by [@mostlygeek](https://github.com/mostlygeek), ClaraCore extends the original vision with intelligent automation that rivals commercial platforms.- ✅ Easy to config: single yaml file or automatic generation

- ✅ On-demand model switching

## 🎯 Why ClaraCore?- ✅ OpenAI API supported endpoints:

  - `v1/completions`

### The Problem  - `v1/chat/completions`

- **Manual configuration** - Other tools required extensive YAML editing  - `v1/embeddings`

- **Hardware detection** - No automatic GPU/CUDA detection  - `v1/models`

- **Binary management** - Manual downloading and setup of llama-server  - `v1/audio/transcriptions`

- **Model organization** - No automatic discovery of existing GGUF collections- ✅ llama-server (llama.cpp) supported endpoints:

  - `v1/rerank`, `v1/reranking`, `/rerank`

### The Solution  - `/infill` - for code infilling

```bash  - `/completion` - for completion endpoint

# Instead of hours of configuration...- ✅ ClaraCore custom API endpoints

./claracore --models-folder /path/to/your/gguf/models  - `/ui` - web UI

  - `/log` - remote log monitoring

# Get instant setup with:  - `/upstream/:model_id` - direct access to upstream HTTP server

✅ Automatic GGUF model detection and metadata parsing  - `/unload` - manually unload running models

✅ Smart hardware detection (CUDA/ROCm/Vulkan/Metal/CPU)  - `/running` - list currently running models

✅ Intelligent binary downloading with system optimization  - `/health` - just returns "OK"

✅ Production-ready configuration generation- ✅ Run multiple models at once with `Groups`

✅ Speculative decoding setup for compatible models- ✅ Automatic unloading of models after timeout by setting a `ttl`

✅ Resource management and GPU assignment- ✅ Use any local OpenAI compatible server (llama.cpp, vllm, tabbyAPI, etc)

```- ✅ Reliable Docker and Podman support using `cmd` and `cmdStop` together

- ✅ Full control over server settings per model

## 🚀 Quick Start- ✅ Preload models on startup with `hooks`



### One-Command Setup## Quick Start with Auto-Setup

```bash

# Download and auto-configure everythingClaraCore can automatically detect your GGUF models and set everything up for you:

./claracore --models-folder "C:\AI\Models"

```bash

# Start serving your models# Auto-detect models and download binaries

./claracore./claracore --models-folder /path/to/your/gguf/models

```

# That's it! ClaraCore will:

**That's it!** ClaraCore will:# 1. Scan for GGUF files and detect model metadata

1. 🔍 **Scan** your folder for GGUF files# 2. Detect your system capabilities (CUDA/ROCm/Vulkan/CPU)

2. 🧠 **Detect** your hardware capabilities# 3. Download the optimal llama-server binary

3. ⬇️ **Download** optimal binaries for your system# 4. Generate an intelligent config.yaml with all your models

4. ⚙️ **Generate** intelligent configuration# 5. Set up speculative decoding, GPU assignments, and optimization

5. 🎯 **Setup** speculative decoding and optimization

6. 🚀 **Launch** your AI inference server# Then start normally:

./claracore

### Traditional Setup (Manual Configuration)```

```bash

# Create config.yaml manually (like the original llama-swap)The auto-setup feature will create a complete configuration including:

./claracore --config config.yaml- **Smart GPU detection** and proper device assignments

```- **Speculative decoding** pairs for compatible models

- **Resource management groups** for efficient memory usage

## 🏗️ Architecture- **Optimized sampling parameters** per model type

- **Model aliases** for easy API access

ClaraCore is built on the proven **llama-swap architecture** with modern enhancements:

## How does ClaraCore work?

```

┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐When a request is made to an OpenAI compatible endpoint, llama-swap will extract the `model` value and load the appropriate server configuration to serve it. If the wrong upstream server is running, it will be replaced with the correct one. This is where the "swap" part comes in. The upstream server is automatically swapped to the correct one to serve the request.

│   Client API    │───▶│    ClaraCore     │───▶│  llama-server   │

│  (OpenAI compat)│    │   (Proxy + AI)   │    │   (llama.cpp)   │In the most basic configuration llama-swap handles one model at a time. For more advanced use cases, the `groups` feature allows multiple models to be loaded at the same time. You have complete control over how your system resources are used.

└─────────────────┘    └──────────────────┘    └─────────────────┘

                              │## config.yaml

                       ┌──────▼──────┐

                       │ Auto-Setup  │llama-swap is managed entirely through a yaml configuration file.

                       │   Engine    │

                       └─────────────┘It can be very minimal to start:

```

```yaml

## 🎛️ Featuresmodels:

  "qwen2.5":

### 🔥 **Auto-Setup Engine** (New in ClaraCore)    cmd: |

- **Smart Model Detection** - Automatically discovers GGUF files with metadata parsing      /path/to/llama-server

- **Hardware Intelligence** - Detects CUDA, ROCm, Vulkan, Metal, and optimizes accordingly      -hf bartowski/Qwen2.5-0.5B-Instruct-GGUF:Q4_K_M

- **Binary Management** - Downloads and manages llama.cpp binaries automatically      --port ${PORT}

- **Configuration AI** - Generates production-ready configs with zero manual input```



### 🚀 **Core Features** (Inherited from llama-swap)However, there are many more capabilities that llama-swap supports:

- **OpenAI Compatible API** - Drop-in replacement for OpenAI endpoints

- **Automatic Model Swapping** - Load models on-demand based on requests- `groups` to run multiple models at once

- **Resource Management** - Intelligent memory and GPU utilization- `ttl` to automatically unload models

- **Multi-Model Support** - Run multiple models simultaneously with groups- `macros` for reusable snippets

- **Web Interface** - Built-in UI for monitoring and management- `aliases` to use familiar model names (e.g., "gpt-4o-mini")

- `env` to pass custom environment variables to inference servers

### 📡 **API Endpoints**- `cmdStop` for to gracefully stop Docker/Podman containers

```- `useModelName` to override model names sent to upstream servers

OpenAI Compatible:- `healthCheckTimeout` to control model startup wait times

├── POST /v1/chat/completions- `${PORT}` automatic port variables for dynamic port assignment

├── POST /v1/completions  

├── POST /v1/embeddingsSee the [configuration documentation](https://github.com/mostlygeek/llama-swap/wiki/Configuration) in the wiki all options and examples.

├── GET  /v1/models

└── POST /v1/audio/transcriptions## Reverse Proxy Configuration (nginx)



llama.cpp Native:If you deploy llama-swap behind nginx, disable response buffering for streaming endpoints. By default, nginx buffers responses which breaks Server‑Sent Events (SSE) and streaming chat completion. ([#236](https://github.com/mostlygeek/llama-swap/issues/236))

├── POST /v1/rerank

├── POST /infillRecommended nginx configuration snippets:

├── POST /completion

└── GET  /health```nginx

# SSE for UI events/logs

ClaraCore Management:location /api/events {

├── GET  /ui              # Web interface    proxy_pass http://your-llama-swap-backend;

├── GET  /log             # Real-time logs    proxy_buffering off;

├── POST /unload          # Manual model unload    proxy_cache off;

├── GET  /running         # Active models}

└── GET  /upstream/:id    # Direct model access

```# Streaming chat completions (stream=true)

location /v1/chat/completions {

## 🛠️ Installation    proxy_pass http://your-llama-swap-backend;

    proxy_buffering off;

### Download Binaries    proxy_cache off;

```bash}

# Windows```

curl -L -o claracore.exe https://github.com/prave/ClaraCore/releases/latest/download/claracore-windows-amd64.exe

As a safeguard, llama-swap also sets `X-Accel-Buffering: no` on SSE responses. However, explicitly disabling `proxy_buffering` at your reverse proxy is still recommended for reliable streaming behavior.

# Linux

curl -L -o claracore https://github.com/prave/ClaraCore/releases/latest/download/claracore-linux-amd64## Web UI



# macOSllama-swap includes a real time web interface for monitoring logs and models:

curl -L -o claracore https://github.com/prave/ClaraCore/releases/latest/download/claracore-darwin-amd64

chmod +x claracore<img width="1360" height="963" alt="image" src="https://github.com/user-attachments/assets/adef4a8e-de0b-49db-885a-8f6dedae6799" />

```

The Activity Page shows recent requests:

### Build from Source

```bash<img width="1360" height="963" alt="image" src="https://github.com/user-attachments/assets/5f3edee6-d03a-4ae5-ae06-b20ac1f135bd" />

git clone https://github.com/prave/ClaraCore.git

cd ClaraCore## Installation

go build -o claracore .

```llama-swap can be installed in multiple ways



## 📖 Usage Examples1. Docker

2. Homebrew (OSX and Linux)

### Example 1: Complete Auto-Setup3. From release binaries

```bash4. From source

# Point ClaraCore at your models folder

./claracore --models-folder ~/AI/Models### Docker Install ([download images](https://github.com/mostlygeek/llama-swap/pkgs/container/llama-swap))



# Output:Docker images with llama-swap and llama-server are built nightly.

# 🚀 Starting ClaraCore auto-setup...

# 📁 Scanning models in: ~/AI/Models```shell

# ✅ Found 15 GGUF models# use CPU inference comes with the example config above

# 🔍 Detecting system capabilities...$ docker run -it --rm -p 9292:8080 ghcr.io/mostlygeek/llama-swap:cpu

# ✅ CUDA detected with 1 GPU

# ⬇️ Downloading llama-server binary...# qwen2.5 0.5B

# ⚙️ Generating configuration...$ curl -s http://localhost:9292/v1/chat/completions \

# 🎉 Setup complete!    -H "Content-Type: application/json" \

    -H "Authorization: Bearer no-key" \

# Start the server    -d '{"model":"qwen2.5","messages": [{"role": "user","content": "tell me a joke"}]}' | \

./claracore    jq -r '.choices[0].message.content'

```

# SmolLM2 135M

### Example 2: Chat with Your Models$ curl -s http://localhost:9292/v1/chat/completions \

```bash    -H "Content-Type: application/json" \

# List available models    -H "Authorization: Bearer no-key" \

curl http://localhost:8080/v1/models    -d '{"model":"smollm2","messages": [{"role": "user","content": "tell me a joke"}]}' | \

    jq -r '.choices[0].message.content'

# Chat completion```

curl http://localhost:8080/v1/chat/completions \

  -H "Content-Type: application/json" \<details>

  -d '{<summary>Docker images are built nightly with llama-server for cuda, intel, vulcan and musa.</summary>

    "model": "llama-3-8b-instruct",

    "messages": [{"role": "user", "content": "Hello!"}]They include:

  }'

```- `ghcr.io/mostlygeek/llama-swap:cpu`

- `ghcr.io/mostlygeek/llama-swap:cuda`

### Example 3: Advanced Configuration- `ghcr.io/mostlygeek/llama-swap:intel`

```yaml- `ghcr.io/mostlygeek/llama-swap:vulkan`

# config.yaml - Auto-generated by ClaraCore- ROCm disabled until fixed in llama.cpp container

models:

  "llama-3-70b":Specific versions are also available and are tagged with the llama-swap, architecture and llama.cpp versions. For example: `ghcr.io/mostlygeek/llama-swap:v89-cuda-b4716`

    cmd: |

      binaries/llama-server/llama-server.exeBeyond the demo you will likely want to run the containers with your downloaded models and custom configuration.

      --model models/llama-3-70b-q4.gguf

      --host 127.0.0.1 --port ${PORT}```shell

      --flash-attn auto -ngl 99$ docker run -it --rm --runtime nvidia -p 9292:8080 \

    draft: "llama-3-8b"  # Speculative decoding  -v /path/to/models:/models \

    proxy: "http://127.0.0.1:${PORT}"  -v /path/to/custom/config.yaml:/app/config.yaml \

      ghcr.io/mostlygeek/llama-swap:cuda

groups:```

  "large-models":

    swap: true</details>

    exclusive: true

    members: ["llama-3-70b", "qwen-72b"]### Homebrew Install (macOS/Linux)

```

The latest release of `llama-swap` can be installed via [Homebrew](https://brew.sh).

## 🔧 Configuration

```shell

### Auto-Generated Configuration# Set up tap and install formula

ClaraCore creates intelligent configurations automatically:brew tap mostlygeek/llama-swap

brew install llama-swap

- **Speculative Decoding** - Pairs large models with smaller draft models# Run llama-swap

- **GPU Assignment** - Distributes models across available GPUsllama-swap --config path/to/config.yaml --listen localhost:8080

- **Memory Management** - Optimizes context sizes and batch settings```

- **Model Groups** - Organizes models by size and capability

- **Sampling Parameters** - Sets optimal defaults per model typeThis will install the `llama-swap` binary and make it available in your path. See the [configuration documentation](https://github.com/mostlygeek/llama-swap/wiki/Configuration)



### Manual Configuration### Pre-built Binaries ([download](https://github.com/mostlygeek/llama-swap/releases))

For advanced users, ClaraCore maintains full compatibility with llama-swap configuration:

Binaries are available for Linux, Mac, Windows and FreeBSD. These are automatically published and are likely a few hours ahead of the docker releases. The binary install works with any OpenAI compatible server, not just llama-server.

```yaml

healthCheckTimeout: 3001. Download a [release](https://github.com/mostlygeek/llama-swap/releases) appropriate for your OS and architecture.

logLevel: info1. Create a configuration file, see the [configuration documentation](https://github.com/mostlygeek/llama-swap/wiki/Configuration).

startPort: 58001. Run the binary with `llama-swap --config path/to/config.yaml --listen localhost:8080`.

   Available flags:

models:   - `--config`: Path to the configuration file (default: `config.yaml`).

  "my-model":   - `--listen`: Address and port to listen on (default: `:8080`).

    cmd: "llama-server --model path/to/model.gguf"   - `--version`: Show version information and exit.

    proxy: "http://localhost:5800"   - `--watch-config`: Automatically reload the configuration file when it changes. This will wait for in-flight requests to complete then stop all running models (default: `false`).

    ttl: 300  # Auto-unload after 5 minutes

```### Building from source



## 🤝 Contributing1. Build requires golang and nodejs for the user interface.

1. `git clone https://github.com/mostlygeek/llama-swap.git`

We welcome contributions! ClaraCore builds upon the excellent foundation of llama-swap and aims to push the boundaries of what's possible with llama.cpp.1. `make clean all`

1. Binaries will be in `build/` subdirectory

### Development Setup

```bash## Monitoring Logs

git clone https://github.com/prave/ClaraCore.git

cd ClaraCoreOpen the `http://<host>:<port>/` with your browser to get a web interface with streaming logs.

go mod download

go run . --helpCLI access is also supported:

```

```shell

### Running Tests# sends up to the last 10KB of logs

```bashcurl http://host/logs'

go test ./...

```# streams combined logs

curl -Ns 'http://host/logs/stream'

## 📄 License

# just llama-swap's logs

MIT License - see [LICENSE](LICENSE) file for details.curl -Ns 'http://host/logs/stream/proxy'



## 🙏 Acknowledgments# just upstream's logs

curl -Ns 'http://host/logs/stream/upstream'

**ClaraCore is built on the shoulders of giants:**

# stream and filter logs with linux pipes

- **[@mostlygeek](https://github.com/mostlygeek)** - Creator of [llama-swap](https://github.com/mostlygeek/llama-swap), the foundational proxy architecture that makes ClaraCore possiblecurl -Ns http://host/logs/stream | grep 'eval time'

- **[llama.cpp team](https://github.com/ggerganov/llama.cpp)** - The incredible inference engine that powers everything

- **[Georgi Gerganov](https://github.com/ggerganov)** - Creator of llama.cpp and pioneer of efficient LLM inference# skips history and just streams new log entries

curl -Ns 'http://host/logs/stream?no-history'

### Why Fork llama-swap?```



llama-swap provided an excellent foundation for model swapping and proxy management. However, we identified a gap in the ecosystem:## Do I need to use llama.cpp's server (llama-server)?



> **"Every major AI platform has automatic setup - vLLM, Ollama, Text Generation WebUI - but llama.cpp users were stuck with manual configuration."**Any OpenAI compatible server would work. llama-swap was originally designed for llama-server and it is the best supported.



ClaraCore bridges this gap by adding:For Python based inference servers like vllm or tabbyAPI it is recommended to run them via podman or docker. This provides clean environment isolation as well as responding correctly to `SIGTERM` signals to shutdown.

- **Zero-configuration setup** that rivals commercial platforms

- **Intelligent hardware detection** that optimizes for your specific system## Star History

- **Automatic model discovery** that works with existing GGUF collections

- **Production-ready defaults** that eliminate guesswork> [!NOTE]

> ⭐️ Star this project to help others discover it! 

We maintain the original llama-swap philosophy of simplicity and reliability while adding the automation that modern AI development demands.

[![Star History Chart](https://api.star-history.com/svg?repos=mostlygeek/llama-swap&type=Date)](https://www.star-history.com/#mostlygeek/llama-swap&Date)

## 🔗 Related Projects

- **[llama-swap](https://github.com/mostlygeek/llama-swap)** - The original inspiration and foundation
- **[llama.cpp](https://github.com/ggerganov/llama.cpp)** - The inference engine
- **[Ollama](https://ollama.ai/)** - Docker-based LLM serving
- **[vLLM](https://github.com/vllm-project/vllm)** - High-performance inference server

---

<div align="center">

**Made with ❤️ for the AI community**

[Report Bug](https://github.com/prave/ClaraCore/issues) • [Request Feature](https://github.com/prave/ClaraCore/issues) • [Documentation](https://github.com/prave/ClaraCore/wiki)

</div>
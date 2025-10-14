# 🚀 ClaraCore Universal Container

## One Container, Every GPU

ClaraCore's universal container automatically detects and adapts to your hardware. No configuration needed!

```bash
# That's literally it
docker compose -f docker-compose.universal.yml up
```

## ✨ What Makes It Universal?

- **🔍 Smart Detection**: Automatically finds NVIDIA, AMD, Intel, or falls back to CPU
- **📦 One Image**: Same container works everywhere - no special builds needed
- **⚡ Optimized**: Always picks the fastest backend for your hardware
- **🛠️ Override Ready**: Force specific backends when needed

## 🎯 Supported Hardware

| Hardware | Auto-Detected | Backend Used | Performance |
|----------|---------------|--------------|-------------|
| NVIDIA GPU | ✅ Yes | CUDA | Best |
| AMD GPU | ✅ Yes | ROCm | Best |
| Intel/AMD/NVIDIA | ✅ Yes | Vulkan | Great |
| No GPU / CPU only | ✅ Yes | CPU | Good |

## 📦 Quick Start

### 1. Build the Image

```bash
cd docker
./build-universal.sh  # Linux/Mac
# or
.\build-universal.ps1  # Windows
```

### 2. Add Your Models

```bash
# Copy your GGUF models to the models folder
cp /path/to/your/*.gguf ./models/
```

### 3. Start the Container

```bash
# Let it auto-detect your hardware (recommended)
docker compose -f docker-compose.universal.yml up

# The container will:
# 1. Detect your GPU (NVIDIA/AMD/Intel) or use CPU
# 2. Configure the optimal backend
# 3. Start ClaraCore with your models
```

### 4. Access the UI

Open your browser to: **http://localhost:5800/ui/**

## 🎛️ Deployment Modes

### Auto-Detection (Recommended)

```bash
docker compose -f docker-compose.universal.yml up
```

The container automatically detects:
1. NVIDIA GPU → Uses CUDA
2. AMD GPU → Uses ROCm
3. Vulkan-capable GPU → Uses Vulkan
4. No GPU → Uses CPU

### Force Specific Backend

```bash
# Force CPU mode
docker compose -f docker-compose.cpu-only.yml up

# Force CUDA (NVIDIA)
docker compose -f docker-compose.cuda-explicit.yml up

# Force ROCm (AMD)
docker compose -f docker-compose.rocm-explicit.yml up

# Force Vulkan (Universal)
docker compose -f docker-compose.vulkan-explicit.yml up
```

### Custom Backend via Environment

```bash
docker run -e CLARACORE_BACKEND=vulkan \
  -v $(pwd)/models:/models \
  -p 5800:5800 \
  claracore:universal
```

## 📁 Folder Structure

```
docker/
├── models/              # Put your GGUF models here
├── config/              # Auto-generated config (persisted)
├── binaries/            # Cached llama.cpp binaries
├── downloads/           # Model downloads cache
├── Dockerfile.ollama-style    # The universal Dockerfile
├── entrypoint-universal.sh    # Smart startup script
├── docker-compose.*.yml       # Various deployment configs
└── DEPLOYMENT.md              # Full documentation
```

## 🔍 Hardware Detection Example

When you start the container, you'll see:

```
🚀 ClaraCore Universal Container Starting...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 Detecting available hardware...

✅ NVIDIA GPU detected
NVIDIA GeForce RTX 4090, 24564 MiB

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎯 Selected Backend: cuda
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📦 Models detected - auto-generating configuration...
🔧 Starting with auto-setup mode...
Clara Core listening on :5800
```

## 🛠️ Advanced Usage

### Custom Port

```bash
# Run on port 8080 instead of 5800
docker run -p 8080:5800 \
  -v $(pwd)/models:/models \
  claracore:universal
```

### Multiple GPU Selection

```bash
# NVIDIA: Use specific GPUs
docker run -e NVIDIA_VISIBLE_DEVICES=0,1 \
  -v $(pwd)/models:/models \
  claracore:universal

# AMD: Use specific GPUs
docker run -e HIP_VISIBLE_DEVICES=0,1 \
  -v $(pwd)/models:/models \
  claracore:universal
```

### Override Hardware Resources

```bash
# Force specific VRAM/RAM amounts
docker run \
  -e CLARACORE_BACKEND=cuda \
  -v $(pwd)/models:/models \
  claracore:universal \
  --listen :5800 \
  --models-folder /models \
  --config /app/config/config.yaml \
  --vram 16 \
  --ram 32
```

## 🐛 Troubleshooting

### Container starts but uses CPU instead of GPU

**Check GPU access:**

```bash
# For NVIDIA
docker run --rm --gpus all nvidia/cuda:12.0-base nvidia-smi

# For AMD
docker run --rm --device=/dev/kfd --device=/dev/dri rocm/rocm-terminal:latest rocm-smi

# For Vulkan
docker run --rm --device=/dev/dri ubuntu:22.04 apt-get update && apt-get install -y vulkan-tools && vulkaninfo
```

If these fail, you need to install GPU support:
- NVIDIA: [nvidia-container-toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
- AMD: [ROCm drivers](https://rocm.docs.amd.com/en/latest/deploy/linux/quick_start.html)

### Force CPU mode for testing

```bash
docker run -e CLARACORE_BACKEND=cpu \
  -v $(pwd)/models:/models \
  -p 5800:5800 \
  claracore:universal
```

### View detection logs

```bash
docker logs claracore 2>&1 | grep -A 20 "Detecting"
```

### No models found

Ensure your models folder has `.gguf` files:

```bash
ls -lh docker/models/*.gguf
```

## 📊 Performance Expectations

### NVIDIA RTX 4090 (CUDA)
- Llama 3 8B: ~100 tok/s
- Llama 3 70B: ~20 tok/s

### AMD RX 7900 XTX (ROCm)
- Llama 3 8B: ~85 tok/s
- Llama 3 70B: ~15 tok/s

### Any GPU (Vulkan)
- Llama 3 8B: ~60-70 tok/s
- Llama 3 70B: ~10-12 tok/s

### CPU (AMD Ryzen 9 5950X)
- Llama 3 8B: ~15 tok/s
- Llama 3 70B: ~2-3 tok/s

*Performance varies based on model quantization and hardware*

## 🎓 How It Works

1. **Container starts** → Runs `entrypoint-universal.sh`
2. **Hardware detection** → Checks for NVIDIA, AMD, Vulkan
3. **Backend selection** → Picks the fastest available
4. **Binary management** → ClaraCore downloads correct llama.cpp binary
5. **Auto-configuration** → Scans models and generates optimal config
6. **Server start** → Listens on port 5800

The beauty is: **All of this happens automatically!**

## 📝 Files Overview

| File | Purpose |
|------|---------|
| `Dockerfile.ollama-style` | Universal container definition |
| `entrypoint-universal.sh` | Smart hardware detection script |
| `docker-compose.universal.yml` | Auto-detect compose file |
| `docker-compose.cpu-only.yml` | Force CPU mode |
| `docker-compose.cuda-explicit.yml` | Force CUDA (NVIDIA) |
| `docker-compose.rocm-explicit.yml` | Force ROCm (AMD) |
| `docker-compose.vulkan-explicit.yml` | Force Vulkan |
| `build-universal.sh` | Build script (Linux/Mac) |
| `build-universal.ps1` | Build script (Windows) |
| `test-universal.sh` | Quick test script |
| `DEPLOYMENT.md` | Full deployment guide |

## 🎯 Best Practices

1. **Use auto-detection** unless you have a specific reason not to
2. **Mount volumes** for models, config, and binaries (faster restarts)
3. **Check logs** on first run to see what hardware was detected
4. **Test with CPU first** if you're having GPU issues
5. **Keep models folder clean** - only GGUF files

## 🚀 Production Deployment

### Docker Compose (Recommended)

```bash
# Start in detached mode
docker compose -f docker-compose.universal.yml up -d

# Check status
docker compose -f docker-compose.universal.yml ps

# View logs
docker compose -f docker-compose.universal.yml logs -f

# Stop
docker compose -f docker-compose.universal.yml down
```

### Docker CLI

```bash
docker run -d \
  --name claracore \
  --restart unless-stopped \
  -p 5800:5800 \
  -v /path/to/models:/models \
  -v /path/to/config:/app/config \
  --gpus all \
  claracore:universal
```

### Kubernetes

See `DEPLOYMENT.md` for Kubernetes deployment examples.

## 💡 Pro Tips

1. **First run is slower** - Downloads binaries, generates config
2. **Subsequent starts are fast** - Everything is cached
3. **One image for all environments** - Build once, deploy anywhere
4. **Hardware changes? No problem** - Container adapts automatically
5. **Override when needed** - But auto-detection usually just works

## 🎉 That's It!

You now have a **truly universal** container that works on:
- ✅ NVIDIA GPUs (CUDA)
- ✅ AMD GPUs (ROCm)
- ✅ Intel GPUs (Vulkan)
- ✅ Any GPU with Vulkan
- ✅ CPU-only systems

**No configuration, no hassle, it just works!**

For more details, see [DEPLOYMENT.md](DEPLOYMENT.md)

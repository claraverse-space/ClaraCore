# ClaraCore Docker Containers

This directory contains Docker configurations for running ClaraCore with GPU acceleration.

## 🚀 Quick Start

### CUDA (NVIDIA GPUs)
```bash
cd docker
docker build -f Dockerfile.cuda -t claracore:cuda ..
docker run -d --gpus all -p 5800:5800 -v /path/to/models:/models claracore:cuda --models-folder /models
```

### ROCm (AMD GPUs)
```bash
cd docker
docker build -f Dockerfile.rocm -t claracore:rocm ..
docker run -d --device=/dev/kfd --device=/dev/dri -p 5800:5800 -v /path/to/models:/models claracore:rocm --models-folder /models
```

## 📁 Files

- **Dockerfile.cuda** - CUDA-optimized container (NVIDIA GPUs)
- **Dockerfile.rocm** - ROCm-optimized container (AMD GPUs)
- **docker-compose.cuda.yml** - Docker Compose for CUDA
- **docker-compose.rocm.yml** - Docker Compose for ROCm
- **build-containers.sh** - Build script (Linux/macOS)
- **build-containers.ps1** - Build script (Windows)
- **test-container.sh** - Test script
- **.dockerignore** - Docker ignore file

## 📚 Documentation

- **[DOCKER_QUICK_START.md](./DOCKER_QUICK_START.md)** - Quick reference guide
- **[CONTAINER_SETUP.md](./CONTAINER_SETUP.md)** - Comprehensive setup guide
- **[CONTAINER_TESTING.md](./CONTAINER_TESTING.md)** - Testing guide

## 🔨 Building

### Build both variants
```bash
# Linux/macOS
./build-containers.sh --all

# Windows
.\build-containers.ps1 -all
```

### Build specific variant
```bash
# CUDA only
./build-containers.sh --cuda

# ROCm only
./build-containers.sh --rocm
```

## 🐳 Using Docker Compose

### CUDA
```bash
docker-compose -f docker-compose.cuda.yml up -d
```

### ROCm
```bash
docker-compose -f docker-compose.rocm.yml up -d
```

## 📊 Container Sizes

These containers are optimized to be **much smaller** than full SDK containers:

| Container | Size | Notes |
|-----------|------|-------|
| claracore:cuda | ~4GB | Runtime only, no SDK |
| claracore:rocm | ~3-4GB | Runtime only, no SDK |
| Full CUDA SDK | ~8-12GB | Development container |
| Full ROCm SDK | ~10-15GB | Development container |

## ✅ Testing

```bash
# Test CUDA container
./test-container.sh cuda

# Test ROCm container
./test-container.sh rocm
```

## 🌐 Accessing the UI

Once running, access:
- Web UI: http://localhost:5800/ui/
- API: http://localhost:5800/v1/
- Setup: http://localhost:5800/ui/setup

## 📦 Volume Mounts

Mount these directories for persistence:

```yaml
volumes:
  - ./models:/models              # Your GGUF models (required)
  - ./config.yaml:/app/config.yaml  # Configuration
  - ./downloads:/app/downloads      # Downloaded models cache
  - ./binaries:/app/binaries        # llama-server binaries
```

## 🔧 GPU Access Requirements

### NVIDIA (CUDA)
- Docker 19.03+ with nvidia-container-toolkit
- NVIDIA driver 525+ (for CUDA 12)
- Use `--gpus all` flag

### AMD (ROCm)
- Docker 19.03+
- ROCm 5.0+ drivers
- Use `--device=/dev/kfd --device=/dev/dri`

## 🐛 Troubleshooting

### GPU Not Detected
```bash
# Test GPU access in container
docker run --rm --gpus all claracore:cuda nvidia-smi
```

### Build Failed
```bash
# Ensure dist/claracore-linux-amd64 exists
ls -la ../dist/claracore-linux-amd64

# Build from project root
cd ..
python build.py
cd docker
```

### Container Won't Start
```bash
# Check logs
docker logs <container-name>

# Check if port is available
netstat -an | grep 5800
```

## 📝 Example Usage

### Auto-setup with models
```bash
docker run -d \
  --name claracore \
  --gpus all \
  -p 5800:5800 \
  -v $(pwd)/models:/models \
  -v $(pwd)/config.yaml:/app/config.yaml \
  claracore:cuda \
  --models-folder /models \
  --backend cuda
```

### With custom settings
```bash
docker run -d \
  --name claracore \
  --gpus all \
  -p 5800:5800 \
  -v $(pwd)/models:/models \
  -e GIN_MODE=debug \
  claracore:cuda \
  --models-folder /models \
  --vram 24 \
  --ram 64 \
  --jinja=true
```

## 🔗 Links

- [Main README](../README.md)
- [ClaraCore Documentation](../docs/)
- [GitHub Repository](https://github.com/claraverse-space/ClaraCore)

---

For detailed setup instructions, see [CONTAINER_SETUP.md](./CONTAINER_SETUP.md)

# ClaraCore Docker - CPU Edition

Clean Docker setup for ClaraCore with CPU-only inference using llama.cpp's official CPU image.

## Prerequisites

- Docker installed
- Docker Compose installed

No GPU required! This version runs on any CPU.

## Quick Start

### Option 1: Pull Pre-built Image (Recommended)

```bash
# Pull the image from Docker Hub
docker pull clara17verse/claracore:cpu

# Run with docker-compose
cd docker-cpu
docker-compose up -d
```

### Option 2: Run Directly with Docker

```bash
docker run -d \
  --name claracore-cpu \
  -p 5890:5890 \
  -v claracore-cpu-downloads:/app/downloads \
  clara17verse/claracore:cpu
```

### Option 3: Run with Custom Model Folder

```bash
docker run -d \
  --name claracore-cpu \
  -p 5890:5890 \
  -v claracore-cpu-downloads:/app/downloads \
  -v /path/to/your/models:/app/models:ro \
  clara17verse/claracore:cpu
```

**Access ClaraCore:** http://localhost:5890/ui/

✅ **Your models are saved** in Docker volume `claracore-cpu-downloads`

## View Logs

```bash
docker-compose logs -f
```

## Stop Container

```bash
docker-compose down
```

## Performance Tips

### CPU Thread Optimization
The container auto-detects available CPU cores. For better performance with large models:

```bash
# Set specific thread count
docker run -d \
  --name claracore-cpu \
  -p 5890:5890 \
  -v claracore-cpu-downloads:/app/downloads \
  -e OMP_NUM_THREADS=8 \
  claracore:cpu
```

### Resource Limits
Limit CPU usage to prevent system overload:

```yaml
# Add to docker-compose.yml under 'claracore' service:
deploy:
  resources:
    limits:
      cpus: '8'
      memory: 16G
    reservations:
      cpus: '4'
      memory: 8G
```

## Building the Image

### Build locally:
```bash
# From ClaraCore root directory
cd /home/bb17g/claracore/ClaraCore

# Build ClaraCore binary for Linux
GOOS=linux GOARCH=amd64 go build -o claracore .

# Build Docker image
cd docker-cpu
docker build -f Dockerfile.cpu -t claracore:cpu ..

# Or use docker-compose
docker-compose build
```

### Build for different architectures:
```bash
# For ARM64 (Raspberry Pi, Apple Silicon)
GOOS=linux GOARCH=arm64 go build -o claracore .
docker build -f Dockerfile.cpu -t claracore:cpu-arm64 ..

# For AMD64 (Intel/AMD x86-64)
GOOS=linux GOARCH=amd64 go build -o claracore .
docker build -f Dockerfile.cpu -t claracore:cpu-amd64 ..
```

## Multi-arch Build

Build for multiple architectures and push to Docker Hub:

```bash
# Enable buildx
docker buildx create --use

# Build and push multi-arch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f docker-cpu/Dockerfile.cpu \
  -t yourusername/claracore:cpu \
  --push \
  .
```

## Data Persistence

✅ **Your downloaded models are safe!** The `/app/downloads` folder is stored in Docker volume `claracore-cpu-downloads`:
- **Models** you download through ClaraCore
- Persists even after `docker-compose down`

**⚠️ Your downloads will ONLY be deleted if you run:** `docker-compose down -v`

## Using Your Own Model Folders

You can easily bind-mount your existing model folders into the container!

### Edit docker-compose.yml:
```yaml
volumes:
  - claracore-cpu-downloads:/app/downloads
  - /path/to/your/models:/app/models:ro          # Add your path here
  - /mnt/storage/llm-models:/app/external:ro     # Multiple folders supported
```

**Tips:**
- Use `:ro` (read-only) to prevent accidental modifications
- Mount multiple folders to different paths
- After mounting, go to Setup in ClaraCore UI and add these folders (e.g., `/app/models`, `/app/external`)

## Managing Your Data

**View volume location:**
```bash
docker volume inspect claracore-cpu-downloads
```

**View what's inside:**
```bash
docker exec claracore-cpu ls -lh /app/downloads
```

**Backup your downloads:**
```bash
docker run --rm -v claracore-cpu-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar czf /backup/claracore-cpu-downloads-backup.tar.gz -C /downloads .
```

**Restore from backup:**
```bash
docker run --rm -v claracore-cpu-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar xzf /backup/claracore-cpu-downloads-backup.tar.gz -C /downloads
```

## Troubleshooting

**Port already in use?** Edit `docker-compose.yml` and change `5890:5890` to `5891:5890`

**Slow inference?** Use smaller quantized models (Q4_K_M or Q5_K_M) for better CPU performance

**Out of memory?** 
- Reduce context size in model settings
- Use lower quantization (Q2_K, Q3_K_M)
- Add memory limits in docker-compose.yml

## Recommended Models for CPU

For best CPU performance, use these quantization formats:
- **Q4_K_M** - Good balance of quality and speed
- **Q5_K_M** - Better quality, still fast
- **Q3_K_M** - Faster inference, lower quality
- **Q2_K** - Maximum speed, minimal quality loss

Small models that work well on CPU:
- Qwen2.5-0.5B-Instruct
- Phi-3-mini
- TinyLlama
- Gemma-2B

## Comparing with GPU Versions

| Feature | CPU | CUDA | ROCm |
|---------|-----|------|------|
| Hardware | Any CPU | NVIDIA GPU | AMD GPU |
| Speed | Slowest | Fastest | Fast |
| Memory | System RAM | VRAM | VRAM |
| Cost | Free | $$ | $$ |
| Availability | Universal | NVIDIA only | AMD only |

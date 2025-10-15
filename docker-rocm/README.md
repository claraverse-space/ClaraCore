# ClaraCore Docker - ROCm Edition

Clean Docker setup for ClaraCore with AMD GPU acceleration using llama.cpp's official ROCm image.

## Prerequisites

- Docker installed
- Docker Compose installed
- AMD GPU (Radeon RX 6000/7000 series, MI series, or compatible)
- ROCm drivers installed on host system

## Supported AMD GPUs

ROCm officially supports:
- **Radeon RX 7900 XTX/XT** (gfx1100)
- **Radeon RX 6900/6950 XT** (gfx1030)
- **Radeon RX 6800/6800 XT** (gfx1030)
- **Radeon RX 6700 XT** (gfx1031)
- **Radeon VII** (gfx906)
- **AMD Instinct MI series** (data center GPUs)

For unofficial support on other AMD GPUs (RX 5000, RX 500, etc.), see "GPU Override" section below.

## Quick Start

```bash
cd docker-rocm
docker-compose up -d
```

That's it! ClaraCore will:
- Download llama-server binaries automatically
- Detect your AMD GPU
- Start with full ROCm acceleration

Access at: **http://localhost:5890/ui/**

## View Logs

```bash
docker-compose logs -f
```

## Stop Container

```bash
docker-compose down
```

## GPU Override for Unsupported Cards

If your AMD GPU is not officially supported, you can override the GFX version. Find your GPU's architecture:

```bash
# Check your GPU
rocminfo | grep gfx
# or
/opt/rocm/bin/rocminfo | grep gfx
```

Common overrides:
- **RX 6000 series (RDNA2)**: `HSA_OVERRIDE_GFX_VERSION=10.3.0` (gfx1030)
- **RX 7000 series (RDNA3)**: `HSA_OVERRIDE_GFX_VERSION=11.0.0` (gfx1100)
- **RX 5000 series (RDNA1)**: `HSA_OVERRIDE_GFX_VERSION=10.1.0` (gfx1010)
- **Vega 64/56**: `HSA_OVERRIDE_GFX_VERSION=9.0.0` (gfx900)
- **Radeon VII**: `HSA_OVERRIDE_GFX_VERSION=9.0.6` (gfx906)

Edit `docker-compose.yml` and update:
```yaml
environment:
  - HSA_OVERRIDE_GFX_VERSION=10.3.0  # Change to your GPU's version
```

## Installing ROCm Drivers

### Ubuntu/Debian:
```bash
# Add ROCm repository
wget https://repo.radeon.com/rocm/rocm.gpg.key -O - | gpg --dearmor | sudo tee /etc/apt/keyrings/rocm.gpg > /dev/null

echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/rocm/apt/6.0 jammy main" | sudo tee /etc/apt/sources.list.d/rocm.list

sudo apt update
sudo apt install rocm-hip-sdk rocm-opencl-sdk

# Add user to video and render groups
sudo usermod -a -G video,render $USER

# Reboot to apply changes
sudo reboot
```

### Arch Linux:
```bash
yay -S rocm-hip-sdk rocm-opencl-runtime
sudo usermod -a -G video,render $USER
sudo reboot
```

## Verify ROCm Installation

Check if ROCm detects your GPU:
```bash
rocm-smi
# or
rocminfo | grep "Name:"
```

Check inside container:
```bash
docker exec claracore-rocm rocm-smi
```

## Building the Image

### Build locally:
```bash
# From ClaraCore root directory
cd /home/bb17g/claracore/ClaraCore

# Build ClaraCore binary for Linux
GOOS=linux GOARCH=amd64 go build -o claracore .

# Build Docker image
cd docker-rocm
docker build -f Dockerfile.rocm -t claracore:rocm ..

# Or use docker-compose
docker-compose build
```

### Push to Docker Hub:
```bash
# Tag for Docker Hub
docker tag claracore:rocm yourusername/claracore:rocm

# Login and push
docker login
docker push yourusername/claracore:rocm
```

## Testing Without AMD GPU

If you want to build the ROCm image but don't have an AMD GPU to test:

```bash
# Build the image (this works on any system)
docker build -f docker-rocm/Dockerfile.rocm -t claracore:rocm .

# Test that it builds successfully
docker images | grep claracore

# You can't run it without ROCm hardware, but you can verify:
# 1. Image builds without errors
# 2. ClaraCore binary is included
# 3. Entry point is correct

docker run --rm claracore:rocm --version
```

The container will fail to start without AMD GPU hardware, but you can:
1. ✅ Build the image successfully
2. ✅ Push it to Docker Hub
3. ✅ Let others with AMD GPUs test it
4. ❌ Cannot verify GPU acceleration without AMD hardware

## Data Persistence

✅ **Your downloaded models are safe!** The `/app/downloads` folder is stored in Docker volume `claracore-rocm-downloads`:
- **Models** you download through ClaraCore
- Persists even after `docker-compose down`

**⚠️ Your downloads will ONLY be deleted if you run:** `docker-compose down -v`

## Using Your Own Model Folders

You can easily bind-mount your existing model folders into the container!

### Edit docker-compose.yml:
```yaml
volumes:
  - claracore-rocm-downloads:/app/downloads
  - /path/to/your/models:/app/models:ro          # Add your path here
  - /mnt/storage/llm-models:/app/external:ro     # Multiple folders supported
```

**Tips:**
- Use `:ro` (read-only) to prevent accidental modifications
- Mount multiple folders to different paths
- After mounting, go to Setup in ClaraCore UI and add these folders

## Managing Your Data

**View volume location:**
```bash
docker volume inspect claracore-rocm-downloads
```

**View what's inside:**
```bash
docker exec claracore-rocm ls -lh /app/downloads
```

**Backup your downloads:**
```bash
docker run --rm -v claracore-rocm-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar czf /backup/claracore-rocm-downloads-backup.tar.gz -C /downloads .
```

## Troubleshooting

### GPU not detected?

1. **Check ROCm installation:**
```bash
rocm-smi
rocminfo
```

2. **Verify user groups:**
```bash
groups $USER
# Should include 'video' and 'render'
```

3. **Check device permissions:**
```bash
ls -l /dev/kfd /dev/dri
# Should be accessible to video/render groups
```

4. **Test with simple container:**
```bash
docker run --rm -it --device=/dev/kfd --device=/dev/dri --group-add video --group-add render rocm/rocm-terminal rocminfo
```

### Performance issues?

1. **Check GPU utilization:**
```bash
watch -n 1 rocm-smi
```

2. **Monitor inside container:**
```bash
docker exec claracore-rocm rocm-smi
```

3. **Check if using GPU:**
Look for "GPU layers: XX" in ClaraCore logs

### Common Errors

**Error: "HSA Error: Invalid argument"**
- Your GPU may need `HSA_OVERRIDE_GFX_VERSION` set
- See "GPU Override" section above

**Error: "failed to initialize hipBLAS"**
- ROCm drivers not installed or outdated
- Update to ROCm 6.0 or newer

**Error: "No ROCm-capable device found"**
- Device permissions issue
- Add user to video/render groups: `sudo usermod -a -G video,render $USER`

## Comparing GPU Backends

| Feature | CPU | CUDA (NVIDIA) | ROCm (AMD) |
|---------|-----|---------------|------------|
| Speed | Slowest | Fastest | Fast |
| Setup | Easiest | Easy | Moderate |
| Compatibility | Universal | NVIDIA only | AMD only |
| Driver Support | N/A | Mature | Improving |
| Model Size | Limited by RAM | Limited by VRAM | Limited by VRAM |

## Known Limitations

- ROCm support varies by GPU model
- Some older AMD GPUs require overrides
- Performance may be lower than NVIDIA CUDA
- ROCm is primarily developed for Linux (limited Windows support)

## Getting Help

If you encounter issues:
1. Check ROCm version: `rocminfo --version`
2. Check GPU: `rocm-smi`
3. Check logs: `docker-compose logs -f`
4. Search ROCm issues: https://github.com/ROCm/ROCm/issues
5. Check llama.cpp ROCm support: https://github.com/ggerganov/llama.cpp

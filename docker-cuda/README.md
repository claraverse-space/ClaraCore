# ClaraCore Docker - CUDA Edition

Clean Docker setup for ClaraCore with NVIDIA GPU acceleration using llama.cpp's official CUDA image.

## Prerequisites

- Docker installed
- Docker Compose installed
- NVIDIA GPU with drivers installed
- NVIDIA Container Toolkit installed

## Quick Start

### Option 1: Pull Pre-built Image (Recommended)

```bash
# Pull the image from Docker Hub
docker pull clara17verse/claracore:cuda

# Run with docker-compose
cd docker-cuda
docker-compose up -d
```

### Option 2: Run Directly with Docker

```bash
docker run -d \
  --name claracore-cuda \
  --gpus all \
  -p 5890:5890 \
  -v claracore-cuda-downloads:/app/downloads \
  clara17verse/claracore:cuda
```

### Option 3: Run with Custom Model Folder

```bash
docker run -d \
  --name claracore-cuda \
  --gpus all \
  -p 5890:5890 \
  -v claracore-cuda-downloads:/app/downloads \
  -v /path/to/your/models:/app/models:ro \
  clara17verse/claracore:cuda
```

**Access ClaraCore:** http://localhost:5890/ui/

✅ **Your models are saved** in Docker volume `claracore-cuda-downloads`

## View Logs

```bash
docker-compose logs -f
```

## Stop Container

```bash
docker-compose down
```

## How It Works

- Uses `ghcr.io/ggml-org/llama.cpp:server-cuda` as base (official llama.cpp CUDA image)
- Adds ClaraCore binary on top
- **Persistent Docker volume** stores all data (models, config, binaries)
- ClaraCore handles everything automatically

## Data Persistence

✅ **Your downloaded models are safe!** The `/app/downloads` folder is stored in Docker volume `claracore-downloads`:
- **Models** you download through ClaraCore
- Persists even after `docker-compose down`

**⚠️ Your downloads will ONLY be deleted if you run:** `docker-compose down -v`

## Using Your Own Model Folders

You can easily bind-mount your existing model folders into the container! Two ways to do this:

### Option 1: Edit docker-compose.yml directly

Uncomment and customize the volume binds in `docker-compose.yml`:
```yaml
volumes:
  - claracore-downloads:/app/downloads
  - /path/to/your/models:/app/models:ro          # Add your path here
  - /mnt/storage/llm-models:/app/external:ro     # Multiple folders supported
```

### Option 2: Use docker-compose.override.yml (Recommended)

This keeps your custom settings separate from the main compose file:

```bash
# Copy the example file
cp docker-compose.override.example.yml docker-compose.override.yml

# Edit it with your paths
nano docker-compose.override.yml

# Restart - it automatically merges with docker-compose.yml
docker-compose down
docker-compose up -d
```

Example override file:
```yaml
name: claracore

services:
  claracore:
    volumes:
      - /home/myuser/models:/app/models:ro
      - /mnt/nas/llm-collection:/app/nas:ro
```

**Tips:**
- Use `:ro` (read-only) to prevent accidental modifications
- Mount multiple folders to different paths
- After mounting, go to Setup in ClaraCore UI and add these folders (e.g., `/app/models`, `/app/nas`)
- Paths work on Linux, Windows (with WSL2), and macOS

### Managing Your Data

**View volume location:**
```bash
docker volume inspect claracore-downloads
```

**View what's inside:**
```bash
docker exec claracore ls -lh /app/downloads
```

**Backup your downloads:**
```bash
# Create tar archive of downloads
docker run --rm -v claracore-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar czf /backup/claracore-downloads-backup.tar.gz -C /downloads .
```

**Restore from backup:**
```bash
docker run --rm -v claracore-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar xzf /backup/claracore-downloads-backup.tar.gz -C /downloads
```

**Clean downloads and start fresh:**
```bash
docker-compose down -v  # ⚠️ Deletes downloaded models!
docker-compose up -d
```

## Verify GPU

Check GPU is accessible:
```bash
docker exec claracore nvidia-smi
```

## ⚠️ Windows (WSL2) Performance Issues

If you're running Docker in WSL2 and mounting Windows drives (like `C:\` or `D:\`), you may experience **slow model scanning** due to filesystem overhead.

### Solution Options:

#### 1️⃣ Use Docker Volume (Fastest - Recommended)
Keep models in Linux filesystem for optimal performance:

```bash
# Create volume
docker volume create claracore-models

# Run container with volume
docker run -d \
  --name claracore \
  --gpus all \
  -p 5890:5890 \
  -v claracore-models:/app/downloads \
  -e NVIDIA_VISIBLE_DEVICES=all \
  -e NVIDIA_DRIVER_CAPABILITIES=compute,utility \
  clara17verse/claracore:cuda

# Copy models from Windows (one-time operation)
docker run --rm \
  -v claracore-models:/app/downloads \
  -v /mnt/c/path/to/your/models:/source \
  alpine sh -c "cp -r /source/* /app/downloads/"
```

#### 2️⃣ Use WSL2 Home Directory
```bash
# Copy models to Linux filesystem first
mkdir -p ~/llama-models
cp -r /mnt/c/YourModelsFolder/* ~/llama-models/

# Then mount the Linux path
docker run -d \
  --name claracore \
  --gpus all \
  -p 5890:5890 \
  -v ~/llama-models:/app/downloads \
  -e NVIDIA_VISIBLE_DEVICES=all \
  -e NVIDIA_DRIVER_CAPABILITIES=compute,utility \
  clara17verse/claracore:cuda
```

#### 3️⃣ Direct Windows Mount (Simplest but Slower)
```bash
# Use Windows path directly (e.g., /mnt/c/BackUP/models)
docker run -d \
  --name claracore \
  --gpus all \
  -p 5890:5890 \
  -v /mnt/c/YourModelsFolder:/app/downloads \
  -e NVIDIA_VISIBLE_DEVICES=all \
  -e NVIDIA_DRIVER_CAPABILITIES=compute,utility \
  clara17verse/claracore:cuda
```

**Why is Windows mount slower?**
- WSL2 filesystem layer adds overhead when accessing Windows drives
- Scanning large GGUF files (GB in size) through `/mnt/c` is significantly slower
- Docker volumes or WSL2 native filesystem avoid this overhead

## Troubleshooting

**GPU not detected?** Install NVIDIA Container Toolkit:
```bash
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
  sudo tee /etc/apt/sources.list.d/nvidia-docker.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

**Port already in use?** Edit `docker-compose.yml` and change `5800:5800` to `5801:5800`

# 🚀 ClaraCore Docker - 30 Second Quickstart

## Build & Run (One Command)

```bash
cd docker && ./build-universal.sh && docker compose -f docker-compose.universal.yml up
```

That's it! Access at: **http://localhost:5800/ui/**

---

## Step by Step

### 1. Build the container

```bash
cd docker
./build-universal.sh  # or .\build-universal.ps1 on Windows
```

### 2. Add models

```bash
# Copy your GGUF model files to docker/models/
cp /path/to/your/model.gguf ./models/
```

### 3. Start it

```bash
docker compose -f docker-compose.universal.yml up
```

### 4. Use it

Open: **http://localhost:5800/ui/**

---

## What Just Happened?

The container:
- ✅ Detected your hardware (NVIDIA/AMD/Intel GPU or CPU)
- ✅ Picked the fastest backend automatically
- ✅ Scanned your models
- ✅ Generated optimal configuration
- ✅ Started the server

**Zero configuration required!**

---

## Need Different Settings?

### Force CPU mode
```bash
docker compose -f docker-compose.cpu-only.yml up
```

### Force CUDA (NVIDIA)
```bash
docker compose -f docker-compose.cuda-explicit.yml up
```

### Force ROCm (AMD)
```bash
docker compose -f docker-compose.rocm-explicit.yml up
```

### Force Vulkan (Universal)
```bash
docker compose -f docker-compose.vulkan-explicit.yml up
```

---

## Troubleshooting

**Container uses CPU but I have a GPU:**
- NVIDIA: Install [nvidia-container-toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
- AMD: Install [ROCm drivers](https://rocm.docs.amd.com/en/latest/deploy/linux/quick_start.html)

**No models detected:**
```bash
# Make sure you have .gguf files in docker/models/
ls -la docker/models/*.gguf
```

**Check what was detected:**
```bash
docker logs claracore | grep "Selected Backend"
```

---

## More Info

- **Full guide:** [DEPLOYMENT.md](DEPLOYMENT.md)
- **Technical details:** [README-UNIVERSAL.md](README-UNIVERSAL.md)
- **Docker folder:** All compose files and configs

---

**Enjoy your containerized ClaraCore!** 🎉

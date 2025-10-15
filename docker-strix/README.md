````markdown
# ClaraCore Docker - AMD Strix Halo Edition

Docker setup for ClaraCore optimized for **AMD Strix Halo APUs** (Ryzen AI Max 300 series).

## Prerequisites

- Docker and Docker Compose installed
- AMD Strix Halo APU (Ryzen AI Max+ 395, 390, or 385)
- Linux OS (Ubuntu 24.04 or Fedora 42+ recommended)

## Host System Setup (REQUIRED for GPU Access)

### Ubuntu 24.04

```bash
# Create udev rules for GPU access
sudo tee /etc/udev/rules.d/99-amd-kfd.rules > /dev/null <<EOF
SUBSYSTEM=="kfd", GROUP="render", MODE="0666", OPTIONS+="last_rule"
SUBSYSTEM=="drm", KERNEL=="card[0-9]*", GROUP="render", MODE="0666", OPTIONS+="last_rule"
EOF

# Reload and apply
sudo udevadm control --reload-rules
sudo udevadm trigger
sudo usermod -a -G video,render $USER
sudo reboot
```

### Fedora 42+ (Optional - for best performance)

```bash
# Edit /etc/default/grub and add to GRUB_CMDLINE_LINUX:
amd_iommu=off amdgpu.gttsize=131072 ttm.pages_limit=33554432

# Apply and reboot
sudo grub2-mkconfig -o /boot/grub2/grub.cfg
sudo reboot
```

## Quick Start

### Using Pre-built Image (Recommended)

```bash
docker pull clara17verse/claracore:strix
cd docker-strix
docker-compose up -d
```

### Building from Source

```bash
cd docker-strix
docker-compose build
docker-compose up -d
```

**Access ClaraCore:** http://localhost:5890/ui/

## Docker Commands

```bash
# View logs
docker-compose logs -f

# Stop container
docker-compose down

# Restart
docker-compose restart

# Check status
docker-compose ps
```

## Run with Docker CLI

```bash
docker run -d \
  --name claracore-strix \
  --device=/dev/dri \
  --group-add video \
  --security-opt seccomp=unconfined \
  -p 5890:5890 \
  -v claracore-strix-downloads:/app/downloads \
  clara17verse/claracore:strix
```

## Troubleshooting

### GPU Not Detected?

```bash
# Check GPU devices
ls -l /dev/dri /dev/kfd

# Verify user groups
groups  # Should show: video render

# Check logs
docker-compose logs
```

### Ubuntu: Permission Denied?

You must create udev rules (see [Host System Setup](#host-system-setup-required-for-gpu-access)).

### Performance Issues?

```bash
# Monitor container
docker stats claracore-strix

# Check system memory
free -h

# Verify kernel parameters (Fedora)
cat /proc/cmdline | grep amdgpu
```

### Container Falls Back to CPU?

- Check `/dev/dri` is mounted in container
- Verify video/render group membership
- Reboot after group changes

## Data Persistence

✅ **Downloaded models are saved** in Docker volume `claracore-strix-downloads`

**View volume:**
```bash
docker volume inspect claracore-strix-downloads
```

**Backup:**
```bash
docker run --rm -v claracore-strix-downloads:/downloads -v $(pwd):/backup ubuntu \
  tar czf /backup/claracore-strix-backup.tar.gz -C /downloads .
```

## Using Your Own Model Folders

Edit `docker-compose.yml`:
```yaml
volumes:
  - claracore-strix-downloads:/app/downloads
  - /path/to/your/models:/app/models:ro
```

## Additional Information

- **Base image:** kyuz0/amd-strix-halo-toolboxes:vulkan-radv
- **Backend:** Vulkan RADV (Mesa driver)
- **Docker Hub:** clara17verse/claracore:strix
- **Port:** 5890

**For more details on Strix Halo optimization:** [kyuz0/amd-strix-halo-toolboxes](https://github.com/kyuz0/amd-strix-halo-toolboxes)

````

`````

````

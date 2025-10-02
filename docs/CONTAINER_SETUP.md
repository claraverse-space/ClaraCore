# ClaraCore in Containers & WSL

This guide helps you run ClaraCore in environments without systemd (containers, WSL, etc.).

## Quick Start

If the installer couldn't set up systemd service, you can still run ClaraCore manually:

```bash
# If 'claracore' command not found, add to PATH:
export PATH="$HOME/.local/bin:$PATH"

# Start ClaraCore directly
claracore --config ~/.config/claracore/config.yaml

# Check version (should show proper version info as of v0.1.1+)
claracore --version

# Or with custom models folder
claracore --models-folder /path/to/models

# Background execution
nohup claracore --config ~/.config/claracore/config.yaml > ~/.config/claracore/claracore.log 2>&1 &
```

**Note**: If you get "command not found", either restart your terminal or run the export command above.

**Version Info**: As of v0.1.1, ClaraCore binaries now show proper version information instead of placeholder values.

## Docker Container

Create a simple Dockerfile for ClaraCore:

```dockerfile
FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install ClaraCore
RUN curl -fsSL https://raw.githubusercontent.com/claraverse-space/ClaraCore/main/scripts/install.sh | bash

# Create volume for models and config
VOLUME ["/app/models", "/app/config"]

# Expose default port
EXPOSE 5800

# Start ClaraCore
CMD ["claracore", "--config", "/app/config/config.yaml", "--models-folder", "/app/models"]
```

Build and run:
```bash
docker build -t claracore .
docker run -p 5800:5800 -v ./models:/app/models -v ./config:/app/config claracore
```

## WSL (Windows Subsystem for Linux)

ClaraCore works perfectly in WSL:

```bash
# Install normally
curl -fsSL https://raw.githubusercontent.com/claraverse-space/ClaraCore/main/scripts/install.sh | bash

# Start manually (systemd not available in WSL1)
claracore --config ~/.config/claracore/config.yaml

# Access from Windows browser
# http://localhost:5800/ui
```

## Process Manager Alternatives

### Using screen/tmux
```bash
# Start in background session
screen -dmS claracore claracore --config ~/.config/claracore/config.yaml
# or
tmux new-session -d -s claracore 'claracore --config ~/.config/claracore/config.yaml'

# Reattach to session
screen -r claracore
# or
tmux attach -t claracore
```

### Using PM2 (Node.js process manager)
```bash
# Install PM2
npm install -g pm2

# Create ecosystem file
cat > ecosystem.config.js << EOF
module.exports = {
  apps: [{
    name: 'claracore',
    script: 'claracore',
    args: '--config ~/.config/claracore/config.yaml',
    cwd: '~',
    instances: 1,
    autorestart: true,
    watch: false,
    max_memory_restart: '1G',
  }]
}
EOF

# Start with PM2
pm2 start ecosystem.config.js
pm2 save
pm2 startup
```

## Configuration

Your configuration files are located at:
- **Config**: `~/.config/claracore/config.yaml`
- **Settings**: `~/.config/claracore/settings.json`
- **Logs**: `~/.config/claracore/logs/`

## Troubleshooting

### Port Already in Use
```bash
# Check what's using port 5800
lsof -i :5800
netstat -tulpn | grep 5800

# Kill process if needed
kill -9 <PID>
```

### Permission Issues
```bash
# Fix permissions
chmod +x ~/.local/bin/claracore
chown -R $USER:$USER ~/.config/claracore
```

### Missing Dependencies
```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y curl ca-certificates

# CentOS/RHEL
sudo yum install -y curl ca-certificates

# Alpine
sudo apk add --no-cache curl ca-certificates
```

## Auto-start Solutions

### Cron (runs at reboot)
```bash
# Edit crontab
crontab -e

# Add line:
@reboot /home/$USER/.local/bin/claracore --config /home/$USER/.config/claracore/config.yaml >> /home/$USER/.config/claracore/startup.log 2>&1 &
```

### Custom init script
```bash
#!/bin/bash
# ~/.config/autostart/claracore.sh

cd "$HOME"
export PATH="$HOME/.local/bin:$PATH"
nohup claracore --config "$HOME/.config/claracore/config.yaml" > "$HOME/.config/claracore/startup.log" 2>&1 &
```

Make executable and run at startup:
```bash
chmod +x ~/.config/autostart/claracore.sh
```

## Support

- **Documentation**: https://github.com/claraverse-space/ClaraCore/tree/main/docs
- **Issues**: https://github.com/claraverse-space/ClaraCore/issues
- **Discussions**: https://github.com/claraverse-space/ClaraCore/discussions
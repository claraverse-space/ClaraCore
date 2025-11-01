#!/bin/bash

# ClaraCore Installation Script
# Supports Linux and macOS with automatic service setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
    REPO="claraverse-space/ClaraCore"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/claracore"
SERVICE_NAME="claracore"

# Platform detection
detect_platform() {
    case "$(uname -s)" in
        Linux*)     
            PLATFORM="linux"
            ARCH=$(uname -m)
            case $ARCH in
                x86_64) ARCH="amd64" ;;
                aarch64|arm64) ARCH="arm64" ;;
                armv7l) ARCH="arm" ;;
                *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
            esac
            ;;
        Darwin*)    
            PLATFORM="darwin"
            ARCH=$(uname -m)
            case $ARCH in
                x86_64) ARCH="amd64" ;;
                arm64) ARCH="arm64" ;;
                *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
            esac
            ;;
        *)          
            echo -e "${RED}Unsupported platform: $(uname -s)${NC}"
            exit 1
            ;;
    esac
    echo -e "${BLUE}Detected platform: $PLATFORM-$ARCH${NC}"
}

# Check if running as root for system-wide install
check_permissions() {
    if [[ $EUID -eq 0 ]]; then
        INSTALL_DIR="/usr/local/bin"
        SYSTEMD_DIR="/etc/systemd/system"
        LAUNCHD_DIR="/Library/LaunchDaemons"
        SYSTEM_INSTALL=true
    else
        INSTALL_DIR="$HOME/.local/bin"
        SYSTEMD_DIR="$HOME/.config/systemd/user"
        LAUNCHD_DIR="$HOME/Library/LaunchAgents"
        SYSTEM_INSTALL=false
    fi
    
    # Ensure directories exist
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    
    if [[ "$PLATFORM" == "linux" ]]; then
        mkdir -p "$SYSTEMD_DIR"
    elif [[ "$PLATFORM" == "darwin" ]]; then
        mkdir -p "$LAUNCHD_DIR"
    fi
}

# Get latest release info
get_latest_release() {
    echo -e "${BLUE}Fetching latest release information...${NC}"
    
    if command -v curl >/dev/null 2>&1; then
        LATEST_RELEASE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        LATEST_RELEASE=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo -e "${RED}Error: curl or wget is required${NC}"
        exit 1
    fi
    
    if [[ -z "$LATEST_RELEASE" ]]; then
        echo -e "${RED}Error: Could not fetch latest release${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Latest release: $LATEST_RELEASE${NC}"
}

# Download and install binary
download_binary() {
    BINARY_NAME="claracore-$PLATFORM-$ARCH"
    if [[ "$PLATFORM" == "darwin" ]]; then
        DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$BINARY_NAME"
    else
        DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$BINARY_NAME"
    fi
    
    echo -e "${BLUE}Downloading ClaraCore binary...${NC}"
    echo -e "${YELLOW}URL: $DOWNLOAD_URL${NC}"
    
    TEMP_FILE=$(mktemp)
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$TEMP_FILE" "$DOWNLOAD_URL"
    fi
    
    if [[ ! -f "$TEMP_FILE" ]] || [[ ! -s "$TEMP_FILE" ]]; then
        echo -e "${RED}Error: Failed to download binary${NC}"
        exit 1
    fi
    
    # Install binary
    echo -e "${BLUE}Installing binary to $INSTALL_DIR/claracore...${NC}"
    chmod +x "$TEMP_FILE"
    
    if [[ "$SYSTEM_INSTALL" == true ]]; then
        mv "$TEMP_FILE" "$INSTALL_DIR/claracore"
    else
        mv "$TEMP_FILE" "$INSTALL_DIR/claracore"
        # Add to PATH if not already there
        if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
            echo -e "${BLUE}Adding ~/.local/bin to PATH...${NC}"
            
            # Add to shell configuration files
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc" 2>/dev/null || true
            
            # Also try common profile files
            [[ -f "$HOME/.profile" ]] && echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.profile"
            
            # Export for current session
            export PATH="$HOME/.local/bin:$PATH"
            
            # Try to source bashrc for current session if running interactively
            if [[ -t 0 ]] && [[ -f "$HOME/.bashrc" ]]; then
                echo -e "${BLUE}Updating current session...${NC}"
                source "$HOME/.bashrc" 2>/dev/null || true
            fi
            
            echo -e "${GREEN}PATH updated. You may need to restart your terminal or run: source ~/.bashrc${NC}"
        else
            echo -e "${GREEN}~/.local/bin already in PATH${NC}"
        fi
    fi
    
    echo -e "${GREEN}Binary installed successfully${NC}"
    
    # Test if binary works and is in PATH
    if command -v claracore >/dev/null 2>&1; then
        echo -e "${GREEN}✓ claracore command is accessible${NC}"
    else
        echo -e "${YELLOW}⚠ claracore not yet in PATH for this session${NC}"
    fi
}

# Create default configuration
create_config() {
    echo -e "${BLUE}Creating default configuration...${NC}"
    
    cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# ClaraCore Configuration
# This file is auto-generated. You can modify it or regenerate via the web UI.

host: "127.0.0.1"
port: 5800
cors: true
api_key: ""

# Models will be auto-discovered and configured
models: []

# Model groups for memory management
groups: {}
EOF

    cat > "$CONFIG_DIR/settings.json" << 'EOF'
{
  "gpuType": "auto",
  "backend": "auto",
  "vramGB": 0,
  "ramGB": 0,
  "preferredContext": 8192,
  "throughputFirst": true,
  "enableJinja": true,
  "requireApiKey": false,
  "apiKey": ""
}
EOF

    echo -e "${GREEN}Default configuration created in $CONFIG_DIR${NC}"
}

# Setup Linux systemd service
setup_linux_service() {
    # Check if systemd is available
    if ! command -v systemctl >/dev/null 2>&1; then
        echo -e "${YELLOW}Systemd not available - skipping service setup${NC}"
        echo -e "${YELLOW}You can manually start ClaraCore with: claracore --config $CONFIG_DIR/config.yaml${NC}"
        return 0
    fi
    
    # Test if systemd is running
    if ! systemctl is-system-running >/dev/null 2>&1; then
        echo -e "${YELLOW}Systemd not running (possibly in container/WSL) - skipping service setup${NC}"
        echo -e "${YELLOW}You can manually start ClaraCore with: claracore --config $CONFIG_DIR/config.yaml${NC}"
        return 0
    fi
    
    echo -e "${BLUE}Setting up systemd service...${NC}"
    
    SERVICE_FILE="$SYSTEMD_DIR/$SERVICE_NAME.service"
    
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=ClaraCore AI Inference Server
After=network.target
Wants=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$CONFIG_DIR
ExecStart=$INSTALL_DIR/claracore --config $CONFIG_DIR/config.yaml
Restart=always
RestartSec=3
Environment=HOME=$HOME
Environment=USER=$USER

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=$CONFIG_DIR $HOME/models

[Install]
WantedBy=default.target
EOF

    if [[ "$SYSTEM_INSTALL" == true ]]; then
        if systemctl daemon-reload 2>/dev/null && systemctl enable "$SERVICE_NAME" 2>/dev/null; then
            echo -e "${GREEN}✓ System service enabled (auto-start on boot)${NC}"

            # Start the service immediately
            echo -e "${BLUE}Starting ClaraCore service...${NC}"
            if systemctl start "$SERVICE_NAME" 2>/dev/null; then
                echo -e "${GREEN}✓ Service started successfully${NC}"

                # Wait a moment for service to initialize
                sleep 2

                # Check service status
                if systemctl is-active --quiet "$SERVICE_NAME"; then
                    echo -e "${GREEN}✓ Service is running${NC}"
                else
                    echo -e "${YELLOW}⚠ Service may not be running properly. Check: sudo systemctl status $SERVICE_NAME${NC}"
                fi
            else
                echo -e "${YELLOW}⚠ Failed to start service automatically${NC}"
                echo -e "${YELLOW}  Manual start: sudo systemctl start $SERVICE_NAME${NC}"
            fi
        else
            echo -e "${YELLOW}Failed to enable system service. You may need to run with sudo or start manually.${NC}"
            echo -e "${YELLOW}Manual start: sudo $INSTALL_DIR/claracore --config $CONFIG_DIR/config.yaml${NC}"
        fi
    else
        if systemctl --user daemon-reload 2>/dev/null && systemctl --user enable "$SERVICE_NAME" 2>/dev/null; then
            echo -e "${GREEN}✓ User service enabled (auto-start on login)${NC}"

            # Start the service immediately
            echo -e "${BLUE}Starting ClaraCore service...${NC}"
            if systemctl --user start "$SERVICE_NAME" 2>/dev/null; then
                echo -e "${GREEN}✓ Service started successfully${NC}"

                # Wait a moment for service to initialize
                sleep 2

                # Check service status
                if systemctl --user is-active --quiet "$SERVICE_NAME"; then
                    echo -e "${GREEN}✓ Service is running${NC}"
                else
                    echo -e "${YELLOW}⚠ Service may not be running properly. Check: systemctl --user status $SERVICE_NAME${NC}"
                fi
            else
                echo -e "${YELLOW}⚠ Failed to start service automatically${NC}"
                echo -e "${YELLOW}  Manual start: systemctl --user start $SERVICE_NAME${NC}"
            fi
        else
            echo -e "${YELLOW}Failed to enable user service. Starting manually may be required.${NC}"
            echo -e "${YELLOW}Manual start: $INSTALL_DIR/claracore --config $CONFIG_DIR/config.yaml${NC}"
        fi
    fi
}

# Setup macOS LaunchAgent/Daemon
setup_macos_service() {
    echo -e "${BLUE}Setting up macOS Launch Agent...${NC}"

    if [[ "$SYSTEM_INSTALL" == true ]]; then
        PLIST_FILE="$LAUNCHD_DIR/com.claracore.server.plist"
        LABEL="com.claracore.server"
    else
        PLIST_FILE="$LAUNCHD_DIR/com.claracore.server.plist"
        LABEL="com.claracore.server"
    fi

    cat > "$PLIST_FILE" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$LABEL</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/claracore</string>
        <string>--config</string>
        <string>$CONFIG_DIR/config.yaml</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$CONFIG_DIR</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$CONFIG_DIR/claracore.log</string>
    <key>StandardErrorPath</key>
    <string>$CONFIG_DIR/claracore.error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>$HOME</string>
        <key>USER</key>
        <string>$USER</string>
    </dict>
</dict>
</plist>
EOF

    # Check if service is already loaded and unload it first
    if launchctl list | grep -q "$LABEL" 2>/dev/null; then
        echo -e "${YELLOW}Service already loaded, unloading first...${NC}"
        launchctl unload "$PLIST_FILE" 2>/dev/null || true
    fi

    # Load the service
    echo -e "${BLUE}Loading ClaraCore service...${NC}"
    if launchctl load "$PLIST_FILE" 2>/dev/null; then
        echo -e "${GREEN}✓ Service loaded successfully${NC}"

        # Wait a moment for service to initialize
        sleep 2

        # Verify service is running
        if launchctl list | grep -q "$LABEL" 2>/dev/null; then
            echo -e "${GREEN}✓ Service is running (auto-start on login enabled)${NC}"
            return 0
        else
            echo -e "${YELLOW}⚠ Service loaded but not found in service list${NC}"
            return 1
        fi
    else
        echo -e "${YELLOW}⚠ Failed to load Launch Agent${NC}"
        echo -e "${YELLOW}  This may be due to permissions or an existing service${NC}"
        echo
        echo -e "${YELLOW}  Try manually with:${NC}"
        if [[ "$SYSTEM_INSTALL" == true ]]; then
            echo -e "    ${BLUE}sudo launchctl load $PLIST_FILE${NC}"
        else
            echo -e "    ${BLUE}launchctl load $PLIST_FILE${NC}"
        fi
        echo -e "${YELLOW}  Or start manually:${NC}"
        echo -e "    ${BLUE}claracore --config $CONFIG_DIR/config.yaml${NC}"
        return 1
    fi
}

# Check if service is healthy and responding
check_service_health() {
    echo -e "${BLUE}Checking service health...${NC}"

    # Wait for service to fully initialize
    local max_attempts=15
    local attempt=1
    local port=5800

    while [ $attempt -le $max_attempts ]; do
        # Try to connect to the service
        if command -v curl >/dev/null 2>&1; then
            if curl -s -f "http://localhost:$port/" >/dev/null 2>&1; then
                echo -e "${GREEN}✓ ClaraCore is running and accessible${NC}"
                echo -e "${GREEN}✓ Web interface available at: ${BLUE}http://localhost:$port/ui/${NC}"
                return 0
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q -O /dev/null "http://localhost:$port/" 2>/dev/null; then
                echo -e "${GREEN}✓ ClaraCore is running and accessible${NC}"
                echo -e "${GREEN}✓ Web interface available at: ${BLUE}http://localhost:$port/ui/${NC}"
                return 0
            fi
        else
            # No curl or wget, try netcat or simple bash tcp check
            if (echo >/dev/tcp/localhost/$port) >/dev/null 2>&1; then
                echo -e "${GREEN}✓ Service is listening on port $port${NC}"
                echo -e "${GREEN}✓ Web interface should be at: ${BLUE}http://localhost:$port/ui/${NC}"
                return 0
            fi
        fi

        # Show progress
        if [ $attempt -eq 1 ]; then
            echo -ne "${YELLOW}  Waiting for service to start"
        else
            echo -ne "."
        fi

        sleep 1
        attempt=$((attempt + 1))
    done

    echo
    echo -e "${YELLOW}⚠ Could not verify service is responding${NC}"
    echo -e "${YELLOW}  The service may still be starting or there may be an issue${NC}"
    echo -e "${YELLOW}  Check the service status and logs${NC}"
    return 1
}

# Main installation flow
main() {
    echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║        ClaraCore Installer           ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"
    echo
    
    detect_platform
    check_permissions
    get_latest_release
    download_binary
    create_config

    # Setup autostart service
    SERVICE_STARTED=false
    if [[ "$PLATFORM" == "linux" ]]; then
        setup_linux_service
        # Check if systemd is available and service was likely started
        if command -v systemctl >/dev/null 2>&1 && systemctl is-system-running >/dev/null 2>&1; then
            SERVICE_STARTED=true
        fi
    elif [[ "$PLATFORM" == "darwin" ]]; then
        # Check if macOS service setup succeeded
        if setup_macos_service; then
            SERVICE_STARTED=true
        fi
    fi

    # Perform health check if service was started
    if [[ "$SERVICE_STARTED" == true ]]; then
        echo
        check_service_health
    fi

    echo
    echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║     Installation Completed!         ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
    echo
    
    # Check if claracore is now accessible
    if command -v claracore >/dev/null 2>&1; then
        echo -e "${GREEN}✓ claracore command is ready to use!${NC}"
    else
        echo -e "${YELLOW}⚠ To use 'claracore' command, restart your terminal or run:${NC}"
        echo -e "   ${BLUE}source ~/.bashrc${NC}"
        echo -e "   ${BLUE}# or${NC}"
        echo -e "   ${BLUE}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
        echo
    fi
    
    if [[ "$SERVICE_STARTED" == true ]]; then
        echo -e "${GREEN}✓ ClaraCore is now running as a service and will auto-start on boot!${NC}"
        echo
        echo -e "${YELLOW}Quick Start:${NC}"
        echo -e "1. ${GREEN}Open your browser and visit: ${BLUE}http://localhost:5800/ui/${NC}"
        echo
        echo -e "2. Configure your models via the web interface:"
        echo -e "   • Click 'Setup' to configure your models folder"
        echo -e "   • Or use the auto-discovery wizard"
        echo
    else
        echo -e "${YELLOW}Next steps:${NC}"
        echo -e "1. Start ClaraCore manually:"
        echo -e "   ${BLUE}claracore --config $CONFIG_DIR/config.yaml${NC}"
        echo
        echo -e "2. Then visit the web interface:"
        echo -e "   ${BLUE}http://localhost:5800/ui/setup${NC}"
        echo
    fi

    echo -e "${YELLOW}Service Management:${NC}"
    if [[ "$PLATFORM" == "linux" ]]; then
        if command -v systemctl >/dev/null 2>&1 && systemctl is-system-running >/dev/null 2>&1; then
            if [[ "$SYSTEM_INSTALL" == true ]]; then
                echo -e "   Status:  ${BLUE}sudo systemctl status $SERVICE_NAME${NC}"
                echo -e "   Stop:    ${BLUE}sudo systemctl stop $SERVICE_NAME${NC}"
                echo -e "   Restart: ${BLUE}sudo systemctl restart $SERVICE_NAME${NC}"
                echo -e "   Logs:    ${BLUE}sudo journalctl -u $SERVICE_NAME -f${NC}"
            else
                echo -e "   Status:  ${BLUE}systemctl --user status $SERVICE_NAME${NC}"
                echo -e "   Stop:    ${BLUE}systemctl --user stop $SERVICE_NAME${NC}"
                echo -e "   Restart: ${BLUE}systemctl --user restart $SERVICE_NAME${NC}"
                echo -e "   Logs:    ${BLUE}journalctl --user -u $SERVICE_NAME -f${NC}"
            fi
        else
            echo -e "   ${YELLOW}Systemd not available - manual start required:${NC}"
            echo -e "   Start:   ${BLUE}claracore --config $CONFIG_DIR/config.yaml${NC}"
        fi
    elif [[ "$PLATFORM" == "darwin" ]]; then
        LABEL="com.claracore.server"
        echo -e "   Status:  ${BLUE}launchctl list | grep claracore${NC}"
        echo -e "   Stop:    ${BLUE}launchctl stop $LABEL${NC}"
        echo -e "   Restart: ${BLUE}launchctl kickstart -k $LABEL${NC}"
        echo -e "   Unload:  ${BLUE}launchctl unload ~/Library/LaunchAgents/$LABEL.plist${NC}"
        echo -e "   Logs:    ${BLUE}tail -f $CONFIG_DIR/claracore.log${NC}"
        echo -e "   Errors:  ${BLUE}tail -f $CONFIG_DIR/claracore.error.log${NC}"
    fi
    echo
    echo -e "${YELLOW}Configuration Files:${NC}"
    echo -e "   Config:   ${BLUE}$CONFIG_DIR/config.yaml${NC}"
    echo -e "   Settings: ${BLUE}$CONFIG_DIR/settings.json${NC}"
    echo
    echo -e "${GREEN}Documentation: https://github.com/$REPO/tree/main/docs${NC}"
    echo -e "${GREEN}Support: https://github.com/$REPO/issues${NC}"
}

# Run main installation
main "$@"

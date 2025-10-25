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
            echo -e "${GREEN}System service enabled. Start with: sudo systemctl start $SERVICE_NAME${NC}"
        else
            echo -e "${YELLOW}Failed to enable system service. You may need to run with sudo or start manually.${NC}"
            echo -e "${YELLOW}Manual start: sudo $INSTALL_DIR/claracore --config $CONFIG_DIR/config.yaml${NC}"
        fi
    else
        if systemctl --user daemon-reload 2>/dev/null && systemctl --user enable "$SERVICE_NAME" 2>/dev/null; then
            echo -e "${GREEN}User service enabled. Start with: systemctl --user start $SERVICE_NAME${NC}"
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

    if [[ "$SYSTEM_INSTALL" == true ]]; then
        launchctl load "$PLIST_FILE"
        echo -e "${GREEN}System daemon loaded. ClaraCore will start automatically.${NC}"
    else
        launchctl load "$PLIST_FILE"
        echo -e "${GREEN}User agent loaded. ClaraCore will start automatically when you log in.${NC}"
    fi
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
    if [[ "$PLATFORM" == "linux" ]]; then
        setup_linux_service
    elif [[ "$PLATFORM" == "darwin" ]]; then
        setup_macos_service
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
    
    echo -e "${YELLOW}Next steps:${NC}"
    echo -e "1. Configure your models folder:"
    echo -e "   ${BLUE}claracore --models-folder /path/to/your/models${NC}"
    echo
    echo -e "2. Or start with the web interface:"
    echo -e "   ${BLUE}claracore${NC}"
    echo -e "   Then visit: ${BLUE}http://localhost:5800/ui/setup${NC}"
    echo
    echo -e "3. Service management:"
    if [[ "$PLATFORM" == "linux" ]]; then
        if command -v systemctl >/dev/null 2>&1 && systemctl is-system-running >/dev/null 2>&1; then
            if [[ "$SYSTEM_INSTALL" == true ]]; then
                echo -e "   Start:   ${BLUE}sudo systemctl start $SERVICE_NAME${NC}"
                echo -e "   Stop:    ${BLUE}sudo systemctl stop $SERVICE_NAME${NC}"
                echo -e "   Status:  ${BLUE}sudo systemctl status $SERVICE_NAME${NC}"
            else
                echo -e "   Start:   ${BLUE}systemctl --user start $SERVICE_NAME${NC}"
                echo -e "   Stop:    ${BLUE}systemctl --user stop $SERVICE_NAME${NC}"
                echo -e "   Status:  ${BLUE}systemctl --user status $SERVICE_NAME${NC}"
            fi
        else
            echo -e "   ${YELLOW}Systemd not available - manual start required:${NC}"
            echo -e "   Start:   ${BLUE}claracore --config $CONFIG_DIR/config.yaml${NC}"
        fi
    elif [[ "$PLATFORM" == "darwin" ]]; then
        echo -e "   Start:   ${BLUE}launchctl start $LABEL${NC}"
        echo -e "   Stop:    ${BLUE}launchctl stop $LABEL${NC}"
        echo -e "   Unload:  ${BLUE}launchctl unload $PLIST_FILE${NC}"
    fi
    echo
    echo -e "4. Configuration files:"
    echo -e "   Config:    ${BLUE}$CONFIG_DIR/config.yaml${NC}"
    echo -e "   Settings:  ${BLUE}$CONFIG_DIR/settings.json${NC}"
    echo
    echo -e "${GREEN}Documentation: https://github.com/$REPO/tree/main/docs${NC}"
    echo -e "${GREEN}Support: https://github.com/$REPO/issues${NC}"
}

# Run main installation
main "$@"

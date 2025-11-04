#!/bin/bash
# ClaraCore Uninstallation Script
# Supports Linux and macOS with automatic service removal

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

status() { echo -e "${BLUE}>>> $*${NC}" >&2; }
error() { echo -e "${RED}ERROR: $*${NC}"; exit 1; }
warning() { echo -e "${YELLOW}WARNING: $*${NC}"; }
success() { echo -e "${GREEN}✓ $*${NC}"; }

available() { command -v $1 >/dev/null 2>&1; }

# Detect OS
OS="$(uname)"
if [[ "$OS" == "Linux" ]]; then
    PLATFORM="linux"
    SERVICE_MANAGER="systemd"
    SERVICE_NAME="claracore"
elif [[ "$OS" == "Darwin" ]]; then
    PLATFORM="darwin"
    SERVICE_MANAGER="launchd"
    SERVICE_NAME="com.claracore.server"
else
    error "Unsupported operating system: $OS"
fi

# Check if running with sudo or as root
SUDO=""
if [[ $EUID -ne 0 ]]; then
    if available sudo; then
        SUDO="sudo"
        warning "Some operations may require sudo permissions"
    fi
fi

echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     ClaraCore Uninstaller           ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"
echo

# Stop and remove service
if [[ "$PLATFORM" == "linux" ]]; then
    if available systemctl; then
        status "Stopping ClaraCore systemd service..."
        
        # Check user service first
        if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            systemctl --user stop "$SERVICE_NAME" 2>/dev/null || true
            systemctl --user disable "$SERVICE_NAME" 2>/dev/null || true
            rm -f "$HOME/.config/systemd/user/$SERVICE_NAME.service" 2>/dev/null || true
            systemctl --user daemon-reload 2>/dev/null || true
            success "User service removed"
        fi
        
        # Check system service
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            $SUDO systemctl stop "$SERVICE_NAME" 2>/dev/null || true
            $SUDO systemctl disable "$SERVICE_NAME" 2>/dev/null || true
            $SUDO rm -f "/etc/systemd/system/$SERVICE_NAME.service" 2>/dev/null || true
            $SUDO systemctl daemon-reload 2>/dev/null || true
            success "System service removed"
        fi
    fi
elif [[ "$PLATFORM" == "darwin" ]]; then
    status "Stopping ClaraCore launchd service..."
    
    # Check user LaunchAgent
    USER_PLIST="$HOME/Library/LaunchAgents/$SERVICE_NAME.plist"
    if [[ -f "$USER_PLIST" ]]; then
        launchctl unload "$USER_PLIST" 2>/dev/null || true
        rm -f "$USER_PLIST" 2>/dev/null || true
        success "User LaunchAgent removed"
    fi
    
    # Check system LaunchDaemon
    SYSTEM_PLIST="/Library/LaunchDaemons/$SERVICE_NAME.plist"
    if [[ -f "$SYSTEM_PLIST" ]]; then
        $SUDO launchctl unload "$SYSTEM_PLIST" 2>/dev/null || true
        $SUDO rm -f "$SYSTEM_PLIST" 2>/dev/null || true
        success "System LaunchDaemon removed"
    fi
fi

# Remove binary
status "Removing ClaraCore binary..."

# Check common binary locations
BINARY_REMOVED=false

if [[ -f "/usr/local/bin/claracore" ]]; then
    $SUDO rm -f "/usr/local/bin/claracore"
    BINARY_REMOVED=true
fi

if [[ -f "$HOME/.local/bin/claracore" ]]; then
    rm -f "$HOME/.local/bin/claracore"
    BINARY_REMOVED=true
fi

if $BINARY_REMOVED; then
    success "Binary removed"
else
    warning "Binary not found in standard locations"
fi

# Ask about config files
CONFIG_DIR="$HOME/.config/claracore"
if [[ -d "$CONFIG_DIR" ]]; then
    echo
    echo -e "${YELLOW}Configuration directory found: $CONFIG_DIR${NC}"
    echo -e "${YELLOW}This contains your config.yaml, settings.json, and logs${NC}"
    
    read -p "$(echo -e ${YELLOW}Delete configuration files? [y/N]:${NC} )" -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$CONFIG_DIR"
        success "Configuration files removed"
    else
        echo -e "${BLUE}Configuration files preserved at: $CONFIG_DIR${NC}"
    fi
fi

echo
echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   Uninstallation Completed!         ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
echo
echo -e "${BLUE}ClaraCore has been removed from your system.${NC}"
echo

# Check if any processes are still running
if pgrep -x "claracore" > /dev/null; then
    warning "ClaraCore processes are still running"
    echo -e "${YELLOW}You may want to manually stop them:${NC}"
    echo -e "  ${BLUE}pkill claracore${NC}"
fi

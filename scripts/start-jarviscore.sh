#!/bin/bash
# Quick Start Script for JarvisCore
# Use this if JarvisCore didn't start automatically after installation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo ""
echo -e "${BLUE}JarvisCore Quick Start${NC}"
echo -e "${BLUE}=====================${NC}"
echo ""

# Detect OS
OS="$(uname)"
if [[ "$OS" == "Linux" ]]; then
    PLATFORM="linux"
    SERVICE_NAME="jarviscore"
elif [[ "$OS" == "Darwin" ]]; then
    PLATFORM="darwin"
    SERVICE_NAME="com.jarviscore.server"
else
    echo -e "${RED}❌ Unsupported operating system: $OS${NC}"
    exit 1
fi

# Find JarvisCore installation
BINARY_PATH=""
CONFIG_PATH="$HOME/.config/jarviscore/config.yaml"

# Check common locations
if [[ -f "/usr/local/bin/jarviscore" ]]; then
    BINARY_PATH="/usr/local/bin/jarviscore"
elif [[ -f "$HOME/.local/bin/jarviscore" ]]; then
    BINARY_PATH="$HOME/.local/bin/jarviscore"
fi

if [[ -z "$BINARY_PATH" ]]; then
    echo -e "${RED}❌ JarvisCore installation not found!${NC}"
    echo ""
    echo -e "${YELLOW}Please run the installer first:${NC}"
    echo -e "  ${CYAN}bash install.sh${NC}"
    echo ""
    exit 1
fi

echo -e "${GREEN}Found JarvisCore at: $BINARY_PATH${NC}"
echo ""

# Check and fix config files
echo -e "${BLUE}Checking configuration files...${NC}"

# Fix models: [] to models: {} in config.yaml
if [[ -f "$CONFIG_PATH" ]]; then
    if grep -q "^models: \[\]" "$CONFIG_PATH"; then
        echo -e "${YELLOW}Fixing config.yaml (models should be {} not [])...${NC}"
        sed -i.bak 's/^models: \[\]/models: {}/' "$CONFIG_PATH"
        echo -e "${GREEN}✓ Fixed config.yaml${NC}"
    fi
fi

# Create model_folders.json if missing
MODEL_FOLDERS_PATH="$(dirname "$CONFIG_PATH")/model_folders.json"
if [[ ! -f "$MODEL_FOLDERS_PATH" ]]; then
    echo -e "${YELLOW}Creating missing model_folders.json...${NC}"
    cat > "$MODEL_FOLDERS_PATH" << 'EOF'
{
  "folders": []
}
EOF
    echo -e "${GREEN}✓ Created model_folders.json${NC}"
fi

echo ""

# Check if already running
if command -v curl >/dev/null 2>&1; then
    if curl -s -f "http://localhost:5800/" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ JarvisCore is already running!${NC}"
        echo ""
        echo -e "${BLUE}Access the web interface at:${NC}"
        echo -e "  ${CYAN}http://localhost:5800/ui/${NC}"
        echo ""
        exit 0
    fi
fi

echo -e "${BLUE}Starting JarvisCore...${NC}"
echo ""

# Try starting via service
SERVICE_STARTED=false

if [[ "$PLATFORM" == "linux" ]]; then
    if command -v systemctl >/dev/null 2>&1; then
        # Try user service first
        if systemctl --user list-unit-files | grep -q "$SERVICE_NAME"; then
            echo -e "${BLUE}Starting user service...${NC}"
            if systemctl --user start "$SERVICE_NAME" 2>/dev/null; then
                echo -e "${GREEN}✓ Started user service${NC}"
                SERVICE_STARTED=true
            fi
        fi
        
        # Try system service
        if [[ "$SERVICE_STARTED" == false ]] && systemctl list-unit-files | grep -q "$SERVICE_NAME"; then
            echo -e "${BLUE}Starting system service...${NC}"
            if sudo systemctl start "$SERVICE_NAME" 2>/dev/null; then
                echo -e "${GREEN}✓ Started system service${NC}"
                SERVICE_STARTED=true
            fi
        fi
    fi
elif [[ "$PLATFORM" == "darwin" ]]; then
    # Check user LaunchAgent
    USER_PLIST="$HOME/Library/LaunchAgents/$SERVICE_NAME.plist"
    if [[ -f "$USER_PLIST" ]]; then
        echo -e "${BLUE}Starting LaunchAgent...${NC}"
        if launchctl load "$USER_PLIST" 2>/dev/null; then
            echo -e "${GREEN}✓ Started LaunchAgent${NC}"
            SERVICE_STARTED=true
        elif launchctl list | grep -q "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ LaunchAgent already loaded${NC}"
            SERVICE_STARTED=true
        fi
    fi
fi

# If no service, start manually
if [[ "$SERVICE_STARTED" == false ]]; then
    echo -e "${YELLOW}Starting JarvisCore manually...${NC}"
    echo ""
    
    # Check if binary is executable
    if [[ ! -x "$BINARY_PATH" ]]; then
        echo -e "${YELLOW}Making binary executable...${NC}"
        chmod +x "$BINARY_PATH"
    fi
    
    # Start in background
    nohup "$BINARY_PATH" --config "$CONFIG_PATH" > "$HOME/.config/jarviscore/logs/jarviscore.log" 2>&1 &
    PROCESS_PID=$!
    
    echo -e "${GREEN}✓ Started JarvisCore process (PID: $PROCESS_PID)${NC}"
    echo -e "${YELLOW}  Note: This is a manual start. Use the installer to set up auto-start.${NC}"
fi

# Wait for service to start
echo ""
echo -e "${BLUE}Waiting for JarvisCore to initialize...${NC}"
sleep 3

# Check if accessible
MAX_ATTEMPTS=10
ATTEMPT=0
RUNNING=false

echo -n "  Checking"

while [[ $ATTEMPT -lt $MAX_ATTEMPTS ]]; do
    if command -v curl >/dev/null 2>&1; then
        if curl -s -f "http://localhost:5800/" >/dev/null 2>&1; then
            RUNNING=true
            break
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -q -O /dev/null "http://localhost:5800/" 2>/dev/null; then
            RUNNING=true
            break
        fi
    fi
    
    echo -n "."
    sleep 1
    ATTEMPT=$((ATTEMPT + 1))
done

echo ""
echo ""

if [[ "$RUNNING" == true ]]; then
    echo -e "${GREEN}✅ JarvisCore is running and accessible!${NC}"
    echo ""
    echo -e "${BLUE}┌─────────────────────────────────────────┐${NC}"
    echo -e "${BLUE}│  Open your browser and visit:          │${NC}"
    echo -e "${BLUE}│                                         │${NC}"
    echo -e "${CYAN}│  http://localhost:5800/ui/              │${NC}"
    echo -e "${BLUE}│                                         │${NC}"
    echo -e "${BLUE}└─────────────────────────────────────────┘${NC}"
    echo ""
else
    echo -e "${YELLOW}⚠ JarvisCore may still be starting up...${NC}"
    echo ""
    echo -e "${CYAN}Try accessing the web interface in a moment:${NC}"
    echo -e "  ${CYAN}http://localhost:5800/ui/${NC}"
    echo ""
    echo -e "${CYAN}To check status:${NC}"
    if [[ "$PLATFORM" == "linux" ]]; then
        echo -e "  ${CYAN}systemctl --user status jarviscore${NC}"
        echo -e "  ${CYAN}# or: sudo systemctl status jarviscore${NC}"
    else
        echo -e "  ${CYAN}launchctl list | grep jarviscore${NC}"
    fi
    echo ""
    echo -e "${CYAN}To view logs:${NC}"
    echo -e "  ${CYAN}tail -f $HOME/.config/jarviscore/logs/jarviscore.error.log${NC}"
    echo ""
fi


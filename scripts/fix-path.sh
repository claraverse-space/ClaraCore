#!/bin/bash
# ClaraCore PATH Fix Script
# Run this if 'claracore' command is not found after installation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}ClaraCore PATH Fix${NC}"
echo "================================"

# Check if claracore binary exists
CLARACORE_PATH="$HOME/.local/bin/claracore"
if [[ ! -f "$CLARACORE_PATH" ]]; then
    echo -e "${RED}Error: ClaraCore binary not found at $CLARACORE_PATH${NC}"
    echo "Please run the installer first:"
    echo "curl -fsSL https://raw.githubusercontent.com/claraverse-space/ClaraCore/main/scripts/install.sh | bash"
    exit 1
fi

echo -e "${GREEN}✓ Found ClaraCore binary at $CLARACORE_PATH${NC}"

# Check if already in PATH
if command -v claracore >/dev/null 2>&1; then
    echo -e "${GREEN}✓ claracore is already accessible in PATH${NC}"
    echo "Try running: claracore --version"
    exit 0
fi

echo -e "${YELLOW}⚠ claracore not in PATH, fixing...${NC}"

# Add to PATH for current session
export PATH="$HOME/.local/bin:$PATH"

# Add to shell configuration files
CONFIG_UPDATED=false

# Bash
if [[ -f "$HOME/.bashrc" ]] && ! grep -q '$HOME/.local/bin' "$HOME/.bashrc"; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
    echo -e "${GREEN}✓ Added to ~/.bashrc${NC}"
    CONFIG_UPDATED=true
fi

# Zsh
if [[ -f "$HOME/.zshrc" ]] && ! grep -q '$HOME/.local/bin' "$HOME/.zshrc"; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc"
    echo -e "${GREEN}✓ Added to ~/.zshrc${NC}"
    CONFIG_UPDATED=true
fi

# Profile
if [[ -f "$HOME/.profile" ]] && ! grep -q '$HOME/.local/bin' "$HOME/.profile"; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.profile"
    echo -e "${GREEN}✓ Added to ~/.profile${NC}"
    CONFIG_UPDATED=true
fi

# Fish shell
if [[ -d "$HOME/.config/fish" ]]; then
    FISH_CONFIG="$HOME/.config/fish/config.fish"
    if [[ ! -f "$FISH_CONFIG" ]] || ! grep -q '$HOME/.local/bin' "$FISH_CONFIG"; then
        mkdir -p "$HOME/.config/fish"
        echo 'set -gx PATH $HOME/.local/bin $PATH' >> "$FISH_CONFIG"
        echo -e "${GREEN}✓ Added to Fish config${NC}"
        CONFIG_UPDATED=true
    fi
fi

if [[ "$CONFIG_UPDATED" == false ]]; then
    echo -e "${YELLOW}No shell configuration files updated (PATH may already be configured)${NC}"
fi

# Test again
if command -v claracore >/dev/null 2>&1; then
    echo -e "${GREEN}✓ SUCCESS: claracore is now accessible!${NC}"
    echo
    echo "Test it with:"
    echo -e "  ${BLUE}claracore --version${NC}"
else
    echo -e "${YELLOW}⚠ claracore still not accessible in current session${NC}"
    echo
    echo "Manual solutions:"
    echo -e "1. Restart your terminal"
    echo -e "2. Or run: ${BLUE}source ~/.bashrc${NC}"
    echo -e "3. Or run: ${BLUE}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
    echo
    echo "Then test with:"
    echo -e "  ${BLUE}claracore --version${NC}"
fi

echo
echo -e "${GREEN}PATH fix completed!${NC}"
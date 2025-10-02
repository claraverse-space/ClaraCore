#!/bin/bash

# ClaraCore Release Helper for Linux/macOS

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo ""
    echo -e "${BLUE}==========================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}==========================================${NC}"
    echo ""
}

print_error() {
    echo -e "${RED}Error: $1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

print_header "ClaraCore Release Manager"

# Check Python
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is not installed or not in PATH"
    echo "Please install Python 3.7+ from your package manager or https://python.org"
    exit 1
fi

# Check pip
if ! command -v pip3 &> /dev/null; then
    print_error "pip3 is not available"
    exit 1
fi

# Install dependencies if needed
echo "Checking Python dependencies..."
if ! python3 -c "import requests, github" 2>/dev/null; then
    echo "Installing Python dependencies..."
    pip3 install -r requirements-release.txt
    if [ $? -ne 0 ]; then
        print_error "Failed to install dependencies"
        exit 1
    fi
fi

print_success "Dependencies OK!"
echo ""

# Get version from user
read -p "Enter release version (e.g., v0.1.0): " VERSION
if [ -z "$VERSION" ]; then
    print_error "Version cannot be empty"
    exit 1
fi

# Get GitHub token
read -p "Use token file? (y/N): " TOKEN_CHOICE
if [[ "$TOKEN_CHOICE" =~ ^[Yy]$ ]]; then
    read -p "Enter token file path (.github_token): " TOKEN_FILE
    TOKEN_FILE=${TOKEN_FILE:-.github_token}
    
    if [ ! -f "$TOKEN_FILE" ]; then
        print_error "Token file not found: $TOKEN_FILE"
        echo ""
        echo "Create a GitHub Personal Access Token with 'repo' scope at:"
        echo "https://github.com/settings/tokens"
        echo ""
        echo "Save it to $TOKEN_FILE file"
        exit 1
    fi
    
    TOKEN_ARG="--token-file $TOKEN_FILE"
else
    read -s -p "Enter GitHub token (will be hidden): " GITHUB_TOKEN
    echo ""
    if [ -z "$GITHUB_TOKEN" ]; then
        print_error "GitHub token cannot be empty"
        exit 1
    fi
    
    TOKEN_ARG="--token $GITHUB_TOKEN"
fi

# Ask about draft
read -p "Create as draft? (y/N): " DRAFT_CHOICE
if [[ "$DRAFT_CHOICE" =~ ^[Yy]$ ]]; then
    DRAFT_ARG="--draft"
else
    DRAFT_ARG=""
fi

print_header "Creating Release $VERSION"

# Run the release script
python3 release.py --version "$VERSION" $TOKEN_ARG $DRAFT_ARG

if [ $? -eq 0 ]; then
    echo ""
    print_header "Release Created Successfully!"
    echo ""
    echo "Visit: https://github.com/badboysm890/ClaraCore/releases"
else
    echo ""
    print_header "Release Failed!"
    exit 1
fi
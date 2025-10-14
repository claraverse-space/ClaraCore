#!/bin/bash
# Build script for ClaraCore Universal Container

set -e

echo "🏗️  Building ClaraCore Universal Container..."
echo "This image will work on CUDA, ROCm, Vulkan, and CPU!"
echo ""

# Check if we're in the docker directory
if [ ! -f "Dockerfile" ]; then
    echo "❌ Error: Must run from the docker/ directory"
    echo "   cd docker && ./build-universal.sh"
    exit 1
fi

# Check if the Linux binary exists
if [ ! -f "../dist/claracore-linux-amd64" ]; then
    echo "❌ Error: Linux binary not found at ../dist/claracore-linux-amd64"
    echo ""
    echo "Please build the Linux binary first:"
    echo "  cd .. && make build-linux"
    echo ""
    exit 1
fi

echo "✅ Found Linux binary"
echo ""

# Build the Docker image
echo "🐳 Building Docker image..."
docker build \
    -f Dockerfile \
    -t claracore:universal \
    -t claracore:latest \
    ..

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Build complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📋 Quick Start:"
echo ""
echo "  # Auto-detect hardware (recommended)"
echo "  docker compose up"
echo ""
echo "  # Force CPU mode"
echo "  docker compose -f docker-compose.cpu-only.yml up"
echo ""
echo "  # Force CUDA (NVIDIA)"
echo "  docker compose -f docker-compose.cuda-explicit.yml up"
echo ""
echo "  # Force ROCm (AMD)"
echo "  docker compose -f docker-compose.rocm-explicit.yml up"
echo ""
echo "  # Force Vulkan (Universal GPU)"
echo "  docker compose -f docker-compose.vulkan-explicit.yml up"
echo ""
echo "📖 See DEPLOYMENT.md for complete documentation"
echo ""

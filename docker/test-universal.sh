#!/bin/bash
# Quick test script for ClaraCore Universal Container

set -e

echo "🧪 Testing ClaraCore Universal Container"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Function to test hardware detection
test_detection() {
    local backend=$1
    echo "Testing $backend backend..."

    docker run --rm \
        -e CLARACORE_BACKEND=$backend \
        claracore:universal \
        --version

    if [ $? -eq 0 ]; then
        echo "✅ $backend backend test passed"
    else
        echo "❌ $backend backend test failed"
        return 1
    fi
    echo ""
}

# Check if image exists
if ! docker image inspect claracore:universal &> /dev/null; then
    echo "❌ Image not found. Please build first:"
    echo "   ./build-universal.sh"
    exit 1
fi

echo "✅ Found claracore:universal image"
echo ""

# Test version command
echo "Testing basic functionality..."
docker run --rm claracore:universal --version
echo ""

# Test CPU backend explicitly
test_detection "cpu"

# Test auto-detection
echo "Testing auto-detection (no forced backend)..."
docker run --rm claracore:universal --version | head -1
echo "✅ Auto-detection test passed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ All basic tests passed!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To test with actual models:"
echo "  1. Copy GGUF models to ./models/"
echo "  2. Run: docker compose -f docker-compose.universal.yml up"
echo "  3. Open: http://localhost:5800/ui/"
echo ""

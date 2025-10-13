#!/bin/bash
# Test script for ClaraCore Docker containers

set -e

echo "🧪 Testing ClaraCore Docker containers..."

VARIANT=${1:-cuda}
TEST_PORT=5801

if [ "$VARIANT" != "cuda" ] && [ "$VARIANT" != "rocm" ]; then
    echo "❌ Invalid variant. Use 'cuda' or 'rocm'"
    exit 1
fi

echo "Testing variant: $VARIANT"

# Start container
echo "🚀 Starting container..."
if [ "$VARIANT" = "cuda" ]; then
    docker run -d \
        --name claracore-test \
        --gpus all \
        -p $TEST_PORT:5800 \
        claracore:$VARIANT
else
    docker run -d \
        --name claracore-test \
        --device=/dev/kfd \
        --device=/dev/dri \
        --group-add video \
        --group-add render \
        -p $TEST_PORT:5800 \
        claracore:$VARIANT
fi

# Wait for container to be ready
echo "⏳ Waiting for container to start..."
sleep 5

# Check if container is running
if ! docker ps | grep -q claracore-test; then
    echo "❌ Container failed to start!"
    docker logs claracore-test
    docker rm -f claracore-test
    exit 1
fi

# Test health endpoint
echo "🔍 Testing health endpoint..."
for i in {1..30}; do
    if curl -f -s http://localhost:$TEST_PORT/api/health > /dev/null 2>&1; then
        echo "✅ Health check passed!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Health check failed after 30 attempts"
        docker logs claracore-test
        docker rm -f claracore-test
        exit 1
    fi
    echo "Attempt $i/30..."
    sleep 2
done

# Test UI endpoint
echo "🔍 Testing UI endpoint..."
if curl -f -s http://localhost:$TEST_PORT/ui/ > /dev/null 2>&1; then
    echo "✅ UI endpoint accessible!"
else
    echo "❌ UI endpoint not accessible"
    docker logs claracore-test
    docker rm -f claracore-test
    exit 1
fi

# Check GPU access
echo "🔍 Checking GPU access..."
if [ "$VARIANT" = "cuda" ]; then
    if docker exec claracore-test nvidia-smi > /dev/null 2>&1; then
        echo "✅ CUDA GPU detected!"
    else
        echo "⚠️  CUDA GPU not detected (may be expected in CI)"
    fi
else
    if docker exec claracore-test rocm-smi > /dev/null 2>&1; then
        echo "✅ ROCm GPU detected!"
    else
        echo "⚠️  ROCm GPU not detected (may be expected in CI)"
    fi
fi

# Check container size
echo "📊 Container size:"
docker images claracore:$VARIANT --format "{{.Repository}}:{{.Tag}} - {{.Size}}"

# Cleanup
echo "🧹 Cleaning up..."
docker rm -f claracore-test

echo ""
echo "🎉 All tests passed for $VARIANT variant!"
echo ""

#!/bin/bash
# Build script for ClaraCore Docker containers

set -e

echo "🐳 Building ClaraCore Docker containers..."

# Check if dist/claracore-linux-amd64 exists
if [ ! -f "dist/claracore-linux-amd64" ]; then
    echo "❌ Error: dist/claracore-linux-amd64 not found!"
    echo "Please build the Linux binary first with: python build.py"
    exit 1
fi

# Parse arguments
BUILD_CUDA=false
BUILD_ROCM=false
PUSH=false
TAG="latest"

while [[ $# -gt 0 ]]; do
    case $1 in
        --cuda)
            BUILD_CUDA=true
            shift
            ;;
        --rocm)
            BUILD_ROCM=true
            shift
            ;;
        --all)
            BUILD_CUDA=true
            BUILD_ROCM=true
            shift
            ;;
        --push)
            PUSH=true
            shift
            ;;
        --tag)
            TAG="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--cuda] [--rocm] [--all] [--push] [--tag TAG]"
            exit 1
            ;;
    esac
done

# If no specific option, build both
if [ "$BUILD_CUDA" = false ] && [ "$BUILD_ROCM" = false ]; then
    BUILD_CUDA=true
    BUILD_ROCM=true
fi

# Build CUDA container
if [ "$BUILD_CUDA" = true ]; then
    echo "🔨 Building CUDA container..."
    docker build -f Dockerfile.cuda -t claracore:cuda-${TAG} -t claracore:cuda .
    
    # Check size
    SIZE=$(docker images claracore:cuda --format "{{.Size}}")
    echo "✅ CUDA container built successfully! Size: $SIZE"
    
    if [ "$PUSH" = true ]; then
        echo "📤 Pushing CUDA container..."
        docker push claracore:cuda-${TAG}
        docker push claracore:cuda
    fi
fi

# Build ROCm container
if [ "$BUILD_ROCM" = true ]; then
    echo "🔨 Building ROCm container..."
    docker build -f Dockerfile.rocm -t claracore:rocm-${TAG} -t claracore:rocm .
    
    # Check size
    SIZE=$(docker images claracore:rocm --format "{{.Size}}")
    echo "✅ ROCm container built successfully! Size: $SIZE"
    
    if [ "$PUSH" = true ]; then
        echo "📤 Pushing ROCm container..."
        docker push claracore:rocm-${TAG}
        docker push claracore:rocm
    fi
fi

echo ""
echo "🎉 Build complete!"
echo ""
echo "To run CUDA container:"
echo "  docker-compose -f docker-compose.cuda.yml up -d"
echo "  or"
echo "  docker run -d --gpus all -p 5800:5800 -v ./models:/models claracore:cuda"
echo ""
echo "To run ROCm container:"
echo "  docker-compose -f docker-compose.rocm.yml up -d"
echo "  or"
echo "  docker run -d --device=/dev/kfd --device=/dev/dri -p 5800:5800 -v ./models:/models claracore:rocm"
echo ""
echo "Web UI: http://localhost:5800/ui/"

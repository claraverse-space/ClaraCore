#!/bin/bash
set -e

# ClaraCore Universal Entrypoint - Ollama Style
# Auto-detects hardware and configures the optimal backend
# Works with CUDA, ROCm, Vulkan, and CPU

echo "🚀 ClaraCore Universal Container Starting..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Function to detect NVIDIA CUDA
detect_cuda() {
    if command -v nvidia-smi &> /dev/null; then
        if nvidia-smi &> /dev/null; then
            echo "✅ NVIDIA GPU detected"
            nvidia-smi --query-gpu=name,memory.total --format=csv,noheader 2>/dev/null || true
            return 0
        fi
    fi

    # Alternative check for CUDA libraries
    if [ -d "/usr/local/cuda" ] || [ -f "/usr/lib/x86_64-linux-gnu/libcuda.so" ]; then
        echo "✅ CUDA libraries detected"
        return 0
    fi

    return 1
}

# Function to detect AMD ROCm
detect_rocm() {
    if command -v rocm-smi &> /dev/null; then
        if rocm-smi &> /dev/null; then
            echo "✅ AMD ROCm GPU detected"
            rocm-smi --showproductname 2>/dev/null || true
            return 0
        fi
    fi

    # Check for ROCm runtime files
    if [ -d "/opt/rocm" ] || [ -f "/usr/lib/x86_64-linux-gnu/libamdhip64.so" ]; then
        echo "✅ ROCm runtime detected"
        return 0
    fi

    # Check for AMD GPU via lspci
    if command -v lspci &> /dev/null; then
        if lspci 2>/dev/null | grep -qi "AMD.*Radeon\|AMD.*Graphics"; then
            echo "✅ AMD GPU detected via lspci"
            return 0
        fi
    fi

    return 1
}

# Function to detect Vulkan support
detect_vulkan() {
    if command -v vulkaninfo &> /dev/null; then
        if vulkaninfo 2>/dev/null | grep -qi "deviceName\|GPU"; then
            echo "✅ Vulkan support detected"
            vulkaninfo 2>/dev/null | grep "deviceName" | head -1 || true
            return 0
        fi
    fi

    # Check for Vulkan libraries
    if [ -f "/usr/lib/x86_64-linux-gnu/libvulkan.so.1" ]; then
        echo "✅ Vulkan libraries available"
        return 0
    fi

    return 1
}

# Hardware detection logic (priority order: CUDA > ROCm > Vulkan > CPU)
DETECTED_BACKEND=""
BACKEND_FLAG=""

echo ""
echo "🔍 Detecting available hardware..."
echo ""

# Check for user-forced backend via environment variable
if [ -n "$CLARACORE_BACKEND" ]; then
    echo "🎯 Backend forced via CLARACORE_BACKEND: $CLARACORE_BACKEND"
    DETECTED_BACKEND="$CLARACORE_BACKEND"
    BACKEND_FLAG="--backend $CLARACORE_BACKEND"
elif detect_cuda; then
    # NVIDIA GPU detected - but llama.cpp Ubuntu binaries don't include CUDA
    # Vulkan requires proper driver mounting which isn't set up yet
    # For now, use CPU mode for maximum compatibility
    echo "⚠️  NVIDIA GPU detected, but Ubuntu llama.cpp binaries don't include CUDA"
    echo "ℹ️  Using CPU mode for now (Vulkan driver mounting not configured)"
    DETECTED_BACKEND="cpu"
    BACKEND_FLAG="--backend cpu"
elif detect_rocm; then
    # Similar for ROCm - use CPU for now
    echo "⚠️  AMD GPU detected, but using CPU mode for maximum compatibility"
    DETECTED_BACKEND="cpu"
    BACKEND_FLAG="--backend cpu"
elif detect_vulkan; then
    # Even if vulkan tools are available, the drivers might not be mounted
    echo "ℹ️  Vulkan detected but using CPU mode for maximum compatibility"
    DETECTED_BACKEND="cpu"
    BACKEND_FLAG="--backend cpu"
else
    echo "ℹ️  No GPU detected - using CPU mode"
    DETECTED_BACKEND="cpu"
    BACKEND_FLAG="--backend cpu"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎯 Selected Backend: $DETECTED_BACKEND"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Optional: Set environment variables for the backend
case "$DETECTED_BACKEND" in
    cuda)
        export CUDA_VISIBLE_DEVICES="${CUDA_VISIBLE_DEVICES:-0}"
        ;;
    rocm)
        export HIP_VISIBLE_DEVICES="${HIP_VISIBLE_DEVICES:-0}"
        export HSA_OVERRIDE_GFX_VERSION="${HSA_OVERRIDE_GFX_VERSION:-10.3.0}"
        ;;
    vulkan)
        # Vulkan generally works without special env vars
        ;;
    cpu)
        # CPU mode needs no special configuration
        ;;
esac

# Check if models folder is mounted
if [ ! -d "/models" ] || [ -z "$(ls -A /models 2>/dev/null)" ]; then
    echo "⚠️  WARNING: No models found in /models"
    echo "   Mount your models with: -v /path/to/models:/models"
    echo ""
fi

# Create config directory if it doesn't exist
mkdir -p /app/config

# Determine config file location (auto-setup saves to /app/config.yaml)
CONFIG_PATH="/app/config.yaml"

# If auto-setup is requested and models exist
if [ -d "/models" ] && [ -n "$(ls -A /models 2>/dev/null)" ] && [ ! -f "$CONFIG_PATH" ]; then
    echo "📦 Models detected but no config found - will auto-generate on startup"
    echo ""
fi

# Build the final command
# If user passed arguments, use them; otherwise use defaults
if [ $# -eq 0 ]; then
    # No arguments provided, check if we should auto-setup
    if [ -d "/models" ] && [ -n "$(ls -A /models 2>/dev/null)" ]; then
        echo "🔧 Starting with auto-setup mode..."
        exec /app/claracore \
            --listen ":5800" \
            --models-folder "/models" \
            --config "$CONFIG_PATH" \
            $BACKEND_FLAG
    else
        echo "🔧 Starting in manual mode (no models to auto-setup)..."
        exec /app/claracore \
            --listen ":5800" \
            --config "$CONFIG_PATH" \
            $BACKEND_FLAG
    fi
else
    # User provided custom arguments - append backend flag
    echo "🔧 Starting with custom arguments..."
    exec /app/claracore "$@" $BACKEND_FLAG
fi

# Universal ClaraCore Container - Ollama Style
# Auto-detects and adapts to: NVIDIA CUDA, AMD ROCm, Vulkan, or CPU
# Works on any hardware with zero configuration

FROM ubuntu:22.04 AS base

# Install runtime dependencies for ALL backends
# This keeps the image size reasonable while supporting everything
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    unzip \
    pciutils \
    lshw \
    # Vulkan support (AMD, Intel, some NVIDIA)
    libvulkan1 \
    vulkan-tools \
    # ROCm runtime essentials (minimal, only what's needed to run)
    && rm -rf /var/lib/apt/lists/*

# Create directories
WORKDIR /app
RUN mkdir -p /models /app/config /app/binaries /app/downloads /tmp/claracore

# Copy the ClaraCore binary
COPY dist/claracore-linux-amd64 /app/claracore
RUN chmod +x /app/claracore

# Copy the smart entrypoint script
COPY docker/entrypoint-universal.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create a wrapper script for llama-server to ensure proper working directory
RUN echo '#!/bin/bash' > /app/llama-wrapper.sh && \
    echo 'cd "$(dirname "$(readlink -f "$0")")/binaries/llama-server/build/bin"' >> /app/llama-wrapper.sh && \
    echo 'export LD_LIBRARY_PATH="$(pwd):$LD_LIBRARY_PATH"' >> /app/llama-wrapper.sh && \
    echo 'exec ./llama-server "$@"' >> /app/llama-wrapper.sh && \
    chmod +x /app/llama-wrapper.sh

# Expose the port
EXPOSE 5800

# Environment variables
ENV GIN_MODE=release \
    NVIDIA_VISIBLE_DEVICES=all \
    NVIDIA_DRIVER_CAPABILITIES=compute,utility \
    HSA_OVERRIDE_GFX_VERSION=10.3.0 \
    ROC_ENABLE_PRE_VEGA=1

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:5800/api/health || exit 1

# Use the smart entrypoint that detects hardware at runtime
# No CMD - let entrypoint handle everything automatically
ENTRYPOINT ["/app/entrypoint.sh"]

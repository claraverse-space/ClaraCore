# Simple ClaraCore Docker Container
# Uses Vulkan backend with pre-built binary
FROM ubuntu:22.04

# Install minimal dependencies for Vulkan support
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libvulkan1 \
    vulkan-tools \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy the pre-built Linux binary from dist folder
COPY dist/claracore-linux-amd64 /app/claracore
RUN chmod +x /app/claracore

# Create directories for models and config
RUN mkdir -p /models /app/binaries /app/downloads

# Expose port 5801
EXPOSE 5801

# Environment variables
ENV GIN_MODE=release

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:5801/api/health || exit 1

# Run ClaraCore with Vulkan backend
ENTRYPOINT ["/app/claracore"]
CMD ["--listen", ":5801", "--config", "/app/config.yaml"]

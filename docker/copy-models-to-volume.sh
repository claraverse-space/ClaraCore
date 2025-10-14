#!/bin/bash
# Copy models from host to Docker volume
# Usage: ./copy-models-to-volume.sh /path/to/models

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 /path/to/models"
    exit 1
fi

SOURCE_PATH="$1"

echo "📦 Copying models to Docker volume..."
echo ""

# Validate source path
if [ ! -d "$SOURCE_PATH" ]; then
    echo "❌ Error: Source path not found: $SOURCE_PATH"
    exit 1
fi

# Count GGUF files
GGUF_COUNT=$(find "$SOURCE_PATH" -name "*.gguf" | wc -l)
if [ "$GGUF_COUNT" -eq 0 ]; then
    echo "⚠️  Warning: No .gguf files found in $SOURCE_PATH"
    exit 1
fi

echo "✅ Found $GGUF_COUNT GGUF model(s)"
echo ""

# Create a temporary container to copy files
echo "🚀 Creating temporary container..."
docker run -d --name claracore-temp -v claracore_models:/models alpine sleep 60

echo "✅ Temporary container created"
echo ""

# Copy files
echo "📁 Copying files..."
find "$SOURCE_PATH" -name "*.gguf" -type f | while read -r file; do
    SIZE=$(du -h "$file" | cut -f1)
    echo "   Copying: $(basename "$file") ($SIZE)"
    docker cp "$file" claracore-temp:/models/
done

echo ""
echo "✅ All files copied successfully!"
echo ""

# List files in volume
echo "📋 Files in Docker volume:"
docker exec claracore-temp ls -lh /models

# Cleanup
echo ""
echo "🧹 Cleaning up..."
docker rm -f claracore-temp > /dev/null

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Models copied to Docker volume!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Start ClaraCore: docker compose up -d"
echo "  2. View logs: docker compose logs -f"
echo "  3. Access UI: http://localhost:5800/ui/"
echo ""

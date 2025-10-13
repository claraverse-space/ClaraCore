#!/bin/bash
set -e

# Check if models have been copied already
INIT_MARKER="/models/.initialized"

if [ ! -f "$INIT_MARKER" ]; then
    echo "ðŸ”„ First boot detected - copying models to volume..."
    echo "This is a ONE-TIME operation and may take a few minutes..."
    
    # Copy specific models/folders from the mounted source
    if [ -d "/source" ]; then
        # Create models directory if it doesn't exist
        mkdir -p /models
        
        # Copy folders
        if [ -d "/source/GLM-4.5-Air-IQ4_XS" ]; then
            echo "ðŸ“¦ Copying GLM-4.5-Air-IQ4_XS..."
            cp -r "/source/GLM-4.5-Air-IQ4_XS" /models/
        fi
        
        if [ -d "/source/Jan" ]; then
            echo "ðŸ“¦ Copying Jan folder..."
            cp -r "/source/Jan" /models/
        fi
        
        # Copy specific GGUF files
        if [ -f "/source/GLM-4.5-Air-UD-Q2_K_XL.gguf" ]; then
            echo "ðŸ“¦ Copying GLM-4.5-Air-UD-Q2_K_XL.gguf..."
            cp "/source/GLM-4.5-Air-UD-Q2_K_XL.gguf" /models/
        fi
        
        if [ -f "/source/ByteDance-Seed_Seed-OSS-36B-Instruct-Q4_K_M.gguf" ]; then
            echo "ðŸ“¦ Copying ByteDance-Seed model..."
            cp "/source/ByteDance-Seed_Seed-OSS-36B-Instruct-Q4_K_M.gguf" /models/
        fi
        
        # Create Qwen directory and copy Qwen models
        if [ -d "/source/Qwen" ]; then
            mkdir -p /models/Qwen
            
            if [ -f "/source/Qwen/Qwen_Qwen3-30B-A3B-Instruct-2507-Q5_K_M.gguf" ]; then
                echo "ðŸ“¦ Copying Qwen3-30B model..."
                cp "/source/Qwen/Qwen_Qwen3-30B-A3B-Instruct-2507-Q5_K_M.gguf" /models/Qwen/
            fi
            
            if [ -f "/source/Qwen/Qwen3-4B-Thinking-2507-Q8_0.gguf" ]; then
                echo "ðŸ“¦ Copying Qwen3-4B-Thinking model..."
                cp "/source/Qwen/Qwen3-4B-Thinking-2507-Q8_0.gguf" /models/Qwen/
            fi
        fi
        
        echo "âœ… Models copied successfully!"
        echo "Creating initialization marker..."
        touch "$INIT_MARKER"
        echo "$(date)" >> "$INIT_MARKER"
    else
        echo "âš ï¸  Warning: Source directory not mounted at /source"
        echo "Creating empty marker to avoid checking again..."
        touch "$INIT_MARKER"
    fi
else
    echo "âœ… Models already initialized - using existing volume"
    echo "ðŸ“Š Initialized on: $(cat $INIT_MARKER)"
fi

# Count models
MODEL_COUNT=$(find /models -name "*.gguf" -type f | wc -l)
echo "ðŸ“¦ Found $MODEL_COUNT GGUF files in /models"

# Now start ClaraCore with the arguments passed to the container
echo "ðŸš€ Starting ClaraCore..."
exec /app/claracore "$@"

# ClaraCore Port Management

ClaraCore now has intelligent port management that automatically handles port conflicts and assignments.

## Port Architecture

### ClaraCore Web Interface
- **Default Port**: 5800
- **Configurable**: Use `--listen` flag to change
- **Example**: `./claracore --listen :8096` to run on port 8096

### Model Servers (llama-server instances)
- **Default Start Port**: 8100
- **Auto-assignment**: Models get ports 8100, 8101, 8102, etc.
- **Configurable**: Set `startPort` in config.yaml
- **Group-specific**: Each group can have its own `startPort`

## Configuration Examples

### Basic Configuration
```yaml
# ClaraCore global settings
startPort: 8100  # Models start from port 8100

models:
  my-model:
    # ... model config
    # Will automatically get port 8100
```

### Group-specific Ports
```yaml
groups:
  large-models:
    startPort: 8200  # Large models start from 8200
    members:
      - model1
      - model2
  
  small-models:
    startPort: 8300  # Small models start from 8300
    members:
      - embedding1
      - embedding2
```

### Custom ClaraCore Port
```bash
# Run ClaraCore on port 9000 instead of 5800
./claracore --listen :9000

# Or bind to specific IP
./claracore --listen 192.168.1.100:5800
```

## Port Ranges

| Service | Default Port Range | Purpose |
|---------|-------------------|---------|
| ClaraCore Web UI | 5800 | Web interface, API endpoints |
| Model Servers | 8100+ | Individual llama-server instances |
| Large Models Group | 8200+ | Heavy models (swappable) |
| Small Models Group | 8300+ | Embeddings, light models |

## Auto-setup Defaults

When using `--models-folder`, ClaraCore automatically:

1. Sets `startPort: 8100` for models
2. Creates groups with appropriate port ranges
3. Keeps ClaraCore web interface on port 5800
4. Ensures no port conflicts

## Benefits

✅ **No Port Conflicts**: ClaraCore and models use separate port ranges  
✅ **Auto-reload Support**: Config changes trigger automatic server restarts  
✅ **Flexible Configuration**: Easy to customize ports for different environments  
✅ **Group Isolation**: Different model groups can use different port ranges  

## Migration from Old Versions

If you have an old config with `startPort: 5800`, update it to:

```yaml
# OLD (conflicts with web interface)
startPort: 5800

# NEW (no conflicts)
startPort: 8100
```

ClaraCore will automatically fix this during auto-setup, but manual configs should be updated.
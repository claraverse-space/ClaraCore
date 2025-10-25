# System Settings Persistence

## Overview

ClaraCore now persists user's manual hardware selections in `settings.json`, ensuring that hardware configurations specified during setup are preserved across "Force Reconfigure" operations.

## Problem Solved

Previously, when users manually configured their hardware settings (because automatic detection was incorrect), these settings would be lost when using "Force Reconfigure". The system would revert to automatic detection, which could be incorrect.

## Solution

### Settings Storage (`settings.json`)

User's hardware and performance preferences are saved in `settings.json`:

```json
{
  "backend": "cuda",
  "vramGB": 24.0,
  "ramGB": 64.0,
  "preferredContext": 32768,
  "throughputFirst": true,
  "enableJinja": true
}
```

### When Settings Are Saved

Settings are automatically saved when:

1. **Initial Setup**: When the user completes the onboarding wizard and manually selects hardware settings
2. **Settings Page**: When the user updates their system configuration through the settings interface (if available)

### When Settings Are Used

Saved settings take priority during:

1. **Force Reconfigure**: When clicking the "Force Reconfigure" button in the navbar, the system rescans all tracked folders and regenerates `config.yaml` using the saved hardware settings instead of re-detecting
2. **Self-Heal**: When the system automatically regenerates config due to errors, it uses saved settings
3. **CLI Auto-Setup**: When running with `--models-folder` flag, if `settings.json` exists, those settings are used

### Priority Order

The system follows this priority order for hardware settings:

1. **User's Saved Settings** (highest priority) - from `settings.json`
2. **Request Options** - explicitly passed in API calls
3. **Auto-Detection** (lowest priority) - automatic hardware detection

## Implementation Details

### Backend (`proxy/proxymanager_api.go`)

The `apiRegenerateConfigFromDatabase` function now:

```go
// Load saved system settings and merge with request options
savedSettings, err := pm.loadSystemSettings()
if err == nil && savedSettings != nil {
    // User's saved settings take priority
    if savedSettings.Backend != "" {
        req.Options.ForceBackend = savedSettings.Backend
    }
    if savedSettings.VRAMGB > 0 {
        req.Options.ForceVRAM = savedSettings.VRAMGB
    }
    // ... etc
}
```

### Frontend (`ui/src/pages/OnboardConfig.tsx`)

Before generating the initial configuration, settings are saved:

```typescript
// Save system settings to persistent storage first
const settingsResponse = await fetch('/api/settings/system', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    backend: systemConfig.backend,
    vramGB: systemConfig.vramGB,
    ramGB: systemConfig.ramGB,
    preferredContext: systemConfig.preferredContext,
    throughputFirst: systemConfig.throughputFirst,
    enableJinja: true,
  })
});
```

### Self-Heal (`claracore.go`)

The `selfHealReconfigure` function already loads and uses saved settings when automatically recovering from config errors.

## User Experience

### Setup Flow

1. User runs ClaraCore setup wizard
2. System auto-detects hardware (e.g., 12GB VRAM)
3. User notices detection is wrong and manually adjusts to 24GB VRAM
4. User completes setup → Settings are saved to `settings.json`
5. Config is generated with 24GB VRAM settings

### Force Reconfigure Flow

1. User adds new models to tracked folders
2. User clicks "Force Reconfigure" in navbar
3. System rescans all folders
4. System loads saved settings (24GB VRAM) from `settings.json`
5. Config is regenerated with 24GB VRAM settings (not auto-detected 12GB)

## API Endpoints

### Save System Settings

```bash
POST /api/settings/system
Content-Type: application/json

{
  "backend": "cuda",
  "vramGB": 24.0,
  "ramGB": 64.0,
  "preferredContext": 32768,
  "throughputFirst": true,
  "enableJinja": true
}
```

### Get System Settings

```bash
GET /api/settings/system
```

### Force Reconfigure with Saved Settings

```bash
POST /api/config/regenerate-from-db
Content-Type: application/json

{
  "options": {
    "enableJinja": true,
    "throughputFirst": true,
    "minContext": 16384,
    "preferredContext": 32768
  }
}
```

The backend will automatically merge saved settings with the provided options, giving priority to saved settings.

## Benefits

1. **User Preferences Persist**: Manual hardware corrections are remembered
2. **Consistent Performance**: Models are configured with the same hardware parameters across reconfigurations
3. **Less Manual Work**: Users don't need to re-enter hardware settings every time
4. **Intelligent Defaults**: If no saved settings exist, auto-detection is used as fallback

## Files Modified

- `proxy/proxymanager_api.go`: Load and merge saved settings in regenerate endpoint
- `ui/src/components/Header.tsx`: Updated Force Reconfigure to use saved settings
- `ui/src/pages/OnboardConfig.tsx`: Save settings during initial setup
- `claracore.go`: Self-heal function already uses saved settings

## Testing

1. Run initial setup and manually adjust hardware settings
2. Verify `settings.json` is created with correct values
3. Add new models to tracked folders
4. Click "Force Reconfigure" in navbar
5. Verify new config uses saved hardware settings (not auto-detected values)

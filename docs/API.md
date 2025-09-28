## ClaraCore HTTP API Reference

Base URL
- Local UI base: /ui/
- API base: /api
- OpenAI-compatible base: /v1

Conventions
- All JSON requests use Content-Type: application/json
- Responses use standard HTTP status codes; error responses include { "error": string }
- SSE endpoint /api/events streams server-sent events

Quick Start Flows
1) Add folders and generate config (multi-folder)
```bash
curl -X POST http://localhost:5800/api/config/scan-folder \
  -H 'Content-Type: application/json' \
  -d '{"folderPaths":["/path/to/models"],"recursive":true,"addToDatabase":true}'

curl -X POST http://localhost:5800/api/config/regenerate-from-db \
  -H 'Content-Type: application/json' \
  -d '{"options":{"enableJinja":true,"throughputFirst":true,"minContext":16384,"preferredContext":32768}}'
```

2) Download a model and let backend auto-reconfigure (folder is auto-added)
```bash
curl -X POST http://localhost:5800/api/models/download \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example/model.gguf","modelId":"model-1","filename":"model.gguf"}'
```

3) Persist system settings once (used on all regenerations)
```bash
curl -X POST http://localhost:5800/api/settings/system \
  -H 'Content-Type: application/json' \
  -d '{
    "gpuType":"apple",
    "backend":"metal",
    "vramGB":32,
    "ramGB":32,
    "preferredContext":32768,
    "throughputFirst":true,
    "enableJinja":true
  }'
```

Server Management
- POST /api/server/restart → Soft restart (reload config, restart process groups)
- POST /api/server/restart/hard → Hard restart (respawn process)

Events & Metrics
- GET /api/events → SSE stream of:
  - modelStatus, logs, metrics, downloadProgress, config reload notifications
- GET /api/metrics → current metrics JSON

System Detection & Settings
- GET /api/system/specs → summary CPU/GPU specs
- GET /api/system/detection → detailed detection (GPU brand, VRAM, recommendations)
- GET /api/settings/system → returns saved settings or { "settings": null }
- POST /api/settings/system → save settings
  - macOS Apple Silicon: unsupported backends (cuda/rocm/vulkan) are mapped to metal

Model Download Manager
- POST /api/models/download { url, modelId, filename, hfApiKey? }
- POST /api/models/download/cancel { downloadId }
- GET /api/models/downloads → map of downloads
- GET /api/models/downloads/:id → download info
- POST /api/models/downloads/:id/pause
- POST /api/models/downloads/:id/resume

Configuration Management
- GET /api/config → current config.yaml rendered JSON
- POST /api/config { yaml?: string, patch?: object } → update config (advanced)
- POST /api/config/model/:id → update parameters for a single model
- POST /api/config/scan-folder { folderPath? | folderPaths[], recursive, addToDatabase }
  - Scans folders for .gguf; returns { models:[], scanSummary:[] }
  - If addToDatabase=true, updates model_folders.json
- POST /api/config/add-model … → add a model (advanced)
- POST /api/config/append-model { filePath, options } → append one model to config
- DELETE /api/config/models/:id → remove a model from config
- GET /api/config/validate → validates YAML structure
- POST /api/config/validate-models → removes missing model files from config
- POST /api/config/cleanup-duplicates → removes duplicates (same file path)
- POST /api/config/regenerate-from-db { options } → regenerate entire config from tracked folders
  - Triggers soft restart automatically

Model Folders Database (model_folders.json)
- GET /api/config/folders → list tracked folders
- POST /api/config/folders { folderPaths[], recursive? } → add/update folders
- DELETE /api/config/folders { folderPaths[] } → remove folders

Options Object (used in regenerate-from-db)
```json
{
  "enableJinja": true,
  "throughputFirst": true,
  "minContext": 16384,
  "preferredContext": 32768,
  "forceBackend": "metal|cuda|rocm|vulkan|cpu|mlx",
  "forceVRAM": 32.0,
  "forceRAM": 32.0
}
```

OpenAI-Compatible Endpoints (proxied to llama-server)
- POST /v1/chat/completions → chat
- POST /v1/completions → completion
- GET /v1/models → list available models
- Health: GET /health

Typical Tasks
1) Append one model to existing config
```bash
curl -X POST http://localhost:5800/api/config/append-model \
  -H 'Content-Type: application/json' \
  -d '{"filePath":"/path/model.gguf","options":{"enableJinja":true}}'
```

2) Force Reconfigure from DB (same as UI "Force Reconfigure")
```bash
curl -X POST http://localhost:5800/api/config/regenerate-from-db \
  -H 'Content-Type: application/json' \
  -d '{"options":{"enableJinja":true,"throughputFirst":true,"minContext":16384,"preferredContext":32768}}'
```

3) Validate & Cleanup
```bash
curl -X POST http://localhost:5800/api/config/validate-models
curl -X POST http://localhost:5800/api/config/cleanup-duplicates
```

Behavioral Guarantees
- Backend auto-reconfigure after a download completes:
  - Adds the file’s folder to DB if missing
  - Regenerates config with saved settings (or defaults)
  - Soft-restart emitted automatically
- Startup self-heal: if config.yaml is invalid/missing, backend attempts full regenerate from DB
- macOS Apple Silicon backend mapping: unsupported backends are mapped to metal

Error Handling
- All endpoints return meaningful errors; operations are additive and avoid destructive changes
- Regeneration skips unreadable folders and continues; logs contain per-folder outcomes

Security & Auth
- Intended for local use by default; place behind your own reverse proxy if exposing remotely

Changelog Notes
- New endpoints: /api/settings/system (GET/POST)
- Auto-reconfigure triggers on download completion
- Force Reconfigure is equivalent to POST /api/config/regenerate-from-db



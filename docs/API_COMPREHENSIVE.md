# ClaraCore API Documentation

ClaraCore provides a comprehensive REST API for model management, configuration, and OpenAI-compatible inference. This document covers all available endpoints with detailed examples.

## Base URL

```
http://localhost:5800
```

## Authentication

ClaraCore supports optional API key authentication for security. When enabled, include the API key in requests:

```bash
Authorization: Bearer your-api-key
```

Configure authentication in system settings or disable it for local development.

---

## 📊 Table of Contents

- [OpenAI-Compatible Endpoints](#openai-compatible-endpoints)
- [Configuration Management](#configuration-management)
- [Model Management](#model-management)
- [System Information](#system-information)
- [Model Downloads](#model-downloads)
- [Settings](#settings)
- [Server Control](#server-control)
- [Events & Monitoring](#events--monitoring)
- [UI Routes](#ui-routes)

---

## 🤖 OpenAI-Compatible Endpoints

ClaraCore provides full OpenAI API compatibility for seamless integration with existing applications.

### Chat Completions

**POST** `/v1/chat/completions`

Create a chat completion using your loaded models.

```bash
curl -X POST http://localhost:5800/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.2-3b-instruct",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ],
    "temperature": 0.7,
    "max_tokens": 150
  }'
```

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "llama-3.2-3b-instruct",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you for asking. How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 15,
    "total_tokens": 27
  }
}
```

### Text Completions (Legacy)

**POST** `/v1/completions`

Legacy completion endpoint for backward compatibility.

```bash
curl -X POST http://localhost:5800/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.2-3b-instruct",
    "prompt": "The capital of France is",
    "max_tokens": 50
  }'
```

### Embeddings

**POST** `/v1/embeddings`

Generate embeddings for text using embedding models.

```bash
curl -X POST http://localhost:5800/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nomic-embed-text-v1.5",
    "input": "Your text here"
  }'
```

### Text-to-Speech

**POST** `/v1/audio/speech`

Generate speech from text using TTS models.

```bash
curl -X POST http://localhost:5800/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "input": "Hello world",
    "voice": "alloy"
  }'
```

### Speech-to-Text

**POST** `/v1/audio/transcriptions`

Transcribe audio files to text using STT models.

```bash
curl -X POST http://localhost:5800/v1/audio/transcriptions \
  -F "file=@audio.wav" \
  -F "model=whisper-1"
```

### Reranking

**POST** `/v1/rerank` or `/v1/reranking`

Rerank documents based on relevance to a query.

```bash
curl -X POST http://localhost:5800/v1/rerank \
  -H "Content-Type: application/json" \
  -d '{
    "model": "jina-reranker-v1-base-en",
    "query": "What is machine learning?",
    "documents": [
      "Machine learning is a subset of AI",
      "Python is a programming language",
      "Neural networks are used in deep learning"
    ]
  }'
```

### List Models

**GET** `/v1/models`

List all available models in OpenAI format.

```bash
curl http://localhost:5800/v1/models
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "llama-3.2-3b-instruct",
      "object": "model",
      "created": 1677610602,
      "owned_by": "organization-owner"
    }
  ]
}
```

---

## ⚙️ Configuration Management

### Get Configuration

**GET** `/api/config`

Retrieve the current configuration in YAML format.

```bash
curl http://localhost:5800/api/config
```

**Response:**
```yaml
host: "127.0.0.1"
port: 8080
models:
  - name: "llama-3.2-3b-instruct"
    backend: "cuda"
    context_length: 8192
    # ... other model parameters
```

### Update Configuration

**POST** `/api/config`

Update the entire configuration. **Note:** This will prompt for a soft restart.

```bash
curl -X POST http://localhost:5800/api/config \
  -H "Content-Type: application/yaml" \
  -d '@config.yaml'
```

**Response:**
```json
{
  "message": "Configuration updated successfully",
  "needsRestart": true,
  "restartMessage": "Configuration updated! A soft restart is recommended to apply changes."
}
```

### Update Model Parameters

**POST** `/api/config/model/:id`

Update specific parameters for a single model. **Note:** This will prompt for a soft restart.

```bash
curl -X POST http://localhost:5800/api/config/model/llama-3.2-3b-instruct \
  -H "Content-Type: application/json" \
  -d '{
    "temperature": 0.8,
    "context_length": 4096,
    "max_tokens": 512
  }'
```

**Response:**
```json
{
  "message": "Model parameters updated successfully",
  "needsRestart": true,
  "restartMessage": "Model configuration updated! A soft restart is recommended to apply changes."
}
```

### Delete Model

**DELETE** `/api/config/models/:id`

Remove a model from the configuration.

```bash
curl -X DELETE http://localhost:5800/api/config/models/llama-3.2-3b-instruct
```

### Validate Configuration

**GET** `/api/config/validate`

Validate the current configuration for syntax and logical errors.

```bash
curl http://localhost:5800/api/config/validate
```

**Response:**
```json
{
  "valid": true,
  "errors": [],
  "warnings": ["Model file path may not exist: /path/to/model.gguf"]
}
```

### Validate Models on Disk

**POST** `/api/config/validate-models`

Check if all configured model files actually exist on disk.

```bash
curl -X POST http://localhost:5800/api/config/validate-models
```

### Cleanup Duplicate Models

**POST** `/api/config/cleanup-duplicates`

Remove duplicate model entries from the configuration.

```bash
curl -X POST http://localhost:5800/api/config/cleanup-duplicates
```

---

## 📁 Model Folder Management

### Get Model Folders

**GET** `/api/config/folders`

Get all tracked model folders from the database.

```bash
curl http://localhost:5800/api/config/folders
```

**Response:**
```json
{
  "folders": [
    {
      "path": "/home/user/models",
      "enabled": true,
      "addedAt": "2025-01-01T12:00:00Z",
      "modelCount": 15
    }
  ]
}
```

### Add Model Folders

**POST** `/api/config/folders`

Add new folders to track for model discovery.

```bash
curl -X POST http://localhost:5800/api/config/folders \
  -H "Content-Type: application/json" \
  -d '{
    "folderPaths": ["/path/to/models", "/another/path"],
    "recursive": true
  }'
```

### Remove Model Folders

**DELETE** `/api/config/folders`

Remove folders from tracking.

```bash
curl -X DELETE http://localhost:5800/api/config/folders \
  -H "Content-Type: application/json" \
  -d '{
    "folderPaths": ["/path/to/remove"]
  }'
```

### Scan Model Folder

**POST** `/api/config/scan-folder`

Scan a specific folder for GGUF models.

```bash
curl -X POST http://localhost:5800/api/config/scan-folder \
  -H "Content-Type: application/json" \
  -d '{
    "folderPath": "/path/to/models",
    "recursive": true,
    "addToDatabase": false
  }'
```

### Add Single Model

**POST** `/api/config/add-model`

Generate configuration for a single model file.

```bash
curl -X POST http://localhost:5800/api/config/add-model \
  -H "Content-Type: application/json" \
  -d '{
    "modelPath": "/path/to/model.gguf",
    "modelName": "custom-model"
  }'
```

### Append Model to Config

**POST** `/api/config/append-model`

Add a model to the existing configuration file. **Note:** This will prompt for a soft restart.

```bash
curl -X POST http://localhost:5800/api/config/append-model \
  -H "Content-Type: application/json" \
  -d '{
    "filePath": "/path/to/model.gguf",
    "options": {
      "enableJinja": true,
      "throughputFirst": true
    }
  }'
```

### Regenerate from Database

**POST** `/api/config/regenerate-from-db`

Regenerate the entire YAML configuration from the tracked folders database. **Note:** This automatically performs a soft restart.

```bash
curl -X POST http://localhost:5800/api/config/regenerate-from-db \
  -H "Content-Type: application/json" \
  -d '{
    "options": {
      "enableJinja": true,
      "throughputFirst": true,
      "minContext": 16384,
      "preferredContext": 32768,
      "forceBackend": "vulkan",
      "forceVRAM": 8.0,
      "forceRAM": 16.0
    }
  }'
```

---

## 🚀 Auto-Setup & Generation

### Generate All Models (Smart Setup)

**POST** `/api/config/generate-all`

Intelligently generate configuration for all models in tracked folders with system optimization.

```bash
curl -X POST http://localhost:5800/api/config/generate-all \
  -H "Content-Type: application/json" \
  -d '{
    "forceBackend": "vulkan",
    "forceVRAM": 8.0,
    "forceRAM": 16.0,
    "enableJinja": true,
    "throughputFirst": true,
    "minContext": 4096,
    "preferredContext": 8192
  }'
```

**Parameters:**
- `forceBackend`: Override backend detection (cuda/rocm/vulkan/metal/cpu)
- `forceVRAM`: Override detected VRAM (in GB)
- `forceRAM`: Override detected RAM (in GB)
- `enableJinja`: Enable Jinja template support
- `throughputFirst`: Prioritize throughput over accuracy
- `minContext`: Minimum context length
- `preferredContext`: Preferred context length

### Get Setup Progress

**GET** `/api/setup/progress`

Get real-time progress of setup operations (used for UI polling).

```bash
curl http://localhost:5800/api/setup/progress
```

**Response:**
```json
{
  "status": "scanning",
  "step": "Detecting models in /path/to/models",
  "progress": 45,
  "totalSteps": 100,
  "currentOperation": "Model detection"
}
```

---

## 🖥️ System Information

### Get System Specs

**GET** `/api/system/specs`

Get basic system specifications.

```bash
curl http://localhost:5800/api/system/specs
```

**Response:**
```json
{
  "os": "windows",
  "arch": "amd64",
  "cpu": "Intel Core i7-10700K",
  "ramGB": 32.0,
  "gpuInfo": "NVIDIA RTX 3080"
}
```

### Get System Detection

**GET** `/api/system/detection`

Get comprehensive system detection results for setup.

```bash
curl http://localhost:5800/api/system/detection
```

**Response:**
```json
{
  "os": "windows",
  "architecture": "amd64",
  "hasCUDA": true,
  "hasROCm": false,
  "hasVulkan": true,
  "hasMetal": false,
  "totalRAMGB": 32.0,
  "totalVRAMGB": 10.0,
  "gpuType": "nvidia",
  "recommendedBackend": "cuda",
  "cpuCores": 8,
  "cpuThreads": 16
}
```

---

## 📥 Model Downloads

### Download Model

**POST** `/api/models/download`

Download a model from Hugging Face.

```bash
curl -X POST http://localhost:5800/api/models/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://huggingface.co/microsoft/DialoGPT-medium/resolve/main/model.gguf",
    "modelId": "diagpt-medium",
    "filename": "model.gguf",
    "hfApiKey": "hf_your_token_here"
  }'
```

### Cancel Download

**POST** `/api/models/download/cancel`

Cancel all active downloads.

```bash
curl -X POST http://localhost:5800/api/models/download/cancel
```

### Get Downloads

**GET** `/api/models/downloads`

List all download tasks and their status.

```bash
curl http://localhost:5800/api/models/downloads
```

### Get Download Status

**GET** `/api/models/downloads/:id`

Get status of a specific download.

```bash
curl http://localhost:5800/api/models/downloads/download-123
```

### Pause Download

**POST** `/api/models/downloads/:id/pause`

Pause a specific download.

```bash
curl -X POST http://localhost:5800/api/models/downloads/download-123/pause
```

### Resume Download

**POST** `/api/models/downloads/:id/resume`

Resume a paused download.

```bash
curl -X POST http://localhost:5800/api/models/downloads/download-123/resume
```

---

## ⚙️ Settings

### Get System Settings

**GET** `/api/settings/system`

Get current system settings (user preferences).

```bash
curl http://localhost:5800/api/settings/system
```

**Response:**
```json
{
  "gpuType": "nvidia",
  "backend": "cuda",
  "vramGB": 10.0,
  "ramGB": 32.0,
  "preferredContext": 8192,
  "throughputFirst": true,
  "enableJinja": true,
  "requireApiKey": false,
  "apiKey": ""
}
```

### Set System Settings

**POST** `/api/settings/system`

Update system settings.

```bash
curl -X POST http://localhost:5800/api/settings/system \
  -H "Content-Type: application/json" \
  -d '{
    "backend": "vulkan",
    "vramGB": 8.0,
    "preferredContext": 4096,
    "requireApiKey": true,
    "apiKey": "your-secret-key"
  }'
```

### Get HuggingFace API Key

**GET** `/api/settings/hf-api-key`

Get the stored HuggingFace API key status.

```bash
curl http://localhost:5800/api/settings/hf-api-key
```

### Set HuggingFace API Key

**POST** `/api/settings/hf-api-key`

Set the HuggingFace API key for model downloads.

```bash
curl -X POST http://localhost:5800/api/settings/hf-api-key \
  -H "Content-Type: application/json" \
  -d '{
    "apiKey": "hf_your_token_here"
  }'
```

---

## 🔄 Server Control

### Soft Restart

**POST** `/api/server/restart`

Perform a soft restart (reload configuration without stopping the process).

```bash
curl -X POST http://localhost:5800/api/server/restart
```

**Response:**
```json
{
  "message": "Server restarted successfully",
  "reloadedAt": "2025-01-01T12:00:00Z"
}
```

### Hard Restart

**POST** `/api/server/restart/hard`

Perform a hard restart (full process restart).

```bash
curl -X POST http://localhost:5800/api/server/restart/hard
```

---

## 📊 Model Management

### Unload All Models

**POST** `/api/models/unload`

Unload all currently loaded models from memory.

```bash
curl -X POST http://localhost:5800/api/models/unload
```

**Response:**
```json
{
  "message": "All models unloaded successfully",
  "unloadedCount": 3
}
```

---

## 📡 Events & Monitoring

### Get Events

**GET** `/api/events`

Get real-time server events via Server-Sent Events (SSE).

```bash
curl -N http://localhost:5800/api/events
```

**Response:** (streaming)
```
data: {"type":"model_status","data":{"models":[...]}}

data: {"type":"log","data":{"source":"llama-server","message":"Model loaded"}}

data: {"type":"metric","data":{"cpu":45.2,"memory":78.3}}
```

### Get Metrics

**GET** `/api/metrics`

Get current system metrics.

```bash
curl http://localhost:5800/api/metrics
```

**Response:**
```json
{
  "cpu": {
    "usage": 45.2,
    "cores": 8
  },
  "memory": {
    "used": 25.6,
    "total": 32.0,
    "percent": 80.0
  },
  "gpu": {
    "usage": 85.5,
    "memoryUsed": 8.2,
    "memoryTotal": 10.0
  },
  "models": {
    "loaded": 2,
    "total": 15
  }
}
```

---

## 🌐 UI Routes

ClaraCore serves a React-based web interface at `/ui/*`. The UI automatically redirects from the root path.

### Available UI Pages

- **`/`** - Redirects to `/ui/models`
- **`/ui/models`** - Model management and chat interface
- **`/ui/setup`** - Initial setup wizard
- **`/ui/configuration`** - Configuration editor
- **`/ui/downloads`** - Model download manager
- **`/ui/settings`** - System settings
- **`/ui/activity`** - Activity and logs viewer

### Static Assets

- **`/ui/assets/*`** - CSS, JS, and other UI assets
- **`/ui/favicon.ico`** - Favicon
- **`/ui/site.webmanifest`** - Web app manifest

---

## 🔐 Authentication & Security

### API Key Authentication

When `requireApiKey` is enabled in system settings, all API endpoints require authentication:

```bash
# Set API key
curl -X POST http://localhost:5800/api/settings/system \
  -H "Content-Type: application/json" \
  -d '{
    "requireApiKey": true,
    "apiKey": "your-secret-key"
  }'

# Use API key in requests
curl -H "Authorization: Bearer your-secret-key" \
  http://localhost:5800/api/config
```

### CORS Configuration

ClaraCore automatically handles CORS for cross-origin requests. The web UI is served from the same origin, so no additional configuration is needed for local development.

---

## 🚨 Error Handling

All API endpoints return consistent error responses:

```json
{
  "error": "Model not found",
  "code": "MODEL_NOT_FOUND",
  "details": {
    "modelId": "nonexistent-model",
    "availableModels": ["llama-3.2-3b-instruct"]
  }
}
```

### Common HTTP Status Codes

- **200** - Success
- **201** - Created
- **400** - Bad Request (invalid parameters)
- **401** - Unauthorized (API key required)
- **404** - Not Found (model/resource not found)
- **409** - Conflict (model already exists)
- **500** - Internal Server Error
- **503** - Service Unavailable (model loading)

---

## 📋 Configuration File Format

ClaraCore uses YAML configuration files. Here's a complete example:

```yaml
# config.yaml
host: "127.0.0.1"
port: 8080
cors: true
api_key: ""

models:
  - name: "llama-3.2-3b-instruct"
    backend: "cuda"
    model: "/path/to/llama-3.2-3b-instruct.Q4_K_M.gguf"
    context_length: 8192
    max_tokens: 4096
    temperature: 0.7
    top_p: 0.9
    top_k: 40
    repeat_penalty: 1.1
    gpu_layers: -1
    batch_size: 512
    threads: 8
    
  - name: "nomic-embed-text-v1.5"
    backend: "cuda"
    model: "/path/to/nomic-embed-text-v1.5.f16.gguf"
    context_length: 8192
    embedding: true
    gpu_layers: -1
```

---

## 🔄 Real-time Features

### Soft Restart Prompts

When you modify model configuration via the API, ClaraCore will return a response indicating that a restart is recommended:

```json
{
  "message": "Configuration updated successfully",
  "needsRestart": true,
  "restartMessage": "Configuration updated! A soft restart is recommended to apply changes."
}
```

The web UI will automatically show a restart prompt when this flag is detected.

### Progress Tracking

Setup operations provide real-time progress via the `/api/setup/progress` endpoint. The UI polls this endpoint every 500ms during setup to show accurate progress.

### Live Events

The `/api/events` endpoint provides real-time updates about:
- Model loading/unloading
- Configuration changes  
- System metrics
- Log messages
- Download progress

---

## 🛠️ Development & Integration

### Using with Python

```python
import requests

# Get models
response = requests.get("http://localhost:5800/v1/models")
models = response.json()

# Chat completion
response = requests.post("http://localhost:5800/v1/chat/completions", json={
    "model": "llama-3.2-3b-instruct",
    "messages": [{"role": "user", "content": "Hello!"}]
})
completion = response.json()
```

### Using with JavaScript

```javascript
// Chat completion
const response = await fetch('http://localhost:5800/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer your-api-key' // if authentication enabled
  },
  body: JSON.stringify({
    model: 'llama-3.2-3b-instruct',
    messages: [{role: 'user', content: 'Hello!'}]
  })
});

const completion = await response.json();
```

### Using with cURL

```bash
# Quick chat completion test
curl -X POST http://localhost:5800/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.2-3b-instruct",
    "messages": [{"role": "user", "content": "Tell me a joke"}],
    "max_tokens": 100
  }' | jq '.choices[0].message.content'
```

---

## 🚀 Quick Start Examples

### 1. Basic Setup Flow

```bash
# 1. Add model folders
curl -X POST http://localhost:5800/api/config/folders \
  -H "Content-Type: application/json" \
  -d '{"folderPaths": ["/path/to/models"], "recursive": true}'

# 2. Generate configuration from folders
curl -X POST http://localhost:5800/api/config/regenerate-from-db \
  -H "Content-Type: application/json" \
  -d '{
    "options": {
      "enableJinja": true,
      "throughputFirst": true,
      "minContext": 16384,
      "preferredContext": 32768
    }
  }'

# 3. Check status
curl http://localhost:5800/v1/models
```

### 2. Model Download & Auto-Config

```bash
# Download a model (auto-configures)
curl -X POST http://localhost:5800/api/models/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://huggingface.co/microsoft/DialoGPT-medium/resolve/main/model.gguf",
    "modelId": "diagpt-medium",
    "filename": "model.gguf"
  }'

# Check download progress
curl http://localhost:5800/api/models/downloads
```

### 3. System Settings Configuration

```bash
# Configure system preferences
curl -X POST http://localhost:5800/api/settings/system \
  -H "Content-Type: application/json" \
  -d '{
    "gpuType": "nvidia",
    "backend": "cuda",
    "vramGB": 10.0,
    "ramGB": 32.0,
    "preferredContext": 8192,
    "throughputFirst": true,
    "enableJinja": true
  }'
```

### 4. Configuration Maintenance

```bash
# Validate current config
curl http://localhost:5800/api/config/validate

# Check for missing model files
curl -X POST http://localhost:5800/api/config/validate-models

# Remove duplicates
curl -X POST http://localhost:5800/api/config/cleanup-duplicates

# Soft restart to apply changes
curl -X POST http://localhost:5800/api/server/restart
```

---

## 📚 Additional Resources

- **Setup Guide**: See `/docs/setup.md` for initial configuration
- **Model Support**: ClaraCore supports GGUF models from Hugging Face
- **Performance Tuning**: Adjust `gpu_layers`, `context_length`, and `batch_size` for optimal performance
- **Troubleshooting**: Check `/api/events` for real-time error messages and logs

---

**Last Updated**: October 2025  
**API Version**: 1.0  
**ClaraCore Version**: Latest
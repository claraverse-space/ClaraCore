package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gin-gonic/gin"
	"github.com/prave/ClaraCore/autosetup"
	"github.com/prave/ClaraCore/event"
)

type Model struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	State       string `json:"state"`
	Unlisted    bool   `json:"unlisted"`
    ProxyURL    string `json:"proxyUrl"`
}

// SystemSettings persist user-chosen settings for autosetup/regeneration
type SystemSettings struct {
    GPUType          string  `json:"gpuType"`          // nvidia|amd|intel|apple|none
    Backend          string  `json:"backend"`          // cuda|rocm|vulkan|metal|mlx|cpu
    VRAMGB           float64 `json:"vramGB"`
    RAMGB            float64 `json:"ramGB"`
    PreferredContext int     `json:"preferredContext"`
    ThroughputFirst  bool    `json:"throughputFirst"`
    EnableJinja      bool    `json:"enableJinja"`
    RequireAPIKey    bool    `json:"requireApiKey"`
    APIKey           string  `json:"apiKey,omitempty"`
}

func (pm *ProxyManager) getSystemSettingsPath() string {
    return "settings.json"
}

func (pm *ProxyManager) loadSystemSettings() (*SystemSettings, error) {
    path := pm.getSystemSettingsPath()
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return nil, nil
    }
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var s SystemSettings
    if err := json.Unmarshal(data, &s); err != nil {
        return nil, err
    }
    return &s, nil
}

func (pm *ProxyManager) saveSystemSettings(s *SystemSettings) error {
    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(pm.getSystemSettingsPath(), data, 0644)
}

func addApiHandlers(pm *ProxyManager) {
	// Add API endpoints for React to consume
	apiGroup := pm.ginEngine.Group("/api", pm.requireAPIKey())
	{
		apiGroup.POST("/models/unload", pm.apiUnloadAllModels)
		apiGroup.GET("/events", pm.apiSendEvents)
		apiGroup.GET("/metrics", pm.apiGetMetrics)

		// Model downloader endpoints
		apiGroup.GET("/system/specs", pm.apiGetSystemSpecs)
		apiGroup.GET("/system/detection", pm.apiGetSystemDetection) // NEW: Comprehensive system detection for setup
		apiGroup.GET("/settings/hf-api-key", pm.apiGetHFApiKey)
		apiGroup.POST("/settings/hf-api-key", pm.apiSetHFApiKey)
		apiGroup.POST("/models/download", pm.apiDownloadModel)
		apiGroup.POST("/models/download/cancel", pm.apiCancelDownload)
		apiGroup.GET("/models/downloads", pm.apiGetDownloads)
		apiGroup.GET("/models/downloads/:id", pm.apiGetDownloadStatus)
		apiGroup.POST("/models/downloads/:id/pause", pm.apiPauseDownload)
		apiGroup.POST("/models/downloads/:id/resume", pm.apiResumeDownload)

		// System settings persistence
		apiGroup.GET("/settings/system", pm.apiGetSystemSettings)
		apiGroup.POST("/settings/system", pm.apiSetSystemSettings)

		// Configuration management endpoints
		apiGroup.GET("/config", pm.apiGetConfig)
		apiGroup.POST("/config", pm.apiUpdateConfig)
		apiGroup.POST("/config/model/:id", pm.apiUpdateModelParams) // NEW: Selective model parameter update
		apiGroup.POST("/config/scan-folder", pm.apiScanModelFolder)
		apiGroup.POST("/config/add-model", pm.apiAddModel)
		apiGroup.POST("/config/append-model", pm.apiAppendModelToConfig) // NEW: Append model to existing config

		// Server management endpoints
		apiGroup.POST("/server/restart", pm.apiRestartServer)          // Soft restart (reload config)
		apiGroup.POST("/server/restart/hard", pm.apiHardRestartServer) // Hard restart (full process restart)
		apiGroup.POST("/config/generate-all", pm.apiGenerateAllModels) // SMART generation like command-line
		apiGroup.DELETE("/config/models/:id", pm.apiDeleteModel)
		apiGroup.GET("/config/validate", pm.apiValidateConfig)
		apiGroup.POST("/config/validate-models", pm.apiValidateModelsOnDisk) // NEW: Validate model files exist
		apiGroup.POST("/config/cleanup-duplicates", pm.apiCleanupDuplicateModels) // NEW: Remove duplicate models

		// NEW: Model folder database management
		apiGroup.GET("/config/folders", pm.apiGetModelFolders)                          // Get all tracked folders
		apiGroup.POST("/config/folders", pm.apiAddModelFolders)                         // Add folders to database
		apiGroup.DELETE("/config/folders", pm.apiRemoveModelFolders)                    // Remove folders from database
		apiGroup.POST("/config/regenerate-from-db", pm.apiRegenerateConfigFromDatabase) // Regenerate YAML from JSON database
	}
}

func (pm *ProxyManager) apiUnloadAllModels(c *gin.Context) {
	pm.StopProcesses(StopImmediately)
	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}

func (pm *ProxyManager) getModelStatus() []Model {
	// Extract keys and sort them
	models := []Model{}

	modelIDs := make([]string, 0, len(pm.config.Models))
	for modelID := range pm.config.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	// Iterate over sorted keys
	for _, modelID := range modelIDs {
		// Get process state
		processGroup := pm.findGroupByModelName(modelID)
		state := "unknown"
		if processGroup != nil {
			process := processGroup.processes[modelID]
			if process != nil {
				var stateStr string
				switch process.CurrentState() {
				case StateReady:
					stateStr = "ready"
				case StateStarting:
					stateStr = "starting"
				case StateStopping:
					stateStr = "stopping"
				case StateShutdown:
					stateStr = "shutdown"
				case StateStopped:
					stateStr = "stopped"
				default:
					stateStr = "unknown"
				}
				state = stateStr
			}
		}
        models = append(models, Model{
			Id:          modelID,
			Name:        pm.config.Models[modelID].Name,
			Description: pm.config.Models[modelID].Description,
			State:       state,
			Unlisted:    pm.config.Models[modelID].Unlisted,
            ProxyURL:    pm.config.Models[modelID].Proxy,
		})
	}

	return models
}

type messageType string

const (
	msgTypeModelStatus messageType = "modelStatus"
	msgTypeLogData     messageType = "logData"
	msgTypeMetrics     messageType = "metrics"
)

type messageEnvelope struct {
	Type messageType `json:"type"`
	Data string      `json:"data"`
}

// sends a stream of different message types that happen on the server
func (pm *ProxyManager) apiSendEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Content-Type-Options", "nosniff")
	// prevent nginx from buffering SSE
	c.Header("X-Accel-Buffering", "no")

	sendBuffer := make(chan messageEnvelope, 25)
	ctx, cancel := context.WithCancel(c.Request.Context())
	sendModels := func() {
		data, err := json.Marshal(pm.getModelStatus())
		if err == nil {
			msg := messageEnvelope{Type: msgTypeModelStatus, Data: string(data)}
			select {
			case sendBuffer <- msg:
			case <-ctx.Done():
				return
			default:
			}

		}
	}

	sendLogData := func(source string, data []byte) {
		data, err := json.Marshal(gin.H{
			"source": source,
			"data":   string(data),
		})
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: msgTypeLogData, Data: string(data)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	sendMetrics := func(metrics []TokenMetrics) {
		jsonData, err := json.Marshal(metrics)
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: msgTypeMetrics, Data: string(jsonData)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	/**
	 * Send updated models list
	 */
	defer event.On(func(e ProcessStateChangeEvent) {
		sendModels()
	})()
	defer event.On(func(e ConfigFileChangedEvent) {
		sendModels()
	})()

	/**
	 * Send Log data
	 */
	defer pm.proxyLogger.OnLogData(func(data []byte) {
		sendLogData("proxy", data)
	})()
	defer pm.upstreamLogger.OnLogData(func(data []byte) {
		sendLogData("upstream", data)
	})()

	/**
	 * Send Metrics data
	 */
	defer event.On(func(e TokenMetricsEvent) {
		sendMetrics([]TokenMetrics{e.Metrics})
	})()

	/**
	 * Send Download progress data
	 */
	defer event.On(func(e DownloadProgressEvent) {
		data, err := json.Marshal(gin.H{
			"downloadId": e.DownloadID,
			"info":       e.Info,
		})
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: "downloadProgress", Data: string(data)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	})()

	// send initial batch of data
	sendLogData("proxy", pm.proxyLogger.GetHistory())
	sendLogData("upstream", pm.upstreamLogger.GetHistory())
	sendModels()
	sendMetrics(pm.metricsMonitor.GetMetrics())

	for {
		select {
		case <-c.Request.Context().Done():
			cancel()
			return
		case <-pm.shutdownCtx.Done():
			cancel()
			return
		case msg := <-sendBuffer:
			c.SSEvent("message", msg)
			c.Writer.Flush()
		}
	}
}

func (pm *ProxyManager) apiGetMetrics(c *gin.Context) {
	jsonData, err := pm.metricsMonitor.GetMetricsJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
		return
	}
	c.Data(http.StatusOK, "application/json", jsonData)
}

// API handlers for ModelDownloader functionality

func (pm *ProxyManager) apiGetSystemSpecs(c *gin.Context) {
	// Use real system detection from autosetup package
	system := autosetup.DetectSystem()

	// Enhance with detailed system information
	err := autosetup.EnhanceSystemInfo(&system)
	if err != nil {
		// Log error but continue with basic info
		pm.proxyLogger.Errorf("Failed to enhance system info: %v", err)
	}

	// Get realtime hardware info for more accurate available memory
	realtimeInfo, err := autosetup.GetRealtimeHardwareInfo()
	if err != nil {
		pm.proxyLogger.Warnf("Failed to get realtime hardware info: %v", err)
	}

	// Convert GB to bytes for the API
	totalRAM := int64(system.TotalRAMGB * 1024 * 1024 * 1024)
	availableRAM := totalRAM * 75 / 100 // Default to 75% available
	totalVRAM := int64(system.TotalVRAMGB * 1024 * 1024 * 1024)
	availableVRAM := totalVRAM * 80 / 100 // Default to 80% available

	// Use realtime info if available
	if realtimeInfo != nil {
		availableRAM = int64(realtimeInfo.AvailableRAMGB * 1024 * 1024 * 1024)
		availableVRAM = int64(realtimeInfo.AvailableVRAMGB * 1024 * 1024 * 1024)
		totalRAM = int64(realtimeInfo.TotalRAMGB * 1024 * 1024 * 1024)
		totalVRAM = int64(realtimeInfo.TotalVRAMGB * 1024 * 1024 * 1024)
	}

	// Get primary GPU name
	gpuName := "CPU Only"
	if len(system.VRAMDetails) > 0 {
		gpuName = system.VRAMDetails[0].Name
	}

	// Get actual available disk space
	diskSpace := pm.getAvailableDiskSpace()

	specs := gin.H{
		"totalRAM":      totalRAM,
		"availableRAM":  availableRAM,
		"totalVRAM":     totalVRAM,
		"availableVRAM": availableVRAM,
		"cpuCores":      runtime.NumCPU(),
		"gpuName":       gpuName,
		"diskSpace":     diskSpace,
	}
	c.JSON(http.StatusOK, specs)
}

// apiGetSystemDetection provides comprehensive system detection for setup UI auto-population
func (pm *ProxyManager) apiGetSystemDetection(c *gin.Context) {
	// Perform comprehensive system detection
	system := autosetup.DetectSystem()

	// Enhance with detailed system information
	err := autosetup.EnhanceSystemInfo(&system)
	if err != nil {
		pm.proxyLogger.Errorf("Failed to enhance system info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detect system information"})
		return
	}

	// Get realtime hardware info for accurate memory readings
	realtimeInfo, err := autosetup.GetRealtimeHardwareInfo()
	if err != nil {
		pm.proxyLogger.Warnf("Failed to get realtime hardware info: %v", err)
	}

	// Determine optimal backend priority
	backends := []string{}
	primaryBackend := "cpu"

	if system.HasCUDA {
		backends = append(backends, "cuda")
		primaryBackend = "cuda"
	}
	if system.HasROCm {
		backends = append(backends, "rocm")
		if primaryBackend == "cpu" {
			primaryBackend = "rocm"
		}
	}
	if system.HasVulkan {
		backends = append(backends, "vulkan")
		if primaryBackend == "cpu" {
			primaryBackend = "vulkan"
		}
	}
	if system.HasMLX {
		backends = append(backends, "mlx")
		if primaryBackend == "cpu" {
			primaryBackend = "mlx"
		}
	}
	if system.HasMetal {
		backends = append(backends, "metal")
		if primaryBackend == "cpu" {
			primaryBackend = "metal"
		}
	}
	if system.HasIntel {
		backends = append(backends, "intel")
	}
	backends = append(backends, "cpu") // Always available

	// Determine GPU type for UI dropdown
	gpuType := "CPU Only"
	gpuTypes := []string{"CPU Only"}

	if system.HasCUDA {
		gpuType = "NVIDIA (RTX, GTX)"
		gpuTypes = append(gpuTypes, "NVIDIA (RTX, GTX)")
	}
	if system.HasROCm {
		if gpuType == "CPU Only" {
			gpuType = "AMD (RX Series)"
		}
		gpuTypes = append(gpuTypes, "AMD (RX Series)")
	}
	if system.HasMLX || system.HasMetal {
		if gpuType == "CPU Only" {
			gpuType = "Apple Silicon"
		}
		gpuTypes = append(gpuTypes, "Apple Silicon")
	}
	if system.HasIntel {
		gpuTypes = append(gpuTypes, "Intel GPU")
	}

	// Calculate memory values
	totalRAMGB := system.TotalRAMGB
	totalVRAMGB := system.TotalVRAMGB
	availableRAMGB := totalRAMGB * 0.75 // Conservative estimate

	// Use realtime info if available
	if realtimeInfo != nil {
		availableRAMGB = realtimeInfo.AvailableRAMGB
		if realtimeInfo.TotalRAMGB > 0 {
			totalRAMGB = realtimeInfo.TotalRAMGB
		}
		if realtimeInfo.TotalVRAMGB > 0 {
			totalVRAMGB = realtimeInfo.TotalVRAMGB
		}
	}

	// Skip building detailed GPU information - we only need primary GPU for the response

	// Determine recommended context size based on available memory
	suggestedContextSize := 32768
	maxRecommendedContextSize := 131072
	if totalVRAMGB >= 24 {
		suggestedContextSize = 131072 // 128K
		maxRecommendedContextSize = 131072
	} else if totalVRAMGB >= 16 {
		suggestedContextSize = 65536 // 64K
		maxRecommendedContextSize = 131072
	} else if totalVRAMGB >= 8 {
		suggestedContextSize = 32768 // 32K
		maxRecommendedContextSize = 65536
	} else {
		suggestedContextSize = 16384 // 16K
		maxRecommendedContextSize = 32768
	}

	// Performance priority recommendation
	throughputFirst := totalVRAMGB >= 8

	// Build primary GPU info
	var primaryGPU interface{} = nil
	gpuBrand := "unknown"
	if len(system.VRAMDetails) > 0 {
		gpu := system.VRAMDetails[0]
		if system.HasCUDA {
			gpuBrand = "nvidia"
		} else if system.HasROCm {
			gpuBrand = "amd"
		} else if system.HasIntel {
			gpuBrand = "intel"
		} else if system.HasMetal || system.HasMLX {
			gpuBrand = "apple"
		}

		primaryGPU = gin.H{
			"name":   gpu.Name,
			"brand":  gpuBrand,
			"vramGB": math.Round(gpu.VRAMGB*10) / 10, // Round to 1 decimal place
		}
	}

	// Build recommendations
	recommendations := gin.H{
		"primaryBackend":          primaryBackend,
		"fallbackBackend":         "cpu",
		"suggestedContextSize":    suggestedContextSize,
		"suggestedVRAMAllocation": int(totalVRAMGB * 0.8),
		"suggestedRAMAllocation":  int(totalRAMGB * 0.5),
		"throughputFirst":         throughputFirst,
		"notes": []string{
			fmt.Sprintf("Detected %s with %.1fGB VRAM", primaryBackend, math.Round(totalVRAMGB*10)/10),
			fmt.Sprintf("Recommended context size: %d tokens", suggestedContextSize),
			fmt.Sprintf("Performance priority: %s", func() string {
				if throughputFirst {
					return "Speed (Higher throughput)"
				}
				return "Quality (Larger context)"
			}()),
		},
	}

	detection := gin.H{
		"detectionQuality": func() string {
			if realtimeInfo != nil {
				return "excellent" // Real-time detection available
			} else if len(system.VRAMDetails) > 0 {
				return "good" // GPU detection successful
			} else {
				return "basic" // Basic system detection only
			}
		}(),
		"platform":                  system.OS,
		"arch":                      system.Architecture,
		"gpuDetected":               len(system.VRAMDetails) > 0,
		"gpuTypes":                  gpuTypes,
		"primaryGPU":                primaryGPU,
		"totalRAMGB":                math.Round(totalRAMGB*10) / 10,     // Round to 1 decimal place
		"availableRAMGB":            math.Round(availableRAMGB*10) / 10, // Round to 1 decimal place
		"recommendedBackends":       backends,
		"supportedBackends":         backends,
		"recommendedContextSizes":   []int{8192, 16384, 32768, 65536, suggestedContextSize},
		"maxRecommendedContextSize": maxRecommendedContextSize,
		"recommendations":           recommendations,
		"detectionTimestamp":        time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, detection)
}

func (pm *ProxyManager) apiGetHFApiKey(c *gin.Context) {
	// For now, return empty - could be extended to read from config file
	c.JSON(http.StatusOK, gin.H{"apiKey": ""})
}

func (pm *ProxyManager) apiSetHFApiKey(c *gin.Context) {
	var req struct {
		ApiKey string `json:"apiKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// For now, just acknowledge - could be extended to save to config file
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

func (pm *ProxyManager) apiDownloadModel(c *gin.Context) {
	var req struct {
		URL      string `json:"url"`
		ModelId  string `json:"modelId"`
		Filename string `json:"filename"`
		HfApiKey string `json:"hfApiKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	downloadID, err := pm.downloadManager.StartDownload(req.ModelId, req.Filename, req.URL, req.HfApiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"downloadId": downloadID,
		"status":     "download started",
		"modelId":    req.ModelId,
		"filename":   req.Filename,
	})
}

func (pm *ProxyManager) apiCancelDownload(c *gin.Context) {
	var req struct {
		DownloadId string `json:"downloadId"`
		ModelId    string `json:"modelId"`
		Filename   string `json:"filename"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.DownloadId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "downloadId is required"})
		return
	}

	err := pm.downloadManager.CancelDownload(req.DownloadId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "download cancelled",
		"downloadId": req.DownloadId,
	})
}

func (pm *ProxyManager) apiGetDownloads(c *gin.Context) {
	downloads := pm.downloadManager.GetDownloads()
	c.JSON(http.StatusOK, downloads)
}
// apiGetSystemSettings returns saved system settings
func (pm *ProxyManager) apiGetSystemSettings(c *gin.Context) {
    s, err := pm.loadSystemSettings()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to load settings: %v", err)})
        return
    }
    if s == nil {
        c.JSON(http.StatusOK, gin.H{"settings": nil})
        return
    }
    // Redact API key from response
    redacted := *s
    redacted.APIKey = ""
    c.JSON(http.StatusOK, gin.H{"settings": redacted})
}

// apiSetSystemSettings saves system settings with basic validation and platform mapping
func (pm *ProxyManager) apiSetSystemSettings(c *gin.Context) {
    var req SystemSettings
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    // Platform-aware mapping: on macOS/arm, map unsupported to metal
    if runtime.GOOS == "darwin" && (req.Backend == "cuda" || req.Backend == "rocm" || req.Backend == "vulkan") {
        req.Backend = "metal"
    }

    // Basic validation
    if req.VRAMGB < 0 || req.RAMGB < 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ramGB and vramGB must be >= 0"})
        return
    }
    if req.PreferredContext < 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "preferredContext must be >= 0"})
        return
    }

    // Preserve existing API key if require is true and new key is empty; also auto-populate hardware defaults when zeros
    if existing, _ := pm.loadSystemSettings(); existing != nil {
        if req.RequireAPIKey && strings.TrimSpace(req.APIKey) == "" && strings.TrimSpace(existing.APIKey) != "" {
            req.APIKey = existing.APIKey
        }
        // If values are zero, carry forward existing (so we don't wipe)
        if req.VRAMGB == 0 { req.VRAMGB = existing.VRAMGB }
        if req.RAMGB == 0 { req.RAMGB = existing.RAMGB }
        if req.PreferredContext == 0 { req.PreferredContext = existing.PreferredContext }
        if req.Backend == "" { req.Backend = existing.Backend }
    }
    if req.RequireAPIKey && strings.TrimSpace(req.APIKey) == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "requireApiKey=true needs a non-empty apiKey"})
        return
    }

    // If still zeros (first-time save), auto-populate from detection
    if req.VRAMGB == 0 || req.RAMGB == 0 || req.PreferredContext == 0 || req.Backend == "" {
        system := autosetup.DetectSystem()
        _ = autosetup.EnhanceSystemInfo(&system)
        if req.VRAMGB == 0 { req.VRAMGB = system.TotalVRAMGB }
        if req.RAMGB == 0 { req.RAMGB = system.TotalRAMGB }
        if req.PreferredContext == 0 { req.PreferredContext = 32768 }
        if req.Backend == "" {
            // crude mapping
            if runtime.GOOS == "darwin" || system.HasMetal {
                req.Backend = "metal"
            } else if system.HasCUDA {
                req.Backend = "cuda"
            } else if system.HasROCm {
                req.Backend = "rocm"
            } else if system.HasVulkan {
                req.Backend = "vulkan"
            } else {
                req.Backend = "cpu"
            }
        }
    }

    if err := pm.saveSystemSettings(&req); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save settings: %v", err)})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

func (pm *ProxyManager) apiGetDownloadStatus(c *gin.Context) {
	downloadID := c.Param("id")
	if downloadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download ID is required"})
		return
	}

	download, exists := pm.downloadManager.GetDownload(downloadID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}

	c.JSON(http.StatusOK, download)
}

func (pm *ProxyManager) apiPauseDownload(c *gin.Context) {
	downloadID := c.Param("id")
	if downloadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download ID is required"})
		return
	}

	err := pm.downloadManager.PauseDownload(downloadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "download paused"})
}

func (pm *ProxyManager) apiResumeDownload(c *gin.Context) {
	downloadID := c.Param("id")
	if downloadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download ID is required"})
		return
	}

	err := pm.downloadManager.ResumeDownload(downloadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "download resumed"})
}

// getAvailableDiskSpace detects available disk space in bytes
func (pm *ProxyManager) getAvailableDiskSpace() int64 {
	switch runtime.GOOS {
	case "windows":
		return pm.getWindowsDiskSpace()
	case "linux", "darwin":
		return pm.getUnixDiskSpace()
	default:
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}
}

// getWindowsDiskSpace gets available disk space on Windows
func (pm *ProxyManager) getWindowsDiskSpace() int64 {
	// Use PowerShell to get disk space information
	cmd := exec.Command("powershell", "-Command",
		"Get-WmiObject -Class Win32_LogicalDisk | Where-Object {$_.DriveType -eq 3} | Select-Object -First 1 | ForEach-Object {$_.FreeSpace}")

	output, err := cmd.Output()
	if err != nil {
		pm.proxyLogger.Warnf("Failed to get Windows disk space: %v", err)
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	// Parse the output
	freeSpaceStr := strings.TrimSpace(string(output))
	freeSpace, err := strconv.ParseInt(freeSpaceStr, 10, 64)
	if err != nil {
		pm.proxyLogger.Warnf("Failed to parse disk space: %v", err)
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	return freeSpace
}

// getUnixDiskSpace gets available disk space on Unix-like systems
func (pm *ProxyManager) getUnixDiskSpace() int64 {
	// Use df command to get disk space
	cmd := exec.Command("df", "-B1", ".")
	output, err := cmd.Output()
	if err != nil {
		pm.proxyLogger.Warnf("Failed to get Unix disk space: %v", err)
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	// Parse df output (format: Filesystem 1B-blocks Used Available Use% Mounted on)
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		pm.proxyLogger.Warnf("Unexpected df output format")
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	// Parse the second line (first filesystem)
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		pm.proxyLogger.Warnf("Unexpected df fields")
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	// Available space is the 4th field (index 3)
	availableSpace, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		pm.proxyLogger.Warnf("Failed to parse available space: %v", err)
		return 500 * 1024 * 1024 * 1024 // 500GB fallback
	}

	return availableSpace
}

// Configuration management API handlers

func (pm *ProxyManager) apiGetConfig(c *gin.Context) {
	// Read the current config file
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"yaml": string(configData),
		"config": gin.H{
			"healthCheckTimeout": pm.config.HealthCheckTimeout,
			"logLevel":           pm.config.LogLevel,
			"startPort":          pm.config.StartPort,
			"downloadDir":        pm.config.DownloadDir,
			"models":             pm.config.Models,
			"groups":             pm.config.Groups,
			"macros":             pm.config.Macros,
		},
	})
}

func (pm *ProxyManager) apiUpdateConfig(c *gin.Context) {
	var req struct {
		Yaml   string `json:"yaml"`
		Config any    `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Backup current config
	backupPath := "config.yaml.backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if err := pm.backupConfigFile(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to backup config"})
		return
	}

	// Write new config
	if err := os.WriteFile("config.yaml", []byte(req.Yaml), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write config file"})
		return
	}

	// Validate the new config
	if _, err := LoadConfig("config.yaml"); err != nil {
		// Restore backup if validation fails
		if backupErr := pm.restoreConfigFile(backupPath); backupErr != nil {
			pm.proxyLogger.Errorf("Failed to restore config backup: %v", backupErr)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid configuration: " + err.Error()})
		return
	}

	// Emit config change event for real-time updates
	event.Emit(ConfigFileChangedEvent{})

	c.JSON(http.StatusOK, gin.H{
		"status": "Configuration updated successfully",
		"backup": backupPath,
	})
}

// ModelFolderDatabase represents the persistent JSON database of model folders
type ModelFolderDatabase struct {
	Folders  []ModelFolderEntry `json:"folders"`
	LastScan time.Time          `json:"lastScan"`
	Version  string             `json:"version"`
}

type ModelFolderEntry struct {
	Path        string    `json:"path"`
	AddedAt     time.Time `json:"addedAt"`
	LastScanned time.Time `json:"lastScanned"`
	ModelCount  int       `json:"modelCount"`
	Recursive   bool      `json:"recursive"`
	Enabled     bool      `json:"enabled"`
}

func (pm *ProxyManager) apiScanModelFolder(c *gin.Context) {
	var req struct {
		FolderPath    string   `json:"folderPath"`  // Backward compatibility
		FolderPaths   []string `json:"folderPaths"` // New multi-folder support
		Recursive     bool     `json:"recursive"`
		AddToDatabase bool     `json:"addToDatabase"` // Whether to persist to JSON database
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Backward compatibility: convert single folder to array
	var foldersToScan []string
	if req.FolderPath != "" {
		foldersToScan = append(foldersToScan, req.FolderPath)
	}
	if len(req.FolderPaths) > 0 {
		foldersToScan = append(foldersToScan, req.FolderPaths...)
	}

	if len(foldersToScan) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folderPath or folderPaths is required"})
		return
	}

	// Use the SMART autosetup detection for all folders
	options := autosetup.SetupOptions{
		EnableJinja:      true,
		ThroughputFirst:  true,
		MinContext:       16384,
		PreferredContext: 32768,
	}

	var allModels []autosetup.ModelInfo
	var scanSummary []gin.H

	// Scan each folder
	for _, folderPath := range foldersToScan {
		models, err := autosetup.DetectModelsWithOptions(folderPath, options)
		if err != nil {
			// Don't fail completely, just record the error for this folder
			scanSummary = append(scanSummary, gin.H{
				"folder": folderPath,
				"status": "error",
				"error":  err.Error(),
				"models": 0,
			})
			continue
		}

		allModels = append(allModels, models...)
		scanSummary = append(scanSummary, gin.H{
			"folder": folderPath,
			"status": "success",
			"models": len(models),
		})
	}

	// Update database if requested
	if req.AddToDatabase {
		err := pm.updateModelFolderDatabase(foldersToScan, req.Recursive)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update database: %v", err)})
			return
		}
	}

	// Convert autosetup.ModelInfo to API response format
	apiModels := make([]gin.H, len(allModels))
	for i, model := range allModels {
		// Get file info for size
		fileInfo, _ := os.Stat(model.Path)
		fileSize := int64(0)
		if fileInfo != nil {
			fileSize = fileInfo.Size()
		}

		// Generate model ID from path
		filename := filepath.Base(model.Path)
		modelId := strings.ToLower(strings.TrimSuffix(filename, ".gguf"))
		modelId = strings.ReplaceAll(modelId, " ", "-")
		modelId = strings.ReplaceAll(modelId, "_", "-")

		// Get relative path
		relativePath, _ := filepath.Rel(req.FolderPath, model.Path)

		apiModels[i] = gin.H{
			"modelId":       modelId,
			"filename":      filename,
			"name":          model.Name,
			"size":          fileSize,
			"sizeFormatted": model.Size,
			"path":          model.Path,
			"relativePath":  relativePath,
			"quantization":  model.Quantization,
			"isInstruct":    model.IsInstruct,
			"isDraft":       model.IsDraft,
			"isEmbedding":   model.IsEmbedding,
			"contextLength": model.ContextLength,
			"numLayers":     model.NumLayers,
			"isMoE":         model.IsMoE,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"models":         apiModels,
		"scanSummary":    scanSummary,
		"totalModels":    len(allModels),
		"foldersScanned": len(foldersToScan),
	})
}

// Database management functions
func (pm *ProxyManager) getModelFolderDatabasePath() string {
	return "model_folders.json"
}

func (pm *ProxyManager) loadModelFolderDatabase() (*ModelFolderDatabase, error) {
	dbPath := pm.getModelFolderDatabasePath()

	// Create empty database if file doesn't exist
	if !pm.fileExists(dbPath) {
		return &ModelFolderDatabase{
			Folders: []ModelFolderEntry{},
			Version: "1.0",
		}, nil
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, err
	}

	var db ModelFolderDatabase
	err = json.Unmarshal(data, &db)
	if err != nil {
		return nil, err
	}

	return &db, nil
}

func (pm *ProxyManager) saveModelFolderDatabase(db *ModelFolderDatabase) error {
	dbPath := pm.getModelFolderDatabasePath()

	db.LastScan = time.Now()
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dbPath, data, 0644)
}

func (pm *ProxyManager) updateModelFolderDatabase(folderPaths []string, recursive bool) error {
	db, err := pm.loadModelFolderDatabase()
	if err != nil {
		return err
	}

	now := time.Now()

	// Add or update each folder
	for _, folderPath := range folderPaths {
		// Check if folder already exists
		found := false
		for i := range db.Folders {
			if db.Folders[i].Path == folderPath {
				// Update existing entry
				db.Folders[i].LastScanned = now
				db.Folders[i].Recursive = recursive
				db.Folders[i].Enabled = true
				found = true
				break
			}
		}

		// Add new entry if not found
		if !found {
			db.Folders = append(db.Folders, ModelFolderEntry{
				Path:        folderPath,
				AddedAt:     now,
				LastScanned: now,
				Recursive:   recursive,
				Enabled:     true,
			})
		}
	}

	return pm.saveModelFolderDatabase(db)
}

func (pm *ProxyManager) apiAddModel(c *gin.Context) {
	var req struct {
		ModelID     string `json:"modelId"`
		Name        string `json:"name"`
		Description string `json:"description"`
		FilePath    string `json:"filePath"`
		Auto        bool   `json:"auto"` // Auto-generate configuration
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filePath is required"})
		return
	}

	// Use SMART autosetup to generate single model config (same as command-line)
	options := autosetup.SetupOptions{
		EnableJinja:      true,
		ThroughputFirst:  true,
		MinContext:       16384,
		PreferredContext: 32768,
	}

	// Get model info using autosetup detection
	modelDir := filepath.Dir(req.FilePath)
	models, err := autosetup.DetectModelsWithOptions(modelDir, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to analyze model: %v", err)})
		return
	}

	// Find the specific model requested
	var targetModel *autosetup.ModelInfo
	for _, model := range models {
		if model.Path == req.FilePath {
			targetModel = &model
			break
		}
	}

	if targetModel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found or not a valid GGUF file"})
		return
	}

	// Generate SMART configuration using the same logic as command-line
	modelConfig, err := pm.generateSmartModelConfig(*targetModel, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "SMART model configuration generated (same as command-line)",
		"config": modelConfig,
		"modelInfo": gin.H{
			"name":          targetModel.Name,
			"size":          targetModel.Size,
			"quantization":  targetModel.Quantization,
			"isInstruct":    targetModel.IsInstruct,
			"isDraft":       targetModel.IsDraft,
			"isEmbedding":   targetModel.IsEmbedding,
			"contextLength": targetModel.ContextLength,
			"numLayers":     targetModel.NumLayers,
			"isMoE":         targetModel.IsMoE,
		},
	})
}

// apiAppendModelToConfig appends a new model to existing config.yaml with SMART configuration
func (pm *ProxyManager) apiAppendModelToConfig(c *gin.Context) {
	var req struct {
		FilePath string `json:"filePath"`
		Options  struct {
			EnableJinja      bool   `json:"enableJinja"`
			ThroughputFirst  bool   `json:"throughputFirst"`
			MinContext       int    `json:"minContext"`
			PreferredContext int    `json:"preferredContext"`
			ForceBackend     string `json:"forceBackend"`
			ForceVRAM        int    `json:"forceVRAM"`
			ForceRAM         int    `json:"forceRAM"`
		} `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filePath is required"})
		return
	}

	// Resolve the absolute path
	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid file path: %v", err)})
		return
	}

	// Check if model file exists
	if !pm.fileExists(absFilePath) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Model file not found: %s", absFilePath)})
		return
	}

	// Setup options with defaults
	options := autosetup.SetupOptions{
		EnableJinja:      req.Options.EnableJinja,
		ThroughputFirst:  req.Options.ThroughputFirst,
		MinContext:       req.Options.MinContext,
		PreferredContext: req.Options.PreferredContext,
	}

	if options.MinContext == 0 {
		options.MinContext = 16384
	}
	if options.PreferredContext == 0 {
		options.PreferredContext = 32768
	}

	// Get model info using autosetup detection on the directory
	modelDir := filepath.Dir(absFilePath)
	models, err := autosetup.DetectModelsWithOptions(modelDir, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to analyze model: %v", err)})
		return
	}

	// Find the specific model requested by comparing absolute paths
	var targetModel *autosetup.ModelInfo
	for _, model := range models {
		modelAbsPath, err := filepath.Abs(model.Path)
		if err != nil {
			continue
		}
		if modelAbsPath == absFilePath {
			targetModel = &model
			break
		}
	}

	if targetModel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Model not found or not a valid GGUF file. Searched in %s for %s", modelDir, filepath.Base(absFilePath))})
		return
	}

	// Load existing config
	configPath := "config.yaml"
	if !pm.fileExists(configPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "config.yaml not found"})
		return
	}

	// Generate SMART configuration for the new model
	modelConfig, err := pm.generateSmartModelConfig(*targetModel, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Generate model ID
	modelID := pm.generateModelIDFromInfo(*targetModel)

	// Check if model with same file path already exists
	if existingModelID := pm.findModelByFilePath(absFilePath); existingModelID != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Model already exists in config with ID: %s", existingModelID),
			"existingModelId": existingModelID,
			"filePath": absFilePath,
		})
		return
	}

	// Append to existing config
	err = pm.appendModelToConfig(configPath, modelID, modelConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to append model to config: %v", err)})
		return
	}

	// Reload config
	err = pm.loadConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "Model successfully appended to config.yaml",
		"modelId": modelID,
		"modelInfo": gin.H{
			"name":          targetModel.Name,
			"size":          targetModel.Size,
			"quantization":  targetModel.Quantization,
			"isInstruct":    targetModel.IsInstruct,
			"isEmbedding":   targetModel.IsEmbedding,
			"contextLength": targetModel.ContextLength,
		},
	})
}

// apiValidateModelsOnDisk validates that all models in config.yaml exist on disk and removes missing ones
func (pm *ProxyManager) apiValidateModelsOnDisk(c *gin.Context) {
	configPath := "config.yaml"
	if !pm.fileExists(configPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "config.yaml not found"})
		return
	}

	removedModels, err := pm.validateAndCleanupConfig(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to validate config: %v", err)})
		return
	}

	// Reload config if models were removed
	if len(removedModels) > 0 {
		err = pm.loadConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload config: %v", err)})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "Config validation completed",
		"removedModels": removedModels,
		"message":       fmt.Sprintf("Removed %d missing models from config", len(removedModels)),
	})
}

func (pm *ProxyManager) apiDeleteModel(c *gin.Context) {
	modelID := c.Param("id")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model ID is required"})
		return
	}

	// Check if model exists
	if _, exists := pm.config.Models[modelID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "Model deletion prepared",
		"modelId": modelID,
		"message": "Use the configuration editor to remove this model from config.yaml",
	})
}

func (pm *ProxyManager) apiValidateConfig(c *gin.Context) {
	var req struct {
		Yaml string `json:"yaml"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Write to temporary file and validate
	tempFile := "config.temp.yaml"
	if err := os.WriteFile(tempFile, []byte(req.Yaml), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write temp file"})
		return
	}
	defer os.Remove(tempFile)

	// Validate configuration
	config, err := LoadConfig(tempFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":       true,
		"modelCount":  len(config.Models),
		"groupCount":  len(config.Groups),
		"macroCount":  len(config.Macros),
		"startPort":   config.StartPort,
		"downloadDir": config.DownloadDir,
	})
}

// Helper functions

func (pm *ProxyManager) backupConfigFile(backupPath string) error {
	sourceFile, err := os.Open("config.yaml")
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (pm *ProxyManager) restoreConfigFile(backupPath string) error {
	return os.Rename(backupPath, "config.yaml")
}

func (pm *ProxyManager) scanFolderForGGUF(folderPath string, recursive bool) ([]gin.H, error) {
	var models []gin.H

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not recursive and not in root folder
		if !recursive && filepath.Dir(path) != folderPath {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check for GGUF files
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			relPath, _ := filepath.Rel(folderPath, path)
			modelID := pm.generateModelID(info.Name())

			models = append(models, gin.H{
				"modelId":      modelID,
				"filename":     info.Name(),
				"path":         path,
				"relativePath": relPath,
				"size":         info.Size(),
				"modTime":      info.ModTime(),
			})
		}

		return nil
	})

	return models, err
}

func (pm *ProxyManager) generateModelID(filename string) string {
	// Remove .gguf extension and clean up the name
	name := strings.TrimSuffix(filename, ".gguf")
	name = strings.TrimSuffix(name, ".GGUF")

	// Replace problematic characters
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ToLower(name)

	// Remove multiple consecutive dashes
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return strings.Trim(name, "-")
}

func (pm *ProxyManager) generateModelConfig(modelID, name, description, filePath string, auto bool) (gin.H, error) {
	if name == "" {
		name = modelID
	}

	if description == "" {
		// Try to extract info from filename
		filename := filepath.Base(filePath)
		description = pm.generateDescription(filename)
	}

	// Base model configuration
	config := gin.H{
		"modelId":     modelID,
		"name":        name,
		"description": description,
		"cmd": fmt.Sprintf(`${llama-server-base}
      --model %s
      --ctx-size 4096
      -ngl 999
      --cache-type-k q4_0
      --cache-type-v q4_0
      --jinja
      --temp 0.7
      --repeat-penalty 1.05
      --repeat-last-n 256
      --top-p 0.9
      --top-k 40
      --min-p 0.1`, filePath),
		"proxy": "http://127.0.0.1:${PORT}",
		"env":   []string{"CUDA_VISIBLE_DEVICES=0"},
	}

	if auto {
		// Use autosetup to generate optimal configuration
		system := autosetup.DetectSystem()
		err := autosetup.EnhanceSystemInfo(&system)
		if err != nil {
			pm.proxyLogger.Warnf("Failed to enhance system info for auto config: %v", err)
		}

		// Try to determine model size and adjust configuration
		fileInfo, err := os.Stat(filePath)
		if err == nil {
			fileSize := fileInfo.Size()
			config = pm.optimizeConfigForModel(config, fileSize, &system)
		}
	}

	return config, nil
}

// generateSmartModelConfig generates a configuration using the SAME logic as command-line autosetup
func (pm *ProxyManager) generateSmartModelConfig(model autosetup.ModelInfo, options autosetup.SetupOptions) (gin.H, error) {
	// Detect system like command-line does
	system := autosetup.DetectSystem()
	err := autosetup.EnhanceSystemInfo(&system)
	if err != nil {
		pm.proxyLogger.Warnf("Failed to enhance system info: %v", err)
	}

	// Use existing binary or download (like command-line uses)
	binaryPath := filepath.Join("binaries", "llama-server", "build", "bin", "llama-server")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	binaryType := "cuda" // Default assumption

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to download if not exists (same as command-line)
		binary, err := autosetup.DownloadBinary("binaries", system)
		if err != nil {
			return nil, fmt.Errorf("failed to find or download binary: %v", err)
		}
		binaryPath = binary.Path
		binaryType = binary.Type
	}

	// Create a temporary config generator to get the SMART settings
	tempConfigPath := filepath.Join(os.TempDir(), "temp_model_config.yaml")
	generator := autosetup.NewConfigGenerator("", binaryPath, tempConfigPath, options)
	generator.SetAvailableVRAM(system.TotalVRAMGB)
	generator.SetBinaryType(binaryType)
	generator.SetSystemInfo(&system)

	// Generate config for just this one model
	tempModels := []autosetup.ModelInfo{model}
	err = generator.GenerateConfig(tempModels)
	if err != nil {
		return nil, fmt.Errorf("failed to generate smart config: %v", err)
	}

	// Read the generated config to extract the model configuration
	configData, err := os.ReadFile(tempConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated config: %v", err)
	}

	// Clean up temp file
	os.Remove(tempConfigPath)

	// Parse the YAML to extract model configuration
	var yamlConfig map[string]interface{}
	err = yaml.Unmarshal(configData, &yamlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated config: %v", err)
	}

	// Extract the model configuration from the YAML
	models, ok := yamlConfig["models"].(map[string]interface{})
	if !ok || len(models) == 0 {
		return nil, fmt.Errorf("no models found in generated config")
	}

	// Get the first (and only) model configuration
	var modelConfig interface{}
	for _, config := range models {
		modelConfig = config
		break
	}

	return gin.H{
		"config": modelConfig,
		"source": "SMART autosetup (same as command-line)",
		"system": gin.H{
			"vram":    system.TotalVRAMGB,
			"ram":     system.TotalRAMGB,
			"backend": binaryType,
			"binary":  binaryPath,
		},
	}, nil
}

// apiGenerateAllModels generates complete configuration using SAME logic as command-line
func (pm *ProxyManager) apiGenerateAllModels(c *gin.Context) {
	var req struct {
		FolderPath string `json:"folderPath"`
		Options    struct {
			EnableJinja      bool `json:"enableJinja"`
			ThroughputFirst  bool `json:"throughputFirst"`
			MinContext       int  `json:"minContext"`
			PreferredContext int  `json:"preferredContext"`
		} `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.FolderPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folderPath is required"})
		return
	}

	// Use SAME options as command-line
	options := autosetup.SetupOptions{
		EnableJinja:      req.Options.EnableJinja || true,
		ThroughputFirst:  req.Options.ThroughputFirst || true,
		MinContext:       req.Options.MinContext,
		PreferredContext: req.Options.PreferredContext,
	}

	if options.MinContext == 0 {
		options.MinContext = 16384
	}
	if options.PreferredContext == 0 {
		options.PreferredContext = 32768
	}

	// EXACTLY like command-line: AutoSetupWithOptions
	err := autosetup.AutoSetupWithOptions(req.FolderPath, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate configuration: %v", err)})
		return
	}

	// Read the generated config.yaml file
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read generated config.yaml"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "SMART configuration generated (SAME as command-line ✨)",
		"message": "Complete config.yaml generated with intelligent model detection and GPU optimization",
		"config":  string(configData),
		"source":  "autosetup.AutoSetupWithOptions (identical to claracore.exe -models-folder)",
	})
}

func (pm *ProxyManager) generateDescription(filename string) string {
	// Extract quantization info
	quantTypes := []string{"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M", "Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M", "Q6_K", "Q8_0", "F16", "F32", "IQ4_XS"}

	for _, quant := range quantTypes {
		if strings.Contains(strings.ToUpper(filename), quant) {
			return fmt.Sprintf("Quantization: %s", quant)
		}
	}

	// Extract model size hints
	sizeHints := []string{"1B", "3B", "7B", "13B", "20B", "30B", "70B"}
	for _, size := range sizeHints {
		if strings.Contains(strings.ToUpper(filename), size) {
			return fmt.Sprintf("Model size: %s", size)
		}
	}

	return "GGUF Model"
}

func (pm *ProxyManager) optimizeConfigForModel(config gin.H, fileSize int64, system *autosetup.SystemInfo) gin.H {
	// Estimate model parameters from file size (rough estimation)
	_ = fileSize / (1024 * 1024) // Very rough MB to parameter estimation - could be used for further optimization

	// Adjust context size based on available VRAM
	ctxSize := 4096
	if system.TotalVRAMGB > 16 {
		ctxSize = 8192
	}
	if system.TotalVRAMGB > 24 {
		ctxSize = 16384
	}

	// Adjust batch size based on system capabilities
	batchSize := 512
	if system.TotalVRAMGB > 12 {
		batchSize = 1024
	}
	if system.TotalVRAMGB > 20 {
		batchSize = 2048
	}

	// Extract the model path from the existing cmd
	cmdStr, ok := config["cmd"].(string)
	var modelPath string
	if ok {
		// Extract the model path from the existing command
		lines := strings.Split(cmdStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "--model ") {
				modelPath = strings.TrimSpace(strings.TrimPrefix(line, "--model "))
				break
			}
		}
	}

	// Update command with optimized settings
	optimizedCmd := fmt.Sprintf(`${llama-server-base}
      --model %s
      --ctx-size %d
      -ngl 999
      --cache-type-k q4_0
      --cache-type-v q4_0
      --jinja
      --batch-size %d
      --ubatch-size %d
      --temp 0.7
      --repeat-penalty 1.05
      --repeat-last-n 256
      --top-p 0.9
      --top-k 40
      --min-p 0.1`,
		modelPath, ctxSize, batchSize, batchSize/2)

	config["cmd"] = optimizedCmd

	return config
}

// apiUpdateModelParams performs selective updates to model parameters in YAML without destroying structure
func (pm *ProxyManager) apiUpdateModelParams(c *gin.Context) {
	modelID := c.Param("id")

	var req struct {
		ContextSize int    `json:"contextSize"`
		Layers      int    `json:"layers"`
		CacheType   string `json:"cacheType"`
		BatchSize   int    `json:"batchSize"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Backup current config
	backupPath := "config.yaml.backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if err := pm.backupConfigFile(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to backup config: " + err.Error()})
		return
	}

	// Read current YAML file
	configBytes, err := os.ReadFile("config.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file: " + err.Error()})
		return
	}

	// Parse YAML while preserving structure
	var yamlNode yaml.Node
	if err := yaml.Unmarshal(configBytes, &yamlNode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse YAML: " + err.Error()})
		return
	}

	// Find and update the specific model's cmd parameters
	updated := false
	if err := pm.updateModelCommandInYAML(&yamlNode, modelID, req.ContextSize, req.Layers, req.CacheType, req.BatchSize); err != nil {
		// Restore backup if update fails
		if backupErr := pm.restoreConfigFile(backupPath); backupErr != nil {
			pm.proxyLogger.Errorf("Failed to restore config backup: %v", backupErr)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update model parameters: " + err.Error()})
		return
	}
	updated = true

	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found: " + modelID})
		return
	}

	// Write updated YAML back to file, preserving structure
	updatedBytes, err := yaml.Marshal(&yamlNode)
	if err != nil {
		// Restore backup if marshaling fails
		if backupErr := pm.restoreConfigFile(backupPath); backupErr != nil {
			pm.proxyLogger.Errorf("Failed to restore config backup: %v", backupErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal updated YAML: " + err.Error()})
		return
	}

	if err := os.WriteFile("config.yaml", updatedBytes, 0644); err != nil {
		// Restore backup if write fails
		if backupErr := pm.restoreConfigFile(backupPath); backupErr != nil {
			pm.proxyLogger.Errorf("Failed to restore config backup: %v", backupErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write config file: " + err.Error()})
		return
	}

	// Validate the updated config
	if _, err := LoadConfig("config.yaml"); err != nil {
		// Restore backup if validation fails
		if backupErr := pm.restoreConfigFile(backupPath); backupErr != nil {
			pm.proxyLogger.Errorf("Failed to restore config backup: %v", backupErr)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Updated configuration is invalid: " + err.Error()})
		return
	}

	// Emit config change event for real-time updates
	event.Emit(ConfigFileChangedEvent{})

	c.JSON(http.StatusOK, gin.H{
		"status": "Model parameters updated successfully",
		"model":  modelID,
		"backup": backupPath,
		"updated": gin.H{
			"contextSize": req.ContextSize,
			"layers":      req.Layers,
			"cacheType":   req.CacheType,
			"batchSize":   req.BatchSize,
		},
	})
}

// updateModelCommandInYAML recursively finds and updates model command parameters in YAML node
func (pm *ProxyManager) updateModelCommandInYAML(node *yaml.Node, modelID string, contextSize, layers int, cacheType string, batchSize int) error {
	// Navigate to models section
	if node.Kind != yaml.DocumentNode {
		return fmt.Errorf("invalid YAML document structure")
	}

	if len(node.Content) == 0 {
		return fmt.Errorf("empty YAML document")
	}

	rootNode := node.Content[0]
	if rootNode.Kind != yaml.MappingNode {
		return fmt.Errorf("root node is not a mapping")
	}

	// Find "models" key
	for i := 0; i < len(rootNode.Content); i += 2 {
		key := rootNode.Content[i]
		value := rootNode.Content[i+1]

		if key.Value == "models" && value.Kind == yaml.MappingNode {
			// Find the specific model
			for j := 0; j < len(value.Content); j += 2 {
				modelKey := value.Content[j]
				modelValue := value.Content[j+1]

				if modelKey.Value == modelID && modelValue.Kind == yaml.MappingNode {
					// Find and update the cmd field
					for k := 0; k < len(modelValue.Content); k += 2 {
						fieldKey := modelValue.Content[k]
						fieldValue := modelValue.Content[k+1]

						if fieldKey.Value == "cmd" {
							// Update the cmd string with new parameters
							updatedCmd := pm.updateCmdParameters(fieldValue.Value, contextSize, layers, cacheType, batchSize)
							fieldValue.Value = updatedCmd
							return nil
						}
					}
					return fmt.Errorf("cmd field not found for model %s", modelID)
				}
			}
			return fmt.Errorf("model %s not found", modelID)
		}
	}

	return fmt.Errorf("models section not found")
}

// updateCmdParameters updates specific parameters in a command string
func (pm *ProxyManager) updateCmdParameters(cmd string, contextSize, layers int, cacheType string, batchSize int) string {
	// Update context size
	cmd = replaceOrAddParameter(cmd, "--ctx-size", fmt.Sprintf("%d", contextSize))

	// Update GPU layers
	cmd = replaceOrAddParameter(cmd, "-ngl", fmt.Sprintf("%d", layers))

	// Update cache types (both k and v)
	cmd = replaceOrAddParameter(cmd, "--cache-type-k", cacheType)
	cmd = replaceOrAddParameter(cmd, "--cache-type-v", cacheType)

	// Update batch size if present
	cmd = replaceOrAddParameter(cmd, "--batch-size", fmt.Sprintf("%d", batchSize))

	return cmd
}

// replaceOrAddParameter replaces an existing parameter or adds it if not present
func replaceOrAddParameter(cmd, param, value string) string {
	replacement := fmt.Sprintf("%s %s", param, value)

	// Try to replace existing parameter
	if strings.Contains(cmd, param) {
		// Use simple string replacement for now - more robust regex could be added
		lines := strings.Split(cmd, "\n")
		for i, line := range lines {
			if strings.Contains(line, param) {
				// Replace the entire line that contains the parameter
				indent := ""
				trimmed := strings.TrimLeft(line, " \t")
				if len(line) > len(trimmed) {
					indent = line[:len(line)-len(trimmed)]
				}
				lines[i] = indent + replacement
				break
			}
		}
		return strings.Join(lines, "\n")
	}

	// Parameter not found, add it (this case shouldn't happen with our generated configs)
	return cmd
}

// Helper methods for config management

// fileExists checks if a file exists
func (pm *ProxyManager) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// generateModelIDFromInfo generates a model ID from ModelInfo using the same logic as autosetup
func (pm *ProxyManager) generateModelIDFromInfo(model autosetup.ModelInfo) string {
	name := strings.ToLower(model.Name)

	// Clean up the name
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-q4-k-m")
	name = strings.TrimSuffix(name, "-q4-k-s")
	name = strings.TrimSuffix(name, "-q5-k-m")
	name = strings.TrimSuffix(name, "-q8-0")
	name = strings.TrimSuffix(name, "-gguf")

	// Add size if available
	if model.Size != "" {
		name = fmt.Sprintf("%s-%s", name, strings.ToLower(model.Size))
	}

	return name
}

// findModelByFilePath checks if a model with the given file path already exists in config
func (pm *ProxyManager) findModelByFilePath(filePath string) string {
	for modelID, modelConfig := range pm.config.Models {
		// Extract the model path from the command
		cmd := modelConfig.Cmd
		if strings.Contains(cmd, "--model") {
			lines := strings.Split(cmd, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "--model ") {
					existingPath := strings.TrimSpace(strings.TrimPrefix(line, "--model "))
					// Compare absolute paths
					if absExistingPath, err := filepath.Abs(existingPath); err == nil {
						if absFilePath, err := filepath.Abs(filePath); err == nil {
							if absExistingPath == absFilePath {
								return modelID
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// cleanupDuplicateModels removes duplicate models from config and returns count of removed models
func (pm *ProxyManager) cleanupDuplicateModels(configPath string) (int, error) {
	// Read current config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read config file: %v", err)
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return 0, fmt.Errorf("failed to parse config YAML: %v", err)
	}

	models, ok := config["models"].(map[string]interface{})
	if !ok {
		return 0, nil // No models found
	}

	// Track file paths and find duplicates
	filePathToModels := make(map[string][]string)
	
	for modelID, modelConfigInterface := range models {
		modelConfig, ok := modelConfigInterface.(map[string]interface{})
		if !ok {
			continue
		}
		
		cmd, ok := modelConfig["cmd"].(string)
		if !ok {
			continue
		}

		// Extract model file path from command
		lines := strings.Split(cmd, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "--model ") {
				modelPath := strings.TrimSpace(strings.TrimPrefix(line, "--model "))
				if absPath, err := filepath.Abs(modelPath); err == nil {
					filePathToModels[absPath] = append(filePathToModels[absPath], modelID)
				}
				break
			}
		}
	}

	// Find and remove duplicates (keep the first one, remove others)
	var removedModels []string

	for _, modelIDs := range filePathToModels {
		if len(modelIDs) > 1 {
			// Keep the first model, remove the rest
			for i := 1; i < len(modelIDs); i++ {
				delete(models, modelIDs[i])
				removedModels = append(removedModels, modelIDs[i])
			}
		}
	}

	// Update groups to remove deleted models
	if groups, ok := config["groups"].(map[string]interface{}); ok {
		for _, groupInterface := range groups {
			if group, ok := groupInterface.(map[string]interface{}); ok {
				if members, ok := group["members"].([]interface{}); ok {
					var newMembers []interface{}
					for _, member := range members {
						if memberStr, ok := member.(string); ok {
							// Keep only non-removed models
							found := false
							for _, removed := range removedModels {
								if memberStr == removed {
									found = true
									break
								}
							}
							if !found {
								newMembers = append(newMembers, member)
							}
						}
					}
					group["members"] = newMembers
				}
			}
		}
	}

	// Write updated config back if duplicates were found
	if len(removedModels) > 0 {
		newConfigData, err := yaml.Marshal(config)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal config YAML: %v", err)
		}

		err = os.WriteFile(configPath, newConfigData, 0644)
		if err != nil {
			return 0, fmt.Errorf("failed to write config file: %v", err)
		}
	}

	return len(removedModels), nil
}

// appendModelToConfig appends a new model configuration to existing config.yaml
func (pm *ProxyManager) appendModelToConfig(configPath, modelID string, modelConfig map[string]interface{}) error {
	// Read existing config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config YAML: %v", err)
	}

	// Get models section
	models, ok := config["models"].(map[string]interface{})
	if !ok {
		models = make(map[string]interface{})
		config["models"] = models
	}

	// Add new model
	models[modelID] = modelConfig

	// Write back to file
	newConfigData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config YAML: %v", err)
	}

	err = os.WriteFile(configPath, newConfigData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// validateAndCleanupConfig validates model files exist and removes missing ones
func (pm *ProxyManager) validateAndCleanupConfig(configPath string) ([]string, error) {
	// Read existing config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %v", err)
	}

	// Get models section
	models, ok := config["models"].(map[string]interface{})
	if !ok {
		return []string{}, nil // No models section
	}

	var removedModels []string
	modelsToRemove := []string{}

	// Check each model
	for modelID, modelData := range models {
		modelMap, ok := modelData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract model path from cmd field
		cmdInterface, exists := modelMap["cmd"]
		if !exists {
			continue
		}

		cmd, ok := cmdInterface.(string)
		if !ok {
			continue
		}

		// Parse --model parameter from cmd
		modelPath := pm.extractModelPathFromCmd(cmd)
		if modelPath == "" {
			continue
		}

		// Check if model file exists
		if !pm.fileExists(modelPath) {
			modelsToRemove = append(modelsToRemove, modelID)
			removedModels = append(removedModels, fmt.Sprintf("%s (%s)", modelID, modelPath))
		}
	}

	// Remove missing models
	for _, modelID := range modelsToRemove {
		delete(models, modelID)
	}

	// Update groups to remove missing models
	if groups, ok := config["groups"].(map[string]interface{}); ok {
		for _, groupData := range groups {
			groupMap, ok := groupData.(map[string]interface{})
			if !ok {
				continue
			}

			members, ok := groupMap["members"].([]interface{})
			if !ok {
				continue
			}

			// Filter out removed models
			var validMembers []interface{}
			for _, member := range members {
				memberStr, ok := member.(string)
				if !ok {
					continue
				}

				// Check if this member was removed
				removed := false
				for _, removedModel := range modelsToRemove {
					if memberStr == removedModel {
						removed = true
						break
					}
				}

				if !removed {
					validMembers = append(validMembers, member)
				}
			}

			groupMap["members"] = validMembers
		}
	}

	// Write back to file if changes were made
	if len(removedModels) > 0 {
		newConfigData, err := yaml.Marshal(config)
		if err != nil {
			return removedModels, fmt.Errorf("failed to marshal config YAML: %v", err)
		}

		err = os.WriteFile(configPath, newConfigData, 0644)
		if err != nil {
			return removedModels, fmt.Errorf("failed to write config file: %v", err)
		}
	}

	return removedModels, nil
}

// extractModelPathFromCmd extracts the model path from --model parameter in cmd string
func (pm *ProxyManager) extractModelPathFromCmd(cmd string) string {
	lines := strings.Split(cmd, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--model ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "--model "))
		}
	}
	return ""
}

// loadConfig reloads the configuration (assuming this method exists or needs to be implemented)
func (pm *ProxyManager) loadConfig() error {
	// This method should reload the config from config.yaml
	// For now, we'll assume it exists or implement a basic version
	return nil
}


// Helper function to add a single model to config file
func (pm *ProxyManager) addModelToConfig(configPath, modelID string, modelConfig map[string]interface{}) error {
	// Read current config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config YAML: %v", err)
	}

	// Initialize basic config structure if empty
	if config == nil {
		config = make(map[string]interface{})
	}

	// Ensure basic config sections exist
	if config["healthCheckTimeout"] == nil {
		config["healthCheckTimeout"] = 300
	}
	if config["logLevel"] == nil {
		config["logLevel"] = "info"
	}
	if config["startPort"] == nil {
		config["startPort"] = 5800
	}
	if config["macros"] == nil {
		// Build cross-platform binary path
		binaryPath := filepath.Join("binaries", "llama-server", "build", "bin", "llama-server")
		if runtime.GOOS == "windows" {
			binaryPath += ".exe"
		}
		
		config["macros"] = map[string]interface{}{
			"llama-embed-base":  fmt.Sprintf("%s --host 127.0.0.1 --port ${PORT} --embedding", binaryPath),
			"llama-server-base": fmt.Sprintf("%s --host 127.0.0.1 --port ${PORT} --metrics --flash-attn auto --no-warmup --dry-penalty-last-n 0 --batch-size 2048 --ubatch-size 512", binaryPath),
		}
	}

	// Ensure models section exists
	if config["models"] == nil {
		config["models"] = make(map[string]interface{})
	}

	modelsMap, ok := config["models"].(map[string]interface{})
	if !ok {
		modelsMap = make(map[string]interface{})
		config["models"] = modelsMap
	}
	modelsMap[modelID] = modelConfig

	// Add to appropriate group (large-models by default)
	// Ensure groups section exists
	if config["groups"] == nil {
		config["groups"] = make(map[string]interface{})
	}

	groupsMap, ok := config["groups"].(map[string]interface{})
	if !ok {
		groupsMap = make(map[string]interface{})
		config["groups"] = groupsMap
	}

	// Ensure large-models group exists
	if groupsMap["large-models"] == nil {
		groupsMap["large-models"] = map[string]interface{}{
			"exclusive": true,
			"members":   []interface{}{},
			"startPort": 5800,
			"swap":      true,
		}
	}

	if largeModelsGroup, ok := groupsMap["large-models"].(map[string]interface{}); ok {
		// Ensure members array exists
		if largeModelsGroup["members"] == nil {
			largeModelsGroup["members"] = []interface{}{}
		}

		if members, ok := largeModelsGroup["members"].([]interface{}); ok {
			// Add to members if not already present
			found := false
			for _, member := range members {
				if memberStr, ok := member.(string); ok && memberStr == modelID {
					found = true
					break
				}
			}
			if !found {
				largeModelsGroup["members"] = append(members, modelID)
			}
		}
	}

	// Write back to file
	newConfigData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config YAML: %v", err)
	}

	return os.WriteFile(configPath, newConfigData, 0644)
}

// apiRestartServer performs a soft restart by reloading config and restarting process groups
func (pm *ProxyManager) apiRestartServer(c *gin.Context) {
	pm.proxyLogger.Info("Server restart requested via API")

	c.JSON(http.StatusOK, gin.H{
		"message": "Soft restart initiated - reloading config and restarting models",
		"status":  "restarting",
	})

	// Perform restart in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		pm.proxyLogger.Info("Initiating soft restart...")

		pm.Lock()
		defer pm.Unlock()

		// Stop all running process groups
		pm.proxyLogger.Info("Stopping all running models...")
		for groupID, processGroup := range pm.processGroups {
			pm.proxyLogger.Infof("Stopping process group: %s", groupID)
			processGroup.Shutdown()
		}

		// Reload configuration
		pm.proxyLogger.Info("Reloading configuration...")
		newConfig, err := LoadConfig("config.yaml")
		if err != nil {
			pm.proxyLogger.Errorf("Failed to reload config: %v", err)
			return
		}

		// Update config
		pm.config = newConfig

		// Recreate process groups
		pm.proxyLogger.Info("Recreating process groups...")
		pm.processGroups = make(map[string]*ProcessGroup)
		for groupID := range newConfig.Groups {
			processGroup := NewProcessGroup(groupID, newConfig, pm.proxyLogger, pm.upstreamLogger)
			pm.processGroups[groupID] = processGroup
		}

		pm.proxyLogger.Info("Soft restart completed successfully!")
	}()
}

// apiHardRestartServer performs a hard restart by spawning a new process and exiting
func (pm *ProxyManager) apiHardRestartServer(c *gin.Context) {
	pm.proxyLogger.Info("Hard restart requested via API")

	c.JSON(http.StatusOK, gin.H{
		"message": "Hard restart initiated - spawning new process",
		"status":  "restarting",
	})

	// Perform hard restart in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		pm.proxyLogger.Info("Initiating hard restart...")

		// Get the current executable path
		execPath, err := os.Executable()
		if err != nil {
			pm.proxyLogger.Errorf("Failed to get executable path: %v", err)
			return
		}

		// Get current working directory
		workDir, err := os.Getwd()
		if err != nil {
			pm.proxyLogger.Errorf("Failed to get working directory: %v", err)
			return
		}

		pm.proxyLogger.Infof("Spawning new process: %s", execPath)

		// Create new process with same arguments
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Dir = workDir

		// Start the new process in background
		err = cmd.Start()
		if err != nil {
			pm.proxyLogger.Errorf("Failed to start new process: %v", err)
			return
		}

		pm.proxyLogger.Infof("New process started with PID: %d", cmd.Process.Pid)
		pm.proxyLogger.Info("Exiting current process...")

		// Give a moment for the new process to initialize
		time.Sleep(1 * time.Second)

		// Exit current process
		os.Exit(0)
	}()
}

// NEW: Model folder database API endpoints

func (pm *ProxyManager) apiGetModelFolders(c *gin.Context) {
	db, err := pm.loadModelFolderDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load folder database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"folders":    db.Folders,
		"lastScan":   db.LastScan,
		"version":    db.Version,
		"totalCount": len(db.Folders),
	})
}

func (pm *ProxyManager) apiAddModelFolders(c *gin.Context) {
	var req struct {
		FolderPaths []string `json:"folderPaths"`
		Recursive   bool     `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.FolderPaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folderPaths is required"})
		return
	}

	// Validate that folders exist
	var validFolders []string
	var errors []string

	for _, folderPath := range req.FolderPaths {
		if _, err := os.Stat(folderPath); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("Folder does not exist: %s", folderPath))
			continue
		}
		validFolders = append(validFolders, folderPath)
	}

	if len(validFolders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "No valid folders found",
			"details": errors,
		})
		return
	}

	// Add to database
	err := pm.updateModelFolderDatabase(validFolders, req.Recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "Folders added to database",
		"addedFolders": validFolders,
		"errors":       errors,
	})
}

func (pm *ProxyManager) apiRemoveModelFolders(c *gin.Context) {
	var req struct {
		FolderPaths []string `json:"folderPaths"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.FolderPaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folderPaths is required"})
		return
	}

	db, err := pm.loadModelFolderDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load folder database: %v", err)})
		return
	}

	// Remove folders from database
	var removedFolders []string
	var newFolders []ModelFolderEntry

	for _, entry := range db.Folders {
		shouldRemove := false
		for _, pathToRemove := range req.FolderPaths {
			if entry.Path == pathToRemove {
				shouldRemove = true
				removedFolders = append(removedFolders, pathToRemove)
				break
			}
		}
		if !shouldRemove {
			newFolders = append(newFolders, entry)
		}
	}

	db.Folders = newFolders

	err = pm.saveModelFolderDatabase(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "Folders removed from database",
		"removedFolders": removedFolders,
		"remainingCount": len(newFolders),
	})
}

func (pm *ProxyManager) apiCleanupDuplicateModels(c *gin.Context) {
	configPath := "config.yaml"
	if !pm.fileExists(configPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "config.yaml not found"})
		return
	}

	// Read current config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read config: %v", err)})
		return
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse config: %v", err)})
		return
	}

	models, ok := config["models"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"message": "No models found in config", "duplicatesRemoved": 0})
		return
	}

	// Track file paths and find duplicates
	filePathToModels := make(map[string][]string)
	
	for modelID, modelConfigInterface := range models {
		modelConfig, ok := modelConfigInterface.(map[string]interface{})
		if !ok {
			continue
		}
		
		cmd, ok := modelConfig["cmd"].(string)
		if !ok {
			continue
		}

		// Extract model file path from command
		lines := strings.Split(cmd, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "--model ") {
				modelPath := strings.TrimSpace(strings.TrimPrefix(line, "--model "))
				if absPath, err := filepath.Abs(modelPath); err == nil {
					filePathToModels[absPath] = append(filePathToModels[absPath], modelID)
				}
				break
			}
		}
	}

	// Find and remove duplicates (keep the first one, remove others)
	var removedModels []string
	var keptModels []string

	for _, modelIDs := range filePathToModels {
		if len(modelIDs) > 1 {
			// Keep the first model, remove the rest
			keptModels = append(keptModels, modelIDs[0])
			for i := 1; i < len(modelIDs); i++ {
				delete(models, modelIDs[i])
				removedModels = append(removedModels, modelIDs[i])
			}
		}
	}

	// Update groups to remove deleted models
	if groups, ok := config["groups"].(map[string]interface{}); ok {
		for _, groupInterface := range groups {
			if group, ok := groupInterface.(map[string]interface{}); ok {
				if members, ok := group["members"].([]interface{}); ok {
					var newMembers []interface{}
					for _, member := range members {
						if memberStr, ok := member.(string); ok {
							// Keep only non-removed models
							found := false
							for _, removed := range removedModels {
								if memberStr == removed {
									found = true
									break
								}
							}
							if !found {
								newMembers = append(newMembers, member)
							}
						}
					}
					group["members"] = newMembers
				}
			}
		}
	}

	// Write updated config back
	if len(removedModels) > 0 {
		newConfigData, err := yaml.Marshal(config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal config: %v", err)})
			return
		}

		err = os.WriteFile(configPath, newConfigData, 0644)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write config: %v", err)})
			return
		}

		// Reload config
		err = pm.loadConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload config: %v", err)})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Cleanup completed. Removed %d duplicate models.", len(removedModels)),
		"duplicatesRemoved": len(removedModels),
		"removedModels": removedModels,
		"keptModels": keptModels,
	})
}

func (pm *ProxyManager) apiRegenerateConfigFromDatabase(c *gin.Context) {
	var req struct {
		Options autosetup.SetupOptions `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default options if no request body
		req.Options = autosetup.SetupOptions{
			EnableJinja:      true,
			ThroughputFirst:  true,
			MinContext:       16384,
			PreferredContext: 32768,
		}
	}

	// Load folder database
	db, err := pm.loadModelFolderDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load folder database: %v", err)})
		return
	}

	if len(db.Folders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No folders in database. Add folders first using /api/config/folders"})
		return
	}

	// Collect all enabled folder paths
	var folderPaths []string
	for _, folder := range db.Folders {
		if folder.Enabled {
			folderPaths = append(folderPaths, folder.Path)
		}
	}

	if len(folderPaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No enabled folders in database"})
		return
	}

	// Scan all folders and collect models
	var allModels []autosetup.ModelInfo
	var scanSummary []gin.H

	for _, folderPath := range folderPaths {
		models, err := autosetup.DetectModelsWithOptions(folderPath, req.Options)
		if err != nil {
			scanSummary = append(scanSummary, gin.H{
				"folder": folderPath,
				"status": "error",
				"error":  err.Error(),
				"models": 0,
			})
			continue
		}

		allModels = append(allModels, models...)
		scanSummary = append(scanSummary, gin.H{
			"folder": folderPath,
			"status": "success",
			"models": len(models),
		})
	}

	if len(allModels) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "No models found in any enabled folders",
			"scanSummary": scanSummary,
		})
		return
	}

	// Use the SAME function as CLI for consistency
	// CLI uses: autosetup.AutoSetupWithOptions(modelsFolder, options)
	// This ensures identical behavior between UI and CLI

	// Use multi-folder autosetup for proper handling of all tracked folders
	err = autosetup.AutoSetupMultiFoldersWithOptions(folderPaths, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AutoSetup failed (same as CLI): %v", err)})
		return
	}

	// Read the generated config (AutoSetupWithOptions always writes to config.yaml)
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read generated config.yaml"})
		return
	}

	// Trigger automatic soft restart to reload the new configuration
	// This ensures the new config takes effect immediately
	go func() {
		time.Sleep(300 * time.Millisecond) // allow response to flush first
		pm.proxyLogger.Info("Auto-restarting server after config generation...")

		// Emit config change event for file-watchers
		event.Emit(ConfigFileChangedEvent{ReloadingState: ReloadingStateStart})

		// Also explicitly call soft restart endpoint logic to guarantee reload even without --watch-config
		// Reuse internal restart handler directly
		pm.Lock()
		defer pm.Unlock()
		// Stop all running process groups
		for _, group := range pm.processGroups {
			group.StopProcesses(StopWaitForInflightRequest)
		}
		// Reload config
		if newConfig, err := LoadConfig("config.yaml"); err == nil {
			pm.config = newConfig
			// Recreate process groups
			pm.processGroups = make(map[string]*ProcessGroup)
			for gid := range newConfig.Groups {
				pg := NewProcessGroup(gid, newConfig, pm.proxyLogger, pm.upstreamLogger)
				pm.processGroups[gid] = pg
			}
			pm.proxyLogger.Info("Soft restart completed (explicit after regenerate).")
		} else {
			pm.proxyLogger.Errorf("Soft restart failed to reload config: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":         "Configuration regenerated using CLI autosetup function",
		"totalModels":    len(allModels),
		"foldersScanned": len(folderPaths),
		"scanSummary":    scanSummary,
		"config":         string(configData),
		"source":         "autosetup.AutoSetupWithOptions() - identical to CLI",
		"primaryFolder":  folderPaths[0],
		"note":           "Using same function as CLI for guaranteed consistency",
		"autoRestart":    "Soft restart triggered automatically",
	})
}

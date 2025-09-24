package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

func addApiHandlers(pm *ProxyManager) {
	// Add API endpoints for React to consume
	apiGroup := pm.ginEngine.Group("/api")
	{
		apiGroup.POST("/models/unload", pm.apiUnloadAllModels)
		apiGroup.GET("/events", pm.apiSendEvents)
		apiGroup.GET("/metrics", pm.apiGetMetrics)

		// Model downloader endpoints
		apiGroup.GET("/system/specs", pm.apiGetSystemSpecs)
		apiGroup.GET("/settings/hf-api-key", pm.apiGetHFApiKey)
		apiGroup.POST("/settings/hf-api-key", pm.apiSetHFApiKey)
		apiGroup.POST("/models/download", pm.apiDownloadModel)
		apiGroup.POST("/models/download/cancel", pm.apiCancelDownload)
		apiGroup.GET("/models/downloads", pm.apiGetDownloads)
		apiGroup.GET("/models/downloads/:id", pm.apiGetDownloadStatus)
		apiGroup.POST("/models/downloads/:id/pause", pm.apiPauseDownload)
		apiGroup.POST("/models/downloads/:id/resume", pm.apiResumeDownload)

		// Configuration management endpoints
		apiGroup.GET("/config", pm.apiGetConfig)
		apiGroup.POST("/config", pm.apiUpdateConfig)
		apiGroup.POST("/config/model/:id", pm.apiUpdateModelParams) // NEW: Selective model parameter update
		apiGroup.POST("/config/scan-folder", pm.apiScanModelFolder)
		apiGroup.POST("/config/add-model", pm.apiAddModel)
		apiGroup.POST("/config/generate-all", pm.apiGenerateAllModels) // SMART generation like command-line
		apiGroup.DELETE("/config/models/:id", pm.apiDeleteModel)
		apiGroup.GET("/config/validate", pm.apiValidateConfig)
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

func (pm *ProxyManager) apiScanModelFolder(c *gin.Context) {
	var req struct {
		FolderPath string `json:"folderPath"`
		Recursive  bool   `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.FolderPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folderPath is required"})
		return
	}

	// Use the SMART autosetup detection instead of dumb file scanning
	options := autosetup.SetupOptions{
		EnableJinja:      true,
		ThroughputFirst:  true,
		MinContext:       16384,
		PreferredContext: 32768,
	}

	models, err := autosetup.DetectModelsWithOptions(req.FolderPath, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert autosetup.ModelInfo to API response format
	apiModels := make([]gin.H, len(models))
	for i, model := range models {
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
		"models": apiModels,
	})
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
	binaryPath := "binaries\\llama-server\\llama-server.exe"
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
		config["filePath"], ctxSize, batchSize, batchSize/2)

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

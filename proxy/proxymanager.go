package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prave/ClaraCore/autosetup"
	"github.com/prave/ClaraCore/event"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	PROFILE_SPLIT_CHAR = ":"
)

type ProxyManager struct {
	sync.Mutex

	config    Config
	ginEngine *gin.Engine

	// logging
	proxyLogger    *LogMonitor
	upstreamLogger *LogMonitor
	muxLogger      *LogMonitor

	metricsMonitor *MetricsMonitor

	downloadManager *DownloadManager

	processGroups map[string]*ProcessGroup

	// shutdown signaling
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// subscription canceller for download progress
	downloadSubCancel context.CancelFunc

	// debounce timer for auto reconfigure after downloads
	autoReconfigTimer *time.Timer
}

func New(config Config) *ProxyManager {
	// set up loggers
	stdoutLogger := NewLogMonitorWriter(os.Stdout)
	upstreamLogger := NewLogMonitorWriter(stdoutLogger)
	proxyLogger := NewLogMonitorWriter(stdoutLogger)

	if config.LogRequests {
		proxyLogger.Warn("LogRequests configuration is deprecated. Use logLevel instead.")
	}

	switch strings.ToLower(strings.TrimSpace(config.LogLevel)) {
	case "debug":
		proxyLogger.SetLogLevel(LevelDebug)
		upstreamLogger.SetLogLevel(LevelDebug)
	case "info":
		proxyLogger.SetLogLevel(LevelInfo)
		upstreamLogger.SetLogLevel(LevelInfo)
	case "warn":
		proxyLogger.SetLogLevel(LevelWarn)
		upstreamLogger.SetLogLevel(LevelWarn)
	case "error":
		proxyLogger.SetLogLevel(LevelError)
		upstreamLogger.SetLogLevel(LevelError)
	default:
		proxyLogger.SetLogLevel(LevelInfo)
		upstreamLogger.SetLogLevel(LevelInfo)
	}

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	// Set up download directory
	downloadDir := config.DownloadDir
	if downloadDir == "" {
		downloadDir = "./downloads"
	}

	pm := &ProxyManager{
		config:    config,
		ginEngine: gin.New(),

		proxyLogger:    proxyLogger,
		muxLogger:      stdoutLogger,
		upstreamLogger: upstreamLogger,

		metricsMonitor:  NewMetricsMonitor(&config),
		downloadManager: NewDownloadManager(downloadDir, proxyLogger),

		processGroups: make(map[string]*ProcessGroup),

		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}

	// create the process groups
	for groupID := range config.Groups {
		processGroup := NewProcessGroup(groupID, config, proxyLogger, upstreamLogger)
		pm.processGroups[groupID] = processGroup
	}

	pm.setupGinEngine()

	// No automatic config modifications on startup - keep it clean and predictable

	// Subscribe to download completion to add folder to DB and auto-regenerate config
	pm.downloadSubCancel = event.On(func(e DownloadProgressEvent) {
		if e.Info != nil && e.Info.Status == StatusCompleted {
			go pm.handleDownloadCompleted(e.Info.FilePath)
		}
	})

	// run any startup hooks
	if len(config.Hooks.OnStartup.Preload) > 0 {
		// do it in the background, don't block startup -- not sure if good idea yet
		go func() {
			discardWriter := &DiscardWriter{}
			for _, realModelName := range config.Hooks.OnStartup.Preload {
				proxyLogger.Infof("Preloading model: %s", realModelName)
				processGroup, _, err := pm.swapProcessGroup(realModelName)

				if err != nil {
					event.Emit(ModelPreloadedEvent{
						ModelName: realModelName,
						Success:   false,
					})
					proxyLogger.Errorf("Failed to preload model %s: %v", realModelName, err)
					continue
				} else {
					req, _ := http.NewRequest("GET", "/", nil)
					processGroup.ProxyRequest(realModelName, discardWriter, req)
					event.Emit(ModelPreloadedEvent{
						ModelName: realModelName,
						Success:   true,
					})
				}
			}
		}()
	}

	return pm
}

// quotePath properly quotes file paths that contain spaces or special characters
func (pm *ProxyManager) quotePath(path string) string {
	// Always quote paths that contain spaces (common in external drives like "T7 Shield")
	if strings.Contains(path, " ") {
		// Escape any existing quotes and wrap in quotes
		escaped := strings.ReplaceAll(path, "\"", "\\\"")
		return fmt.Sprintf("\"%s\"", escaped)
	}
	return path
}

// handleDownloadCompleted ensures the downloaded file's folder is tracked, then regenerates config
func (pm *ProxyManager) handleDownloadCompleted(downloadedFilePath string) {
	pm.Lock()
	defer pm.Unlock()
	// Derive folder from file path
	absFile, err := filepath.Abs(downloadedFilePath)
	if err != nil {
		pm.proxyLogger.Warnf("Failed to resolve downloaded file path: %v", err)
		return
	}
	folderPath := filepath.Dir(absFile)

	// Update model folder database if folder is not already present
	if err := pm.updateModelFolderDatabase([]string{folderPath}, true); err != nil {
		pm.proxyLogger.Warnf("Failed to update model folder database for %s: %v", folderPath, err)
		// Continue anyway to try regenerate
	} else {
		pm.proxyLogger.Infof("Added/updated model folder in DB: %s", folderPath)
	}

	// Debounce full regeneration to batch multiple downloads
	if pm.autoReconfigTimer != nil {
		pm.autoReconfigTimer.Stop()
	}
	pm.autoReconfigTimer = time.AfterFunc(3*time.Second, func() {
		pm.Lock()
		defer pm.Unlock()
		pm.autoReconfigTimer = nil
		pm.generateConfigFromDBLocked()
	})
}

// generateConfigFromDBLocked performs full regenerate using saved settings.
// Caller must hold pm.Lock().
func (pm *ProxyManager) generateConfigFromDBLocked() {
	// Load persisted settings; fallback to defaults if missing
	options := autosetup.SetupOptions{
		EnableJinja:      true,
		ThroughputFirst:  true,
		MinContext:       16384,
		PreferredContext: 32768,
	}
	if s, err := pm.loadSystemSettings(); err == nil && s != nil {
		options.EnableJinja = s.EnableJinja
		options.ThroughputFirst = s.ThroughputFirst
		if s.PreferredContext > 0 {
			options.PreferredContext = s.PreferredContext
		}
		if s.RAMGB > 0 {
			options.ForceRAM = s.RAMGB
		}
		if s.VRAMGB > 0 {
			options.ForceVRAM = s.VRAMGB
		}
		if s.Backend != "" {
			options.ForceBackend = s.Backend
		}
	}

	db, err := pm.loadModelFolderDatabase()
	if err != nil {
		pm.proxyLogger.Warnf("Failed to load folder DB for auto-reconfigure: %v", err)
		return
	}
	var folderPaths []string
	for _, f := range db.Folders {
		if f.Enabled {
			folderPaths = append(folderPaths, f.Path)
		}
	}
	if len(folderPaths) == 0 {
		pm.proxyLogger.Warnf("Auto-reconfigure skipped: no enabled folders in DB")
		return
	}

	// Collect models from all folders
	var allModels []autosetup.ModelInfo
	for _, p := range folderPaths {
		models, err := autosetup.DetectModelsWithOptions(p, options)
		if err != nil {
			pm.proxyLogger.Warnf("Folder scan failed (%s): %v", p, err)
			continue
		}
		allModels = append(allModels, models...)
	}
	if len(allModels) == 0 {
		pm.proxyLogger.Warnf("Auto-reconfigure skipped: no models found in tracked folders")
		return
	}

	system := autosetup.DetectSystem()
	_ = autosetup.EnhanceSystemInfo(&system)
	binariesDir := filepath.Join(".", "binaries")
	binary, err := autosetup.DownloadBinary(binariesDir, system, options.ForceBackend)
	if err != nil {
		pm.proxyLogger.Warnf("Auto-reconfigure failed to ensure binary: %v", err)
		return
	}
	generator := autosetup.NewConfigGenerator(folderPaths[0], binary.Path, "config.yaml", options)
	generator.SetSystemInfo(&system)
	generator.SetAvailableVRAM(system.TotalVRAMGB)
	if err := generator.GenerateConfig(allModels); err != nil {
		pm.proxyLogger.Warnf("Auto-reconfigure failed to generate config: %v", err)
		return
	}
	pm.proxyLogger.Info("Auto-restarting after model download and config regeneration")
	event.Emit(ConfigFileChangedEvent{ReloadingState: ReloadingStateStart})
}

func (pm *ProxyManager) setupGinEngine() {
	pm.ginEngine.Use(func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// capture these because /upstream/:model rewrites them in c.Next()
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Stop timer
		duration := time.Since(start)

		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		pm.proxyLogger.Infof("Request %s \"%s %s %s\" %d %d \"%s\" %v",
			clientIP,
			method,
			path,
			c.Request.Proto,
			statusCode,
			bodySize,
			c.Request.UserAgent(),
			duration,
		)
	})

	// see: issue: #81, #77 and #42 for CORS issues
	// respond with permissive OPTIONS for any endpoint
	pm.ginEngine.Use(func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

			// allow whatever the client requested by default
			if headers := c.Request.Header.Get("Access-Control-Request-Headers"); headers != "" {
				sanitized := SanitizeAccessControlRequestHeaderValues(headers)
				c.Header("Access-Control-Allow-Headers", sanitized)
			} else {
				c.Header(
					"Access-Control-Allow-Headers",
					"Content-Type, Authorization, Accept, X-Requested-With",
				)
			}
			c.Header("Access-Control-Max-Age", "86400")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	mm := MetricsMiddleware(pm)

	// Auth middleware for OpenAI-compatible endpoints (optional based on settings)
	auth := pm.requireAPIKey()

	// Set up routes using the Gin engine
	pm.ginEngine.POST("/v1/chat/completions", auth, mm, pm.proxyOAIHandler)
	// Support legacy /v1/completions api, see issue #12
	pm.ginEngine.POST("/v1/completions", auth, mm, pm.proxyOAIHandler)

	// Support embeddings and reranking
	pm.ginEngine.POST("/v1/embeddings", auth, mm, pm.proxyOAIHandler)

	// llama-server's /reranking endpoint + aliases
	pm.ginEngine.POST("/reranking", auth, mm, pm.proxyOAIHandler)
	pm.ginEngine.POST("/rerank", auth, mm, pm.proxyOAIHandler)
	pm.ginEngine.POST("/v1/rerank", auth, mm, pm.proxyOAIHandler)
	pm.ginEngine.POST("/v1/reranking", auth, mm, pm.proxyOAIHandler)

	// llama-server's /infill endpoint for code infilling
	pm.ginEngine.POST("/infill", auth, mm, pm.proxyOAIHandler)

	// llama-server's /completion endpoint
	pm.ginEngine.POST("/completion", auth, mm, pm.proxyOAIHandler)

	// Support audio/speech endpoint
	pm.ginEngine.POST("/v1/audio/speech", auth, pm.proxyOAIHandler)
	pm.ginEngine.POST("/v1/audio/transcriptions", auth, pm.proxyOAIPostFormHandler)

	pm.ginEngine.GET("/v1/models", auth, pm.listModelsHandler)

	// Info endpoint to show model-to-port mappings
	pm.ginEngine.GET("/info", auth, pm.infoHandler)

	// in proxymanager_loghandlers.go
	pm.ginEngine.GET("/logs", pm.sendLogsHandlers)
	pm.ginEngine.GET("/logs/stream", pm.streamLogsHandler)
	pm.ginEngine.GET("/logs/stream/:logMonitorID", pm.streamLogsHandler)

	/**
	 * User Interface Endpoints
	 */
	pm.ginEngine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui")
	})

	pm.ginEngine.GET("/upstream", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/models")
	})
	pm.ginEngine.Any("/upstream/*upstreamPath", pm.proxyToUpstream)

	pm.ginEngine.GET("/unload", pm.unloadAllModelsHandler)
	pm.ginEngine.GET("/running", pm.listRunningProcessesHandler)
	pm.ginEngine.GET("/health", pm.healthCheckHandler)

	pm.ginEngine.GET("/favicon.ico", func(c *gin.Context) {
		if data, err := reactStaticFS.ReadFile("ui_dist/favicon.ico"); err == nil {
			c.Data(http.StatusOK, "image/x-icon", data)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
	})

	pm.ginEngine.GET("/apple-touch-icon.png", func(c *gin.Context) {
		if data, err := reactStaticFS.ReadFile("ui_dist/apple-touch-icon.png"); err == nil {
			c.Data(http.StatusOK, "image/png", data)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
	})

	reactFS, err := GetReactFS()
	if err != nil {
		pm.proxyLogger.Errorf("Failed to load React filesystem: %v", err)
	} else {

		// serve files that exist under /ui/*
		pm.ginEngine.StaticFS("/ui", reactFS)

		// server SPA for UI under /ui/*
		pm.ginEngine.NoRoute(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/ui") {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			file, err := reactFS.Open("index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			defer file.Close()
			http.ServeContent(c.Writer, c.Request, "index.html", time.Now(), file)

		})
	}

	// see: proxymanager_api.go
	// add API handler functions
	addApiHandlers(pm)

	// Disable console color for testing
	gin.DisableConsoleColor()
}

// ServeHTTP implements http.Handler interface
func (pm *ProxyManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pm.ginEngine.ServeHTTP(w, r)
}

// requireAPIKey returns a gin.HandlerFunc that enforces API key only if enabled in settings.
func (pm *ProxyManager) requireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow unauthenticated access to settings endpoint so users can configure a key
		if strings.HasPrefix(c.Request.URL.Path, "/api/settings/system") {
			c.Next()
			return
		}

		if settings, _ := pm.loadSystemSettings(); settings != nil && settings.RequireAPIKey {
			key := c.GetHeader("Authorization")
			if key == "" {
				key = c.GetHeader("X-API-Key")
			}
			if key == "" {
				// Allow API key via query param for EventSource and limited clients
				key = c.Query("api_key")
			}
			if strings.HasPrefix(strings.ToLower(key), "bearer ") {
				key = strings.TrimSpace(key[7:])
			}
			if strings.TrimSpace(key) == "" || strings.TrimSpace(settings.APIKey) == "" || key != settings.APIKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required or invalid"})
				return
			}
		}
		c.Next()
	}
}

// StopProcesses acquires a lock and stops all running upstream processes.
// This is the public method safe for concurrent calls.
// Unlike Shutdown, this method only stops the processes but doesn't perform
// a complete shutdown, allowing for process replacement without full termination.
func (pm *ProxyManager) StopProcesses(strategy StopStrategy) {
	pm.Lock()
	defer pm.Unlock()

	// stop Processes in parallel
	var wg sync.WaitGroup
	for _, processGroup := range pm.processGroups {
		wg.Add(1)
		go func(processGroup *ProcessGroup) {
			defer wg.Done()
			processGroup.StopProcesses(strategy)
		}(processGroup)
	}

	wg.Wait()
}

// Shutdown stops all processes managed by this ProxyManager
func (pm *ProxyManager) Shutdown() {
	pm.Lock()
	defer pm.Unlock()

	pm.proxyLogger.Debug("Shutdown() called in proxy manager")

	var wg sync.WaitGroup
	// Send shutdown signal to all process in groups
	for _, processGroup := range pm.processGroups {
		wg.Add(1)
		go func(processGroup *ProcessGroup) {
			defer wg.Done()
			processGroup.Shutdown()
		}(processGroup)
	}
	wg.Wait()
	if pm.downloadSubCancel != nil {
		pm.downloadSubCancel()
		pm.downloadSubCancel = nil
	}
	pm.shutdownCancel()
}

func (pm *ProxyManager) swapProcessGroup(requestedModel string) (*ProcessGroup, string, error) {
	// de-alias the real model name and get a real one
	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		return nil, realModelName, fmt.Errorf("could not find real modelID for %s", requestedModel)
	}

	processGroup := pm.findGroupByModelName(realModelName)
	if processGroup == nil {
		return nil, realModelName, fmt.Errorf("could not find process group for model %s", requestedModel)
	}

	if processGroup.exclusive {
		pm.proxyLogger.Debugf("Exclusive mode for group %s, stopping other process groups", processGroup.id)
		for groupId, otherGroup := range pm.processGroups {
			if groupId != processGroup.id && !otherGroup.persistent {
				otherGroup.StopProcesses(StopWaitForInflightRequest)
			}
		}
	}

	return processGroup, realModelName, nil
}

func (pm *ProxyManager) listModelsHandler(c *gin.Context) {
	data := make([]gin.H, 0, len(pm.config.Models))
	createdTime := time.Now().Unix()

	for id, modelConfig := range pm.config.Models {
		if modelConfig.Unlisted {
			continue
		}

		record := gin.H{
			"id":       id,
			"object":   "model",
			"created":  createdTime,
			"owned_by": "ClaraCore",
		}

		if name := strings.TrimSpace(modelConfig.Name); name != "" {
			record["name"] = name
		}
		if desc := strings.TrimSpace(modelConfig.Description); desc != "" {
			record["description"] = desc
		}

		data = append(data, record)
	}

	// Sort by the "id" key
	sort.Slice(data, func(i, j int) bool {
		si, _ := data[i]["id"].(string)
		sj, _ := data[j]["id"].(string)
		return si < sj
	})

	// Set CORS headers if origin exists
	if origin := c.GetHeader("Origin"); origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
	}

	// Use gin's JSON method which handles content-type and encoding
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

func (pm *ProxyManager) proxyToUpstream(c *gin.Context) {
	upstreamPath := c.Param("upstreamPath")

	// If API key is required, enforce it
	if settings, _ := pm.loadSystemSettings(); settings != nil && settings.RequireAPIKey {
		key := c.GetHeader("Authorization")
		// Accept either Bearer <key> or raw key in X-API-Key
		if key == "" {
			key = c.GetHeader("X-API-Key")
		}
		if key == "" {
			// Allow API key via query param for EventSource and limited clients
			key = c.Query("api_key")
		}
		if strings.HasPrefix(strings.ToLower(key), "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		if strings.TrimSpace(key) == "" || strings.TrimSpace(settings.APIKey) == "" || key != settings.APIKey {
			pm.sendErrorResponse(c, http.StatusUnauthorized, "API key required or invalid")
			return
		}
	}

	// split the upstream path by / and search for the model name
	parts := strings.Split(strings.TrimSpace(upstreamPath), "/")
	if len(parts) == 0 {
		pm.sendErrorResponse(c, http.StatusBadRequest, "model id required in path")
		return
	}

	modelFound := false
	searchModelName := ""
	var modelName, remainingPath string
	for i, part := range parts {
		if parts[i] == "" {
			continue
		}

		if searchModelName == "" {
			searchModelName = part
		} else {
			searchModelName = searchModelName + "/" + parts[i]
		}

		if real, ok := pm.config.RealModelName(searchModelName); ok {
			modelName = real
			remainingPath = "/" + strings.Join(parts[i+1:], "/")
			modelFound = true
			break
		}
	}

	if !modelFound {
		pm.sendErrorResponse(c, http.StatusBadRequest, "model id required in path")
		return
	}

	processGroup, realModelName, err := pm.swapProcessGroup(modelName)
	if err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error swapping process group: %s", err.Error()))
		return
	}

	// rewrite the path
	c.Request.URL.Path = remainingPath
	processGroup.ProxyRequest(realModelName, c.Writer, c.Request)
}
func (pm *ProxyManager) proxyOAIHandler(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		pm.sendErrorResponse(c, http.StatusBadRequest, "could not ready request body")
		return
	}

	requestedModel := gjson.GetBytes(bodyBytes, "model").String()
	if requestedModel == "" {
		pm.sendErrorResponse(c, http.StatusBadRequest, "missing or invalid 'model' key")
		return
	}

	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		pm.sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("could not find real modelID for %s", requestedModel))
		return
	}

	processGroup, _, err := pm.swapProcessGroup(realModelName)
	if err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error swapping process group: %s", err.Error()))
		return
	}

	// issue #69 allow custom model names to be sent to upstream
	useModelName := pm.config.Models[realModelName].UseModelName
	if useModelName != "" {
		bodyBytes, err = sjson.SetBytes(bodyBytes, "model", useModelName)
		if err != nil {
			pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error rewriting model name in JSON: %s", err.Error()))
			return
		}
	}

	// issue #174 strip parameters from the JSON body
	stripParams, err := pm.config.Models[realModelName].Filters.SanitizedStripParams()
	if err != nil { // just log it and continue
		pm.proxyLogger.Errorf("Error sanitizing strip params string: %s, %s", pm.config.Models[realModelName].Filters.StripParams, err.Error())
	} else {
		for _, param := range stripParams {
			pm.proxyLogger.Debugf("<%s> stripping param: %s", realModelName, param)
			bodyBytes, err = sjson.DeleteBytes(bodyBytes, param)
			if err != nil {
				pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error deleting parameter %s from request", param))
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// dechunk it as we already have all the body bytes see issue #11
	c.Request.Header.Del("transfer-encoding")
	c.Request.Header.Set("content-length", strconv.Itoa(len(bodyBytes)))
	c.Request.ContentLength = int64(len(bodyBytes))

	if err := processGroup.ProxyRequest(realModelName, c.Writer, c.Request); err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error proxying request: %s", err.Error()))
		pm.proxyLogger.Errorf("Error Proxying Request for processGroup %s and model %s", processGroup.id, realModelName)
		return
	}
}

func (pm *ProxyManager) proxyOAIPostFormHandler(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max memory, larger files go to tmp disk
		pm.sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("error parsing multipart form: %s", err.Error()))
		return
	}

	// Get model parameter from the form
	requestedModel := c.Request.FormValue("model")
	if requestedModel == "" {
		pm.sendErrorResponse(c, http.StatusBadRequest, "missing or invalid 'model' parameter in form data")
		return
	}

	processGroup, realModelName, err := pm.swapProcessGroup(requestedModel)
	if err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error swapping process group: %s", err.Error()))
		return
	}

	// We need to reconstruct the multipart form in any case since the body is consumed
	// Create a new buffer for the reconstructed request
	var requestBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBuffer)

	// Copy all form values
	for key, values := range c.Request.MultipartForm.Value {
		for _, value := range values {
			fieldValue := value
			// If this is the model field and we have a profile, use just the model name
			if key == "model" {
				// # issue #69 allow custom model names to be sent to upstream
				useModelName := pm.config.Models[realModelName].UseModelName

				if useModelName != "" {
					fieldValue = useModelName
				} else {
					fieldValue = requestedModel
				}
			}
			field, err := multipartWriter.CreateFormField(key)
			if err != nil {
				pm.sendErrorResponse(c, http.StatusInternalServerError, "error recreating form field")
				return
			}
			if _, err = field.Write([]byte(fieldValue)); err != nil {
				pm.sendErrorResponse(c, http.StatusInternalServerError, "error writing form field")
				return
			}
		}
	}

	// Copy all files from the original request
	for key, fileHeaders := range c.Request.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			formFile, err := multipartWriter.CreateFormFile(key, fileHeader.Filename)
			if err != nil {
				pm.sendErrorResponse(c, http.StatusInternalServerError, "error recreating form file")
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				pm.sendErrorResponse(c, http.StatusInternalServerError, "error opening uploaded file")
				return
			}

			if _, err = io.Copy(formFile, file); err != nil {
				file.Close()
				pm.sendErrorResponse(c, http.StatusInternalServerError, "error copying file data")
				return
			}
			file.Close()
		}
	}

	// Close the multipart writer to finalize the form
	if err := multipartWriter.Close(); err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, "error finalizing multipart form")
		return
	}

	// Create a new request with the reconstructed form data
	modifiedReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		c.Request.URL.String(),
		&requestBuffer,
	)
	if err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, "error creating modified request")
		return
	}

	// Copy the headers from the original request
	modifiedReq.Header = c.Request.Header.Clone()
	modifiedReq.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// set the content length of the body
	modifiedReq.Header.Set("Content-Length", strconv.Itoa(requestBuffer.Len()))
	modifiedReq.ContentLength = int64(requestBuffer.Len())

	// Use the modified request for proxying
	if err := processGroup.ProxyRequest(realModelName, c.Writer, modifiedReq); err != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error proxying request: %s", err.Error()))
		pm.proxyLogger.Errorf("Error Proxying Request for processGroup %s and model %s", processGroup.id, realModelName)
		return
	}
}

func (pm *ProxyManager) sendErrorResponse(c *gin.Context, statusCode int, message string) {
	acceptHeader := c.GetHeader("Accept")

	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(statusCode, gin.H{"error": message})
	} else {
		c.String(statusCode, message)
	}
}

func (pm *ProxyManager) unloadAllModelsHandler(c *gin.Context) {
	pm.StopProcesses(StopImmediately)
	c.String(http.StatusOK, "OK")
}

func (pm *ProxyManager) healthCheckHandler(c *gin.Context) {
	// Count running models
	runningCount := 0
	totalModels := len(pm.config.Models)

	for _, processGroup := range pm.processGroups {
		for _, process := range processGroup.processes {
			if process.CurrentState() == StateReady {
				runningCount++
			}
		}
	}

	// Calculate uptime (would need to be tracked from server start)
	// For now, we'll return a simple OK status with counts
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"models_total":  totalModels,
		"models_loaded": runningCount,
		"timestamp":     time.Now().Unix(),
	})
}

func (pm *ProxyManager) listRunningProcessesHandler(context *gin.Context) {
	context.Header("Content-Type", "application/json")
	runningProcesses := make([]gin.H, 0) // Default to an empty response.

	for _, processGroup := range pm.processGroups {
		for _, process := range processGroup.processes {
			if process.CurrentState() == StateReady {
				runningProcesses = append(runningProcesses, gin.H{
					"model": process.ID,
					"state": process.state,
				})
			}
		}
	}

	// Put the results under the `running` key.
	response := gin.H{
		"running": runningProcesses,
	}

	context.JSON(http.StatusOK, response) // Always return 200 OK
}

func (pm *ProxyManager) infoHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	modelInfo := make([]gin.H, 0)

	for modelID, modelConfig := range pm.config.Models {
		// Extract port from proxy URL if available
		port := ""
		if modelConfig.Proxy != "" {
			// Parse the proxy URL to extract port
			if strings.Contains(modelConfig.Proxy, ":") {
				parts := strings.Split(modelConfig.Proxy, ":")
				if len(parts) >= 3 {
					port = parts[len(parts)-1] // Get the last part (port)
				}
			}
		}

		// Check if model is currently running
		isRunning := false
		processGroup := pm.findGroupByModelName(modelID)
		if processGroup != nil {
			if process, exists := processGroup.processes[modelID]; exists {
				isRunning = process.CurrentState() == StateReady
			}
		}

		modelInfo = append(modelInfo, gin.H{
			"model":       modelID,
			"port":        port,
			"proxy":       modelConfig.Proxy,
			"running":     isRunning,
			"name":        modelConfig.Name,
			"description": modelConfig.Description,
		})
	}

	// Sort by model ID for consistent output
	sort.Slice(modelInfo, func(i, j int) bool {
		mi, _ := modelInfo[i]["model"].(string)
		mj, _ := modelInfo[j]["model"].(string)
		return mi < mj
	})

	response := gin.H{
		"models": modelInfo,
		"total":  len(modelInfo),
	}

	c.JSON(http.StatusOK, response)
}

func (pm *ProxyManager) findGroupByModelName(modelName string) *ProcessGroup {
	for _, group := range pm.processGroups {
		if group.HasMember(modelName) {
			return group
		}
	}
	return nil
}

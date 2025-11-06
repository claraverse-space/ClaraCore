package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/prave/ClaraCore/autosetup"
	"github.com/prave/ClaraCore/event"
	"github.com/prave/ClaraCore/proxy"
)

var (
	version   string = "0"
	commit    string = "abcd1234"
	date      string = "unknown"
	startTime        = time.Now()
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			printVersion()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		case "serve":
			// Remove 'serve' from args and continue to server logic
			os.Args = append(os.Args[:1], os.Args[2:]...)
		case "service":
			handleServiceCommand()
			return
		case "ps":
			handlePsCommand()
			return
		case "list":
			handleListCommand()
			return
		default:
			// If it starts with -, it's a flag, continue to server logic
			if !strings.HasPrefix(os.Args[1], "-") {
				fmt.Printf("Unknown command: %s\n", os.Args[1])
				fmt.Println("Run 'claracore help' for usage information")
				os.Exit(1)
			}
		}
	}

	// Default behavior: start server
	startServer()
}

func printVersion() {
	fmt.Printf("ClaraCore version: %s (%s)\n", version, commit)
	fmt.Printf("Built: %s\n", date)
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println(`ClaraCore - AI Inference Server

USAGE:
    claracore [command] [flags]

COMMANDS:
    serve              Start the ClaraCore server (default if no command given)
    service <action>   Manage the ClaraCore background service
    ps                 Show running models and their status
    list               List all available models
    version            Show version information
    help               Show this help message

SERVICE ACTIONS:
    start              Start the background service
    stop               Stop the background service
    restart            Restart the background service
    status             Show service status
    logs               Show service logs
    enable             Enable auto-start on boot
    disable            Disable auto-start on boot

SERVER FLAGS:
    --config <path>          Config file path (default: config.yaml)
    --listen <addr>          Listen address (default: :5800)
    --socket <path>          Unix socket path (Linux/macOS only)
    --watch-config           Auto-reload config on change (default: true)
    --models-folder <path>   Auto-detect GGUF models and generate config
    --auto-draft             Enable draft model pairing
    --jinja                  Enable Jinja templating (default: true)
    --parallel               Enable parallel processing (default: true)
    --realtime               Enable real-time hardware monitoring
    --backend <type>         Force backend: cuda, rocm, cpu, vulkan, metal
    --ram <GB>               Force RAM amount (e.g., --ram 64)
    --vram <GB>              Force VRAM amount (e.g., --vram 24)

EXAMPLES:
    # Start server with default settings
    claracore

    # Start server on custom port
    claracore --listen :8080

    # Start with auto-setup from models folder
    claracore --models-folder /path/to/models

    # Use Unix socket (Linux/macOS)
    claracore --socket ~/.claracore/claracore.sock

    # Manage background service
    claracore service status
    claracore service logs

    # Show running models
    claracore ps

    # List available models
    claracore list

MORE INFO:
    Documentation: https://github.com/claraverse-space/ClaraCore
    Issues: https://github.com/claraverse-space/ClaraCore/issues
`)
}

func startServer() {
	// Define command-line flags
	configPath := flag.String("config", "config.yaml", "config file path")
	listenStr := flag.String("listen", ":5800", "listen address for HTTP server")
	socketPath := flag.String("socket", "", "Unix socket path (Linux/macOS only, takes precedence over --listen)")
	watchConfig := flag.Bool("watch-config", true, "automatically reload config on change")
	modelsFolder := flag.String("models-folder", "", "auto-detect GGUF models in folder")
	autoDraft := flag.Bool("auto-draft", false, "enable automatic draft model pairing")
	enableJinja := flag.Bool("jinja", true, "enable Jinja templating")
	parallel := flag.Bool("parallel", true, "enable parallel processing")
	realtime := flag.Bool("realtime", false, "enable real-time hardware monitoring")
	forceBackend := flag.String("backend", "", "force backend: cuda, rocm, cpu, vulkan, metal")
	forceRAM := flag.Float64("ram", 0, "force RAM in GB")
	forceVRAM := flag.Float64("vram", 0, "force VRAM in GB")

	flag.Parse()

	// Ensure config file exists
	if _, statErr := os.Stat(*configPath); statErr != nil {
		if os.IsNotExist(statErr) {
			if err := os.MkdirAll(filepath.Dir(*configPath), 0755); err != nil {
				fmt.Printf("Error creating config directory: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(*configPath, []byte{}, 0644); err != nil {
				fmt.Printf("Error creating empty config file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Created empty config at %s\n", *configPath)
		} else {
			fmt.Printf("Error checking config file: %v\n", statErr)
			os.Exit(1)
		}
	}

	// Handle auto-setup mode
	if *modelsFolder != "" {
		fmt.Println("🔄 Running auto-setup mode...")
		err := autosetup.AutoSetupWithOptions(*modelsFolder, autosetup.SetupOptions{
			EnableDraftModels: *autoDraft,
			EnableJinja:       *enableJinja,
			EnableParallel:    *parallel,
			EnableRealtime:    *realtime,
			ForceBackend:      *forceBackend,
			ForceRAM:          *forceRAM,
			ForceVRAM:         *forceVRAM,
		})
		if err != nil {
			fmt.Printf("❌ Auto-setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-setup completed successfully!")
		fmt.Println("🚀 Starting ClaraCore server...")
	}

	// Load and validate configuration
	config, err := proxy.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		if selfHealReconfigure(*configPath) {
			fmt.Println("🔧 Self-heal: regenerated configuration. Retrying...")
			config, err = proxy.LoadConfig(*configPath)
		}
		if err != nil {
			os.Exit(1)
		}
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	if len(config.Profiles) > 0 {
		fmt.Println("⚠️  WARNING: Profiles are deprecated. Use Groups instead.")
	}

	// Set Gin mode
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Setup channels for server management
	exitChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Determine listen address (Unix socket or HTTP)
	var listener net.Listener
	var listenAddr string

	if *socketPath != "" && (runtime.GOOS == "linux" || runtime.GOOS == "darwin") {
		// Use Unix socket
		if err := os.MkdirAll(filepath.Dir(*socketPath), 0755); err != nil {
			fmt.Printf("❌ Error creating socket directory: %v\n", err)
			os.Exit(1)
		}

		// Remove existing socket if it exists
		os.Remove(*socketPath)

		listener, err = net.Listen("unix", *socketPath)
		if err != nil {
			fmt.Printf("❌ Error creating Unix socket: %v\n", err)
			os.Exit(1)
		}

		// Set socket permissions
		if err := os.Chmod(*socketPath, 0600); err != nil {
			fmt.Printf("⚠️  Warning: Could not set socket permissions: %v\n", err)
		}

		listenAddr = *socketPath
		fmt.Printf("🔌 Listening on Unix socket: %s\n", listenAddr)
	} else {
		// Use HTTP
		listener, err = net.Listen("tcp", *listenStr)
		if err != nil {
			fmt.Printf("❌ Error binding to %s: %v\n", *listenStr, err)
			os.Exit(1)
		}
		listenAddr = *listenStr
		fmt.Printf("🌐 Listening on HTTP: %s\n", listenAddr)
		fmt.Printf("🎛️  Web interface: http://localhost%s/ui/\n", listenAddr)
	}

	defer listener.Close()

	// Create HTTP server
	srv := &http.Server{}

	// Config reload handler
	reloadProxyManager := func() {
		if currentPM, ok := srv.Handler.(*proxy.ProxyManager); ok {
			newConfig, err := proxy.LoadConfig(*configPath)
			if err != nil {
				fmt.Printf("⚠️  Warning: Unable to reload config: %v\n", err)
				return
			}

			fmt.Println("📝 Configuration changed - reloading...")
			currentPM.Shutdown()
			srv.Handler = proxy.New(newConfig)
			fmt.Println("✅ Configuration reloaded successfully")

			time.AfterFunc(3*time.Second, func() {
				event.Emit(proxy.ConfigFileChangedEvent{
					ReloadingState: proxy.ReloadingStateEnd,
				})
			})
		} else {
			newConfig, err := proxy.LoadConfig(*configPath)
			if err != nil {
				fmt.Printf("❌ Error loading config: %v\n", err)
				if selfHealReconfigure(*configPath) {
					fmt.Println("🔧 Self-heal: regenerated configuration")
					newConfig, err = proxy.LoadConfig(*configPath)
				}
				if err != nil {
					os.Exit(1)
				}
			}
			srv.Handler = proxy.New(newConfig)
		}
	}

	// Load initial proxy manager
	reloadProxyManager()
	debouncedReload := debounce(time.Second, reloadProxyManager)

	// Watch config file for changes
	if *watchConfig {
		defer event.On(func(e proxy.ConfigFileChangedEvent) {
			if e.ReloadingState == proxy.ReloadingStateStart {
				debouncedReload()
			}
		})()

		fmt.Printf("📁 Watching config file for changes: %s\n", *configPath)
		go watchConfigFile(*configPath)
	}

	// Handle shutdown signals
	go func() {
		sig := <-sigChan
		fmt.Printf("\n🛑 Received signal %v, shutting down gracefully...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if pm, ok := srv.Handler.(*proxy.ProxyManager); ok {
			pm.Shutdown()
		}

		if err := srv.Shutdown(ctx); err != nil {
			fmt.Printf("⚠️  Server shutdown error: %v\n", err)
		}
		close(exitChan)
	}()

	// Start server
	fmt.Println("✅ ClaraCore server started successfully!")
	fmt.Println("📊 System ready to serve requests")
	fmt.Println("💡 Press Ctrl+C to stop the server")

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Fatal server error: %v\n", err)
		}
	}()

	// Wait for exit signal
	<-exitChan
	fmt.Println("👋 ClaraCore stopped")
}

func watchConfigFile(configPath string) {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		fmt.Printf("⚠️  Error getting absolute path: %v\n", err)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("⚠️  Error creating file watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	configDir := filepath.Dir(absConfigPath)
	if err := watcher.Add(configDir); err != nil {
		fmt.Printf("⚠️  Error watching config directory: %v\n", err)
		return
	}

	for {
		select {
		case changeEvent := <-watcher.Events:
			if changeEvent.Name == absConfigPath &&
			   (changeEvent.Has(fsnotify.Write) ||
			    changeEvent.Has(fsnotify.Create) ||
			    changeEvent.Has(fsnotify.Remove)) {
				event.Emit(proxy.ConfigFileChangedEvent{
					ReloadingState: proxy.ReloadingStateStart,
				})
			} else if changeEvent.Name == filepath.Join(configDir, "..data") &&
			          changeEvent.Has(fsnotify.Create) {
				// Kubernetes ConfigMap change
				event.Emit(proxy.ConfigFileChangedEvent{
					ReloadingState: proxy.ReloadingStateStart,
				})
			}
		case err := <-watcher.Errors:
			log.Printf("⚠️  File watcher error: %v", err)
		}
	}
}

func validateConfig(config proxy.Config) error {
	// Validate port ranges
	if config.StartPort < 1024 || config.StartPort > 65535 {
		return fmt.Errorf("invalid startPort: must be between 1024 and 65535")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "": true,
	}
	if !validLogLevels[strings.ToLower(config.LogLevel)] {
		return fmt.Errorf("invalid logLevel: must be debug, info, warn, or error")
	}

	// Validate health check timeout
	if config.HealthCheckTimeout < 0 {
		return fmt.Errorf("invalid healthCheckTimeout: must be non-negative")
	}

	// Validate model configurations
	for modelID, model := range config.Models {
		if model.Cmd == "" {
			return fmt.Errorf("model '%s': cmd is required", modelID)
		}
		if model.Proxy == "" {
			return fmt.Errorf("model '%s': proxy URL is required", modelID)
		}
	}

	return nil
}

func debounce(interval time.Duration, f func()) func() {
	var timer *time.Timer
	return func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(interval, f)
	}
}

func selfHealReconfigure(configPath string) bool {
	dbPath := "model_folders.json"
	data, err := os.ReadFile(dbPath)
	if err != nil {
		fmt.Printf("Self-heal: no folder DB: %v\n", err)
		return false
	}

	var db struct {
		Folders []struct {
			Path    string
			Enabled bool
		} `json:"folders"`
	}

	if err := json.Unmarshal(data, &db); err != nil {
		fmt.Printf("Self-heal: invalid folder DB: %v\n", err)
		return false
	}

	var folders []string
	for _, f := range db.Folders {
		if f.Enabled {
			folders = append(folders, f.Path)
		}
	}

	if len(folders) == 0 {
		fmt.Println("Self-heal: no enabled folders")
		return false
	}

	// Load saved settings
	settingsPath := "settings.json"
	opts := autosetup.SetupOptions{
		EnableJinja:      true,
		ThroughputFirst:  true,
		MinContext:       16384,
		PreferredContext: 32768,
	}

	if sdata, err := os.ReadFile(settingsPath); err == nil {
		var s struct {
			Backend          string  `json:"backend"`
			VRAMGB           float64 `json:"vramGB"`
			RAMGB            float64 `json:"ramGB"`
			PreferredContext int     `json:"preferredContext"`
			ThroughputFirst  bool    `json:"throughputFirst"`
			EnableJinja      bool    `json:"enableJinja"`
		}
		if json.Unmarshal(sdata, &s) == nil {
			opts.EnableJinja = s.EnableJinja
			opts.ThroughputFirst = s.ThroughputFirst
			if s.PreferredContext > 0 {
				opts.PreferredContext = s.PreferredContext
			}
			if s.RAMGB > 0 {
				opts.ForceRAM = s.RAMGB
			}
			if s.VRAMGB > 0 {
				opts.ForceVRAM = s.VRAMGB
			}
			if s.Backend != "" {
				opts.ForceBackend = s.Backend
			}
		}
	}

	fmt.Printf("Self-heal: regenerating config from %d folder(s)\n", len(folders))

	if len(folders) > 1 {
		if err := autosetup.AutoSetupMultiFoldersWithOptions(folders, opts); err != nil {
			fmt.Printf("Self-heal: generation failed: %v\n", err)
			return false
		}
	} else {
		if err := autosetup.AutoSetupWithOptions(folders[0], opts); err != nil {
			fmt.Printf("Self-heal: generation failed: %v\n", err)
			return false
		}
	}

	event.Emit(proxy.ConfigFileChangedEvent{ReloadingState: proxy.ReloadingStateStart})
	return true
}

func handleServiceCommand() {
	fmt.Println("Service management command (implementation in next step)")
	// This will be implemented with platform-specific service management
}

func handlePsCommand() {
	fmt.Println("Show running models (implementation in next step)")
	// This will query the running instance via HTTP/Unix socket
}

func handleListCommand() {
	fmt.Println("List available models (implementation in next step)")
	// This will query the running instance via HTTP/Unix socket
}

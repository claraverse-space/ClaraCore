package autosetup

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetupOptions contains configuration options for auto-setup
type SetupOptions struct {
	EnableDraftModels bool
	EnableJinja       bool
	EnableParallel    bool    // Enable parallel processing (should be renamed to EnableDeployment)
	EnableRealtime    bool    // Enable real-time hardware monitoring for dynamic allocation
	ThroughputFirst   bool    // Prioritize speed over maximum context
	MaxSpeed          bool    // Maximum GPU utilization, minimum context
	MinContext        int     // Minimum context size (default: 16384)
	PreferredContext  int     // Preferred context size (default: 32768)
	ForceBackend      string  // Force specific backend (cuda, rocm, cpu, vulkan) - overrides auto-detection
	ForceRAM          float64 // Force total RAM in GB - overrides auto-detection
	ForceVRAM         float64 // Force total VRAM in GB - overrides auto-detection
}

// AutoSetup performs automatic model detection and configuration with default options
func AutoSetup(modelsFolder string) error {
	return AutoSetupWithOptions(modelsFolder, SetupOptions{
		EnableDraftModels: false, // Disabled by default
		EnableJinja:       true,  // Enabled by default
		EnableParallel:    false, // Disabled by default - only enable for deployment
		ThroughputFirst:   true,  // Prioritize speed by default
		MaxSpeed:          false, // Balanced approach by default
		MinContext:        16384, // 16K minimum context
		PreferredContext:  32768, // 32K preferred context
	})
}

// AutoSetupWithOptions performs automatic model detection and configuration with custom options
func AutoSetupWithOptions(modelsFolder string, options SetupOptions) error {
	fmt.Println("🚀 Starting llama-swap auto-setup...")

	// Validate models folder
	if modelsFolder == "" {
		return fmt.Errorf("models folder path is required")
	}

	if _, err := os.Stat(modelsFolder); os.IsNotExist(err) {
		return fmt.Errorf("models folder does not exist: %s", modelsFolder)
	}

	fmt.Printf("📁 Scanning models in: %s\n", modelsFolder)

	// Detect models with options
	models, err := DetectModelsWithOptions(modelsFolder, options)
	if err != nil {
		return fmt.Errorf("failed to detect models: %v", err)
	}

	if len(models) == 0 {
		return fmt.Errorf("no GGUF models found in: %s", modelsFolder)
	}

	fmt.Printf("✅ Found %d GGUF models:\n", len(models))
	for _, model := range models {
		fmt.Printf("   - %s", model.Name)
		if model.Size != "" {
			fmt.Printf(" (%s)", model.Size)
		}
		if model.Quantization != "" {
			fmt.Printf(" [%s]", model.Quantization)
		}
		if model.IsInstruct {
			fmt.Printf(" [Instruct]")
		}
		if model.IsDraft {
			fmt.Printf(" [Draft]")
		}
		fmt.Println()
	}

	// Detect system
	fmt.Println("\n🔍 Detecting system capabilities...")
	system := DetectSystem()

	// Enhance system information with detailed detection
	if err := EnhanceSystemInfo(&system); err != nil {
		fmt.Printf("Warning: Failed to enhance system detection: %v\n", err)
	}

	// Apply hardware overrides if specified
	if options.ForceBackend != "" || options.ForceRAM > 0 || options.ForceVRAM > 0 {
		fmt.Println("\n🎛️  Applying hardware overrides...")

		if options.ForceBackend != "" {
			// Determine current backend preference
			currentBackend := "cpu"
			if system.HasCUDA {
				currentBackend = "cuda"
			} else if system.HasVulkan {
				currentBackend = "vulkan"
			} else if system.HasMetal {
				currentBackend = "metal"
			} else if system.HasROCm {
				currentBackend = "rocm"
			}

			// Override system capabilities based on forced backend
			system.HasCUDA = (options.ForceBackend == "cuda")
			system.HasVulkan = (options.ForceBackend == "vulkan")
			system.HasMetal = (options.ForceBackend == "metal")
			system.HasROCm = (options.ForceBackend == "rocm")

			fmt.Printf("   🔧 Backend: %s → %s (forced)\n", currentBackend, options.ForceBackend)
		}

		if options.ForceRAM > 0 {
			originalRAM := system.TotalRAMGB
			system.TotalRAMGB = options.ForceRAM
			fmt.Printf("   🧠 RAM: %.1f GB → %.1f GB (forced)\n", originalRAM, system.TotalRAMGB)
		}

		if options.ForceVRAM > 0 {
			originalVRAM := system.TotalVRAMGB
			system.TotalVRAMGB = options.ForceVRAM
			fmt.Printf("   🎮 VRAM: %.1f GB → %.1f GB (forced)\n", originalVRAM, system.TotalVRAMGB)
		}
	}

	// Print comprehensive system information
	fmt.Printf("\n")
	PrintSystemInfo(&system)
	fmt.Printf("\n")

	// Print detailed model information
	PrintModelInfo(models, modelsFolder)
	fmt.Printf("\n")

	// Debug mmproj files (temporary for testing)
	DebugMMProjMetadata(modelsFolder)
	fmt.Printf("\n")

	// Debug main model metadata to find matching keys
	DebugModelMetadata(models)
	fmt.Printf("\n")

	// Debug embedding detection to verify classification accuracy
	DebugEmbeddingDetection(models)
	fmt.Printf("\n")

	// Find mmproj matches using metadata-based matching
	mmprojMatches := FindMMProjMatches(models, modelsFolder)
	fmt.Printf("\n")

	// Download binary
	fmt.Println("\n⬇️  Downloading llama-server binary...")

	// Create binaries directory
	binariesDir := filepath.Join(".", "binaries")
	binary, err := DownloadBinary(binariesDir, system)
	if err != nil {
		return fmt.Errorf("failed to download binary: %v", err)
	}

	fmt.Printf("✅ Downloaded: %s (%s)\n", binary.Path, binary.Type)

	// Generate configuration
	fmt.Println("\n⚙️  Generating configuration...")

	if options.EnableDraftModels {
		fmt.Println("🚀 Draft models enabled - Speculative decoding will be used for suitable models")
	} else {
		fmt.Println("⏭️  Draft models disabled - Use --auto-draft to enable speculative decoding")
	}

	if options.EnableJinja {
		fmt.Println("📝 Jinja templating enabled for chat models")
	}

	if options.EnableParallel {
		fmt.Println("⚡ Parallel processing enabled for faster setup")
	}

	// Initialize memory estimator
	memEstimator := NewMemoryEstimator()

	// Use total GPU VRAM instead of available VRAM for allocation
	totalVRAM := system.TotalVRAMGB
	if totalVRAM == 0 {
		// Fallback to memory estimator if system detection failed
		fmt.Print("🔍 Detecting available VRAM... ")
		availableVRAM, err := memEstimator.GetAvailableVRAM()
		if err != nil {
			fmt.Printf("failed (using default 12GB): %v\n", err)
			totalVRAM = 12.0 // Default fallback
		} else {
			fmt.Printf("%.1f GB detected\n", availableVRAM)
			totalVRAM = availableVRAM
		}
	} else {
		fmt.Printf("🎯 Using total GPU VRAM: %.1f GB for allocation\n", totalVRAM)
	}

	// Use config generator with smart GPU allocation
	configPath := "config.yaml"
	generator := NewConfigGenerator(modelsFolder, binary.Path, configPath, options)
	generator.SetAvailableVRAM(totalVRAM)
	generator.SetBinaryType(binary.Type)
	generator.SetSystemInfo(&system)          // Pass system info for optimal parameters
	generator.SetMMProjMatches(mmprojMatches) // Pass mmproj matches to config generator

	fmt.Printf("⚙️  Generating configuration (SMART GPU ALLOCATION: fit max layers in VRAM)...\n")
	err = generator.GenerateConfig(models)
	if err != nil {
		return fmt.Errorf("failed to generate configuration: %v", err)
	}

	fmt.Printf("✅ Configuration saved to: %s\n", configPath)

	// Print summary
	fmt.Println("\n📋 Setup Summary:")
	fmt.Printf("   Models folder: %s\n", modelsFolder)
	fmt.Printf("   Binary: %s\n", binary.Path)
	fmt.Printf("   Configuration: %s\n", configPath)
	fmt.Printf("   Models detected: %d\n", len(models))

	// Print platform support summary
	PrintPlatformSupportSummary()

	// Print next steps
	fmt.Println("\n🎉 Setup complete! Next steps:")
	fmt.Println("   1. Review the generated config.yaml file")
	fmt.Println("   2. Start ClaraCore: ./clara-core")
	fmt.Println("   3. Test with: curl http://localhost:8080/v1/models")

	// Print available models
	fmt.Println("\n📚 Available models:")
	for _, model := range models {
		if !model.IsDraft {
			modelID := generator.generateModelID(model)
			fmt.Printf("   - %s\n", modelID)
		}
	}

	return nil
}

// ValidateSetup checks if auto-setup has been run and is valid
func ValidateSetup() error {
	// Check if config.yaml exists
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		return fmt.Errorf("config.yaml not found - run with --models-folder to auto-generate")
	}

	// Check if binaries directory exists
	if _, err := os.Stat("binaries"); os.IsNotExist(err) {
		return fmt.Errorf("binaries directory not found - run with --models-folder to auto-download")
	}

	return nil
}

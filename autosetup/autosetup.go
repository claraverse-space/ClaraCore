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
	fmt.Println("🚀 Starting ClaraCore auto-setup...")

	// Validate and create models folder if needed
	if modelsFolder == "" {
		return fmt.Errorf("models folder path is required")
	}

	if _, err := os.Stat(modelsFolder); os.IsNotExist(err) {
		fmt.Printf("📁 Models folder does not exist, creating: %s\n", modelsFolder)
		err = os.MkdirAll(modelsFolder, 0755)
		if err != nil {
			return fmt.Errorf("failed to create models folder %s: %v", modelsFolder, err)
		}
		fmt.Printf("✅ Created models folder: %s\n", modelsFolder)
	}

	fmt.Printf("📁 Scanning models in: %s\n", modelsFolder)

	// Detect models with options
	models, err := DetectModelsWithOptions(modelsFolder, options)
	if err != nil {
		return fmt.Errorf("failed to detect models: %v", err)
	}

	if len(models) == 0 {
		fmt.Printf("⚠️  No GGUF models found in: %s\n", modelsFolder)
		fmt.Printf("💡 You can:\n")
		fmt.Printf("   1. Add .gguf model files to: %s\n", modelsFolder)
		fmt.Printf("   2. Use the web interface to download models: http://localhost:5800/ui/setup\n")
		fmt.Printf("   3. Use huggingface-cli to download models:\n")
		fmt.Printf("      huggingface-cli download <model-name> --include '*.gguf' --local-dir %s\n", modelsFolder)
		fmt.Printf("\n📝 Creating basic configuration file for when you add models...\n")
		
		// Create a basic config with just the folder path for future use
		err = createBasicConfig(modelsFolder)
		if err != nil {
			return fmt.Errorf("failed to create basic configuration: %v", err)
		}
		
		fmt.Printf("✅ Basic configuration created. Add models to %s and restart ClaraCore.\n", modelsFolder)
		return nil
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
	binary, err := DownloadBinary(binariesDir, system, options.ForceBackend)
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

// AutoSetupMultiFoldersWithOptions performs automatic model detection and configuration from multiple folders
func AutoSetupMultiFoldersWithOptions(modelsFolders []string, options SetupOptions) error {
	fmt.Println("🚀 Starting ClaraCore multi-folder auto-setup...")

	// Validate folders
	if len(modelsFolders) == 0 {
		return fmt.Errorf("at least one models folder path is required")
	}

	var validFolders []string
	for _, folder := range modelsFolders {
		if folder == "" {
			continue
		}
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			fmt.Printf("⚠️  Skipping non-existent folder: %s\n", folder)
			continue
		}
		validFolders = append(validFolders, folder)
	}

	if len(validFolders) == 0 {
		return fmt.Errorf("no valid model folders found")
	}

	fmt.Printf("📁 Scanning models in %d folders:\n", len(validFolders))
	for _, folder := range validFolders {
		fmt.Printf("   - %s\n", folder)
	}

	// Detect models from all folders
	var allModels []ModelInfo
	var allMMProjMatches []MMProjMatch

	for _, folder := range validFolders {
		fmt.Printf("\n🔍 Scanning folder: %s\n", folder)

		// Detect models with options
		models, err := DetectModelsWithOptions(folder, options)
		if err != nil {
			fmt.Printf("⚠️  Failed to detect models in %s: %v\n", folder, err)
			continue
		}

		if len(models) == 0 {
			fmt.Printf("⚠️  No GGUF models found in: %s\n", folder)
			continue
		}

		fmt.Printf("✅ Found %d GGUF models in %s:\n", len(models), folder)
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

		allModels = append(allModels, models...)

		// Detect mmproj files in this folder
		mmprojMatches := FindMMProjMatches(models, folder)
		allMMProjMatches = append(allMMProjMatches, mmprojMatches...)
	}

	if len(allModels) == 0 {
		return fmt.Errorf("no GGUF models found in any of the provided folders")
	}

	fmt.Printf("\n📊 Total models found across all folders: %d\n", len(allModels))

	// Detect system (same as single folder)
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
			fmt.Printf("   🔧 Backend: %s (forced)\n", options.ForceBackend)
			// Note: PreferredBackend field doesn't exist in SystemInfo, but that's okay
			// The backend selection is handled elsewhere in the system
		}
		if options.ForceRAM > 0 {
			fmt.Printf("   🧠 RAM: %.1f GB → %.1f GB (forced)\n", system.TotalRAMGB, options.ForceRAM)
			system.TotalRAMGB = options.ForceRAM
		}
		if options.ForceVRAM > 0 {
			fmt.Printf("   🎮 VRAM: %.1f GB → %.1f GB (forced)\n", system.TotalVRAMGB, options.ForceVRAM)
			system.TotalVRAMGB = options.ForceVRAM
		}
	}

	// Print system information
	PrintSystemInfo(&system)

	// Download binary (same as single folder)
	fmt.Println("\n⬇️  Downloading llama-server binary...")

	// Create binaries directory
	binariesDir := filepath.Join(".", "binaries")
	binary, err := DownloadBinary(binariesDir, system, options.ForceBackend)
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
	// Use multi-folder config generator to properly track all model folders
	configPath := "config.yaml"
	generator := NewConfigGeneratorMultiFolder(validFolders, binary.Path, configPath, options)
	generator.SetAvailableVRAM(totalVRAM)
	generator.SetBinaryType(binary.Type)
	generator.SetSystemInfo(&system)             // Pass system info for optimal parameters
	generator.SetMMProjMatches(allMMProjMatches) // Pass all mmproj matches to config generator

	fmt.Printf("⚙️  Generating configuration from %d folders (SMART GPU ALLOCATION: fit max layers in VRAM)...\n", len(validFolders))
	err = generator.GenerateConfig(allModels) // Use ALL models from ALL folders
	if err != nil {
		return fmt.Errorf("failed to generate configuration: %v", err)
	}

	fmt.Printf("✅ Configuration saved to: %s\n", configPath)

	// Print summary
	fmt.Println("\n📋 Setup Summary:")
	fmt.Printf("   Model folders: %d\n", len(validFolders))
	for i, folder := range validFolders {
		fmt.Printf("     %d. %s\n", i+1, folder)
	}
	fmt.Printf("   Binary: %s\n", binary.Path)
	fmt.Printf("   Configuration: %s\n", configPath)
	fmt.Printf("   Models detected: %d\n", len(allModels))

	// Print platform support summary
	PrintPlatformSupportSummary()

	// Print next steps
	fmt.Println("\n🎉 Setup complete! Next steps:")
	fmt.Println("   1. Review the generated config.yaml file")
	fmt.Println("   2. Start ClaraCore: ./clara-core")
	fmt.Println("   3. Test with: curl http://localhost:8080/v1/models")

	// Print available models
	fmt.Println("\n📚 Available models:")
	for _, model := range allModels {
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

// createBasicConfig creates a minimal config.yaml with the models folder path
func createBasicConfig(modelsFolder string) error {
	basicConfig := fmt.Sprintf(`# ClaraCore Configuration
# Generated automatically - add models to %s and regenerate

server:
  port: 8080
  max_request_size: 100MB

groups:
  default:
    timeout: 30s

models:
  # Models will be auto-detected when you add .gguf files to %s
  # Run: ./claracore --models-folder %s
  # Or use the web interface: http://localhost:5800/ui/setup

# Model folder for scanning
model_folders:
  - "%s"
`, modelsFolder, modelsFolder, modelsFolder, modelsFolder)

	err := os.WriteFile("config.yaml", []byte(basicConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config.yaml: %v", err)
	}

	return nil
}

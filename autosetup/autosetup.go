package autosetup

import (
	"fmt"
	"os"
	"path/filepath"
)

// AutoSetup performs automatic model detection and configuration
func AutoSetup(modelsFolder string) error {
	fmt.Println("🚀 Starting llama-swap auto-setup...")

	// Validate models folder
	if modelsFolder == "" {
		return fmt.Errorf("models folder path is required")
	}

	if _, err := os.Stat(modelsFolder); os.IsNotExist(err) {
		return fmt.Errorf("models folder does not exist: %s", modelsFolder)
	}

	fmt.Printf("📁 Scanning models in: %s\n", modelsFolder)

	// Detect models
	models, err := DetectModels(modelsFolder)
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

	fmt.Printf("   OS: %s/%s\n", system.OS, system.Architecture)
	if system.HasCUDA {
		fmt.Println("   GPU: CUDA detected ✅")
	} else if system.HasROCm {
		fmt.Println("   GPU: ROCm detected ✅")
	} else if system.HasVulkan {
		fmt.Println("   GPU: Vulkan detected ✅")
	} else if system.HasMetal {
		fmt.Println("   GPU: Metal detected ✅")
	} else {
		fmt.Println("   GPU: CPU-only mode")
	}

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

	generator := &ConfigGenerator{
		Models:    models,
		Binary:    binary,
		System:    system,
		StartPort: 5800,
		ModelsDir: modelsFolder,
	}

	configPath := "config.yaml"
	err = generator.SaveConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("✅ Configuration saved to: %s\n", configPath)

	// Print summary
	fmt.Println("\n📋 Setup Summary:")
	fmt.Printf("   Models folder: %s\n", modelsFolder)
	fmt.Printf("   Binary: %s\n", binary.Path)
	fmt.Printf("   Configuration: %s\n", configPath)
	fmt.Printf("   Models detected: %d\n", len(models))

	// Print next steps
	fmt.Println("\n🎉 Setup complete! Next steps:")
	fmt.Println("   1. Review the generated config.yaml file")
	fmt.Println("   2. Start llama-swap: ./llama-swap")
	fmt.Println("   3. Test with: curl http://localhost:8080/v1/models")

	// Print available models
	fmt.Println("\n📚 Available models:")
	for _, model := range models {
		if !model.IsDraft {
			generator := &ConfigGenerator{Models: models}
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

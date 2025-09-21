package autosetup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ConfigGenerator generates YAML configuration from detected models and binary
type ConfigGenerator struct {
	Models    []ModelInfo
	Binary    *BinaryInfo
	System    SystemInfo
	StartPort int
	ModelsDir string
}

// GenerateConfig creates a complete YAML configuration
func (cg *ConfigGenerator) GenerateConfig() (string, error) {
	config := strings.Builder{}

	// Header
	config.WriteString("# Auto-generated llama-swap configuration\n")
	config.WriteString("# Generated from models in: " + cg.ModelsDir + "\n")
	config.WriteString("# Binary: " + cg.Binary.Path + " (" + cg.Binary.Type + ")\n")
	config.WriteString("# System: " + cg.System.OS + "/" + cg.System.Architecture + "\n\n")

	// Global settings
	config.WriteString("healthCheckTimeout: 300\n")
	config.WriteString("logLevel: info\n")
	config.WriteString("startPort: " + strconv.Itoa(cg.StartPort) + "\n\n")

	// Add macros
	config.WriteString("macros:\n")
	config.WriteString("  \"llama-server-base\": >\n")
	config.WriteString("    " + cg.Binary.Path + "\n")
	config.WriteString("    --host 127.0.0.1\n")
	config.WriteString("    --port ${PORT}\n")
	config.WriteString("    --metrics\n")
	config.WriteString("    --flash-attn auto\n")

	// Add GPU-specific flags
	if cg.Binary.Type == "cuda" || cg.Binary.Type == "rocm" {
		config.WriteString("    -ngl 99\n")
	}

	config.WriteString("\n")

	// Sort models by size (largest first)
	sortedModels := SortModelsBySize(cg.Models)

	// Generate models section
	config.WriteString("models:\n")

	currentPort := cg.StartPort
	usedIDs := make(map[string]int) // Track used IDs to handle duplicates

	for i, model := range sortedModels {
		// Skip draft models from main models list
		if model.IsDraft {
			continue
		}

		baseID := cg.generateModelID(model)
		modelID := baseID

		// Handle duplicates by adding suffix
		if count, exists := usedIDs[baseID]; exists {
			usedIDs[baseID] = count + 1
			modelID = fmt.Sprintf("%s-%d", baseID, count+1)
		} else {
			usedIDs[baseID] = 1
		}

		config.WriteString("  \"" + modelID + "\":\n") // Add model metadata
		if model.Size != "" {
			config.WriteString("    name: \"" + cg.generateModelName(model) + "\"\n")
			config.WriteString("    description: \"" + cg.generateModelDescription(model) + "\"\n")
		}

		// Find a draft model if available
		draftModel := FindDraftModel(cg.Models, model)

		// Generate command
		config.WriteString("    cmd: |\n")
		config.WriteString("      ${llama-server-base}\n")
		config.WriteString("      --model " + model.Path + "\n")

		// Add context size based on model size
		ctxSize := cg.getOptimalContextSize(model)
		config.WriteString("      --ctx-size " + strconv.Itoa(ctxSize) + "\n")

		// Add draft model for speculative decoding if available
		if draftModel != nil {
			config.WriteString("      --model-draft " + draftModel.Path + "\n")
			config.WriteString("      -ngld 99\n")
			config.WriteString("      --draft-max 16\n")
			config.WriteString("      --draft-min 4\n")
			config.WriteString("      --draft-p-min 0.4\n")

			// Add GPU assignment for multi-GPU setups
			if cg.System.HasCUDA && cg.hasMultipleGPUs() {
				config.WriteString("      --device CUDA0\n")
				config.WriteString("      --device-draft CUDA1\n")
			}
		}

		// Add sampling parameters for instruct models
		if model.IsInstruct {
			config.WriteString("      --temp 0.7\n")
			config.WriteString("      --repeat-penalty 1.1\n")
			config.WriteString("      --top-p 0.9\n")
			config.WriteString("      --top-k 40\n")
		}

		// Set proxy URL
		config.WriteString("    proxy: \"http://127.0.0.1:${PORT}\"\n")

		// Add common aliases for popular models
		aliases := cg.generateAliases(model)
		if len(aliases) > 0 {
			config.WriteString("    aliases:\n")
			for _, alias := range aliases {
				config.WriteString("      - \"" + alias + "\"\n")
			}
		}

		// Add TTL for larger models to save memory
		if cg.shouldAddTTL(model) {
			config.WriteString("    ttl: 300  # Auto-unload after 5 minutes of inactivity\n")
		}

		// Add environment variables for GPU selection
		if cg.System.HasCUDA && len(sortedModels) > 1 {
			gpuIndex := i % cg.getGPUCount()
			config.WriteString("    env:\n")
			config.WriteString("      - \"CUDA_VISIBLE_DEVICES=" + strconv.Itoa(gpuIndex) + "\"\n")
		}

		config.WriteString("\n")
		currentPort++
	}

	// Add groups configuration for advanced setups
	if len(sortedModels) > 2 {
		config.WriteString(cg.generateGroupsConfig(sortedModels))
	}

	return config.String(), nil
}

// generateModelID creates a clean model ID from the filename
func (cg *ConfigGenerator) generateModelID(model ModelInfo) string {
	name := strings.ToLower(model.Name)

	// Remove common suffixes
	suffixes := []string{"-instruct", "-chat", "-gguf"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	// Remove quantization and replace with size if available
	if model.Quantization != "" {
		name = strings.ReplaceAll(name, strings.ToLower(model.Quantization), "")
	}

	// Clean up the name
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.Trim(name, "-")

	// Add size if available
	if model.Size != "" {
		name = name + "-" + strings.ToLower(model.Size)
	}

	return name
}

// generateModelName creates a display name
func (cg *ConfigGenerator) generateModelName(model ModelInfo) string {
	name := model.Name
	if model.Size != "" {
		return fmt.Sprintf("%s %s", extractModelFamily(name), model.Size)
	}
	return name
}

// generateModelDescription creates a description
func (cg *ConfigGenerator) generateModelDescription(model ModelInfo) string {
	desc := "Auto-detected model"
	if model.Size != "" {
		desc += " (" + model.Size + ")"
	}
	if model.Quantization != "" {
		desc += " with " + model.Quantization + " quantization"
	}
	if model.IsInstruct {
		desc += " - Instruction-tuned"
	}
	return desc
}

// generateAliases creates common aliases for models
func (cg *ConfigGenerator) generateAliases(model ModelInfo) []string {
	var aliases []string
	family := strings.ToLower(extractModelFamily(model.Name))

	// Add family-based aliases
	switch {
	case strings.Contains(family, "qwen"):
		if model.Size == "32B" {
			aliases = append(aliases, "qwen-large", "coder")
		} else if model.Size == "7B" || model.Size == "8B" {
			aliases = append(aliases, "qwen-medium")
		}
	case strings.Contains(family, "llama"):
		if model.Size == "70B" {
			aliases = append(aliases, "llama-large")
		} else if model.Size == "8B" {
			aliases = append(aliases, "llama-medium")
		}
		if model.IsInstruct {
			aliases = append(aliases, "gpt-4o-mini", "gpt-3.5-turbo")
		}
	case strings.Contains(family, "mistral"):
		aliases = append(aliases, "mistral")
	}

	return aliases
}

// getOptimalContextSize returns optimal context size based on model size
func (cg *ConfigGenerator) getOptimalContextSize(model ModelInfo) int {
	switch model.Size {
	case "0.5B", "1B", "1.5B":
		return 8192
	case "3B", "7B", "8B":
		return 16384
	case "13B":
		return 32768
	case "32B":
		return 65536
	case "70B", "405B":
		return 131072
	default:
		return 16384
	}
}

// shouldAddTTL determines if a model should have TTL
func (cg *ConfigGenerator) shouldAddTTL(model ModelInfo) bool {
	switch model.Size {
	case "32B", "70B", "405B":
		return true
	}
	return false
}

// hasMultipleGPUs checks if system has multiple GPUs
func (cg *ConfigGenerator) hasMultipleGPUs() bool {
	// This is a simple heuristic - in a real implementation,
	// you'd check nvidia-smi or similar
	return cg.System.HasCUDA
}

// getGPUCount returns estimated GPU count
func (cg *ConfigGenerator) getGPUCount() int {
	if !cg.System.HasCUDA {
		return 1
	}

	// Try to get actual GPU count using nvidia-smi
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Check Windows paths for nvidia-smi
		paths := []string{
			"C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
			"C:\\Windows\\System32\\nvidia-smi.exe",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				cmd = exec.Command(path, "--list-gpus")
				output, err := cmd.Output()
				if err == nil {
					// Count GPU lines
					lines := strings.Split(strings.TrimSpace(string(output)), "\n")
					gpuCount := 0
					for _, line := range lines {
						if strings.Contains(line, "GPU") {
							gpuCount++
						}
					}
					if gpuCount > 0 {
						return gpuCount
					}
				}
				break
			}
		}
	} else {
		// Unix systems
		if _, err := os.Stat("/usr/bin/nvidia-smi"); err == nil {
			cmd = exec.Command("nvidia-smi", "--list-gpus")
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				gpuCount := 0
				for _, line := range lines {
					if strings.Contains(line, "GPU") {
						gpuCount++
					}
				}
				if gpuCount > 0 {
					return gpuCount
				}
			}
		}
	}

	// Fallback - assume 1 GPU if detection fails
	return 1
}

// generateGroupsConfig creates groups configuration for multiple models
func (cg *ConfigGenerator) generateGroupsConfig(models []ModelInfo) string {
	config := strings.Builder{}
	config.WriteString("groups:\n")

	// Create size-based groups
	config.WriteString("  \"large-models\":\n")
	config.WriteString("    swap: true\n")
	config.WriteString("    exclusive: true\n")
	config.WriteString("    members:\n")

	for _, model := range models {
		if model.IsDraft {
			continue
		}
		switch model.Size {
		case "32B", "70B", "405B":
			config.WriteString("      - \"" + cg.generateModelID(model) + "\"\n")
		}
	}

	config.WriteString("\n  \"small-models\":\n")
	config.WriteString("    swap: false\n")
	config.WriteString("    exclusive: false\n")
	config.WriteString("    members:\n")

	for _, model := range models {
		if model.IsDraft {
			continue
		}
		switch model.Size {
		case "0.5B", "1B", "1.5B", "3B", "7B", "8B":
			config.WriteString("      - \"" + cg.generateModelID(model) + "\"\n")
		}
	}

	config.WriteString("\n")
	return config.String()
}

// extractModelFamily extracts the model family name from filename
func extractModelFamily(filename string) string {
	lower := strings.ToLower(filename)

	families := []string{
		"qwen", "llama", "codellama", "mistral", "phi", "gemma", "deepseek", "yi",
	}

	for _, family := range families {
		if strings.Contains(lower, family) {
			return strings.Title(family)
		}
	}

	// Fallback: return first part before number or dash
	parts := strings.FieldsFunc(filename, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})

	if len(parts) > 0 {
		return parts[0]
	}

	return "Unknown"
}

// SaveConfig saves the generated config to a file
func (cg *ConfigGenerator) SaveConfig(configPath string) error {
	config, err := cg.GenerateConfig()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, []byte(config), 0644)
}

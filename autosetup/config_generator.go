package autosetup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// ConfigGenerator generates YAML configuration from detected models and binary
type ConfigGenerator struct {
	Models          []ModelInfo
	Binary          *BinaryInfo
	System          SystemInfo
	StartPort       int
	ModelsDir       string
	MemoryEstimator *MemoryEstimator
	AvailableVRAMGB float64
	Options         SetupOptions
	throughputCache map[string]ThroughputConfig // Cache for throughput optimization results
}

// GenerateConfig creates a complete YAML configuration
func (cg *ConfigGenerator) GenerateConfig() (string, error) {
	// Initialize memory estimator if not set
	if cg.MemoryEstimator == nil {
		cg.MemoryEstimator = NewMemoryEstimator()
	}

	// Detect available VRAM if not set
	if cg.AvailableVRAMGB == 0 {
		vram, err := cg.MemoryEstimator.GetAvailableVRAM()
		if err != nil {
			// Fallback to default if VRAM detection fails
			cg.AvailableVRAMGB = 12.0 // Assume 12GB if detection fails
		} else {
			cg.AvailableVRAMGB = vram
		}
	}

	config := strings.Builder{}

	// Header
	config.WriteString("# Auto-generated llama-swap configuration\n")
	config.WriteString("# Generated from models in: " + cg.ModelsDir + "\n")
	config.WriteString("# Binary: " + cg.Binary.Path + " (" + cg.Binary.Type + ")\n")
	config.WriteString("# System: " + cg.System.OS + "/" + cg.System.Architecture + "\n")
	config.WriteString(fmt.Sprintf("# Available VRAM: %.1f GB\n\n", cg.AvailableVRAMGB))

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
	config.WriteString("    --no-warmup\n")            // Skip warmup for faster first token
	config.WriteString("    --dry-penalty-last-n 0\n") // Disable DRY penalty to avoid slow startup
	config.WriteString("    --batch-size 2048\n")      // Optimal batch size for throughput
	config.WriteString("    --ubatch-size 512\n")      // Micro-batch size for GPU efficiency
	config.WriteString("    --cache-type-k f16\n")     // Use f16 for KV cache (faster than f32)
	config.WriteString("    --cache-type-v f16\n")     // Use f16 for KV cache (faster than f32)

	// Add GPU-specific flags (but not -ngl, that's model-specific)
	// if cg.Binary.Type == "cuda" || cg.Binary.Type == "rocm" {
	//     config.WriteString("    -ngl 99\n")
	// }

	config.WriteString("\n")

	// Sort models by size (largest first)
	sortedModels := SortModelsBySize(cg.Models)

	// Generate models section
	config.WriteString("models:\n")

	currentPort := cg.StartPort
	usedIDs := make(map[string]int) // Track used IDs to handle duplicates

	// Count non-draft models for progress tracking
	nonDraftModels := 0
	for _, model := range sortedModels {
		if !model.IsDraft {
			nonDraftModels++
		}
	}

	if nonDraftModels > 0 {
		fmt.Printf("🔧 Generating configuration for %d models...\n", nonDraftModels)
	}

	processedModels := 0

	for i, model := range sortedModels {
		// Skip draft models from main models list
		if model.IsDraft {
			continue
		}

		processedModels++
		if processedModels%3 == 0 || processedModels == nonDraftModels {
			percentage := float64(processedModels) / float64(nonDraftModels) * 100
			fmt.Printf("   📝 Progress: %d/%d (%.1f%%) models configured\n", processedModels, nonDraftModels, percentage)
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

		// Find a draft model if available and enabled
		var draftModel *ModelInfo
		if cg.Options.EnableDraftModels {
			draftModel = FindDraftModel(cg.Models, model, cg.MemoryEstimator)
		}

		// Generate command
		config.WriteString("    cmd: |\n")
		config.WriteString("      ${llama-server-base}\n")
		config.WriteString("      --model " + model.Path + "\n")

		// Add context size based on optimal memory calculation
		ctxSize := cg.getOptimalContextSize(model)
		config.WriteString("      --ctx-size " + strconv.Itoa(ctxSize) + "\n")

		// Add KV cache quantization for memory optimization (CRITICAL FEATURE)
		kvCacheType := cg.getOptimalKVCacheType(model)
		if kvCacheType != "" && kvCacheType != "f16" {
			config.WriteString("      --cache-type-k " + kvCacheType + "\n")
			config.WriteString("      --cache-type-v " + kvCacheType + "\n")
		}

		// Add Jinja templating support if enabled (uses model's built-in template)
		if cg.Options.EnableJinja {
			config.WriteString("      --jinja\n")
		}

		// Add GPU layer configuration for CUDA/ROCm/Vulkan/Metal
		if cg.Binary.Type == "cuda" || cg.Binary.Type == "rocm" || cg.Binary.Type == "vulkan" || cg.Binary.Type == "metal" {
			nglLayers := cg.getOptimalGPULayers(model)
			config.WriteString("      -ngl " + strconv.Itoa(nglLayers) + "\n")
		}

		// Add backend-specific optimizations
		cg.addBackendOptimizations(&config, model)

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

		// Add sampling parameters based on model type and intended use case
		modelType := cg.detectModelType(model)
		switch modelType {
		case "embedding":
			// Embedding models don't need sampling parameters
			config.WriteString("      --embedding\n")
		case "multimodal":
			// Multimodal models need special handling
			if model.IsInstruct {
				config.WriteString("      --temp 0.7\n")
				config.WriteString("      --repeat-penalty 1.05\n")
				config.WriteString("      --repeat-last-n 256\n")
				config.WriteString("      --top-p 0.9\n")
				config.WriteString("      --top-k 40\n")
				config.WriteString("      --min-p 0.1\n")
			}
		case "instruct":
			// For chat/instruct models - optimized for speculative decoding
			config.WriteString("      --temp 0.7\n") // Lower temp = more predictable = better speculation
			config.WriteString("      --repeat-penalty 1.05\n")
			config.WriteString("      --repeat-last-n 256\n")
			config.WriteString("      --top-p 0.9\n") // More focused sampling helps speculation
			config.WriteString("      --top-k 40\n")
			config.WriteString("      --min-p 0.1\n")
		case "code":
			// Code models benefit most from speculation due to structured syntax
			config.WriteString("      --temp 0.3\n")            // Very low temp for predictable code
			config.WriteString("      --repeat-penalty 1.02\n") // Light penalty for code
			config.WriteString("      --repeat-last-n 128\n")
			config.WriteString("      --top-p 0.95\n")
			config.WriteString("      --min-p 0.05\n")
		case "base":
			// For base/completion models, use moderate sampling
			config.WriteString("      --temp 0.8\n")
			config.WriteString("      --repeat-penalty 1.02\n")
			config.WriteString("      --repeat-last-n 128\n")
			config.WriteString("      --top-p 0.95\n")
			config.WriteString("      --min-p 0.05\n")
		}

		// Add performance optimizations for large models
		if cg.shouldOptimizeForPerformance(model) {
			config.WriteString("      --cont-batching\n") // Continuous batching for throughput
			config.WriteString("      --parallel 4\n")    // Allow multiple parallel requests
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

// getOptimalContextSize returns optimal context size focused on maximizing context length
func (cg *ConfigGenerator) getOptimalContextSize(model ModelInfo) int {
	// Check if throughput-first mode is enabled
	if cg.Options.ThroughputFirst {
		config := cg.optimizeForThroughput(model)
		return config.ContextSize
	}

	// Original maximum context optimization logic for backward compatibility
	modelSizeGB := cg.calculateModelSize(model)
	availableVRAMGB := cg.AvailableVRAMGB

	if availableVRAMGB <= 0 {
		if cg.Options.EnableParallel {
			fmt.Printf("   ⚠️ No VRAM info, using default large context\n")
		}
		return 65536 // Default to 64K when no VRAM info (generous default)
	}

	// Calculate how much memory is left after loading the model
	overheadGB := 0.5 // Minimal overhead for operations
	if model.IsMoE {
		overheadGB = 0.8 // MoE needs more overhead
	}

	remainingGB := availableVRAMGB - modelSizeGB - overheadGB

	if remainingGB <= 0 {
		if cg.Options.EnableParallel {
			fmt.Printf("   ⚠️ Model uses all VRAM, using minimum context\n")
		}
		return 8192 // Minimum viable context
	}

	// Use memory estimator for precise calculation if available
	if cg.MemoryEstimator != nil {
		if cg.Options.EnableParallel {
			fmt.Printf("   🧠 Calculating maximum context for: %s\n", filepath.Base(model.Path))
		}

		// Find the maximum context that fits in VRAM
		optimalContext, err := cg.MemoryEstimator.FindOptimalContextSize(model.Path, cg.AvailableVRAMGB)
		if err == nil && optimalContext > 0 {
			maxContext := optimalContext

			// Cap at model's native context if known
			if model.ContextLength > 0 && maxContext > model.ContextLength {
				maxContext = model.ContextLength
			}

			// Ensure minimum viable context
			if maxContext < 8192 {
				maxContext = 8192
			}

			// Round to standard context sizes, preferring larger contexts
			standardSizes := []int{8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576}
			for i := len(standardSizes) - 1; i >= 0; i-- {
				if maxContext >= standardSizes[i] {
					if cg.Options.EnableParallel {
						fmt.Printf("   ✅ Maximum context: %dK tokens (%.1f GB remaining)\n",
							standardSizes[i]/1024, remainingGB)
					}
					return standardSizes[i]
				}
			}
			return maxContext
		}
	}

	// Enhanced fallback: Calculate maximum context from available memory
	// Use aggressive KV cache quantization (q8_0) to maximize context
	embeddingSize := 4096 // Default assumption
	if model.EmbeddingSize > 0 {
		embeddingSize = model.EmbeddingSize
	}

	// KV cache with q8_0 quantization: context * embedding * 2 (K+V) * 1 byte
	maxTokensFromMemory := int(remainingGB * 1024 * 1024 * 1024 / float64(embeddingSize*2))

	// Apply reasonable limits but prefer larger contexts
	maxContext := maxTokensFromMemory
	if model.ContextLength > 0 {
		maxContext = min(maxContext, model.ContextLength)
	}

	// Cap at 1M tokens (very generous)
	maxContext = min(maxContext, 1048576)

	// Ensure minimum viable context
	maxContext = max(maxContext, 8192)

	// Round to standard sizes, preferring larger contexts
	standardSizes := []int{8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576}
	for i := len(standardSizes) - 1; i >= 0; i-- {
		if maxContext >= standardSizes[i] {
			if cg.Options.EnableParallel {
				fmt.Printf("   💡 Max context: %dK tokens (%.1f GB available, using q8_0 KV cache)\n",
					standardSizes[i]/1024, remainingGB)
			}
			return standardSizes[i]
		}
	}

	// Ultimate fallback - generous default
	return 65536
}

// getOptimalKVCacheType determines the best KV cache quantization for maximum context
func (cg *ConfigGenerator) getOptimalKVCacheType(model ModelInfo) string {
	// Check if throughput-first mode is enabled
	if cg.Options.ThroughputFirst {
		config := cg.optimizeForThroughput(model)
		return config.KVCacheType
	}

	// ALWAYS use q8_0 quantization to maximize context length
	// q8_0 provides the best balance of quality and memory savings
	// This allows much larger context sizes while maintaining good performance
	return "q8_0"
}

// calculateModelSize estimates model size in GB
func (cg *ConfigGenerator) calculateModelSize(model ModelInfo) float64 {
	// Try to get actual file size first
	if actualSize := cg.getActualModelSize(model.Path); actualSize > 0 {
		return actualSize
	}

	// Fallback to estimation
	return cg.estimateModelSizeFromMetadata(model)
}

// getActualModelSize gets the actual file size, handling multi-part models
func (cg *ConfigGenerator) getActualModelSize(modelPath string) float64 {
	if modelPath == "" {
		return 0
	}

	// Check for multi-part model pattern
	if strings.Contains(modelPath, "-of-") {
		return cg.getMultiPartModelSize(modelPath)
	}

	// Single file
	if fileInfo, err := os.Stat(modelPath); err == nil {
		return float64(fileInfo.Size()) / (1024 * 1024 * 1024)
	}

	return 0
}

// getMultiPartModelSize calculates total size of multi-part models
func (cg *ConfigGenerator) getMultiPartModelSize(modelPath string) float64 {
	// Extract base path and part info
	re := regexp.MustCompile(`(.*)-(\d+)-of-(\d+)\.gguf$`)
	matches := re.FindStringSubmatch(modelPath)
	if len(matches) < 4 {
		return 0
	}

	basePath := matches[1]
	totalPartsStr := matches[3]
	totalParts, err := strconv.Atoi(totalPartsStr)
	if err != nil {
		return 0
	}

	var totalSize float64
	for i := 1; i <= totalParts; i++ {
		partPath := fmt.Sprintf("%s-%05d-of-%s.gguf", basePath, i, totalPartsStr)
		if fileInfo, err := os.Stat(partPath); err == nil {
			totalSize += float64(fileInfo.Size()) / (1024 * 1024 * 1024)
		}
	}

	return totalSize
}

// estimateModelSizeFromMetadata estimates size from model metadata
func (cg *ConfigGenerator) estimateModelSizeFromMetadata(model ModelInfo) float64 {
	// Use quantization info if available
	quantMultiplier := 0.5 // Default for Q4 quantization

	if model.Quantization != "" {
		quantMap := map[string]float64{
			"F32": 4.0, "F16": 2.0, "Q8_0": 1.0, "Q6_K": 0.75,
			"Q5_K_M": 0.625, "Q5_K_S": 0.625, "Q4_K_M": 0.5, "Q4_K_S": 0.5,
			"Q4_0": 0.5, "Q3_K_M": 0.375, "Q3_K_S": 0.375, "Q2_K": 0.25,
		}
		if mult, exists := quantMap[model.Quantization]; exists {
			quantMultiplier = mult
		}
	}

	// Estimate based on size if available
	switch model.Size {
	case "0.5B", "1B":
		return 1.0 * quantMultiplier
	case "1.5B", "3B":
		return 3.0 * quantMultiplier
	case "7B", "8B":
		return 7.0 * quantMultiplier
	case "13B":
		return 13.0 * quantMultiplier
	case "32B":
		return 32.0 * quantMultiplier
	case "70B":
		return 70.0 * quantMultiplier
	case "405B":
		return 405.0 * quantMultiplier
	default:
		return 7.0 * quantMultiplier // Default to 7B equivalent
	}
}

// getOptimalGPULayers calculates optimal GPU layers to use all available VRAM efficiently
func (cg *ConfigGenerator) getOptimalGPULayers(model ModelInfo) int {
	// Check if throughput-first mode is enabled
	if cg.Options.ThroughputFirst {
		config := cg.optimizeForThroughput(model)
		return config.GPULayers
	}

	// Original maximum layers optimization logic for backward compatibility
	modelSizeGB := cg.calculateModelSize(model)
	availableVRAMGB := cg.AvailableVRAMGB

	if availableVRAMGB <= 0 || modelSizeGB <= 0 {
		// Fallback to conservative approach
		return cg.getFallbackGPULayers(model)
	}

	// Calculate memory requirements
	overheadGB := 0.3 // Minimal overhead for operations
	if model.IsMoE {
		overheadGB = 0.5 // MoE models need slightly more overhead
	}

	// Estimate KV cache memory using q8_0 quantization (our standard)
	ctxSize := cg.getOptimalContextSize(model)
	kvCacheGB := cg.estimateKVCacheMemory(ctxSize, model.EmbeddingSize, "q8_0")

	// Calculate usable VRAM for model layers - use 95% of total VRAM aggressively
	usableVRAMForModel := availableVRAMGB*0.95 - kvCacheGB - overheadGB

	if usableVRAMForModel <= 0 {
		if cg.Options.EnableParallel {
			fmt.Printf("   ⚠️ Not enough VRAM for GPU layers after KV cache\n")
		}
		return 0 // CPU only
	}

	// Try to use memory estimator for precise calculation
	if cg.MemoryEstimator != nil {
		layerResult, err := cg.MemoryEstimator.CalculateOptimalLayers(model.Path, cg.AvailableVRAMGB, ctxSize)
		if err == nil && layerResult != nil {
			if cg.Options.EnableParallel {
				fmt.Printf("   🎯 Memory estimator: %d GPU layers (using %.1f%% VRAM)\n",
					layerResult.GPULayers, 95.0)
			}
			return layerResult.GPULayers
		}
	}

	// Advanced calculation: determine how many layers can fit
	totalLayers := model.NumLayers
	if totalLayers == 0 {
		totalLayers = cg.estimateLayerCount(model)
	}

	// Calculate layer ratio based on usable VRAM
	layerRatio := usableVRAMForModel / modelSizeGB

	// For MoE models, we can fit more layers due to sparse activation
	if model.IsMoE {
		layerRatio *= 1.15 // 15% bonus for MoE efficiency
	}

	// Calculate GPU layers with aggressive approach
	gpuLayers := int(float64(totalLayers) * layerRatio)

	// Apply constraints
	if gpuLayers > totalLayers {
		gpuLayers = totalLayers // Can't exceed total layers
	}

	// For very small models that fit entirely in GPU, use -ngl 99
	if modelSizeGB <= availableVRAMGB*0.6 { // Model uses less than 60% of VRAM
		gpuLayers = 99
	}

	// Use ALL layers if we're close (within 2 layers) or if model fits completely
	if gpuLayers >= totalLayers-2 && totalLayers > 10 {
		gpuLayers = 99 // Use -ngl 99 for complete GPU offloading
	} else if gpuLayers >= totalLayers {
		gpuLayers = 99 // Model fits completely, use -ngl 99
	}

	// Additional check for models smaller than 8GB that can fit entirely
	if modelSizeGB <= 8.0 && (gpuLayers >= int(float64(totalLayers)*0.9)) {
		gpuLayers = 99 // Use -ngl 99 for models that fit almost entirely
	}

	// Ensure we use at least some GPU if we have decent VRAM
	if gpuLayers < 1 && gpuLayers != 99 && availableVRAMGB > 6 {
		gpuLayers = max(1, totalLayers/4) // Use at least 25% of layers
	}

	if cg.Options.EnableParallel {
		vramUsage := (float64(gpuLayers)/float64(totalLayers))*modelSizeGB + kvCacheGB + overheadGB
		if gpuLayers == 99 {
			fmt.Printf("   🚀 GPU layers: -ngl 99 (complete GPU offloading, %.1f%% VRAM usage)\n",
				vramUsage/availableVRAMGB*100)
		} else {
			fmt.Printf("   🎯 GPU layers: %d/%d (%.1f%% of model, %.1f%% VRAM usage)\n",
				gpuLayers, totalLayers,
				float64(gpuLayers)/float64(totalLayers)*100,
				vramUsage/availableVRAMGB*100)
		}
	}

	return gpuLayers
} // estimateKVCacheMemory estimates KV cache memory usage in GB
func (cg *ConfigGenerator) estimateKVCacheMemory(contextSize int, embeddingSize int, kvCacheType string) float64 {
	if embeddingSize == 0 {
		embeddingSize = 4096 // Default fallback
	}

	// Bytes per element based on quantization
	var bytesPerElement float64 = 2.0 // f16 default
	switch kvCacheType {
	case "q8_0":
		bytesPerElement = 1.0
	case "q4_0", "q4_1":
		bytesPerElement = 0.5
	case "f16":
		bytesPerElement = 2.0
	case "f32":
		bytesPerElement = 4.0
	}

	// KV cache size: context * embedding * 2 (K+V) * bytes_per_element
	kvCacheBytes := float64(contextSize*embeddingSize*2) * bytesPerElement

	// Add some overhead for cache management
	return (kvCacheBytes * 1.1) / (1024 * 1024 * 1024) // Convert to GB
}

// estimateLayerCount estimates the number of layers based on model metadata
func (cg *ConfigGenerator) estimateLayerCount(model ModelInfo) int {
	// Try to estimate from model size
	switch model.Size {
	case "0.5B", "1B", "1.5B":
		return 22
	case "3B":
		return 26
	case "7B", "8B":
		return 32
	case "13B":
		return 40
	case "32B":
		return 60
	case "70B":
		return 80
	case "405B":
		return 126
	default:
		return 32 // Default fallback
	}
}

// getFallbackGPULayers provides size-based heuristics when memory estimation fails
func (cg *ConfigGenerator) getFallbackGPULayers(model ModelInfo) int {
	// Try to use memory estimator for optimal layer calculation
	if cg.MemoryEstimator != nil && cg.AvailableVRAMGB > 0 {
		// Get optimal context size first
		ctxSize := cg.getOptimalContextSize(model)

		// Try layer offloading analysis
		offloadResult, err := cg.MemoryEstimator.FindOptimalContextSizeWithOffload(model.Path, cg.AvailableVRAMGB)
		if err == nil && offloadResult != nil {
			return offloadResult.GPULayers
		}

		// Fallback: try calculating layers for the context size
		layerResult, err := cg.MemoryEstimator.CalculateOptimalLayers(model.Path, cg.AvailableVRAMGB, ctxSize)
		if err == nil && layerResult != nil {
			return layerResult.GPULayers
		}
	}

	// Final fallback to size-based heuristics
	switch model.Size {
	case "0.5B", "1B", "1.5B", "3B", "7B", "8B":
		return 99 // All layers for small models
	case "13B":
		if cg.AvailableVRAMGB >= 16 {
			return 99 // All layers if enough VRAM
		}
		return 32 // Partial offloading
	case "32B":
		if cg.AvailableVRAMGB >= 32 {
			return 99
		}
		return 24
	case "70B", "405B":
		if cg.AvailableVRAMGB >= 64 {
			return 99
		}
		return 16 // Heavy offloading for very large models
	default:
		// For unknown sizes, be conservative
		if cg.AvailableVRAMGB >= 24 {
			return 99
		}
		return 24
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

// addBackendOptimizations adds backend-specific performance and compatibility optimizations
func (cg *ConfigGenerator) addBackendOptimizations(config *strings.Builder, model ModelInfo) {
	switch cg.Binary.Type {
	case "vulkan":
		// Vulkan-specific optimizations - CRITICAL for avoiding OOM and crashes
		config.WriteString("      --no-mmap\n")         // Vulkan may have issues with mmap
		config.WriteString("      --batch-size 512\n")  // Conservative batch size for Vulkan
		config.WriteString("      --ubatch-size 256\n") // Conservative micro-batch

		// Vulkan doesn't support flash attention
		// Note: flash attention is handled by base macro, but we can't disable it per model easily

		// Reduce parallelism for Vulkan stability
		config.WriteString("      --parallel 2\n")

	case "metal":
		// Metal (Apple Silicon) optimizations
		config.WriteString("      --no-mmap\n")         // Metal unified memory works better without mmap
		config.WriteString("      --batch-size 1024\n") // Metal can handle larger batches
		config.WriteString("      --ubatch-size 512\n")

		// Metal supports flash attention and benefits from it
		config.WriteString("      --flash-attn\n")

	case "rocm":
		// ROCm (AMD GPU) optimizations
		config.WriteString("      --batch-size 1024\n") // ROCm can handle good batch sizes
		config.WriteString("      --ubatch-size 512\n")

		// ROCm supports flash attention on newer cards
		if cg.System.HasROCm {
			config.WriteString("      --flash-attn\n")
		}

	case "cuda":
		// CUDA optimizations
		modelSizeGB := cg.calculateModelSize(model)

		// Enable flash attention for modern GPUs (compute capability 8.0+)
		// This is already in the base macro, but we can add model-specific overrides

		// For large models on CUDA, enable advanced features
		if modelSizeGB > 20 {
			config.WriteString("      --cont-batching\n")    // Continuous batching for large models
			config.WriteString("      --defrag-thold 0.1\n") // Aggressive defragmentation
		}

		// CUDA can handle larger batch sizes efficiently
		if modelSizeGB < 10 {
			config.WriteString("      --batch-size 2048\n")
			config.WriteString("      --ubatch-size 512\n")
		} else {
			config.WriteString("      --batch-size 1024\n")
			config.WriteString("      --ubatch-size 256\n")
		}

	case "cpu":
		// CPU-only optimizations
		config.WriteString("      -ngl 0\n")           // Force CPU mode
		config.WriteString("      --batch-size 512\n") // Reasonable batch for CPU
		config.WriteString("      --ubatch-size 128\n")
		config.WriteString("      --threads " + strconv.Itoa(runtime.NumCPU()*2/3) + "\n")

		// CPU benefits from mlock for better performance
		config.WriteString("      --mlock\n")
	}

	// Model-specific optimizations
	if model.IsMoE {
		// MoE models benefit from specific settings
		config.WriteString("      --split-mode row\n") // Better for expert distribution

		// More conservative batching for MoE due to expert routing complexity
		if cg.Binary.Type == "cuda" || cg.Binary.Type == "rocm" {
			config.WriteString("      --batch-size 512\n")
			config.WriteString("      --ubatch-size 128\n")
		}
	}

	// Large model optimizations
	modelSizeGB := cg.calculateModelSize(model)
	if modelSizeGB > 30 {
		config.WriteString("      --keep 1024\n") // Conservative keep for large models
	} else if modelSizeGB > 10 {
		config.WriteString("      --keep 2048\n") // Moderate keep for medium models
	} else {
		config.WriteString("      --keep 4096\n") // Generous keep for small models
	}
}

// shouldOptimizeForPerformance determines if a model needs performance optimizations
func (cg *ConfigGenerator) shouldOptimizeForPerformance(model ModelInfo) bool {
	// Enable performance optimizations for larger models or when VRAM is limited
	switch model.Size {
	case "13B", "32B", "70B", "405B":
		return true
	}

	// Also enable for models that require offloading
	if cg.MemoryEstimator != nil && cg.AvailableVRAMGB > 0 {
		// Check if model requires layer offloading
		memInfo, err := cg.MemoryEstimator.GetModelMemoryInfo(model.Path)
		if err == nil {
			minMemory := memInfo.ModelSizeGB + cg.MemoryEstimator.OverheadGB
			if minMemory > cg.AvailableVRAMGB {
				return true // Needs offloading, so enable performance opts
			}
		}
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

	// Track added model IDs to prevent duplicates
	largeModels := make(map[string]bool)
	smallModels := make(map[string]bool)

	// Large models group - all non-embedding models need swap: true
	config.WriteString("  \"large-models\":\n")
	config.WriteString("    swap: true\n")
	config.WriteString("    exclusive: true\n")
	config.WriteString("    members:\n")

	for _, model := range models {
		if model.IsDraft {
			continue
		}
		// All models except embeddings go to large-models
		modelType := cg.detectModelType(model)
		if modelType != "embedding" {
			modelID := cg.generateModelID(model)
			if !largeModels[modelID] {
				config.WriteString("      - \"" + modelID + "\"\n")
				largeModels[modelID] = true
			}
		}
	}

	// Small models group - only embedding models that can run together
	config.WriteString("\n  \"small-models\":\n")
	config.WriteString("    swap: false\n")
	config.WriteString("    exclusive: false\n")
	config.WriteString("    members:\n")

	for _, model := range models {
		if model.IsDraft {
			continue
		}
		// Only embedding models go to small-models
		modelType := cg.detectModelType(model)
		if modelType == "embedding" {
			modelID := cg.generateModelID(model)
			if !smallModels[modelID] {
				config.WriteString("      - \"" + modelID + "\"\n")
				smallModels[modelID] = true
			}
		}
	}

	config.WriteString("\n")
	return config.String()
}

// detectModelType determines the type of model for appropriate configuration
func (cg *ConfigGenerator) detectModelType(model ModelInfo) string {
	// Check GGUF metadata first
	if model.IsEmbedding {
		return "embedding"
	}

	// Fallback to filename/path detection
	lower := strings.ToLower(model.Name)
	lowerPath := strings.ToLower(model.Path)

	// Check for embedding models by name and path
	if strings.Contains(lower, "embed") || strings.Contains(lower, "embedding") ||
		strings.Contains(lowerPath, "embed") || strings.Contains(lowerPath, "embedding") ||
		strings.Contains(lower, "mxbai") || // mxbai models are embeddings
		strings.Contains(lower, "bge-") || // BGE embedding models
		strings.Contains(lower, "e5-") { // E5 embedding models
		return "embedding"
	}

	// Check for code models
	if strings.Contains(lower, "code") || strings.Contains(lower, "coder") ||
		strings.Contains(lower, "programming") || strings.Contains(lower, "codellama") ||
		strings.Contains(lower, "starcoder") || strings.Contains(lower, "deepseek-coder") {
		return "code"
	}

	// Check for multimodal models (vision capabilities)
	if strings.Contains(lower, "vision") || strings.Contains(lower, "mmproj") ||
		strings.Contains(lower, "internvl") || strings.Contains(lower, "llava") ||
		strings.Contains(lower, "minicpm") {
		return "multimodal"
	}

	// Check for instruct/chat models
	if model.IsInstruct || strings.Contains(lower, "chat") || strings.Contains(lower, "instruct") ||
		strings.Contains(lower, "tools") || strings.Contains(lower, "-it") ||
		strings.Contains(lower, "assistant") {
		return "instruct"
	}

	// Default to base model
	return "base"
}

// extractModelFamily extracts the model family name from filename
func extractModelFamily(filename string) string {
	lower := strings.ToLower(filename)

	families := []string{
		"qwen", "llama", "codellama", "mistral", "phi", "gemma", "deepseek", "yi",
	}

	for _, family := range families {
		if strings.Contains(lower, family) {
			// Capitalize first letter manually since strings.Title is deprecated
			if len(family) > 0 {
				return strings.ToUpper(string(family[0])) + family[1:]
			}
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

// ThroughputConfig represents a throughput-optimized configuration attempt
type ThroughputConfig struct {
	ContextSize   int
	KVCacheType   string
	GPULayers     int
	EstimatedVRAM float64
	Priority      int // Lower is higher priority
}

// optimizeForThroughput implements size-first allocation strategy for maximum performance
func (cg *ConfigGenerator) optimizeForThroughput(model ModelInfo) ThroughputConfig {
	// Initialize cache if needed
	if cg.throughputCache == nil {
		cg.throughputCache = make(map[string]ThroughputConfig)
	}

	// Check cache first
	if cached, exists := cg.throughputCache[model.Path]; exists {
		return cached
	}

	if cg.Options.EnableParallel {
		fmt.Printf("   🚀 THROUGHPUT MODE: Optimizing %s\n", model.Name)
	}

	availableVRAM := cg.AvailableVRAMGB * 0.95 // Use 95% for aggressive optimization
	modelSizeGB := cg.calculateModelSize(model)

	if cg.Options.EnableParallel {
		fmt.Printf("   📊 Model size: %.1f GB, Available VRAM: %.1f GB\n", modelSizeGB, availableVRAM)
	}

	var result ThroughputConfig

	// PHASE 1: Check if model fits entirely in VRAM
	if modelSizeGB <= availableVRAM {
		result = cg.optimizeModelThatFitsInVRAM(model, modelSizeGB, availableVRAM)
	} else {
		// PHASE 2: Model doesn't fit - use hybrid CPU/GPU strategy
		result = cg.optimizeOversizedModel(model, modelSizeGB, availableVRAM)
	}

	// Cache the result
	cg.throughputCache[model.Path] = result
	return result
}

// estimateVRAMUsage calculates approximate VRAM usage for a configuration
func (cg *ConfigGenerator) estimateVRAMUsage(model ModelInfo, config ThroughputConfig) float64 {
	// Get model size estimation
	modelSizeGB := cg.calculateModelSize(model)

	// Calculate KV cache size based on context and quantization
	kvMultiplier := 1.0
	switch config.KVCacheType {
	case "q8_0":
		kvMultiplier = 0.5 // 50% savings vs f16
	case "q4_0":
		kvMultiplier = 0.25 // 75% savings vs f16
	default:
		kvMultiplier = 1.0 // f16 baseline
	}

	// Rough KV cache calculation (simplified)
	// Real calculation would consider heads, dimensions, layers, etc.
	contextSizeMB := float64(config.ContextSize) * 0.002 // ~2KB per token baseline
	kvCacheGB := (contextSizeMB / 1024) * kvMultiplier

	// Overhead for CUDA kernels, fragmentation, etc.
	overheadGB := 1.0

	return modelSizeGB + kvCacheGB + overheadGB
}

// optimizeModelThatFitsInVRAM optimizes models that fit entirely in VRAM
func (cg *ConfigGenerator) optimizeModelThatFitsInVRAM(model ModelInfo, modelSizeGB, availableVRAM float64) ThroughputConfig {
	// Model fits completely - use -ngl 999 (all layers on GPU)
	remainingVRAM := availableVRAM - modelSizeGB - 0.5 // 0.5GB overhead

	if cg.Options.EnableParallel {
		fmt.Printf("   🚀 Model fits entirely in VRAM! Using -ngl 999, %.1f GB left for context\n", remainingVRAM)
	}

	// Now maximize context with the remaining VRAM
	// Try different KV cache quantizations: f16 → q8_0 → q4_0
	kvStrategies := []struct {
		name       string
		multiplier float64 // Memory multiplier vs f16 baseline
	}{
		{"f16", 1.0},   // Baseline
		{"q8_0", 0.5},  // 50% savings
		{"q4_0", 0.25}, // 75% savings
	}

	for _, kv := range kvStrategies {
		maxContext := cg.calculateMaxContextForKVCache(remainingVRAM, kv.multiplier)

		// Check if we can achieve minimum 16K context
		if maxContext >= cg.Options.MinContext {
			// Try for preferred 32K if possible
			targetContext := cg.Options.PreferredContext
			if maxContext < cg.Options.PreferredContext {
				targetContext = maxContext
			}

			// Validate parallel processing requirements
			finalContext := cg.validateParallelContext(targetContext)

			if cg.Options.EnableParallel {
				fmt.Printf("   ✅ Optimized: %dK context, %s KV cache, -ngl 999\n",
					finalContext/1024, kv.name)
			}

			return ThroughputConfig{
				ContextSize:   finalContext,
				KVCacheType:   kv.name,
				GPULayers:     999, // All layers on GPU
				EstimatedVRAM: modelSizeGB + cg.estimateKVCacheSize(finalContext, kv.multiplier) + 0.5,
				Priority:      1, // Highest priority
			}
		}
	}

	// Fallback: even q4_0 can't reach minimum - use tightest packing possible
	maxContext := cg.calculateMaxContextForKVCache(remainingVRAM, 0.25) // q4_0
	finalContext := cg.validateParallelContext(max(8192, maxContext))   // Minimum 8K

	if cg.Options.EnableParallel {
		fmt.Printf("   ⚠️ Tight packing: %dK context, q4_0 KV cache, -ngl 999 (below minimum)\n",
			finalContext/1024)
	}

	return ThroughputConfig{
		ContextSize:   finalContext,
		KVCacheType:   "q4_0",
		GPULayers:     999,
		EstimatedVRAM: availableVRAM * 0.98, // Use almost all VRAM
		Priority:      2,
	}
}

// optimizeOversizedModel handles models that exceed VRAM capacity
func (cg *ConfigGenerator) optimizeOversizedModel(model ModelInfo, modelSizeGB, availableVRAM float64) ThroughputConfig {
	if cg.Options.EnableParallel {
		fmt.Printf("   ⚡ Model exceeds VRAM, using hybrid CPU/GPU strategy\n")
	}

	// Reserve space for KV cache (use q8_0 as default for hybrid)
	kvCacheSize := cg.estimateKVCacheSize(cg.Options.PreferredContext, 0.5) // q8_0
	overheadGB := 0.5

	availableForModel := availableVRAM - kvCacheSize - overheadGB

	if availableForModel <= 0 {
		// Not enough VRAM even for KV cache - reduce context
		kvCacheSize = cg.estimateKVCacheSize(cg.Options.MinContext, 0.5)
		availableForModel = availableVRAM - kvCacheSize - overheadGB
	}

	// Calculate how many layers can fit on GPU
	layerCount := 32 // Default assumption
	if model.NumLayers > 0 {
		layerCount = model.NumLayers
	}

	layersPerGB := float64(layerCount) / modelSizeGB
	maxGPULayers := int(availableForModel * layersPerGB)

	// Ensure we use at least some GPU layers
	maxGPULayers = max(8, maxGPULayers)
	maxGPULayers = min(maxGPULayers, layerCount)

	contextSize := cg.Options.PreferredContext
	if kvCacheSize > availableVRAM*0.3 {
		contextSize = cg.Options.MinContext
	}

	finalContext := cg.validateParallelContext(contextSize)

	if cg.Options.EnableParallel {
		fmt.Printf("   🔄 Hybrid: %dK context, q8_0 KV, %d/%d layers on GPU\n",
			finalContext/1024, maxGPULayers, layerCount)
	}

	return ThroughputConfig{
		ContextSize:   finalContext,
		KVCacheType:   "q8_0",
		GPULayers:     maxGPULayers,
		EstimatedVRAM: availableVRAM * 0.9,
		Priority:      3, // Lower priority (hybrid)
	}
}

// calculateMaxContextForKVCache calculates maximum context size for given VRAM and KV quantization
func (cg *ConfigGenerator) calculateMaxContextForKVCache(availableVRAM, kvMultiplier float64) int {
	// Rough estimation: each token uses ~2 bytes for K+V caches in f16
	// With quantization, multiply by kvMultiplier
	bytesPerToken := 2.0 * kvMultiplier

	// Convert available VRAM to bytes and calculate tokens
	availableBytes := availableVRAM * 1024 * 1024 * 1024
	maxTokens := int(availableBytes / bytesPerToken)

	// Round down to standard context sizes
	standardSizes := []int{8192, 16384, 32768, 65536, 131072, 262144, 524288}
	for i := len(standardSizes) - 1; i >= 0; i-- {
		if maxTokens >= standardSizes[i] {
			return standardSizes[i]
		}
	}

	return 8192 // Minimum fallback
}

// estimateKVCacheSize estimates KV cache memory usage
func (cg *ConfigGenerator) estimateKVCacheSize(contextSize int, kvMultiplier float64) float64 {
	bytesPerToken := 2.0 * kvMultiplier // K+V caches
	totalBytes := float64(contextSize) * bytesPerToken
	return totalBytes / (1024 * 1024 * 1024) // Convert to GB
}

// validateParallelContext ensures parallel processing requirements are met
func (cg *ConfigGenerator) validateParallelContext(contextSize int) int {
	// When using --parallel 4, ensure ctx_size/parallel >= 5000
	// to prevent preempt taking more tokens than available
	if cg.hasParallelProcessing() {
		parallelSlots := 4 // Default parallel value
		minRequiredContext := parallelSlots * 5000

		if contextSize < minRequiredContext {
			// Adjust context upward to meet parallel requirements
			adjustedContext := minRequiredContext

			// Round up to nearest standard size
			standardSizes := []int{8192, 16384, 32768, 65536, 131072, 262144}
			for _, size := range standardSizes {
				if size >= adjustedContext {
					if cg.Options.EnableParallel {
						fmt.Printf("   ⚡ Parallel processing: adjusted context %dK → %dK (min %d per slot)\n",
							contextSize/1024, size/1024, 5000)
					}
					return size
				}
			}
		}
	}

	return contextSize
}

// hasParallelProcessing checks if parallel processing is being used
func (cg *ConfigGenerator) hasParallelProcessing() bool {
	// Check if model should use parallel processing (large models)
	return cg.Options.EnableParallel && cg.shouldOptimizeForPerformance(ModelInfo{})
}

// fallbackHybridConfig creates a hybrid CPU/GPU configuration when full GPU doesn't fit
func (cg *ConfigGenerator) fallbackHybridConfig(model ModelInfo, availableVRAM float64) ThroughputConfig {
	// Try to fit the preferred context with partial GPU layers
	contextSize := cg.Options.PreferredContext
	kvCacheType := "q8_0"

	// Calculate how many layers can fit
	modelSizeGB := cg.calculateModelSize(model)
	kvCacheGB := (float64(contextSize) * 0.002 / 1024) * 0.5 // q8_0 KV cache
	overheadGB := 1.0

	availableForModel := availableVRAM - kvCacheGB - overheadGB
	layerCount := 32 // Default assumption, would be better to get from GGUF metadata

	// Estimate how many layers can fit
	layersPerGB := float64(layerCount) / modelSizeGB
	maxLayers := int(availableForModel * layersPerGB)

	// Ensure we use at least some GPU layers
	if maxLayers < 8 {
		// If too few layers, reduce context and try again
		contextSize = cg.Options.MinContext
		kvCacheGB = (float64(contextSize) * 0.002 / 1024) * 0.5
		availableForModel = availableVRAM - kvCacheGB - overheadGB
		maxLayers = int(availableForModel * layersPerGB)
	}

	if cg.Options.EnableParallel {
		fmt.Printf("   ⚡ Hybrid config: %dK context, %s KV cache, -ngl %d (partial GPU)\n",
			contextSize/1024, kvCacheType, maxLayers)
	}

	return ThroughputConfig{
		ContextSize:   contextSize,
		KVCacheType:   kvCacheType,
		GPULayers:     maxLayers,
		EstimatedVRAM: availableVRAM * 0.9, // Use most of available VRAM
		Priority:      99,                  // Lowest priority (fallback)
	}
}

// estimateModelSize provides a rough estimation of model size in GB
func (cg *ConfigGenerator) estimateModelSize(model ModelInfo) float64 {
	// Parse size from model.Size field (e.g., "3B", "7B", "13B")
	sizeStr := strings.TrimSuffix(model.Size, "B")
	if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
		// Rough estimation based on quantization
		switch {
		case strings.Contains(model.Quantization, "Q4"):
			return size * 0.6 // Q4 is ~60% of original size
		case strings.Contains(model.Quantization, "Q5"):
			return size * 0.7 // Q5 is ~70% of original size
		case strings.Contains(model.Quantization, "Q8"):
			return size * 0.8 // Q8 is ~80% of original size
		default:
			return size * 0.6 // Default to Q4 estimation
		}
	}

	// Fallback: try to get actual file size
	if cg.MemoryEstimator != nil {
		if memInfo, err := cg.MemoryEstimator.GetModelMemoryInfo(model.Path); err == nil {
			return memInfo.ModelSizeGB
		}
	}

	return 4.0 // Default fallback
}

// SaveConfig saves the generated config to a file
func (cg *ConfigGenerator) SaveConfig(configPath string) error {
	config, err := cg.GenerateConfig()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, []byte(config), 0644)
}

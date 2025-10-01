package autosetup

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// ConfigGenerator generates optimized configurations with intelligent GPU allocation
type ConfigGenerator struct {
	ModelsPath    string
	BinaryPath    string
	BinaryType    string
	OutputPath    string
	Options       SetupOptions
	TotalVRAMGB   float64
	SystemInfo    *SystemInfo    // Add system info for optimal parameters
	usedModelIDs  map[string]int // Track used model IDs and their counts
	mmprojMatches []MMProjMatch  // Store mmproj matches for automatic --mmproj parameter addition
}

// NewConfigGenerator creates a new config generator
func NewConfigGenerator(modelsPath, binaryPath, outputPath string, options SetupOptions) *ConfigGenerator {
	return &ConfigGenerator{
		ModelsPath:   modelsPath,
		BinaryPath:   binaryPath,
		OutputPath:   outputPath,
		Options:      options,
		usedModelIDs: make(map[string]int),
	}
}

// SetAvailableVRAM sets the total VRAM in GB
func (scg *ConfigGenerator) SetAvailableVRAM(vramGB float64) {
	scg.TotalVRAMGB = vramGB
}

// SetBinaryType sets the binary type (cuda, rocm, cpu)
func (scg *ConfigGenerator) SetBinaryType(binaryType string) {
	scg.BinaryType = binaryType
}

// SetMMProjMatches sets the mmproj matches for automatic --mmproj parameter addition
func (scg *ConfigGenerator) SetMMProjMatches(matches []MMProjMatch) {
	scg.mmprojMatches = matches
}

// SetSystemInfo sets the system information for optimal parameter calculation
func (scg *ConfigGenerator) SetSystemInfo(systemInfo *SystemInfo) {
	scg.SystemInfo = systemInfo
}

// GenerateConfig generates a simple configuration file
func (scg *ConfigGenerator) GenerateConfig(models []ModelInfo) error {
	pm := GetProgressManager()
	pm.UpdateStatus("generating")
	pm.UpdateStep("Starting configuration generation...")

	// Use real-time hardware monitoring if enabled
	if scg.Options.EnableRealtime {
		pm.UpdateStep("Checking real-time hardware info...")
		fmt.Println("🔄 Real-time hardware monitoring enabled...")
		realtimeInfo, err := GetRealtimeHardwareInfo()
		if err != nil {
			fmt.Printf("⚠️  Real-time monitoring failed, using static values: %v\n", err)
		} else {
			PrintRealtimeInfo(realtimeInfo)
			// Update hardware values with real-time data
			scg.TotalVRAMGB = realtimeInfo.AvailableVRAMGB
			if scg.SystemInfo != nil {
				scg.SystemInfo.TotalRAMGB = realtimeInfo.AvailableRAMGB
			} else {
				scg.SystemInfo = &SystemInfo{
					TotalRAMGB: realtimeInfo.AvailableRAMGB,
				}
			}
			fmt.Printf("✅ Using real-time values: %.2f GB VRAM, %.2f GB RAM available\n",
				realtimeInfo.AvailableVRAMGB, realtimeInfo.AvailableRAMGB)
		}
	}

	pm.UpdateStep("Building configuration structure...")
	config := strings.Builder{}

	// Write header
	scg.writeHeader(&config)

	// Write macros
	scg.writeMacros(&config)

	pm.UpdateStep("Processing model configurations...")
	// Generate model IDs consistently (first pass)
	modelIDMap := make(map[string]string)
	validModels := 0
	for _, model := range models {
		if !model.IsDraft {
			validModels++
		}
	}

	processed := 0
	for _, model := range models {
		if model.IsDraft {
			continue
		}
		processed++
		pm.UpdateProgress(processed, validModels, model.Name)
		modelIDMap[model.Path] = scg.generateModelID(model)
	}

	pm.UpdateStep("Writing model definitions...")
	// Write models
	config.WriteString("\nmodels:\n")
	processed = 0
	for _, model := range models {
		if model.IsDraft {
			continue // Skip draft models
		}
		processed++
		pm.UpdateProgress(processed, validModels, model.Name)
		scg.writeModel(&config, model, modelIDMap)
	}

	pm.UpdateStep("Writing model groups...")
	// Write groups
	scg.writeGroups(&config, models, modelIDMap)

	pm.UpdateStep("Saving configuration file...")
	// Save to file
	err := os.WriteFile(scg.OutputPath, []byte(config.String()), 0644)
	if err != nil {
		pm.SetError(fmt.Sprintf("Failed to save config file: %v", err))
		return err
	}

	pm.UpdateStatus("completed")
	return nil
}

// writeHeader writes the configuration header
func (scg *ConfigGenerator) writeHeader(config *strings.Builder) {
	config.WriteString("# Auto-generated Clara Core configuration (SMART GPU ALLOCATION)\n")
	config.WriteString(fmt.Sprintf("# Generated from models in: %s\n", scg.ModelsPath))
	config.WriteString(fmt.Sprintf("# Binary: %s (%s)\n", scg.BinaryPath, scg.BinaryType))
	config.WriteString(fmt.Sprintf("# System: %s/%s\n", runtime.GOOS, runtime.GOARCH))

	if scg.Options.EnableRealtime {
		config.WriteString("# Hardware monitoring: REAL-TIME (current available memory)\n")
		if scg.TotalVRAMGB > 0 {
			config.WriteString(fmt.Sprintf("# Available VRAM: %.1f GB (real-time)\n", scg.TotalVRAMGB))
		}
		if scg.SystemInfo != nil && scg.SystemInfo.TotalRAMGB > 0 {
			config.WriteString(fmt.Sprintf("# Available RAM: %.1f GB (real-time)\n", scg.SystemInfo.TotalRAMGB))
		}
	} else {
		config.WriteString("# Hardware monitoring: STATIC (total memory)\n")
		if scg.TotalVRAMGB > 0 {
			config.WriteString(fmt.Sprintf("# Total GPU VRAM: %.1f GB\n", scg.TotalVRAMGB))
		}
	}

	config.WriteString("# Algorithm: Hybrid VRAM+RAM allocation with intelligent layer distribution\n")
	config.WriteString("\n")
	config.WriteString("healthCheckTimeout: 300\n")
	config.WriteString("logLevel: info\n")
	config.WriteString("startPort: 5800\n")
}

// writeMacros writes the base macros
func (scg *ConfigGenerator) writeMacros(config *strings.Builder) {
	config.WriteString("\nmacros:\n")
	config.WriteString("  \"llama-server-base\": >\n")
	config.WriteString(fmt.Sprintf("    %s\n", scg.BinaryPath))
	config.WriteString("    --host 127.0.0.1\n")
	config.WriteString("    --port ${PORT}\n")
	config.WriteString("    --metrics\n")
	config.WriteString("    --flash-attn auto\n")
	config.WriteString("    --no-warmup\n")
	config.WriteString("    --dry-penalty-last-n 0\n")
	config.WriteString("    --batch-size 2048\n")
	config.WriteString("    --ubatch-size 512\n")
	config.WriteString("\n")
	config.WriteString("  \"llama-embed-base\": >\n")
	config.WriteString(fmt.Sprintf("    %s\n", scg.BinaryPath))
	config.WriteString("    --host 127.0.0.1\n")
	config.WriteString("    --port ${PORT}\n")
	config.WriteString("    --embedding\n")
	// Pooling type will be set per model based on model family
	// KV cache types are now set per model based on optimal calculation
}

// writeModel writes a single model configuration
func (scg *ConfigGenerator) writeModel(config *strings.Builder, model ModelInfo, modelIDMap map[string]string) {
	modelID := modelIDMap[model.Path] // Use pre-generated ID from map

	config.WriteString(fmt.Sprintf("  \"%s\":\n", modelID))

	// Add name and description if available
	if model.Name != "" {
		config.WriteString(fmt.Sprintf("    name: \"%s\"\n", model.Name))
	}

	description := scg.generateDescription(model)
	if description != "" {
		config.WriteString(fmt.Sprintf("    description: \"%s\"\n", description))
	}

	// Write command
	config.WriteString("    cmd: |\n")
	if scg.isEmbeddingModel(model) {
		config.WriteString("      ${llama-embed-base}\n")
	} else {
		config.WriteString("      ${llama-server-base}\n")
	}
	config.WriteString(fmt.Sprintf("      --model %s\n", quotePath(model.Path)))

	// Add --mmproj parameter if a matching mmproj file is found
	mmprojPath := scg.findMatchingMMProj(model.Path)
	if mmprojPath != "" {
		config.WriteString(fmt.Sprintf("      --mmproj %s\n", quotePath(mmprojPath)))
	}

	// Smart GPU layer allocation algorithm (applies to all models including embeddings)
	nglValue := scg.calculateOptimalNGL(model)

	// Get model file info for context calculation
	modelInfo, err := GetModelFileInfo(model.Path)
	modelSizeGB := 20.0 // Default fallback
	if err == nil {
		modelSizeGB = modelInfo.ActualSizeGB
	}

	// Calculate optimal context size and KV cache type for use in optimizations
	optimalContext, kvCacheType := scg.calculateOptimalContext(model, nglValue, modelSizeGB)

	// For embedding models, skip base context and ngl as they'll be handled in writeOptimizations
	if !scg.isEmbeddingModel(model) {
		config.WriteString(fmt.Sprintf("      --ctx-size %d\n", optimalContext))
		config.WriteString(fmt.Sprintf("      -ngl %d\n", nglValue))

		// Set KV cache type
		config.WriteString(fmt.Sprintf("      --cache-type-k %s\n", kvCacheType))
		config.WriteString(fmt.Sprintf("      --cache-type-v %s\n", kvCacheType))
	}

	// Add optimizations
	scg.writeOptimizations(config, model, optimalContext)

	// Add proxy
	config.WriteString("    proxy: \"http://127.0.0.1:${PORT}\"\n")

	// Add environment
	config.WriteString("    env:\n")
	config.WriteString("      - \"CUDA_VISIBLE_DEVICES=0\"\n")
	config.WriteString("\n")
}

// calculateOptimalNGL calculates the optimal number of GPU layers based on model size vs VRAM and system RAM
func (scg *ConfigGenerator) calculateOptimalNGL(model ModelInfo) int {
	// For CPU-only configurations (only return 0 for actual CPU backend)
	if scg.BinaryType == "cpu" {
		return 0
	}

	// Get model file info to get actual size and layer count
	modelInfo, err := GetModelFileInfo(model.Path)
	if err != nil {
		// Fallback to -ngl 999 if we can't read model info
		return 999
	}

	modelSizeGB := modelInfo.ActualSizeGB
	totalLayers := modelInfo.LayerCount

	// If no layer count available, fallback to -ngl 999
	if totalLayers == 0 {
		return 999
	}

	// Reserve VRAM for context and other overhead (2GB)
	reservedVRAM := 2.0
	usableVRAM := scg.TotalVRAMGB - reservedVRAM

	// Get available system RAM (leave 25% buffer for system)
	availableRAM := 0.0
	if scg.SystemInfo != nil && scg.SystemInfo.TotalRAMGB > 0 {
		availableRAM = scg.SystemInfo.TotalRAMGB * 0.75
	}

	fmt.Printf("🧮 Model: %s\n", model.Name)
	fmt.Printf("   Size: %.2f GB, Layers: %d\n", modelSizeGB, totalLayers)
	fmt.Printf("   VRAM: Total %.2f GB, Usable %.2f GB\n", scg.TotalVRAMGB, usableVRAM)
	fmt.Printf("   RAM: Available %.2f GB\n", availableRAM)

	// Check if entire model fits in VRAM
	if modelSizeGB <= usableVRAM {
		fmt.Printf("   ✅ Model fits entirely in VRAM: using -ngl 999 (all layers)\n")
		return 999
	}

	// Check if model fits in VRAM + RAM combined
	if availableRAM > 0 && modelSizeGB <= (usableVRAM+availableRAM) {
		// Hybrid allocation: maximize GPU layers, rest goes to CPU/RAM
		layerSizeGB := modelSizeGB / float64(totalLayers)
		maxGPULayers := int(usableVRAM / layerSizeGB)

		// **CRITICAL OPTIMIZATION**: If only 1-2 layers would be on CPU, force full GPU
		// This avoids the massive performance penalty of hybrid allocation
		cpuLayers := totalLayers - maxGPULayers
		if cpuLayers <= 2 {
			fmt.Printf("   🚀 PERFORMANCE OPTIMIZATION: Only %d CPU layers - forcing full GPU allocation for speed\n", cpuLayers)
			fmt.Printf("   💡 Trading some VRAM overhead for 8x better performance (based on QA testing)\n")
			return 999 // Force all layers to GPU
		}

		// Ensure at least 1 layer on GPU for performance
		if maxGPULayers < 1 {
			maxGPULayers = 1
		}

		// Ensure we don't exceed total layers
		if maxGPULayers > totalLayers {
			maxGPULayers = totalLayers
		}

		cpuMemoryGB := float64(cpuLayers) * layerSizeGB

		fmt.Printf("   🔄 Hybrid allocation: %d GPU layers (%.2f GB), %d CPU layers (%.2f GB)\n",
			maxGPULayers, usableVRAM, cpuLayers, cpuMemoryGB)
		fmt.Printf("   ⚠️  Warning: Hybrid allocation may reduce performance significantly\n")

		return maxGPULayers
	}

	// Model doesn't fit in available memory - warn user but try best effort
	fmt.Printf("   ⚠️  Model (%.2f GB) exceeds available memory (VRAM: %.2f GB + RAM: %.2f GB)\n",
		modelSizeGB, usableVRAM, availableRAM)

	// Best effort: fit as many layers as possible in VRAM
	layerSizeGB := modelSizeGB / float64(totalLayers)
	layersThatFitInVRAM := int(usableVRAM / layerSizeGB)

	// Ensure we don't exceed total layers
	if layersThatFitInVRAM > totalLayers {
		layersThatFitInVRAM = totalLayers
	}

	// Ensure at least 1 layer on GPU if we have any VRAM
	if layersThatFitInVRAM < 1 && usableVRAM > 1.0 {
		layersThatFitInVRAM = 1
	}

	fmt.Printf("   📊 Layer size: %.3f GB each, Fitting %d/%d layers in usable VRAM\n",
		layerSizeGB, layersThatFitInVRAM, totalLayers)
	fmt.Printf("   🎯 Using -ngl %d\n", layersThatFitInVRAM)

	return layersThatFitInVRAM
}

// calculateKVCacheSize calculates VRAM usage for KV cache in GB
func calculateKVCacheSize(contextSize int, layers int, kvCacheType string) float64 {
	// KV cache size calculation: 2 * layers * hiddenSize * contextSize * bytesPerElement
	// Estimate hidden size based on layer count - more accurate approach

	var hiddenSize int
	if layers <= 28 {
		hiddenSize = 2048 // Small models (0.6B-1B)
	} else if layers <= 36 {
		hiddenSize = 3072 // Medium models (3B-7B)
	} else if layers <= 48 {
		hiddenSize = 4096 // Large models (13B-30B)
	} else {
		hiddenSize = 5120 // Very large models (70B+)
	}

	var bytesPerElement float64
	switch kvCacheType {
	case "f16":
		bytesPerElement = 2.0
	case "q8_0":
		bytesPerElement = 1.0
	case "q4_0":
		bytesPerElement = 0.5
	default:
		bytesPerElement = 2.0 // Default to f16
	}

	// Formula: 2 (K + V) * layers * hiddenSize * contextSize * bytesPerElement
	// Only count GPU layers for KV cache calculation
	kvCacheSizeBytes := 2.0 * float64(layers) * float64(hiddenSize) * float64(contextSize) * bytesPerElement
	kvCacheSizeGB := kvCacheSizeBytes / (1024 * 1024 * 1024)

	return kvCacheSizeGB
}

// calculateOptimalContext calculates optimal context size based on remaining VRAM and available system RAM
func (scg *ConfigGenerator) calculateOptimalContext(model ModelInfo, nglLayers int, modelSizeGB float64) (int, string) {
	// Get model info for layer count and SWA support
	modelInfo, err := GetModelFileInfo(model.Path)
	totalModelLayers := 64 // Default fallback
	hasSWA := false
	if err == nil && modelInfo.LayerCount > 0 {
		totalModelLayers = modelInfo.LayerCount
		hasSWA = modelInfo.SlidingWindow > 0
	}

	// Calculate how much VRAM is used by model layers
	var layersOnGPU int
	var layersOnCPU int
	var modelVRAMUsage float64

	if nglLayers == 999 {
		// All layers on GPU
		layersOnGPU = totalModelLayers
		layersOnCPU = 0
		modelVRAMUsage = modelSizeGB
	} else {
		// Partial layers on GPU, rest on CPU
		layersOnGPU = nglLayers
		layersOnCPU = totalModelLayers - nglLayers
		layerSizeGB := modelSizeGB / float64(totalModelLayers)
		modelVRAMUsage = layerSizeGB * float64(nglLayers)
	}

	// Calculate remaining VRAM for KV cache
	remainingVRAM := scg.TotalVRAMGB - modelVRAMUsage - 1.0 // 1GB overhead for operations

	// Calculate available system RAM for hybrid KV cache
	availableRAM := 0.0
	if scg.SystemInfo != nil && scg.SystemInfo.TotalRAMGB > 0 {
		// Reserve RAM for CPU layers and system operations
		cpuLayerRAM := 0.0
		if layersOnCPU > 0 {
			layerSizeGB := modelSizeGB / float64(totalModelLayers)
			cpuLayerRAM = layerSizeGB * float64(layersOnCPU)
		}

		systemRAMBuffer := scg.SystemInfo.TotalRAMGB * 0.25 // 25% buffer for system
		usedRAM := cpuLayerRAM + systemRAMBuffer
		availableRAM = scg.SystemInfo.TotalRAMGB - usedRAM

		if availableRAM < 0 {
			availableRAM = 0
		}
	}

	fmt.Printf("   💾 Model allocation: GPU %.2f GB (%d layers), CPU %.2f GB (%d layers)\n",
		modelVRAMUsage, layersOnGPU, modelSizeGB-modelVRAMUsage, layersOnCPU)
	fmt.Printf("   🎯 Available for KV cache: VRAM %.2f GB, RAM %.2f GB\n",
		remainingVRAM, availableRAM)

	// For SWA models, force f16 KV cache (no quantization)
	// For large hybrid models (>50GB), prioritize q4_0 for performance
	var kvCacheTypes []string
	if hasSWA {
		kvCacheTypes = []string{"f16"} // Only f16 for SWA models
		fmt.Printf("   🪟 SWA detected: using f16 KV cache (no quantization)\n")
	} else if modelSizeGB > 50.0 && layersOnCPU > 0 {
		kvCacheTypes = []string{"q4_0", "q8_0"} // Large hybrid models: prioritize q4_0
		fmt.Printf("   🔧 Large hybrid model: prioritizing q4_0 KV cache for performance\n")
	} else {
		kvCacheTypes = []string{"f16", "q8_0", "q4_0"} // Try all types for other models
	}

	bestContextSize := 4096 // Minimum fallback
	bestKVCacheType := "f16"

	// Get model's maximum context if available
	maxModelContext := 131072 // Default max
	if err == nil && modelInfo.ContextLength > 0 {
		maxModelContext = modelInfo.ContextLength
	}

	// **CRITICAL CHANGE**: Only use hybrid if model doesn't fit entirely in GPU
	useGPUOnly := (nglLayers == 999) // Model fits entirely in GPU

	if useGPUOnly {
		fmt.Printf("   🎯 GPU-only optimization: Model fits entirely in VRAM, maximizing GPU context\n")

		// GPU-ONLY MODE: Maximize context using available VRAM
		for _, kvType := range kvCacheTypes {
			// Test with maximum granularity for GPU-only context
			contextSizes := []int{4096, 8192, 12288, 16384, 20480, 24576, 28672, 32768, 40960, 49152, 57344, 65536, 81920, 98304, 114688, 131072, 163840, 196608, 229376, 262144, 327680, 393216, 458752, 524288, 655360, 786432, 917504, 1048576}

			for _, contextSize := range contextSizes {
				if contextSize > maxModelContext {
					break // Don't exceed model's max context
				}

				kvCacheSize := calculateKVCacheSize(contextSize, layersOnGPU, kvType)

				// Only use VRAM - no hybrid for GPU-only models
				if kvCacheSize <= remainingVRAM {
					if contextSize > bestContextSize {
						bestContextSize = contextSize
						bestKVCacheType = kvType
					}
				} else {
					// This context size won't fit in VRAM - stop trying larger sizes for this KV type
					break
				}
			}
		}

		if bestContextSize >= 16384 {
			kvCacheUsage := calculateKVCacheSize(bestContextSize, layersOnGPU, bestKVCacheType)
			fmt.Printf("   🎯 GPU-only optimal: %d tokens (%s KV cache, %.2f GB VRAM)\n",
				bestContextSize, bestKVCacheType, kvCacheUsage)
		} else {
			// Force minimum 16K for GPU-only models (16384 tokens = 16K)
			bestContextSize = 16384
			bestKVCacheType = "q4_0" // Use most efficient quantization
			fmt.Printf("   ⚠️ GPU VRAM tight: forced minimum 16K context with q4_0 KV cache\n")
		}

	} else {
		fmt.Printf("   🔄 Hybrid mode: Model requires CPU+GPU allocation\n")

		// HYBRID MODE: Only for models that don't fit entirely in GPU
		// **PERFORMANCE LIMIT**: Cap hybrid context at 24K for usable performance
		for _, kvType := range kvCacheTypes {
			// Limited context sizes for hybrid allocation (based on QA testing)
			contextSizes := []int{16384, 20480, 24576} // Max 24K for hybrid performance

			for _, contextSize := range contextSizes {
				if contextSize > maxModelContext {
					break
				}

				kvCacheSize := calculateKVCacheSize(contextSize, layersOnGPU, kvType)

				// Try VRAM first, then hybrid
				if kvCacheSize <= remainingVRAM {
					// Fits in VRAM only
					if contextSize > bestContextSize {
						bestContextSize = contextSize
						bestKVCacheType = kvType
					}
				} else if availableRAM > 0 {
					// Use hybrid VRAM+RAM (but limit context for performance)
					totalKVMemoryNeeded := kvCacheSize
					if totalKVMemoryNeeded <= (remainingVRAM + availableRAM) {
						if contextSize > bestContextSize {
							bestContextSize = contextSize
							bestKVCacheType = kvType
							fmt.Printf("   🔄 Hybrid KV cache: VRAM %.2f GB + RAM %.2f GB for context %dK (performance-limited)\n",
								remainingVRAM, totalKVMemoryNeeded-remainingVRAM, contextSize/1024)
						}
					}
				}
			}
		}

		if bestContextSize > 16384 {
			fmt.Printf("   ⚠️  Hybrid context limited to %dK tokens for usable performance (QA validated)\n", bestContextSize/1024)
		}
	}

	// Ensure minimum 16K context (16384 tokens)
	if bestContextSize < 16384 {
		bestContextSize = 16384
		if hasSWA {
			bestKVCacheType = "f16" // Force f16 for SWA models
		} else {
			bestKVCacheType = "q4_0" // Use most efficient quantization for non-SWA
		}
	}

	kvCacheUsage := calculateKVCacheSize(bestContextSize, layersOnGPU, bestKVCacheType)
	fmt.Printf("   🧠 Optimal context: %d tokens (%s KV cache, %.2f GB)\n",
		bestContextSize, bestKVCacheType, kvCacheUsage)

	return bestContextSize, bestKVCacheType
}

// getMaxContextForModel returns the maximum context size for a model
func (scg *ConfigGenerator) getMaxContextForModel(model ModelInfo) int {
	// Use model's maximum context if available
	if model.ContextLength > 0 {
		return model.ContextLength
	}

	// Default maximum contexts based on model size
	sizeStr := strings.TrimSuffix(model.Size, "B")
	if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
		switch {
		case size >= 30: // 30B+ models
			return 1048576 // 1M tokens
		case size >= 20: // 20B+ models
			return 524288 // 512K tokens
		case size >= 7: // 7B+ models
			return 262144 // 256K tokens
		case size >= 3: // 3B+ models
			return 131072 // 128K tokens
		default: // Small models
			return 65536 // 64K tokens
		}
	}

	// Default fallback
	return 32768 // 32K tokens
}

// writeOptimizations writes model-specific optimizations
func (scg *ConfigGenerator) writeOptimizations(config *strings.Builder, model ModelInfo, contextSize int) {
	// Embedding models - use metadata-based detection with optimal parameters
	if scg.isEmbeddingModel(model) {
		// Add pooling parameter based on model family
		poolingType := scg.detectPoolingType(model)
		config.WriteString(fmt.Sprintf("      --pooling %s\n", poolingType))

		// NO ctx-size for embedding models as per specifications

		// Optimal batch settings for embedding models
		config.WriteString("      --batch-size 1024\n")
		config.WriteString("      --ubatch-size 512\n")

		// Use the same NGL calculation as other models (respects CPU backend)
		nglValue := scg.calculateOptimalNGL(model)
		config.WriteString(fmt.Sprintf("      -ngl %d\n", nglValue))
		if scg.SystemInfo != nil && scg.SystemInfo.PhysicalCores > 0 {
			threads := scg.SystemInfo.PhysicalCores / 2
			if threads < 1 {
				threads = 1 // Minimum 1 thread
			}
			config.WriteString(fmt.Sprintf("      --threads %d\n", threads))
		}

		// Memory management parameters with RAM awareness
		config.WriteString("      --keep 1024\n")        // Cache management
		config.WriteString("      --defrag-thold 0.1\n") // Memory defragmentation

		// Only use --mlock if sufficient RAM is available
		if scg.shouldUseMlock(model) {
			config.WriteString("      --mlock\n") // Lock model in RAM (if sufficient)
		}

		config.WriteString("      --flash-attn on\n") // Flash attention
		config.WriteString("      --cont-batching\n") // Continuous batching
		config.WriteString("      --jinja\n")         // Template processing
		config.WriteString("      --no-warmup\n")     // Skip warmup

		// Don't add chat-specific parameters for embedding models
		return
	}

	// Add jinja templating for all non-embedding models
	// Modern llama.cpp can handle chat templates for virtually all language models
	if scg.Options.EnableJinja {
		config.WriteString("      --jinja\n")
	}

	// Model size based optimizations
	sizeStr := strings.TrimSuffix(model.Size, "B")
	if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
		switch {
		case size >= 20: // Large models (20B+)
			config.WriteString("      --cont-batching\n")
			config.WriteString("      --defrag-thold 0.1\n")
			config.WriteString("      --batch-size 1024\n")
			config.WriteString("      --ubatch-size 256\n")
			config.WriteString("      --keep 2048\n")

			// Add parallel processing with context size validation
			scg.addParallelProcessing(config, contextSize)
		case size >= 7: // Medium models (7B+)
			config.WriteString("      --batch-size 1024\n")
			config.WriteString("      --ubatch-size 256\n")
			config.WriteString("      --keep 2048\n")
		default: // Small models
			config.WriteString("      --batch-size 2048\n")
			config.WriteString("      --ubatch-size 512\n")
			config.WriteString("      --keep 4096\n")
		}
	}

	// Chat template parameters
	config.WriteString("      --temp 0.7\n")
	config.WriteString("      --repeat-penalty 1.05\n")
	config.WriteString("      --repeat-last-n 256\n")
	config.WriteString("      --top-p 0.9\n")
	config.WriteString("      --top-k 40\n")
	config.WriteString("      --min-p 0.1\n")
}

// generateModelID generates a unique model ID
func (scg *ConfigGenerator) generateModelID(model ModelInfo) string {
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

	// Check if this ID has been used before and handle duplicates
	baseID := name
	if count, exists := scg.usedModelIDs[baseID]; exists {
		// Increment the count and append version number
		scg.usedModelIDs[baseID] = count + 1
		return fmt.Sprintf("%s-v%d", baseID, count+1)
	} else {
		// First occurrence, just track it
		scg.usedModelIDs[baseID] = 1
		return baseID
	}
}

// generateDescription generates a model description
func (scg *ConfigGenerator) generateDescription(model ModelInfo) string {
	parts := []string{}

	if model.Size != "" {
		parts = append(parts, fmt.Sprintf("Model size: %s", model.Size))
	}

	if model.Quantization != "" {
		parts = append(parts, fmt.Sprintf("Quantization: %s", model.Quantization))
	}

	if model.IsInstruct {
		parts = append(parts, "Instruction-tuned")
	}

	if len(parts) > 0 {
		return strings.Join(parts, " - ")
	}

	return "Auto-detected model"
}

// addParallelProcessing adds parallel processing with context size validation
func (scg *ConfigGenerator) addParallelProcessing(config *strings.Builder, contextSize int) {
	// Only add parallel processing if deployment mode is enabled
	if !scg.Options.EnableParallel {
		return // Skip parallel processing - will default to 1
	}

	const baseParallel = 4

	// Ensure context size / parallel is at least 8000 to prevent context shift issues
	if contextSize/baseParallel >= 8000 {
		config.WriteString(fmt.Sprintf("      --parallel %d\n", baseParallel))
	} else {
		// Calculate appropriate parallel value
		maxParallel := contextSize / 8000
		if maxParallel >= 2 {
			config.WriteString(fmt.Sprintf("      --parallel %d\n", maxParallel))
		}
		// If maxParallel < 2, don't add parallel processing (defaults to 1)
	}
}

// writeGroups writes model groups
func (scg *ConfigGenerator) writeGroups(config *strings.Builder, models []ModelInfo, modelIDMap map[string]string) {
	largeModels := []string{}
	smallModels := []string{}

	// Use pre-generated model IDs from map
	for _, model := range models {
		if model.IsDraft {
			continue
		}

		modelID := modelIDMap[model.Path]

		// Categorize by model type - use metadata-based embedding detection
		if scg.isEmbeddingModel(model) {
			smallModels = append(smallModels, modelID)
		} else {
			largeModels = append(largeModels, modelID)
		}
	}

	config.WriteString("\ngroups:\n")

	if len(largeModels) > 0 {
		config.WriteString("  \"large-models\":\n")
		config.WriteString("    swap: true\n")
		config.WriteString("    exclusive: true\n")
		config.WriteString("    startPort: 5800\n")
		config.WriteString("    members:\n")
		for _, model := range largeModels {
			config.WriteString(fmt.Sprintf("      - \"%s\"\n", model))
		}
		config.WriteString("\n")
	}

	if len(smallModels) > 0 {
		config.WriteString("  \"small-models\":\n")
		config.WriteString("    swap: false\n")
		config.WriteString("    exclusive: false\n")
		config.WriteString("    persistent: true\n")
		config.WriteString("    startPort: 6000\n")
		config.WriteString("    members:\n")
		for _, model := range smallModels {
			config.WriteString(fmt.Sprintf("      - \"%s\"\n", model))
		}
	}
}

// findMatchingMMProj finds the matching mmproj file for a given model path
func (scg *ConfigGenerator) findMatchingMMProj(modelPath string) string {
	// Look through all mmproj matches to find one for this model
	for _, match := range scg.mmprojMatches {
		if match.ModelPath == modelPath {
			// Return the mmproj path with the highest confidence for this model
			return match.MMProjPath
		}
	}
	return "" // No matching mmproj found
}

// quotePath properly quotes file paths that contain spaces or special characters
func quotePath(path string) string {
	// Always quote paths that contain spaces (common in external drives like "T7 Shield")
	if strings.Contains(path, " ") {
		// Escape any existing quotes and wrap in quotes
		escaped := strings.ReplaceAll(path, "\"", "\\\"")
		return fmt.Sprintf("\"%s\"", escaped)
	}
	return path
}

// isEmbeddingModel determines if a model is an embedding model using GGUF metadata
func (scg *ConfigGenerator) isEmbeddingModel(model ModelInfo) bool {
	// Read GGUF metadata to make intelligent decision
	metadata, err := ReadAllGGUFKeys(model.Path)
	if err != nil {
		// Fallback to name-based detection if metadata read fails
		return strings.Contains(strings.ToLower(model.Name), "embed")
	}

	// Use the same detection logic as in the debug function
	architecture := ""
	if val, exists := metadata["general.architecture"]; exists {
		if str, ok := val.(string); ok {
			architecture = str
		}
	}

	return detectEmbeddingFromMetadata(metadata, architecture)
}

// detectPoolingTypeByName detects the pooling type based on model family
func (scg *ConfigGenerator) detectPoolingTypeByName(model ModelInfo) string {
	modelName := strings.ToLower(model.Name)
	modelPath := strings.ToLower(model.Path)

	// Combine name and path for better detection
	fullName := modelName + " " + modelPath

	// BGE models
	if strings.Contains(fullName, "bge") {
		return "cls"
	}

	// E5 models
	if strings.Contains(fullName, "e5") {
		return "mean"
	}

	// GTE models
	if strings.Contains(fullName, "gte") {
		return "mean"
	}

	// MXBAI models
	if strings.Contains(fullName, "mxbai") {
		return "mean"
	}

	// Nomic Embed models
	if strings.Contains(fullName, "nomic") {
		return "mean"
	}

	// Jina models - need to detect version
	if strings.Contains(fullName, "jina") {
		// Jina v2/v3 use 'last', v1 uses 'cls'
		if strings.Contains(fullName, "v2") || strings.Contains(fullName, "v3") {
			return "last"
		}
		return "cls" // v1 or unknown version
	}

	// Stella models
	if strings.Contains(fullName, "stella") {
		return "mean"
	}

	// Arctic models
	if strings.Contains(fullName, "arctic") {
		return "mean"
	}

	// SFR models
	if strings.Contains(fullName, "sfr") {
		return "mean"
	}

	// Default fallback
	return "mean"
}

// detectPoolingType detects the pooling type from model metadata
func (scg *ConfigGenerator) detectPoolingType(model ModelInfo) string {
	// Read GGUF metadata to find pooling type
	metadata, err := ReadAllGGUFKeys(model.Path)
	if err != nil {
		return scg.detectPoolingTypeByName(model) // Fallback to name-based detection
	}

	// Get architecture to construct the pooling key
	architecture := ""
	if val, exists := metadata["general.architecture"]; exists {
		if str, ok := val.(string); ok {
			architecture = str
		}
	}

	// Look for pooling type in metadata
	poolingKey := fmt.Sprintf("%s.pooling_type", architecture)
	if val, exists := metadata[poolingKey]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}

	// Check alternative keys
	alternativeKeys := []string{
		"pooling_type",
		fmt.Sprintf("%s.pooling", architecture),
		"pooling",
	}

	for _, key := range alternativeKeys {
		if val, exists := metadata[key]; exists {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}

	// Fallback to name-based detection
	return scg.detectPoolingTypeByName(model)
}

// shouldUseMlock determines if --mlock should be used based on available system RAM
func (scg *ConfigGenerator) shouldUseMlock(model ModelInfo) bool {
	// If no system info available, default to conservative approach (no mlock)
	if scg.SystemInfo == nil || scg.SystemInfo.TotalRAMGB <= 0 {
		return false
	}

	// Get model size
	modelSizeGB := 0.0
	if sizeStr := strings.TrimSuffix(model.Size, "B"); sizeStr != "" {
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			modelSizeGB = size
		}
	}

	// If model size is unknown, use file size as fallback
	if modelSizeGB == 0.0 {
		if info, err := os.Stat(model.Path); err == nil {
			modelSizeGB = float64(info.Size()) / (1024 * 1024 * 1024) // Convert bytes to GB
		}
	}

	// Calculate available RAM (leave 25% buffer for system operations)
	availableRAM := scg.SystemInfo.TotalRAMGB * 0.75

	// For embedding models, use mlock if model + 2GB buffer fits in available RAM
	if scg.isEmbeddingModel(model) {
		requiredRAM := modelSizeGB + 2.0 // Model + 2GB buffer
		return requiredRAM <= availableRAM
	}

	// For large language models, be more conservative (need more RAM for context processing)
	// Only use mlock for small models (< 8GB) if sufficient RAM is available
	if modelSizeGB < 8.0 {
		requiredRAM := modelSizeGB + 4.0 // Model + 4GB buffer for LLMs
		return requiredRAM <= availableRAM
	}

	// Don't use mlock for large models to avoid system instability
	return false
}

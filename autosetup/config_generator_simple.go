package autosetup

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// SimpleConfigGenerator generates configurations with -ngl 999 and maximum context
type SimpleConfigGenerator struct {
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

// NewSimpleConfigGenerator creates a new simple config generator
func NewSimpleConfigGenerator(modelsPath, binaryPath, outputPath string, options SetupOptions) *SimpleConfigGenerator {
	return &SimpleConfigGenerator{
		ModelsPath:   modelsPath,
		BinaryPath:   binaryPath,
		OutputPath:   outputPath,
		Options:      options,
		usedModelIDs: make(map[string]int),
	}
}

// SetAvailableVRAM sets the total VRAM in GB
func (scg *SimpleConfigGenerator) SetAvailableVRAM(vramGB float64) {
	scg.TotalVRAMGB = vramGB
}

// SetBinaryType sets the binary type (cuda, rocm, cpu)
func (scg *SimpleConfigGenerator) SetBinaryType(binaryType string) {
	scg.BinaryType = binaryType
}

// SetMMProjMatches sets the mmproj matches for automatic --mmproj parameter addition
func (scg *SimpleConfigGenerator) SetMMProjMatches(matches []MMProjMatch) {
	scg.mmprojMatches = matches
}

// SetSystemInfo sets the system information for optimal parameter calculation
func (scg *SimpleConfigGenerator) SetSystemInfo(systemInfo *SystemInfo) {
	scg.SystemInfo = systemInfo
}

// GenerateConfig generates a simple configuration file
func (scg *SimpleConfigGenerator) GenerateConfig(models []ModelInfo) error {
	config := strings.Builder{}

	// Write header
	scg.writeHeader(&config)

	// Write macros
	scg.writeMacros(&config)

	// Generate model IDs consistently (first pass)
	modelIDMap := make(map[string]string)
	for _, model := range models {
		if model.IsDraft {
			continue
		}
		modelIDMap[model.Path] = scg.generateModelID(model)
	}

	// Write models
	config.WriteString("\nmodels:\n")
	for _, model := range models {
		if model.IsDraft {
			continue // Skip draft models
		}
		scg.writeModel(&config, model, modelIDMap)
	}

	// Write groups
	scg.writeGroups(&config, models, modelIDMap)

	// Save to file
	return os.WriteFile(scg.OutputPath, []byte(config.String()), 0644)
}

// writeHeader writes the configuration header
func (scg *SimpleConfigGenerator) writeHeader(config *strings.Builder) {
	config.WriteString("# Auto-generated llama-swap configuration (SMART GPU ALLOCATION)\n")
	config.WriteString(fmt.Sprintf("# Generated from models in: %s\n", scg.ModelsPath))
	config.WriteString(fmt.Sprintf("# Binary: %s (%s)\n", scg.BinaryPath, scg.BinaryType))
	config.WriteString(fmt.Sprintf("# System: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	if scg.TotalVRAMGB > 0 {
		config.WriteString(fmt.Sprintf("# Total GPU VRAM: %.1f GB\n", scg.TotalVRAMGB))
	}
	config.WriteString("# Algorithm: If model < total VRAM: -ngl 999, else fit max layers in total VRAM\n")
	config.WriteString("\n")
	config.WriteString("healthCheckTimeout: 300\n")
	config.WriteString("logLevel: info\n")
	config.WriteString("startPort: 5800\n")
}

// writeMacros writes the base macros
func (scg *SimpleConfigGenerator) writeMacros(config *strings.Builder) {
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
func (scg *SimpleConfigGenerator) writeModel(config *strings.Builder, model ModelInfo, modelIDMap map[string]string) {
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
	config.WriteString(fmt.Sprintf("      --model %s\n", model.Path))

	// Add --mmproj parameter if a matching mmproj file is found
	mmprojPath := scg.findMatchingMMProj(model.Path)
	if mmprojPath != "" {
		config.WriteString(fmt.Sprintf("      --mmproj %s\n", mmprojPath))
	}

	// Smart GPU layer allocation algorithm
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

// calculateOptimalNGL calculates the optimal number of GPU layers based on model size vs VRAM
func (scg *SimpleConfigGenerator) calculateOptimalNGL(model ModelInfo) int {
	// For CPU-only configurations
	if scg.BinaryType != "cuda" && scg.BinaryType != "rocm" {
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

	// Reserve some VRAM for context and other overhead (2GB)
	reservedVRAM := 2.0
	usableVRAM := scg.TotalVRAMGB - reservedVRAM

	fmt.Printf("🧮 Model: %s\n", model.Name)
	fmt.Printf("   Size: %.2f GB, Layers: %d, Total VRAM: %.2f GB, Usable: %.2f GB\n",
		modelSizeGB, totalLayers, scg.TotalVRAMGB, usableVRAM)

	// Algorithm: If model size < usable VRAM, use all layers (-ngl 999)
	if modelSizeGB <= usableVRAM {
		fmt.Printf("   ✅ Model fits in VRAM: using -ngl 999 (all layers)\n")
		return 999
	}

	// Algorithm: Calculate how many layers fit in VRAM
	// Assume layers are roughly equal in size
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

// calculateOptimalContext calculates optimal context size based on remaining VRAM
func (scg *SimpleConfigGenerator) calculateOptimalContext(model ModelInfo, nglLayers int, modelSizeGB float64) (int, string) {
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
	var modelVRAMUsage float64

	if nglLayers == 999 {
		// All layers on GPU
		layersOnGPU = totalModelLayers
		modelVRAMUsage = modelSizeGB
	} else {
		// Partial layers on GPU
		layersOnGPU = nglLayers
		layerSizeGB := modelSizeGB / float64(totalModelLayers)
		modelVRAMUsage = layerSizeGB * float64(nglLayers)
	}

	// Calculate remaining VRAM for KV cache
	remainingVRAM := scg.TotalVRAMGB - modelVRAMUsage - 1.0 // 1GB overhead for other operations

	fmt.Printf("   💾 Model VRAM usage: %.2f GB, Remaining for KV cache: %.2f GB\n",
		modelVRAMUsage, remainingVRAM)

	// For SWA models, force f16 KV cache (no quantization)
	var kvCacheTypes []string
	if hasSWA {
		kvCacheTypes = []string{"f16"} // Only f16 for SWA models
		fmt.Printf("   🪟 SWA detected: using f16 KV cache (no quantization)\n")
	} else {
		kvCacheTypes = []string{"f16", "q8_0", "q4_0"} // Try all types for non-SWA models
	}

	bestContextSize := 4096 // Minimum fallback
	bestKVCacheType := "f16"

	// Get model's maximum context if available
	maxModelContext := 131072 // Default max
	if err == nil && modelInfo.ContextLength > 0 {
		maxModelContext = modelInfo.ContextLength
	}

	for _, kvType := range kvCacheTypes {
		// Test different context sizes
		contextSizes := []int{4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576}

		for _, contextSize := range contextSizes {
			if contextSize > maxModelContext {
				break // Don't exceed model's max context
			}

			kvCacheSize := calculateKVCacheSize(contextSize, layersOnGPU, kvType)

			if kvCacheSize <= remainingVRAM {
				if contextSize > bestContextSize {
					bestContextSize = contextSize
					bestKVCacheType = kvType
				}
			} else {
				break // This and larger contexts won't fit
			}
		}
	}

	// Ensure minimum 16K context
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
func (scg *SimpleConfigGenerator) getMaxContextForModel(model ModelInfo) int {
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
func (scg *SimpleConfigGenerator) writeOptimizations(config *strings.Builder, model ModelInfo, contextSize int) {
	// Embedding models - use metadata-based detection with optimal parameters
	if scg.isEmbeddingModel(model) {
		// Add pooling parameter based on model family
		poolingType := scg.detectPoolingType(model)
		config.WriteString(fmt.Sprintf("      --pooling %s\n", poolingType))

		// NO ctx-size for embedding models as per specifications

		// Optimal batch settings for embedding models
		config.WriteString("      --batch-size 4096\n")
		config.WriteString("      --ubatch-size 256\n")
		config.WriteString("      -ngl 999\n") // Calculate optimal threads (half of physical cores)
		if scg.SystemInfo != nil && scg.SystemInfo.PhysicalCores > 0 {
			threads := scg.SystemInfo.PhysicalCores / 2
			if threads < 1 {
				threads = 1 // Minimum 1 thread
			}
			config.WriteString(fmt.Sprintf("      --threads %d\n", threads))
		}

		// Optional but helpful parameters
		config.WriteString("      --keep 1024\n")        // Cache management
		config.WriteString("      --defrag-thold 0.1\n") // Memory defragmentation
		config.WriteString("      --mlock\n")            // Lock model in RAM
		config.WriteString("      --flash-attn on\n")    // Flash attention
		config.WriteString("      --cont-batching\n")    // Continuous batching
		config.WriteString("      --jinja\n")            // Template processing
		config.WriteString("      --no-warmup\n")        // Skip warmup

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
func (scg *SimpleConfigGenerator) generateModelID(model ModelInfo) string {
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
func (scg *SimpleConfigGenerator) generateDescription(model ModelInfo) string {
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
func (scg *SimpleConfigGenerator) addParallelProcessing(config *strings.Builder, contextSize int) {
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
func (scg *SimpleConfigGenerator) writeGroups(config *strings.Builder, models []ModelInfo, modelIDMap map[string]string) {
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
		config.WriteString("    members:\n")
		for _, model := range smallModels {
			config.WriteString(fmt.Sprintf("      - \"%s\"\n", model))
		}
	}
}

// findMatchingMMProj finds the matching mmproj file for a given model path
func (scg *SimpleConfigGenerator) findMatchingMMProj(modelPath string) string {
	// Look through all mmproj matches to find one for this model
	for _, match := range scg.mmprojMatches {
		if match.ModelPath == modelPath {
			// Return the mmproj path with the highest confidence for this model
			return match.MMProjPath
		}
	}
	return "" // No matching mmproj found
}

// isEmbeddingModel determines if a model is an embedding model using GGUF metadata
func (scg *SimpleConfigGenerator) isEmbeddingModel(model ModelInfo) bool {
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
func (scg *SimpleConfigGenerator) detectPoolingTypeByName(model ModelInfo) string {
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
func (scg *SimpleConfigGenerator) detectPoolingType(model ModelInfo) string {
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

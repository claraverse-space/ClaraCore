package autosetup

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// MemoryEstimator calculates memory requirements and optimal context sizes
type MemoryEstimator struct {
	OverheadGB float64 // Overhead for compute buffers, drivers, etc.
}

// NewMemoryEstimator creates a new memory estimator
func NewMemoryEstimator() *MemoryEstimator {
	return &MemoryEstimator{
		OverheadGB: 2.0, // Default 2GB overhead
	}
}

// ModelMemoryInfo contains memory information for a model
type ModelMemoryInfo struct {
	ModelSizeGB       float64
	BytesPerToken     int64
	MaxContextLength  uint32
	HasSlidingWindow  bool
	SlidingWindowSize uint32
	IsScout           bool
}

// ContextMemoryResult contains the result of context memory calculation
type ContextMemoryResult struct {
	ContextSize     int
	KVCacheGB       float64
	TotalMemoryGB   float64
	CanFitInVRAM    bool
	OptimalLayers   int  // Number of layers that can fit on GPU
	RequiresOffload bool // Whether CPU offloading is needed
}

// LayerOffloadResult contains layer offloading analysis
type LayerOffloadResult struct {
	TotalLayers     uint32
	GPULayers       int
	CPULayers       int
	GPUMemoryGB     float64
	EstimatedCPURAM float64
	ContextSize     int
}

// GetModelMemoryInfo extracts memory-related information from a model
func (me *MemoryEstimator) GetModelMemoryInfo(modelPath string) (*ModelMemoryInfo, error) {
	// Read GGUF metadata
	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GGUF metadata: %w", err)
	}

	// Get model file size (handle multi-part models)
	modelSizeBytes, err := me.getTotalModelSize(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get model size: %w", err)
	}

	// Calculate bytes per token per layer for KV cache
	bytesPerToken := int64(metadata.HeadCountKV) * int64(metadata.KeyLength+metadata.ValueLength) * 2

	// Check if it's a scout model or has sliding window
	isScout := strings.Contains(strings.ToLower(metadata.ModelName), "scout")
	hasSlidingWindow := metadata.SlidingWindow > 0 || isScout

	slidingWindowSize := metadata.SlidingWindow
	if isScout && slidingWindowSize == 0 {
		slidingWindowSize = 8192 // Default for scout models
	}

	return &ModelMemoryInfo{
		ModelSizeGB:       float64(modelSizeBytes) / (1024 * 1024 * 1024),
		BytesPerToken:     bytesPerToken,
		MaxContextLength:  metadata.ContextLength,
		HasSlidingWindow:  hasSlidingWindow,
		SlidingWindowSize: slidingWindowSize,
		IsScout:           isScout,
	}, nil
}

// CalculateMemoryForContext calculates memory usage for a specific context size
func (me *MemoryEstimator) CalculateMemoryForContext(memInfo *ModelMemoryInfo, contextSize int, blockCount uint32) *ContextMemoryResult {
	var kvCacheBytes int64

	if memInfo.HasSlidingWindow {
		// For sliding window models, context memory is limited by window size
		effectiveContext := int(math.Min(float64(contextSize), float64(memInfo.SlidingWindowSize)))
		kvCacheBytes = int64(effectiveContext) * int64(blockCount) * memInfo.BytesPerToken
	} else {
		// Regular models use full context
		kvCacheBytes = int64(contextSize) * int64(blockCount) * memInfo.BytesPerToken
	}

	kvCacheGB := float64(kvCacheBytes) / (1024 * 1024 * 1024)
	totalMemoryGB := memInfo.ModelSizeGB + kvCacheGB + me.OverheadGB

	return &ContextMemoryResult{
		ContextSize:     contextSize,
		KVCacheGB:       kvCacheGB,
		TotalMemoryGB:   totalMemoryGB,
		CanFitInVRAM:    false,                                   // Will be set by caller based on available VRAM
		OptimalLayers:   0,                                       // Will be calculated if needed
		RequiresOffload: totalMemoryGB > memInfo.ModelSizeGB*1.5, // Rough heuristic
	}
}

// FindOptimalContextSize finds the maximum context size that fits in available VRAM
func (me *MemoryEstimator) FindOptimalContextSize(modelPath string, availableVRAMGB float64) (int, error) {
	// Get model memory info
	memInfo, err := me.GetModelMemoryInfo(modelPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get model memory info: %w", err)
	}

	// Read metadata to get block count
	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Check if model can fit at all
	minMemory := memInfo.ModelSizeGB + me.OverheadGB
	if minMemory > availableVRAMGB {
		return 0, fmt.Errorf("model requires %.2f GB but only %.2f GB available", minMemory, availableVRAMGB)
	}

	// Binary search for optimal context size
	minContext := 512
	maxContext := int(memInfo.MaxContextLength)
	if maxContext == 0 {
		maxContext = 1048576 // 1M tokens max if not specified
	}

	// Common context sizes to try
	commonSizes := []int{512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576}

	optimalContext := minContext

	// First, try common sizes
	for _, size := range commonSizes {
		if size > maxContext {
			break
		}

		result := me.CalculateMemoryForContext(memInfo, size, metadata.BlockCount)
		if result.TotalMemoryGB <= availableVRAMGB {
			optimalContext = size
		} else {
			break
		}
	}

	// If we found a good common size, try to optimize further
	if optimalContext > minContext {
		// Binary search between the last good size and next size
		low := optimalContext
		high := optimalContext * 2
		if high > maxContext {
			high = maxContext
		}

		for low < high {
			mid := (low + high + 1) / 2
			result := me.CalculateMemoryForContext(memInfo, mid, metadata.BlockCount)

			if result.TotalMemoryGB <= availableVRAMGB {
				low = mid
			} else {
				high = mid - 1
			}
		}
		optimalContext = low
	}

	return optimalContext, nil
}

// CalculateOptimalLayers calculates how many layers can fit on GPU with given VRAM
func (me *MemoryEstimator) CalculateOptimalLayers(modelPath string, availableVRAMGB float64, contextSize int) (*LayerOffloadResult, error) {
	// Get model metadata
	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	memInfo, err := me.GetModelMemoryInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	totalLayers := metadata.BlockCount
	if totalLayers == 0 {
		return nil, fmt.Errorf("could not determine number of layers")
	}

	// Estimate model size per layer (rough approximation)
	modelSizePerLayerGB := memInfo.ModelSizeGB / float64(totalLayers)

	// Calculate KV cache memory for given context
	kvCacheResult := me.CalculateMemoryForContext(memInfo, contextSize, totalLayers)
	kvCachePerLayerGB := kvCacheResult.KVCacheGB / float64(totalLayers)

	// Binary search for optimal number of layers
	low := 0
	high := int(totalLayers)
	optimalLayers := 0

	for low <= high {
		mid := (low + high) / 2

		// Calculate memory for 'mid' layers on GPU
		gpuModelMemory := float64(mid) * modelSizePerLayerGB
		gpuKVMemory := float64(mid) * kvCachePerLayerGB
		totalGPUMemory := gpuModelMemory + gpuKVMemory + me.OverheadGB

		if totalGPUMemory <= availableVRAMGB {
			optimalLayers = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	// Calculate CPU memory requirement for remaining layers
	cpuLayers := int(totalLayers) - optimalLayers
	cpuModelMemory := float64(cpuLayers) * modelSizePerLayerGB
	cpuKVMemory := float64(cpuLayers) * kvCachePerLayerGB
	estimatedCPURAM := cpuModelMemory + cpuKVMemory

	// Calculate actual GPU memory usage
	gpuMemoryGB := float64(optimalLayers)*modelSizePerLayerGB +
		float64(optimalLayers)*kvCachePerLayerGB +
		me.OverheadGB

	return &LayerOffloadResult{
		TotalLayers:     totalLayers,
		GPULayers:       optimalLayers,
		CPULayers:       cpuLayers,
		GPUMemoryGB:     gpuMemoryGB,
		EstimatedCPURAM: estimatedCPURAM,
		ContextSize:     contextSize,
	}, nil
}

// FindOptimalContextSizeWithOffload finds optimal context size considering layer offloading
func (me *MemoryEstimator) FindOptimalContextSizeWithOffload(modelPath string, availableVRAMGB float64) (*LayerOffloadResult, error) {
	// Get model metadata
	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	memInfo, err := me.GetModelMemoryInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	// If model fits entirely in VRAM, use regular optimization
	if memInfo.ModelSizeGB+me.OverheadGB <= availableVRAMGB {
		contextSize, err := me.FindOptimalContextSize(modelPath, availableVRAMGB)
		if err != nil {
			return nil, err
		}
		return &LayerOffloadResult{
			TotalLayers:     metadata.BlockCount,
			GPULayers:       int(metadata.BlockCount),
			CPULayers:       0,
			GPUMemoryGB:     memInfo.ModelSizeGB + me.OverheadGB,
			EstimatedCPURAM: 0,
			ContextSize:     contextSize,
		}, nil
	}

	// Try different context sizes to find the best offloading configuration
	contextSizes := []int{4096, 8192, 16384, 32768, 65536}
	var bestResult *LayerOffloadResult

	for _, ctx := range contextSizes {
		if memInfo.MaxContextLength > 0 && uint32(ctx) > memInfo.MaxContextLength {
			continue
		}

		result, err := me.CalculateOptimalLayers(modelPath, availableVRAMGB, ctx)
		if err != nil {
			continue
		}

		// Prefer configurations with more GPU layers and reasonable context size
		if bestResult == nil || result.GPULayers > bestResult.GPULayers ||
			(result.GPULayers == bestResult.GPULayers && result.ContextSize > bestResult.ContextSize) {
			bestResult = result
		}
	}

	if bestResult == nil {
		return nil, fmt.Errorf("could not find viable offloading configuration")
	}

	return bestResult, nil
}

// GetAvailableVRAM detects available VRAM in GB
func (me *MemoryEstimator) GetAvailableVRAM() (float64, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Try Windows paths for nvidia-smi
		paths := []string{
			"C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
			"C:\\Windows\\System32\\nvidia-smi.exe",
		}

		var nvidiaSmiPath string
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				nvidiaSmiPath = path
				break
			}
		}

		if nvidiaSmiPath == "" {
			return 0, fmt.Errorf("nvidia-smi not found")
		}

		cmd = exec.Command(nvidiaSmiPath, "--query-gpu=memory.total", "--format=csv,noheader,nounits")
	} else {
		// Unix systems
		if _, err := os.Stat("/usr/bin/nvidia-smi"); err != nil {
			return 0, fmt.Errorf("nvidia-smi not found")
		}
		cmd = exec.Command("nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits")
	}

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run nvidia-smi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("no GPU memory information found")
	}

	// Parse the first GPU's memory (in MB)
	memoryMB, err := strconv.ParseFloat(strings.TrimSpace(lines[0]), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory value: %w", err)
	}

	// Convert MB to GB and leave some buffer (90% of total)
	totalGB := memoryMB / 1024
	availableGB := totalGB * 0.9 // Use 90% of total VRAM

	return availableGB, nil
}

// getTotalModelSize calculates the total size of a model (handles multi-part files)
func (me *MemoryEstimator) getTotalModelSize(modelPath string) (int64, error) {
	// Check if this is a multi-part model
	re := regexp.MustCompile(`-(\d{5})-of-(\d{5})\.gguf$`)
	matches := re.FindStringSubmatch(modelPath)

	if len(matches) == 0 {
		// Single file
		info, err := os.Stat(modelPath)
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}

	// Multi-part model
	basePath := modelPath[:len(modelPath)-len(matches[0])]
	totalParts, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, fmt.Errorf("invalid part count: %s", matches[2])
	}

	var totalSize int64
	foundParts := 0

	for i := 1; i <= totalParts; i++ {
		partPath := fmt.Sprintf("%s-%05d-of-%s.gguf", basePath, i, matches[2])
		if info, err := os.Stat(partPath); err == nil {
			totalSize += info.Size()
			foundParts++
		}
	}

	if foundParts != totalParts {
		return 0, fmt.Errorf("expected %d parts, found %d", totalParts, foundParts)
	}

	return totalSize, nil
}

// FormatMemory formats bytes as human-readable memory size
func FormatMemory(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatMemoryGB formats GB as human-readable string
func FormatMemoryGB(gb float64) string {
	if gb < 1.0 {
		return fmt.Sprintf("%.0f MiB", gb*1024)
	}
	return fmt.Sprintf("%.2f GiB", gb)
}

// EstimateModelForVRAM provides a complete analysis of a model for given VRAM
func (me *MemoryEstimator) EstimateModelForVRAM(modelPath string, availableVRAMGB float64) (*ModelAnalysis, error) {
	memInfo, err := me.GetModelMemoryInfo(modelPath)
	if err != nil {
		return nil, err
	}

	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return nil, err
	}

	// Check if model needs offloading
	minMemory := memInfo.ModelSizeGB + me.OverheadGB
	needsOffloading := minMemory > availableVRAMGB

	var analysis *ModelAnalysis

	if needsOffloading {
		// Use offloading analysis
		offloadResult, err := me.FindOptimalContextSizeWithOffload(modelPath, availableVRAMGB)
		if err != nil {
			return nil, err
		}

		analysis = &ModelAnalysis{
			ModelPath:        modelPath,
			ModelName:        metadata.ModelName,
			ModelSizeGB:      memInfo.ModelSizeGB,
			MaxContextLength: memInfo.MaxContextLength,
			OptimalContext:   offloadResult.ContextSize,
			MemoryResult: &ContextMemoryResult{
				ContextSize:     offloadResult.ContextSize,
				KVCacheGB:       offloadResult.GPUMemoryGB - me.OverheadGB - (memInfo.ModelSizeGB * float64(offloadResult.GPULayers) / float64(offloadResult.TotalLayers)),
				TotalMemoryGB:   offloadResult.GPUMemoryGB,
				CanFitInVRAM:    true, // GPU portion fits
				OptimalLayers:   offloadResult.GPULayers,
				RequiresOffload: true,
			},
			AvailableVRAMGB: availableVRAMGB,
			OffloadResult:   offloadResult,
		}
	} else {
		// Regular analysis
		optimalContext, err := me.FindOptimalContextSize(modelPath, availableVRAMGB)
		if err != nil {
			return nil, err
		}

		result := me.CalculateMemoryForContext(memInfo, optimalContext, metadata.BlockCount)
		result.CanFitInVRAM = result.TotalMemoryGB <= availableVRAMGB
		result.OptimalLayers = int(metadata.BlockCount) // All layers on GPU
		result.RequiresOffload = false

		analysis = &ModelAnalysis{
			ModelPath:        modelPath,
			ModelName:        metadata.ModelName,
			ModelSizeGB:      memInfo.ModelSizeGB,
			MaxContextLength: memInfo.MaxContextLength,
			OptimalContext:   optimalContext,
			MemoryResult:     result,
			AvailableVRAMGB:  availableVRAMGB,
		}
	}

	return analysis, nil
}

// ModelAnalysis contains complete analysis of a model
type ModelAnalysis struct {
	ModelPath        string
	ModelName        string
	ModelSizeGB      float64
	MaxContextLength uint32
	OptimalContext   int
	MemoryResult     *ContextMemoryResult
	AvailableVRAMGB  float64
	OffloadResult    *LayerOffloadResult // Only set if offloading is needed
}

// Print prints the analysis in a readable format
func (ma *ModelAnalysis) Print() {
	fmt.Printf("\n--- Model Analysis: %s ---\n", ma.ModelName)
	fmt.Printf("Model Size: %s\n", FormatMemoryGB(ma.ModelSizeGB))
	fmt.Printf("Max Context: %d tokens\n", ma.MaxContextLength)
	fmt.Printf("Available VRAM: %s\n", FormatMemoryGB(ma.AvailableVRAMGB))

	if ma.OffloadResult != nil {
		// Offloading configuration
		fmt.Printf("\n🔄 Layer Offloading Configuration:\n")
		fmt.Printf("  Total Layers: %d\n", ma.OffloadResult.TotalLayers)
		fmt.Printf("  GPU Layers: %d (%.1f%%)\n", ma.OffloadResult.GPULayers,
			float64(ma.OffloadResult.GPULayers)/float64(ma.OffloadResult.TotalLayers)*100)
		fmt.Printf("  CPU Layers: %d (%.1f%%)\n", ma.OffloadResult.CPULayers,
			float64(ma.OffloadResult.CPULayers)/float64(ma.OffloadResult.TotalLayers)*100)
		fmt.Printf("  Context Size: %d tokens\n", ma.OffloadResult.ContextSize)
		fmt.Printf("  GPU Memory: %s\n", FormatMemoryGB(ma.OffloadResult.GPUMemoryGB))
		fmt.Printf("  Est. CPU RAM: %s\n", FormatMemoryGB(ma.OffloadResult.EstimatedCPURAM))
		fmt.Printf("  Status: ✓ Hybrid GPU+CPU configuration\n")
		fmt.Printf("  llama.cpp flag: -ngl %d\n", ma.OffloadResult.GPULayers)
	} else {
		// Regular configuration
		fmt.Printf("\nOptimal Configuration:\n")
		fmt.Printf("  Context Size: %d tokens\n", ma.OptimalContext)
		fmt.Printf("  KV Cache: %s\n", FormatMemoryGB(ma.MemoryResult.KVCacheGB))
		fmt.Printf("  Total VRAM: %s\n", FormatMemoryGB(ma.MemoryResult.TotalMemoryGB))
		if ma.MemoryResult.CanFitInVRAM {
			fmt.Printf("  Status: ✓ Fits entirely in VRAM\n")
			fmt.Printf("  llama.cpp flag: -ngl 99 (all layers)\n")
		} else {
			fmt.Printf("  Status: ✗ Does not fit in VRAM\n")
		}
	}
}

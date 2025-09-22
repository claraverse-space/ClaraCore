package autosetup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// ModelInfo represents information about a detected GGUF model
type ModelInfo struct {
	Name          string
	Path          string
	Size          string
	IsInstruct    bool
	IsDraft       bool
	IsEmbedding   bool // Whether this is an embedding model
	Quantization  string
	ContextLength int  // Maximum context length supported by the model
	EmbeddingSize int  // Embedding dimension size
	NumLayers     int  // Number of transformer layers
	IsMoE         bool // Whether this is a Mixture of Experts model
}

// DetectModels scans a directory for GGUF files and returns model information
func DetectModels(modelsDir string) ([]ModelInfo, error) {
	return DetectModelsWithOptions(modelsDir, SetupOptions{EnableParallel: true})
}

// DetectModelsWithOptions scans a directory for GGUF files with parallel processing options
func DetectModelsWithOptions(modelsDir string, options SetupOptions) ([]ModelInfo, error) {
	var allFiles []string

	// First, collect all GGUF files
	err := filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			allFiles = append(allFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan models directory: %v", err)
	}

	if !options.EnableParallel {
		// Sequential processing (original method)
		var models []ModelInfo
		for _, path := range allFiles {
			filename := filepath.Base(path)
			model := parseGGUFFilename(path, filename)
			models = append(models, model)
		}
		return models, nil
	}

	// Parallel processing for better performance
	fmt.Printf("🔄 Processing %d models in parallel...\n", len(allFiles))
	models := make([]ModelInfo, len(allFiles))
	var wg sync.WaitGroup
	var processed int32
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent operations

	// Progress ticker
	progressTicker := make(chan struct{})
	go func() {
		for range progressTicker {
			current := atomic.LoadInt32(&processed)
			percentage := float64(current) / float64(len(allFiles)) * 100
			fmt.Printf("\r   📊 Progress: %d/%d (%.1f%%) models processed", current, len(allFiles), percentage)
		}
	}()

	for i, path := range allFiles {
		wg.Add(1)
		go func(index int, filePath string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			filename := filepath.Base(filePath)
			model := parseGGUFFilename(filePath, filename)
			models[index] = model

			// Update progress
			current := atomic.AddInt32(&processed, 1)
			if current%5 == 0 || current == int32(len(allFiles)) { // Update every 5 models or at end
				select {
				case progressTicker <- struct{}{}:
				default:
				}
			}
		}(i, path)
	}

	wg.Wait()
	close(progressTicker)
	fmt.Printf("\r   ✅ Completed: %d/%d (100.0%%) models processed\n", len(allFiles), len(allFiles))
	return models, nil
}

// parseGGUFFilename extracts model information from filename and GGUF metadata
func parseGGUFFilename(fullPath, filename string) ModelInfo {
	filename = strings.TrimSuffix(filename, ".gguf")
	lower := strings.ToLower(filename)

	model := ModelInfo{
		Name: filename,
		Path: fullPath,
	}

	// Skip multi-part files (they'll be handled by the main file)
	if strings.Contains(lower, "-00002-of-") || strings.Contains(lower, "-00003-of-") {
		model.IsDraft = true // Mark as draft so they're skipped in main models
		return model
	}

	// Skip projection files (.mmproj) - these are for multimodal models
	if strings.Contains(lower, "mmproj") {
		model.IsDraft = true // Mark as draft so they're skipped in main models
		return model
	}

	// Try to read GGUF metadata for better detection
	if ggufMeta, err := ReadGGUFMetadata(fullPath); err == nil {
		// Extract information from GGUF metadata
		if ggufMeta.ContextLength > 0 {
			model.ContextLength = int(ggufMeta.ContextLength)
		}
		if ggufMeta.BlockCount > 0 {
			model.NumLayers = int(ggufMeta.BlockCount)
		}

		// Calculate embedding size from attention dimensions
		if ggufMeta.KeyLength > 0 && ggufMeta.HeadCountKV > 0 {
			model.EmbeddingSize = int(ggufMeta.KeyLength * ggufMeta.HeadCountKV)
		}

		// Check for embedding-specific architectures
		arch := strings.ToLower(ggufMeta.Architecture)
		if arch == "bert" || arch == "nomic-bert" || strings.Contains(arch, "embed") {
			model.IsEmbedding = true
		}
	}

	// Fallback: Check filename/path for embedding indicators
	lowerPath := strings.ToLower(fullPath)
	if strings.Contains(lower, "embed") || strings.Contains(lower, "embedding") ||
		strings.Contains(lowerPath, "embed") || strings.Contains(lowerPath, "embedding") ||
		strings.Contains(lower, "mxbai") || // mxbai models are embeddings
		strings.Contains(lower, "bge-") || // BGE embedding models
		strings.Contains(lower, "e5-") { // E5 embedding models
		model.IsEmbedding = true
	}

	// Detect if it's an instruct/chat model
	if strings.Contains(lower, "instruct") || strings.Contains(lower, "chat") ||
		strings.Contains(lower, "-it") || strings.Contains(lower, "tools") {
		model.IsInstruct = true
	}

	// Detect quantization level with more patterns
	quantizations := []string{
		"iq4_xs", "iq4_nl", "iq3_m", "iq3_s", "iq2_m", "iq2_s", "iq1_m", "iq1_s",
		"q8_0", "q6_k", "q5_k_m", "q5_k_s", "q4_k_m", "q4_k_s", "q4_0", "q3_k_m", "q2_k",
		"f32", "f16", "bf16",
	}
	for _, quant := range quantizations {
		if strings.Contains(lower, quant) {
			model.Quantization = strings.ToUpper(quant)
			break
		}
	}

	// Detect size with more patterns
	// Look for patterns like "3b", "7b", "8b", "13b", "30b", "32b", "70b", etc.
	sizePatterns := []string{
		"0.5b", "0.6b", "1b", "1.5b", "2b", "3b", "4b", "7b", "8b", "9b",
		"13b", "14b", "20b", "27b", "30b", "32b", "36b", "45b", "70b", "72b", "405b",
	}
	for _, size := range sizePatterns {
		if strings.Contains(lower, size) {
			model.Size = strings.ToUpper(size)
			break
		}
	}

	// If no size detected from filename, try to extract from model name patterns
	if model.Size == "" {
		// Try patterns like "qwen3-4b", "llama-8b", etc.
		parts := strings.FieldsFunc(lower, func(r rune) bool {
			return r == '-' || r == '_' || r == '.'
		})
		for _, part := range parts {
			for _, size := range sizePatterns {
				if part == size {
					model.Size = strings.ToUpper(size)
					break
				}
			}
			if model.Size != "" {
				break
			}
		}
	}

	return model
}

// FindDraftModel finds a suitable draft model for speculative decoding
func FindDraftModel(models []ModelInfo, mainModel ModelInfo, memEstimator *MemoryEstimator) *ModelInfo {
	// Don't use draft models for small main models (not worth the overhead)
	if mainModel.Size != "" {
		switch mainModel.Size {
		case "0.5B", "1B", "1.5B", "3B", "7B", "8B":
			return nil // Too small to benefit from speculative decoding
		}
	}

	// Check if model type is suitable for draft pairing
	if !isSuitableForDraftModel(mainModel) {
		return nil
	}

	mainLower := strings.ToLower(mainModel.Name)
	var bestDraft *ModelInfo
	var bestScore float64

	for _, model := range models {
		// Skip if same model or if draft is larger/same size as main
		if model.Path == mainModel.Path || model.IsDraft {
			continue
		}

		// Skip projection files (.mmproj) - these are not suitable as draft models
		if strings.Contains(strings.ToLower(model.Name), "mmproj") {
			continue
		}

		// Skip embedding models as drafts
		if strings.Contains(strings.ToLower(model.Name), "embed") {
			continue
		}

		// Check if it's from the same model family
		draftLower := strings.ToLower(model.Name)
		sameFamily := false
		families := []string{"qwen", "llama", "codellama", "mistral", "phi", "gemma", "deepseek"}
		for _, family := range families {
			if strings.Contains(mainLower, family) && strings.Contains(draftLower, family) {
				sameFamily = true
				break
			}
		}

		if !sameFamily {
			continue
		}

		// Use memory estimator to compare actual model sizes if possible
		if memEstimator != nil {
			mainInfo, mainErr := memEstimator.GetModelMemoryInfo(mainModel.Path)
			draftInfo, draftErr := memEstimator.GetModelMemoryInfo(model.Path)

			if mainErr == nil && draftErr == nil {
				// Draft must be significantly smaller than main model
				if draftInfo.ModelSizeGB >= mainInfo.ModelSizeGB*0.8 {
					continue // Draft too large
				}

				// Calculate a score based on size ratio and family match
				sizeRatio := draftInfo.ModelSizeGB / mainInfo.ModelSizeGB
				score := (1.0 - sizeRatio) * 100 // Higher score for smaller draft models

				// Prefer models with similar instruction tuning
				if model.IsInstruct == mainModel.IsInstruct {
					score += 10
				}

				// Bonus for models specifically good for drafting
				if isDraftOptimal(model) {
					score += 20
				}

				if score > bestScore {
					bestScore = score
					bestDraft = &model
				}
				continue
			}
		}

		// Fallback to size-based comparison if memory estimator fails
		if mainModel.Size != "" && model.Size != "" {
			mainSizeOrder := getSizeOrder(mainModel.Size)
			draftSizeOrder := getSizeOrder(model.Size)

			// Draft must be smaller than main model
			if draftSizeOrder >= mainSizeOrder {
				continue
			}

			// Calculate score based on size difference
			sizeDiff := mainSizeOrder - draftSizeOrder
			score := float64(sizeDiff)

			// Prefer models with similar instruction tuning
			if model.IsInstruct == mainModel.IsInstruct {
				score += 0.5
			}

			// Bonus for models specifically good for drafting
			if isDraftOptimal(model) {
				score += 2.0
			}

			if score > bestScore {
				bestScore = score
				bestDraft = &model
			}
		}
	}

	return bestDraft
}

// isSuitableForDraftModel determines if a model type benefits from speculative decoding
func isSuitableForDraftModel(model ModelInfo) bool {
	lower := strings.ToLower(model.Name)

	// ✅ Perfect for draft models - predictable patterns
	if strings.Contains(lower, "chat") || strings.Contains(lower, "instruct") ||
		strings.Contains(lower, "assistant") || strings.Contains(lower, "tools") {
		return true
	}

	// ✅ Code models benefit from speculation due to structured syntax
	if strings.Contains(lower, "code") || strings.Contains(lower, "coder") ||
		strings.Contains(lower, "programming") {
		return true
	}

	// ❌ Skip embedding models - different use case
	if strings.Contains(lower, "embed") || strings.Contains(lower, "embedding") {
		return false
	}

	// ❌ Skip creative writing models - high entropy makes speculation less effective
	if strings.Contains(lower, "creative") || strings.Contains(lower, "storytelling") ||
		strings.Contains(lower, "writing") {
		return false
	}

	// ✅ Default to allowing for base models (they often benefit)
	return true
}

// isDraftOptimal checks if a model is particularly good as a draft model
func isDraftOptimal(model ModelInfo) bool {
	lower := strings.ToLower(model.Name)

	// Small, fast models are ideal drafts
	if model.Size != "" {
		switch model.Size {
		case "0.5B", "1B", "1.5B", "3B":
			return true
		}
	}

	// Models specifically designed for efficiency
	if strings.Contains(lower, "mini") || strings.Contains(lower, "nano") ||
		strings.Contains(lower, "tiny") || strings.Contains(lower, "fast") {
		return true
	}

	// Higher quantization for drafts is OK (faster inference)
	if model.Quantization != "" {
		switch model.Quantization {
		case "Q4_K_M", "Q4_K_S", "Q4_0", "Q3_K_M":
			return true
		}
	}

	return false
}

// getSizeOrder returns a numeric order for model sizes
func getSizeOrder(size string) int {
	sizeOrder := map[string]int{
		"0.5B": 1, "1B": 2, "1.5B": 3, "3B": 4, "7B": 5, "8B": 6,
		"13B": 7, "32B": 8, "70B": 9, "405B": 10,
	}
	if order, exists := sizeOrder[size]; exists {
		return order
	}
	return 0
}

// SortModelsBySize sorts models by size (largest first)
func SortModelsBySize(models []ModelInfo) []ModelInfo {
	sizeOrder := map[string]int{
		"405B": 9, "70B": 8, "32B": 7, "13B": 6, "8B": 5, "7B": 4, "3B": 3, "1.5B": 2, "1B": 1, "0.5B": 0,
	}

	sorted := make([]ModelInfo, len(models))
	copy(sorted, models)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			iOrder, iExists := sizeOrder[sorted[i].Size]
			jOrder, jExists := sizeOrder[sorted[j].Size]

			if !iExists {
				iOrder = -1
			}
			if !jExists {
				jOrder = -1
			}

			if jOrder > iOrder {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

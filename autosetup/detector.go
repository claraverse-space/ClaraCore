package autosetup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModelInfo represents information about a detected GGUF model
type ModelInfo struct {
	Name         string
	Path         string
	Size         string
	IsInstruct   bool
	IsDraft      bool
	Quantization string
}

// DetectModels scans a directory for GGUF files and returns model information
func DetectModels(modelsDir string) ([]ModelInfo, error) {
	var models []ModelInfo

	err := filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			model := parseGGUFFilename(path, info.Name())
			models = append(models, model)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan models directory: %v", err)
	}

	return models, nil
}

// parseGGUFFilename extracts model information from filename
func parseGGUFFilename(fullPath, filename string) ModelInfo {
	filename = strings.TrimSuffix(filename, ".gguf")
	lower := strings.ToLower(filename)

	model := ModelInfo{
		Name: filename,
		Path: fullPath,
	}

	// Detect if it's an instruct model
	if strings.Contains(lower, "instruct") {
		model.IsInstruct = true
	}

	// Detect quantization level
	quantizations := []string{"q8_0", "q6_k", "q5_k_m", "q5_k_s", "q4_k_m", "q4_k_s", "q4_0", "q3_k_m", "q2_k"}
	for _, quant := range quantizations {
		if strings.Contains(lower, quant) {
			model.Quantization = strings.ToUpper(quant)
			break
		}
	}

	// Detect size (0.5B, 1B, 3B, 7B, 13B, 32B, 70B, etc.)
	sizes := []string{"0.5b", "1b", "1.5b", "3b", "7b", "8b", "13b", "32b", "70b", "405b"}
	for _, size := range sizes {
		if strings.Contains(lower, size) {
			model.Size = strings.ToUpper(size)
			break
		}
	}

	// Determine if it's suitable as a draft model (smaller models)
	if model.Size != "" {
		switch model.Size {
		case "0.5B", "1B", "1.5B", "3B":
			model.IsDraft = true
		}
	}

	return model
}

// FindDraftModel finds a suitable draft model for speculative decoding
func FindDraftModel(models []ModelInfo, mainModel ModelInfo) *ModelInfo {
	for _, model := range models {
		if model.IsDraft && model.IsInstruct == mainModel.IsInstruct {
			// Try to find models from the same family
			mainLower := strings.ToLower(mainModel.Name)
			draftLower := strings.ToLower(model.Name)

			// Check if they're from the same model family
			families := []string{"qwen", "llama", "codellama", "mistral", "phi"}
			for _, family := range families {
				if strings.Contains(mainLower, family) && strings.Contains(draftLower, family) {
					return &model
				}
			}
		}
	}
	return nil
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

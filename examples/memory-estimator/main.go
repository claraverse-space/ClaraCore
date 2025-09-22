package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/prave/ClaraCore/autosetup"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run mem_test.go <path-to-gguf-file>")
		os.Exit(1)
	}

	modelPath := os.Args[1]

	fmt.Printf("Testing Go GGUF memory estimator with: %s\n", modelPath)

	// Create memory estimator
	estimator := autosetup.NewMemoryEstimator()

	// Get available VRAM
	fmt.Print("Detecting available VRAM... ")
	availableVRAM, err := estimator.GetAvailableVRAM()
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		availableVRAM = 12.0 // Use default
		fmt.Printf("Using default: %.1f GB\n", availableVRAM)
	} else {
		fmt.Printf("%.1f GB\n", availableVRAM)
	}

	// Analyze the model
	fmt.Println("\nAnalyzing model...")
	analysis, err := estimator.EstimateModelForVRAM(modelPath, availableVRAM)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Print analysis
	analysis.Print()

	// Test various context sizes
	fmt.Println("\n--- Memory Usage at Different Context Sizes ---")
	fmt.Printf("%-15s | %-15s | %-15s | %s\n", "Context Size", "KV Cache", "Total VRAM", "Status")
	fmt.Println(strings.Repeat("-", 70))

	contextSizes := []int{4096, 8192, 16384, 32768, 65536, 131072}
	memInfo, _ := estimator.GetModelMemoryInfo(modelPath)
	metadata, _ := autosetup.ReadGGUFMetadata(modelPath)

	for _, ctx := range contextSizes {
		if metadata.ContextLength > 0 && uint32(ctx) > metadata.ContextLength {
			continue
		}

		result := estimator.CalculateMemoryForContext(memInfo, ctx, metadata.BlockCount)
		status := "✗ Too large"
		if result.TotalMemoryGB <= availableVRAM {
			status = "✓ Fits"
		}

		fmt.Printf("%-15d | %-15s | %-15s | %s\n",
			ctx,
			autosetup.FormatMemoryGB(result.KVCacheGB),
			autosetup.FormatMemoryGB(result.TotalMemoryGB),
			status)
	}
}
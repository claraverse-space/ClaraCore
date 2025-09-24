package autosetup

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// SystemInfo contains information about the current system
type SystemInfo struct {
	OS           string
	Architecture string
	HasCUDA      bool
	HasROCm      bool
	HasVulkan    bool
	HasMetal     bool
	// Extended system information
	CPUCores      int
	PhysicalCores int
	TotalRAMGB    float64
	CUDAVersion   string
	ROCmVersion   string
	VRAMDetails   []GPUInfo
	TotalVRAMGB   float64
	HasMLX        bool
	HasIntel      bool
}

// GPUInfo contains information about individual GPUs
type GPUInfo struct {
	Name     string
	VRAMGB   float64
	Type     string // "CUDA", "ROCm", "MLX", "Intel"
	DeviceID int
}

// BinaryInfo contains information about the downloaded binary
type BinaryInfo struct {
	Path    string
	Version string
	Type    string // "cpu", "cuda", "rocm", "vulkan", "metal"
}

// BinaryMetadata stores information about the currently installed binary
type BinaryMetadata struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

const (
	LLAMA_CPP_RELEASE_URL   = "https://github.com/ggml-org/llama.cpp/releases/tag/b6527"
	LLAMA_CPP_DOWNLOAD_BASE = "https://github.com/ggml-org/llama.cpp/releases/download/b6527"
	BINARY_METADATA_FILE    = "binary_metadata.json"
)

// saveBinaryMetadata saves information about the installed binary
func saveBinaryMetadata(extractDir string, binaryInfo *BinaryInfo) error {
	metadata := BinaryMetadata{
		Type:    binaryInfo.Type,
		Version: binaryInfo.Version,
		Path:    binaryInfo.Path,
	}

	metadataPath := filepath.Join(extractDir, BINARY_METADATA_FILE)
	file, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

// loadBinaryMetadata loads information about the currently installed binary
func loadBinaryMetadata(extractDir string) (*BinaryMetadata, error) {
	metadataPath := filepath.Join(extractDir, BINARY_METADATA_FILE)
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, err // File doesn't exist or can't be read
	}
	defer file.Close()

	var metadata BinaryMetadata
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %v", err)
	}

	return &metadata, nil
}

// DetectSystem detects the current system capabilities
func DetectSystem() SystemInfo {
	system := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// Detect GPU capabilities
	system.HasCUDA = detectCUDA()
	system.HasROCm = detectROCm()
	system.HasVulkan = detectVulkan()
	system.HasMetal = detectMetal()

	return system
}

// GetOptimalBinaryURL returns the best binary download URL for the system
func GetOptimalBinaryURL(system SystemInfo) (string, string, error) {
	var filename, binaryType string

	switch system.OS {
	case "windows":
		if system.HasCUDA {
			filename = "llama-b6527-bin-win-cuda-12.4-x64.zip"
			binaryType = "cuda"
		} else if system.HasVulkan {
			filename = "llama-b6527-bin-win-vulkan-x64.zip"
			binaryType = "vulkan"
		} else {
			filename = "llama-b6527-bin-win-cpu-x64.zip"
			binaryType = "cpu"
		}
	case "linux":
		if system.HasCUDA {
			filename = "llama-b6527-bin-ubuntu-x64.zip"
			binaryType = "cuda"
		} else if system.HasVulkan {
			filename = "llama-b6527-bin-ubuntu-vulkan-x64.zip"
			binaryType = "vulkan"
		} else {
			filename = "llama-b6527-bin-ubuntu-x64.zip"
			binaryType = "cpu"
		}
	case "darwin":
		if system.Architecture == "arm64" {
			filename = "llama-b6527-bin-macos-arm64.zip"
			binaryType = "metal"
		} else {
			filename = "llama-b6527-bin-macos-x64.zip"
			binaryType = "cpu"
		}
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", system.OS)
	}

	url := fmt.Sprintf("%s/%s", LLAMA_CPP_DOWNLOAD_BASE, filename)
	return url, binaryType, nil
}

// DownloadBinary downloads and extracts the llama-server binary
func DownloadBinary(downloadDir string, system SystemInfo) (*BinaryInfo, error) {
	url, binaryType, err := GetOptimalBinaryURL(system)
	if err != nil {
		return nil, err
	}

	// Create download directory
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create download directory: %v", err)
	}

	extractDir := filepath.Join(downloadDir, "llama-server")

	// Check if binary already exists
	existingServerPath, err := findLlamaServer(extractDir)
	if err == nil {
		// Binary exists, check if it's the right type for our system
		fmt.Printf("✅ Found existing llama-server binary: %s\n", existingServerPath)

		// Check metadata to see if the existing binary matches the required type
		metadata, metaErr := loadBinaryMetadata(extractDir)
		if metaErr == nil && metadata.Type == binaryType {
			// Binary type matches, check for additional requirements
			if system.HasCUDA && system.OS == "windows" {
				cudartPath := filepath.Join(extractDir, "cudart64_12.dll")
				if _, err := os.Stat(cudartPath); err == nil {
					fmt.Printf("✅ Existing %s binary is compatible, skipping download\n", binaryType)
					return &BinaryInfo{
						Path:    existingServerPath,
						Version: "b6527",
						Type:    binaryType,
					}, nil
				} else {
					fmt.Printf("⚠️  CUDA runtime missing, will download both runtime and binary\n")
				}
			} else {
				// Non-CUDA system or metadata matches, existing binary is sufficient
				fmt.Printf("✅ Existing %s binary is compatible, skipping download\n", binaryType)
				return &BinaryInfo{
					Path:    existingServerPath,
					Version: "b6527",
					Type:    binaryType,
				}, nil
			}
		} else {
			// Binary type doesn't match or no metadata - need to re-download
			if metaErr == nil {
				fmt.Printf("🔄 Binary type mismatch: existing=%s, required=%s. Re-downloading...\n", metadata.Type, binaryType)
			} else {
				fmt.Printf("🔄 No binary metadata found. Re-downloading %s binary...\n", binaryType)
			}

			// Remove existing binary directory to ensure clean installation
			err = os.RemoveAll(extractDir)
			if err != nil {
				fmt.Printf("⚠️  Failed to remove existing binary directory: %v\n", err)
			} else {
				fmt.Printf("🗑️  Removed existing binary directory\n")
			}
		}
	}

	// If we get here, we need to download
	fmt.Printf("⬇️  Downloading llama-server binary...\n")

	// For CUDA on Windows, download both runtime and binary
	if system.HasCUDA && system.OS == "windows" {
		cudartURL := LLAMA_CPP_DOWNLOAD_BASE + "/cudart-llama-bin-win-cuda-12.4-x64.zip"
		fmt.Printf("Downloading CUDA runtime from: %s\n", cudartURL)

		// Download CUDA runtime
		cudartZipPath := filepath.Join(downloadDir, "cudart.zip")
		err = downloadFile(cudartURL, cudartZipPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download CUDA runtime: %v", err)
		}

		// Extract CUDA runtime
		err = extractZip(cudartZipPath, extractDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract CUDA runtime: %v", err)
		}
		os.Remove(cudartZipPath)

		fmt.Printf("Downloading llama-server (%s) from: %s\n", binaryType, url)

		// Download llama binary
		llamaZipPath := filepath.Join(downloadDir, "llama-server.zip")
		err = downloadFile(url, llamaZipPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download llama binary: %v", err)
		}

		// Extract llama binary to same directory
		err = extractZip(llamaZipPath, extractDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract llama binary: %v", err)
		}
		os.Remove(llamaZipPath)
	} else {
		// Single download for non-CUDA or non-Windows
		fmt.Printf("Downloading llama-server (%s) from: %s\n", binaryType, url)

		// Download the file
		zipPath := filepath.Join(downloadDir, "llama-server.zip")
		err = downloadFile(url, zipPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download binary: %v", err)
		}

		// Extract the zip file
		err = extractZip(zipPath, extractDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract binary: %v", err)
		}
		os.Remove(zipPath)
	}

	// Find the llama-server executable
	serverPath, err := findLlamaServer(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find llama-server executable: %v", err)
	}

	// Make it executable on Unix systems
	if system.OS != "windows" {
		err = os.Chmod(serverPath, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to make binary executable: %v", err)
		}
	}

	binaryInfo := &BinaryInfo{
		Path:    serverPath,
		Version: "b6527",
		Type:    binaryType,
	}

	// Save metadata about the downloaded binary
	err = saveBinaryMetadata(extractDir, binaryInfo)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to save binary metadata: %v\n", err)
		// Don't fail the entire process for metadata saving failure
	} else {
		fmt.Printf("📝 Saved binary metadata: %s type\n", binaryType)
	}

	return binaryInfo, nil
}

// downloadFile downloads a file from URL to local path
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractZip extracts a zip file to destination directory
func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	os.MkdirAll(dest, 0755)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.FileInfo().Mode())
			continue
		}

		os.MkdirAll(filepath.Dir(path), 0755)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, rc)
		if err != nil {
			return err
		}
	}

	return nil
}

// findLlamaServer finds the llama-server executable in extracted directory
func findLlamaServer(dir string) (string, error) {
	var serverPath string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := info.Name()
		if strings.Contains(name, "llama-server") || strings.Contains(name, "server") {
			if runtime.GOOS == "windows" && strings.HasSuffix(name, ".exe") {
				serverPath = path
				return filepath.SkipDir
			} else if runtime.GOOS != "windows" && !strings.Contains(name, ".") {
				serverPath = path
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if serverPath == "" {
		return "", fmt.Errorf("llama-server executable not found in extracted files")
	}

	return serverPath, nil
}

// Detection functions for different GPU types
func detectCUDA() bool {
	// Check for nvidia-smi command and try to query devices
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Check Windows paths for nvidia-smi
		paths := []string{
			"C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
			"C:\\Windows\\System32\\nvidia-smi.exe",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				// Found nvidia-smi, try to query for devices
				cmd = exec.Command(path, "--list-gpus")
				output, err := cmd.Output()
				if err == nil && len(output) > 0 {
					// Check if output contains actual GPU info
					return strings.Contains(string(output), "GPU")
				}
				// nvidia-smi exists but no devices found
				return false
			}
		}
	} else {
		// Check for nvidia-smi on Unix systems
		if _, err := os.Stat("/usr/bin/nvidia-smi"); err == nil {
			cmd = exec.Command("nvidia-smi", "--list-gpus")
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				return strings.Contains(string(output), "GPU")
			}
			return false
		}
	}

	return false
}

func detectROCm() bool {
	// Check for ROCm installation
	paths := []string{
		"/opt/rocm",
		"/usr/bin/rocm-smi",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

func detectVulkan() bool {
	// Check for Vulkan library
	if runtime.GOOS == "windows" {
		// Check for vulkan-1.dll in system32
		if _, err := os.Stat("C:\\Windows\\System32\\vulkan-1.dll"); err == nil {
			return true
		}
	} else {
		// Check for libvulkan.so
		paths := []string{
			"/usr/lib/x86_64-linux-gnu/libvulkan.so.1",
			"/usr/lib/libvulkan.so.1",
			"/usr/lib64/libvulkan.so.1",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}

	return false
}

func detectMetal() bool {
	// Metal is only available on macOS
	return runtime.GOOS == "darwin"
}

// Enhanced system detection functions

// EnhanceSystemInfo adds detailed system information to existing SystemInfo
func EnhanceSystemInfo(info *SystemInfo) error {
	// Add CPU information
	info.CPUCores = runtime.NumCPU()
	info.PhysicalCores = detectPhysicalCores()

	// Add RAM information
	info.TotalRAMGB = detectTotalRAM()

	// Enhanced GPU detection
	enhanceGPUDetection(info)

	return nil
}

// detectPhysicalCores detects the number of physical CPU cores
func detectPhysicalCores() int {
	switch runtime.GOOS {
	case "windows":
		return detectWindowsPhysicalCores()
	case "linux":
		return detectLinuxPhysicalCores()
	case "darwin":
		return detectMacOSPhysicalCores()
	default:
		return runtime.NumCPU() / 2 // Fallback assumption
	}
}

// detectWindowsPhysicalCores detects physical cores on Windows
func detectWindowsPhysicalCores() int {
	cmd := exec.Command("wmic", "cpu", "get", "NumberOfCores", "/value")
	output, err := cmd.Output()
	if err != nil {
		return runtime.NumCPU() / 2
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "NumberOfCores=") {
			coreStr := strings.TrimPrefix(line, "NumberOfCores=")
			coreStr = strings.TrimSpace(coreStr)
			if cores, err := strconv.Atoi(coreStr); err == nil {
				return cores
			}
		}
	}
	return runtime.NumCPU() / 2
}

// detectLinuxPhysicalCores detects physical cores on Linux
func detectLinuxPhysicalCores() int {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return runtime.NumCPU() / 2
	}
	defer file.Close()

	physicalIDs := make(map[string]bool)
	coresPerSocket := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "physical id") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				physicalIDs[strings.TrimSpace(parts[1])] = true
			}
		} else if strings.HasPrefix(line, "cpu cores") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				if cores, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					coresPerSocket = cores
				}
			}
		}
	}

	if len(physicalIDs) > 0 && coresPerSocket > 0 {
		return len(physicalIDs) * coresPerSocket
	}
	return runtime.NumCPU() / 2
}

// detectMacOSPhysicalCores detects physical cores on macOS
func detectMacOSPhysicalCores() int {
	cmd := exec.Command("sysctl", "-n", "hw.physicalcpu")
	output, err := cmd.Output()
	if err != nil {
		return runtime.NumCPU() / 2
	}

	coreStr := strings.TrimSpace(string(output))
	if cores, err := strconv.Atoi(coreStr); err == nil {
		return cores
	}
	return runtime.NumCPU() / 2
}

// detectTotalRAM detects total system RAM in GB
func detectTotalRAM() float64 {
	switch runtime.GOOS {
	case "windows":
		return detectWindowsRAM()
	case "linux":
		return detectLinuxRAM()
	case "darwin":
		return detectMacOSRAM()
	default:
		return 16.0 // Fallback
	}
}

// detectWindowsRAM detects RAM on Windows using modern PowerShell commands
func detectWindowsRAM() float64 {
	// Use PowerShell to get total physical memory capacity
	cmd := exec.Command("powershell", "-Command",
		"Get-CimInstance -ClassName Win32_PhysicalMemory | Measure-Object -Property Capacity -Sum | Select-Object -ExpandProperty Sum")
	output, err := cmd.Output()
	if err != nil {
		return 16.0
	}

	totalBytes, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 16.0
	}

	return totalBytes / (1024 * 1024 * 1024) // Convert bytes to GB
}

// detectLinuxRAM detects RAM on Linux
func detectLinuxRAM() float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 16.0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if memKB, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return float64(memKB) / (1024 * 1024)
				}
			}
		}
	}
	return 16.0
}

// detectMacOSRAM detects RAM on macOS
func detectMacOSRAM() float64 {
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return 16.0
	}

	memStr := strings.TrimSpace(string(output))
	if memBytes, err := strconv.ParseInt(memStr, 10, 64); err == nil {
		return float64(memBytes) / (1024 * 1024 * 1024)
	}
	return 16.0
}

// enhanceGPUDetection adds detailed GPU and VRAM information
func enhanceGPUDetection(info *SystemInfo) {
	// Enhanced CUDA detection
	if info.HasCUDA {
		enhanceCUDADetection(info)
	}

	// Enhanced ROCm detection
	if info.HasROCm {
		enhanceROCmDetection(info)
	}

	// MLX detection for Apple Silicon
	if runtime.GOOS == "darwin" {
		enhanceMLXDetection(info)
	}

	// Intel GPU detection
	enhanceIntelGPUDetection(info)

	// Calculate total VRAM
	for _, gpu := range info.VRAMDetails {
		info.TotalVRAMGB += gpu.VRAMGB
	}
}

// enhanceCUDADetection gets detailed NVIDIA GPU information
func enhanceCUDADetection(info *SystemInfo) {
	// Try nvidia-smi for detailed info
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// Get CUDA version
	versionCmd := exec.Command("nvcc", "--version")
	if versionOutput, err := versionCmd.Output(); err == nil {
		lines := strings.Split(string(versionOutput), "\n")
		for _, line := range lines {
			if strings.Contains(line, "release") {
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "release" && i+1 < len(parts) {
						info.CUDAVersion = strings.TrimSuffix(parts[i+1], ",")
						break
					}
				}
			}
		}
	}

	// Parse GPU info
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ", ")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[0])
			vramStr := strings.TrimSpace(parts[1])

			if vramMB, err := strconv.ParseFloat(vramStr, 64); err == nil {
				info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
					Name:     name,
					VRAMGB:   vramMB / 1024.0,
					Type:     "CUDA",
					DeviceID: i,
				})
			}
		}
	}
}

// enhanceROCmDetection gets detailed AMD GPU information
func enhanceROCmDetection(info *SystemInfo) {
	// Try rocm-smi
	cmd := exec.Command("rocm-smi", "--showproductname", "--showmeminfo", "vram")
	output, err := cmd.Output()
	if err != nil {
		// Fallback: assume basic AMD GPU
		info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
			Name:     "AMD GPU",
			VRAMGB:   8.0, // Conservative estimate
			Type:     "ROCm",
			DeviceID: 0,
		})
		return
	}

	// Parse ROCm GPU info (simplified)
	lines := strings.Split(string(output), "\n")
	deviceID := 0
	for _, line := range lines {
		if strings.Contains(line, "GPU") && strings.Contains(line, "MB") {
			// Basic parsing - would need more sophisticated parsing
			info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
				Name:     "AMD GPU",
				VRAMGB:   8.0, // Placeholder
				Type:     "ROCm",
				DeviceID: deviceID,
			})
			deviceID++
		}
	}
}

// enhanceMLXDetection detects Apple Metal/MLX capabilities
func enhanceMLXDetection(info *SystemInfo) {
	// Check for Metal support
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Metal") {
		info.HasMLX = true

		// Parse for Apple Silicon GPU info
		// This is simplified - real implementation would parse more details
		info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
			Name:     "Apple GPU",
			VRAMGB:   16.0, // Placeholder - shared memory
			Type:     "MLX",
			DeviceID: 0,
		})
	}
}

// enhanceIntelGPUDetection detects Intel integrated GPUs
func enhanceIntelGPUDetection(info *SystemInfo) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		if strings.Contains(strings.ToLower(string(output)), "intel") {
			info.HasIntel = true
			info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
				Name:     "Intel GPU",
				VRAMGB:   4.0, // Shared memory estimate
				Type:     "Intel",
				DeviceID: 0,
			})
		}
	case "linux":
		// Check for Intel GPU on Linux
		cmd := exec.Command("lspci", "-nn")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		if strings.Contains(strings.ToLower(string(output)), "intel") &&
			strings.Contains(strings.ToLower(string(output)), "graphics") {
			info.HasIntel = true
			info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
				Name:     "Intel GPU",
				VRAMGB:   4.0, // Shared memory estimate
				Type:     "Intel",
				DeviceID: 0,
			})
		}
	}
}

// ModelFileInfo contains detailed information about a model file
type ModelFileInfo struct {
	Path           string
	ActualSizeGB   float64
	LayerCount     int
	ContextLength  int
	Architecture   string
	ParameterCount string
	Quantization   string
	SlidingWindow  uint32
}

// GetModelFileInfo reads detailed information from a GGUF model file
func GetModelFileInfo(modelPath string) (*ModelFileInfo, error) {
	// Get file size
	fileInfo, err := os.Stat(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	actualSize := float64(fileInfo.Size()) / (1024 * 1024 * 1024) // Convert to GB

	// Handle multi-part models
	if strings.Contains(filepath.Base(modelPath), "-of-") {
		actualSize = getTotalMultiPartSize(modelPath)
	}

	// Read GGUF metadata
	metadata, err := ReadGGUFMetadata(modelPath)
	if err != nil {
		return &ModelFileInfo{
			Path:          modelPath,
			ActualSizeGB:  actualSize,
			LayerCount:    0,
			Quantization:  detectQuantizationFromFilename(modelPath),
			SlidingWindow: 0,
		}, nil // Return partial info even if GGUF reading fails
	}

	return &ModelFileInfo{
		Path:           modelPath,
		ActualSizeGB:   actualSize,
		LayerCount:     int(metadata.BlockCount),
		ContextLength:  int(metadata.ContextLength),
		Architecture:   metadata.Architecture,
		ParameterCount: metadata.ModelName,
		Quantization:   detectQuantizationFromFilename(modelPath),
		SlidingWindow:  metadata.SlidingWindow,
	}, nil
}

// getTotalMultiPartSize calculates total size of multi-part models
func getTotalMultiPartSize(modelPath string) float64 {
	dir := filepath.Dir(modelPath)
	base := filepath.Base(modelPath)

	// Extract pattern like "model-00001-of-00003.gguf"
	parts := strings.Split(base, "-")
	if len(parts) < 3 {
		return 0
	}

	var totalSize int64
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "-of-") && strings.HasSuffix(file.Name(), ".gguf") {
			if info, err := file.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}

	return float64(totalSize) / (1024 * 1024 * 1024)
}

// detectQuantizationFromFilename detects quantization type from filename
func detectQuantizationFromFilename(filename string) string {
	filename = strings.ToUpper(filename)

	quantTypes := []string{"Q4_K_M", "Q4_K_S", "Q5_K_M", "Q5_K_S", "Q8_0", "Q6_K", "IQ4_XS", "F16", "F32"}

	for _, qtype := range quantTypes {
		if strings.Contains(filename, qtype) {
			return qtype
		}
	}

	return "Unknown"
}

// PrintSystemInfo prints comprehensive system information
func PrintSystemInfo(info *SystemInfo) {
	fmt.Printf("🖥️  System Information:\n")
	fmt.Printf("   OS: %s/%s\n", info.OS, info.Architecture)
	fmt.Printf("   CPU Cores: %d logical, %d physical\n", info.CPUCores, info.PhysicalCores)
	fmt.Printf("   Total RAM: %.1f GB\n", info.TotalRAMGB)

	fmt.Printf("🎮 GPU Detection:\n")
	if info.HasCUDA {
		fmt.Printf("   ✅ NVIDIA CUDA detected")
		if info.CUDAVersion != "" {
			fmt.Printf(" (version %s)", info.CUDAVersion)
		}
		fmt.Printf("\n")
	}
	if info.HasROCm {
		fmt.Printf("   ✅ AMD ROCm detected")
		if info.ROCmVersion != "" {
			fmt.Printf(" (version %s)", info.ROCmVersion)
		}
		fmt.Printf("\n")
	}
	if info.HasMLX {
		fmt.Printf("   ✅ Apple MLX detected\n")
	}
	if info.HasIntel {
		fmt.Printf("   ✅ Intel GPU detected\n")
	}
	if info.HasVulkan {
		fmt.Printf("   ✅ Vulkan detected\n")
	}
	if info.HasMetal {
		fmt.Printf("   ✅ Metal detected\n")
	}

	if len(info.VRAMDetails) > 0 {
		fmt.Printf("   Total VRAM: %.1f GB\n", info.TotalVRAMGB)
		for i, gpu := range info.VRAMDetails {
			fmt.Printf("     GPU %d: %s (%.1f GB %s)\n", i, gpu.Name, gpu.VRAMGB, gpu.Type)
		}
	} else {
		fmt.Printf("   No dedicated GPUs detected\n")
	}
}

// PrintModelInfo prints detailed model information
func PrintModelInfo(models []ModelInfo, modelsPath string) {
	fmt.Printf("📁 Model Analysis:\n")

	var totalSizeGB float64
	validModels := 0

	for _, model := range models {
		if model.IsDraft {
			continue
		}

		modelInfo, err := GetModelFileInfo(model.Path)
		if err != nil {
			fmt.Printf("   ⚠️  %s: Failed to read file info\n", model.Name)
			continue
		}

		totalSizeGB += modelInfo.ActualSizeGB
		validModels++

		fmt.Printf("   📦 %s:\n", model.Name)
		fmt.Printf("      Size: %.2f GB\n", modelInfo.ActualSizeGB)
		if modelInfo.LayerCount > 0 {
			fmt.Printf("      Layers: %d\n", modelInfo.LayerCount)
		}
		if modelInfo.ContextLength > 0 {
			fmt.Printf("      Max Context: %d tokens\n", modelInfo.ContextLength)
		}
		if modelInfo.Architecture != "" {
			fmt.Printf("      Architecture: %s\n", modelInfo.Architecture)
		}
		if modelInfo.SlidingWindow > 0 {
			fmt.Printf("      SWA Support: Yes (window size: %d)\n", modelInfo.SlidingWindow)
		}
		fmt.Printf("      Quantization: %s\n", modelInfo.Quantization)
	}

	fmt.Printf("   📊 Summary: %d models, %.2f GB total\n", validModels, totalSizeGB)
}

// DebugMMProjMetadata reads and prints all metadata keys from mmproj files
func DebugMMProjMetadata(modelsPath string) {
	fmt.Printf("🔍 Scanning for mmproj files in: %s\n", modelsPath)

	// Find all mmproj files
	var mmprojFiles []string

	err := filepath.Walk(modelsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.Contains(strings.ToLower(info.Name()), "mmproj") && strings.HasSuffix(path, ".gguf") {
			mmprojFiles = append(mmprojFiles, path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		return
	}

	fmt.Printf("📦 Found %d mmproj files:\n", len(mmprojFiles))

	for i, mmprojPath := range mmprojFiles {
		fmt.Printf("\n--- mmproj file %d: %s ---\n", i+1, filepath.Base(mmprojPath))

		// Try to read GGUF metadata
		allKeys, err := ReadAllGGUFKeys(mmprojPath)
		if err != nil {
			fmt.Printf("❌ Failed to read metadata: %v\n", err)
			continue
		}

		fmt.Printf("📊 Total metadata keys found: %d\n", len(allKeys))
		fmt.Printf("🎯 Interesting keys:\n")

		// Print interesting keys for vision models
		interestingPrefixes := []string{
			"clip.",
			"vision.",
			"projector.",
			"original.",
			"general.",
			"model.",
			"llava.",
			"mm.",
		}

		for key, value := range allKeys {
			for _, prefix := range interestingPrefixes {
				if strings.HasPrefix(strings.ToLower(key), prefix) {
					fmt.Printf("   %s: %v\n", key, value)
					break
				}
			}
		}

		fmt.Printf("\n📝 All keys (first 50):\n")
		count := 0
		for key := range allKeys {
			if count >= 50 {
				fmt.Printf("   ... and %d more keys\n", len(allKeys)-50)
				break
			}
			fmt.Printf("   - %s\n", key)
			count++
		}
	}

	if len(mmprojFiles) == 0 {
		fmt.Printf("❌ No mmproj files found\n")
	}
}

// DebugModelMetadata reads and prints metadata keys from sample main model files to compare with mmproj
func DebugModelMetadata(models []ModelInfo) {
	fmt.Printf("\n🔍 Analyzing main model metadata for matching keys...\n")

	// Pick a few different models to analyze (max 3 for brevity)
	sampledModels := []ModelInfo{}
	for _, model := range models {
		if !model.IsDraft && len(sampledModels) < 3 {
			sampledModels = append(sampledModels, model)
		}
		if len(sampledModels) >= 3 {
			break
		}
	}

	if len(sampledModels) == 0 {
		fmt.Printf("❌ No valid models found for analysis\n")
		return
	}

	fmt.Printf("📦 Analyzing %d sample models:\n", len(sampledModels))

	for i, model := range sampledModels {
		fmt.Printf("\n--- Model %d: %s ---\n", i+1, model.Name)

		// Try to read GGUF metadata
		allKeys, err := ReadAllGGUFKeys(model.Path)
		if err != nil {
			fmt.Printf("❌ Failed to read metadata: %v\n", err)
			continue
		}

		fmt.Printf("📊 Total metadata keys found: %d\n", len(allKeys))
		fmt.Printf("🎯 Keys that might help match with mmproj:\n")

		// Print keys that might be useful for matching with mmproj files
		matchingPrefixes := []string{
			"general.",
			"llama.",
			"model.",
			"tokenizer.",
			"clip.",
			"vision.",
		}

		for key, value := range allKeys {
			for _, prefix := range matchingPrefixes {
				if strings.HasPrefix(strings.ToLower(key), prefix) {
					// Only show keys that might contain model identification info
					if strings.Contains(strings.ToLower(key), "name") ||
						strings.Contains(strings.ToLower(key), "base") ||
						strings.Contains(strings.ToLower(key), "type") ||
						strings.Contains(strings.ToLower(key), "arch") ||
						strings.Contains(strings.ToLower(key), "family") ||
						strings.Contains(strings.ToLower(key), "id") {
						fmt.Printf("   %s: %v\n", key, value)
					}
					break
				}
			}
		}

		fmt.Printf("\n📝 All general.* keys:\n")
		for key, value := range allKeys {
			if strings.HasPrefix(strings.ToLower(key), "general.") {
				fmt.Printf("   %s: %v\n", key, value)
			}
		}
	}
}

// MMProjMatch represents a matched mmproj file with a main model
type MMProjMatch struct {
	ModelPath    string
	ModelName    string
	MMProjPath   string
	MMProjName   string
	MatchType    string  // "architecture", "basename", "name_similarity"
	Confidence   float64 // 0.0 to 1.0
	MatchDetails string
}

// FindMMProjMatches finds and matches mmproj files with their corresponding main models
func FindMMProjMatches(models []ModelInfo, modelsPath string) []MMProjMatch {
	fmt.Printf("🔗 Searching for mmproj-to-model matches...\n")

	// Find all mmproj files
	var mmprojFiles []string
	err := filepath.Walk(modelsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.Contains(strings.ToLower(info.Name()), "mmproj") && strings.HasSuffix(path, ".gguf") {
			mmprojFiles = append(mmprojFiles, path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("❌ Error scanning for mmproj files: %v\n", err)
		return []MMProjMatch{}
	}

	var matches []MMProjMatch

	// For each mmproj file, try to find matching models
	for _, mmprojPath := range mmprojFiles {
		fmt.Printf("\n🔍 Analyzing mmproj: %s\n", filepath.Base(mmprojPath))

		// Read mmproj metadata
		mmprojMeta, err := ReadAllGGUFKeys(mmprojPath)
		if err != nil {
			fmt.Printf("   ❌ Failed to read mmproj metadata: %v\n", err)
			continue
		}

		// Extract key matching fields from mmproj
		mmprojArch := getStringValue(mmprojMeta, "clip.projector_type")
		mmprojName := getStringValue(mmprojMeta, "general.name")
		mmprojBasename := getStringValue(mmprojMeta, "general.basename")
		mmprojBaseModelName := getStringValue(mmprojMeta, "general.base_model.0.name")

		// For mmproj: look for projection dimensions
		mmprojEmbedDim := getIntValue(mmprojMeta, "clip.vision.projection_dim")

		fmt.Printf("   📋 mmproj fields: arch=%s, name=%s, basename=%s, base_model=%s, proj_dim=%d\n",
			mmprojArch, mmprojName, mmprojBasename, mmprojBaseModelName, mmprojEmbedDim)

		// Try to match with each main model
		for _, model := range models {
			if model.IsDraft {
				continue // Skip draft models (including other mmproj files)
			}

			// Read model metadata
			modelMeta, err := ReadAllGGUFKeys(model.Path)
			if err != nil {
				continue
			}

			// Extract key matching fields from model
			modelArch := getStringValue(modelMeta, "general.architecture")
			modelName := getStringValue(modelMeta, "general.name")
			modelBasename := getStringValue(modelMeta, "general.basename")
			modelBaseModelName := getStringValue(modelMeta, "general.base_model.0.name")

			// Try different matching strategies

			// 1. Architecture + name-based size matching (highest confidence)
			if mmprojArch != "" && modelArch != "" &&
				strings.EqualFold(mmprojArch, modelArch) {

				// Check if model size matches mmproj expectations
				nameCompatibility := isModelNameCompatibleWithMMProj(model.Name, mmprojEmbedDim)
				if nameCompatibility {
					matches = append(matches, MMProjMatch{
						ModelPath:    model.Path,
						ModelName:    model.Name,
						MMProjPath:   mmprojPath,
						MMProjName:   filepath.Base(mmprojPath),
						MatchType:    "architecture_name_compatible",
						Confidence:   0.90,
						MatchDetails: fmt.Sprintf("arch: %s → %s, name-size match for %d dim", mmprojArch, modelArch, mmprojEmbedDim),
					})
					fmt.Printf("   ✅ ARCH+NAME MATCH: %s (conf: 0.90) [%s arch, size compatible with %d dim]\n",
						model.Name, mmprojArch, mmprojEmbedDim)
					continue
				} else {
					fmt.Printf("   ⚠️  ARCH MATCH BUT SIZE INCOMPATIBLE: %s (model size doesn't match %d dim mmproj)\n",
						model.Name, mmprojEmbedDim)
					continue
				}
			}

			// 2. Direct basename matching (high confidence)
			if mmprojBasename != "" && modelBasename != "" &&
				strings.EqualFold(mmprojBasename, modelBasename) {
				matches = append(matches, MMProjMatch{
					ModelPath:    model.Path,
					ModelName:    model.Name,
					MMProjPath:   mmprojPath,
					MMProjName:   filepath.Base(mmprojPath),
					MatchType:    "basename",
					Confidence:   0.90,
					MatchDetails: fmt.Sprintf("basename: %s → %s", mmprojBasename, modelBasename),
				})
				fmt.Printf("   ✅ BASENAME MATCH: %s (conf: 0.90)\n", model.Name)
				continue
			}

			// 3. Name similarity matching (medium confidence)
			nameSimilarity := calculateNameSimilarity(mmprojName, modelName)
			if nameSimilarity > 0.7 {
				matches = append(matches, MMProjMatch{
					ModelPath:    model.Path,
					ModelName:    model.Name,
					MMProjPath:   mmprojPath,
					MMProjName:   filepath.Base(mmprojPath),
					MatchType:    "name_similarity",
					Confidence:   nameSimilarity,
					MatchDetails: fmt.Sprintf("name similarity: %.2f", nameSimilarity),
				})
				fmt.Printf("   ✅ NAME MATCH: %s (conf: %.2f)\n", model.Name, nameSimilarity)
				continue
			}

			// 4. Base model name similarity (medium confidence)
			if mmprojBaseModelName != "" && modelBaseModelName != "" {
				baseModelSimilarity := calculateNameSimilarity(mmprojBaseModelName, modelBaseModelName)
				if baseModelSimilarity > 0.7 {
					matches = append(matches, MMProjMatch{
						ModelPath:    model.Path,
						ModelName:    model.Name,
						MMProjPath:   mmprojPath,
						MMProjName:   filepath.Base(mmprojPath),
						MatchType:    "base_model_similarity",
						Confidence:   baseModelSimilarity,
						MatchDetails: fmt.Sprintf("base model similarity: %.2f", baseModelSimilarity),
					})
					fmt.Printf("   ✅ BASE MODEL MATCH: %s (conf: %.2f)\n", model.Name, baseModelSimilarity)
					continue
				}
			}
		}
	}

	// Report summary
	fmt.Printf("\n📊 Matching Results:\n")
	if len(matches) == 0 {
		fmt.Printf("   ❌ No mmproj matches found\n")
	} else {
		fmt.Printf("   ✅ Found %d mmproj matches:\n", len(matches))
		for i, match := range matches {
			fmt.Printf("   %d. %s ↔ %s\n", i+1, match.MMProjName, match.ModelName)
			fmt.Printf("      Type: %s, Confidence: %.2f, Details: %s\n",
				match.MatchType, match.Confidence, match.MatchDetails)
		}
	}

	return matches
}

// getStringValue safely extracts a string value from metadata map
func getStringValue(metadata map[string]interface{}, key string) string {
	if val, exists := metadata[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// calculateNameSimilarity calculates similarity between two names using fuzzy matching
func calculateNameSimilarity(name1, name2 string) float64 {
	if name1 == "" || name2 == "" {
		return 0.0
	}

	// Normalize names for comparison
	norm1 := strings.ToLower(strings.ReplaceAll(name1, "-", " "))
	norm2 := strings.ToLower(strings.ReplaceAll(name2, "-", " "))

	// Exact match
	if norm1 == norm2 {
		return 1.0
	}

	// Contains check (bidirectional)
	if strings.Contains(norm1, norm2) || strings.Contains(norm2, norm1) {
		return 0.8
	}

	// Word-based similarity
	words1 := strings.Fields(norm1)
	words2 := strings.Fields(norm2)

	commonWords := 0
	totalWords := len(words1) + len(words2)

	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 {
				commonWords++
				break
			}
		}
	}

	if totalWords == 0 {
		return 0.0
	}

	return float64(commonWords*2) / float64(totalWords)
}

// DebugEmbeddingDetection analyzes models to debug embedding detection using GGUF metadata
func DebugEmbeddingDetection(models []ModelInfo) {
	fmt.Printf("\n🔍 Debugging embedding model detection using GGUF metadata...\n")

	embeddingModels := []string{}
	chatModels := []string{}
	unknownModels := []string{}

	for _, model := range models {
		if model.IsDraft {
			continue // Skip draft models
		}

		fmt.Printf("\n--- Analyzing: %s ---\n", model.Name)

		// Read GGUF metadata
		metadata, err := ReadAllGGUFKeys(model.Path)
		if err != nil {
			fmt.Printf("❌ Failed to read metadata: %v\n", err)
			unknownModels = append(unknownModels, model.Name)
			continue
		}

		// Extract key fields for embedding detection
		architecture := getStringValue(metadata, "general.architecture")
		modelType := getStringValue(metadata, "tokenizer.ggml.model")
		contextLength := getIntValue(metadata, fmt.Sprintf("%s.context_length", architecture))
		embeddingLength := getIntValue(metadata, fmt.Sprintf("%s.embedding_length", architecture))
		poolingType := getStringValue(metadata, fmt.Sprintf("%s.pooling_type", architecture))
		hasRope := hasKey(metadata, fmt.Sprintf("%s.rope", architecture))
		hasHeadCount := hasKey(metadata, fmt.Sprintf("%s.head_count", architecture))

		fmt.Printf("   📋 Metadata Analysis:\n")
		fmt.Printf("      Architecture: %s\n", architecture)
		fmt.Printf("      Model Type: %s\n", modelType)
		fmt.Printf("      Context Length: %d\n", contextLength)
		fmt.Printf("      Embedding Length: %d\n", embeddingLength)
		fmt.Printf("      Pooling Type: %s\n", poolingType)
		fmt.Printf("      Has RoPE: %t\n", hasRope)
		fmt.Printf("      Has Head Count: %t\n", hasHeadCount)

		// Apply embedding detection logic
		isEmbedding := detectEmbeddingFromMetadata(metadata, architecture)
		currentlyDetectedAsEmbedding := strings.Contains(strings.ToLower(model.Name), "embed")

		fmt.Printf("   🎯 Detection Results:\n")
		fmt.Printf("      New Algorithm: %s\n", boolToEmoji(isEmbedding))
		fmt.Printf("      Current Algorithm: %s\n", boolToEmoji(currentlyDetectedAsEmbedding))

		if isEmbedding != currentlyDetectedAsEmbedding {
			fmt.Printf("   ⚠️  MISMATCH DETECTED!\n")
		}

		if isEmbedding {
			embeddingModels = append(embeddingModels, model.Name)
		} else {
			chatModels = append(chatModels, model.Name)
		}
	}

	// Summary
	fmt.Printf("\n📊 Detection Summary:\n")
	fmt.Printf("   📝 Embedding Models (%d):\n", len(embeddingModels))
	for _, name := range embeddingModels {
		fmt.Printf("      - %s\n", name)
	}
	fmt.Printf("   💬 Chat Models (%d):\n", len(chatModels))
	for _, name := range chatModels {
		fmt.Printf("      - %s\n", name)
	}
	if len(unknownModels) > 0 {
		fmt.Printf("   ❓ Unknown Models (%d):\n", len(unknownModels))
		for _, name := range unknownModels {
			fmt.Printf("      - %s\n", name)
		}
	}
}

// detectEmbeddingFromMetadata uses comprehensive GGUF metadata to detect embedding models
func detectEmbeddingFromMetadata(metadata map[string]interface{}, architecture string) bool {
	// 1. Architecture check - dead giveaway
	switch strings.ToLower(architecture) {
	case "bert", "roberta", "nomic-bert", "jina-bert":
		return true
	case "llama", "mistral", "qwen", "gemma", "gemma3", "qwen2", "qwen3", "glm4moe", "seed_oss", "gpt-oss":
		return false
	}

	// 2. Pooling type check - smoking gun
	poolingType := getStringValue(metadata, fmt.Sprintf("%s.pooling_type", architecture))
	if poolingType != "" {
		// If pooling_type exists, it's definitely an embedding model
		return true
	}

	// 3. Context length patterns
	contextLength := getIntValue(metadata, fmt.Sprintf("%s.context_length", architecture))
	if contextLength > 0 {
		if contextLength <= 8192 {
			// Typical embedding model context length
			// But need more evidence since some chat models also have small context
		} else if contextLength >= 32768 {
			// Definitely a chat model
			return false
		}
	}

	// 4. Embedding dimension patterns
	embeddingLength := getIntValue(metadata, fmt.Sprintf("%s.embedding_length", architecture))
	if embeddingLength > 0 && embeddingLength <= 1024 {
		// Small embedding dimensions typical of embedding models
		// But check for other evidence
	}

	// 5. Missing chat model keys
	hasRope := hasKey(metadata, fmt.Sprintf("%s.rope", architecture))
	hasHeadCount := hasKey(metadata, fmt.Sprintf("%s.head_count", architecture))

	// Chat models typically have these, embedding models don't
	if !hasRope && !hasHeadCount && embeddingLength <= 1024 {
		return true
	}

	// 6. Tokenizer model check
	tokenizerModel := getStringValue(metadata, "tokenizer.ggml.model")
	if strings.Contains(strings.ToLower(tokenizerModel), "bert") {
		return true
	}

	// 7. Name-based fallback (least reliable)
	modelName := getStringValue(metadata, "general.name")
	lowerName := strings.ToLower(modelName)
	if strings.Contains(lowerName, "embed") ||
		strings.Contains(lowerName, "embedding") ||
		strings.HasPrefix(lowerName, "e5") ||
		strings.HasPrefix(lowerName, "bge") ||
		strings.HasPrefix(lowerName, "gte") {
		return true
	}

	// Default to chat model if no clear embedding indicators
	return false
}

// Helper functions for metadata analysis
func getIntValue(metadata map[string]interface{}, key string) int {
	if val, exists := metadata[key]; exists {
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		case float32:
			return int(v)
		case uint32:
			return int(v)
		case uint64:
			return int(v)
		}
	}
	return 0
}

func hasKey(metadata map[string]interface{}, keyPrefix string) bool {
	for key := range metadata {
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(keyPrefix)) {
			return true
		}
	}
	return false
}

func boolToEmoji(b bool) string {
	if b {
		return "✅ Embedding"
	}
	return "💬 Chat"
}

// isModelNameCompatibleWithMMProj checks if model name suggests compatibility with mmproj projection dimension
func isModelNameCompatibleWithMMProj(modelName string, mmprojEmbedDim int) bool {
	lowerName := strings.ToLower(modelName)

	// Extract size indicators from model name
	if strings.Contains(lowerName, "27b") || strings.Contains(lowerName, "22b") || strings.Contains(lowerName, "30b") {
		// Large models - should work with 5376 dimension mmproj
		return mmprojEmbedDim == 5376
	}

	if strings.Contains(lowerName, "9b") || strings.Contains(lowerName, "8b") || strings.Contains(lowerName, "7b") {
		// Medium models - should work with 3584 dimension mmproj
		return mmprojEmbedDim == 3584
	}

	if strings.Contains(lowerName, "4b") || strings.Contains(lowerName, "3b") || strings.Contains(lowerName, "2b") {
		// Small models - should work with 2560 dimension mmproj
		return mmprojEmbedDim == 2560
	}

	// Special cases for models with size indicators
	if strings.Contains(lowerName, "1b") || strings.Contains(lowerName, "0.6b") || strings.Contains(lowerName, "0.5b") {
		// Very small models - likely compatible with smaller mmproj
		return mmprojEmbedDim <= 2560
	}

	// If we can't determine size from name, check for other patterns
	// InternVL, LLaVA, etc. might have different naming conventions
	if strings.Contains(lowerName, "14b") {
		// 14B models often use 5120 projection dimension
		return mmprojEmbedDim == 5120 || mmprojEmbedDim == 5376
	}

	// For unknown sizes, be more permissive but still check for obvious mismatches
	// Don't match very large mmproj (5376) with obviously small model names
	if mmprojEmbedDim == 5376 && (strings.Contains(lowerName, "nano") || strings.Contains(lowerName, "tiny") || strings.Contains(lowerName, "mini")) {
		return false
	}

	// Default to allowing the match if we can't determine incompatibility
	return true
}

package autosetup

import (
	"archive/zip"
	"bufio"
	"context"
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
	"time"
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

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	CreatedAt  string `json:"created_at"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

const (
	LLAMA_CPP_GITHUB_API      = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"
	LLAMA_CPP_CURRENT_VERSION = "b6527" // Fallback version
	BINARY_METADATA_FILE      = "binary_metadata.json"
)

// GetLatestReleaseVersion fetches the latest llama.cpp release version from GitHub
func GetLatestReleaseVersion() (string, error) {
	fmt.Printf("🔍 Checking for latest llama.cpp release...\n")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(LLAMA_CPP_GITHUB_API)
	if err != nil {
		fmt.Printf("⚠️  Failed to check latest release, using fallback version %s\n", LLAMA_CPP_CURRENT_VERSION)
		return LLAMA_CPP_CURRENT_VERSION, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("⚠️  GitHub API returned %d, using fallback version %s\n", resp.StatusCode, LLAMA_CPP_CURRENT_VERSION)
		return LLAMA_CPP_CURRENT_VERSION, nil
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Printf("⚠️  Failed to parse release info, using fallback version %s\n", LLAMA_CPP_CURRENT_VERSION)
		return LLAMA_CPP_CURRENT_VERSION, nil
	}

	// Validate the tag name format (should be like "b6527" or "v1.0.0")
	version := release.TagName
	if version == "" {
		fmt.Printf("⚠️  Empty version tag, using fallback version %s\n", LLAMA_CPP_CURRENT_VERSION)
		return LLAMA_CPP_CURRENT_VERSION, nil
	}

	fmt.Printf("✅ Latest release found: %s\n", version)
	return version, nil
}

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

// LoadBinaryMetadata loads information about the currently installed binary
func LoadBinaryMetadata(extractDir string) (*BinaryMetadata, error) {
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

// Utility functions for command checking and logging

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// shouldUseEmoji returns whether to use emoji in output based on environment
func shouldUseEmoji() bool {
	// Check NO_EMOJI environment variable
	if os.Getenv("NO_EMOJI") != "" || os.Getenv("CLARACORE_NO_EMOJI") != "" {
		return false
	}

	// Disable emoji in CI/CD environments
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return false
	}

	return true
}

// formatLogPrefix returns appropriate prefix based on emoji setting
func formatLogPrefix(emojiPrefix, textPrefix string) string {
	if shouldUseEmoji() {
		return emojiPrefix
	}
	return textPrefix
}

// runCommandWithTimeout runs a command with a timeout
func runCommandWithTimeout(name string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", timeout)
	}

	return output, err
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
func GetOptimalBinaryURL(system SystemInfo, forceBackend string, version string) (string, string, error) {
	var filename, binaryType string

	// If version is empty, get the latest version
	if version == "" {
		var err error
		version, err = GetLatestReleaseVersion()
		if err != nil {
			version = LLAMA_CPP_CURRENT_VERSION
		}
	}

	// If a backend is forced, use that instead of auto-detection
	if forceBackend != "" {
		binaryType = forceBackend
		fmt.Printf("🎯 Using forced backend: %s\n", forceBackend)
	} else {
		// Auto-detect best backend for the system
		switch system.OS {
		case "windows":
			// Windows: CUDA > ROCm > Vulkan > CPU
			if system.HasCUDA {
				binaryType = "cuda"
			} else if system.HasROCm {
				binaryType = "rocm"
			} else if system.HasVulkan {
				binaryType = "vulkan"
			} else {
				binaryType = "cpu"
			}
		case "linux":
			// Linux: CUDA > ROCm > Vulkan > CPU
			if system.HasCUDA {
				binaryType = "cuda"
			} else if system.HasROCm {
				binaryType = "rocm"
			} else if system.HasVulkan {
				binaryType = "vulkan"
			} else {
				binaryType = "cpu"
			}
		case "darwin":
			// macOS: Metal (Apple Silicon) > CPU (Intel)
			if system.Architecture == "arm64" {
				binaryType = "metal"
			} else {
				binaryType = "cpu"
			}
		default:
			return "", "", fmt.Errorf("unsupported operating system: %s", system.OS)
		}
	}

	// Now determine the filename based on the chosen backend and version
	switch system.OS {
	case "windows":
		switch binaryType {
		case "cuda":
			filename = fmt.Sprintf("llama-%s-bin-win-cuda-12.4-x64.zip", version)
		case "rocm":
			filename = fmt.Sprintf("llama-%s-bin-win-rocm-x64.zip", version)
		case "vulkan":
			filename = fmt.Sprintf("llama-%s-bin-win-vulkan-x64.zip", version)
		case "cpu":
			filename = fmt.Sprintf("llama-%s-bin-win-cpu-x64.zip", version)
		default:
			return "", "", fmt.Errorf("unsupported backend '%s' for Windows", binaryType)
		}
	case "linux":
		switch binaryType {
		case "cuda":
			// Use CPU binary which includes all backends (CUDA, Vulkan, ROCm, CPU)
			filename = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", version)
		case "rocm":
			// Use CPU binary which includes all backends
			filename = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", version)
		case "vulkan":
			// Use CPU binary which includes all backends
			filename = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", version)
		case "cpu":
			filename = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", version)
		default:
			return "", "", fmt.Errorf("unsupported backend '%s' for Linux", binaryType)
		}
	case "darwin":
		switch binaryType {
		case "metal":
			filename = fmt.Sprintf("llama-%s-bin-macos-arm64.zip", version)
		case "cpu":
			if system.Architecture == "arm64" {
				filename = fmt.Sprintf("llama-%s-bin-macos-arm64.zip", version)
			} else {
				filename = fmt.Sprintf("llama-%s-bin-macos-x64.zip", version)
			}
		default:
			return "", "", fmt.Errorf("unsupported backend '%s' for macOS", binaryType)
		}
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", system.OS)
	}

	downloadBase := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s", version)
	url := fmt.Sprintf("%s/%s", downloadBase, filename)
	return url, binaryType, nil
}

// removeDirectoryRobust attempts to remove a directory with retry logic for Windows file locking issues
func removeDirectoryRobust(dir string) error {
	// First, try to kill any running llama-server processes
	if runtime.GOOS == "windows" {
		killLlamaServerProcesses()
	}

	maxRetries := 5
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := os.RemoveAll(dir)
		if err == nil {
			return nil
		}

		// If it's not a permission error, don't retry
		if !strings.Contains(err.Error(), "Access is denied") &&
			!strings.Contains(err.Error(), "being used by another process") {
			return err
		}

		if attempt < maxRetries-1 {
			fmt.Printf("⏳ Retry %d/%d: Waiting for file handles to be released...\n", attempt+1, maxRetries)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed to remove directory after %d attempts", maxRetries)
}

// killLlamaServerProcesses kills any running llama-server processes on Windows
func killLlamaServerProcesses() {
	if runtime.GOOS != "windows" {
		return
	}

	// Only kill llama-server.exe processes, not claracore.exe
	cmd := exec.Command("taskkill", "/F", "/IM", "llama-server.exe")
	err := cmd.Run()
	if err == nil {
		fmt.Printf("🔄 Terminated running llama-server processes\n")
	}

	// Give a moment for cleanup
	time.Sleep(200 * time.Millisecond)
}

// DownloadBinary downloads and extracts the llama-server binary
func DownloadBinary(downloadDir string, system SystemInfo, forceBackend string) (*BinaryInfo, error) {
	// Get the latest version
	version, err := GetLatestReleaseVersion()
	if err != nil {
		version = LLAMA_CPP_CURRENT_VERSION
	}

	url, binaryType, err := GetOptimalBinaryURL(system, forceBackend, version)
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
	fmt.Printf("🔍 Checking for existing binary in: %s\n", extractDir)
	existingServerPath, err := FindLlamaServer(extractDir)
	if err == nil {
		// Binary exists, check if it's the right type and version
		fmt.Printf("✅ Found existing llama-server binary: %s\n", existingServerPath)

		// Check metadata to see if the existing binary matches the required type and version
		metadata, metaErr := LoadBinaryMetadata(extractDir)
		if metaErr == nil && metadata.Type == binaryType && metadata.Version == version {
			// Binary type and version match, check for additional requirements
			if system.HasCUDA && system.OS == "windows" {
				cudartPath := filepath.Join(extractDir, "cudart64_12.dll")
				if _, err := os.Stat(cudartPath); err == nil {
					fmt.Printf("✅ Existing %s binary (v%s) is compatible, skipping download\n", binaryType, version)
					return &BinaryInfo{
						Path:    existingServerPath,
						Version: version,
						Type:    binaryType,
					}, nil
				} else {
					fmt.Printf("⚠️  CUDA runtime missing, will download both runtime and binary\n")
				}
			} else {
				// Non-CUDA system or metadata matches, existing binary is sufficient
				fmt.Printf("✅ Existing %s binary (v%s) is compatible, skipping download\n", binaryType, version)
				return &BinaryInfo{
					Path:    existingServerPath,
					Version: version,
					Type:    binaryType,
				}, nil
			}
		} else {
			// Binary type doesn't match, version is outdated, or no metadata - need to re-download
			if metaErr == nil {
				if metadata.Version != version {
					fmt.Printf("🔄 Version update available: %s -> %s. Re-downloading...\n", metadata.Version, version)
				} else {
					fmt.Printf("🔄 Binary type mismatch: existing=%s, required=%s. Re-downloading...\n", metadata.Type, binaryType)
				}
			} else {
				fmt.Printf("🔄 No binary metadata found. Re-downloading %s binary (v%s)...\n", binaryType, version)
			}

			// Remove existing binary directory to ensure clean installation
			err = removeDirectoryRobust(extractDir)
			if err != nil {
				fmt.Printf("⚠️  Failed to remove existing binary directory: %v\n", err)
				fmt.Printf("💡 This can happen if binary files are locked by Windows.\n")
				fmt.Printf("   Try:\n")
				fmt.Printf("   1. Restart ClaraCore\n")
				fmt.Printf("   2. Wait a few seconds and try again\n")
				fmt.Printf("   3. Manually delete the 'binaries' folder if needed\n")
				// Continue with download anyway - it might still work
			} else {
				fmt.Printf("🗑️  Removed existing binary directory\n")
			}
		}
	}

	// If we get here, we need to download
	fmt.Printf("⬇️  Downloading llama-server binary (v%s)...\n", version)

	// For CUDA on Windows, download both runtime and binary
	if system.HasCUDA && system.OS == "windows" {
		cudartURL := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/cudart-llama-bin-win-cuda-12.4-x64.zip", version)
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
	fmt.Printf("🔍 Searching for llama-server executable in: %s\n", extractDir)
	serverPath, err := FindLlamaServer(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find llama-server executable: %v", err)
	}
	fmt.Printf("✅ Found llama-server at: %s\n", serverPath)

	// Make it executable on Unix systems
	if system.OS != "windows" {
		err = os.Chmod(serverPath, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to make binary executable: %v", err)
		}
	}

	binaryInfo := &BinaryInfo{
		Path:    serverPath,
		Version: version,
		Type:    binaryType,
	}

	// Save metadata about the downloaded binary
	err = saveBinaryMetadata(extractDir, binaryInfo)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to save binary metadata: %v\n", err)
		// Don't fail the entire process for metadata saving failure
	} else {
		fmt.Printf("📝 Saved binary metadata: %s type, version %s\n", binaryType, version)
	}

	return binaryInfo, nil
}

// ForceDownloadBinary forces a download and re-extraction of the llama-server binary, bypassing existing files
func ForceDownloadBinary(downloadDir string, system SystemInfo, forceBackend string) (*BinaryInfo, error) {
	// Get the latest version
	version, err := GetLatestReleaseVersion()
	if err != nil {
		version = LLAMA_CPP_CURRENT_VERSION
	}

	url, binaryType, err := GetOptimalBinaryURL(system, forceBackend, version)
	if err != nil {
		return nil, err
	}

	// Create download directory
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create download directory: %v", err)
	}

	extractDir := filepath.Join(downloadDir, "llama-server")

	// Force remove existing binary directory
	fmt.Printf("🗑️  Removing existing binary directory for forced update...\n")
	err = removeDirectoryRobust(extractDir)
	if err != nil {
		fmt.Printf("⚠️  Failed to remove existing binary directory: %v\n", err)
		// Continue with download anyway - it might still work
	} else {
		fmt.Printf("🗑️  Removed existing binary directory\n")
	}

	// Always download fresh binary
	fmt.Printf("⬇️  Force downloading llama-server binary (%s v%s)...\n", binaryType, version)

	// For CUDA on Windows, download both runtime and binary
	if system.HasCUDA && system.OS == "windows" {
		cudartURL := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/cudart-llama-bin-win-cuda-12.4-x64.zip", version)
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
	fmt.Printf("🔍 Searching for llama-server executable in: %s\n", extractDir)
	serverPath, err := FindLlamaServer(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find llama-server executable: %v", err)
	}
	fmt.Printf("✅ Found llama-server at: %s\n", serverPath)

	// Make it executable on Unix systems
	if system.OS != "windows" {
		err = os.Chmod(serverPath, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to make binary executable: %v", err)
		}
	}

	binaryInfo := &BinaryInfo{
		Path:    serverPath,
		Version: version,
		Type:    binaryType,
	}

	// Save metadata about the downloaded binary
	err = saveBinaryMetadata(extractDir, binaryInfo)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to save binary metadata: %v\n", err)
		// Don't fail the entire process for metadata saving failure
	} else {
		fmt.Printf("📝 Saved binary metadata: %s type, version %s\n", binaryType, version)
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

// FindLlamaServer finds the llama-server executable in extracted directory
func FindLlamaServer(dir string) (string, error) {
	var serverPath string

	// Priority order for searching llama-server executable
	searchPaths := []string{
		filepath.Join(dir, "build", "bin"), // Most common: build/bin/llama-server
		filepath.Join(dir, "bin"),          // Alternative: bin/llama-server
		filepath.Join(dir),                 // Root: llama-server
	}

	// Define possible executable names based on OS
	var executableNames []string
	if runtime.GOOS == "windows" {
		executableNames = []string{
			"llama-server.exe",
			"server.exe",
			"main.exe", // Some builds use main.exe
		}
	} else {
		executableNames = []string{
			"llama-server",
			"server",
			"main", // Some builds use main
		}
	}

	// Search each path in priority order
	for _, searchPath := range searchPaths {
		for _, execName := range executableNames {
			candidatePath := filepath.Join(searchPath, execName)
			if _, err := os.Stat(candidatePath); err == nil {
				// Found the executable, verify it's actually executable
				if runtime.GOOS != "windows" {
					if info, err := os.Stat(candidatePath); err == nil {
						if info.Mode()&0111 != 0 { // Check if executable bit is set
							return candidatePath, nil
						}
					}
				} else {
					// On Windows, if file exists and has .exe extension, it's executable
					return candidatePath, nil
				}
			}
		}
	}

	// Fallback: Walk the entire directory tree as before (for unusual structures)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := info.Name()
		// Look for any file that might be the server executable
		if strings.Contains(strings.ToLower(name), "llama-server") ||
			strings.Contains(strings.ToLower(name), "server") ||
			(strings.Contains(strings.ToLower(name), "main") && !strings.Contains(strings.ToLower(name), ".")) {

			if runtime.GOOS == "windows" && strings.HasSuffix(name, ".exe") {
				serverPath = path
				return filepath.SkipDir
			} else if runtime.GOOS != "windows" && !strings.Contains(name, ".") {
				// Verify it's executable on Unix systems
				if info.Mode()&0111 != 0 {
					serverPath = path
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if serverPath == "" {
		return "", fmt.Errorf("llama-server executable not found in extracted files. Searched paths: %v", searchPaths)
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
	detectionLog := []string{}

	switch runtime.GOOS {
	case "windows":
		// Check Windows ROCm installation paths
		paths := []string{
			"C:\\Program Files\\AMD\\ROCm\\5.7\\bin\\rocm-smi.exe",
			"C:\\Program Files\\AMD\\ROCm\\5.6\\bin\\rocm-smi.exe",
			"C:\\Program Files\\AMD\\ROCm\\5.5\\bin\\rocm-smi.exe",
			"C:\\AMD\\ROCm\\bin\\rocm-smi.exe",
		}

		detectionLog = append(detectionLog, "🔍 ROCm Detection: Checking Windows paths...")

		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				detectionLog = append(detectionLog, fmt.Sprintf("   ✓ Found rocm-smi at: %s", path))
				// Found rocm-smi, try to query for devices
				cmd := exec.Command(path, "--showid")
				output, err := cmd.Output()
				if err != nil {
					detectionLog = append(detectionLog, fmt.Sprintf("   ⚠ rocm-smi command failed: %v", err))
					// Don't return false immediately - try fallback methods
					continue
				}
				if len(output) > 0 && strings.Contains(string(output), "GPU") {
					detectionLog = append(detectionLog, "   ✅ ROCm GPU devices detected!")
					fmt.Println(strings.Join(detectionLog, "\n"))
					return true
				}
				detectionLog = append(detectionLog, "   ⚠ rocm-smi found but no GPU devices detected")
			}
		}

		// Fallback 1: Check for AMD GPU with ROCm driver
		detectionLog = append(detectionLog, "   🔄 Trying fallback: Checking for AMD GPU...")
		cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
		output, err := cmd.Output()
		if err == nil && strings.Contains(strings.ToLower(string(output)), "amd") {
			detectionLog = append(detectionLog, "   ✓ AMD GPU found")
			// Check for ROCm runtime
			if _, err := os.Stat("C:\\Windows\\System32\\amdhip64.dll"); err == nil {
				detectionLog = append(detectionLog, "   ✅ ROCm runtime (amdhip64.dll) detected!")
				fmt.Println(strings.Join(detectionLog, "\n"))
				return true
			}
			detectionLog = append(detectionLog, "   ⚠ AMD GPU found but ROCm runtime not detected")
		}

		// Fallback 2: Check for HIP environment variables
		if os.Getenv("HIP_PATH") != "" || os.Getenv("ROCM_PATH") != "" {
			detectionLog = append(detectionLog, "   ✅ ROCm environment variables detected!")
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}

	case "linux":
		// Check for ROCm installation on Linux
		detectionLog = append(detectionLog, "🔍 ROCm Detection: Checking Linux paths...")

		paths := []string{
			"/opt/rocm/bin/rocm-smi",
			"/usr/bin/rocm-smi",
		}

		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				detectionLog = append(detectionLog, fmt.Sprintf("   ✓ Found rocm-smi at: %s", path))
				// Try to query ROCm devices
				cmd := exec.Command(path, "--showid")
				output, err := cmd.Output()
				if err != nil {
					detectionLog = append(detectionLog, fmt.Sprintf("   ⚠ rocm-smi command failed: %v", err))
					// Don't return false - try fallback methods
					continue
				}
				if len(output) > 0 && strings.Contains(string(output), "GPU") {
					detectionLog = append(detectionLog, "   ✅ ROCm GPU devices detected!")
					fmt.Println(strings.Join(detectionLog, "\n"))
					return true
				}
				detectionLog = append(detectionLog, "   ⚠ rocm-smi found but no GPU devices detected")
			}
		}

		// Check if /opt/rocm directory exists (indicates ROCm installation)
		if _, err := os.Stat("/opt/rocm"); err == nil {
			detectionLog = append(detectionLog, "   ✓ ROCm installation directory found")
		}

		// Fallback 1: Check for AMD GPU with ROCm driver
		detectionLog = append(detectionLog, "   🔄 Trying fallback: Checking for AMD GPU...")
		cmd := exec.Command("lspci", "-nn")
		output, err := cmd.Output()
		if err == nil {
			outputStr := strings.ToLower(string(output))
			if strings.Contains(outputStr, "amd") && (strings.Contains(outputStr, "display") || strings.Contains(outputStr, "vga")) {
				detectionLog = append(detectionLog, "   ✓ AMD GPU found")
				// Check for ROCm runtime libraries
				rocmLibPaths := []string{
					"/usr/lib/x86_64-linux-gnu/libamdhip64.so",
					"/opt/rocm/lib/libamdhip64.so",
					"/usr/lib64/libamdhip64.so",
				}
				for _, libPath := range rocmLibPaths {
					if _, err := os.Stat(libPath); err == nil {
						detectionLog = append(detectionLog, fmt.Sprintf("   ✅ ROCm runtime found at: %s", libPath))
						fmt.Println(strings.Join(detectionLog, "\n"))
						return true
					}
				}
				detectionLog = append(detectionLog, "   ⚠ AMD GPU found but ROCm runtime not detected")
			}
		} else {
			detectionLog = append(detectionLog, fmt.Sprintf("   ⚠ lspci command failed: %v", err))
		}

		// Fallback 2: Check for HIP environment variables
		if os.Getenv("HIP_PATH") != "" || os.Getenv("ROCM_PATH") != "" {
			detectionLog = append(detectionLog, "   ✅ ROCm environment variables detected!")
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}

	case "darwin":
		// ROCm not supported on macOS
		detectionLog = append(detectionLog, "🔍 ROCm Detection: Not supported on macOS")
		return false
	}

	detectionLog = append(detectionLog, "   ❌ ROCm not detected")
	fmt.Println(strings.Join(detectionLog, "\n"))
	return false
}

func detectVulkan() bool {
	detectionLog := []string{}

	switch runtime.GOOS {
	case "windows":
		detectionLog = append(detectionLog, formatLogPrefix("🔍", "[DETECT]")+" Vulkan Detection: Checking Windows paths...")

		// Check for vulkan-1.dll in system32
		if _, err := os.Stat("C:\\Windows\\System32\\vulkan-1.dll"); err == nil {
			detectionLog = append(detectionLog, "   "+formatLogPrefix("✓", "[OK]")+" Found vulkan-1.dll in System32")

			// Try to verify Vulkan devices exist (ONLY if vulkaninfo is available)
			if commandExists("vulkaninfo") {
				cmd := exec.Command("vulkaninfo", "--summary")
				output, err := cmd.Output()
				if err == nil && strings.Contains(string(output), "deviceType") {
					detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" Vulkan devices verified via vulkaninfo!")
					fmt.Println(strings.Join(detectionLog, "\n"))
					return true
				}
				if err != nil {
					detectionLog = append(detectionLog, fmt.Sprintf("   "+formatLogPrefix("⚠", "[WARN]")+" vulkaninfo command failed: %v", err))
					// Don't assume Vulkan works if verification failed
					detectionLog = append(detectionLog, "   "+formatLogPrefix("❌", "[FAIL]")+" Cannot verify Vulkan GPU support")
					fmt.Println(strings.Join(detectionLog, "\n"))
					return false
				}
			} else {
				detectionLog = append(detectionLog, "   "+formatLogPrefix("⚠", "[WARN]")+" vulkaninfo not available, cannot verify GPU support")
				// Be conservative: library exists but can't verify actual GPU support
				detectionLog = append(detectionLog, "   "+formatLogPrefix("ℹ", "[INFO]")+" Vulkan library found but GPU support unverified - assuming available")
				fmt.Println(strings.Join(detectionLog, "\n"))
				return true
			}
		}

		// Fallback: Check for Vulkan SDK installation
		vulkanSDKPath := os.Getenv("VULKAN_SDK")
		if vulkanSDKPath != "" {
			detectionLog = append(detectionLog, fmt.Sprintf("   "+formatLogPrefix("✅", "[SUCCESS]")+" Vulkan SDK environment variable detected: %s", vulkanSDKPath))
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}

	case "linux":
		detectionLog = append(detectionLog, "🔍 Vulkan Detection: Checking Linux paths...")

		// Check for libvulkan.so on Linux
		vulkanPaths := []string{
			"/usr/lib/x86_64-linux-gnu/libvulkan.so.1",
			"/usr/lib/libvulkan.so.1",
			"/usr/lib64/libvulkan.so.1",
			"/usr/local/lib/libvulkan.so.1",
			"/lib/x86_64-linux-gnu/libvulkan.so.1",
		}

		for _, path := range vulkanPaths {
			if _, err := os.Stat(path); err == nil {
				detectionLog = append(detectionLog, fmt.Sprintf("   ✓ Found libvulkan at: %s", path))
				// Try to verify Vulkan devices exist
				cmd := exec.Command("vulkaninfo", "--summary")
				output, err := cmd.Output()
				if err == nil && strings.Contains(string(output), "deviceType") {
					detectionLog = append(detectionLog, "   ✅ Vulkan devices verified via vulkaninfo!")
					fmt.Println(strings.Join(detectionLog, "\n"))
					return true
				}
				if err != nil {
					detectionLog = append(detectionLog, fmt.Sprintf("   ⚠ vulkaninfo command failed: %v (but library exists)", err))
				}
				// Vulkan library exists even if vulkaninfo fails
				detectionLog = append(detectionLog, "   ✅ Vulkan library detected (assuming devices available)")
				fmt.Println(strings.Join(detectionLog, "\n"))
				return true
			}
		}

		// Fallback 1: Check for Vulkan using ldconfig
		detectionLog = append(detectionLog, "   🔄 Trying fallback: Checking ldconfig...")
		cmd := exec.Command("ldconfig", "-p")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "libvulkan.so") {
			detectionLog = append(detectionLog, "   ✅ Vulkan library found via ldconfig!")
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}
		if err != nil {
			detectionLog = append(detectionLog, fmt.Sprintf("   ⚠ ldconfig command failed: %v", err))
		}

		// Fallback 2: Check for Vulkan SDK installation
		vulkanSDKPath := os.Getenv("VULKAN_SDK")
		if vulkanSDKPath != "" {
			detectionLog = append(detectionLog, fmt.Sprintf("   ✅ Vulkan SDK environment variable detected: %s", vulkanSDKPath))
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}

	case "darwin":
		detectionLog = append(detectionLog, "🔍 Vulkan Detection: Checking macOS (MoltenVK)...")

		// Check for MoltenVK on macOS (Vulkan → Metal translation layer)
		moltenVKPaths := []string{
			"/usr/local/lib/libvulkan.1.dylib",
			"/opt/homebrew/lib/libvulkan.1.dylib",
			"/System/Library/Frameworks/Vulkan.framework",
			"/Library/Frameworks/vulkan.framework",
		}

		for _, path := range moltenVKPaths {
			if _, err := os.Stat(path); err == nil {
				detectionLog = append(detectionLog, fmt.Sprintf("   ✅ MoltenVK found at: %s", path))
				fmt.Println(strings.Join(detectionLog, "\n"))
				return true
			}
		}

		// Fallback 1: Check if MoltenVK is installed via Homebrew
		detectionLog = append(detectionLog, "   🔄 Trying fallback: Checking Homebrew...")
		cmd := exec.Command("brew", "list", "molten-vk")
		if err := cmd.Run(); err == nil {
			detectionLog = append(detectionLog, "   ✅ MoltenVK installed via Homebrew!")
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}

		// Fallback 2: Check for Vulkan SDK
		vulkanSDKPath := os.Getenv("VULKAN_SDK")
		if vulkanSDKPath != "" {
			detectionLog = append(detectionLog, fmt.Sprintf("   ✅ Vulkan SDK environment variable detected: %s", vulkanSDKPath))
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}
	}

	detectionLog = append(detectionLog, "   ❌ Vulkan not detected")
	fmt.Println(strings.Join(detectionLog, "\n"))
	return false
}

func detectMetal() bool {
	detectionLog := []string{}

	// Metal is only available on macOS
	if runtime.GOOS != "darwin" {
		return false
	}

	detectionLog = append(detectionLog, formatLogPrefix("🔍", "[DETECT]")+" Metal Detection: Checking macOS...")

	// For Apple Silicon, Metal is always available
	if runtime.GOARCH == "arm64" {
		detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" Apple Silicon detected - Metal always available!")
		fmt.Println(strings.Join(detectionLog, "\n"))
		return true
	}

	// Check if Metal framework exists
	metalFrameworkPaths := []string{
		"/System/Library/Frameworks/Metal.framework",
		"/System/Library/PrivateFrameworks/Metal.framework",
	}

	frameworkFound := false
	for _, path := range metalFrameworkPaths {
		if _, err := os.Stat(path); err == nil {
			detectionLog = append(detectionLog, fmt.Sprintf("   "+formatLogPrefix("✓", "[OK]")+" Found Metal framework at: %s", path))
			frameworkFound = true

			// Metal framework exists, try to verify GPU support with timeout
			if commandExists("system_profiler") {
				detectionLog = append(detectionLog, "   "+formatLogPrefix("🔄", "[INFO]")+" Verifying GPU support (this may take a few seconds)...")
				output, err := runCommandWithTimeout("system_profiler", 15*time.Second, "SPDisplaysDataType")
				if err != nil {
					detectionLog = append(detectionLog, fmt.Sprintf("   "+formatLogPrefix("⚠", "[WARN]")+" system_profiler failed: %v", err))
				} else {
					outputStr := strings.ToLower(string(output))
					// Check for Apple Silicon or modern Intel GPUs with Metal support
					if strings.Contains(outputStr, "apple") ||
						strings.Contains(outputStr, "metal") ||
						strings.Contains(outputStr, "intel iris") ||
						strings.Contains(outputStr, "amd radeon") {
						detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" Metal-compatible GPU verified via system_profiler!")
						fmt.Println(strings.Join(detectionLog, "\n"))
						return true
					}
					detectionLog = append(detectionLog, "   "+formatLogPrefix("⚠", "[WARN]")+" system_profiler didn't show Metal support explicitly")
				}
			}

			// Check macOS version for Intel Macs
			if commandExists("sw_vers") {
				output, err := runCommandWithTimeout("sw_vers", 5*time.Second, "-productVersion")
				if err == nil {
					version := strings.TrimSpace(string(output))
					detectionLog = append(detectionLog, fmt.Sprintf("   "+formatLogPrefix("ℹ", "[INFO]")+" macOS version: %s", version))

					// Metal requires macOS 10.11 (El Capitan) or later
					// Parse version and check
					versionParts := strings.Split(version, ".")
					if len(versionParts) >= 2 {
						majorMinor := versionParts[0] + "." + versionParts[1]
						// Simple version check: 10.11+ or 11.0+
						if strings.HasPrefix(version, "10.") {
							// Check if 10.11 or later
							if majorMinor >= "10.11" {
								detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" macOS version supports Metal (10.11+)")
								fmt.Println(strings.Join(detectionLog, "\n"))
								return true
							} else {
								detectionLog = append(detectionLog, "   "+formatLogPrefix("❌", "[FAIL]")+" macOS too old for Metal (requires 10.11+)")
								fmt.Println(strings.Join(detectionLog, "\n"))
								return false
							}
						} else {
							// macOS 11+ always has Metal
							detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" Modern macOS with Metal support")
							fmt.Println(strings.Join(detectionLog, "\n"))
							return true
						}
					}
				}
			}

			// Framework exists, conservatively assume Metal support for modern macOS
			detectionLog = append(detectionLog, "   "+formatLogPrefix("✅", "[SUCCESS]")+" Metal framework detected (assuming GPU support)")
			fmt.Println(strings.Join(detectionLog, "\n"))
			return true
		}
	}

	if !frameworkFound {
		detectionLog = append(detectionLog, "   "+formatLogPrefix("⚠", "[WARN]")+" Metal framework not found")
	}

	detectionLog = append(detectionLog, "   "+formatLogPrefix("❌", "[FAIL]")+" Metal not detected")
	fmt.Println(strings.Join(detectionLog, "\n"))
	return false
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
	// MLX is only for Apple Silicon Macs
	if runtime.GOARCH != "arm64" {
		return
	}

	// Check for Metal Performance Shaders framework
	metalFrameworks := []string{
		"/System/Library/Frameworks/Metal.framework",
		"/System/Library/Frameworks/MetalPerformanceShaders.framework",
	}

	hasMetalFramework := false
	for _, framework := range metalFrameworks {
		if _, err := os.Stat(framework); err == nil {
			hasMetalFramework = true
			break
		}
	}

	if !hasMetalFramework {
		return
	}

	// Get detailed system info for Apple Silicon
	cmd := exec.Command("system_profiler", "SPHardwareDataType", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		// Fallback for Apple Silicon
		info.HasMLX = true
		info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
			Name:     "Apple GPU",
			VRAMGB:   getAppleSiliconUnifiedMemory(),
			Type:     "MLX",
			DeviceID: 0,
		})
		return
	}

	outputStr := strings.ToLower(string(output))

	// Detect Apple Silicon chip type for memory estimates
	var gpuName string
	var unifiedMemoryGB float64

	// Look for chip name patterns in the output - be more flexible with matching
	if strings.Contains(outputStr, "m1") || strings.Contains(outputStr, "apple m1") {
		if strings.Contains(outputStr, "max") {
			gpuName = "Apple M1 Max"
			unifiedMemoryGB = 32.0 // M1 Max typical config
		} else if strings.Contains(outputStr, "pro") {
			gpuName = "Apple M1 Pro"
			unifiedMemoryGB = 16.0 // M1 Pro typical config
		} else {
			gpuName = "Apple M1"
			unifiedMemoryGB = 8.0 // Base M1 typical config
		}
	} else if strings.Contains(outputStr, "m2") || strings.Contains(outputStr, "apple m2") {
		if strings.Contains(outputStr, "ultra") {
			gpuName = "Apple M2 Ultra"
			unifiedMemoryGB = 96.0 // M2 Ultra high-end config
		} else if strings.Contains(outputStr, "max") {
			gpuName = "Apple M2 Max"
			unifiedMemoryGB = 38.0 // M2 Max typical config
		} else if strings.Contains(outputStr, "pro") {
			gpuName = "Apple M2 Pro"
			unifiedMemoryGB = 16.0 // M2 Pro typical config
		} else {
			gpuName = "Apple M2"
			unifiedMemoryGB = 8.0 // Base M2 typical config
		}
	} else if strings.Contains(outputStr, "m3") || strings.Contains(outputStr, "apple m3") {
		if strings.Contains(outputStr, "max") {
			gpuName = "Apple M3 Max"
			unifiedMemoryGB = 48.0 // M3 Max typical config
		} else if strings.Contains(outputStr, "pro") {
			gpuName = "Apple M3 Pro"
			unifiedMemoryGB = 18.0 // M3 Pro typical config
		} else {
			gpuName = "Apple M3"
			unifiedMemoryGB = 8.0 // Base M3 typical config
		}
	} else if strings.Contains(outputStr, "m4") || strings.Contains(outputStr, "apple m4") {
		if strings.Contains(outputStr, "max") {
			gpuName = "Apple M4 Max"
			unifiedMemoryGB = 64.0 // M4 Max estimated config
		} else if strings.Contains(outputStr, "pro") {
			gpuName = "Apple M4 Pro"
			unifiedMemoryGB = 24.0 // M4 Pro estimated config
		} else {
			gpuName = "Apple M4"
			unifiedMemoryGB = 10.0 // Base M4 estimated config
		}
	} else if runtime.GOARCH == "arm64" {
		// Running on Apple Silicon but couldn't detect specific chip
		gpuName = "Apple Silicon GPU"
		unifiedMemoryGB = getAppleSiliconUnifiedMemory()
	} else {
		// Intel Mac or unknown - use conservative estimate
		gpuName = "Apple GPU"
		unifiedMemoryGB = getAppleSiliconUnifiedMemory()
	}

	// Check for actual total memory and adjust
	if info.TotalRAMGB > 0 {
		// Use 70% of total RAM as available for GPU tasks (conservative)
		adjustedMemory := info.TotalRAMGB * 0.7
		if adjustedMemory < unifiedMemoryGB {
			unifiedMemoryGB = adjustedMemory
		}
	}

	info.HasMLX = true
	info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
		Name:     gpuName,
		VRAMGB:   unifiedMemoryGB,
		Type:     "MLX",
		DeviceID: 0,
	})
}

// getAppleSiliconUnifiedMemory estimates unified memory for Apple Silicon
func getAppleSiliconUnifiedMemory() float64 {
	// Get total system memory and estimate GPU portion
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return 8.0 // Conservative fallback
	}

	memStr := strings.TrimSpace(string(output))
	if memBytes, err := strconv.ParseInt(memStr, 10, 64); err == nil {
		totalGB := float64(memBytes) / (1024 * 1024 * 1024)
		// Use 70% of total memory as available for GPU tasks
		return totalGB * 0.7
	}
	return 8.0 // Fallback
}

// PrintPlatformSupportSummary prints a comprehensive summary of platform support
func PrintPlatformSupportSummary() {
	fmt.Printf("\n🌍 ClaraCore Platform Support Matrix:\n")
	fmt.Printf("══════════════════════════════════════════════════════════════\n")

	// Windows Support
	fmt.Printf("🪟 WINDOWS SUPPORT:\n")
	fmt.Printf("   ✅ NVIDIA CUDA    - Best performance (RTX 40/30/20 series, GTX)\n")
	fmt.Printf("   ✅ AMD ROCm       - AMD GPU acceleration (RX 7000/6000/5000 series)\n")
	fmt.Printf("   ✅ Vulkan        - Cross-platform GPU support\n")
	fmt.Printf("   ✅ Intel GPU     - Integrated graphics acceleration\n")
	fmt.Printf("   ✅ CPU           - Multithreaded fallback\n")
	fmt.Printf("   🎯 Priority: CUDA > ROCm > Vulkan > CPU\n\n")

	// Linux Support
	fmt.Printf("🐧 LINUX SUPPORT:\n")
	fmt.Printf("   ✅ NVIDIA CUDA    - Optimal for data centers & gaming rigs\n")
	fmt.Printf("   ✅ AMD ROCm       - Open-source AMD GPU acceleration\n")
	fmt.Printf("   ✅ Vulkan        - Modern GPU API support\n")
	fmt.Printf("   ✅ Intel GPU     - Integrated & discrete Intel graphics\n")
	fmt.Printf("   ✅ CPU           - Excellent Linux optimization\n")
	fmt.Printf("   🎯 Priority: CUDA > ROCm > Vulkan > CPU\n\n")

	// macOS Support
	fmt.Printf("🍎 macOS SUPPORT:\n")
	fmt.Printf("   ✅ Apple MLX      - Apple Silicon unified memory (M1/M2/M3/M4)\n")
	fmt.Printf("   ✅ Metal         - Apple GPU acceleration framework\n")
	fmt.Printf("   ✅ Vulkan (MoltenVK) - Cross-platform compatibility layer\n")
	fmt.Printf("   ✅ Intel GPU     - Intel Mac integrated graphics\n")
	fmt.Printf("   ✅ CPU           - macOS-optimized processing\n")
	fmt.Printf("   🎯 Priority: Metal+MLX > Vulkan > CPU\n\n")

	// Hardware Recommendations
	fmt.Printf("🔧 HARDWARE RECOMMENDATIONS:\n")
	fmt.Printf("   🥇 Best:    NVIDIA RTX 4090 (24GB) / RTX 4080 (16GB) - Windows/Linux\n")
	fmt.Printf("   🥇 Best:    Apple M3 Max (128GB) / M2 Ultra (192GB) - macOS\n")
	fmt.Printf("   🥈 Great:   AMD RX 7900XTX (24GB) / RTX 3080 Ti (12GB)\n")
	fmt.Printf("   🥉 Good:    RTX 3060 (12GB) / Intel Arc A770 (16GB)\n")
	fmt.Printf("   💻 Budget:  CPU-only with 32GB+ RAM\n\n")

	// Model Size Recommendations
	fmt.Printf("📊 MODEL SIZE vs HARDWARE:\n")
	fmt.Printf("   🤖 70B+ models:  24GB+ VRAM or Apple Silicon with 64GB+ unified memory\n")
	fmt.Printf("   🤖 30B models:   16GB+ VRAM or 32GB+ unified memory\n")
	fmt.Printf("   🤖 13B models:   8GB+ VRAM or 16GB+ unified memory\n")
	fmt.Printf("   🤖 7B models:    6GB+ VRAM or 8GB+ unified memory\n")
	fmt.Printf("   🤖 3B models:    4GB+ VRAM or 4GB+ unified memory\n\n")

	// Installation Notes
	fmt.Printf("📝 INSTALLATION NOTES:\n")
	fmt.Printf("   Windows: Automatic driver detection and binary selection\n")
	fmt.Printf("   Linux:   Install CUDA/ROCm drivers manually for best performance\n")
	fmt.Printf("   macOS:   Metal/MLX work out-of-the-box on Apple Silicon\n")
	fmt.Printf("══════════════════════════════════════════════════════════════\n")
}

// enhanceIntelGPUDetection detects Intel integrated GPUs
func enhanceIntelGPUDetection(info *SystemInfo) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name,AdapterRAM")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		outputStr := strings.ToLower(string(output))
		if strings.Contains(outputStr, "intel") {
			// Parse Intel GPU details
			lines := strings.Split(string(output), "\n")
			var gpuName string
			var sharedMemoryGB float64 = 4.0 // Default estimate

			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), "intel") && !strings.Contains(line, "AdapterRAM") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						// Extract GPU name and memory estimate
						if strings.Contains(strings.ToLower(line), "iris xe") {
							gpuName = "Intel Iris Xe"
							sharedMemoryGB = 8.0 // Modern integrated GPU
						} else if strings.Contains(strings.ToLower(line), "iris") {
							gpuName = "Intel Iris"
							sharedMemoryGB = 6.0
						} else {
							gpuName = "Intel HD Graphics"
							sharedMemoryGB = 4.0
						}
						break
					}
				}
			}

			info.HasIntel = true
			info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
				Name:     gpuName,
				VRAMGB:   sharedMemoryGB,
				Type:     "Intel",
				DeviceID: 0,
			})
		}

	case "linux":
		// Check for Intel GPU on Linux
		cmd := exec.Command("lspci", "-v")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		outputStr := strings.ToLower(string(output))
		if strings.Contains(outputStr, "intel") && strings.Contains(outputStr, "graphics") {
			var gpuName string
			var sharedMemoryGB float64 = 4.0

			// Parse for specific Intel GPU types
			if strings.Contains(outputStr, "iris xe") {
				gpuName = "Intel Iris Xe"
				sharedMemoryGB = 8.0
			} else if strings.Contains(outputStr, "iris") {
				gpuName = "Intel Iris"
				sharedMemoryGB = 6.0
			} else if strings.Contains(outputStr, "uhd") {
				gpuName = "Intel UHD Graphics"
				sharedMemoryGB = 5.0
			} else {
				gpuName = "Intel HD Graphics"
				sharedMemoryGB = 4.0
			}

			info.HasIntel = true
			info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
				Name:     gpuName,
				VRAMGB:   sharedMemoryGB,
				Type:     "Intel",
				DeviceID: 0,
			})
		}

	case "darwin":
		// Check for Intel GPU on Intel-based Macs
		if runtime.GOARCH == "amd64" { // Intel Macs
			cmd := exec.Command("system_profiler", "SPDisplaysDataType")
			output, err := cmd.Output()
			if err != nil {
				return
			}

			outputStr := strings.ToLower(string(output))
			if strings.Contains(outputStr, "intel") {
				var gpuName string
				var sharedMemoryGB float64 = 1.5 // macOS Intel integrated

				if strings.Contains(outputStr, "iris") {
					gpuName = "Intel Iris Pro"
					sharedMemoryGB = 2.0
				} else {
					gpuName = "Intel HD Graphics"
					sharedMemoryGB = 1.5
				}

				info.HasIntel = true
				info.VRAMDetails = append(info.VRAMDetails, GPUInfo{
					Name:     gpuName,
					VRAMGB:   sharedMemoryGB,
					Type:     "Intel",
					DeviceID: 0,
				})
			}
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

// PrintSystemInfo prints comprehensive system information with platform-specific details
func PrintSystemInfo(info *SystemInfo) {
	fmt.Printf("🖥️  System Information:\n")
	fmt.Printf("   OS: %s/%s\n", info.OS, info.Architecture)
	fmt.Printf("   CPU Cores: %d logical, %d physical\n", info.CPUCores, info.PhysicalCores)
	fmt.Printf("   Total RAM: %.1f GB\n", info.TotalRAMGB)

	// Platform-specific acceleration support
	fmt.Printf("🚀 Platform Acceleration Support:\n")

	switch info.OS {
	case "windows":
		fmt.Printf("   🪟 Windows Platform:\n")
		if info.HasCUDA {
			fmt.Printf("      ✅ NVIDIA CUDA")
			if info.CUDAVersion != "" {
				fmt.Printf(" (v%s)", info.CUDAVersion)
			}
			fmt.Printf(" - Best performance for NVIDIA GPUs\n")
		}
		if info.HasROCm {
			fmt.Printf("      ✅ AMD ROCm")
			if info.ROCmVersion != "" {
				fmt.Printf(" (v%s)", info.ROCmVersion)
			}
			fmt.Printf(" - AMD GPU acceleration\n")
		}
		if info.HasVulkan {
			fmt.Printf("      ✅ Vulkan - Cross-platform GPU acceleration\n")
		}
		if info.HasIntel {
			fmt.Printf("      ✅ Intel GPU - Integrated graphics acceleration\n")
		}
		if !info.HasCUDA && !info.HasROCm && !info.HasVulkan && !info.HasIntel {
			fmt.Printf("      💻 CPU-only - Software acceleration\n")
		}

	case "linux":
		fmt.Printf("   🐧 Linux Platform:\n")
		if info.HasCUDA {
			fmt.Printf("      ✅ NVIDIA CUDA")
			if info.CUDAVersion != "" {
				fmt.Printf(" (v%s)", info.CUDAVersion)
			}
			fmt.Printf(" - Optimal for NVIDIA GPUs\n")
		}
		if info.HasROCm {
			fmt.Printf("      ✅ AMD ROCm")
			if info.ROCmVersion != "" {
				fmt.Printf(" (v%s)", info.ROCmVersion)
			}
			fmt.Printf(" - AMD GPU acceleration\n")
		}
		if info.HasVulkan {
			fmt.Printf("      ✅ Vulkan - Universal GPU acceleration\n")
		}
		if info.HasIntel {
			fmt.Printf("      ✅ Intel GPU - Integrated graphics support\n")
		}
		if !info.HasCUDA && !info.HasROCm && !info.HasVulkan && !info.HasIntel {
			fmt.Printf("      💻 CPU-only - Multithreaded processing\n")
		}

	case "darwin":
		fmt.Printf("   🍎 macOS Platform:\n")
		if info.HasMLX && runtime.GOARCH == "arm64" {
			fmt.Printf("      ✅ Apple MLX - Optimized for Apple Silicon\n")
		}
		if info.HasMetal {
			fmt.Printf("      ✅ Metal - Apple GPU acceleration\n")
		}
		if info.HasVulkan {
			fmt.Printf("      ✅ Vulkan (MoltenVK) - Cross-platform compatibility\n")
		}
		if info.HasIntel && runtime.GOARCH == "amd64" {
			fmt.Printf("      ✅ Intel GPU - Intel Mac graphics\n")
		}
		if runtime.GOARCH == "amd64" && !info.HasMetal && !info.HasVulkan && !info.HasIntel {
			fmt.Printf("      💻 CPU-only - Intel Mac software processing\n")
		}
	}

	// GPU Memory Details
	if len(info.VRAMDetails) > 0 {
		fmt.Printf("\n💾 GPU Memory Information:\n")
		fmt.Printf("   Total Available: %.1f GB\n", info.TotalVRAMGB)
		for i, gpu := range info.VRAMDetails {
			emoji := getGPUEmoji(gpu.Type)
			fmt.Printf("   %s GPU %d: %s (%.1f GB)\n", emoji, i, gpu.Name, gpu.VRAMGB)

			// Add platform-specific notes
			switch gpu.Type {
			case "CUDA":
				fmt.Printf("      🎯 Optimal for large language models\n")
			case "ROCm":
				fmt.Printf("      🔥 AMD GPU acceleration with ROCm\n")
			case "MLX":
				fmt.Printf("      🧠 Unified memory for Apple Silicon efficiency\n")
			case "Intel":
				fmt.Printf("      ⚡ Shared system memory for GPU tasks\n")
			}
		}
	} else {
		fmt.Printf("\n💻 No dedicated GPU memory - Using system RAM\n")
	}

	// Platform recommendations
	fmt.Printf("\n💡 Platform Recommendations:\n")
	switch info.OS {
	case "windows":
		if info.HasCUDA {
			fmt.Printf("   🥇 CUDA backend recommended for best performance\n")
		} else if info.HasROCm {
			fmt.Printf("   🥈 ROCm backend recommended for AMD GPUs\n")
		} else if info.HasVulkan {
			fmt.Printf("   🥉 Vulkan backend for cross-platform compatibility\n")
		} else {
			fmt.Printf("   💻 CPU backend - Consider GPU upgrade for better performance\n")
		}
	case "linux":
		if info.HasCUDA {
			fmt.Printf("   🥇 CUDA backend optimal for NVIDIA hardware\n")
		} else if info.HasROCm {
			fmt.Printf("   🥈 ROCm backend excellent for AMD GPUs\n")
		} else if info.HasVulkan {
			fmt.Printf("   🥉 Vulkan backend for modern GPU support\n")
		} else {
			fmt.Printf("   💻 CPU backend with excellent Linux optimization\n")
		}
	case "darwin":
		if info.HasMLX && runtime.GOARCH == "arm64" {
			fmt.Printf("   🥇 Metal backend optimal for Apple Silicon\n")
			fmt.Printf("   ⚡ MLX framework provides best efficiency\n")
		} else if info.HasMetal {
			fmt.Printf("   🥈 Metal backend for GPU acceleration\n")
		} else if info.HasVulkan {
			fmt.Printf("   🥉 Vulkan (MoltenVK) for compatibility\n")
		} else {
			fmt.Printf("   💻 CPU backend with macOS optimizations\n")
		}
	}
}

// getGPUEmoji returns appropriate emoji for GPU type
func getGPUEmoji(gpuType string) string {
	switch gpuType {
	case "CUDA":
		return "🟢" // NVIDIA green
	case "ROCm":
		return "🔴" // AMD red
	case "MLX":
		return "🍎" // Apple
	case "Intel":
		return "🔵" // Intel blue
	default:
		return "⚪" // Generic
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

		// Apply embedding detection logic (pass filename too for better detection)
		isEmbedding := detectEmbeddingFromMetadata(metadata, architecture, model.Name)
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
func detectEmbeddingFromMetadata(metadata map[string]interface{}, architecture string, filename string) bool {
	// Get model name from metadata AND filename for checks
	metadataName := getStringValue(metadata, "general.name")
	archLower := strings.ToLower(architecture)

	// Check BOTH metadata name AND filename
	lowerMetadataName := strings.ToLower(metadataName)
	lowerFilename := strings.ToLower(filename)

	// PRIORITY 1: Name-based check (HIGHEST PRIORITY - trust explicit naming)
	// Check BOTH metadata name AND filename - if either explicitly says "embed" or "embedding", trust it!
	if strings.Contains(lowerMetadataName, "embed") ||
		strings.Contains(lowerMetadataName, "embedding") ||
		strings.Contains(lowerFilename, "embed") ||
		strings.Contains(lowerFilename, "embedding") ||
		strings.HasPrefix(lowerMetadataName, "e5-") ||
		strings.HasPrefix(lowerFilename, "e5-") ||
		strings.HasPrefix(lowerMetadataName, "bge-") ||
		strings.HasPrefix(lowerFilename, "bge-") ||
		strings.HasPrefix(lowerMetadataName, "gte-") ||
		strings.HasPrefix(lowerFilename, "gte-") ||
		strings.Contains(lowerMetadataName, "minilm") ||
		strings.Contains(lowerFilename, "minilm") ||
		strings.Contains(lowerMetadataName, "mxbai") ||
		strings.Contains(lowerFilename, "mxbai") {
		return true
	}

	// PRIORITY 2: Check pooling_type metadata (VERY RELIABLE for models without explicit names)
	// Embedding models have pooling_type set to: mean, cls, last, rank
	// Language models have NO pooling_type or pooling_type = "none"
	poolingType := getStringValue(metadata, fmt.Sprintf("%s.pooling_type", architecture))
	if poolingType != "" && poolingType != "none" {
		// If pooling_type exists and is not "none", it's definitely an embedding model
		return true
	}

	// PRIORITY 3: Architecture check - BERT-based models are embeddings
	switch archLower {
	case "bert", "roberta", "nomic-bert", "jina-bert":
		return true
	case "llama", "mistral", "gemma", "gemma3", "glm4moe", "seed_oss", "gpt-oss":
		// These are typically generative models
		return false
	}

	// PRIORITY 4: Exclude Vision-Language models (only if name didn't indicate embedding)
	// This comes AFTER name check so models explicitly named "embedding" still pass through
	if archLower == "qwen2vl" || archLower == "llava" || strings.Contains(archLower, "vision") {
		return false
	}

	// PRIORITY 4: Tokenizer model check - BERT tokenizers indicate embeddings
	tokenizerModel := getStringValue(metadata, "tokenizer.ggml.model")
	if strings.Contains(strings.ToLower(tokenizerModel), "bert") {
		return true
	}

	// PRIORITY 5: Missing chat model keys + small embedding dimensions
	embeddingLength := getIntValue(metadata, fmt.Sprintf("%s.embedding_length", architecture))
	hasRope := hasKey(metadata, fmt.Sprintf("%s.rope", architecture))
	hasHeadCount := hasKey(metadata, fmt.Sprintf("%s.head_count", architecture))

	// Chat models typically have RoPE and head_count, embedding models often don't
	if !hasRope && !hasHeadCount && embeddingLength > 0 && embeddingLength <= 1024 {
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

package autosetup

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
}

// BinaryInfo contains information about the downloaded binary
type BinaryInfo struct {
	Path    string
	Version string
	Type    string // "cpu", "cuda", "rocm", "vulkan", "metal"
}

const (
	LLAMA_CPP_RELEASE_URL   = "https://github.com/ggml-org/llama.cpp/releases/tag/b6527"
	LLAMA_CPP_DOWNLOAD_BASE = "https://github.com/ggml-org/llama.cpp/releases/download/b6527"
)

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

		// Check if we have all required CUDA files for CUDA systems
		if system.HasCUDA && system.OS == "windows" {
			cudartPath := filepath.Join(extractDir, "cudart64_12.dll")
			if _, err := os.Stat(cudartPath); err == nil {
				fmt.Printf("✅ CUDA runtime already present, skipping download\n")
				return &BinaryInfo{
					Path:    existingServerPath,
					Version: "b6527",
					Type:    binaryType,
				}, nil
			} else {
				fmt.Printf("⚠️  CUDA runtime missing, will download both runtime and binary\n")
			}
		} else {
			// Non-CUDA system, existing binary is sufficient
			fmt.Printf("✅ Using existing binary, skipping download\n")
			return &BinaryInfo{
				Path:    existingServerPath,
				Version: "b6527",
				Type:    binaryType,
			}, nil
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

	return &BinaryInfo{
		Path:    serverPath,
		Version: "b6527",
		Type:    binaryType,
	}, nil
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

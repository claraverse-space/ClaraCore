package autosetup

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// RealtimeHardwareInfo contains current available hardware resources
type RealtimeHardwareInfo struct {
	AvailableVRAMGB float64 // Currently available VRAM (not total)
	AvailableRAMGB  float64 // Currently available system RAM
	TotalVRAMGB     float64 // Total VRAM for reference
	TotalRAMGB      float64 // Total system RAM for reference
}

// GetRealtimeHardwareInfo detects current available VRAM and RAM
func GetRealtimeHardwareInfo() (*RealtimeHardwareInfo, error) {
	info := &RealtimeHardwareInfo{}

	// Get current available VRAM
	availableVRAM, totalVRAM, err := getCurrentVRAM()
	if err != nil {
		// Fallback to total VRAM detection
		me := &MemoryEstimator{}
		totalVRAM, _ = me.GetAvailableVRAM()
		availableVRAM = totalVRAM * 0.9 // Assume 90% available
	}
	info.AvailableVRAMGB = availableVRAM
	info.TotalVRAMGB = totalVRAM

	// Get current available RAM
	availableRAM, totalRAM, err := getCurrentRAM()
	if err != nil {
		// Fallback to total RAM detection
		totalRAM = detectTotalRAM()
		availableRAM = totalRAM * 0.75 // Assume 75% available
	}
	info.AvailableRAMGB = availableRAM
	info.TotalRAMGB = totalRAM

	return info, nil
}

// getCurrentVRAM gets current available and total VRAM using nvidia-smi
func getCurrentVRAM() (available, total float64, err error) {
	// Try nvidia-smi first
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.free,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("nvidia-smi not available: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return 0, 0, fmt.Errorf("no GPU memory information found")
	}

	// Parse first GPU (primary GPU)
	parts := strings.Split(strings.TrimSpace(lines[0]), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected nvidia-smi output format")
	}

	// Parse available memory (free)
	availableMB, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse available VRAM: %v", err)
	}

	// Parse total memory
	totalMB, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse total VRAM: %v", err)
	}

	// Convert MB to GB
	available = availableMB / 1024.0
	total = totalMB / 1024.0

	return available, total, nil
}

// getCurrentRAM gets current available and total system RAM
func getCurrentRAM() (available, total float64, err error) {
	switch runtime.GOOS {
	case "windows":
		return getCurrentRAMWindows()
	case "linux":
		return getCurrentRAMLinux()
	case "darwin":
		return getCurrentRAMMacOS()
	default:
		return 0, 0, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// getCurrentRAMWindows gets current RAM info on Windows using modern PowerShell commands
func getCurrentRAMWindows() (available, total float64, err error) {
	// Get total physical memory capacity using PowerShell
	cmd := exec.Command("powershell", "-Command",
		"Get-CimInstance -ClassName Win32_PhysicalMemory | Measure-Object -Property Capacity -Sum | Select-Object -ExpandProperty Sum")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total RAM: %v", err)
	}

	totalBytes, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse total RAM: %v", err)
	}

	// Get available memory using PowerShell
	cmd = exec.Command("powershell", "-Command",
		"Get-CimInstance -ClassName Win32_OperatingSystem | Select-Object -ExpandProperty FreePhysicalMemory")
	output, err = cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get available RAM: %v", err)
	}

	availableKB, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse available RAM: %v", err)
	}

	// Convert to GB
	total = totalBytes / (1024 * 1024 * 1024)               // Bytes to GB
	available = (availableKB * 1024) / (1024 * 1024 * 1024) // KB to GB

	return available, total, nil
}

// getCurrentRAMLinux gets current RAM info on Linux
func getCurrentRAMLinux() (available, total float64, err error) {
	cmd := exec.Command("cat", "/proc/meminfo")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read /proc/meminfo: %v", err)
	}

	var totalKB, availableKB float64
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return 0, 0, fmt.Errorf("failed to parse total RAM: %v", err)
				}
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availableKB, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return 0, 0, fmt.Errorf("failed to parse available RAM: %v", err)
				}
			}
		}
	}

	if totalKB == 0 || availableKB == 0 {
		return 0, 0, fmt.Errorf("failed to parse memory information")
	}

	// Convert KB to GB
	available = availableKB / (1024 * 1024)
	total = totalKB / (1024 * 1024)

	return available, total, nil
}

// getCurrentRAMMacOS gets current RAM info on macOS
func getCurrentRAMMacOS() (available, total float64, err error) {
	// Get total memory
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total RAM: %v", err)
	}

	totalBytes, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse total RAM: %v", err)
	}

	// Get memory pressure info (approximation for available memory)
	cmd = exec.Command("vm_stat")
	output, err = cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get memory stats: %v", err)
	}

	// Parse vm_stat output for free + inactive pages
	var freePages, inactivePages float64
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Pages free:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				freeStr := strings.TrimSuffix(fields[2], ".")
				freePages, _ = strconv.ParseFloat(freeStr, 64)
			}
		} else if strings.Contains(line, "Pages inactive:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				inactiveStr := strings.TrimSuffix(fields[2], ".")
				inactivePages, _ = strconv.ParseFloat(inactiveStr, 64)
			}
		}
	}

	// macOS page size is typically 4KB
	pageSize := 4096.0
	availableBytes := (freePages + inactivePages) * pageSize

	// Convert to GB
	available = availableBytes / (1024 * 1024 * 1024)
	total = totalBytes / (1024 * 1024 * 1024)

	return available, total, nil
}

// PrintRealtimeInfo displays current hardware status
func PrintRealtimeInfo(info *RealtimeHardwareInfo) {
	fmt.Printf("🔄 Real-time Hardware Status:\n")
	fmt.Printf("   VRAM: %.2f GB available / %.2f GB total (%.1f%% free)\n",
		info.AvailableVRAMGB, info.TotalVRAMGB, (info.AvailableVRAMGB/info.TotalVRAMGB)*100)
	fmt.Printf("   RAM:  %.2f GB available / %.2f GB total (%.1f%% free)\n",
		info.AvailableRAMGB, info.TotalRAMGB, (info.AvailableRAMGB/info.TotalRAMGB)*100)
}

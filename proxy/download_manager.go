package proxy

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prave/ClaraCore/event"
)

// DownloadStatus represents the current state of a download
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusCancelled   DownloadStatus = "cancelled"
)

// DownloadInfo contains information about a download
type DownloadInfo struct {
	ID              string         `json:"id"`
	ModelID         string         `json:"modelId"`
	Filename        string         `json:"filename"`
	URL             string         `json:"url"`
	Status          DownloadStatus `json:"status"`
	Progress        float64        `json:"progress"` // 0-100
	DownloadedBytes int64          `json:"downloadedBytes"`
	TotalBytes      int64          `json:"totalBytes"`
	Speed           int64          `json:"speed"` // bytes per second
	ETA             int64          `json:"eta"`   // seconds remaining
	StartTime       time.Time      `json:"startTime"`
	FilePath        string         `json:"filePath"`
	Error           string         `json:"error,omitempty"`
	RetryCount      int            `json:"retryCount"`
	HFApiKey        string         `json:"-"` // Don't serialize API key
}

// DownloadManager handles concurrent downloads with resume capability
type DownloadManager struct {
	downloads     map[string]*DownloadInfo
	downloadsMux  sync.RWMutex
	activeWorkers map[string]context.CancelFunc
	workersMux    sync.RWMutex
	downloadDir   string
	logger        *LogMonitor
}

// DownloadProgressEvent is fired when download progress changes
type DownloadProgressEvent struct {
	DownloadID string
	Info       *DownloadInfo
}

func (e DownloadProgressEvent) Type() uint32 {
	return DownloadProgressEventID
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(downloadDir string, logger *LogMonitor) *DownloadManager {
	// Ensure download directory exists
	os.MkdirAll(downloadDir, 0755)

	dm := &DownloadManager{
		downloads:     make(map[string]*DownloadInfo),
		activeWorkers: make(map[string]context.CancelFunc),
		downloadDir:   downloadDir,
		logger:        logger,
	}

	// Start periodic cleanup of old completed downloads (keep for 30 minutes)
	go dm.startPeriodicCleanup()

	return dm
}

// startPeriodicCleanup runs cleanup every 5 minutes to remove old completed downloads
func (dm *DownloadManager) startPeriodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Keep completed downloads visible for 30 minutes
		dm.Cleanup(30 * time.Minute)
	}
}

// StartDownload initiates a new download
func (dm *DownloadManager) StartDownload(modelID, filename, url, hfApiKey, destinationPath string) (string, error) {
	// Validate inputs
	if filename == "" || filename == "undefined" {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	downloadID := fmt.Sprintf("%s-%s-%d", modelID, filename, time.Now().Unix())

	// Determine download directory
	downloadDir := dm.downloadDir
	if destinationPath != "" {
		// Use custom destination path if provided
		downloadDir = destinationPath
	}

	// Ensure download directory exists
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %v", err)
	}

	// Clean filename for filesystem
	cleanFilename := dm.sanitizeFilename(filename)
	filePath := filepath.Join(downloadDir, cleanFilename)

	downloadInfo := &DownloadInfo{
		ID:        downloadID,
		ModelID:   modelID,
		Filename:  filename,
		URL:       url,
		Status:    StatusPending,
		Progress:  0,
		StartTime: time.Now(),
		FilePath:  filePath,
		HFApiKey:  hfApiKey,
	}

	dm.downloadsMux.Lock()
	dm.downloads[downloadID] = downloadInfo
	dm.downloadsMux.Unlock()

	// Start download worker in separate goroutine
	ctx, cancel := context.WithCancel(context.Background())
	dm.workersMux.Lock()
	dm.activeWorkers[downloadID] = cancel
	dm.workersMux.Unlock()

	go dm.downloadWorker(ctx, downloadInfo)

	dm.logger.Infof("Started download: %s -> %s", url, filePath)
	return downloadID, nil
}

// StartMultiPartDownload initiates multiple downloads for a multi-part model
func (dm *DownloadManager) StartMultiPartDownload(modelID, quantization string, filePaths []string, hfApiKey, destinationPath string) ([]string, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no files provided for multi-part download")
	}

	// Determine download directory
	downloadDir := dm.downloadDir
	if destinationPath != "" {
		downloadDir = destinationPath
	}

	// For multi-part downloads, create a subdirectory for the quantization
	quantDir := filepath.Join(downloadDir, quantization)
	if err := os.MkdirAll(quantDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create quantization directory: %v", err)
	}

	downloadIDs := make([]string, 0, len(filePaths))

	// Start download for each part
	for _, filePath := range filePaths {
		// Extract filename from path (e.g., "Q4_K_M/file-00001-of-00003.gguf" -> "file-00001-of-00003.gguf")
		parts := strings.Split(filePath, "/")
		filename := parts[len(parts)-1]

		// Construct HuggingFace URL
		url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, filePath)

		// Use the quantization directory as destination
		downloadID, err := dm.StartDownload(modelID, filename, url, hfApiKey, quantDir)
		if err != nil {
			dm.logger.Errorf("Failed to start download for %s: %v", filename, err)
			// Continue with other files even if one fails
			continue
		}

		downloadIDs = append(downloadIDs, downloadID)
	}

	if len(downloadIDs) == 0 {
		return nil, fmt.Errorf("failed to start any downloads")
	}

	dm.logger.Infof("Started multi-part download: %d files for %s/%s", len(downloadIDs), modelID, quantization)
	return downloadIDs, nil
}

// downloadWorker handles the actual download process with robust retry mechanism
func (dm *DownloadManager) downloadWorker(ctx context.Context, info *DownloadInfo) {
	defer func() {
		dm.workersMux.Lock()
		delete(dm.activeWorkers, info.ID)
		dm.workersMux.Unlock()
	}()

	maxRetries := 50 // Allow many retries for large downloads
	baseDelay := time.Second * 2

	for retryCount := 0; retryCount <= maxRetries; retryCount++ {
		// Update retry count in download info
		dm.downloadsMux.Lock()
		info.RetryCount = retryCount
		dm.downloadsMux.Unlock()

		// Update status to downloading
		dm.updateStatus(info.ID, StatusDownloading)

		// Send initial progress event
		event.Emit(DownloadProgressEvent{
			DownloadID: info.ID,
			Info:       info,
		})

		// Check if file already exists (resume support)
		existingSize := int64(0)
		if stat, err := os.Stat(info.FilePath); err == nil {
			existingSize = stat.Size()
			info.DownloadedBytes = existingSize
			if retryCount > 0 {
				dm.logger.Infof("Retry %d: Resuming download from byte %d", retryCount, existingSize)
			} else {
				dm.logger.Infof("Resuming download from byte %d", existingSize)
			}
		}

		// Attempt download
		success, shouldRetry := dm.attemptDownload(ctx, info, existingSize, retryCount)
		if success {
			// Success! Download completed
			return
		}

		if !shouldRetry {
			// Permanent failure, don't retry
			return
		}

		// Check if we should continue retrying
		select {
		case <-ctx.Done():
			dm.updateError(info.ID, "Download cancelled")
			return
		default:
		}

		// If we've exceeded retries, fail
		if retryCount >= maxRetries {
			dm.updateError(info.ID, fmt.Sprintf("Download failed after %d retries", maxRetries))
			return
		}

		// Wait before retry with exponential backoff
		delay := time.Duration(float64(baseDelay) * math.Pow(1.5, float64(retryCount)))
		if delay > time.Minute*5 {
			delay = time.Minute * 5 // Cap at 5 minutes
		}

		dm.logger.Warnf("Download failed, retrying in %v (attempt %d/%d)", delay, retryCount+1, maxRetries)

		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			dm.updateError(info.ID, "Download cancelled during retry wait")
			return
		}
	}
}

// attemptDownload performs a single download attempt
// Returns (success, shouldRetry)
func (dm *DownloadManager) attemptDownload(ctx context.Context, info *DownloadInfo, existingSize int64, retryCount int) (bool, bool) {

	// Create HTTP request with resume support
	req, err := http.NewRequestWithContext(ctx, "GET", info.URL, nil)
	if err != nil {
		if retryCount > 0 {
			dm.logger.Errorf("Retry %d failed to create request: %v", retryCount, err)
		}
		return false, false // Don't retry request creation errors
	}

	// Add authorization header if API key is provided
	if info.HFApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+info.HFApiKey)
	}

	// Add range header for resume
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Make the request with NO timeout - let it run as long as needed
	client := &http.Client{
		// Remove timeout completely - downloads can take hours for large models
	}
	resp, err := client.Do(req)
	if err != nil {
		if retryCount > 0 {
			dm.logger.Errorf("Retry %d failed to connect: %v", retryCount, err)
		}
		return false, true // Retry connection errors
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			dm.updateError(info.ID, "File not found on server")
			return false, false // Don't retry 404 errors
		}
		return false, true // Retry other HTTP errors
	}

	// Check if server supports range requests for resume
	if existingSize > 0 && resp.StatusCode != http.StatusPartialContent {
		dm.logger.Warnf("Server doesn't support resume, starting from beginning")
		existingSize = 0
		info.DownloadedBytes = 0
	}

	// Get total file size
	if resp.ContentLength > 0 {
		if existingSize > 0 {
			info.TotalBytes = existingSize + resp.ContentLength
		} else {
			info.TotalBytes = resp.ContentLength
		}
	}

	// Open file for writing (create or append)
	var file *os.File
	if existingSize > 0 {
		file, err = os.OpenFile(info.FilePath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(info.FilePath)
	}
	if err != nil {
		dm.updateError(info.ID, fmt.Sprintf("Failed to create file: %v", err))
		return false, false // Don't retry file creation errors
	}
	defer file.Close()

	// Download with progress tracking
	success := dm.downloadWithProgress(ctx, info, resp.Body, file)
	return success, true // Always allow retry if download fails
}

// downloadWithProgress handles the download with real-time progress updates
// Returns true if download completed successfully, false if failed
func (dm *DownloadManager) downloadWithProgress(ctx context.Context, info *DownloadInfo, reader io.Reader, writer io.Writer) bool {
	buffer := make([]byte, 64*1024) // 64KB buffer for optimal performance
	lastUpdate := time.Now()
	lastBytes := info.DownloadedBytes

	for {
		select {
		case <-ctx.Done():
			dm.updateStatus(info.ID, StatusCancelled)
			return false
		default:
			n, err := reader.Read(buffer)
			if n > 0 {
				// Write to file
				if _, writeErr := writer.Write(buffer[:n]); writeErr != nil {
					dm.logger.Errorf("Write error: %v", writeErr)
					return false
				}

				// Update progress
				info.DownloadedBytes += int64(n)

				// Calculate speed and ETA every second
				now := time.Now()
				if now.Sub(lastUpdate) >= time.Second {
					elapsed := now.Sub(lastUpdate).Seconds()
					if elapsed > 0 {
						bytesThisSecond := info.DownloadedBytes - lastBytes
						info.Speed = int64(float64(bytesThisSecond) / elapsed)
					}

					// Calculate progress and ETA safely
					if info.TotalBytes > 0 {
						info.Progress = float64(info.DownloadedBytes) / float64(info.TotalBytes) * 100
						if info.Speed > 0 {
							remaining := info.TotalBytes - info.DownloadedBytes
							if remaining > 0 {
								info.ETA = remaining / info.Speed
							} else {
								info.ETA = 0
							}
						}
					} else {
						// If we don't know total size, show as indeterminate
						info.Progress = -1
						info.ETA = 0
					}

					// Fire progress event
					event.Emit(DownloadProgressEvent{
						DownloadID: info.ID,
						Info:       info,
					})

					lastUpdate = now
					lastBytes = info.DownloadedBytes
				}
			}

			if err != nil {
				if err == io.EOF {
					// Download completed successfully
					info.Progress = 100
					dm.updateStatus(info.ID, StatusCompleted)
					dm.logger.Infof("Download completed: %s", info.FilePath)

					// Send final progress event
					event.Emit(DownloadProgressEvent{
						DownloadID: info.ID,
						Info:       info,
					})

					return true
				} else {
					dm.logger.Errorf("Read error during download: %v", err)
					return false
				}
			}
		}
	}
}

// PauseDownload pauses an active download
func (dm *DownloadManager) PauseDownload(downloadID string) error {
	dm.workersMux.Lock()
	if cancel, exists := dm.activeWorkers[downloadID]; exists {
		cancel()
		delete(dm.activeWorkers, downloadID)
		dm.updateStatus(downloadID, StatusPaused)
		dm.logger.Infof("Paused download: %s", downloadID)
	}
	dm.workersMux.Unlock()
	return nil
}

// ResumeDownload resumes a paused download
func (dm *DownloadManager) ResumeDownload(downloadID string) error {
	dm.downloadsMux.RLock()
	info, exists := dm.downloads[downloadID]
	dm.downloadsMux.RUnlock()

	if !exists {
		return fmt.Errorf("download not found: %s", downloadID)
	}

	if info.Status != StatusPaused {
		return fmt.Errorf("download is not paused: %s", downloadID)
	}

	// Start new worker for resumed download
	ctx, cancel := context.WithCancel(context.Background())
	dm.workersMux.Lock()
	dm.activeWorkers[downloadID] = cancel
	dm.workersMux.Unlock()

	go dm.downloadWorker(ctx, info)

	dm.logger.Infof("Resumed download: %s", downloadID)
	return nil
}

// CancelDownload cancels and removes a download
func (dm *DownloadManager) CancelDownload(downloadID string) error {
	// Cancel active worker
	dm.workersMux.Lock()
	if cancel, exists := dm.activeWorkers[downloadID]; exists {
		cancel()
		delete(dm.activeWorkers, downloadID)
	}
	dm.workersMux.Unlock()

	// Remove partial file
	dm.downloadsMux.RLock()
	info, exists := dm.downloads[downloadID]
	dm.downloadsMux.RUnlock()

	if exists && info.Status != StatusCompleted {
		os.Remove(info.FilePath)
		dm.logger.Infof("Removed partial file: %s", info.FilePath)
	}

	// Remove from downloads map
	dm.downloadsMux.Lock()
	delete(dm.downloads, downloadID)
	dm.downloadsMux.Unlock()

	dm.logger.Infof("Cancelled download: %s", downloadID)
	return nil
}

// GetDownloads returns all download information
func (dm *DownloadManager) GetDownloads() map[string]*DownloadInfo {
	dm.downloadsMux.RLock()
	defer dm.downloadsMux.RUnlock()

	result := make(map[string]*DownloadInfo)
	for k, v := range dm.downloads {
		// Create a copy to avoid race conditions
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetDownload returns information about a specific download
func (dm *DownloadManager) GetDownload(downloadID string) (*DownloadInfo, bool) {
	dm.downloadsMux.RLock()
	defer dm.downloadsMux.RUnlock()

	info, exists := dm.downloads[downloadID]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *info
	return &copy, true
}

// updateStatus updates the status of a download
func (dm *DownloadManager) updateStatus(downloadID string, status DownloadStatus) {
	dm.downloadsMux.Lock()
	if info, exists := dm.downloads[downloadID]; exists {
		info.Status = status
		if status == StatusCompleted {
			info.Progress = 100
		}
	}
	dm.downloadsMux.Unlock()
}

// updateError updates the error status of a download
func (dm *DownloadManager) updateError(downloadID string, errorMsg string) {
	dm.downloadsMux.Lock()
	if info, exists := dm.downloads[downloadID]; exists {
		info.Status = StatusFailed
		info.Error = errorMsg
	}
	dm.downloadsMux.Unlock()
	dm.logger.Errorf("Download error [%s]: %s", downloadID, errorMsg)
}

// sanitizeFilename removes invalid characters from filename
func (dm *DownloadManager) sanitizeFilename(filename string) string {
	// Replace invalid characters
	invalid := []string{"<", ">", ":", "\"", "|", "?", "*"}
	clean := filename
	for _, char := range invalid {
		clean = strings.ReplaceAll(clean, char, "_")
	}
	return clean
}

// Cleanup removes completed downloads older than specified duration
func (dm *DownloadManager) Cleanup(maxAge time.Duration) {
	dm.downloadsMux.Lock()
	defer dm.downloadsMux.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, info := range dm.downloads {
		if info.Status == StatusCompleted && info.StartTime.Before(cutoff) {
			delete(dm.downloads, id)
			dm.logger.Infof("Cleaned up old download record: %s", id)
		}
	}
}

package autosetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ProgressState struct {
	Status          string    `json:"status"`
	CurrentStep     string    `json:"current_step"`
	Progress        float64   `json:"progress"`
	TotalModels     int       `json:"total_models"`
	ProcessedModels int       `json:"processed_models"`
	CurrentModel    string    `json:"current_model"`
	Error           string    `json:"error"`
	Completed       bool      `json:"completed"`
	StartedAt       time.Time `json:"started_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProgressWrapper struct {
	SetupProgress ProgressState `json:"setup_progress"`
}

type ProgressManager struct {
	filePath string
	state    ProgressState
	mutex    sync.RWMutex
}

func NewProgressManager() *ProgressManager {
	// Get the executable directory or current working directory
	execDir, err := os.Executable()
	if err != nil {
		execDir, _ = os.Getwd()
	} else {
		execDir = filepath.Dir(execDir)
	}

	filePath := filepath.Join(execDir, "progress_state.json")

	pm := &ProgressManager{
		filePath: filePath,
		state: ProgressState{
			Status:    "idle",
			UpdatedAt: time.Now(),
		},
	}

	// Initialize file if it doesn't exist
	pm.saveToFile()

	return pm
}

func (pm *ProgressManager) UpdateStatus(status string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.state.Status = status
	pm.state.UpdatedAt = time.Now()

	if status == "scanning" || status == "generating" {
		pm.state.StartedAt = time.Now()
		pm.state.Completed = false
		pm.state.Error = ""
	} else if status == "completed" {
		pm.state.Completed = true
		pm.state.Progress = 100
	}

	pm.saveToFile()
}

func (pm *ProgressManager) UpdateStep(step string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.state.CurrentStep = step
	pm.state.UpdatedAt = time.Now()
	pm.saveToFile()
}

func (pm *ProgressManager) UpdateProgress(current, total int, currentModel string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.state.ProcessedModels = current
	pm.state.TotalModels = total
	pm.state.CurrentModel = currentModel
	pm.state.UpdatedAt = time.Now()

	if total > 0 {
		pm.state.Progress = float64(current) / float64(total) * 100
	}

	pm.saveToFile()
}

func (pm *ProgressManager) SetError(err string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.state.Status = "error"
	pm.state.Error = err
	pm.state.UpdatedAt = time.Now()
	pm.saveToFile()
}

func (pm *ProgressManager) Reset() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.state = ProgressState{
		Status:    "idle",
		UpdatedAt: time.Now(),
	}
	pm.saveToFile()
}

func (pm *ProgressManager) GetState() ProgressState {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return pm.state
}

func (pm *ProgressManager) saveToFile() {
	wrapper := ProgressWrapper{
		SetupProgress: pm.state,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return
	}

	// Write to temporary file first, then rename (atomic operation)
	tmpFile := pm.filePath + ".tmp"
	err = os.WriteFile(tmpFile, data, 0644)
	if err != nil {
		return
	}

	err = os.Rename(tmpFile, pm.filePath)
	if err != nil {
		os.Remove(tmpFile) // Clean up on failure
	}
}

// Global progress manager instance
var globalProgressManager *ProgressManager
var progressManagerOnce sync.Once

func GetProgressManager() *ProgressManager {
	progressManagerOnce.Do(func() {
		globalProgressManager = NewProgressManager()
	})
	return globalProgressManager
}

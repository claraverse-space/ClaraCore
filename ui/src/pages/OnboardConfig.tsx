import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  RefreshCwIcon, 
  SettingsIcon,
  CheckCircleIcon,
  AlertTriangleIcon,
  WandIcon,
  FileIcon,
  ZapIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  MonitorIcon,
  MemoryStickIcon,
  SearchIcon,
  InfoIcon,
  DownloadIcon,
  FolderIcon
} from 'lucide-react';
import { Card, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import type { SystemDetection } from '../types';

interface ModelScanResult {
  modelId: string;
  filename: string;
  name: string;
  size: number;
  sizeFormatted: string;
  path: string;
  relativePath: string;
  quantization: string;
  isInstruct: boolean;
  isDraft: boolean;
  isEmbedding: boolean;
  contextLength: number;
  numLayers: number;
  isMoE: boolean;
}

interface SystemConfig {
  hasGPU: boolean;
  gpuType: 'nvidia' | 'amd' | 'intel' | 'apple' | 'none';
  vramGB: number;
  ramGB: number;
  backend: 'cuda' | 'rocm' | 'vulkan' | 'metal' | 'mlx' | 'cpu';
  preferredContext: number;
  throughputFirst: boolean;
}

const OnboardConfig: React.FC = () => {
  const [currentStep, setCurrentStep] = useState(0);
  
  // Multi-folder support (backward compatible)
  const [folderPath, setFolderPath] = useState(''); // Single folder input (backward compatibility)
  const [folderPaths, setFolderPaths] = useState<string[]>([]); // Multiple tracked folders
  const [currentFolderInput, setCurrentFolderInput] = useState(''); // Current input field
  
  const [scanResults, setScanResults] = useState<ModelScanResult[]>([]);
  const [systemConfig, setSystemConfig] = useState<SystemConfig>({
    hasGPU: true,
    gpuType: 'nvidia',
    vramGB: 12,
    ramGB: 32,
    backend: 'cuda',
    preferredContext: 32768,
    throughputFirst: true,
  });
  const [systemDetection, setSystemDetection] = useState<SystemDetection | null>(null);
  const [isScanning, setIsScanning] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [isDetecting, setIsDetecting] = useState(false);
  const [autoDetected, setAutoDetected] = useState(false);
  const [notification, setNotification] = useState<{type: 'success' | 'error' | 'info', message: string} | null>(null);
  
  // Progress tracking for config generation
  const [generationProgress, setGenerationProgress] = useState({
    current: 0,
    total: 0,
    currentModel: '',
    stage: 'Initializing...',
    startTime: null as Date | null,
    estimatedTimeRemaining: null as number | null
  });
  const [hasExistingModels, setHasExistingModels] = useState(false);
  const [showSetup, setShowSetup] = useState(true);
  const [modelSource, setModelSource] = useState<'existing' | 'download' | null>(null);
  const [backendNotice, setBackendNotice] = useState<string | null>(null);

  const showNotification = (type: 'success' | 'error' | 'info', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 5000);
  };

  const checkExistingModels = async () => {
    try {
      const response = await fetch('/api/events');
      if (response.ok) {
        const text = await response.text();
        // Check if there are any models in the events stream
        const hasModels = text.includes('"type":"modelStatus"') && text.includes('"id":');
        setHasExistingModels(hasModels);
        if (hasModels) {
          setShowSetup(false); // Hide setup if models already exist
        }
      }
    } catch (error) {
      console.error('Failed to check existing models:', error);
    }
  };

  // Load existing folders from database
  const loadExistingFolders = async () => {
    try {
      const response = await fetch('/api/config/folders');
      if (response.ok) {
        const data = await response.json();
        if (data.folders && data.folders.length > 0) {
          const enabledFolders = data.folders
            .filter((f: any) => f.enabled)
            .map((f: any) => f.path);
          setFolderPaths(enabledFolders);
          showNotification('info', `Loaded ${enabledFolders.length} folders from database`);
        }
      }
    } catch (error) {
      console.error('Failed to load existing folders:', error);
    }
  };

  // Auto-detect system on component mount and check for existing models
  useEffect(() => {
    detectSystem();
    checkExistingModels();
    loadExistingFolders(); // Load existing folder database
  }, []);

  // If folders already exist or models are already present, jump to Step 3 (Folder management)
  useEffect(() => {
    if ((folderPaths && folderPaths.length > 0) || hasExistingModels) {
      // Jump to the folder management step (Step 3 of 6), not the results step
      setCurrentStep(2);
    }
  }, [folderPaths, hasExistingModels]);

  const detectSystem = async () => {
    setIsDetecting(true);
    try {
      const response = await fetch('/api/system/detection');
      if (response.ok) {
        const detection = await response.json();
        setSystemDetection(detection);
        
        // Auto-populate system config based on detection
        if (detection.primaryGPU) {
          let gpuType: 'nvidia' | 'amd' | 'intel' | 'apple' = 'intel';
          if (detection.primaryGPU.brand === 'nvidia') gpuType = 'nvidia';
          else if (detection.primaryGPU.brand === 'amd') gpuType = 'amd';
          else if (detection.primaryGPU.brand === 'apple') gpuType = 'apple';
          else if (detection.primaryGPU.brand === 'intel') gpuType = 'intel';

          setSystemConfig(prev => ({
            ...prev,
            hasGPU: detection.gpuDetected || false,
            gpuType: gpuType,
            vramGB: Math.floor(detection.primaryGPU.vramGB || 0),
            backend: detection.recommendations?.primaryBackend || 'cuda',
          }));
        } else {
          setSystemConfig(prev => ({
            ...prev,
            hasGPU: false,
            backend: 'cpu',
          }));
        }
        
        setSystemConfig(prev => ({
          ...prev,
          ramGB: Math.floor(detection.totalRAMGB || 0),
          preferredContext: detection.recommendations?.suggestedContextSize || 32768,
          throughputFirst: detection.recommendations?.throughputFirst || true,
        }));
        
        setAutoDetected(true);
        showNotification('success', `Hardware detected: ${detection.primaryGPU ? detection.primaryGPU.name : 'CPU-only configuration'}`);
      } else {
        showNotification('error', 'Failed to detect system. Please fill in manually.');
      }
    } catch (error) {
      console.error('System detection error:', error);
      showNotification('error', 'System detection unavailable. Please fill in manually.');
    } finally {
      setIsDetecting(false);
    }
  };



  // Add folder to tracking list
  const addFolderToTracking = () => {
    if (!currentFolderInput.trim()) {
      showNotification('error', 'Please enter a folder path');
      return;
    }

    if (folderPaths.includes(currentFolderInput.trim())) {
      showNotification('error', 'Folder already added');
      return;
    }

    setFolderPaths(prev => [...prev, currentFolderInput.trim()]);
    setCurrentFolderInput('');
    showNotification('success', 'Folder added for scanning');
  };

  // Remove folder from tracking list
  const removeFolderFromTracking = async (folderToRemove: string) => {
    try {
      // Call backend API to remove folder from database
      const response = await fetch('/api/config/folders', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          folderPaths: [folderToRemove]
        }),
      });

      if (response.ok) {
        // Only update UI state if backend removal was successful
        setFolderPaths(prev => prev.filter(f => f !== folderToRemove));
        showNotification('success', 'Folder removed from tracking');
      } else {
        const errorData = await response.json();
        showNotification('error', `Failed to remove folder: ${errorData.error || 'Unknown error'}`);
      }
    } catch (error) {
      showNotification('error', `Error removing folder: ${error}`);
    }
  };

  // Scan all folders (supports both single and multiple folder modes)
  const scanModelFolder = async () => {
    // Determine which folders to scan
    let foldersToScan: string[] = [];
    
    if (folderPaths.length > 0) {
      // Multi-folder mode
      foldersToScan = folderPaths;
    } else if (folderPath.trim()) {
      // Single folder mode (backward compatibility)
      foldersToScan = [folderPath.trim()];
    } else {
      showNotification('error', 'Please enter at least one folder path');
      return;
    }

    setIsScanning(true);
    
    // Start progress polling for scanning
    const pollScanProgress = setInterval(async () => {
      try {
        const progressResponse = await fetch('/api/setup/progress');
        if (progressResponse.ok) {
          const progressData = await progressResponse.json();
          console.log('Scan progress:', progressData);
          // You could use this data to show scan progress in the UI
        }
      } catch (pollError) {
        console.warn('Failed to poll scan progress:', pollError);
      }
    }, 500);
    
    try {
      const response = await fetch('/api/config/scan-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          folderPaths: foldersToScan, 
          recursive: true,
          addToDatabase: true // Persist to JSON database
        }),
      });

      clearInterval(pollScanProgress);

      if (response.ok) {
        const data = await response.json();
        setScanResults(data.models || []);
        
        if (data.models && data.models.length > 0) {
          const summary = data.scanSummary || [];
          const successCount = summary.filter((s: any) => s.status === 'success').length;
          const errorCount = summary.filter((s: any) => s.status === 'error').length;
          
          showNotification('success', 
            `Found ${data.models.length} GGUF models from ${successCount} folders!` +
            (errorCount > 0 ? ` (${errorCount} folders had errors)` : '')
          );
          setCurrentStep(3); // Move to model selection step
        } else {
          showNotification('error', 'No GGUF models found in this folder');
        }
      } else {
        showNotification('error', 'Failed to scan folder');
      }
    } catch (error) {
      clearInterval(pollScanProgress);
      showNotification('error', 'Scan error: ' + error);
    } finally {
      setIsScanning(false);
    }
  };

  const generateSmartConfig = async () => {
    setIsGenerating(true);
    const startTime = new Date();
    
    // Initialize progress
    setGenerationProgress({
      current: 0,
      total: scanResults.length,
      currentModel: '',
      stage: 'Initializing configuration generation...',
      startTime,
      estimatedTimeRemaining: null
    });
    
    try {
      showNotification('info', '🚀 Generating your personalized SMART configuration...');
      
      // Connect to real-time progress updates via polling
      // This replaces the old simulated progress with actual backend progress
      const pollProgressInterval = setInterval(async () => {
        try {
          const progressResponse = await fetch('/api/setup/progress');
          if (progressResponse.ok) {
            const progressData = await progressResponse.json();
            
            const elapsed = (new Date().getTime() - startTime.getTime()) / 1000;
            const remaining = progressData.progress > 0 && progressData.progress < 100 ? 
              (elapsed * (100 - progressData.progress)) / progressData.progress : null;
            
            setGenerationProgress({
              current: progressData.processed_models || 0,
              total: progressData.total_models || scanResults.length,
              currentModel: progressData.current_model || '',
              stage: progressData.current_step || 'Generating configuration...',
              startTime,
              estimatedTimeRemaining: remaining ? Math.ceil(remaining) : null
            });

            // Check if completed or error
            if (progressData.completed || progressData.status === 'completed') {
              clearInterval(pollProgressInterval);
              setGenerationProgress(prev => ({
                ...prev,
                current: prev.total,
                stage: 'Configuration generated successfully!',
                estimatedTimeRemaining: 0
              }));
            } else if (progressData.error) {
              clearInterval(pollProgressInterval);
              throw new Error(progressData.error);
            }
          }
        } catch (pollError) {
          console.warn('Failed to poll progress:', pollError);
        }
      }, 500); // Poll every 500ms
      
      // Cleanup function for polling
      const cleanup = () => {
        clearInterval(pollProgressInterval);
      };

      // Save system settings to persistent storage first
      // This ensures user's manual hardware selections persist across Force Reconfigure
      try {
        const settingsResponse = await fetch('/api/settings/system', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            backend: systemConfig.backend,
            vramGB: systemConfig.vramGB,
            ramGB: systemConfig.ramGB,
            preferredContext: systemConfig.preferredContext,
            throughputFirst: systemConfig.throughputFirst,
            enableJinja: true,
          })
        });
        
        if (!settingsResponse.ok) {
          console.warn('Failed to save system settings:', await settingsResponse.text());
          // Continue anyway - settings save is not critical to config generation
        }
      } catch (settingsError) {
        console.warn('Error saving system settings:', settingsError);
        // Continue anyway
      }

      // Use the database-driven config generation that handles ALL tracked folders
      // This is the SAME endpoint that "Force Reconfigure" uses
      const response = await fetch('/api/config/regenerate-from-db', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          options: {
            enableJinja: true,
            throughputFirst: systemConfig.throughputFirst,
            minContext: Math.min(16384, systemConfig.preferredContext),
            preferredContext: systemConfig.preferredContext,
            forceBackend: systemConfig.backend,      // Force the user-selected backend
            forceVRAM: systemConfig.vramGB,         // Force the user-selected VRAM
            forceRAM: systemConfig.ramGB,           // Force the user-selected RAM
          }
        }),
      });

      // Clean up polling
      cleanup();

      if (!response.ok) {
        cleanup(); // Clean up on error too
        throw new Error('Failed to generate configuration');
      }

      // Complete progress
      setGenerationProgress(prev => ({
        ...prev,
        current: prev.total,
        stage: 'Configuration generated successfully!',
        estimatedTimeRemaining: 0
      }));

      await response.json();
      showNotification('success', '🎉 Configuration generated successfully!');
      setCurrentStep(5); // Move to completion step
      
    } catch (error) {
      showNotification('error', 'Error generating configuration: ' + error);
    } finally {
      setIsGenerating(false);
      setGenerationProgress(prev => ({ ...prev, estimatedTimeRemaining: 0 }));
    }
  };

  const steps = [
    {
      title: "Welcome to ClaraCore Setup! 🚀",
      description: "Let's get you set up with your AI models in just a few steps",
      component: (
        <div className="text-center py-8">
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ type: "spring", stiffness: 200 }}
            className="w-24 h-24 bg-gradient-to-br from-brand-500 to-brand-600 rounded-full flex items-center justify-center mx-auto mb-6"
          >
            <ZapIcon className="w-12 h-12 text-white" />
          </motion.div>
          <h2 className="text-2xl font-bold text-text-primary mb-4">Ready to get started?</h2>
          <p className="text-text-secondary mb-8 max-w-md mx-auto">
            We'll help you scan your model folder, detect your system capabilities, 
            and generate an optimized configuration automatically.
          </p>
          <Button 
            onClick={() => setCurrentStep(1)}
            size="lg"
            icon={<ArrowRightIcon size={20} />}
          >
            Let's Begin!
          </Button>
        </div>
      )
    },
    {
      title: "Step 1: Choose Your Model Source 🎯",
      description: "Do you have existing models or would you like to download new ones?",
      component: (
        <div className="py-8">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-2xl mx-auto">
            {/* Existing Folder Option */}
            <motion.div
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              className={`p-8 rounded-xl border-2 cursor-pointer transition-all ${
                modelSource === 'existing'
                  ? 'border-brand-500  dark:bg-brand-900/20'
                  : 'border-border-secondary bg-surface hover:border-brand-300 hover:bg-surface-secondary'
              }`}
              onClick={() => setModelSource('existing')}
            >
              <div className="text-center">
                <div className="w-16 h-16 bg-gradient-to-br from-blue-500 to-blue-600 rounded-full flex items-center justify-center mx-auto mb-4">
                  <FolderIcon className="w-8 h-8 text-white" />
                </div>
                <h3 className="text-xl font-bold text-text-primary mb-2">
                  I Have Models
                </h3>
                <p className="text-text-secondary mb-4">
                  I already have GGUF model files on my system that I want to use
                </p>
                <div className="text-sm text-text-secondary space-y-1">
                  <div>✓ Use existing model collection</div>
                  <div>✓ Scan local folder</div>
                  <div>✓ Quick setup</div>
                </div>
              </div>
            </motion.div>

            {/* Download New Option */}
            <motion.div
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              className={`p-8 rounded-xl border-2 cursor-pointer transition-all ${
                modelSource === 'download'
                  ? 'border-brand-500  dark:bg-brand-900/20'
                  : 'border-border-secondary bg-surface hover:border-brand-300 hover:bg-surface-secondary'
              }`}
              onClick={() => setModelSource('download')}
            >
              <div className="text-center">
                <div className="w-16 h-16 bg-gradient-to-br from-green-500 to-green-600 rounded-full flex items-center justify-center mx-auto mb-4">
                  <DownloadIcon className="w-8 h-8 text-white" />
                </div>
                <h3 className="text-xl font-bold text-text-primary mb-2">
                  Download Models
                </h3>
                <p className="text-text-secondary mb-4">
                  I want to browse and download new models from Hugging Face
                </p>
                <div className="text-sm text-text-secondary space-y-1">
                  <div>✓ Browse model library</div>
                  <div>✓ Download latest models</div>
                  <div>✓ Guided selection</div>
                </div>
              </div>
            </motion.div>
          </div>

          {/* Action Buttons */}
          <div className="flex justify-center mt-8 space-x-4">
            {modelSource === 'existing' && (
              <Button
                onClick={() => setCurrentStep(2)}
                icon={<ArrowRightIcon size={16} />}
                size="lg"
              >
                Continue with Existing Models
              </Button>
            )}
            {modelSource === 'download' && (
              <Button
                onClick={() => window.location.href = '/ui/downloader'}
                icon={<DownloadIcon size={16} />}
                size="lg"
                className="bg-green-500 hover:bg-green-600 text-white"
              >
                Go to Model Downloader
              </Button>
            )}
            <Button
              variant="outline"
              onClick={() => setCurrentStep(0)}
              icon={<ArrowLeftIcon size={16} />}
            >
              Back
            </Button>
          </div>
        </div>
      )
    },
    {
      title: "Step 2: Where are your models? 📁",
      description: "Point us to the folder containing your GGUF model files (Takes a bit to scan if too many files)",
      component: (
        <div className="py-6">
          {/* Database info panel */}
          {folderPaths.length > 0 && (
            <div className="mb-6 p-4 bg-gradient-to-r from-brand-50 to-blue-50 dark:from-brand-900/20 dark:to-blue-900/20 border border-brand-200 dark:border-brand-800 rounded-lg">
              <div className="flex items-center space-x-2 mb-2">
                <InfoIcon className="w-5 h-5 text-brand-600 dark:text-brand-400" />
                <h3 className="font-semibold text-brand-700 dark:text-brand-300">Smart Folder Database Active</h3>
              </div>
              <p className="text-sm text-brand-600 dark:text-brand-400">
                Your folders are saved to a persistent database. Future model downloads will automatically update your configuration.
                You can scan all folders again or add new ones below.
              </p>
            </div>
          )}

          {/* Multi-folder management */}
          {folderPaths.length > 0 && (
            <div className="mb-6">
              <label className="block text-sm font-medium text-text-secondary mb-3">
                📁 Tracked Model Folders ({folderPaths.length})
              </label>
              <div className="space-y-2 max-h-32 overflow-y-auto">
                {folderPaths.map((folder, index) => (
                  <div key={index} className="flex items-center justify-between p-3 bg-surface-secondary border border-border-secondary rounded-lg">
                    <div className="flex items-center space-x-2">
                      <FolderIcon className="w-4 h-4 text-text-secondary" />
                      <span className="text-sm text-text-primary font-mono">{folder}</span>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeFolderFromTracking(folder)}
                      className="text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                    >
                      Remove
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Folder input section */}
          <div className="mb-6">
            <label className="block text-sm font-medium text-text-secondary mb-3">
              {folderPaths.length > 0 ? 'Add Another Model Folder' : 'Model Folder Path'}
            </label>
            <div className="flex space-x-2">
              <Input
                value={folderPaths.length > 0 ? currentFolderInput : folderPath}
                onChange={(e) => folderPaths.length > 0 ? setCurrentFolderInput(e.target.value) : setFolderPath(e.target.value)}
                placeholder="C:\models\llama-models"
                className="text-lg flex-1"
              />
              {folderPaths.length > 0 && (
                <Button
                  onClick={addFolderToTracking}
                  variant="outline"
                  disabled={!currentFolderInput.trim()}
                  icon={<ArrowRightIcon size={16} />}
                >
                  Add
                </Button>
              )}
            </div>
            <p className="text-sm text-text-secondary mt-2">
              💡 All folders will be scanned recursively for .gguf files
              {folderPaths.length === 0 && (
                <span className="block mt-1">
                  ✨ <strong>Tip:</strong> After scanning your first folder, you can add more folders to the database!
                </span>
              )}
            </p>
          </div>
          
          <div className="flex space-x-4">
            <Button
              onClick={scanModelFolder}
              loading={isScanning}
              icon={<RefreshCwIcon size={16} />}
              disabled={folderPaths.length === 0 && !folderPath.trim()}
            >
              {isScanning ? 'Scanning...' : folderPaths.length > 0 ? `Scan ${folderPaths.length} Folders` : 'Scan Folder'}
            </Button>
            
            {/* Multi-folder mode switch */}
            {folderPaths.length === 0 && folderPath.trim() && (
              <Button
                variant="outline"
                onClick={() => {
                  setFolderPaths([folderPath.trim()]);
                  setCurrentFolderInput('');
                  setFolderPath('');
                  showNotification('info', 'Switched to multi-folder mode. Add more folders now!');
                }}
                icon={<FolderIcon size={16} />}
                className="text-brand-600 border-brand-300 hover: dark:hover:bg-brand-900/20"
              >
                + Add More Folders
              </Button>
            )}
            
            <Button
              variant="outline"
              onClick={() => setCurrentStep(1)}
              icon={<ArrowLeftIcon size={16} />}
            >
              Back
            </Button>
          </div>
        </div>
      )
    },
    {
      title: `Found ${scanResults.length} Models! 🎯`,
      description: "These models will be configured for optimal performance",
      component: (
        <div className="py-6">
          <div className="max-h-64 overflow-y-auto mb-6">
            <div className="grid grid-cols-1 gap-3">
              {scanResults.slice(0, 8).map((model, index) => (
                <motion.div
                  key={index}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: index * 0.1 }}
                  className="p-4 bg-surface-secondary rounded-lg border border-border-secondary"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <FileIcon className="w-5 h-5 text-brand-500" />
                      <div>
                        <h4 className="font-medium text-text-primary">{model.name}</h4>
                        <p className="text-sm text-text-secondary">
                          {model.quantization} • {model.sizeFormatted}
                          {model.isInstruct && " • Instruct"}
                          {model.isEmbedding && " • Embedding"}
                        </p>
                      </div>
                    </div>
                    <CheckCircleIcon className="w-5 h-5 text-success-500" />
                  </div>
                </motion.div>
              ))}
              {scanResults.length > 8 && (
                <p className="text-sm text-text-secondary text-center py-2">
                  ... and {scanResults.length - 8} more models
                </p>
              )}
            </div>
          </div>
          
          <div className="flex space-x-4">
            <Button
              onClick={() => setCurrentStep(4)}
              icon={<ArrowRightIcon size={16} />}
            >
              Configure These Models
            </Button>
            <Button
              variant="outline"
              onClick={() => setCurrentStep(2)}
              icon={<ArrowLeftIcon size={16} />}
            >
              Back
            </Button>
          </div>
        </div>
      )
    },
    {
      title: "Step 3: System Configuration",
      description: "Configure your hardware settings for optimal AI model performance.",
      component: (
        <div className="py-6 space-y-8">
          {/* Detection Status Header */}
          <div className="flex items-center justify-between p-4 bg-surface-secondary rounded-xl border border-border-secondary">
            <div className="flex items-center space-x-3">
              {autoDetected && systemDetection ? (
                <>
                  <div className="w-10 h-10 bg-success-500 rounded-full flex items-center justify-center">
                    <CheckCircleIcon className="w-6 h-6 text-white" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-text-primary">Hardware Detected</h3>
                    <p className="text-sm text-text-secondary">
                      {systemDetection.primaryGPU ? systemDetection.primaryGPU.name : 'CPU-only system'}
                    </p>
                  </div>
                </>
              ) : (
                <>
                  <div className="w-10 h-10 bg-warning-500 rounded-full flex items-center justify-center">
                    <SearchIcon className="w-6 h-6 text-white" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-text-primary">Manual Configuration</h3>
                    <p className="text-sm text-text-secondary">
                      Please configure your hardware settings
                    </p>
                  </div>
                </>
              )}
            </div>
            <Button
              variant="outline"
              onClick={detectSystem}
              loading={isDetecting}
              disabled={isDetecting}
              className="!bg-surface-secondary !text-text-primary !border-border-secondary hover:!bg-surface-tertiary hover:!border-brand-500 hover:!text-text-primary"
            >
              {isDetecting ? (
                <>
                  <motion.div
                    className="w-4 h-4 border-2 border-current border-t-transparent rounded-full mr-2"
                    animate={{ rotate: 360 }}
                    transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                  />
                  Detecting...
                </>
              ) : (
                <>
                  <SearchIcon className="w-4 h-4 mr-2" />
                  {autoDetected ? 'Re-detect' : 'Auto-detect'}
                </>
              )}
            </Button>
          </div>

          {/* GPU Configuration */}
          <Card className="p-6 bg-surface border-border-secondary">
            <div className="flex items-center mb-6">
              <div className="w-8 h-8 0 rounded-lg flex items-center justify-center mr-3">
                <MonitorIcon className="w-5 h-5 text-white" />
              </div>
              <div>
                <h3 className="font-semibold text-text-primary">Graphics Processing</h3>
                <p className="text-sm text-text-secondary">Configure your GPU for acceleration</p>
              </div>
            </div>

            <div className="space-y-6">
              {/* GPU Type Selection */}
              <div>
                <label className="block text-sm font-medium text-text-primary mb-3">
                  Hardware Type
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                      systemConfig.hasGPU 
                        ? 'border-brand-500  dark:bg-brand-900/20' 
                        : 'border-border-secondary bg-background hover:border-border-accent'
                    }`}
                    onClick={() => setSystemConfig(prev => ({ ...prev, hasGPU: true, backend: 'cuda' }))}
                  >
                    <div className="flex items-center">
                      <div className={`w-4 h-4 rounded-full border-2 mr-3 ${
                        systemConfig.hasGPU ? 'border-brand-500 0' : 'border-border-secondary'
                      }`}>
                        {systemConfig.hasGPU && <div className="w-2 h-2 bg-white rounded-full mx-auto mt-0.5" />}
                      </div>
                      <div>
                        <div className="font-medium text-text-primary">Dedicated GPU</div>
                        <div className="text-xs text-text-secondary">NVIDIA, AMD, or Intel graphics card</div>
                      </div>
                    </div>
                  </motion.div>
                  
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                      !systemConfig.hasGPU 
                        ? 'border-brand-500  dark:bg-brand-900/20' 
                        : 'border-border-secondary bg-background hover:border-border-accent'
                    }`}
                    onClick={() => setSystemConfig(prev => ({ ...prev, hasGPU: false, backend: 'cpu' }))}
                  >
                    <div className="flex items-center">
                      <div className={`w-4 h-4 rounded-full border-2 mr-3 ${
                        !systemConfig.hasGPU ? 'border-brand-500 0' : 'border-border-secondary'
                      }`}>
                        {!systemConfig.hasGPU && <div className="w-2 h-2 bg-white rounded-full mx-auto mt-0.5" />}
                      </div>
                      <div>
                        <div className="font-medium text-text-primary">CPU Only</div>
                        <div className="text-xs text-text-secondary">Use processor for all computations</div>
                      </div>
                    </div>
                  </motion.div>
                </div>
              </div>

              {/* GPU Specific Settings */}
              {systemConfig.hasGPU && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="space-y-4"
                >
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-text-primary mb-2">
                        GPU Brand
                      </label>
                      <select
                        value={systemConfig.gpuType}
                        onChange={(e) => {
                          const gpuType = e.target.value as 'nvidia' | 'amd' | 'intel' | 'apple';
                          let backend: 'cuda' | 'rocm' | 'vulkan' | 'metal' | 'mlx' | 'cpu';
                          // Set default backend based on GPU type, but allow user to change it
                          if (gpuType === 'nvidia') backend = 'cuda';
                          else if (gpuType === 'amd') backend = 'rocm';
                          else if (gpuType === 'apple') backend = 'metal';
                          else backend = 'vulkan';

                          // Platform-aware filtering: on macOS Apple Silicon, map unsupported backends to metal
                          const isAppleSilicon = (systemDetection?.primaryGPU?.brand === 'apple') || (/Mac/i.test(navigator.platform));
                          if (isAppleSilicon) {
                            const unsupported = backend === 'rocm' || backend === 'vulkan' || backend === 'cuda';
                            if (unsupported) {
                              setBackendNotice(`Requested backend '${backend}' is unsupported on macOS Apple Silicon. Using 'metal' instead.`);
                              backend = 'metal';
                            } else {
                              setBackendNotice(null);
                            }
                          } else {
                            setBackendNotice(null);
                          }

                          setSystemConfig(prev => ({ ...prev, gpuType, backend }));
                        }}
                        className="w-full p-3 border border-border-secondary rounded-lg bg-background text-text-primary focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                      >
                        {/* Hide/disable incompatible GPU families based on platform */}
                        <option value="nvidia" disabled={systemDetection?.primaryGPU?.brand === 'apple'}>
                          NVIDIA (RTX, GTX)
                        </option>
                        <option value="amd" disabled={systemDetection?.primaryGPU?.brand === 'apple'}>
                          AMD (RX, Radeon)
                        </option>
                        <option value="intel" disabled={systemDetection?.primaryGPU?.brand === 'apple'}>
                          Intel (Arc, Iris)
                        </option>
                        <option value="apple">Apple Silicon (M1, M2, M3, M4)</option>
                      </select>
                    </div>

                    {/* Backend Selection */}
                    <div>
                      <label className="block text-sm font-medium text-text-primary mb-2">
                        Compute Backend
                        <span className="text-xs text-text-secondary ml-1">(Choose your GPU acceleration)</span>
                      </label>
                      <select
                        value={systemConfig.backend}
                        onChange={(e) => {
                          const backend = e.target.value as 'cuda' | 'rocm' | 'vulkan' | 'metal' | 'mlx' | 'cpu';

                          // Platform-aware warnings
                          const isAppleSilicon = (systemDetection?.primaryGPU?.brand === 'apple') || (/Mac/i.test(navigator.platform));
                          if (isAppleSilicon && ['rocm', 'vulkan', 'cuda'].includes(backend)) {
                            setBackendNotice(`Warning: '${backend}' may not be supported on macOS Apple Silicon. Consider using 'metal' instead.`);
                          } else {
                            setBackendNotice(null);
                          }

                          setSystemConfig(prev => ({ ...prev, backend }));
                        }}
                        className="w-full p-3 border border-border-secondary rounded-lg bg-background text-text-primary focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                      >
                        {/* CUDA - NVIDIA only */}
                        <option value="cuda" disabled={systemConfig.gpuType !== 'nvidia'}>
                          CUDA (NVIDIA only) {systemConfig.gpuType !== 'nvidia' ? '- Not available for your GPU' : '- Recommended for NVIDIA'}
                        </option>

                        {/* ROCm - AMD only */}
                        <option value="rocm" disabled={systemConfig.gpuType !== 'amd'}>
                          ROCm (AMD only) {systemConfig.gpuType !== 'amd' ? '- Not available for your GPU' : '- Recommended for AMD'}
                        </option>

                        {/* Vulkan - Universal but not Apple */}
                        <option value="vulkan" disabled={systemConfig.gpuType === 'apple'}>
                          Vulkan (Cross-platform) {systemConfig.gpuType === 'apple' ? '- Not available on Apple Silicon' : '- Works on NVIDIA/AMD/Intel'}
                        </option>

                        {/* Metal - Apple only */}
                        <option value="metal" disabled={systemConfig.gpuType !== 'apple'}>
                          Metal (Apple only) {systemConfig.gpuType !== 'apple' ? '- Only for Apple Silicon' : '- Recommended for Apple'}
                        </option>

                        {/* CPU - Always available */}
                        <option value="cpu">
                          CPU (No GPU acceleration) - Works everywhere but slower
                        </option>
                      </select>
                    </div>

                    {backendNotice && (
                      <div className="mt-2 p-2 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800">
                        <div className="text-xs text-amber-700 dark:text-amber-300">
                          ⚠️ {backendNotice}
                        </div>
                      </div>
                    )}
                    
                    <div>
                      <label className="block text-sm font-medium text-text-primary mb-2">
                        VRAM (GB)
                      </label>
                      <Input
                        type="number"
                        value={systemConfig.vramGB}
                        onChange={(e) => setSystemConfig(prev => ({ ...prev, vramGB: parseInt(e.target.value) || 0 }))}
                        placeholder="24"
                        min="4"
                        max="128"
                        className="bg-background border-border-secondary focus:border-brand-500"
                      />
                    </div>
                  </div>
                  
                  {autoDetected && systemDetection?.primaryGPU && (
                    <div className="p-3 bg-info-50 dark:bg-info-900/20 rounded-lg border border-info-200 dark:border-info-800">
                      <div className="flex items-center text-info-700 dark:text-info-300">
                        <CheckCircleIcon className="w-4 h-4 mr-2" />
                        <span className="text-sm">
                          Detected: {systemDetection.primaryGPU.name} ({systemDetection.primaryGPU.vramGB}GB VRAM)
                        </span>
                      </div>
                    </div>
                  )}
                </motion.div>
              )}
            </div>
          </Card>

          {/* Memory Configuration */}
          <Card className="p-6 bg-surface border-border-secondary">
            <div className="flex items-center mb-6">
              <div className="w-8 h-8 0 rounded-lg flex items-center justify-center mr-3">
                <MemoryStickIcon className="w-5 h-5 text-white" />
              </div>
              <div>
                <h3 className="font-semibold text-text-primary">System Memory</h3>
                <p className="text-sm text-text-secondary">Configure RAM and performance settings</p>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-text-primary mb-2">
                  Total RAM (GB)
                </label>
                <Input
                  type="number"
                  value={systemConfig.ramGB}
                  onChange={(e) => setSystemConfig(prev => ({ ...prev, ramGB: parseInt(e.target.value) || 0 }))}
                  placeholder="64"
                  min="8"
                  max="256"
                  className="bg-background border-border-secondary focus:border-brand-500 text-lg font-medium"
                />
                {autoDetected && systemDetection && (
                  <div className="mt-2 p-3 bg-success-50 dark:bg-success-900/20 rounded-lg border border-success-200 dark:border-success-800">
                    <div className="flex items-center text-success-700 dark:text-success-300">
                      <CheckCircleIcon className="w-4 h-4 mr-2" />
                      <span className="text-sm">
                        Detected: {systemDetection.totalRAMGB}GB total ({systemDetection.availableRAMGB}GB available)
                      </span>
                    </div>
                  </div>
                )}
              </div>
              
              <div>
                <label className="block text-sm font-medium text-text-primary mb-2">
                  Performance Mode
                </label>
                <div className="space-y-3">
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className={`p-3 rounded-lg border-2 cursor-pointer transition-all ${
                      systemConfig.throughputFirst 
                        ? 'border-brand-500  dark:bg-brand-900/20' 
                        : 'border-border-secondary bg-background hover:border-border-accent'
                    }`}
                    onClick={() => setSystemConfig(prev => ({ ...prev, throughputFirst: true }))}
                  >
                    <div className="flex items-center">
                      <div className={`w-4 h-4 rounded-full border-2 mr-3 ${
                        systemConfig.throughputFirst ? 'border-brand-500 0' : 'border-border-secondary'
                      }`}>
                        {systemConfig.throughputFirst && <div className="w-2 h-2 bg-white rounded-full mx-auto mt-0.5" />}
                      </div>
                      <div>
                        <div className="font-medium text-text-primary">Speed Priority</div>
                        <div className="text-xs text-text-secondary">Higher throughput, faster responses</div>
                      </div>
                    </div>
                  </motion.div>
                  
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className={`p-3 rounded-lg border-2 cursor-pointer transition-all ${
                      !systemConfig.throughputFirst 
                        ? 'border-brand-500  dark:bg-brand-900/20' 
                        : 'border-border-secondary bg-background hover:border-border-accent'
                    }`}
                    onClick={() => setSystemConfig(prev => ({ ...prev, throughputFirst: false }))}
                  >
                    <div className="flex items-center">
                      <div className={`w-4 h-4 rounded-full border-2 mr-3 ${
                        !systemConfig.throughputFirst ? 'border-brand-500 0' : 'border-border-secondary'
                      }`}>
                        {!systemConfig.throughputFirst && <div className="w-2 h-2 bg-white rounded-full mx-auto mt-0.5" />}
                      </div>
                      <div>
                        <div className="font-medium text-text-primary">Quality Priority</div>
                        <div className="text-xs text-text-secondary">Larger context, better understanding</div>
                      </div>
                    </div>
                  </motion.div>
                </div>
                
                {autoDetected && systemDetection?.recommendations && (
                  <div className="mt-3 p-3 bg-info-50 dark:bg-info-900/20 rounded-lg border border-info-200 dark:border-info-800">
                    <div className="flex items-center text-info-700 dark:text-info-300">
                      <CheckCircleIcon className="w-4 h-4 mr-2" />
                      <span className="text-sm">
                        Recommended: {systemDetection.recommendations.throughputFirst ? 'Speed' : 'Quality'} priority
                      </span>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </Card>

          {/* Context Configuration */}
          <Card className="p-6 bg-surface border-border-secondary">
            <div className="flex items-center mb-6">
              <div className="w-8 h-8 0 rounded-lg flex items-center justify-center mr-3">
                <SettingsIcon className="w-5 h-5 text-white" />
              </div>
              <div>
                <h3 className="font-semibold text-text-primary">Context Settings</h3>
                <p className="text-sm text-text-secondary">Configure model context window size</p>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-text-primary mb-3">
                Context Window Size
              </label>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
                {[
                  { value: 8192, label: '8K', desc: 'Fast & efficient' },
                  { value: 16384, label: '16K', desc: 'Balanced' },
                  { value: 32768, label: '32K', desc: 'Recommended' },
                  { value: 65536, label: '64K', desc: 'Large documents' },
                  { value: 131072, label: '128K', desc: 'Maximum' }
                ].map((option) => (
                  <motion.div
                    key={option.value}
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className={`p-4 rounded-lg border-2 cursor-pointer transition-all text-center ${
                      systemConfig.preferredContext === option.value
                        ? 'border-brand-500  dark:bg-brand-900/20' 
                        : 'border-border-secondary bg-background hover:border-border-accent'
                    }`}
                    onClick={() => setSystemConfig(prev => ({ ...prev, preferredContext: option.value }))}
                  >
                    <div className="font-semibold text-text-primary text-lg">{option.label}</div>
                    <div className="text-xs text-text-secondary mt-1">{option.desc}</div>
                    {systemConfig.preferredContext === option.value && (
                      <motion.div
                        initial={{ scale: 0 }}
                        animate={{ scale: 1 }}
                        className="w-2 h-2 0 rounded-full mx-auto mt-2"
                      />
                    )}
                  </motion.div>
                ))}
              </div>
              
              {autoDetected && systemDetection?.recommendations?.suggestedContextSize && (
                <div className="mt-4 p-3 bg-info-50 dark:bg-info-900/20 rounded-lg border border-info-200 dark:border-info-800">
                  <div className="flex items-center text-info-700 dark:text-info-300">
                    <CheckCircleIcon className="w-4 h-4 mr-2" />
                    <span className="text-sm">
                      Recommended: {systemDetection.recommendations.suggestedContextSize.toLocaleString()} tokens for your system
                    </span>
                  </div>
                </div>
              )}
            </div>
          </Card>
          
          {/* System Recommendations */}
          {autoDetected && systemDetection && systemDetection.recommendations?.notes && systemDetection.recommendations.notes.length > 0 && (
            <Card className="p-6 bg-surface border-border-secondary border-l-4 border-l-info-500">
              <div className="flex items-center mb-4">
                <div className="w-8 h-8 bg-info-500 rounded-lg flex items-center justify-center mr-3">
                  <InfoIcon className="w-5 h-5 text-white" />
                </div>
                <div>
                  <h3 className="font-semibold text-text-primary">System Recommendations</h3>
                  <p className="text-sm text-text-secondary">Optimizations based on your hardware</p>
                </div>
              </div>
              <div className="space-y-3">
                {systemDetection.recommendations.notes.map((note, index) => (
                  <div key={index} className="flex items-start p-3 bg-info-50 dark:bg-info-900/20 rounded-lg">
                    <CheckCircleIcon className="w-4 h-4 text-info-500 mr-3 mt-0.5 flex-shrink-0" />
                    <span className="text-sm text-info-700 dark:text-info-200">{note}</span>
                  </div>
                ))}
              </div>
            </Card>
          )}
          
          {/* Progress Display during Generation */}
          {isGenerating && (
            <Card className="border-brand-500/20 bg-gradient-to-r from-brand-50 to-brand-100 dark:from-brand-900/20 dark:to-brand-800/20">
              <CardContent className="p-6">
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="font-semibold text-brand-700 dark:text-brand-300">Configuration Generation in Progress</h3>
                    <span className="text-sm text-brand-600 dark:text-brand-400">
                      {generationProgress.current}/{generationProgress.total} models
                    </span>
                  </div>
                  
                  {/* Progress Bar */}
                  <div className="w-full bg-brand-200 dark:bg-brand-800 rounded-full h-2.5">
                    <motion.div
                      className="bg-gradient-to-r from-brand-500 to-brand-600 h-2.5 rounded-full"
                      initial={{ width: 0 }}
                      animate={{ 
                        width: `${generationProgress.total > 0 ? (generationProgress.current / generationProgress.total) * 100 : 0}%` 
                      }}
                      transition={{ duration: 0.5, ease: "easeOut" }}
                    />
                  </div>
                  
                  {/* Current Status */}
                  <div className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <motion.div
                        className="w-3 h-3 border-2 border-brand-500 border-t-transparent rounded-full"
                        animate={{ rotate: 360 }}
                        transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                      />
                      <span className="text-brand-700 dark:text-brand-300">
                        {generationProgress.stage}
                      </span>
                    </div>
                    
                    {generationProgress.estimatedTimeRemaining && generationProgress.estimatedTimeRemaining > 0 && (
                      <span className="text-brand-600 dark:text-brand-400 font-mono">
                        ~{generationProgress.estimatedTimeRemaining}s remaining
                      </span>
                    )}
                  </div>
                  
                  {/* Current Model */}
                  {generationProgress.currentModel && (
                    <div className="text-xs text-brand-600 dark:text-brand-400 bg-brand-100 dark:bg-brand-800/50 px-3 py-2 rounded-md">
                      <span className="font-medium">Currently processing:</span> {generationProgress.currentModel}
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
          
          {/* Database approach information */}
          {folderPaths.length > 0 && (
            <Card className="mb-6 border-info-200 dark:border-info-800 bg-gradient-to-r from-info-50 to-blue-50 dark:from-info-900/20 dark:to-blue-900/20">
              <CardContent className="p-4">
                <div className="flex items-start space-x-3">
                  <InfoIcon className="w-5 h-5 text-info-600 dark:text-info-400 mt-0.5 flex-shrink-0" />
                  <div>
                    <h3 className="font-semibold text-info-700 dark:text-info-300 mb-2">Smart Database Configuration</h3>
                    <p className="text-sm text-info-600 dark:text-info-400 mb-3">
                      Your {folderPaths.length} folder{folderPaths.length > 1 ? 's are' : ' is'} saved to a persistent JSON database (<code>model_folders.json</code>). 
                      This configuration will be regenerated from <strong>all tracked folders</strong>, ensuring your YAML config stays current even if individual models are added or removed.
                    </p>
                    <div className="flex items-center space-x-4 text-xs text-info-500 dark:text-info-400">
                      <span>📁 {folderPaths.length} tracked folder{folderPaths.length > 1 ? 's' : ''}</span>
                      <span>🔄 Auto-regeneration enabled</span>
                      <span>💾 Database persisted</span>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Navigation */}
          <div className="flex items-center justify-between pt-6 border-t border-border-secondary">
            <Button
              variant="outline"
              onClick={() => setCurrentStep(3)}
              className="bg-background hover:bg-surface text-text-primary border-border-secondary hover:border-brand-500"
            >
              <ArrowLeftIcon className="w-4 h-4 mr-2" />
              Back to Model Selection
            </Button>
            
            <Button
              onClick={generateSmartConfig}
              loading={isGenerating}
              disabled={isGenerating}
              size="lg"
              className="bg-gradient-to-r from-brand-500 to-brand-600 hover:from-brand-400 hover:to-brand-500 text-white shadow-lg"
            >
              {isGenerating ? (
                <div className="flex items-center justify-center">
                  <motion.div
                    className="w-4 h-4 border-2 border-current border-t-transparent rounded-full mr-3"
                    animate={{ rotate: 360 }}
                    transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                  />
                  <span>Processing {generationProgress.current}/{generationProgress.total}</span>
                </div>
              ) : (
                <>
                  <WandIcon className="w-4 h-4 mr-2" />
                  Generate Smart Configuration
                </>
              )}
            </Button>
          </div>
        </div>
      )
    },
    {
      title: "Setup Complete! 🎉",
      description: "Your ClaraCore configuration has been generated",
      component: (
        <div className="text-center py-8">
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ type: "spring", stiffness: 200, delay: 0.2 }}
            className="w-24 h-24 bg-gradient-to-br from-success-500 to-success-600 rounded-full flex items-center justify-center mx-auto mb-6"
          >
            <CheckCircleIcon className="w-12 h-12 text-white" />
          </motion.div>
          <h2 className="text-2xl font-bold text-text-primary mb-4">All Set! 🚀</h2>
          <p className="text-text-secondary mb-8 max-w-md mx-auto">
            Your configuration has been optimized for your system with {scanResults.length} models configured.
          </p>
          <div className="space-y-4">
            <Button 
              onClick={() => {
                setShowSetup(false);
                window.location.href = '/ui/';
              }}
              size="lg"
              icon={<ZapIcon size={20} />}
            >
              Start Using ClaraCore
            </Button>
            <br />
            <Button 
              variant="outline"
              onClick={() => window.location.href = '/ui/config'}
            >
              View Configuration Details
            </Button>
            <br />
            {/* Hide Setup button removed as requested */}
          </div>
        </div>
      )
    }
  ];

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-4xl mx-auto px-6 py-8">
        {/* Setup Toggle (show when models exist) */}
        {hasExistingModels && !showSetup && (
          <motion.div 
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-8"
          >
            <Card className="p-6 bg-surface border-border-secondary">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <div className="w-10 h-10 bg-success-500 rounded-full flex items-center justify-center">
                    <CheckCircleIcon className="w-6 h-6 text-white" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-text-primary">Setup Complete</h3>
                    <p className="text-sm text-text-secondary">
                      ClaraCore is configured and ready to use
                    </p>
                  </div>
                </div>
                <Button
                  variant="outline"
                  onClick={() => setShowSetup(true)}
                  icon={<SettingsIcon className="w-4 h-4" />}
                >
                  Reconfigure Setup
                </Button>
              </div>
            </Card>
          </motion.div>
        )}

        {/* Main Setup Interface */}
        {showSetup && (
          <>
            {/* Progress Bar */}
            <div className="mb-8">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-lg font-medium text-text-secondary">
              Setup Progress
            </h1>
            <span className="text-sm text-text-secondary">
              Step {currentStep + 1} of {steps.length}
            </span>
          </div>
          <div className="w-full bg-surface-secondary rounded-full h-2">
            <motion.div
              className="bg-gradient-to-r from-brand-500 to-brand-600 h-2 rounded-full"
              initial={{ width: 0 }}
              animate={{ width: `${((currentStep + 1) / steps.length) * 100}%` }}
              transition={{ duration: 0.5 }}
            />
          </div>
        </div>

        {/* Notification */}
        <AnimatePresence>
          {notification && (
            <motion.div
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="mb-6"
            >
              <Card className={`border-l-4 ${
                notification.type === 'success' ? 'border-l-success-500 bg-success-50 dark:bg-success-900/20' :
                notification.type === 'error' ? 'border-l-error-500 bg-error-50 dark:bg-error-900/20' :
                'border-l-info-500 bg-info-50 dark:bg-info-900/20'
              }`}>
                <CardContent className="flex items-center space-x-3">
                  {notification.type === 'success' ? <CheckCircleIcon className="w-5 h-5 text-success-500" /> :
                   notification.type === 'error' ? <AlertTriangleIcon className="w-5 h-5 text-error-500" /> :
                   <SettingsIcon className="w-5 h-5 text-info-500" />}
                  <span className={`${
                    notification.type === 'success' ? 'text-success-700 dark:text-success-200' :
                    notification.type === 'error' ? 'text-error-700 dark:text-error-200' :
                    'text-info-700 dark:text-info-200'
                  }`}>
                    {notification.message}
                  </span>
                </CardContent>
              </Card>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Main Content */}
        <AnimatePresence mode="wait">
          <motion.div
            key={currentStep}
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={{ duration: 0.3 }}
          >
            <Card variant="elevated" className="p-8">
              <div className="text-center mb-8">
                <h1 className="text-3xl font-bold text-text-primary mb-2">
                  {steps[currentStep].title}
                </h1>
                <p className="text-lg text-text-secondary">
                  {steps[currentStep].description}
                </p>
              </div>
              
              {steps[currentStep].component}
            </Card>
          </motion.div>
        </AnimatePresence>
          </>
        )}
      </div>
    </div>
  );
};

export default OnboardConfig;
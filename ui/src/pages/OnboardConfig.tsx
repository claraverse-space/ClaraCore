import React, { useState } from 'react';
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
  MemoryStickIcon
} from 'lucide-react';
import { Card, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

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
  gpuType: 'nvidia' | 'amd' | 'intel' | 'none';
  vramGB: number;
  ramGB: number;
  backend: 'cuda' | 'rocm' | 'vulkan' | 'cpu';
  preferredContext: number;
  throughputFirst: boolean;
}

const OnboardConfig: React.FC = () => {
  const [currentStep, setCurrentStep] = useState(0);
  const [folderPath, setFolderPath] = useState('');
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
  const [isScanning, setIsScanning] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [notification, setNotification] = useState<{type: 'success' | 'error' | 'info', message: string} | null>(null);

  const showNotification = (type: 'success' | 'error' | 'info', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 5000);
  };



  const scanModelFolder = async () => {
    if (!folderPath.trim()) {
      showNotification('error', 'Please enter a folder path');
      return;
    }

    setIsScanning(true);
    try {
      const response = await fetch('/api/config/scan-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ folderPath: folderPath, recursive: true }),
      });

      if (response.ok) {
        const data = await response.json();
        setScanResults(data.models || []);
        if (data.models && data.models.length > 0) {
          showNotification('success', `Found ${data.models.length} GGUF models!`);
          setCurrentStep(2); // Move to model selection step
        } else {
          showNotification('error', 'No GGUF models found in this folder');
        }
      } else {
        showNotification('error', 'Failed to scan folder');
      }
    } catch (error) {
      showNotification('error', 'Scan error: ' + error);
    } finally {
      setIsScanning(false);
    }
  };

  const generateSmartConfig = async () => {
    setIsGenerating(true);
    try {
      showNotification('info', '🚀 Generating your personalized SMART configuration...');
      
      const response = await fetch('/api/config/generate-all', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          folderPath: folderPath,
          options: {
            enableJinja: true,
            throughputFirst: systemConfig.throughputFirst,
            minContext: Math.min(16384, systemConfig.preferredContext),
            preferredContext: systemConfig.preferredContext,
            forceBackend: systemConfig.backend,
            forceVRAM: systemConfig.vramGB,
            forceRAM: systemConfig.ramGB,
          }
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to generate configuration');
      }

      await response.json();
      showNotification('success', '🎉 Configuration generated successfully!');
      setCurrentStep(4); // Move to completion step
      
    } catch (error) {
      showNotification('error', 'Error generating configuration: ' + error);
    } finally {
      setIsGenerating(false);
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
      title: "Step 1: Where are your models? 📁",
      description: "Point us to the folder containing your GGUF model files",
      component: (
        <div className="py-6">
          <div className="mb-6">
            <label className="block text-sm font-medium text-text-secondary mb-3">
              Model Folder Path
            </label>
            <Input
              value={folderPath}
              onChange={(e) => setFolderPath(e.target.value)}
              placeholder="C:\models\llama-models"
              className="text-lg"
            />
            <p className="text-sm text-text-tertiary mt-2">
              💡 This folder will be scanned recursively for .gguf files
            </p>
          </div>
          
          <div className="flex space-x-4">
            <Button
              onClick={scanModelFolder}
              loading={isScanning}
              icon={<RefreshCwIcon size={16} />}
              disabled={!folderPath.trim()}
            >
              {isScanning ? 'Scanning...' : 'Scan Folder'}
            </Button>
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
                <p className="text-sm text-text-tertiary text-center py-2">
                  ... and {scanResults.length - 8} more models
                </p>
              )}
            </div>
          </div>
          
          <div className="flex space-x-4">
            <Button
              onClick={() => setCurrentStep(3)}
              icon={<ArrowRightIcon size={16} />}
            >
              Configure These Models
            </Button>
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
      title: "Step 2: Tell us about your system 🖥️",
      description: "We'll optimize the configuration based on your hardware",
      component: (
        <div className="py-6 space-y-6">
          {/* GPU Section */}
          <div>
            <h3 className="font-semibold text-text-primary mb-4 flex items-center">
              <MonitorIcon className="w-5 h-5 mr-2 text-brand-500" />
              Graphics Card (GPU)
            </h3>
            <div className="grid grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Do you have a dedicated GPU?
                </label>
                <div className="flex space-x-4">
                  <label className="flex items-center">
                    <input
                      type="radio"
                      checked={systemConfig.hasGPU}
                      onChange={() => setSystemConfig(prev => ({ ...prev, hasGPU: true, backend: 'cuda' }))}
                      className="mr-2"
                    />
                    Yes
                  </label>
                  <label className="flex items-center">
                    <input
                      type="radio"
                      checked={!systemConfig.hasGPU}
                      onChange={() => setSystemConfig(prev => ({ ...prev, hasGPU: false, backend: 'cpu' }))}
                      className="mr-2"
                    />
                    No (CPU only)
                  </label>
                </div>
              </div>
              
              {systemConfig.hasGPU && (
                <div>
                  <label className="block text-sm font-medium text-text-secondary mb-2">
                    GPU Type
                  </label>
                  <select
                    value={systemConfig.gpuType}
                    onChange={(e) => {
                      const gpuType = e.target.value as 'nvidia' | 'amd' | 'intel';
                      const backend = gpuType === 'nvidia' ? 'cuda' : gpuType === 'amd' ? 'rocm' : 'vulkan';
                      setSystemConfig(prev => ({ ...prev, gpuType, backend }));
                    }}
                    className="w-full p-2 border border-border-secondary rounded-lg bg-background"
                  >
                    <option value="nvidia">NVIDIA (RTX, GTX)</option>
                    <option value="amd">AMD (RX, Radeon)</option>
                    <option value="intel">Intel (Arc, Iris)</option>
                  </select>
                </div>
              )}
            </div>
            
            {systemConfig.hasGPU && (
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  GPU VRAM (GB)
                </label>
                <Input
                  type="number"
                  value={systemConfig.vramGB}
                  onChange={(e) => setSystemConfig(prev => ({ ...prev, vramGB: parseInt(e.target.value) || 0 }))}
                  placeholder="12"
                  min="4"
                  max="128"
                />
                <p className="text-xs text-text-tertiary mt-1">
                  💡 Check GPU-Z or Task Manager for your VRAM amount
                </p>
              </div>
            )}
          </div>

          {/* RAM Section */}
          <div>
            <h3 className="font-semibold text-text-primary mb-4 flex items-center">
              <MemoryStickIcon className="w-5 h-5 mr-2 text-brand-500" />
              System Memory (RAM)
            </h3>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Total RAM (GB)
                </label>
                <Input
                  type="number"
                  value={systemConfig.ramGB}
                  onChange={(e) => setSystemConfig(prev => ({ ...prev, ramGB: parseInt(e.target.value) || 0 }))}
                  placeholder="32"
                  min="8"
                  max="256"
                />
              </div>
              
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-2">
                  Performance Priority
                </label>
                <select
                  value={systemConfig.throughputFirst ? 'speed' : 'quality'}
                  onChange={(e) => setSystemConfig(prev => ({ ...prev, throughputFirst: e.target.value === 'speed' }))}
                  className="w-full p-2 border border-border-secondary rounded-lg bg-background"
                >
                  <option value="speed">Speed (Higher throughput)</option>
                  <option value="quality">Quality (Larger context)</option>
                </select>
              </div>
            </div>
          </div>

          {/* Advanced Options */}
          <div>
            <h3 className="font-semibold text-text-primary mb-4 flex items-center">
              <SettingsIcon className="w-5 h-5 mr-2 text-brand-500" />
              Advanced Options
            </h3>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-2">
                Preferred Context Size
              </label>
              <select
                value={systemConfig.preferredContext}
                onChange={(e) => setSystemConfig(prev => ({ ...prev, preferredContext: parseInt(e.target.value) }))}
                className="w-full p-2 border border-border-secondary rounded-lg bg-background"
              >
                <option value={8192}>8K (Fast, basic tasks)</option>
                <option value={16384}>16K (Balanced)</option>
                <option value={32768}>32K (Recommended)</option>
                <option value={65536}>64K (Large documents)</option>
                <option value={131072}>128K (Maximum, requires lots of VRAM)</option>
              </select>
            </div>
          </div>
          
          <div className="flex space-x-4">
            <Button
              onClick={generateSmartConfig}
              loading={isGenerating}
              icon={<WandIcon size={16} />}
              disabled={isGenerating}
            >
              {isGenerating ? 'Generating Configuration...' : 'Generate Smart Configuration ✨'}
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
              onClick={() => window.location.href = '/'}
              size="lg"
              icon={<ZapIcon size={20} />}
            >
              Start Using ClaraCore
            </Button>
            <br />
            <Button 
              variant="outline"
              onClick={() => window.location.href = '/config'}
            >
              View Configuration Details
            </Button>
          </div>
        </div>
      )
    }
  ];

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-4xl mx-auto px-6 py-8">
        {/* Progress Bar */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-lg font-medium text-text-secondary">
              Setup Progress
            </h1>
            <span className="text-sm text-text-tertiary">
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
      </div>
    </div>
  );
};

export default OnboardConfig;
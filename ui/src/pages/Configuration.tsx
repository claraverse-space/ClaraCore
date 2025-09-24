import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { useNavigate } from 'react-router-dom';
import { 
  SettingsIcon, 
  SaveIcon, 
  RefreshCwIcon, 
  AlertTriangleIcon, 
  CheckCircleIcon,
  DatabaseIcon,
  FileIcon,
  SlidersIcon,
  CpuIcon,
  ZapIcon,
  LayersIcon,
  MemoryStickIcon
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';

interface ConfiguredModel {
  id: string;
  name: string;
  description: string;
  cmd: string;
  proxy: string;
  env: string[];
  contextSize: number;
  layers: number;
  cacheType: string;
  batchSize: number;
  filePath?: string;
}

interface ModelSettings {
  contextSize: number;
  layers: number;
  cacheType: 'q4_0' | 'q4_1' | 'q8_0' | 'f16';
  batchSize: number;
}

const Configuration: React.FC = () => {
  const navigate = useNavigate();
  const [models, setModels] = useState<ConfiguredModel[]>([]);
  const [selectedModel, setSelectedModel] = useState<ConfiguredModel | null>(null);
  const [modelSettings, setModelSettings] = useState<ModelSettings>({
    contextSize: 4096,
    layers: 999,
    cacheType: 'q4_0',
    batchSize: 512,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [notification, setNotification] = useState<{type: 'success' | 'error' | 'info', message: string} | null>(null);

  useEffect(() => {
    loadConfiguration();
  }, []);

  const showNotification = (type: 'success' | 'error' | 'info', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 4000);
  };

  const loadConfiguration = async () => {
    setIsLoading(true);
    try {
      const response = await fetch('/api/config');
      if (response.ok) {
        const data = await response.json();
        
        // Check if config has models
        if (!data.config.models || Object.keys(data.config.models).length === 0) {
          // No models configured, redirect to setup
          navigate('/setup');
          return;
        }

        // Parse models from config (API returns expanded Cmd, not original cmd)
        const configuredModels: ConfiguredModel[] = Object.entries(data.config.models).map(([id, model]: [string, any]) => {
          // The API returns 'Cmd' (capital C) which is the expanded command after macro substitution
          const cmd = model.Cmd || model.cmd || '';
          const contextMatch = cmd.match(/--ctx-size\s+(\d+)/);
          const layersMatch = cmd.match(/-ngl\s+(\d+)/);
          // Look for both cache-type-k and cache-type-v, prioritize the first match
          const cacheKMatch = cmd.match(/--cache-type-k\s+(\w+)/);
          const cacheVMatch = cmd.match(/--cache-type-v\s+(\w+)/);
          const cacheType = cacheKMatch ? cacheKMatch[1] : (cacheVMatch ? cacheVMatch[1] : 'q4_0');
          const batchMatch = cmd.match(/--batch-size\s+(\d+)/);

          return {
            id,
            name: model.Name || model.name || id,
            description: model.Description || model.description || 'Configured model',
            cmd: cmd,
            proxy: model.Proxy || model.proxy || '',
            env: model.Env || model.env || [],
            contextSize: contextMatch ? parseInt(contextMatch[1]) : 4096,
            layers: layersMatch ? parseInt(layersMatch[1]) : 999,
            cacheType: cacheType,
            batchSize: batchMatch ? parseInt(batchMatch[1]) : 512,
            filePath: extractModelPath(cmd),
          };
        });

        setModels(configuredModels);
        
        // Select first model by default
        if (configuredModels.length > 0) {
          selectModel(configuredModels[0]);
        }
      } else {
        // API error, likely no config file
        navigate('/setup');
      }
    } catch (error) {
      console.error('Failed to load configuration:', error);
      navigate('/setup');
    } finally {
      setIsLoading(false);
    }
  };

  const extractModelPath = (cmd: string): string => {
    // Handle multiline format - look for --model followed by path on next line or same line
    const match = cmd.match(/--model\s+([^\s\n\r]+)/);
    if (match) {
      return match[1].trim();
    }
    return '';
  };

  const selectModel = (model: ConfiguredModel) => {
    setSelectedModel(model);
    setModelSettings({
      contextSize: model.contextSize,
      layers: model.layers,
      cacheType: model.cacheType as 'q4_0' | 'q4_1' | 'q8_0' | 'f16',
      batchSize: model.batchSize,
    });
  };

  const updateModelSettings = (newSettings: Partial<ModelSettings>) => {
    setModelSettings(prev => ({ ...prev, ...newSettings }));
  };

  const saveModelSettings = async () => {
    if (!selectedModel) return;

    setIsSaving(true);
    try {
      // Use the new selective update API that preserves YAML structure
      const response = await fetch(`/api/config/model/${selectedModel.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          contextSize: modelSettings.contextSize,
          layers: modelSettings.layers,
          cacheType: modelSettings.cacheType,
          batchSize: modelSettings.batchSize,
        }),
      });

      if (response.ok) {
        // Update the local model state to reflect changes
        const updatedModel = {
          ...selectedModel,
          contextSize: modelSettings.contextSize,
          layers: modelSettings.layers,
          cacheType: modelSettings.cacheType,
          batchSize: modelSettings.batchSize,
        };

        setModels(prev => prev.map(m => m.id === selectedModel.id ? updatedModel : m));
        setSelectedModel(updatedModel);

        showNotification('success', 'Model settings saved successfully! YAML structure preserved.');
      } else {
        const error = await response.json();
        throw new Error(error.error || 'Failed to save configuration');
      }
    } catch (error) {
      showNotification('error', 'Failed to save model settings: ' + error);
    } finally {
      setIsSaving(false);
    }
  };



  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <RefreshCwIcon className="w-8 h-8 animate-spin mx-auto mb-4 text-brand-500" />
          <p className="text-text-secondary">Loading configuration...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center justify-between"
      >
        <div className="flex items-center space-x-4">
          <div className="p-3 bg-gradient-to-br from-brand-500 to-brand-600 rounded-xl">
            <SettingsIcon className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-text-primary">Model Configuration</h1>
            <p className="text-text-secondary">
              Fine-tune your model settings: context size, layers, and cache types
            </p>
          </div>
        </div>
        
        <div className="flex space-x-3">
          <Button
            variant="outline"
            onClick={loadConfiguration}
            icon={<RefreshCwIcon size={16} />}
          >
            Reload
          </Button>
          <Button
            onClick={() => navigate('/setup')}
            variant="secondary"
          >
            Add More Models
          </Button>
        </div>
      </motion.div>

      {/* Notification */}
      {notification && (
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -20 }}
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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Sidebar - Model List */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          className="space-y-4"
        >
          <Card>
            <CardHeader>
              <div className="flex items-center space-x-2">
                <DatabaseIcon className="w-5 h-5 text-brand-500" />
                <CardTitle>Configured Models</CardTitle>
              </div>
              <CardDescription>
                {models.length} model{models.length !== 1 ? 's' : ''} ready to configure
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {models.map((model, index) => (
                  <motion.div
                    key={model.id}
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: index * 0.05 }}
                    onClick={() => selectModel(model)}
                    className={`p-3 rounded-lg border cursor-pointer transition-all hover:border-brand-500 ${
                      selectedModel?.id === model.id 
                        ? 'border-brand-500 bg-brand-50 dark:bg-brand-900/30' 
                        : 'border-border-secondary hover:bg-surface-secondary'
                    }`}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center space-x-2 mb-1">
                          <FileIcon className={`w-4 h-4 flex-shrink-0 ${
                            selectedModel?.id === model.id 
                              ? 'text-brand-600 dark:text-brand-400' 
                              : 'text-brand-500'
                          }`} />
                          <h4 className={`font-medium text-sm truncate ${
                            selectedModel?.id === model.id 
                              ? 'text-brand-900 dark:text-brand-100' 
                              : 'text-text-primary'
                          }`}>
                            {model.name}
                          </h4>
                        </div>
                        <p className={`text-xs truncate mb-1 ${
                          selectedModel?.id === model.id 
                            ? 'text-brand-700 dark:text-brand-300' 
                            : 'text-text-tertiary'
                        }`}>
                          ID: {model.id}
                        </p>
                        <div className={`flex items-center space-x-4 text-xs ${
                          selectedModel?.id === model.id 
                            ? 'text-brand-800 dark:text-brand-200' 
                            : 'text-text-secondary'
                        }`}>
                          <span>CTX: {model.contextSize}</span>
                          <span>Layers: {model.layers}</span>
                          <span>Cache: {model.cacheType}</span>
                        </div>
                      </div>
                      {selectedModel?.id === model.id && (
                        <div className="w-2 h-2 0 rounded-full flex-shrink-0 mt-1"></div>
                      )}
                    </div>
                  </motion.div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Right Panel - Model Settings */}
        <motion.div
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-2"
        >
          {selectedModel ? (
            <Card variant="elevated">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <SlidersIcon className="w-6 h-6 text-brand-500" />
                    <div>
                      <CardTitle>{selectedModel.name}</CardTitle>
                      <CardDescription>{selectedModel.description}</CardDescription>
                    </div>
                  </div>
                  <Button
                    onClick={saveModelSettings}
                    loading={isSaving}
                    icon={<SaveIcon size={16} />}
                  >
                    Save Settings
                  </Button>
                </div>
              </CardHeader>
              
              <CardContent className="space-y-6">
                {/* Context Size Slider */}
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center space-x-2">
                      <MemoryStickIcon className="w-5 h-5 text-brand-500" />
                      <label className="font-medium text-text-primary">Context Size</label>
                    </div>
                    <span className="text-sm font-medium text-brand-500  dark:bg-brand-900/20 px-2 py-1 rounded">
                      {modelSettings.contextSize.toLocaleString()} tokens
                    </span>
                  </div>
                  <input
                    type="range"
                    min="1024"
                    max="131072"
                    step="1024"
                    value={modelSettings.contextSize}
                    onChange={(e) => updateModelSettings({ contextSize: parseInt(e.target.value) })}
                    className="w-full h-2 bg-surface-secondary rounded-lg appearance-none cursor-pointer slider"
                  />
                  <div className="flex justify-between text-xs text-text-tertiary mt-1">
                    <span>1K</span>
                    <span>32K</span>
                    <span>128K</span>
                  </div>
                </div>

                {/* GPU Layers Slider */}
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center space-x-2">
                      <LayersIcon className="w-5 h-5 text-brand-500" />
                      <label className="font-medium text-text-primary">GPU Layers</label>
                    </div>
                    <span className="text-sm font-medium text-brand-500  dark:bg-brand-900/20 px-2 py-1 rounded">
                      {modelSettings.layers === 999 ? 'All' : modelSettings.layers} layers
                    </span>
                  </div>
                  <input
                    type="range"
                    min="0"
                    max="999"
                    step="1"
                    value={modelSettings.layers}
                    onChange={(e) => updateModelSettings({ layers: parseInt(e.target.value) })}
                    className="w-full h-2 bg-surface-secondary rounded-lg appearance-none cursor-pointer slider"
                  />
                  <div className="flex justify-between text-xs text-text-tertiary mt-1">
                    <span>CPU Only</span>
                    <span>Mixed</span>
                    <span>All GPU</span>
                  </div>
                </div>

                {/* Cache Type Selection */}
                <div>
                  <div className="flex items-center space-x-2 mb-3">
                    <CpuIcon className="w-5 h-5 text-brand-500" />
                    <label className="font-medium text-text-primary">Cache Type</label>
                  </div>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                    {['q4_0', 'q4_1', 'q8_0', 'f16'].map((type) => (
                      <button
                        key={type}
                        onClick={() => updateModelSettings({ cacheType: type as any })}
                        className={`p-3 rounded-lg border text-center transition-all ${
                          modelSettings.cacheType === type
                            ? 'border-brand-500  dark:bg-brand-900/20 text-brand-700 dark:text-brand-200'
                            : 'border-border-secondary hover:border-border-accent text-text-secondary hover:text-text-primary'
                        }`}
                      >
                        <div className="font-medium">{type.toUpperCase()}</div>
                        <div className="text-xs mt-1">
                          {type === 'f16' ? 'Highest Quality' : 
                           type === 'q8_0' ? 'High Quality' :
                           type === 'q4_1' ? 'Balanced' : 'Fastest'}
                        </div>
                      </button>
                    ))}
                  </div>
                </div>

                {/* Batch Size Slider */}
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center space-x-2">
                      <ZapIcon className="w-5 h-5 text-brand-500" />
                      <label className="font-medium text-text-primary">Batch Size</label>
                    </div>
                    <span className="text-sm font-medium text-brand-500  dark:bg-brand-900/20 px-2 py-1 rounded">
                      {modelSettings.batchSize}
                    </span>
                  </div>
                  <input
                    type="range"
                    min="128"
                    max="4096"
                    step="128"
                    value={modelSettings.batchSize}
                    onChange={(e) => updateModelSettings({ batchSize: parseInt(e.target.value) })}
                    className="w-full h-2 bg-surface-secondary rounded-lg appearance-none cursor-pointer slider"
                  />
                  <div className="flex justify-between text-xs text-text-tertiary mt-1">
                    <span>128</span>
                    <span>1024</span>
                    <span>4096</span>
                  </div>
                </div>

                {/* Model Info */}
                <div className="bg-surface-secondary rounded-lg p-4">
                  <h4 className="font-medium text-text-primary mb-3">Model Information</h4>
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div>
                      <span className="text-text-secondary">Model ID:</span>
                      <p className="font-medium text-text-primary">{selectedModel.id}</p>
                    </div>
                    <div>
                      <span className="text-text-secondary">File Path:</span>
                      <p className="font-mono text-xs text-text-primary truncate">{selectedModel.filePath}</p>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          ) : (
            <Card variant="elevated">
              <CardContent className="flex flex-col items-center justify-center py-12">
                <SlidersIcon className="w-16 h-16 text-text-tertiary mb-4" />
                <h3 className="text-lg font-medium text-text-primary mb-2">Select a Model</h3>
                <p className="text-text-secondary text-center">
                  Choose a model from the list to configure its settings
                </p>
              </CardContent>
            </Card>
          )}
        </motion.div>
      </div>
    </div>
  );
};

export default Configuration;
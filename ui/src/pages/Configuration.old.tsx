import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { 
  SettingsIcon, 
  SaveIcon, 
  RefreshCwIcon, 
  AlertTriangleIcon, 
  CheckCircleIcon,
  DatabaseIcon,
  TrashIcon,
  WandIcon,
  FolderIcon,
  FileIcon,
  ZapIcon,
  HardDriveIcon,
  ArrowRightIcon
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';



interface ModelScanResult {
  modelId: string;
  filename: string;
  path: string;
  relativePath: string;
  size: number;
  modTime: string;
}

interface ConfiguredModel {
  id: string;
  name: string;
  description: string;
  cmd: string;
  proxy: string;
  env: string[];
  filePath: string;
  size: number;
}

const Configuration: React.FC = () => {
  const [models, setModels] = useState<ConfiguredModel[]>([]);
  const [scanResults, setScanResults] = useState<ModelScanResult[]>([]);
  const [folderPath, setFolderPath] = useState('C:\\BackUP\\llama-modelsss');
  const [isScanning, setIsScanning] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [notification, setNotification] = useState<{type: 'success' | 'error' | 'info', message: string} | null>(null);

  useEffect(() => {
    loadExistingModels();
    autoDetectDownloadedModels();
  }, []);

  const autoDetectDownloadedModels = async () => {
    try {
      // Check common download directories
      const commonPaths = [
        'C:\\BackUP\\llama-modelsss',
        './models',
        './downloads',
        '%USERPROFILE%\\Downloads\\models',
      ];

      for (const path of commonPaths) {
        try {
          const response = await fetch('/api/config/scan-folder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ folderPath: path, recursive: true }),
          });

          if (response.ok) {
            const data = await response.json();
            if (data.models && data.models.length > 0) {
              setFolderPath(path);
              showNotification('info', `🔍 Auto-detected ${data.models.length} models in ${path}`);
              break; // Use first found path
            }
          }
        } catch (error) {
          // Silently continue to next path
        }
      }
    } catch (error) {
      // Silently fail - this is just auto-detection
    }
  };

  const loadExistingModels = async () => {
    try {
      const response = await fetch('/api/config');
      if (response.ok) {
        const data = await response.json();
        const configuredModels: ConfiguredModel[] = Object.entries(data.config.models).map(([id, model]: [string, any]) => ({
          id,
          name: model.name || id,
          description: model.description || 'Configured model',
          cmd: model.cmd || '',
          proxy: model.proxy || '',
          env: model.env || [],
          filePath: extractModelPath(model.cmd),
          size: 0, // We'll get this from file system if needed
        }));
        setModels(configuredModels);
      }
    } catch (error) {
      showNotification('error', 'Failed to load existing models');
    }
  };

  const extractModelPath = (cmd: string): string => {
    const match = cmd.match(/--model\s+([^\s]+)/);
    return match ? match[1] : '';
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
        body: JSON.stringify({ 
          folderPath: folderPath.trim(), 
          recursive: true 
        }),
      });

      if (response.ok) {
        const result = await response.json();
        setScanResults(result.models);
        showNotification('success', `Found ${result.count} GGUF models ready to configure!`);
      } else {
        const error = await response.json();
        showNotification('error', 'Scan failed: ' + error.error);
      }
    } catch (error) {
      showNotification('error', 'Scan error: ' + error);
    } finally {
      setIsScanning(false);
    }
  };

  const autoConfigureModel = async (model: ModelScanResult) => {
    try {
      const response = await fetch('/api/config/add-model', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          modelId: model.modelId,
          name: model.filename.replace('.gguf', ''),
          description: `Auto-configured from ${model.relativePath}`,
          filePath: model.path,
          auto: true,
        }),
      });

      if (response.ok) {
        const result = await response.json();
        
        // Add to our models list
        const newModel: ConfiguredModel = {
          id: model.modelId,
          name: result.config.name,
          description: result.config.description,
          cmd: result.config.cmd,
          proxy: result.config.proxy,
          env: result.config.env,
          filePath: model.path,
          size: model.size,
        };
        
        setModels(prev => [...prev, newModel]);
        
        // Remove from scan results
        setScanResults(prev => prev.filter(m => m.modelId !== model.modelId));
        
        showNotification('success', `${model.filename} configured automatically!`);
      } else {
        const error = await response.json();
        showNotification('error', 'Failed to configure: ' + error.error);
      }
    } catch (error) {
      showNotification('error', 'Error configuring model: ' + error);
    }
  };

  const configureAllModels = async () => {
    setIsSaving(true);
    try {
      showNotification('info', '🚀 Generating SMART configuration (same as command-line)...');
      
      // Use the SAME autosetup logic as command-line
      const response = await fetch('/api/config/generate-all', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          folderPath: folderPath,
          options: {
            enableJinja: true,
            throughputFirst: true,
            minContext: 16384,
            preferredContext: 32768,
          }
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to generate configuration');
      }

      const result = await response.json();
      showNotification('success', result.status + ' ✨');
      
      // Reload the page to show the new configuration
      setTimeout(() => {
        window.location.reload();
      }, 2000);
      
    } catch (error) {
      showNotification('error', 'Error generating SMART configuration: ' + error);
    } finally {
      setIsSaving(false);
    }
  };

  const saveConfiguration = async () => {
    setIsSaving(true);
    try {
      // Generate YAML from our models
      const yamlConfig = generateYAMLConfig(models);
      
      const response = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ yaml: yamlConfig }),
      });

      if (response.ok) {
        const result = await response.json();
        showNotification('success', `Configuration saved! Backup: ${result.backup}`);
      } else {
        const error = await response.json();
        showNotification('error', 'Failed to save: ' + error.error);
      }
    } catch (error) {
      showNotification('error', 'Save error: ' + error);
    } finally {
      setIsSaving(false);
    }
  };

  const generateYAMLConfig = (models: ConfiguredModel[]): string => {
    // This is a simplified YAML generation - in a real implementation you'd want proper YAML formatting
    let yaml = `# Auto-generated ClaraCore configuration
healthCheckTimeout: 300
logLevel: info
startPort: 5800

macros:
  "llama-server-base": >
    binaries\\llama-server\\llama-server.exe
    --host 127.0.0.1
    --port \${PORT}
    --metrics
    --flash-attn auto
    --no-warmup
    --batch-size 2048
    --ubatch-size 512

models:
`;

    models.forEach(model => {
      yaml += `  "${model.id}":
    name: "${model.name}"
    description: "${model.description}"
    cmd: |
${model.cmd.split('\n').map(line => '      ' + line.trim()).join('\n')}
    proxy: "${model.proxy}"
    env:
${model.env.map(env => `      - "${env}"`).join('\n')}

`;
    });

    yaml += `
groups:
  "large-models":
    swap: true
    exclusive: true
    startPort: 5800
    members:
${models.map(model => `      - "${model.id}"`).join('\n')}
`;

    return yaml;
  };

  const removeModel = (modelId: string) => {
    setModels(prev => prev.filter(m => m.id !== modelId));
    showNotification('info', 'Model removed from configuration');
  };

  const showNotification = (type: 'success' | 'error' | 'info', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 5000);
  };



  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

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
            <ZapIcon className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-text-primary">Easy Model Manager</h1>
            <p className="text-text-secondary">
              Scan folders and automatically configure GGUF models - no YAML editing needed!
            </p>
          </div>
        </div>
        
        <div className="flex items-center space-x-4">
          {models.length === 0 && (
            <Button
              onClick={() => window.location.href = '/ui/setup'}
              variant="primary"
              icon={<ZapIcon size={16} />}
              size="lg"
            >
              🚀 First Time Setup
            </Button>
          )}
          
          {models.length > 0 && (
            <Button
              onClick={saveConfiguration}
              loading={isSaving}
              icon={<SaveIcon size={16} />}
              size="lg"
            >
              Save All Models
            </Button>
          )}
        </div>
      </motion.div>

      {/* First Time User Banner */}
      {models.length === 0 && scanResults.length === 0 && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <Card className="border-l-4 border-l-brand-500  dark:bg-brand-900/20">
            <CardContent className="flex items-center justify-between p-6">
              <div className="flex items-center space-x-4">
                <div className="p-3 bg-brand-500 rounded-full">
                  <ZapIcon className="w-6 h-6 text-white" />
                </div>
                <div>
                  <h3 className="font-semibold text-brand-700 dark:text-brand-200">
                    👋 New to ClaraCore? Let's get you started!
                  </h3>
                  <p className="text-brand-600 dark:text-brand-300 mt-1">
                    Our guided setup will help you configure your models in just a few clicks.
                  </p>
                </div>
              </div>
              <Button
                onClick={() => window.location.href = '/ui/setup'}
                variant="primary"
                icon={<ArrowRightIcon size={16} />}
              >
                Start Setup Guide
              </Button>
            </CardContent>
          </Card>
        </motion.div>
      )}

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
        {/* Model Scanner */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          className="lg:col-span-2"
        >
          <Card variant="elevated">
            <CardHeader>
              <div className="flex items-center space-x-3">
                <FolderIcon className="w-6 h-6 text-brand-500" />
                <div>
                  <CardTitle>1. Scan for Models</CardTitle>
                  <CardDescription>Point to your model folder and we'll find all GGUF files</CardDescription>
                </div>
              </div>
            </CardHeader>
            
            <CardContent>
              <div className="flex space-x-4 mb-6">
                <Input
                  value={folderPath}
                  onChange={(e) => setFolderPath(e.target.value)}
                  placeholder="C:\BackUP\llama-modelsss"
                  className="flex-1"
                />
                <Button
                  onClick={scanModelFolder}
                  loading={isScanning}
                  icon={<RefreshCwIcon size={16} />}
                >
                  {isScanning ? 'Scanning...' : 'Scan Folder'}
                </Button>
              </div>

              {/* Scan Results */}
              {scanResults.length > 0 && (
                <div>
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="font-semibold text-text-primary flex items-center">
                      <span className="bg-brand-500 text-white text-sm px-3 py-1 rounded-full mr-3">
                        {scanResults.length}
                      </span>
                      Found Models Ready to Configure
                    </h3>
                    <Button
                      onClick={configureAllModels}
                      loading={isSaving}
                      icon={<WandIcon size={16} />}
                      variant="primary"
                    >
                      {isSaving ? 'Generating SMART Config...' : 'SMART Configure All ✨'}
                    </Button>
                  </div>
                  
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {scanResults.map((model, index) => (
                      <motion.div
                        key={index}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: index * 0.05 }}
                      >
                        <Card className="hover:border-brand-500 transition-colors">
                          <CardContent className="p-4">
                            <div className="flex items-start justify-between">
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center space-x-2 mb-2">
                                  <FileIcon className="w-4 h-4 text-brand-500 flex-shrink-0" />
                                  <h4 className="font-medium text-text-primary truncate">
                                    {model.filename}
                                  </h4>
                                </div>
                                <p className="text-xs text-text-tertiary truncate mb-1">
                                  {model.relativePath}
                                </p>
                                <p className="text-xs text-text-secondary">
                                  {formatBytes(model.size)}
                                </p>
                              </div>
                              <Button
                                size="sm"
                                onClick={() => autoConfigureModel(model)}
                                icon={<WandIcon size={14} />}
                                className="ml-2"
                              >
                                SMART Config
                              </Button>
                            </div>
                          </CardContent>
                        </Card>
                      </motion.div>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>

        {/* Configured Models Sidebar */}
        <motion.div
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          className="space-y-6"
        >
          {/* Configured Models */}
          <Card>
            <CardHeader>
              <div className="flex items-center space-x-2">
                <DatabaseIcon className="w-5 h-5 text-brand-500" />
                <CardTitle>2. Configured Models</CardTitle>
              </div>
              <CardDescription>
                Models ready to use in ClaraCore
              </CardDescription>
            </CardHeader>
            <CardContent>
              {models.length === 0 ? (
                <div className="text-center py-8 text-text-tertiary">
                  <DatabaseIcon className="w-12 h-12 mx-auto mb-3 opacity-50" />
                  <p>No models configured yet</p>
                  <p className="text-sm">Scan a folder to get started!</p>
                </div>
              ) : (
                <div className="space-y-3 max-h-96 overflow-y-auto">
                  {models.map((model, index) => (
                    <motion.div
                      key={model.id}
                      initial={{ opacity: 0, x: 20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: index * 0.1 }}
                      className="p-3 bg-surface-secondary rounded-lg border border-border-secondary"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1 min-w-0">
                          <h4 className="font-medium text-text-primary text-sm truncate">
                            {model.name}
                          </h4>
                          <p className="text-xs text-text-tertiary truncate">
                            ID: {model.id}
                          </p>
                          <p className="text-xs text-text-secondary mt-1">
                            {formatBytes(model.size)}
                          </p>
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => removeModel(model.id)}
                          icon={<TrashIcon size={12} />}
                          className="p-1 h-6 w-6 text-error-500 hover:text-error-600"
                        />
                      </div>
                    </motion.div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <div className="flex items-center space-x-2">
                <HardDriveIcon className="w-5 h-5 text-brand-500" />
                <CardTitle>Summary</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-text-secondary">Total Models:</span>
                  <span className="font-semibold text-brand-500">
                    {models.length}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-text-secondary">Found Models:</span>
                  <span className="font-medium text-text-primary">
                    {scanResults.length}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-text-secondary">Total Size:</span>
                  <span className="font-medium text-text-primary">
                    {formatBytes(models.reduce((acc, model) => acc + model.size, 0))}
                  </span>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </div>
    </div>
  );
};

export default Configuration;
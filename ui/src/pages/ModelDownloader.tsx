import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  SearchIcon,
  DownloadIcon,
  FilterIcon,
  SettingsIcon,
  KeyIcon,
  DatabaseIcon,
  CpuIcon,
  HardDriveIcon,
  StarIcon,
  ClockIcon,
  ExternalLinkIcon,
  AlertCircleIcon,
  XIcon
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';

// Types
interface HFModel {
  id: string;
  author: string;
  downloads: number;
  likes: number;
  updatedAt: string;
  tags: string[];
  pipeline_tag?: string;
  siblings?: Array<{
    rfilename: string;
    size?: number;
  }>;
}

interface SystemSpecs {
  totalRAM: number;
  availableRAM: number;
  totalVRAM: number;
  availableVRAM: number;
  cpuCores: number;
  gpuName: string;
  diskSpace: number;
}

interface DownloadProgress {
  id: string;
  modelId: string;
  filename: string;
  url: string;
  progress: number;
  speed: number;
  bytesDownloaded: number;
  totalBytes: number;
  estimatedTimeRemaining: number;
  status: 'pending' | 'downloading' | 'paused' | 'completed' | 'failed' | 'cancelled';
  error?: string;
  startTime: string;
  endTime?: string;
  retryCount: number;
}

const ModelDownloaderPage: React.FC = () => {
  // State
  const [searchQuery, setSearchQuery] = useState('');
  const [hfApiKey, setHfApiKey] = useState('');
  const [models, setModels] = useState<HFModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedModel, setSelectedModel] = useState<HFModel | null>(null);
  const [downloads, setDownloads] = useState<DownloadProgress[]>([]);
  const [systemSpecs, setSystemSpecs] = useState<SystemSpecs | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [selectedFilter, setSelectedFilter] = useState<string>('all');

  // Filters
  const filters = [
    { key: 'all', label: 'All Models', count: models.length },
    { key: 'text-generation', label: 'Text Generation', count: models.filter(m => m.pipeline_tag === 'text-generation').length },
    { key: 'conversational', label: 'Chat Models', count: models.filter(m => m.pipeline_tag === 'conversational').length },
    { key: 'code', label: 'Code Models', count: models.filter(m => m.tags?.includes('code')).length },
  ];

  // Filter models
  const filteredModels = models.filter(model => {
    if (selectedFilter === 'all') return true;
    if (selectedFilter === 'code') return model.tags?.includes('code');
    return model.pipeline_tag === selectedFilter;
  });

  // Fetch system specs
  useEffect(() => {
    const fetchSystemSpecs = async () => {
      try {
        const response = await fetch('/api/system/specs');
        if (response.ok) {
          const specs = await response.json();
          setSystemSpecs(specs);
        }
      } catch (error) {
        console.error('Failed to fetch system specs:', error);
      }
    };
    fetchSystemSpecs();
  }, []);

  // Load API key
  useEffect(() => {
    const loadApiKey = async () => {
      try {
        const response = await fetch('/api/settings/hf-api-key');
        if (response.ok) {
          const data = await response.json();
          setHfApiKey(data.apiKey || '');
        }
      } catch (error) {
        console.error('Failed to load API key:', error);
      }
    };
    loadApiKey();
  }, []);

  // Fetch current downloads
  useEffect(() => {
    const fetchDownloads = async () => {
      try {
        const response = await fetch('/api/models/downloads');
        if (response.ok) {
          const downloads = await response.json();
          const downloadArray = Object.values(downloads) as DownloadProgress[];
          setDownloads(downloadArray);
        }
      } catch (error) {
        console.error('Failed to fetch downloads:', error);
      }
    };
    fetchDownloads();
  }, []);

  // Set up real-time download progress updates
  useEffect(() => {
    const eventSource = new EventSource('/api/events');
    
    eventSource.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        if (message.type === 'downloadProgress') {
          const progressData = JSON.parse(message.data);
          
          setDownloads(prev => {
            const updated = [...prev];
            const index = updated.findIndex(d => d.id === progressData.downloadId);
            
            if (index !== -1) {
              const oldStatus = updated[index].status;
              const newStatus = progressData.info?.status;
              
              // Update existing download
              updated[index] = { ...updated[index], ...progressData.info };
              
              // Check if download just completed
              if (oldStatus !== 'completed' && newStatus === 'completed') {
                const completedDownload = updated[index];
                if (completedDownload.filename && completedDownload.filename.toLowerCase().endsWith('.gguf')) {
                  // Construct the full path to the downloaded model
                  const filePath = `./downloads/${completedDownload.filename}`;
                  
                  // Add model to config after a short delay to ensure file is fully written
                  setTimeout(() => {
                    addModelToConfig(filePath);
                  }, 2000);
                }
              }
            } else if (progressData.info) {
              // Add new download
              updated.push(progressData.info);
            }
            
            return updated;
          });
        }
      } catch (error) {
        console.error('Error parsing download progress event:', error);
      }
    };

    return () => {
      eventSource.close();
    };
  }, []);

  // Save API key
  const saveApiKey = async (key: string) => {
    try {
      await fetch('/api/settings/hf-api-key', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ apiKey: key }),
      });
      setHfApiKey(key);
    } catch (error) {
      console.error('Failed to save API key:', error);
    }
  };

  // Search models
  const searchModels = async () => {
    if (!searchQuery.trim()) return;
    
    setLoading(true);
    try {
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      
      if (hfApiKey) {
        headers['Authorization'] = `Bearer ${hfApiKey}`;
      }

      // Automatically add "GGUF" to search if not already present since ClaraCore only supports GGUF
      let enhancedQuery = searchQuery.trim();
      if (!enhancedQuery.toLowerCase().includes('gguf')) {
        enhancedQuery = `${enhancedQuery} GGUF`;
      }

      const response = await fetch(
        `https://huggingface.co/api/models?search=${encodeURIComponent(enhancedQuery)}&limit=50&full=true`,
        { headers }
      );

      if (response.ok) {
        const data = await response.json();
        setModels(data);
      }
    } catch (error) {
      console.error('Error searching models:', error);
    } finally {
      setLoading(false);
    }
  };

  // Start download
  const startDownload = async (model: HFModel, filename: string) => {
    // Validate inputs
    if (!filename || filename === 'undefined') {
      console.error('Invalid filename:', filename);
      alert('Error: Invalid filename. Please try again.');
      return;
    }
    
    console.log('Starting download:', { modelId: model.id, filename });
    
    try {
      // Construct HuggingFace download URL
      const downloadUrl = `https://huggingface.co/${model.id}/resolve/main/${filename}`;
      
      const response = await fetch('/api/models/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: downloadUrl,
          modelId: model.id,
          filename,
          hfApiKey,
        }),
      });

      if (response.ok) {
        const result = await response.json();
        // The UI will be updated via real-time events, so we don't need to manually add to downloads
        console.log('Download started:', result);
      } else {
        console.error('Failed to start download:', await response.text());
      }
    } catch (error) {
      console.error('Failed to start download:', error);
    }
  };

  // Cancel download
  const cancelDownload = async (downloadId: string) => {
    try {
      await fetch('/api/models/download/cancel', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ downloadId }),
      });
      // The UI will be updated via real-time events
      console.log('Download cancelled:', downloadId);
    } catch (error) {
      console.error('Failed to cancel download:', error);
    }
  };

  // Pause download
  const pauseDownload = async (downloadId: string) => {
    try {
      await fetch(`/api/models/downloads/${downloadId}/pause`, {
        method: 'POST',
      });
      console.log('Download paused:', downloadId);
    } catch (error) {
      console.error('Failed to pause download:', error);
    }
  };

  // Resume download
  const resumeDownload = async (downloadId: string) => {
    try {
      await fetch(`/api/models/downloads/${downloadId}/resume`, {
        method: 'POST',
      });
      console.log('Download resumed:', downloadId);
    } catch (error) {
      console.error('Failed to resume download:', error);
    }
  };

  // Add downloaded model to config
  const addModelToConfig = async (filePath: string) => {
    try {
      console.log('Adding model to config:', filePath);
      
      const response = await fetch('/api/config/append-model', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          filePath,
          options: {
            enableJinja: true,
            throughputFirst: true,
            minContext: 16384,
            preferredContext: 32768,
          }
        }),
      });

      if (response.ok) {
        const result = await response.json();
        console.log('Model added to config:', result);
        
        // Show success notification (you might want to add a toast/notification system)
        alert(`✅ Model "${result.modelInfo.name}" has been added to your configuration and is ready to use!`);
      } else {
        const error = await response.text();
        console.error('Failed to add model to config:', error);
        alert(`❌ Failed to add model to configuration: ${error}`);
      }
    } catch (error) {
      console.error('Failed to add model to config:', error);
      alert(`❌ Failed to add model to configuration: ${error}`);
    }
  };

  // Format file size
  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-7xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 bg-gradient-to-br from-brand-500 to-brand-600 rounded-xl flex items-center justify-center">
              <DatabaseIcon className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-text-primary">Model Discovery</h1>
              <p className="text-text-secondary">Browse and download AI models from HuggingFace</p>
            </div>
          </div>
          
          <Button
            variant="ghost"
            onClick={() => setShowSettings(true)}
            className="flex items-center gap-2"
          >
            <SettingsIcon className="w-4 h-4" />
            Settings
          </Button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* Sidebar */}
          <div className="lg:col-span-1 space-y-6">
            {/* System Specs */}
            {systemSpecs && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <CpuIcon className="w-4 h-4" />
                    System Info
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex justify-between text-sm">
                    <span className="text-text-secondary">RAM:</span>
                    <span className="text-text-primary">{formatFileSize(systemSpecs.totalRAM)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-text-secondary">VRAM:</span>
                    <span className="text-text-primary">{formatFileSize(systemSpecs.totalVRAM)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-text-secondary">GPU:</span>
                    <span className="text-text-primary text-xs">{systemSpecs.gpuName}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-text-secondary">Storage:</span>
                    <span className="text-text-primary">{formatFileSize(systemSpecs.diskSpace)}</span>
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Filters */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <FilterIcon className="w-4 h-4" />
                  Filters
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {filters.map((filter) => (
                    <button
                      key={filter.key}
                      onClick={() => setSelectedFilter(filter.key)}
                      className={`w-full flex items-center justify-between p-3 rounded-lg text-left transition-colors ${
                        selectedFilter === filter.key
                          ? 'text-brand-700 border border-brand-200 dark:bg-brand-900/20 dark:text-brand-300'
                          : 'text-text-secondary hover:text-text-primary hover:bg-surface-secondary'
                      }`}
                    >
                      <span className="text-sm font-medium">{filter.label}</span>
                      <span className="text-xs bg-surface-secondary px-2 py-1 rounded-full">
                        {filter.count}
                      </span>
                    </button>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Active Downloads */}
            {downloads.length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <DownloadIcon className="w-4 h-4" />
                    Downloads ({downloads.length})
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    {downloads.map((download) => (
                      <div key={`${download.modelId}-${download.filename}`} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium text-text-primary truncate">
                            {download.filename}
                          </span>
                          <div className="flex items-center gap-2">
                            {download.status === 'completed' && download.filename.toLowerCase().endsWith('.gguf') && (
                              <Button
                                variant="primary"
                                size="sm"
                                onClick={() => addModelToConfig(`./downloads/${download.filename}`)}
                                className="text-xs"
                              >
                                📋 Add to Config
                              </Button>
                            )}
                            {download.status === 'downloading' && (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => pauseDownload(download.id)}
                              >
                                ⏸️
                              </Button>
                            )}
                            {download.status === 'paused' && (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => resumeDownload(download.id)}
                              >
                                ▶️
                              </Button>
                            )}
                            {download.status !== 'completed' && (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => cancelDownload(download.id)}
                              >
                                <XIcon className="w-3 h-3" />
                              </Button>
                            )}
                          </div>
                        </div>
                        <div className="w-full bg-surface-secondary rounded-full h-2">
                          <div
                            className={`h-2 rounded-full transition-all ${
                              download.progress < 0 || isNaN(download.progress) 
                                ? 'bg-brand-500 animate-pulse' 
                                : 'bg-brand-500'
                            }`}
                            style={{ 
                              width: download.progress < 0 || isNaN(download.progress) 
                                ? '100%' 
                                : `${Math.min(100, Math.max(0, download.progress))}%` 
                            }}
                          />
                        </div>
                        <div className="flex justify-between text-xs text-text-secondary">
                          <span>
                            {download.progress < 0 || isNaN(download.progress) 
                              ? 'Downloading...' 
                              : `${download.progress.toFixed(1)}%`} • {download.status}
                            {download.retryCount > 0 && (
                              <> • Retry {download.retryCount}</>
                            )}
                            {(() => {
                              // Calculate downloaded bytes from percentage if bytesDownloaded is NaN or invalid
                              let downloadedBytes = download.bytesDownloaded;
                              if (download.totalBytes > 0 && (isNaN(downloadedBytes) || downloadedBytes === undefined)) {
                                // Calculate from percentage
                                downloadedBytes = Math.round((download.progress / 100) * download.totalBytes);
                              }
                              
                              if (download.totalBytes > 0 && !isNaN(downloadedBytes) && downloadedBytes >= 0) {
                                return <> • {formatFileSize(downloadedBytes)} / {formatFileSize(download.totalBytes)}</>;
                              }
                              
                              if (download.totalBytes <= 0 && !isNaN(downloadedBytes) && downloadedBytes > 0) {
                                return <> • {formatFileSize(downloadedBytes)}</>;
                              }
                              
                              return null;
                            })()}
                          </span>
                          <span>
                            {download.speed > 0 && !isNaN(download.speed) && `${formatFileSize(download.speed)}/s`}
                            {download.estimatedTimeRemaining > 0 && download.speed > 0 && !isNaN(download.estimatedTimeRemaining) && (
                              <> • ETA: {Math.ceil(download.estimatedTimeRemaining / 60)}m</>
                            )}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}
          </div>

          {/* Main Content */}
          <div className="lg:col-span-3 space-y-6">
            {/* Search Bar */}
            <Card>
              <CardContent className="p-6">
                <div className="flex gap-4">
                  <div className="flex-1">
                    <Input
                      placeholder="Search GGUF models... (e.g., 'llama gguf', 'mistral gguf', 'qwen gguf')"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      onKeyPress={(e) => e.key === 'Enter' && searchModels()}
                      icon={<SearchIcon className="w-4 h-4" />}
                      className="w-full"
                    />
                  </div>
                  <Button
                    onClick={searchModels}
                    disabled={!searchQuery.trim() || loading}
                    loading={loading}
                    className="flex items-center gap-2"
                  >
                    <SearchIcon className="w-4 h-4" />
                    Search
                  </Button>
                </div>
                
                {!hfApiKey && (
                  <div className="mt-4 p-4 bg-warning-50 border border-warning-200 rounded-lg dark:bg-warning-900/20 dark:border-warning-800">
                    <div className="flex items-start gap-3">
                      <AlertCircleIcon className="w-5 h-5 text-warning-600 dark:text-warning-400 flex-shrink-0 mt-0.5" />
                      <div>
                        <p className="text-sm font-medium text-warning-800 dark:text-warning-200">
                          HuggingFace API Key Required
                        </p>
                        <p className="text-sm text-warning-700 dark:text-warning-300 mt-1">
                          Add your API key in settings to avoid rate limits and access private models.
                        </p>
                        <Button
                          variant="primary"
                          size="sm"
                          onClick={() => setShowSettings(true)}
                          className="mt-3 bg-warning-600 hover:bg-warning-700 border-warning-600 hover:border-warning-700"
                        >
                          <KeyIcon className="w-3 h-3 mr-1" />
                          Configure API Key
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Models Grid */}
            {filteredModels.length > 0 && (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
                {filteredModels.map((model) => (
                  <motion.div
                    key={model.id}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    whileHover={{ y: -4 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Card variant="elevated" hover className="h-full">
                      <CardHeader>
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex-1 min-w-0">
                            <CardTitle className="text-base truncate">{model.id}</CardTitle>
                            <CardDescription className="text-sm text-text-tertiary">
                              by {model.author}
                            </CardDescription>
                          </div>
                          <div className="flex items-center gap-2 text-xs text-text-tertiary">
                            <StarIcon className="w-3 h-3" />
                            {model.likes.toLocaleString()}
                          </div>
                        </div>
                      </CardHeader>

                      <CardContent className="space-y-4">
                        {/* Tags */}
                        {model.tags && model.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {model.tags.slice(0, 3).map((tag) => (
                              <span
                                key={tag}
                                className="text-xs bg-surface-secondary text-text-secondary px-2 py-1 rounded-full"
                              >
                                {tag}
                              </span>
                            ))}
                            {model.tags.length > 3 && (
                              <span className="text-xs text-text-tertiary">
                                +{model.tags.length - 3} more
                              </span>
                            )}
                          </div>
                        )}

                        {/* Stats */}
                        <div className="flex items-center justify-between text-xs text-text-secondary">
                          <div className="flex items-center gap-1">
                            <DownloadIcon className="w-3 h-3" />
                            {model.downloads.toLocaleString()} downloads
                          </div>
                          <div className="flex items-center gap-1">
                            <ClockIcon className="w-3 h-3" />
                            {new Date(model.updatedAt).toLocaleDateString()}
                          </div>
                        </div>

                        {/* Model size info */}
                        {(() => {
                          const ggufFiles = model.siblings?.filter(s => 
                            s.rfilename.toLowerCase().endsWith('.gguf')
                          ) || [];
                          
                          return ggufFiles.length > 0 && (
                            <div className="text-xs text-text-secondary">
                              <div className="flex items-center gap-1 mb-1">
                                <HardDriveIcon className="w-3 h-3" />
                                {ggufFiles.length} GGUF {ggufFiles.length === 1 ? 'file' : 'files'}
                              </div>
                              {ggufFiles.some(s => s.size) && (
                                <div className="text-text-tertiary">
                                  Total: {formatFileSize(
                                    ggufFiles.reduce((sum, s) => sum + (s.size || 0), 0)
                                  )}
                                </div>
                              )}
                            </div>
                          );
                        })()}

                        {/* Actions */}
                        <div className="flex gap-2 pt-2">
                          <Button
                            variant="primary"
                            size="sm"
                            onClick={() => setSelectedModel(model)}
                            className="flex-1"
                          >
                            <DownloadIcon className="w-3 h-3 mr-1" />
                            Download
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => window.open(`https://huggingface.co/${model.id}`, '_blank')}
                          >
                            <ExternalLinkIcon className="w-3 h-3" />
                          </Button>
                        </div>
                      </CardContent>
                    </Card>
                  </motion.div>
                ))}
              </div>
            )}

            {/* No results */}
            {!loading && models.length === 0 && searchQuery && (
              <Card>
                <CardContent className="text-center py-12">
                  <DatabaseIcon className="w-16 h-16 text-text-tertiary mx-auto mb-4" />
                  <h3 className="text-lg font-semibold text-text-primary mb-2">No models found</h3>
                  <p className="text-text-secondary">
                    Try searching for different keywords or check your spelling.
                  </p>
                </CardContent>
              </Card>
            )}

            {/* Getting started */}
            {!searchQuery && (
              <Card>
                <CardContent className="text-center py-12">
                  <SearchIcon className="w-16 h-16 text-text-tertiary mx-auto mb-4" />
                  <h3 className="text-lg font-semibold text-text-primary mb-2">Discover AI Models</h3>
                  <p className="text-text-secondary mb-4">
                    Search for AI models from HuggingFace Hub. Try popular models like "llama", "mistral", or "code".
                  </p>
                  <div className="flex flex-wrap justify-center gap-2">
                    {['llama gguf', 'mistral gguf', 'qwen gguf', 'phi gguf', 'gemma gguf'].map((suggestion) => (
                      <Button
                        key={suggestion}
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setSearchQuery(suggestion);
                          // Use setTimeout to ensure state is updated before search
                          setTimeout(() => searchModels(), 0);
                        }}
                      >
                        {suggestion}
                      </Button>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </div>

      {/* Settings Modal */}
      <Modal
        open={showSettings}
        onClose={() => setShowSettings(false)}
        title="Download Settings"
        description="Configure your HuggingFace API key and download preferences"
        size="md"
      >
        <div className="space-y-6">
          <div>
            <label className="block text-sm font-medium text-text-primary mb-2">
              HuggingFace API Key
            </label>
            <Input
              type="password"
              placeholder="hf_xxxxxxxxxxxxxxxxxxxx"
              value={hfApiKey}
              onChange={(e) => setHfApiKey(e.target.value)}
              icon={<KeyIcon className="w-4 h-4" />}
              helper="Get your API key from https://huggingface.co/settings/tokens"
            />
          </div>

          <div className="flex justify-end gap-3">
            <Button variant="ghost" onClick={() => setShowSettings(false)}>
              Cancel
            </Button>
            <Button 
              variant="primary" 
              onClick={() => {
                saveApiKey(hfApiKey);
                setShowSettings(false);
              }}
            >
              Save Settings
            </Button>
          </div>
        </div>
      </Modal>

      {/* Model Details Modal */}
      {selectedModel && (
        <Modal
          open={!!selectedModel}
          onClose={() => setSelectedModel(null)}
          title={selectedModel.id}
          description={`by ${selectedModel.author}`}
          size="lg"
        >
          <div className="space-y-6">
            {/* Model Info */}
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-text-secondary">Downloads:</span>
                <span className="ml-2 text-text-primary">{selectedModel.downloads.toLocaleString()}</span>
              </div>
              <div>
                <span className="text-text-secondary">Likes:</span>
                <span className="ml-2 text-text-primary">{selectedModel.likes.toLocaleString()}</span>
              </div>
              <div>
                <span className="text-text-secondary">Updated:</span>
                <span className="ml-2 text-text-primary">{new Date(selectedModel.updatedAt).toLocaleDateString()}</span>
              </div>
              <div>
                <span className="text-text-secondary">Pipeline:</span>
                <span className="ml-2 text-text-primary">{selectedModel.pipeline_tag || 'N/A'}</span>
              </div>
            </div>

            {/* Files */}
            <div>
              <h4 className="text-sm font-semibold text-text-primary mb-3">Available GGUF Files</h4>
              {(() => {
                // Filter to only show GGUF files since ClaraCore only supports GGUF
                const ggufFiles = selectedModel.siblings?.filter(file => 
                  file.rfilename.toLowerCase().endsWith('.gguf')
                ) || [];
                
                return ggufFiles.length > 0 ? (
                  <div className="space-y-2 max-h-60 overflow-y-auto">
                    {ggufFiles.map((file) => (
                    <div
                      key={file.rfilename}
                      className="flex items-center justify-between p-3 bg-surface-secondary rounded-lg"
                    >
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-text-primary truncate">
                          {file.rfilename}
                        </p>
                        {file.size && (
                          <p className="text-xs text-text-secondary">
                            {formatFileSize(file.size)}
                          </p>
                        )}
                      </div>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => startDownload(selectedModel, file.rfilename)}
                        className="ml-3"
                      >
                        <DownloadIcon className="w-3 h-3 mr-1" />
                        Download
                      </Button>
                    </div>
                  ))}
                  </div>
                ) : (
                  <div className="text-center py-8 text-text-secondary">
                    <p>No GGUF files found for this model.</p>
                    <p className="text-xs mt-1">ClaraCore only supports GGUF format models. Try searching for models with "GGUF" in the name.</p>
                  </div>
                );
              })()}
            </div>

            <div className="flex justify-end gap-3">
              <Button variant="ghost" onClick={() => setSelectedModel(null)}>
                Close
              </Button>
              <Button
                variant="outline"
                onClick={() => window.open(`https://huggingface.co/${selectedModel.id}`, '_blank')}
              >
                <ExternalLinkIcon className="w-4 h-4 mr-2" />
                View on HuggingFace
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default ModelDownloaderPage;
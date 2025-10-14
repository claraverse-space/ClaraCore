import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { 
  FolderIcon, 
  DownloadIcon, 
  PlusIcon,
  CheckIcon,
  AlertCircleIcon,
  HardDriveIcon
} from 'lucide-react';
import { Modal } from './ui/Modal';
import { Button } from './ui/Button';
import { Input } from './ui/Input';

interface DownloadDestination {
  path: string;
  name: string;
  type: 'default' | 'folder';
  enabled: boolean;
  modelCount?: number;
  lastScanned?: string;
  description: string;
}

interface DownloadDestinationModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (destinationPath?: string) => void;
  modelName: string;
  filename: string;
}

export const DownloadDestinationModal: React.FC<DownloadDestinationModalProps> = ({
  open,
  onClose,
  onSelect,
  modelName,
  filename
}) => {
  const [destinations, setDestinations] = useState<DownloadDestination[]>([]);
  const [selectedDestination, setSelectedDestination] = useState<string>('');
  const [showCustomPath, setShowCustomPath] = useState(false);
  const [customPath, setCustomPath] = useState('');
  const [loading, setLoading] = useState(false);

  // Fetch available destinations when modal opens
  useEffect(() => {
    if (open) {
      fetchDestinations();
    }
  }, [open]);

  const fetchDestinations = async () => {
    setLoading(true);
    try {
      // Add 1-second timeout to the fetch request
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 1000);

      const response = await fetch('/api/models/download-destinations', {
        signal: controller.signal
      });

      clearTimeout(timeoutId);

      if (response.ok) {
        const data = await response.json();
        setDestinations(data.destinations || []);

        // Auto-select the first destination if available
        if (data.destinations && data.destinations.length > 0) {
          setSelectedDestination(data.destinations[0].path);
        } else {
          // If no destinations returned, the backend will use default downloads folder
          // Set empty string to indicate using backend default
          setSelectedDestination('');
        }
      } else {
        // If API fails, still allow download with empty destination (backend will use default)
        console.warn('Failed to fetch destinations, will use default downloads folder');
        setDestinations([]);
        setSelectedDestination('');
      }
    } catch (error: any) {
      if (error?.name === 'AbortError') {
        console.warn('Fetch destinations timed out after 1 second, will use default downloads folder');
      } else {
        console.error('Failed to fetch download destinations:', error);
      }
      // On timeout or error, allow download with empty destination (backend will use default)
      setDestinations([]);
      setSelectedDestination('');
    } finally {
      setLoading(false);
    }
  };

  const handleSelect = () => {
    const finalPath = showCustomPath ? customPath : selectedDestination;
    // Allow empty finalPath (will use backend default downloads folder)
    onSelect(finalPath || undefined);
    onClose();
  };

  const handleClose = () => {
    setShowCustomPath(false);
    setCustomPath('');
    setSelectedDestination('');
    onClose();
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Choose Download Destination"
      description={`Where would you like to download ${filename}?`}
      size="lg"
    >
      <div className="space-y-6">
        {/* Model Info */}
        <div className="flex items-center gap-3 p-4 bg-surface-secondary rounded-lg">
          <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center">
            <DownloadIcon className="w-5 h-5 text-white" />
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="font-medium text-text-primary truncate">{modelName}</h3>
            <p className="text-sm text-text-secondary truncate">{filename}</p>
          </div>
        </div>

        {/* Destination Options */}
        <div className="space-y-4">
          <h4 className="text-sm font-semibold text-text-primary">Available Destinations</h4>
          
          {loading ? (
            <div className="text-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-brand-500 mx-auto"></div>
              <p className="text-text-secondary mt-2">Loading destinations...</p>
            </div>
          ) : (
            <div className="space-y-2">
              {/* Show message if no destinations available */}
              {destinations.length === 0 && (
                <div className="p-4 bg-surface-secondary rounded-lg text-center">
                  <p className="text-sm text-text-secondary mb-2">
                    No download destinations configured yet
                  </p>
                  <p className="text-xs text-text-tertiary">
                    File will be downloaded to the default downloads folder
                  </p>
                </div>
              )}

              {/* Existing Destinations */}
              {destinations.map((destination) => (
                <motion.div
                  key={destination.path}
                  whileHover={{ scale: 1.01 }}
                  className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                    selectedDestination === destination.path && !showCustomPath
                      ? 'border-brand-500  dark:bg-brand-900/20'
                      : 'border-border-secondary hover:border-brand-300 hover:bg-surface-secondary'
                  }`}
                  onClick={() => {
                    setSelectedDestination(destination.path);
                    setShowCustomPath(false);
                  }}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                      destination.type === 'default' 
                        ? 'bg-green-100 dark:bg-green-900/30' 
                        : 'bg-blue-100 dark:bg-blue-900/30'
                    }`}>
                      {destination.type === 'default' ? (
                        <DownloadIcon className="w-4 h-4 text-green-600 dark:text-green-400" />
                      ) : (
                        <FolderIcon className="w-4 h-4 text-blue-600 dark:text-blue-400" />
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-text-primary">{destination.name}</span>
                        {selectedDestination === destination.path && !showCustomPath && (
                          <CheckIcon className="w-4 h-4 text-brand-500" />
                        )}
                      </div>
                      <p className="text-xs text-text-secondary truncate">{destination.path}</p>
                      <p className="text-xs text-text-tertiary">{destination.description}</p>
                    </div>
                    {destination.modelCount !== undefined && (
                      <div className="text-right">
                        <div className="text-xs text-text-secondary">
                          {destination.modelCount} models
                        </div>
                      </div>
                    )}
                  </div>
                </motion.div>
              ))}

              {/* Custom Path Option */}
              <motion.div
                whileHover={{ scale: 1.01 }}
                className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
                  showCustomPath
                    ? 'border-brand-500  dark:bg-brand-900/20'
                    : 'border-border-secondary hover:border-brand-300 hover:bg-surface-secondary'
                }`}
                onClick={() => {
                  setShowCustomPath(true);
                  setSelectedDestination('');
                }}
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 bg-purple-100 dark:bg-purple-900/30 rounded-full flex items-center justify-center">
                    <PlusIcon className="w-4 h-4 text-purple-600 dark:text-purple-400" />
                  </div>
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-text-primary">Custom Path</span>
                      {showCustomPath && (
                        <CheckIcon className="w-4 h-4 text-brand-500" />
                      )}
                    </div>
                    <p className="text-xs text-text-tertiary">Specify a custom download location</p>
                  </div>
                </div>
              </motion.div>

              {/* Custom Path Input */}
              {showCustomPath && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="mt-3"
                >
                  <Input
                    placeholder="C:\\AI\\Models\\Custom"
                    value={customPath}
                    onChange={(e) => setCustomPath(e.target.value)}
                    icon={<HardDriveIcon className="w-4 h-4" />}
                    className="w-full"
                  />
                  <p className="text-xs text-text-tertiary mt-2">
                    Enter the full path where you want to download the model
                  </p>
                </motion.div>
              )}
            </div>
          )}

          {/* Info Box */}
          <div className="flex items-start gap-3 p-3  dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
            <AlertCircleIcon className="w-4 h-4 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
            <div className="text-xs text-blue-700 dark:text-blue-300">
              <p className="font-medium mb-1">Download Location Info</p>
              <p>• The model will be saved to the selected folder</p>
              <p>• You can add this folder to your model database later</p>
              <p>• Default downloads folder is automatically tracked</p>
            </div>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex justify-end gap-3">
          <Button variant="ghost" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleSelect}
            disabled={showCustomPath && !customPath}
            className="flex items-center gap-2"
          >
            <DownloadIcon className="w-4 h-4" />
            Start Download
          </Button>
        </div>
      </div>
    </Modal>
  );
};

export default DownloadDestinationModal;
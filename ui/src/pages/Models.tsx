import { useState, useCallback, useMemo } from "react";
import { useAPI } from "../contexts/APIProvider";
import { LogPanel } from "./LogViewer";
import { usePersistentState } from "../hooks/usePersistentState";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { useTheme } from "../contexts/ThemeProvider";
import { EyeIcon, EyeOffIcon, StopCircleIcon, RefreshCwIcon, ExternalLinkIcon, CpuIcon } from "lucide-react";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, Button, Table } from "../components";
import { motion } from "framer-motion";

export default function ModelsPage() {
  const { isNarrow } = useTheme();
  const direction = isNarrow ? "vertical" : "horizontal";
  const { upstreamLogs } = useAPI();

  return (
    <PanelGroup direction={direction} className="gap-2" autoSaveId={"models-panel-group"}>
      <Panel id="models" defaultSize={50} minSize={isNarrow ? 0 : 25} maxSize={100} collapsible={isNarrow}>
        <ModelsPanel />
      </Panel>

      <PanelResizeHandle
        className={`panel-resize-handle ${
          direction === "horizontal" ? "w-3 h-full" : "w-full h-3 horizontal"
        }`}
      />
      <Panel collapsible={true} defaultSize={50} minSize={0}>
        <div className="flex flex-col h-full space-y-4">
          {direction === "horizontal" && <StatsPanel />}
          <div className="flex-1 min-h-0">
            <LogPanel id="modelsupstream" title="Upstream Logs" logData={upstreamLogs} />
          </div>
        </div>
      </Panel>
    </PanelGroup>
  );
}

function ModelsPanel() {
  const { models, loadModel, unloadAllModels } = useAPI();
  const [isUnloading, setIsUnloading] = useState(false);
  const [showUnlisted, setShowUnlisted] = usePersistentState("showUnlisted", true);
  const [showIdorName, setShowIdorName] = usePersistentState<"id" | "name">("showIdorName", "id");

  const filteredModels = useMemo(() => {
    return models.filter((model) => showUnlisted || !model.unlisted);
  }, [models, showUnlisted]);

  const handleUnloadAllModels = useCallback(async () => {
    setIsUnloading(true);
    try {
      await unloadAllModels();
    } catch (e) {
      console.error(e);
    } finally {
      setTimeout(() => {
        setIsUnloading(false);
      }, 1000);
    }
  }, [unloadAllModels]);

  const toggleIdorName = useCallback(() => {
    setShowIdorName((prev) => (prev === "name" ? "id" : "name"));
  }, [setShowIdorName]);

  return (
    <Card className="h-full flex flex-col m-2">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 bg-gradient-to-br from-brand-500 to-brand-600 rounded-lg flex items-center justify-center">
              <CpuIcon className="w-5 h-5 text-white" />
            </div>
            <div>
              <CardTitle className="text-2xl">Models</CardTitle>
              <CardDescription>{filteredModels.length} models available</CardDescription>
            </div>
          </div>
          
          <Button
            variant="danger"
            size="lg"
            loading={isUnloading}
            onClick={handleUnloadAllModels}
            disabled={isUnloading}
            className="flex items-center gap-3"
          >
            <StopCircleIcon className="w-5 h-5" />
            {isUnloading ? "Unloading All..." : "Unload All Models"}
          </Button>
        </div>

        {/* Controls */}
        <div className="flex gap-3 mt-4">
          <Button
            variant="secondary"
            onClick={toggleIdorName}
            className="flex items-center gap-2"
          >
            <RefreshCwIcon className="w-4 h-4" />
            Show {showIdorName === "id" ? "Names" : "IDs"}
          </Button>

          <Button
            variant={showUnlisted ? "primary" : "secondary"}
            onClick={() => setShowUnlisted(!showUnlisted)}
            className="flex items-center gap-2"
          >
            {showUnlisted ? <EyeIcon className="w-4 h-4" /> : <EyeOffIcon className="w-4 h-4" />}
            {showUnlisted ? "Hide" : "Show"} Unlisted
          </Button>
        </div>
      </CardHeader>

      <CardContent className="flex-1 overflow-y-auto">
        <div className="space-y-3">
          {filteredModels.map((model, index) => (
            <motion.div
              key={model.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.05 }}
            >
              <ModelCard 
                model={model} 
                showIdorName={showIdorName}
                onLoad={() => loadModel(model.id)}
              />
            </motion.div>
          ))}
        </div>
        
        {filteredModels.length === 0 && (
          <motion.div 
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="text-center py-12"
          >
            <div className="w-16 h-16 bg-surface-secondary rounded-full mx-auto mb-4 flex items-center justify-center">
              <CpuIcon className="w-8 h-8 text-text-tertiary" />
            </div>
            <p className="text-text-secondary mb-2">No models found</p>
            <p className="text-text-tertiary text-sm">Try adjusting your filters</p>
          </motion.div>
        )}
      </CardContent>
    </Card>
  );
}

// Modern ModelCard component
interface ModelCardProps {
  model: any;
  showIdorName: "id" | "name";
  onLoad: () => void;
}

function ModelCard({ model, showIdorName, onLoad }: ModelCardProps) {
  const displayName = showIdorName === "id" ? model.id : (model.name || model.id);
  const isAvailable = model.state === "stopped";
  
  return (
    <Card 
      variant="elevated" 
      hover={isAvailable}
      className={`transition-all duration-300 ${
        model.unlisted ? 'opacity-75' : ''
      } ${isAvailable ? 'hover:border-brand-500/50' : ''}`}
    >
      <CardContent>
        <div className="flex items-center justify-between">
          {/* Left side: Model info */}
          <div className="flex items-center gap-4 flex-1 min-w-0">
            {/* Model Avatar */}
            <div className="w-12 h-12 bg-gradient-to-br from-brand-500 to-brand-600 rounded-lg flex items-center justify-center flex-shrink-0">
              <span className="text-white font-bold text-lg">
                {displayName.charAt(0).toUpperCase()}
              </span>
            </div>
            
            {/* Model Details */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3 mb-1">
                <h3 className={`font-semibold text-lg ${
                  model.unlisted ? 'text-text-tertiary' : 'text-text-primary'
                } truncate`}>
                  {displayName}
                </h3>
                <ModelStatusBadge state={model.state} />
              </div>
              
              {showIdorName === "name" && model.name && (
                <p className="text-text-tertiary text-sm font-mono mb-1 truncate">{model.id}</p>
              )}
              
              {model.description && (
                <p className={`text-sm line-clamp-1 ${
                  model.unlisted ? 'text-text-tertiary' : 'text-text-secondary'
                }`}>
                  {model.description}
                </p>
              )}
            </div>
          </div>

          {/* Right side: Actions */}
          <div className="flex items-center gap-3 flex-shrink-0 ml-4">
            <a 
              href={model.proxyUrl && model.proxyUrl.trim().length > 0 ? `${model.proxyUrl.replace(/\/$/, '')}/` : `/upstream/${model.id}/`} 
              target="_blank"
              title="View model details"
              className="inline-flex items-center justify-center gap-2 px-3 py-2 text-sm font-medium rounded-lg border border-border-secondary bg-surface hover:bg-surface-secondary transition-colors"
            >
              <ExternalLinkIcon className="w-4 h-4" />
            </a>
            
            <Button
              variant={isAvailable ? "primary" : "secondary"}
              size="sm"
              disabled={!isAvailable}
              onClick={onLoad}
              className="min-w-[80px]"
            >
              {isAvailable ? "Load" : model.state}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// Modern StatusBadge component
function ModelStatusBadge({ state }: { state: string }) {
  const getStatusConfig = (state: string) => {
    switch (state) {
      case 'stopped':
        return { color: 'bg-neutral-100 text-neutral-800 dark:bg-neutral-800 dark:text-neutral-300', icon: '⏹️' };
      case 'loading':
        return { color: 'bg-info-100 text-info-800 dark:bg-info-900/20 dark:text-info-300', icon: '⏳' };
      case 'ready':
        return { color: 'bg-success-100 text-success-800 dark:bg-success-900/20 dark:text-success-300', icon: '✅' };
      case 'error':
        return { color: 'bg-error-100 text-error-800 dark:bg-error-900/20 dark:text-error-300', icon: '❌' };
      default:
        return { color: 'bg-neutral-100 text-neutral-800 dark:bg-neutral-800 dark:text-neutral-300', icon: '❓' };
    }
  };
  
  const config = getStatusConfig(state);
  
  return (
    <motion.span 
      initial={{ scale: 0.8, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      className={`px-2 py-1 rounded-full text-xs font-medium flex items-center gap-1 ${config.color}`}
    >
      <span className="text-xs">{config.icon}</span>
      {state}
    </motion.span>
  );
}

function StatsPanel() {
  const { metrics } = useAPI();

  const statsData = useMemo(() => {
    const totalRequests = metrics.length;
    if (totalRequests === 0) {
      return [{
        requests: '0',
        processed: '0',
        generated: '0',
        tokensPerSec: '0.00'
      }];
    }
    const totalInputTokens = metrics.reduce((sum, m) => sum + m.input_tokens, 0);
    const totalOutputTokens = metrics.reduce((sum, m) => sum + m.output_tokens, 0);
    const avgTokensPerSecond = (metrics.reduce((sum, m) => sum + m.tokens_per_second, 0) / totalRequests).toFixed(2);
    
    return [{
      requests: totalRequests.toString(),
      processed: new Intl.NumberFormat().format(totalInputTokens),
      generated: new Intl.NumberFormat().format(totalOutputTokens),
      tokensPerSec: avgTokensPerSecond
    }];
  }, [metrics]);

  const columns = [
    { key: 'requests', title: 'Requests', dataIndex: 'requests' as const, align: 'right' as const },
    { key: 'processed', title: 'Processed', dataIndex: 'processed' as const, align: 'right' as const },
    { key: 'generated', title: 'Generated', dataIndex: 'generated' as const, align: 'right' as const },
    { key: 'tokensPerSec', title: 'Tokens/Sec', dataIndex: 'tokensPerSec' as const, align: 'right' as const },
  ];

  return (
    <Card>
      <CardContent className="p-0">
        <Table 
          data={statsData}
          columns={columns}
          sortable={false}
          className="[&_th]:text-right [&_td]:text-right"
        />
      </CardContent>
    </Card>
  );
}

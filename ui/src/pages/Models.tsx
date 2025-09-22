import { useState, useCallback, useMemo } from "react";
import { useAPI } from "../contexts/APIProvider";
import { LogPanel } from "./LogViewer";
import { usePersistentState } from "../hooks/usePersistentState";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { useTheme } from "../contexts/ThemeProvider";
import { RiEyeFill, RiEyeOffFill, RiStopCircleLine, RiSwapBoxFill } from "react-icons/ri";

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
  const [showIdorName, setShowIdorName] = usePersistentState<"id" | "name">("showIdorName", "id"); // true = show ID, false = show name

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
  }, [showIdorName]);

  return (
    <div className="bg-surface rounded-xl h-full flex flex-col p-6 m-2 glass border border-gray-600/50">
      {/* Enhanced Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 bg-gradient-to-br from-primary to-sakura-600 rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-lg">M</span>
          </div>
          <div>
            <h2 className="text-2xl font-bold text-white">Models</h2>
            <p className="text-gray-400 text-sm">{filteredModels.length} models available</p>
          </div>
        </div>
        
        <button
          className={`btn-danger btn-lg flex items-center gap-3 ${isUnloading ? 'btn-loading' : ''}`}
          onClick={handleUnloadAllModels}
          disabled={isUnloading}
        >
          <RiStopCircleLine size="20" />
          <span>{isUnloading ? "Unloading All..." : "Unload All Models"}</span>
        </button>
      </div>

      {/* Controls */}
      <div className="flex gap-3 mb-6">
        <button
          className="btn-secondary flex items-center gap-2"
          onClick={toggleIdorName}
        >
          <RiSwapBoxFill size="18" />
          <span>Show {showIdorName === "id" ? "Names" : "IDs"}</span>
        </button>

        <button
          className={`flex items-center gap-2 ${showUnlisted ? 'btn-primary' : 'btn-secondary'}`}
          onClick={() => setShowUnlisted(!showUnlisted)}
        >
          {showUnlisted ? <RiEyeFill size="18" /> : <RiEyeOffFill size="18" />}
          <span>{showUnlisted ? "Hide" : "Show"} Unlisted</span>
        </button>
      </div>

      {/* Card List */}
      <div className="flex-1 overflow-y-auto">
        <div className="space-y-3">
          {filteredModels.map((model) => (
            <ModelCard 
              key={model.id} 
              model={model} 
              showIdorName={showIdorName}
              onLoad={() => loadModel(model.id)}
            />
          ))}
        </div>
        
        {filteredModels.length === 0 && (
          <div className="text-center py-12">
            <div className="w-16 h-16 bg-gray-800 rounded-full mx-auto mb-4 flex items-center justify-center">
              <span className="text-gray-500 text-2xl">📦</span>
            </div>
            <p className="text-gray-400">No models found</p>
            <p className="text-gray-500 text-sm">Try adjusting your filters</p>
          </div>
        )}
      </div>
    </div>
  );
}

// New ModelCard component
interface ModelCardProps {
  model: any;
  showIdorName: "id" | "name";
  onLoad: () => void;
}

function ModelCard({ model, showIdorName, onLoad }: ModelCardProps) {
  const displayName = showIdorName === "id" ? model.id : (model.name || model.id);
  const isAvailable = model.state === "stopped";
  
  return (
    <div className={`glass rounded-xl p-4 border transition-all duration-300 hover:shadow-xl ${
      model.unlisted ? 'border-gray-700 opacity-75' : 'border-gray-600'
    } ${isAvailable ? 'hover:border-primary/50' : ''}`}>
      
      <div className="flex items-center justify-between">
        {/* Left side: Model info */}
        <div className="flex items-center gap-4 flex-1 min-w-0">
          {/* Model Avatar */}
          <div className="w-12 h-12 bg-gradient-to-br from-primary to-sakura-600 rounded-lg flex items-center justify-center flex-shrink-0">
            <span className="text-white font-bold text-lg">
              {displayName.charAt(0).toUpperCase()}
            </span>
          </div>
          
          {/* Model Details */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-3 mb-1">
              <h3 className={`font-semibold text-lg ${
                model.unlisted ? 'text-gray-400' : 'text-white'
              } truncate`}>
                {displayName}
              </h3>
              <ModelStatusBadge state={model.state} />
            </div>
            
            {showIdorName === "name" && model.name && (
              <p className="text-gray-500 text-sm font-mono mb-1 truncate">{model.id}</p>
            )}
            
            {model.description && (
              <p className={`text-sm line-clamp-1 ${
                model.unlisted ? 'text-gray-500' : 'text-gray-300'
              }`}>
                {model.description}
              </p>
            )}
          </div>
        </div>

        {/* Right side: Actions */}
        <div className="flex items-center gap-3 flex-shrink-0 ml-4">
          <a 
            href={`/upstream/${model.id}/`} 
            target="_blank"
            className="btn-secondary btn-sm px-3 py-2"
            title="View model details"
          >
            <span className="text-sm">🔗</span>
          </a>
          
          <button
            className={`btn-sm px-4 py-2 min-w-[80px] ${isAvailable ? 'btn-primary' : 'btn-secondary'}`}
            disabled={!isAvailable}
            onClick={onLoad}
          >
            {isAvailable ? "Load" : model.state}
          </button>
        </div>
      </div>
    </div>
  );
}

// Enhanced StatusBadge component
function ModelStatusBadge({ state }: { state: string }) {
  const getStatusConfig = (state: string) => {
    switch (state) {
      case 'stopped':
        return { color: 'bg-gray-700 text-gray-300', icon: '⏹️' };
      case 'loading':
        return { color: 'bg-blue-900/50 text-blue-300', icon: '⏳' };
      case 'ready':
        return { color: 'bg-green-900/50 text-green-300', icon: '✅' };
      case 'error':
        return { color: 'bg-red-900/50 text-red-300', icon: '❌' };
      default:
        return { color: 'bg-gray-700 text-gray-300', icon: '❓' };
    }
  };
  
  const config = getStatusConfig(state);
  
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium flex items-center gap-1 ${config.color}`}>
      <span className="text-xs">{config.icon}</span>
      {state}
    </span>
  );
}

function StatsPanel() {
  const { metrics } = useAPI();

  const [totalRequests, totalInputTokens, totalOutputTokens, avgTokensPerSecond] = useMemo(() => {
    const totalRequests = metrics.length;
    if (totalRequests === 0) {
      return [0, 0, 0];
    }
    const totalInputTokens = metrics.reduce((sum, m) => sum + m.input_tokens, 0);
    const totalOutputTokens = metrics.reduce((sum, m) => sum + m.output_tokens, 0);
    const avgTokensPerSecond = (metrics.reduce((sum, m) => sum + m.tokens_per_second, 0) / totalRequests).toFixed(2);
    return [totalRequests, totalInputTokens, totalOutputTokens, avgTokensPerSecond];
  }, [metrics]);

  return (
    <div className="bg-surface rounded-lg p-4">
      <div className="rounded-lg overflow-hidden border border-gray-600">
        <table className="w-full">
          <thead>
            <tr className="border-b border-gray-600 text-right bg-surface-elevated">
              <th className="text-gray-200 py-3 px-4">Requests</th>
              <th className="border-l border-gray-600 text-gray-200 py-3 px-4">Processed</th>
              <th className="border-l border-gray-600 text-gray-200 py-3 px-4">Generated</th>
              <th className="border-l border-gray-600 text-gray-200 py-3 px-4">Tokens/Sec</th>
            </tr>
          </thead>
          <tbody>
            <tr className="text-right text-gray-300">
              <td className="border-r border-gray-600 py-3 px-4">{totalRequests}</td>
              <td className="border-r border-gray-600 py-3 px-4">
                {new Intl.NumberFormat().format(totalInputTokens)}
              </td>
              <td className="border-r border-gray-600 py-3 px-4">
                {new Intl.NumberFormat().format(totalOutputTokens)}
              </td>
              <td className="py-3 px-4">{avgTokensPerSecond}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

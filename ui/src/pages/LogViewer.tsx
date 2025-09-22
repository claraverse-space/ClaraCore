import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useAPI } from "../contexts/APIProvider";
import { usePersistentState } from "../hooks/usePersistentState";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import {
  RiTextWrap,
  RiAlignJustify,
  RiFontSize,
  RiMenuSearchLine,
  RiMenuSearchFill,
  RiCloseCircleFill,
} from "react-icons/ri";
import { useTheme } from "../contexts/ThemeProvider";

const LogViewer = () => {
  const { proxyLogs, upstreamLogs } = useAPI();
  const { screenWidth } = useTheme();
  const direction = screenWidth === "xs" || screenWidth === "sm" ? "vertical" : "horizontal";

  return (
    <PanelGroup direction={direction} className="gap-2" autoSaveId="logviewer-panel-group">
      <Panel id="proxy" defaultSize={50} minSize={5} maxSize={100} collapsible={true}>
        <LogPanel id="proxy" title="Proxy Logs" logData={proxyLogs} />
      </Panel>
      <PanelResizeHandle
        className={`panel-resize-handle ${
          direction === "horizontal" ? "w-3 h-full" : "w-full h-3 horizontal"
        }`}
      />
      <Panel id="upstream" defaultSize={50} minSize={5} maxSize={100} collapsible={true}>
        <LogPanel id="upstream" title="Upstream Logs" logData={upstreamLogs} />
      </Panel>
    </PanelGroup>
  );
};

interface LogPanelProps {
  id: string;
  title: string;
  logData: string;
}
export const LogPanel = ({ id, title, logData }: LogPanelProps) => {
  const [filterRegex, setFilterRegex] = useState("");
  const [fontSize, setFontSize] = usePersistentState<"xxs" | "xs" | "small" | "normal">(
    `logPanel-${id}-fontSize`,
    "normal"
  );
  const [wrapText, setTextWrap] = usePersistentState(`logPanel-${id}-wrapText`, false);
  const [showFilter, setShowFilter] = usePersistentState(`logPanel-${id}-showFilter`, false);

  const textWrapClass = useMemo(() => {
    return wrapText ? "whitespace-pre-wrap" : "whitespace-pre";
  }, [wrapText]);

  const toggleFontSize = useCallback(() => {
    setFontSize((prev) => {
      switch (prev) {
        case "xxs":
          return "xs";
        case "xs":
          return "small";
        case "small":
          return "normal";
        case "normal":
          return "xxs";
      }
    });
  }, []);

  const toggleWrapText = useCallback(() => {
    setTextWrap((prev) => !prev);
  }, []);

  const toggleFilter = useCallback(() => {
    if (showFilter) {
      setShowFilter(false);
      setFilterRegex(""); // Clear filter when closing
    } else {
      setShowFilter(true);
    }
  }, [filterRegex, setFilterRegex, showFilter]);

  const fontSizeClass = useMemo(() => {
    switch (fontSize) {
      case "xxs":
        return "text-[0.5rem]"; // 0.5rem (8px)
      case "xs":
        return "text-[0.75rem]"; // 0.75rem (12px)
      case "small":
        return "text-[0.875rem]"; // 0.875rem (14px)
      case "normal":
        return "text-base"; // 1rem (16px)
    }
  }, [fontSize]);

  const filteredLogs = useMemo(() => {
    if (!filterRegex) return logData;
    try {
      const regex = new RegExp(filterRegex, "i");
      const lines = logData.split("\n");
      const filtered = lines.filter((line) => regex.test(line));
      return filtered.join("\n");
    } catch (e) {
      return logData; // Return unfiltered if regex is invalid
    }
  }, [logData, filterRegex]);

  // auto scroll to bottom
  const preTagRef = useRef<HTMLPreElement>(null);
  useEffect(() => {
    if (!preTagRef.current) return;
    preTagRef.current.scrollTop = preTagRef.current.scrollHeight;
  }, [filteredLogs]);

  return (
    <div className="glass rounded-xl overflow-hidden flex flex-col h-full m-2 border border-gray-600/50">
      {/* Header with better styling */}
      <div className="bg-gradient-to-r from-surface-elevated to-surface-hover px-6 py-4 border-b border-gray-600/50">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-3 h-3 rounded-full bg-primary animate-pulse"></div>
            <h3 className="text-lg font-semibold text-white">{title}</h3>
            <span className="text-xs text-gray-400 bg-gray-800 px-2 py-1 rounded-full">
              {logData.split('\n').filter(line => line.trim()).length} lines
            </span>
          </div>

          <div className="flex gap-2 items-center">
            <button 
              className="btn-secondary btn-sm flex items-center gap-1.5" 
              onClick={toggleFontSize}
              title="Toggle font size"
            >
              <RiFontSize className="w-4 h-4" />
              <span className="text-xs">{fontSize.toUpperCase()}</span>
            </button>
            <button 
              className="btn-secondary btn-sm flex items-center gap-1.5" 
              onClick={toggleWrapText}
              title="Toggle text wrapping"
            >
              {wrapText ? <RiTextWrap className="w-4 h-4" /> : <RiAlignJustify className="w-4 h-4" />}
            </button>
            <button 
              className={`btn-sm flex items-center gap-1.5 ${showFilter ? 'btn-primary' : 'btn-secondary'}`}
              onClick={toggleFilter}
              title="Toggle filter"
            >
              {showFilter ? <RiMenuSearchFill className="w-4 h-4" /> : <RiMenuSearchLine className="w-4 h-4" />}
              {showFilter && <span className="text-xs">Filter</span>}
            </button>
          </div>
        </div>

        {/* Enhanced filtering UI */}
        {showFilter && (
          <div className="mt-4 flex gap-3 items-center">
            <div className="flex-1 relative">
              <input
                type="text"
                className="w-full text-sm p-3 pl-10 rounded-lg bg-surface border border-gray-600 text-white placeholder-gray-400 focus:border-primary focus:ring-1 focus:ring-primary focus:outline-none transition-all"
                placeholder="Filter logs (regex supported)..."
                value={filterRegex}
                onChange={(e) => setFilterRegex(e.target.value)}
              />
              <RiMenuSearchLine className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
            </div>
            <button 
              className="btn-danger btn-sm flex items-center gap-1.5" 
              onClick={() => setFilterRegex("")}
              title="Clear filter"
            >
              <RiCloseCircleFill className="w-4 h-4" />
            </button>
          </div>
        )}
      </div>

      {/* Enhanced log display */}
      <div className="flex-1 overflow-hidden relative">
        <pre 
          ref={preTagRef} 
          className={`${textWrapClass} ${fontSizeClass} h-full overflow-auto p-6 text-gray-200 font-mono leading-relaxed log-content`}
          style={{
            background: 'linear-gradient(180deg, rgba(10,10,10,0.95) 0%, rgba(17,17,17,0.95) 100%)'
          }}
        >
          {filteredLogs}
        </pre>
        
        {/* Scroll indicator */}
        <div className="absolute bottom-4 right-4 bg-gray-800/80 text-gray-300 text-xs px-2 py-1 rounded-full backdrop-blur-sm">
          {filteredLogs.split('\n').length} lines
        </div>
      </div>
    </div>
  );
};
export default LogViewer;

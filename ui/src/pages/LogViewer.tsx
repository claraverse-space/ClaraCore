import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useAPI } from "../contexts/APIProvider";
import { usePersistentState } from "../hooks/usePersistentState";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { 
  WrapTextIcon, 
  AlignJustifyIcon, 
  TypeIcon, 
  SearchIcon, 
  XIcon,
  FilterIcon
} from "lucide-react";
import { useTheme } from "../contexts/ThemeProvider";
import { Card, CardHeader, CardTitle, CardContent, Button, Input } from "../components";
import { motion } from "framer-motion";

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
  }, [setFontSize]);

  const toggleWrapText = useCallback(() => {
    setTextWrap((prev) => !prev);
  }, [setTextWrap]);

  const toggleFilter = useCallback(() => {
    if (showFilter) {
      setShowFilter(false);
      setFilterRegex(""); // Clear filter when closing
    } else {
      setShowFilter(true);
    }
  }, [showFilter, setShowFilter]);

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

  const lineCount = useMemo(() => {
    return logData.split('\n').filter(line => line.trim()).length;
  }, [logData]);

  // auto scroll to bottom
  const preTagRef = useRef<HTMLPreElement>(null);
  useEffect(() => {
    if (!preTagRef.current) return;
    preTagRef.current.scrollTop = preTagRef.current.scrollHeight;
  }, [filteredLogs]);

  return (
    <Card className="h-full flex flex-col m-2 overflow-hidden">
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <motion.div 
              animate={{ scale: [1, 1.2, 1] }}
              transition={{ duration: 2, repeat: Infinity }}
              className="w-3 h-3 rounded-full bg-brand-500"
            />
            <CardTitle className="text-lg">{title}</CardTitle>
            <span className="text-xs text-text-tertiary bg-surface-secondary px-2 py-1 rounded-full">
              {lineCount} lines
            </span>
          </div>

          <div className="flex gap-2 items-center">
            <Button 
              variant="ghost" 
              size="sm"
              onClick={toggleFontSize}
              className="flex items-center gap-1.5"
            >
              <TypeIcon className="w-4 h-4" />
              <span className="text-xs">{fontSize.toUpperCase()}</span>
            </Button>
            <Button 
              variant="ghost" 
              size="sm"
              onClick={toggleWrapText}
              className="flex items-center gap-1.5"
            >
              {wrapText ? <WrapTextIcon className="w-4 h-4" /> : <AlignJustifyIcon className="w-4 h-4" />}
            </Button>
            <Button 
              variant={showFilter ? "primary" : "ghost"}
              size="sm"
              onClick={toggleFilter}
              className="flex items-center gap-1.5"
            >
              <FilterIcon className="w-4 h-4" />
              {showFilter && <span className="text-xs">Filter</span>}
            </Button>
          </div>
        </div>

        {/* Enhanced filtering UI */}
        {showFilter && (
          <motion.div 
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="mt-4 flex gap-3 items-center"
          >
            <Input
              placeholder="Filter logs (regex supported)..."
              value={filterRegex}
              onChange={(e) => setFilterRegex(e.target.value)}
              icon={<SearchIcon className="w-4 h-4" />}
              className="flex-1"
            />
            <Button 
              variant="ghost" 
              size="sm" 
              onClick={() => setFilterRegex("")}
              className="flex items-center gap-1.5"
            >
              <XIcon className="w-4 h-4" />
            </Button>
          </motion.div>
        )}
      </CardHeader>

      <CardContent className="flex-1 overflow-hidden p-0">
        <div className="relative h-full">
          <pre 
            ref={preTagRef} 
            className={`${textWrapClass} ${fontSizeClass} h-full overflow-auto p-6 text-text-secondary font-mono leading-relaxed bg-surface-secondary`}
          >
            {filteredLogs || "Waiting for log data..."}
          </pre>
          
          {/* Scroll indicator */}
          <div className="absolute bottom-4 right-4 bg-surface/80 text-text-tertiary text-xs px-2 py-1 rounded-full backdrop-blur-sm border border-border-secondary">
            {filteredLogs.split('\n').length} lines
          </div>
        </div>
      </CardContent>
    </Card>
  );
};
export default LogViewer;

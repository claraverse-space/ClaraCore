import { createContext, useState, useContext, useEffect, useCallback, useMemo, type ReactNode, useRef } from "react";
import type { ConnectionState } from "../lib/types";
import { Modal } from "../components/ui/Modal";
import { Button } from "../components/ui/Button";

type ModelStatus = "ready" | "starting" | "stopping" | "stopped" | "shutdown" | "unknown";
const LOG_LENGTH_LIMIT = 1024 * 100; /* 100KB of log data */

export interface Model {
  id: string;
  state: ModelStatus;
  name: string;
  description: string;
  unlisted: boolean;
  proxyUrl?: string;
}

interface APIProviderType {
  models: Model[];
  listModels: () => Promise<Model[]>;
  unloadAllModels: () => Promise<void>;
  loadModel: (model: string) => Promise<void>;
  enableAPIEvents: (enabled: boolean) => void;
  proxyLogs: string;
  upstreamLogs: string;
  metrics: Metrics[];
  connectionStatus: ConnectionState;
}

interface Metrics {
  id: number;
  timestamp: string;
  model: string;
  cache_tokens: number;
  input_tokens: number;
  output_tokens: number;
  prompt_per_second: number;
  tokens_per_second: number;
  duration_ms: number;
}

interface LogData {
  source: "upstream" | "proxy";
  data: string;
}
interface APIEventEnvelope {
  type: "modelStatus" | "logData" | "metrics";
  data: string;
}

const APIContext = createContext<APIProviderType | undefined>(undefined);
type APIProviderProps = {
  children: ReactNode;
  autoStartAPIEvents?: boolean;
};

let apiEventSource: EventSource | null = null;

export function APIProvider({ children, autoStartAPIEvents = true }: APIProviderProps) {
  const [proxyLogs, setProxyLogs] = useState("");
  const [upstreamLogs, setUpstreamLogs] = useState("");
  const [metrics, setMetrics] = useState<Metrics[]>([]);
  const [connectionStatus, setConnectionState] = useState<ConnectionState>("disconnected");
  //const apiEventSource = useRef<EventSource | null>(null);

  const [models, setModels] = useState<Model[]>([]);

  // API key modal state
  const [showKeyModal, setShowKeyModal] = useState(false);
  const [tempKey, setTempKey] = useState("");
  const keyResolvers = useRef<{ resolve: (k: string) => void; reject: (e?: any) => void }[]>([]);

  const requestApiKey = useCallback((): Promise<string> => {
    return new Promise((resolve, reject) => {
      keyResolvers.current.push({ resolve, reject });
      setShowKeyModal(true);
    });
  }, []);

  const submitApiKey = useCallback(() => {
    const k = tempKey.trim();
    if (k.length === 0) return;
    // persist
    try { localStorage.setItem("cc_api_key", k); } catch {}
    setShowKeyModal(false);
    setTempKey("");
    const pending = keyResolvers.current.splice(0, keyResolvers.current.length);
    pending.forEach(p => p.resolve(k));
  }, [tempKey]);

  const cancelApiKey = useCallback(() => {
    setShowKeyModal(false);
    setTempKey("");
    const pending = keyResolvers.current.splice(0, keyResolvers.current.length);
    pending.forEach(p => p.reject(new Error("API key entry cancelled")));
  }, []);

  const appendLog = useCallback((newData: string, setter: React.Dispatch<React.SetStateAction<string>>) => {
    setter((prev) => {
      const updatedLog = prev + newData;
      return updatedLog.length > LOG_LENGTH_LIMIT ? updatedLog.slice(-LOG_LENGTH_LIMIT) : updatedLog;
    });
  }, []);

  const enableAPIEvents = useCallback((enabled: boolean) => {
    if (!enabled) {
      apiEventSource?.close();
      apiEventSource = null;
      setMetrics([]);
      return;
    }

    let retryCount = 0;
    const initialDelay = 1000; // 1 second

    const connect = () => {
      apiEventSource?.close();
      // Attach API key via query param (headers are not supported by EventSource)
      let url = "/api/events";
      try {
        const stored = localStorage.getItem("cc_api_key");
        if (stored && stored.trim().length > 0) {
          const qp = new URLSearchParams({ api_key: stored.trim() });
          url = `/api/events?${qp.toString()}`;
        }
      } catch {}
      apiEventSource = new EventSource(url);

      setConnectionState("connecting");

      apiEventSource.onopen = () => {
        // clear everything out on connect to keep things in sync
        setProxyLogs("");
        setUpstreamLogs("");
        setMetrics([]); // clear metrics on reconnect
        setModels([]); // clear models on reconnect
        retryCount = 0;
        setConnectionState("connected");
      };

      apiEventSource.onmessage = (e: MessageEvent) => {
        try {
          const message = JSON.parse(e.data) as APIEventEnvelope;
          switch (message.type) {
            case "modelStatus":
              {
                const models = JSON.parse(message.data) as Model[];

                // sort models by name and id
                models.sort((a, b) => {
                  return (a.name + a.id).localeCompare(b.name + b.id);
                });

                setModels(models);
              }
              break;

            case "logData":
              const logData = JSON.parse(message.data) as LogData;
              switch (logData.source) {
                case "proxy":
                  appendLog(logData.data, setProxyLogs);
                  break;
                case "upstream":
                  appendLog(logData.data, setUpstreamLogs);
                  break;
              }
              break;

            case "metrics":
              {
                const newMetrics = JSON.parse(message.data) as Metrics[];
                setMetrics((prevMetrics) => {
                  return [...newMetrics, ...prevMetrics];
                });
              }
              break;
          }
        } catch (err) {
          console.error(e.data, err);
        }
      };

      apiEventSource.onerror = () => {
        apiEventSource?.close();
        retryCount++;
        const delay = Math.min(initialDelay * Math.pow(2, retryCount - 1), 5000);
        setConnectionState("disconnected");
        setTimeout(connect, delay);
      };
    };

    connect();
  }, []);

  useEffect(() => {
    // Wrap global fetch to attach API key and react to 401s
    const origFetch = window.fetch.bind(window);
    window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
      // Normalize URL
      let urlStr: string;
      if (typeof input === "string") urlStr = input;
      else if (input instanceof URL) urlStr = input.toString();
      else urlStr = input.url;

      let needAuth = false;
      try {
        const u = new URL(urlStr, window.location.origin);
        const isSameOrigin = u.origin === window.location.origin;
        const isBackendPath = u.pathname.startsWith("/api") || u.pathname.startsWith("/v1") || u.pathname.startsWith("/upstream");
        needAuth = isSameOrigin && isBackendPath;
      } catch {
        // Relative paths will resolve above; if something goes wrong, default to not attaching auth
        needAuth = false;
      }

      let headers = new Headers((init && init.headers) || undefined);
      if (needAuth && !headers.has("Authorization") && !headers.has("X-API-Key")) {
        try {
          const stored = localStorage.getItem("cc_api_key");
          if (stored && stored.trim().length > 0) {
            headers.set("Authorization", `Bearer ${stored.trim()}`);
          }
        } catch {}
      }

      const res = await origFetch(input as any, { ...(init || {}), headers });
      if (res.status !== 401 && res.status !== 403) return res;

      // 401/403 → prompt for key, retry once (only for backend endpoints)
      if (!needAuth) return res;
      try {
        const key = await requestApiKey();
        const retryHeaders = new Headers((init && init.headers) || undefined);
        retryHeaders.set("Authorization", `Bearer ${key}`);
        return await origFetch(input as any, { ...(init || {}), headers: retryHeaders });
      } catch {
        return res; // user cancelled; return original response
      }
    };

    if (autoStartAPIEvents) {
      enableAPIEvents(true);
    }

    return () => {
      enableAPIEvents(false);
      // restore? not strictly necessary in SPA lifecycle, but keep safety
      window.fetch = origFetch;
    };
  }, [enableAPIEvents, autoStartAPIEvents, requestApiKey]);

  const listModels = useCallback(async (): Promise<Model[]> => {
    try {
      const response = await fetch("/api/models/");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      return data || [];
    } catch (error) {
      console.error("Failed to fetch models:", error);
      return []; // Return empty array as fallback
    }
  }, []);

  const unloadAllModels = useCallback(async () => {
    try {
      const response = await fetch(`/api/models/unload/`, {
        method: "POST",
      });
      if (!response.ok) {
        throw new Error(`Failed to unload models: ${response.status}`);
      }
    } catch (error) {
      console.error("Failed to unload models:", error);
      throw error; // Re-throw to let calling code handle it
    }
  }, []);

  const loadModel = useCallback(async (model: string) => {
    try {
      const response = await fetch(`/upstream/${model}/`, {
        method: "GET",
      });
      if (!response.ok) {
        throw new Error(`Failed to load model: ${response.status}`);
      }
    } catch (error) {
      console.error("Failed to load model:", error);
      throw error; // Re-throw to let calling code handle it
    }
  }, []);

  const value = useMemo(
    () => ({
      models,
      listModels,
      unloadAllModels,
      loadModel,
      enableAPIEvents,
      proxyLogs,
      upstreamLogs,
      metrics,
      connectionStatus,
    }),
    [models, listModels, unloadAllModels, loadModel, enableAPIEvents, proxyLogs, upstreamLogs, metrics]
  );
  return (
    <APIContext.Provider value={value}>
      {children}
      {/* API Key Modal */}
      <Modal open={showKeyModal} onClose={cancelApiKey} title="API Key Required" description="Enter the API key to access ClaraCore endpoints.">
        <div className="space-y-3">
          <input
            type="password"
            placeholder="Enter API key"
            value={tempKey}
            onChange={(e) => setTempKey(e.target.value)}
            className="w-full p-2 rounded border border-border-secondary bg-background text-text-primary"
          />
          <div className="flex gap-2 justify-end">
            <Button variant="outline" onClick={cancelApiKey}>Cancel</Button>
            <Button onClick={submitApiKey}>Submit</Button>
          </div>
        </div>
      </Modal>
    </APIContext.Provider>
  );
}

export function APIProviderWithAuthModal(props: APIProviderProps) {
  return (
    <APIProvider {...props}>
      {props.children}
    </APIProvider>
  );
}

// We render the modal inside APIProvider component tree:
// Note: Injecting modal here by patching return above is messy; simpler to append alongside children.

export function useAPI() {
  const context = useContext(APIContext);
  if (context === undefined) {
    throw new Error("useAPI must be used within an APIProvider");
  }
  return context;
}

import { useMemo } from "react";
import { useAPI } from "../contexts/APIProvider";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, Table } from "../components";
import { ActivityIcon, ClockIcon, CpuIcon, ZapIcon } from "lucide-react";
import { motion } from "framer-motion";

const formatSpeed = (speed: number): string => {
  return speed < 0 ? "unknown" : speed.toFixed(2) + " t/s";
};

const formatDuration = (ms: number): string => {
  return (ms / 1000).toFixed(2) + "s";
};

const formatRelativeTime = (timestamp: string): string => {
  const now = new Date();
  const date = new Date(timestamp);
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  // Handle future dates by returning "just now"
  if (diffInSeconds < 5) {
    return "now";
  }

  if (diffInSeconds < 60) {
    return `${diffInSeconds}s ago`;
  }

  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes}m ago`;
  }

  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours}h ago`;
  }

  return "a while ago";
};

const ActivityPage = () => {
  const { metrics } = useAPI();
  
  const sortedMetrics = useMemo(() => {
    return [...metrics].sort((a, b) => b.id - a.id);
  }, [metrics]);

  const columns = [
    { 
      key: 'id', 
      title: 'ID', 
      dataIndex: 'id' as const,
      render: (id: number) => id + 1, // un-zero index
      width: 80,
    },
    { 
      key: 'timestamp', 
      title: 'Time', 
      dataIndex: 'timestamp' as const,
      render: (timestamp: string) => (
        <div className="flex items-center gap-2">
          <ClockIcon className="w-4 h-4 text-text-tertiary" />
          {formatRelativeTime(timestamp)}
        </div>
      ),
    },
    { 
      key: 'model', 
      title: 'Model', 
      dataIndex: 'model' as const,
      render: (model: string) => (
        <div className="flex items-center gap-2">
          <CpuIcon className="w-4 h-4 text-brand-500" />
          <span className="font-medium text-brand-600 dark:text-brand-400">{model}</span>
        </div>
      ),
    },
    { 
      key: 'cache_tokens', 
      title: (
        <div className="flex items-center gap-1">
          Cached <Tooltip content="prompt tokens from cache" />
        </div>
      ), 
      dataIndex: 'cache_tokens' as const,
      render: (tokens: number) => tokens > 0 ? tokens.toLocaleString() : "-",
      align: 'right' as const,
    },
    { 
      key: 'input_tokens', 
      title: (
        <div className="flex items-center gap-1">
          Prompt <Tooltip content="new prompt tokens processed" />
        </div>
      ), 
      dataIndex: 'input_tokens' as const,
      render: (tokens: number) => tokens.toLocaleString(),
      align: 'right' as const,
    },
    { 
      key: 'output_tokens', 
      title: 'Generated', 
      dataIndex: 'output_tokens' as const,
      render: (tokens: number) => tokens.toLocaleString(),
      align: 'right' as const,
    },
    { 
      key: 'prompt_per_second', 
      title: 'Prompt Processing', 
      dataIndex: 'prompt_per_second' as const,
      render: (speed: number) => formatSpeed(speed),
      align: 'right' as const,
    },
    { 
      key: 'tokens_per_second', 
      title: (
        <div className="flex items-center gap-1">
          <ZapIcon className="w-4 h-4" />
          Generation Speed
        </div>
      ), 
      dataIndex: 'tokens_per_second' as const,
      render: (speed: number) => formatSpeed(speed),
      align: 'right' as const,
    },
    { 
      key: 'duration_ms', 
      title: 'Duration', 
      dataIndex: 'duration_ms' as const,
      render: (ms: number) => formatDuration(ms),
      align: 'right' as const,
    },
  ];

  return (
    <div className="p-6 space-y-6">
      {/* Page Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center gap-4"
      >
        <div className="w-10 h-10 bg-gradient-to-br from-brand-500 to-brand-600 rounded-lg flex items-center justify-center">
          <ActivityIcon className="w-5 h-5 text-white" />
        </div>
        <div>
          <h1 className="text-3xl font-bold text-text-primary">Activity</h1>
          <p className="text-text-secondary">Monitor request performance and token usage</p>
        </div>
      </motion.div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-6">
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-brand-100 dark:bg-brand-900/20 rounded-lg flex items-center justify-center">
                <ActivityIcon className="w-6 h-6 text-brand-600 dark:text-brand-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-text-primary">{metrics.length}</p>
                <p className="text-sm text-text-secondary">Total Requests</p>
              </div>
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-success-100 dark:bg-success-900/20 rounded-lg flex items-center justify-center">
                <ZapIcon className="w-6 h-6 text-success-600 dark:text-success-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-text-primary">
                  {metrics.length > 0 ? (metrics.reduce((sum, m) => sum + m.tokens_per_second, 0) / metrics.length).toFixed(1) : '0'}
                </p>
                <p className="text-sm text-text-secondary">Avg Speed (t/s)</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-info-100 dark:bg-info-900/20 rounded-lg flex items-center justify-center">
                <CpuIcon className="w-6 h-6 text-info-600 dark:text-info-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-text-primary">
                  {new Intl.NumberFormat('en', { notation: 'compact' }).format(
                    metrics.reduce((sum, m) => sum + m.input_tokens, 0)
                  )}
                </p>
                <p className="text-sm text-text-secondary">Input Tokens</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-warning-100 dark:bg-warning-900/20 rounded-lg flex items-center justify-center">
                <ClockIcon className="w-6 h-6 text-warning-600 dark:text-warning-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-text-primary">
                  {new Intl.NumberFormat('en', { notation: 'compact' }).format(
                    metrics.reduce((sum, m) => sum + m.output_tokens, 0)
                  )}
                </p>
                <p className="text-sm text-text-secondary">Output Tokens</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Activity Table */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
          <CardDescription>
            Latest model inference requests and performance metrics
          </CardDescription>
        </CardHeader>
        <CardContent>
          {metrics.length === 0 ? (
            <motion.div 
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="text-center py-12"
            >
              <div className="w-16 h-16 bg-surface-secondary rounded-full mx-auto mb-4 flex items-center justify-center">
                <ActivityIcon className="w-8 h-8 text-text-tertiary" />
              </div>
              <p className="text-text-secondary mb-2">No activity data available</p>
              <p className="text-text-tertiary text-sm">Metrics will appear here after model requests</p>
            </motion.div>
          ) : (
            <Table 
              data={sortedMetrics}
              columns={columns}
              pagination={{
                current: 1,
                pageSize: 20,
                total: sortedMetrics.length,
                onChange: (page, pageSize) => console.log('Page changed:', page, pageSize)
              }}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
};

interface TooltipProps {
  content: string;
}

const Tooltip: React.FC<TooltipProps> = ({ content }) => {
  return (
    <div className="relative group inline-block">
      <span className="text-gray-400 hover:text-gray-300 cursor-help">ⓘ</span>
      <div
        className="absolute top-full left-1/2 transform -translate-x-1/2 mt-2
                     px-3 py-2 bg-gray-800 text-white text-sm rounded-md
                     opacity-0 group-hover:opacity-100 transition-opacity
                     duration-200 pointer-events-none whitespace-nowrap z-50 normal-case
                     border border-gray-600"
      >
        {content}
        <div
          className="absolute bottom-full left-1/2 transform -translate-x-1/2
                       border-4 border-transparent border-b-gray-800"
        ></div>
      </div>
    </div>
  );
};

export default ActivityPage;

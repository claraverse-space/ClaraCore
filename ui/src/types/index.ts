// ===== CORE TYPES =====

export interface SystemInfo {
  os: string;
  arch: string;
  cpuCores: number;
  totalRAMGB: number;
  totalVRAMGB: number;
  gpus: GPUInfo[];
  backend: BackendType;
}

export interface GPUInfo {
  id: number;
  name: string;
  vramGB: number;
  computeCapability?: string;
  driver?: string;
  brand: 'nvidia' | 'amd' | 'intel' | 'apple' | 'unknown';
}

export interface SystemDetection {
  detectionQuality: 'excellent' | 'good' | 'basic' | 'minimal';
  platform: string;
  arch: string;
  gpuDetected: boolean;
  gpuTypes: string[];
  primaryGPU: {
    name: string;
    brand: 'nvidia' | 'amd' | 'intel' | 'apple' | 'unknown';
    vramGB: number;
    computeCapability?: string;
  } | null;
  totalRAMGB: number;
  availableRAMGB: number;
  recommendedBackends: string[];
  supportedBackends: string[];
  backendCapabilities: Record<string, {
    available: boolean;
    priority: number;
    requirements?: string[];
    limitations?: string[];
  }>;
  recommendedContextSizes: number[];
  maxRecommendedContextSize: number;
  memoryEstimates: {
    backend: string;
    contextSize: number;
    estimatedVRAM: number;
    estimatedRAM: number;
    feasible: boolean;
  }[];
  recommendations: {
    primaryBackend: string;
    fallbackBackend: string;
    suggestedContextSize: number;
    suggestedVRAMAllocation: number;
    suggestedRAMAllocation: number;
    throughputFirst: boolean;
    notes: string[];
  };
  detectionTimestamp: string;
}

export type BackendType = 'cuda' | 'rocm' | 'vulkan' | 'metal' | 'mlx' | 'cpu';

// ===== MODEL TYPES =====

export interface Model {
  id: string;
  name: string;
  description?: string;
  path: string;
  size: number; // in bytes
  sizeGB: number; // in GB
  architecture: string;
  quantization?: string;
  contextLength?: number;
  layers?: number;
  parameters?: string; // e.g., "7B", "13B"
  state: ModelState;
  unlisted?: boolean;
  isDraft?: boolean;
  isEmbedding?: boolean;
  metadata?: ModelMetadata;
  config?: ModelConfig;
  performance?: ModelPerformance;
  tags?: string[];
  license?: string;
  source?: ModelSource;
  createdAt?: string;
  updatedAt?: string;
}

export type ModelState = 'stopped' | 'loading' | 'ready' | 'error' | 'downloading' | 'converting';

export interface ModelMetadata {
  author?: string;
  version?: string;
  baseModel?: string;
  finetune?: string;
  dataset?: string;
  language?: string[];
  tasks?: string[];
  homepage?: string;
  repository?: string;
  paperUrl?: string;
  citation?: string;
  limitations?: string;
  bias?: string;
  intendedUse?: string;
}

export interface ModelConfig {
  contextSize: number;
  ngl: number; // GPU layers
  kvCacheType: 'f16' | 'q8_0' | 'q4_0';
  useMLock: boolean;
  threads?: number;
  batchSize?: number;
  temperature?: number;
  topP?: number;
  topK?: number;
  repeatPenalty?: number;
  systemPrompt?: string;
  stopTokens?: string[];
}

export interface ModelPerformance {
  tokensPerSecond?: number;
  firstTokenLatency?: number; // ms
  memoryUsage?: number; // bytes
  vramUsage?: number; // bytes
  lastUpdated?: string;
  benchmarkResults?: BenchmarkResult[];
}

export interface BenchmarkResult {
  name: string;
  score: number;
  unit: string;
  timestamp: string;
  config: Partial<ModelConfig>;
}

export interface ModelSource {
  type: 'local' | 'huggingface' | 'url' | 'git';
  url?: string;
  repository?: string;
  branch?: string;
  revision?: string;
  originalFormat?: string;
}

// ===== HUGGINGFACE TYPES =====

export interface HuggingFaceModel {
  id: string;
  author: string;
  modelName: string;
  description?: string;
  downloads: number;
  likes: number;
  tags: string[];
  pipeline_tag?: string;
  library_name?: string;
  license?: string;
  created_at: string;
  last_modified: string;
  card_data?: {
    language?: string[];
    datasets?: string[];
    metrics?: string[];
    base_model?: string;
  };
  siblings: HuggingFaceFile[];
  spaces?: string[];
  safetensors?: boolean;
  gated?: boolean;
  disabled?: boolean;
}

export interface HuggingFaceFile {
  rfilename: string;
  size?: number;
  blob_id?: string;
  lfs?: boolean;
}

export interface HuggingFaceSearchParams {
  query?: string;
  author?: string;
  filter?: string;
  sort?: 'downloads' | 'likes' | 'updated' | 'created';
  direction?: 'asc' | 'desc';
  limit?: number;
  offset?: number;
  tags?: string[];
  pipeline_tag?: string;
  library?: string[];
}

export interface HuggingFaceSearchResult {
  models: HuggingFaceModel[];
  total: number;
  hasMore: boolean;
}

// ===== DOWNLOAD TYPES =====

export interface DownloadTask {
  id: string;
  modelId: string;
  fileName: string;
  url: string;
  status: DownloadStatus;
  progress: number; // 0-100
  downloadedBytes: number;
  totalBytes: number;
  speed: number; // bytes per second
  startTime: string;
  endTime?: string;
  error?: string;
  priority: 'high' | 'normal' | 'low';
  canPause: boolean;
  canRetry: boolean;
}

export type DownloadStatus = 'pending' | 'downloading' | 'paused' | 'completed' | 'failed' | 'cancelled';

export interface ConversionTask {
  id: string;
  modelId: string;
  inputPath: string;
  outputPath: string;
  format: 'gguf' | 'safetensors' | 'pytorch';
  quantization?: string;
  status: ConversionStatus;
  progress: number;
  startTime: string;
  endTime?: string;
  error?: string;
  settings: ConversionSettings;
}

export type ConversionStatus = 'pending' | 'converting' | 'completed' | 'failed' | 'cancelled';

export interface ConversionSettings {
  quantization: string;
  contextLength?: number;
  vocabulary?: 'spm' | 'bpe';
  addBosToken?: boolean;
  addEosToken?: boolean;
  splitTensors?: boolean;
}

// ===== PERFORMANCE TYPES =====

export interface SystemMetrics {
  timestamp: string;
  cpu: {
    usage: number; // percentage
    temperature?: number; // celsius
    frequency?: number; // MHz
  };
  memory: {
    used: number; // bytes
    total: number; // bytes
    usage: number; // percentage
  };
  gpu?: GPUMetrics[];
  disk: {
    used: number; // bytes
    total: number; // bytes
    usage: number; // percentage
    readSpeed?: number; // bytes/s
    writeSpeed?: number; // bytes/s
  };
}

export interface GPUMetrics {
  id: number;
  name: string;
  usage: number; // percentage
  memoryUsed: number; // bytes
  memoryTotal: number; // bytes
  memoryUsage: number; // percentage
  temperature?: number; // celsius
  powerUsage?: number; // watts
  frequency?: number; // MHz
}

export interface RequestMetrics {
  id: string;
  modelId: string;
  timestamp: string;
  duration: number; // ms
  inputTokens: number;
  outputTokens: number;
  tokensPerSecond: number;
  firstTokenLatency: number; // ms
  memoryUsage: number; // bytes
  success: boolean;
  error?: string;
}

// ===== API TYPES =====

export interface APIResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  timestamp: string;
}

export interface APIError {
  code: string;
  message: string;
  details?: any;
  timestamp: string;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

// ===== UI TYPES =====

export interface Theme {
  mode: 'light' | 'dark' | 'system';
  primaryColor: string;
  accentColor: string;
  fontScale: number;
  compactMode: boolean;
  animations: boolean;
}

export interface UIPreferences {
  theme: Theme;
  sidebarCollapsed: boolean;
  showAdvancedOptions: boolean;
  defaultModelView: 'cards' | 'list' | 'table';
  autoRefresh: boolean;
  refreshInterval: number; // seconds
  notifications: NotificationSettings;
}

export interface NotificationSettings {
  enabled: boolean;
  modelLoad: boolean;
  downloads: boolean;
  errors: boolean;
  performance: boolean;
  position: 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left';
  duration: number; // ms
}

export interface Toast {
  id: string;
  type: 'success' | 'error' | 'warning' | 'info';
  title: string;
  message?: string;
  duration?: number;
  action?: {
    label: string;
    handler: () => void;
  };
}

// ===== CONFIGURATION TYPES =====

export interface AppConfig {
  version: string;
  apiEndpoint: string;
  modelsPath: string;
  binariesPath: string;
  logsPath: string;
  maxConcurrentDownloads: number;
  defaultContextSize: number;
  autoOptimize: boolean;
  telemetry: boolean;
  experimental: ExperimentalFeatures;
}

export interface ExperimentalFeatures {
  speculativeDecoding: boolean;
  parallelProcessing: boolean;
  advancedQuantization: boolean;
  cloudIntegration: boolean;
  autoScaling: boolean;
}

// ===== LOG TYPES =====

export interface LogEntry {
  id: string;
  timestamp: string;
  level: 'debug' | 'info' | 'warn' | 'error';
  source: string;
  message: string;
  metadata?: Record<string, any>;
}

export interface LogFilter {
  level?: string[];
  source?: string[];
  dateRange?: {
    start: string;
    end: string;
  };
  search?: string;
}

// ===== FORM TYPES =====

export interface FormField<T = any> {
  name: string;
  label: string;
  type: 'text' | 'number' | 'select' | 'checkbox' | 'textarea' | 'file' | 'range';
  value: T;
  placeholder?: string;
  description?: string;
  required?: boolean;
  disabled?: boolean;
  options?: { label: string; value: any }[];
  validation?: {
    min?: number;
    max?: number;
    pattern?: string;
    custom?: (value: T) => string | null;
  };
}

export interface FormState {
  values: Record<string, any>;
  errors: Record<string, string>;
  touched: Record<string, boolean>;
  isSubmitting: boolean;
  isValid: boolean;
}

// ===== UTILITY TYPES =====

export type DeepPartial<T> = {
  [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
};

export type RequiredKeys<T, K extends keyof T> = T & {
  [P in K]-?: T[P];
};

export type OptionalKeys<T, K extends keyof T> = Omit<T, K> & {
  [P in K]?: T[P];
};

export type Nullable<T> = T | null;

export type Optional<T> = T | undefined;

// ===== COMPONENT PROPS TYPES =====

export interface BaseComponentProps {
  className?: string;
  id?: string;
  testId?: string;
  children?: React.ReactNode;
}

export interface ButtonProps extends BaseComponentProps {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'outline';
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  loading?: boolean;
  icon?: React.ReactNode;
  onClick?: () => void;
  type?: 'button' | 'submit' | 'reset';
}

export interface ModalProps extends BaseComponentProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  description?: string;
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full';
  closeOnOverlayClick?: boolean;
  closeOnEscape?: boolean;
}

export interface CardProps extends BaseComponentProps {
  variant?: 'default' | 'elevated' | 'outlined' | 'ghost';
  padding?: 'none' | 'sm' | 'md' | 'lg';
  hover?: boolean;
}

export interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  error?: string;
  success?: string;
  helper?: string;
  size?: 'sm' | 'md' | 'lg';
  icon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export interface TableColumn<T = any> {
  key: string;
  title: string | React.ReactNode;
  dataIndex?: keyof T;
  render?: (value: any, record: T, index: number) => React.ReactNode;
  sortable?: boolean;
  width?: string | number;
  align?: 'left' | 'center' | 'right';
  fixed?: 'left' | 'right';
}

export interface TableProps<T = any> extends BaseComponentProps {
  data: T[];
  columns: TableColumn<T>[];
  loading?: boolean;
  sortable?: boolean;
  pagination?: {
    current: number;
    pageSize: number;
    total: number;
    onChange: (page: number, pageSize: number) => void;
  };
  selection?: {
    selectedKeys: string[];
    onChange: (selectedKeys: string[]) => void;
  };
  onRow?: (record: T, index: number) => Record<string, any>;
}
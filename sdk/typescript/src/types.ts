// Media types
export type MediaStatus = 'uploading' | 'processing' | 'ready' | 'error';

export interface Media {
  id: string;
  originalFilename: string;
  contentType: string;
  size: number;
  gcsSourcePath: string;
  gcsProxyPath?: string;
  status: MediaStatus;
  error?: string;
  createdAt: string;
}

export interface UploadResponse {
  id: string;
  filename: string;
  size: number;
  status: MediaStatus;
  gcsPath: string;
}

export interface SignedUrlResponse {
  url: string;
}

// Job types
export type JobType = 'proxy_generation' | 'export_render';
export type JobStatus = 'queued' | 'downloading' | 'rendering' | 'uploading' | 'complete' | 'failed';

export interface Job {
  id: string;
  type: JobType;
  sessionId?: string;
  mediaId?: string;
  status: JobStatus;
  progress: number;
  fps?: number;
  speed?: string;
  eta?: number;
  stage?: string;
  resultUrl?: string;
  error?: string;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
}

export interface SessionJobsResponse {
  jobs: Job[];
}

// Error types
export interface AppError {
  code: string;
  message: string;
  retryable?: boolean;
  details?: Record<string, unknown>;
}

export interface ErrorResponse {
  error: AppError;
}

// EDL types
export interface Filter {
  type: string;
  params?: Record<string, unknown>;
}

export interface Clip {
  mediaId: string;
  startTime: number;
  duration: number;
  inPoint: number;
  outPoint: number;
  filters?: Filter[];
}

export interface Track {
  id: string;
  type: 'video' | 'audio';
  clips: Clip[];
}

export interface OutputSettings {
  format: 'mp4' | 'webm';
  quality: 'draft' | 'preview' | 'final';
  resolution?: { width: number; height: number };
}

export interface EDL {
  version: '1.0';
  projectId: string;
  timeline: {
    duration: number;
    tracks: Track[];
  };
  output: OutputSettings;
}

// WebSocket message types
export type MessageType =
  | 'ping'
  | 'pong'
  | 'edl.submit'
  | 'edl.ack'
  | 'job.progress'
  | 'job.complete'
  | 'job.error'
  | 'media.status';

export interface WSMessage<T = unknown> {
  type: MessageType;
  id?: string;
  payload?: T;
}

export interface ProgressPayload {
  jobId: string;
  percent: number;
  fps?: number;
  speed?: string;
  eta?: number;
  stage: string;
}

export interface CompletePayload {
  jobId: string;
  url: string;
}

export interface WSErrorPayload {
  code: string;
  message: string;
  jobId?: string;
  retryable: boolean;
}

export interface MediaStatusPayload {
  mediaId: string;
  status: MediaStatus;
  error?: string;
}

export interface EDLAckPayload {
  projectId: string;
  jobId: string;
}

// Client config
export interface ClientConfig {
  baseUrl: string;
  wsUrl?: string;
  timeout?: number;
}

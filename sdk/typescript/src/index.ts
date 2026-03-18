export { CloudCutClient, CloudCutError } from './client';
export { CloudCutWS } from './ws';
export type {
  // Client config
  ClientConfig,

  // Media types
  Media,
  MediaStatus,
  UploadResponse,
  SignedUrlResponse,

  // Job types
  Job,
  JobType,
  JobStatus,
  SessionJobsResponse,

  // Error types
  AppError,
  ErrorResponse,

  // EDL types
  EDL,
  Track,
  Clip,
  Filter,
  OutputSettings,

  // WebSocket types
  WSMessage,
  MessageType,
  ProgressPayload,
  CompletePayload,
  WSErrorPayload,
  MediaStatusPayload,
  EDLAckPayload,
} from './types';

export type { WSEventMap } from './ws';

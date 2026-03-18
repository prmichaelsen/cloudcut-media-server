import type {
  ClientConfig,
  Media,
  UploadResponse,
  SignedUrlResponse,
  Job,
  SessionJobsResponse,
  ErrorResponse,
  AppError,
} from './types';

export class CloudCutError extends Error {
  code: string;
  retryable: boolean;
  details?: Record<string, unknown>;

  constructor(appError: AppError) {
    super(appError.message);
    this.name = 'CloudCutError';
    this.code = appError.code;
    this.retryable = appError.retryable ?? false;
    this.details = appError.details;
  }
}

/**
 * CloudCut Media Server REST API client.
 *
 * @example
 * ```ts
 * const client = new CloudCutClient({ baseUrl: 'https://your-server.run.app' });
 *
 * // Upload a video
 * const media = await client.uploadMedia(file);
 *
 * // Check status
 * const status = await client.getMedia(media.id);
 *
 * // Get signed URL for proxy
 * const { url } = await client.getProxyUrl(media.id);
 * ```
 */
export class CloudCutClient {
  private baseUrl: string;
  private timeout: number;

  constructor(config: ClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, '');
    this.timeout = config.timeout ?? 30000;
  }

  /**
   * Check server health.
   */
  async health(): Promise<{ status: string }> {
    return this.get('/health');
  }

  /**
   * Upload a media file.
   *
   * @param file - File or Blob to upload
   * @param filename - Optional filename override
   * @returns Upload response with media ID and status
   */
  async uploadMedia(file: Blob | File, filename?: string): Promise<UploadResponse> {
    const formData = new FormData();
    const name = filename ?? (file instanceof File ? file.name : 'upload.mp4');
    formData.append('file', file, name);

    const response = await this.fetch('/api/v1/media/upload', {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      await this.handleError(response);
    }

    return response.json();
  }

  /**
   * Get media details by ID.
   */
  async getMedia(mediaId: string): Promise<Media> {
    return this.get(`/api/v1/media/${mediaId}`);
  }

  /**
   * Get signed URL for the original source file.
   */
  async getSourceUrl(mediaId: string): Promise<SignedUrlResponse> {
    return this.get(`/api/v1/media/${mediaId}/url`);
  }

  /**
   * Get signed URL for the proxy file.
   */
  async getProxyUrl(mediaId: string): Promise<SignedUrlResponse> {
    return this.get(`/api/v1/media/${mediaId}/proxy/url`);
  }

  /**
   * Get job status by ID.
   */
  async getJob(jobId: string): Promise<Job> {
    return this.get(`/api/v1/jobs/${jobId}`);
  }

  /**
   * List all jobs for a session.
   */
  async getSessionJobs(sessionId: string): Promise<SessionJobsResponse> {
    return this.get(`/api/v1/sessions/${sessionId}/jobs`);
  }

  /**
   * Poll a job until it reaches a terminal status (complete or failed).
   *
   * @param jobId - Job ID to poll
   * @param intervalMs - Polling interval in milliseconds (default: 1000)
   * @param onProgress - Optional callback for progress updates
   * @returns The completed job
   */
  async waitForJob(
    jobId: string,
    intervalMs = 1000,
    onProgress?: (job: Job) => void
  ): Promise<Job> {
    const terminalStatuses = new Set(['complete', 'failed']);

    while (true) {
      const job = await this.getJob(jobId);

      if (onProgress) {
        onProgress(job);
      }

      if (terminalStatuses.has(job.status)) {
        if (job.status === 'failed') {
          throw new CloudCutError({
            code: 'JOB_FAILED',
            message: job.error ?? 'Job failed',
            retryable: false,
          });
        }
        return job;
      }

      await new Promise((resolve) => setTimeout(resolve, intervalMs));
    }
  }

  /**
   * Wait for media to reach 'ready' status.
   *
   * @param mediaId - Media ID to poll
   * @param intervalMs - Polling interval in milliseconds (default: 2000)
   * @returns The ready media object
   */
  async waitForMedia(mediaId: string, intervalMs = 2000): Promise<Media> {
    const terminalStatuses = new Set(['ready', 'error']);

    while (true) {
      const media = await this.getMedia(mediaId);

      if (terminalStatuses.has(media.status)) {
        if (media.status === 'error') {
          throw new CloudCutError({
            code: 'MEDIA_PROCESSING_FAILED',
            message: media.error ?? 'Media processing failed',
            retryable: false,
          });
        }
        return media;
      }

      await new Promise((resolve) => setTimeout(resolve, intervalMs));
    }
  }

  private async get<T>(path: string): Promise<T> {
    const response = await this.fetch(path, { method: 'GET' });

    if (!response.ok) {
      await this.handleError(response);
    }

    return response.json();
  }

  private async fetch(path: string, init: RequestInit): Promise<Response> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      return await globalThis.fetch(`${this.baseUrl}${path}`, {
        ...init,
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timeoutId);
    }
  }

  private async handleError(response: Response): Promise<never> {
    let errorBody: ErrorResponse;
    try {
      errorBody = await response.json();
    } catch {
      throw new CloudCutError({
        code: 'UNKNOWN_ERROR',
        message: `HTTP ${response.status}: ${response.statusText}`,
        retryable: response.status >= 500,
      });
    }

    throw new CloudCutError(errorBody.error);
  }
}

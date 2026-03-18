import type {
  WSMessage,
  EDL,
  ProgressPayload,
  CompletePayload,
  WSErrorPayload,
  MediaStatusPayload,
  EDLAckPayload,
} from './types';

export type WSEventMap = {
  open: () => void;
  close: (code: number, reason: string) => void;
  error: (error: Event) => void;
  progress: (payload: ProgressPayload) => void;
  complete: (payload: CompletePayload) => void;
  'job.error': (payload: WSErrorPayload) => void;
  'media.status': (payload: MediaStatusPayload) => void;
  'edl.ack': (payload: EDLAckPayload) => void;
  message: (message: WSMessage) => void;
};

/**
 * WebSocket client for CloudCut Media Server.
 *
 * @example
 * ```ts
 * const ws = new CloudCutWS('wss://your-server.run.app/ws');
 *
 * ws.on('open', () => {
 *   ws.submitEDL(myEdl);
 * });
 *
 * ws.on('progress', (payload) => {
 *   console.log(`${payload.percent}% - ${payload.stage}`);
 * });
 *
 * ws.on('complete', (payload) => {
 *   console.log('Download:', payload.url);
 * });
 *
 * ws.connect();
 * ```
 */
export class CloudCutWS {
  private url: string;
  private ws: WebSocket | null = null;
  private sessionId: string | null = null;
  private listeners: Map<string, Set<Function>> = new Map();
  private pingInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private autoReconnect = true;
  private msgCounter = 0;

  constructor(url: string, options?: { autoReconnect?: boolean; maxReconnectAttempts?: number }) {
    this.url = url;
    if (options?.autoReconnect !== undefined) this.autoReconnect = options.autoReconnect;
    if (options?.maxReconnectAttempts !== undefined) this.maxReconnectAttempts = options.maxReconnectAttempts;
  }

  /**
   * Connect to the WebSocket server.
   * If a session ID is known from a previous connection, it will attempt to reconnect.
   */
  connect(): void {
    const wsUrl = this.sessionId ? `${this.url}?session_id=${this.sessionId}` : this.url;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.startPing();
      this.emit('open');
    };

    this.ws.onclose = (event) => {
      this.stopPing();
      this.emit('close', event.code, event.reason);

      if (this.autoReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
        setTimeout(() => this.connect(), Math.min(delay, 30000));
      }
    };

    this.ws.onerror = (event) => {
      this.emit('error', event);
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        this.handleMessage(msg);
      } catch {
        // Ignore malformed messages
      }
    };
  }

  /**
   * Disconnect from the WebSocket server.
   */
  disconnect(): void {
    this.autoReconnect = false;
    this.stopPing();
    if (this.ws) {
      this.ws.close(1000, 'client disconnect');
      this.ws = null;
    }
  }

  /**
   * Submit an EDL for server-side rendering.
   *
   * @param edl - Edit Decision List to render
   * @returns Message ID for correlation
   */
  submitEDL(edl: EDL): string {
    const id = `edl-${++this.msgCounter}`;
    this.send({
      type: 'edl.submit',
      id,
      payload: edl as unknown as Record<string, unknown>,
    });
    return id;
  }

  /**
   * Send a ping message to keep the connection alive.
   */
  ping(): void {
    this.send({
      type: 'ping',
      id: `ping-${++this.msgCounter}`,
    });
  }

  /**
   * Register an event listener.
   */
  on<K extends keyof WSEventMap>(event: K, listener: WSEventMap[K]): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(listener);
  }

  /**
   * Remove an event listener.
   */
  off<K extends keyof WSEventMap>(event: K, listener: WSEventMap[K]): void {
    this.listeners.get(event)?.delete(listener);
  }

  /**
   * Get the current session ID (assigned by server on first connection).
   */
  getSessionId(): string | null {
    return this.sessionId;
  }

  /**
   * Check if the WebSocket is connected.
   */
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private send(msg: WSMessage): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected');
    }
    this.ws.send(JSON.stringify(msg));
  }

  private handleMessage(msg: WSMessage): void {
    // Emit raw message
    this.emit('message', msg);

    switch (msg.type) {
      case 'pong':
        break;
      case 'edl.ack':
        this.emit('edl.ack', msg.payload as EDLAckPayload);
        break;
      case 'job.progress':
        this.emit('progress', msg.payload as ProgressPayload);
        break;
      case 'job.complete':
        this.emit('complete', msg.payload as CompletePayload);
        break;
      case 'job.error':
        this.emit('job.error', msg.payload as WSErrorPayload);
        break;
      case 'media.status':
        this.emit('media.status', msg.payload as MediaStatusPayload);
        break;
    }
  }

  private emit(event: string, ...args: unknown[]): void {
    const listeners = this.listeners.get(event);
    if (listeners) {
      for (const listener of listeners) {
        try {
          listener(...args);
        } catch {
          // Don't let listener errors break the event loop
        }
      }
    }
  }

  private startPing(): void {
    this.stopPing();
    this.pingInterval = setInterval(() => this.ping(), 30000);
  }

  private stopPing(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }
}

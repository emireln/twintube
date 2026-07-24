/* TwinTube - WebSocket Client Engine */

export class WSClient {
  constructor() {
    this.ws = null;
    this.handlers = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 10;
    this.isConnecting = false;
  }

  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL = `${protocol}//${window.location.host}/ws`;

    this.isConnecting = true;
    console.log('[WS] Connecting to', wsURL);
    this.ws = new WebSocket(wsURL);

    this.ws.onopen = () => {
      console.log('[WS] Connected successfully.');
      this.isConnecting = false;
      this.reconnectAttempts = 0;
      this.trigger('open');
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.action) {
          let payload = msg.payload;
          if (typeof payload === 'string') {
            try { payload = JSON.parse(payload); } catch (e) {}
          }
          this.trigger(msg.action, payload, msg.timestamp);
        }
      } catch (err) {
        console.error('[WS] Failed to parse message:', err, event.data);
      }
    };

    this.ws.onclose = () => {
      console.warn('[WS] Connection closed.');
      this.isConnecting = false;
      this.trigger('close');
      this.attemptReconnect();
    };

    this.ws.onerror = (err) => {
      console.error('[WS] Error:', err);
      this.trigger('error', err);
    };
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('[WS] Max reconnect attempts reached.');
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 10000);
    console.log(`[WS] Reconnecting in ${delay}ms (Attempt ${this.reconnectAttempts})...`);
    setTimeout(() => this.connect(), delay);
  }

  sendAction(action, payload = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Cannot send action, socket not open:', action);
      return false;
    }

    const message = {
      action: action,
      payload: payload,
      timestamp: Date.now()
    };

    this.ws.send(JSON.stringify(message));
    return true;
  }

  on(action, callback) {
    if (!this.handlers.has(action)) {
      this.handlers.set(action, []);
    }
    this.handlers.get(action).push(callback);
  }

  trigger(action, ...args) {
    if (this.handlers.has(action)) {
      this.handlers.get(action).forEach(cb => cb(...args));
    }
  }
}

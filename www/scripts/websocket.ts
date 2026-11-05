export type NullableMediaChangePayload = { type: "none", newVersion: number } | MediaChangePayload;
export type MediaChangePayload = {
  type: "yt" | "testvideo" | "testaudio" | "sc" | "2525"
  url: string
  aspectRatio: string
  newVersion: number
}
export type SocketMsg = {
  type: "handshake" | "swap" | "event"
  payload: string
} | {
  type: "media-change"
  payload: MediaChangePayload
}

const SLOW_CONNECTION_TIMEOUT_MS = 6000;

export class Plst4Socket {
  socket: WebSocket | undefined = undefined;
  events: EventSource | undefined = undefined;
  retryCount = 0;
  hasHandshake = false;
  onMessage: (msg: SocketMsg) => void;
  queue: SocketMsg[] = [];
  playlist: string;
  slowConnectionTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  slowConnectionPrompt: HTMLElement | undefined = undefined;

  constructor(onMessage: (msg: SocketMsg) => void) {
    this.onMessage = onMessage;
    this.playlist = document.querySelector("main")!.dataset.playlist as string;
    this.#init();
  }

  #init() {
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    const wssUri = `${scheme}//${location.host}/ws/${this.playlist}`;
    console.log(`Attempting to connect to WebSocket endpoint at ${wssUri}`);

    this.socket = new WebSocket(wssUri);
    this.#armSlowConnectionTimer();

    this.socket.onopen = () => {
      console.log("WebSocket connection established");
      if (this.socket !== undefined) {
        for (const msg of this.queue) {
          console.log("message sented:", msg);
          this.socket.send(JSON.stringify(msg));
        }

        this.queue = [];
      }
    };

    this.socket.onerror = (ev) => {
      console.error("WebSocket error: ", ev);
    };

    this.socket.onmessage = (msg) => {
      this.#handleMessage(JSON.parse(msg.data));
    };

    this.socket.onclose = (ev) => {
      this.#clearSlowConnectionTimer();
      this.socket = undefined;
      console.error("WebSocket closed: ", ev);
      if (!this.hasHandshake) {
        this.#initSSE(this.playlist);
        return;
      }

      // Abnormal Closure/Service Restart/Try Again Later
      if ([1006, 1012, 1013].includes(ev.code)) {
        const exp = Math.min(this.retryCount, 6);
        const maxDelay = 1000 * Math.pow(2, exp);
        const delay = maxDelay * Math.random();
        console.log(`Retrying in ${delay}ms`);
        setTimeout(() => this.#init(), delay);
      }
    };
  }

  #armSlowConnectionTimer() {
    this.#clearSlowConnectionTimer();
    this.slowConnectionTimer = setTimeout(() => this.#showSlowConnectionPrompt(), SLOW_CONNECTION_TIMEOUT_MS);
  }

  #clearSlowConnectionTimer() {
    if (this.slowConnectionTimer !== undefined) {
      clearTimeout(this.slowConnectionTimer);
      this.slowConnectionTimer = undefined;
    }
  }

  #showSlowConnectionPrompt() {
    if (this.slowConnectionPrompt !== undefined || this.hasHandshake || this.events !== undefined) return;

    const box = document.querySelector(".toast-notification-box");
    if (box === null) return;

    const prompt = document.createElement("div");
    prompt.className = "prompt";
    prompt.innerHTML = `
      <div class="toast-wrapper">
        <h1>Connection is slow</h1>
        <p>The realtime connection is taking longer than expected. Switch to the SSE fallback?</p>
        <div class="prompt-actions">
          <button type="button" class="base-background" data-action="dismiss">Keep waiting</button>
          <button type="button" class="accent-background" data-action="switch">Switch to SSE</button>
        </div>
      </div>
    `;
    prompt.querySelector('[data-action="switch"]')!.addEventListener("click", () => {
      this.#dismissSlowConnectionPrompt();
      this.switchToSSE();
    });
    prompt.querySelector('[data-action="dismiss"]')!.addEventListener("click", () => {
      this.#dismissSlowConnectionPrompt();
    });

    box.prepend(prompt);
    this.slowConnectionPrompt = prompt;
  }

  #dismissSlowConnectionPrompt() {
    this.slowConnectionPrompt?.remove();
    this.slowConnectionPrompt = undefined;
  }

  switchToSSE() {
    this.#clearSlowConnectionTimer();
    if (this.socket !== undefined) {
      this.socket.onopen = null;
      this.socket.onerror = null;
      this.socket.onmessage = null;
      this.socket.onclose = null;
      this.socket.close();
      this.socket = undefined;
    }
    this.#initSSE(this.playlist);
  }

  #initSSE(playlist: string) {
    if (this.events !== undefined) return;

    this.#clearSlowConnectionTimer();
    this.#dismissSlowConnectionPrompt();

    const uri = `/sse/${playlist}`;
    console.log(`Falling back to SSE endpoint at ${uri}`);
    this.events = new EventSource(uri);

    this.events.onopen = () => {
      console.log("SSE connection established");
    };

    this.events.onerror = (ev) => {
      console.error("SSE error: ", ev);
    };

    this.events.onmessage = (msg) => {
      this.#handleMessage(JSON.parse(msg.data));
    };
  }

  #handleMessage(msg: SocketMsg) {
    if (msg.type === "handshake") {
      this.hasHandshake = true;
      this.#clearSlowConnectionTimer();
      this.#dismissSlowConnectionPrompt();
    }

    this.onMessage(msg);
  }

  send(msg: SocketMsg) {
    if (this.socket !== undefined) {
      console.debug("Message sent", msg);
      this.socket.send(JSON.stringify(msg));
    } else {
      this.queue.push(msg);
    }
  }
}

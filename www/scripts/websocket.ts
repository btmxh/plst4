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

export class Plst4Socket {
  socket: WebSocket | undefined = undefined;
  events: EventSource | undefined = undefined;
  retryCount = 0;
  hasHandshake = false;
  onMessage: (msg: SocketMsg) => void;
  queue: SocketMsg[] = [];

  constructor(onMessage: (msg: SocketMsg) => void) {
    this.onMessage = onMessage;
    this.#init();
  }

  #init() {
    const playlist = document.querySelector("main")!.dataset.playlist as string;
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    const wssUri = `${scheme}//${location.host}/ws/${playlist}`;
    console.log(`Attempting to connect to WebSocket endpoint at ${wssUri}`);

    this.socket = new WebSocket(wssUri);

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
      this.socket = undefined;
      console.error("WebSocket closed: ", ev);
      if (!this.hasHandshake) {
        this.#initSSE(playlist);
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

  #initSSE(playlist: string) {
    if (this.events !== undefined) return;

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

import htmx from "htmx.org";
import { MediaChangePayload } from "../websocket.js";

export const waitUntilDefined = (fn: () => any, callback: () => void) => {
  if (fn() === undefined) {
    setTimeout(() => waitUntilDefined(fn, callback), 100);
    return;
  }

  callback();
};

let lastNextRequest = -Infinity;

export class Player {
  play() {
  }

  pause() {
  }

  stop() {
  }

  start(payload: MediaChangePayload) {
  }

  hide() {
  }

  show() {
  }

  async nextRequest() {
    const now = Date.now();
    // ratelimit
    if (now - lastNextRequest < 100) {
      await new Promise(r => setTimeout(r, 100));
    }

    lastNextRequest = Date.now();
    const form = new FormData();
    form.set("quiet", "true")
    return fetch(`/watch/${(document.querySelector("main") as HTMLElement).dataset.playlist}/queue/nextreq`, {
      method: "post",
      body: form,
    }).then(res => res.text()).then(res => htmx.swap("body", res, {
      swapStyle: "none",
      swapDelay: 100,
      settleDelay: 20,
    }));
  }
}

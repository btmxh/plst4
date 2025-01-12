import htmx from "htmx.org";
import { MediaChangePayload } from "../websocket.js";

export const waitUntilDefined = (fn: () => any, callback: () => void) => {
  if (fn() === undefined) {
    setTimeout(() => waitUntilDefined(fn, callback), 100);
    return;
  }

  callback();
};

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

  nextRequest() {
    const form = new FormData();
    form.set("quiet", "true")
    fetch(`/watch/${(document.querySelector("main") as HTMLElement).dataset.playlist}/queue/nextreq`, {
      method: "post",
      body: form,
    }).then(res => res.text()).then(res => htmx.swap("body", res, {
      swapStyle: "none",
      swapDelay: 100,
      settleDelay: 20,
    }));
  }
}

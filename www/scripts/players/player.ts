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
}

import htmx from "htmx.org";
import { Plst4Socket } from "./websocket.js";

(window as any).copyPrevInput = (e: MouseEvent) => {
  let elm = e.currentTarget as HTMLElement;
  while (!(elm instanceof HTMLInputElement)) elm = elm.previousSibling as HTMLElement;
  navigator.clipboard.writeText(elm.value);
};

(window as any).copyPrevLink = (e: MouseEvent) => {
  let elm = e.currentTarget as HTMLElement;
  while (!(elm instanceof HTMLAnchorElement)) elm = elm.previousSibling as HTMLElement;
  navigator.clipboard.writeText(elm.href);
};

const socket = new Plst4Socket((msg) => {
  console.debug(msg);
  switch (msg.type) {
    case "handshake":
      (document.querySelector("#websocket-id-input") as HTMLInputElement).value = msg.payload;
      break;
    case "swap":
      htmx.swap("body", msg.payload, {
        swapStyle: "none",
        swapDelay: 100,
        settleDelay: 20,
      })
      break;
    case "event":
      htmx.trigger(document.body, msg.payload);
      break;
  }
});

import htmx from "htmx.org";
import { MediaChangePayload, Plst4Socket } from "./websocket.js";
import { Youtube } from "./players/youtube.js";
import { Player } from "./players/player.js";

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
    case "media-change":
      handleMediaChange(msg.payload);
      htmx.trigger(document.body, "refresh-playlist");
      break;
  }
});

const players = {
  "yt": new Youtube(),
} satisfies Record<string, Player>;

const handleMediaChange = (payload: MediaChangePayload) => {
  for (const [key, player] of Object.entries(players)) {
    player.stop();
    if (key === payload.type) {
      player.show();
      player.start(payload);
    } else {
      player.hide();
    }
  }
};


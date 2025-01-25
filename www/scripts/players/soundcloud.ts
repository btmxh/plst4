import { MediaChangePayload } from "../websocket.js";
import { Player, waitUntilDefined } from "./player.js";

declare global {
  interface Window {
    SC: any;
  }
}

export class SoundCloud extends Player {
  player: any
  pendingPayload: MediaChangePayload | undefined

  constructor() {
    super()
    this.init()
  }

  init() {
    waitUntilDefined(() => window.SC, () => {
      waitUntilDefined(() => window.SC.Widget, () => {
        const SC = window.SC;
        const player = new SC.Widget("soundcloud-music-player");
        player.bind(SC.Widget.Events.READY, () => {
          console.log("SoundCloud embed player initialized");
          this.player = player;
          if (this.pendingPayload !== undefined) {
            this.start(this.pendingPayload);
            this.pendingPayload = undefined;
          }
        });

        player.bind(SC.Widget.Events.FINISH, () => {
          this.nextRequest();
        });
        player.bind(SC.Widget.Events.ERROR, () => {
          console.debug("SoundCloud embed player error");
          this.nextRequest();
        });
      });
    });
  }

  show() {
    document.querySelector("#soundcloud-music-player-wrapper")!.classList.add("show")
  }

  hide() {
    document.querySelector("#soundcloud-music-player-wrapper")!.classList.remove("show")
  }

  play() {
    this.player?.play()
  }

  pause() {
    this.player?.pause()
  }

  stop() {
    this.player?.pause()
    this.player?.seekTo(0)
  }

  start(payload: MediaChangePayload) {
    if (payload.type !== "sc") {
      console.error("Invalid payload media type")
      return;
    }

    if (this.player === undefined) {
      this.pendingPayload = payload
      return
    }

    this.player.load(payload.url + "?auto_play=true");
  }
}

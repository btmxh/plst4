import { MediaChangePayload } from "../websocket.js";
import { Player } from "./player.js";

export class Html5Player extends Player {
  player: HTMLVideoElement | HTMLAudioElement;

  constructor(player: HTMLVideoElement | HTMLAudioElement) {
    super();
    this.player = player;
    this.player.addEventListener("ended", () => this.nextRequest());
    this.player.addEventListener("error", (evt) => {
      if (this.player.src == "") {
        return;
      }
      console.debug("HTML5 player error", evt);
      // playwright browsers might not support the necessary codecs
      if (!navigator.webdriver && this.player.error?.code !== 4) {
        this.nextRequest();
      }
    });
  }

  play() {
    this.player.play();
  }

  pause() {
    this.player.pause();
  }

  stop() {
    this.player.pause();
    this.player.currentTime = 0;
  }

  start(payload: MediaChangePayload) {
    this.player.src = payload.url;
    this.play();
  }

  hide() {
    this.player.parentElement!.classList.remove("show");
  }

  show() {
    this.player.parentElement!.classList.add("show");
  }
}

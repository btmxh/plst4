import { MediaChangePayload } from "../websocket.js";
import { Player } from "./player.js";

export class TestVideoPlayer extends Player {
  player: HTMLVideoElement;

  constructor() {
    super();
    this.player = document.querySelector("video#test-video-player")!;
    this.player.addEventListener("ended", () => this.nextRequest());
    this.player.addEventListener("error", (evt) => {
      console.debug("Video player error", evt);
      this.nextRequest();
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
    this.player.load();
    this.play();
  }

  hide() {
    this.player.parentElement!.classList.remove("show");
  }

  show() {
    this.player.parentElement!.classList.add("show");
  }
}

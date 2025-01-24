import { MediaChangePayload } from "../websocket.js";
import { Player } from "./player.js";

export class TestVideoPlayer extends Player {
  player: HTMLVideoElement;

  constructor() {
    super();
    this.player = document.querySelector("video#test-video-player")!;
    this.player.addEventListener("ended", () => this.nextRequest());
    this.player.addEventListener("error", (evt) => {
      if(this.player.src === "") {
        return;
      }
      console.debug("Video player error", evt);
      // playwright browsers might not support the necessary codecs
      if(!navigator.webdriver && this.player.error?.code !== 4) {
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

import { MediaChangePayload } from "../websocket.js";
import { Player } from "./player.js";

export class TestAudioPlayer extends Player {
  player: HTMLAudioElement;

  constructor() {
    super();
    this.player = document.querySelector("audio#test-audio-player")!;
    this.player.addEventListener("ended", () => this.nextRequest());
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

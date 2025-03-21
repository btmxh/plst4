import { MediaChangePayload } from "../websocket.js";
import { Player } from "./player.js";


export class Niconico extends Player {
  static origin = 'https://embed.nicovideo.jp';
  static playerId = 0;

  container: HTMLDivElement;
  player: HTMLIFrameElement | undefined;
  pendingMessages: any[] = [];
  playerLoaded: boolean = false;

  constructor() {
    super();
    this.container = document.querySelector("div#niconico-video-player-wrapper")!;
    window.addEventListener("message", (e) => this.onMessage(e));
  }

  show() {
    this.container.classList.add("show");
  }

  hide() {
    this.container.classList.remove("show");
    this.container.replaceChildren();
  }

  play() {
    this.postMessage({ eventName: "play" });
  }

  pause() {
    this.postMessage({ eventName: "pause" });
  }

  stop() {
    this.pause();
    this.postMessage({ eventName: "seek", data: { time: 0 } });
  }

  start(payload: MediaChangePayload) {
    if (payload.type !== "2525") {
      console.error("Invalid payload media type")
      return;
    }

    this.playerLoaded = false;
    this.player = document.createElement("iframe");
    const id = payload.url.substring("https://www.nicovideo.jp/watch/".length);
    const playerId = ++Niconico.playerId;
    this.player.addEventListener('load', () => {
      this.playerLoaded = true;
    });
    this.player.src = `${Niconico.origin}/watch/${id}?jsapi=1&playerId=${playerId}&autoplay=1`;
    this.player.id = "niconico-video-player";
    this.player.allow = "autoplay; fullscreen";
    this.container.replaceChildren(this.player);
    this.container.style.aspectRatio = payload.aspectRatio;
    this.play();
  }

  postMessage(msg: any) {
    msg = Object.assign({
      sourceConnectorType: 1,
      playerId: Niconico.playerId
    }, msg);
    let done = false;
    const callback = () => {
      if (!done) {
        this.player?.contentWindow?.postMessage(msg, Niconico.origin);
        done = true;
      }
    }
    this.player?.addEventListener('load', () => {
      callback();
    });
    if (this.playerLoaded) {
      callback();
    }
  }

  onMessage(e: MessageEvent) {
    if (e.origin !== Niconico.origin || e.data.playerId !== Niconico.playerId.toString()) {
      return;
    }

    if (e.data.eventName === "playerStatusChange" && e.data.data.playerStatus === 4) {
      this.nextRequest();
    }

    if (e.data.eventName === "error") {
      this.nextRequest();
    }

    console.debug(e.data);
  }
}


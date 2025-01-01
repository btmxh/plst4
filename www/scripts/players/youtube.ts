import htmx from "htmx.org";
import { MediaChangePayload } from "../websocket.js";
import { Player, waitUntilDefined } from "./player.js";

export class Youtube extends Player {
  player: YT.Player | undefined
  pendingPayload: MediaChangePayload | undefined

  constructor() {
    super()
    this.init()
  }

  init() {
    waitUntilDefined(() => window["YT"], () => {
      waitUntilDefined(() => YT.Player, () => {
        console.log("Initializing YouTube embed player");
        const player = new YT.Player("#youtube-video-player", {
          width: "100%",
          height: "100%",
          playerVars: {
            playsinline: 1,
            autoplay: 1,
            enablejsapi: 1,
            modestbranding: 0,
            cc_lang_pref: "en",
          },
          events: {
            onReady: () => {
              console.log("YouTube embed player initialized");
              this.player = player;
              if (this.pendingPayload !== undefined) {
                this.start(this.pendingPayload);
                this.pendingPayload = undefined;
              }
            },
            onStateChange: (state) => {
              if (state.data === YT.PlayerState.ENDED) {
                const form = new FormData();
                form.set("quiet", "true")
                fetch(`/watch/${(document.querySelector("main") as HTMLElement).dataset.playlist}/queue/nextreq`, {
                  method: "post",
                  body: form,
                }).then(res => res.text()).then(res => htmx.swap("body", res, {
                  swapStyle: "none",
                  swapDelay: 100,
                  settleDelay: 20,
                }))
              }
            },
            onError: (err) => {
              console.error(err);
            }
          }
        })

      })
    })
  }

  show() {
    document.querySelector("#youtube-video-player-wrapper")!.classList.add("show")
  }

  hide() {
    document.querySelector("#youtube-video-player-wrapper")!.classList.remove("show")
  }

  play() {
    this.player?.playVideo()
  }

  pause() {
    this.player?.pauseVideo()
  }

  stop() {
    this.player?.stopVideo()
  }

  start(payload: MediaChangePayload) {
    if (payload.type !== "yt") {
      console.error("Invalid payload media type")
      return;
    }

    if (this.player === undefined) {
      this.pendingPayload = payload
      return
    }

    const id = payload.url.substring("https://youtu.be/".length);
    this.player.loadVideoById(id);
    (document.querySelector("#youtube-video-player-wrapper") as HTMLElement).style.aspectRatio = payload.aspectRatio;
  }
}

import { Html5Player } from "./html5.js";

export class TestVideoPlayer extends Html5Player {
  constructor() {
    super(document.querySelector("video#test-video-player")!);
  }
}

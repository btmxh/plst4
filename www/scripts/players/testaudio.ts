import { Html5Player } from "./html5.js";

export class TestAudioPlayer extends Html5Player {
  constructor() {
    super(document.querySelector("audio#test-audio-player")!);
  }
}


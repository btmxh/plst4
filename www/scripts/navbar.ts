const toggle = () => document.querySelectorAll(".navbar").forEach(n => n.classList.toggle("hidden"));

(window as any).toggleNavbar = (e: MouseEvent) => {
  (e.currentTarget as HTMLElement).blur();
  console.log(e.currentTarget);
  toggle();
};

const threshold = 500; //ms
let lastPress: number | undefined = undefined;
document.addEventListener("keydown", ev => {
  if (ev.key !== " ") {
    lastPress = undefined;
    return;
  }

  const now = Date.now();
  if (lastPress !== undefined && now - lastPress < threshold) {
    lastPress = undefined;
    toggle();
    ev.preventDefault();
    return;
  }

  lastPress = now;
  ev.preventDefault();
});

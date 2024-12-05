import htmx from "htmx.org";

(window as any).removeMe = (e: MouseEvent) => (e.currentTarget as HTMLElement).remove();
htmx.onLoad(n => {
  console.log(n);
  if (n instanceof HTMLElement && n.hasAttribute("toast")) {
    setTimeout(() => n.remove(), 5000);
  }
})

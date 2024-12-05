import htmx from "htmx.org";

const handle = (s) => {
  if (s.classList.contains("timestamp") && s.tagName === "SPAN") {
    const date = new Date(s.dataset.value);
    s.textContent = date.toLocaleString();
  }
};

htmx.on("htmx:afterProcessNode", (e) => {
  const elt = e.detail.elt;
  if (elt.getAttribute) {
    handle(elt);
    if (elt.querySelectorAll) {
      const children = elt.querySelectorAll("span.timestamp");
      for (let i = 0; i < children.length; i++) {
        handle(children[i]);
      }
    }
  }
});

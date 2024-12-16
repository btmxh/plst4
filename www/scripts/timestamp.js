import htmx from "htmx.org";

const handle = (s) => {
  if (s.classList.contains("timestamp") && s.tagName === "SPAN") {
    const date = new Date(s.dataset.value);
    const formattedDate = date.getFullYear() +
      '-' + String(date.getMonth() + 1).padStart(2, '0') +
      '-' + String(date.getDate()).padStart(2, '0') +
      ' ' + String(date.getHours()).padStart(2, '0') +
      ':' + String(date.getMinutes()).padStart(2, '0') +
      ':' + String(date.getSeconds()).padStart(2, '0');
    s.textContent = formattedDate;
  }
};

htmx.on("htmx:afterProcessNode", (e) => {
  const elt = e.detail.elt;
  if (elt.getAttribute) {
    handle(elt);
    if (elt.querySelectorAll) {
      elt.querySelectorAll("span.timestamp").forEach(handle);
    }
  }
});

document.querySelectorAll("span.timestamp").forEach(handle);

import htmx from "htmx.org";

const handle = (s) => {
  if (s.classList.contains("timestamp") && s.tagName === "SPAN") {
    const date = new Date(s.dataset.value);
    const formattedDate = Intl.DateTimeFormat('en-GB', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      timeZoneName: 'short',
    }).format(date);
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

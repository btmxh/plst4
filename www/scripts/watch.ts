(window as any).copyPrevInput = (e: MouseEvent) => {
  let elm = e.currentTarget as HTMLElement;
  while (!(elm instanceof HTMLInputElement)) elm = elm.previousSibling as HTMLElement;
  navigator.clipboard.writeText(elm.value);
};

// Soft-keyboard input for the terminal.
//
// ghostty-web reads keystrokes from `keydown` and maps them through
// `KeyboardEvent.code`. Phone keyboards give it nothing to work with: Android
// reports every key as keyCode 229 with an empty `code`, and delivers the text
// through composition and `beforeinput` events instead. ghostty cancels
// `beforeinput` outright and only forwards a composition when it ends, so a
// word shows up after the following space at best — and the space, Enter and
// Backspace never arrive at all.
//
// This fills that gap from the capture phase on the terminal host, ahead of
// ghostty's own listeners. Hardware keyboards are unaffected: ghostty cancels
// the `keydown` it handles, so no `beforeinput` follows for us to double-send.
// See REGRESSIONS.md: "Soft keyboards don't type through keydown".

const DEL = "\x7f";

// Keys ghostty drops when the keyboard doesn't say which physical key it was.
const UNCODED_KEYS: Record<string, string> = {
  Enter: "\r",
  Backspace: DEL,
  Tab: "\t",
};

export function attachSoftKeyboardInput(
  host: HTMLElement,
  send: (data: string) => void,
): () => void {
  // What the PTY has received of the word being composed. The keyboard keeps
  // revising that word (autocorrect, backspace, swipe typing), so each update
  // is sent as "erase what changed, type the rest" to keep the line live.
  let composed: string[] = [];

  const syncComposition = (text: string) => {
    const next = Array.from(text);
    let same = 0;
    while (same < composed.length && same < next.length && composed[same] === next[same]) same++;
    const out = DEL.repeat(composed.length - same) + next.slice(same).join("");
    composed = next;
    if (out) send(out);
  };

  // Composed text can't be cancelled, so the browser leaves it behind in
  // whichever element had focus: the host itself or ghostty's textarea.
  const discardLeftovers = (target: EventTarget | null) => {
    if (target instanceof HTMLTextAreaElement) target.value = "";
    for (const node of Array.from(host.childNodes)) {
      if (node.nodeType === Node.TEXT_NODE) node.remove();
    }
  };

  // The composition events are kept from ghostty, which would otherwise send
  // the whole word a second time when it ends.
  const onCompositionStart = (e: CompositionEvent) => {
    e.stopImmediatePropagation();
    composed = [];
  };
  const onCompositionUpdate = (e: CompositionEvent) => {
    e.stopImmediatePropagation();
    syncComposition(e.data);
  };
  const onCompositionEnd = (e: CompositionEvent) => {
    e.stopImmediatePropagation();
    syncComposition(e.data);
    composed = [];
    discardLeftovers(e.target);
  };

  const onBeforeInput = (e: InputEvent) => {
    let data: string | null = null;
    switch (e.inputType) {
      case "insertText":
        // Mid-composition text arrives through the composition events.
        if (!e.isComposing) data = e.data;
        break;
      case "insertLineBreak":
      case "insertParagraph":
        data = "\r";
        break;
      case "deleteContentBackward":
        if (!e.isComposing) data = DEL;
        break;
    }
    if (!data) return;
    e.preventDefault();
    send(data);
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.isComposing || e.keyCode === 229) return;
    if (e.code !== "" && e.code !== "Unidentified") return;
    if (e.ctrlKey || e.altKey || e.metaKey) return;
    const data = UNCODED_KEYS[e.key];
    if (!data) return;
    // Cancelling also suppresses the `beforeinput` that would repeat it.
    e.preventDefault();
    send(data);
  };

  const opts = { capture: true };
  host.addEventListener("compositionstart", onCompositionStart, opts);
  host.addEventListener("compositionupdate", onCompositionUpdate, opts);
  host.addEventListener("compositionend", onCompositionEnd, opts);
  host.addEventListener("beforeinput", onBeforeInput, opts);
  host.addEventListener("keydown", onKeyDown, opts);

  return () => {
    host.removeEventListener("compositionstart", onCompositionStart, opts);
    host.removeEventListener("compositionupdate", onCompositionUpdate, opts);
    host.removeEventListener("compositionend", onCompositionEnd, opts);
    host.removeEventListener("beforeinput", onBeforeInput, opts);
    host.removeEventListener("keydown", onKeyDown, opts);
  };
}

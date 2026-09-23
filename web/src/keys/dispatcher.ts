import { ACTIONS } from "./registry";
import { matchEvent } from "./serialize";
import { getBinding } from "./storage";

type Handler = () => void;

const handlers = new Map<string, Set<Handler>>();
let listening = false;

function isEditableTarget(el: EventTarget | null): boolean {
  if (!(el instanceof Element)) return false;
  const tag = el.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  if ((el as HTMLElement).isContentEditable) return true;
  if (el.getAttribute("role") === "textbox") return true;
  return false;
}

function onKeyDown(e: KeyboardEvent): void {
  // Skip while a binding-capture button is recording so the user can press
  // any combination — including ones that match existing actions.
  if (e.target instanceof Element && e.target.getAttribute("data-keybind-capture") === "true") {
    return;
  }
  const editable = isEditableTarget(e.target);
  for (const action of ACTIONS) {
    const set = handlers.get(action.id);
    if (!set || set.size === 0) continue;
    const binding = getBinding(action.id);
    if (!binding) continue;
    if (!matchEvent(e, binding)) continue;
    // Skip when typing into an editable element unless the binding uses
    // Ctrl or Meta — those modifiers can't be confused with text input.
    if (editable && !binding.ctrl && !binding.meta) return;
    e.preventDefault();
    e.stopPropagation();
    for (const h of set) h();
    return;
  }
}

function ensureListening(): void {
  if (listening) return;
  // Capture phase: terminals (xterm) call stopPropagation on keydown, so a
  // bubble listener on window would never see those keys.
  window.addEventListener("keydown", onKeyDown, { capture: true });
  listening = true;
}

export function register(actionId: string, handler: Handler): () => void {
  let set = handlers.get(actionId);
  if (!set) {
    set = new Set();
    handlers.set(actionId, set);
  }
  set.add(handler);
  ensureListening();
  return () => {
    const s = handlers.get(actionId);
    if (s) s.delete(handler);
  };
}

import { ACTIONS_BY_ID } from "./registry";
import { formatBinding, parseBinding } from "./serialize";
import type { ActionId, Binding } from "./types";

const STORAGE_KEY = "kanban.keybindings";
export const KEYBINDINGS_EVENT = "kanban:keybindings-changed";

// Map of actionId → Binding (override) or null (explicitly unbound). A missing
// entry means "use the action's default".
export type Overrides = Record<ActionId, Binding | null>;

function load(): Overrides {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, string | null>;
    const out: Overrides = {};
    for (const [id, str] of Object.entries(parsed)) {
      if (str === null) {
        out[id] = null;
      } else if (typeof str === "string") {
        const b = parseBinding(str);
        if (b) out[id] = b;
      }
    }
    return out;
  } catch {
    return {};
  }
}

function save(overrides: Overrides): void {
  try {
    const serializable: Record<string, string | null> = {};
    for (const [id, b] of Object.entries(overrides)) {
      serializable[id] = b === null ? null : formatBinding(b);
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(serializable));
  } catch {
    // ignore quota / disabled storage
  }
  window.dispatchEvent(new Event(KEYBINDINGS_EVENT));
}

export function getOverrides(): Overrides {
  return load();
}

export function getBinding(id: ActionId): Binding | null {
  const overrides = load();
  if (id in overrides) return overrides[id];
  return ACTIONS_BY_ID[id]?.defaultBinding ?? null;
}

export function setBinding(id: ActionId, binding: Binding | null): void {
  save({ ...load(), [id]: binding });
}

export function resetBinding(id: ActionId): void {
  const next = { ...load() };
  delete next[id];
  save(next);
}

export function resetAll(): void {
  save({});
}

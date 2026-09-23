import type { Binding } from "./types";

const MODIFIER_KEYS = new Set(["Control", "Alt", "Shift", "Meta"]);

export function isModifierKey(key: string): boolean {
  return MODIFIER_KEYS.has(key);
}

export function normalizeKey(key: string): string {
  if (key.length === 1) return key.toUpperCase();
  return key;
}

export function bindingFromEvent(e: KeyboardEvent): Binding | null {
  if (isModifierKey(e.key)) return null;
  return {
    ctrl: e.ctrlKey,
    alt: e.altKey,
    shift: e.shiftKey,
    meta: e.metaKey,
    key: normalizeKey(e.key),
  };
}

export function matchEvent(e: KeyboardEvent, b: Binding): boolean {
  return (
    e.ctrlKey === b.ctrl &&
    e.altKey === b.alt &&
    e.shiftKey === b.shift &&
    e.metaKey === b.meta &&
    normalizeKey(e.key) === b.key
  );
}

const KEY_LABELS: Record<string, string> = {
  ArrowUp: "↑",
  ArrowDown: "↓",
  ArrowLeft: "←",
  ArrowRight: "→",
  " ": "Space",
  Escape: "Esc",
};

const REVERSE_LABELS: Record<string, string> = {
  "↑": "ArrowUp",
  "↓": "ArrowDown",
  "←": "ArrowLeft",
  "→": "ArrowRight",
  Space: " ",
  Esc: "Escape",
};

export function formatBinding(b: Binding): string {
  const parts: string[] = [];
  if (b.ctrl) parts.push("Ctrl");
  if (b.alt) parts.push("Alt");
  if (b.shift) parts.push("Shift");
  if (b.meta) parts.push("Meta");
  parts.push(KEY_LABELS[b.key] ?? b.key);
  return parts.join("+");
}

export function parseBinding(s: string): Binding | null {
  const parts = s
    .split("+")
    .map((p) => p.trim())
    .filter(Boolean);
  if (parts.length === 0) return null;
  let ctrl = false;
  let alt = false;
  let shift = false;
  let meta = false;
  let key = "";
  for (const p of parts) {
    const lower = p.toLowerCase();
    if (lower === "ctrl" || lower === "control") ctrl = true;
    else if (lower === "alt" || lower === "option") alt = true;
    else if (lower === "shift") shift = true;
    else if (lower === "meta" || lower === "cmd" || lower === "command") meta = true;
    else key = p;
  }
  if (!key) return null;
  key = REVERSE_LABELS[key] ?? normalizeKey(key);
  return { ctrl, alt, shift, meta, key };
}

export function bindingsEqual(a: Binding | null, b: Binding | null): boolean {
  if (a === null || b === null) return a === b;
  return (
    a.ctrl === b.ctrl &&
    a.alt === b.alt &&
    a.shift === b.shift &&
    a.meta === b.meta &&
    a.key === b.key
  );
}

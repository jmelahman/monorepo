// localStorage helpers for the Overview view. Each function is defensive:
// quota / disabled storage / corrupt JSON returns the sane fallback rather
// than throwing, mirroring the pattern in `src/storage.ts`.

import { isSlotKey, type SlotKey } from "./tile";

const PANELS_KEY = "overview.panels";
const COLLAPSED_KEY = "overview.tree.collapsed";

export type PersistedPanel = {
  ticketId: number;
  boardId: number;
  x: number;
  y: number;
  width: number;
  height: number;
  // When set, the panel is tiled to this slot; x/y/width/height reflect the
  // last computed slot rect but are recomputed against the live canvas size
  // each render. floatRect captures the floating rect at the time of tiling
  // so we can restore on un-tile.
  tile?: SlotKey | null;
  floatRect?: { x: number; y: number; width: number; height: number };
  // Slot to return to when un-maxing via double-click. Only meaningful when
  // tile === "max" and the panel reached "max" via toggleMaximize from a
  // previously tiled state.
  prevTile?: SlotKey | null;
};

export function loadPanels(): PersistedPanel[] {
  try {
    const raw = localStorage.getItem(PANELS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isPanel);
  } catch {
    return [];
  }
}

export function writePanels(panels: PersistedPanel[]): void {
  try {
    localStorage.setItem(PANELS_KEY, JSON.stringify(panels));
  } catch {
    // ignore quota / disabled storage
  }
}

function isPanel(v: unknown): v is PersistedPanel {
  if (!v || typeof v !== "object") return false;
  const o = v as Record<string, unknown>;
  if (
    typeof o.ticketId !== "number" ||
    typeof o.boardId !== "number" ||
    typeof o.x !== "number" ||
    typeof o.y !== "number" ||
    typeof o.width !== "number" ||
    typeof o.height !== "number"
  ) {
    return false;
  }
  if (o.tile != null && !isSlotKey(o.tile)) return false;
  if (o.prevTile != null && !isSlotKey(o.prevTile)) return false;
  if (o.floatRect != null) {
    const f = o.floatRect as Record<string, unknown>;
    if (
      typeof f.x !== "number" ||
      typeof f.y !== "number" ||
      typeof f.width !== "number" ||
      typeof f.height !== "number"
    ) {
      return false;
    }
  }
  return true;
}

export function loadCollapsedBoards(): Set<number> {
  try {
    const raw = localStorage.getItem(COLLAPSED_KEY);
    if (!raw) return new Set();
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((n: unknown): n is number => typeof n === "number"));
  } catch {
    return new Set();
  }
}

export function writeCollapsedBoards(set: Set<number>): void {
  try {
    localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...set]));
  } catch {
    // ignore
  }
}

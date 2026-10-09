// localStorage helpers for the Overview view. Each function is defensive:
// quota / disabled storage / corrupt JSON returns the sane fallback rather
// than throwing, mirroring the pattern in `src/storage.ts`.

import { isSlotKey, type SlotKey } from "./tile";

const PANELS_KEY = "overview.panels";
const COLLAPSED_KEY = "overview.tree.collapsed";
const TREE_FILTER_KEY = "overview.tree.filter";
const OPEN_ONLY_KEY = "overview.tree.openOnly";

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

function loadFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === "1";
  } catch {
    return false;
  }
}

function writeFlag(key: string, on: boolean): void {
  try {
    localStorage.setItem(key, on ? "1" : "0");
  } catch {
    // ignore
  }
}

export const loadOpenOnly = (): boolean => loadFlag(OPEN_ONLY_KEY);
export const writeOpenOnly = (on: boolean): void => writeFlag(OPEN_ONLY_KEY, on);

// Sidebar ticket filter, in the order its button cycles. Each step is stricter
// than the last: everything → only columns/boards that hold tickets → only
// tickets with a running session.
export const TREE_FILTERS = ["all", "tickets", "running"] as const;
export type TreeFilter = (typeof TREE_FILTERS)[number];

export function loadTreeFilter(): TreeFilter {
  try {
    const raw = localStorage.getItem(TREE_FILTER_KEY);
    return TREE_FILTERS.find((f) => f === raw) ?? "all";
  } catch {
    return "all";
  }
}

export function writeTreeFilter(filter: TreeFilter): void {
  try {
    localStorage.setItem(TREE_FILTER_KEY, filter);
  } catch {
    // ignore
  }
}

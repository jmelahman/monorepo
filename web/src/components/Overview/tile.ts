import type { PersistedPanel } from "./storage";

export type SlotKey = "left" | "right" | "top" | "bottom" | "tl" | "tr" | "bl" | "br" | "max";

export type Direction = "L" | "R" | "U" | "D";

export type Rect = { x: number; y: number; width: number; height: number };

const SLOT_FRACTIONS: Record<SlotKey, { x: number; y: number; w: number; h: number }> = {
  left: { x: 0, y: 0, w: 0.5, h: 1 },
  right: { x: 0.5, y: 0, w: 0.5, h: 1 },
  top: { x: 0, y: 0, w: 1, h: 0.5 },
  bottom: { x: 0, y: 0.5, w: 1, h: 0.5 },
  tl: { x: 0, y: 0, w: 0.5, h: 0.5 },
  tr: { x: 0.5, y: 0, w: 0.5, h: 0.5 },
  bl: { x: 0, y: 0.5, w: 0.5, h: 0.5 },
  br: { x: 0.5, y: 0.5, w: 0.5, h: 0.5 },
  max: { x: 0, y: 0, w: 1, h: 1 },
};

export function isSlotKey(v: unknown): v is SlotKey {
  return typeof v === "string" && v in SLOT_FRACTIONS;
}

export function slotRect(canvasW: number, canvasH: number, slot: SlotKey): Rect {
  const f = SLOT_FRACTIONS[slot];
  return {
    x: Math.round(f.x * canvasW),
    y: Math.round(f.y * canvasH),
    width: Math.round(f.w * canvasW),
    height: Math.round(f.h * canvasH),
  };
}

// Each non-max slot is an (h, v) pair where h ∈ {L, R, F} and v ∈ {T, B, F};
// (F, F) is floating. Each arrow toggles its axis: pressing the same edge
// you already hug removes it, pressing the opposite edge swaps. `max` is a
// special state where any arrow exits.
export function nextSlot(current: SlotKey | null | undefined, dir: Direction): SlotKey | null {
  if (current === "max") {
    if (dir === "L") return "left";
    if (dir === "R") return "right";
    return null;
  }
  let h: "L" | "R" | "F" = "F";
  let v: "T" | "B" | "F" = "F";
  if (current === "left" || current === "tl" || current === "bl") h = "L";
  else if (current === "right" || current === "tr" || current === "br") h = "R";
  if (current === "top" || current === "tl" || current === "tr") v = "T";
  else if (current === "bottom" || current === "bl" || current === "br") v = "B";
  if (dir === "L") h = h === "L" ? "F" : "L";
  if (dir === "R") h = h === "R" ? "F" : "R";
  if (dir === "U") v = v === "T" ? "F" : "T";
  if (dir === "D") v = v === "B" ? "F" : "B";
  if (h === "F" && v === "F") return null;
  if (h === "F") return v === "T" ? "top" : "bottom";
  if (v === "F") return h === "L" ? "left" : "right";
  if (h === "L") return v === "T" ? "tl" : "bl";
  return v === "T" ? "tr" : "br";
}

const EDGE = 24;
const CORNER = 80;

// Mouse position relative to canvas → tile zone (or null). Corners take
// priority over edges so the quadrant zones feel reachable.
export function tileZoneFromPoint(
  px: number,
  py: number,
  canvasW: number,
  canvasH: number,
): SlotKey | null {
  if (px < 0 || py < 0 || px > canvasW || py > canvasH) return null;
  if (px < CORNER && py < CORNER) return "tl";
  if (px > canvasW - CORNER && py < CORNER) return "tr";
  if (px < CORNER && py > canvasH - CORNER) return "bl";
  if (px > canvasW - CORNER && py > canvasH - CORNER) return "br";
  if (py < EDGE) return "max";
  if (px < EDGE) return "left";
  if (px > canvasW - EDGE) return "right";
  if (py > canvasH - EDGE) return "bottom";
  return null;
}

const SNAP_THRESHOLD = 8;

export type SnapGuides = {
  vertical: number[];
  horizontal: number[];
};

export function snapPosition(
  moving: { x: number; y: number; width: number; height: number; ticketId: number },
  others: PersistedPanel[],
  canvasW: number,
  canvasH: number,
): { x: number; y: number; guides: SnapGuides } {
  const verticalLines = [0, canvasW / 2, canvasW];
  const horizontalLines = [0, canvasH / 2, canvasH];
  for (const o of others) {
    if (o.ticketId === moving.ticketId) continue;
    verticalLines.push(o.x, o.x + o.width);
    horizontalLines.push(o.y, o.y + o.height);
  }
  const movingX = [moving.x, moving.x + moving.width];
  const movingY = [moving.y, moving.y + moving.height];
  let bestDx = 0;
  let bestDxAbs = SNAP_THRESHOLD + 1;
  for (const vl of verticalLines) {
    for (const e of movingX) {
      const d = vl - e;
      const ad = Math.abs(d);
      if (ad <= SNAP_THRESHOLD && ad < bestDxAbs) {
        bestDx = d;
        bestDxAbs = ad;
      }
    }
  }
  let bestDy = 0;
  let bestDyAbs = SNAP_THRESHOLD + 1;
  for (const hl of horizontalLines) {
    for (const e of movingY) {
      const d = hl - e;
      const ad = Math.abs(d);
      if (ad <= SNAP_THRESHOLD && ad < bestDyAbs) {
        bestDy = d;
        bestDyAbs = ad;
      }
    }
  }
  const newX = moving.x + bestDx;
  const newY = moving.y + bestDy;
  const newRight = newX + moving.width;
  const newBottom = newY + moving.height;
  const matchedV: number[] = [];
  const matchedH: number[] = [];
  for (const vl of verticalLines) {
    if (Math.abs(vl - newX) < 0.5 || Math.abs(vl - newRight) < 0.5) {
      if (!matchedV.includes(vl)) matchedV.push(vl);
    }
  }
  for (const hl of horizontalLines) {
    if (Math.abs(hl - newY) < 0.5 || Math.abs(hl - newBottom) < 0.5) {
      if (!matchedH.includes(hl)) matchedH.push(hl);
    }
  }
  return { x: newX, y: newY, guides: { vertical: matchedV, horizontal: matchedH } };
}

// Closest pane in the requested direction. "In direction" means the
// neighbor's center sits in the half-plane on that side of the source's
// center, with the perpendicular distance smaller than the parallel one
// (so a pane that's mostly above-and-slightly-right counts as "up", not
// "right").
export function findNeighborInDirection(
  rects: Array<{ ticketId: number; x: number; y: number; width: number; height: number }>,
  fromTicketId: number,
  dir: Direction,
): number | null {
  const from = rects.find((p) => p.ticketId === fromTicketId);
  if (!from) return null;
  const fromCx = from.x + from.width / 2;
  const fromCy = from.y + from.height / 2;
  let best: { id: number; dist: number } | null = null;
  for (const p of rects) {
    if (p.ticketId === fromTicketId) continue;
    const cx = p.x + p.width / 2;
    const cy = p.y + p.height / 2;
    const dx = cx - fromCx;
    const dy = cy - fromCy;
    let inDir = false;
    if (dir === "L") inDir = dx < -1 && Math.abs(dx) >= Math.abs(dy);
    else if (dir === "R") inDir = dx > 1 && Math.abs(dx) >= Math.abs(dy);
    else if (dir === "U") inDir = dy < -1 && Math.abs(dy) >= Math.abs(dx);
    else if (dir === "D") inDir = dy > 1 && Math.abs(dy) >= Math.abs(dx);
    if (!inDir) continue;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (!best || dist < best.dist) best = { id: p.ticketId, dist };
  }
  return best?.id ?? null;
}

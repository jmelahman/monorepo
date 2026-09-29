import { juiced, tween } from "./motion"

/**
 * Continuity across a rebuild.
 *
 * Every dispatch throws the screen away, so a relic bought from the shelf does
 * not move to the tray: one card is deleted and another, somewhere else, is
 * built. FLIP (First, Last, Invert, Play) is the standard way to make that read
 * as one card travelling. Before the render, measure every node that carries a
 * `data-flip` key; after it, find the node carrying the same key in the new
 * screen and animate it from the old box to its own.
 *
 * Keys are the views' business and must be unique on a screen: `relic-3`, not
 * `relic`. A key missing on either side is simply not animated, which is the
 * right answer for something arriving or leaving and is left to the scene that
 * owns arrivals and departures.
 *
 * Only transform and opacity move, per the guardrails, and the tween holds no
 * fill, so the node ends exactly where its stylesheet put it.
 */

export type Snapshot = Map<string, DOMRect>

/** Measure every keyed node. Empty (and free) off the table. */
export function snapshot(root: ParentNode): Snapshot {
  const rects: Snapshot = new Map()
  if (!juiced()) return rects
  for (const node of root.querySelectorAll<HTMLElement>("[data-flip]")) {
    const key = node.dataset.flip
    if (key) rects.set(key, node.getBoundingClientRect())
  }
  return rects
}

/** Animate every keyed node that moved from where the snapshot saw it. */
export function play(root: ParentNode, before: Snapshot, duration = 420): void {
  if (!before.size || !juiced()) return
  for (const node of root.querySelectorAll<HTMLElement>("[data-flip]")) {
    const key = node.dataset.flip
    const was = key ? before.get(key) : undefined
    if (!was) continue
    const now = node.getBoundingClientRect()
    if (!now.width || !now.height) continue
    const dx = was.left - now.left
    const dy = was.top - now.top
    const sx = was.width / now.width
    const sy = was.height / now.height
    // Under a pixel and a percent is a node that did not move; animating it
    // would only cost a layer.
    if (
      Math.abs(dx) < 1 &&
      Math.abs(dy) < 1 &&
      Math.abs(sx - 1) < 0.01 &&
      Math.abs(sy - 1) < 0.01
    ) {
      continue
    }
    void tween(
      node,
      [
        {
          transformOrigin: "top left",
          transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})`,
        },
        { transformOrigin: "top left", transform: "none" },
      ],
      { duration, easing: "cubic-bezier(0.23, 1, 0.32, 1)" },
    )
  }
}

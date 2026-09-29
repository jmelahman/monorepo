/**
 * The layer that outlives renders.
 *
 * Every dispatch throws `#app`'s children away and builds new ones, which is
 * the property the whole UI is designed around and the one thing an effect
 * cannot survive: a spark or a flying card appended inside `#app` is deleted by
 * the next render, and the scoring sequence renders at both ends. So effects
 * live in `#fx`, a sibling of `#app` that nothing but this module ever touches:
 * a fixed, full-viewport, click-through box holding a canvas for particles and
 * a plain DOM layer for text, badges and clones of things in flight.
 *
 * It is created on first use rather than at startup, and only the table ever
 * uses it, so a phone's document is exactly the document it was before this
 * existed.
 *
 * Coordinates are viewport coordinates throughout. The layer is fixed at the
 * origin, so an element's `getBoundingClientRect()` is already a position in
 * it, and nothing here has to know where `#app` sits or how far it scrolled.
 */

let root: HTMLDivElement | null = null
let dom: HTMLDivElement | null = null
let canvas: HTMLCanvasElement | null = null

function ensure(): { root: HTMLDivElement; dom: HTMLDivElement; canvas: HTMLCanvasElement } {
  if (root && dom && canvas) return { root, dom, canvas }
  root = document.createElement("div")
  root.id = "fx"
  root.setAttribute("aria-hidden", "true")
  // Styled here rather than in a partial because the three rules are the whole
  // of what the layer *is*, not how it looks, and a stylesheet that could lose
  // them to a cascade accident would leave a full-viewport box eating every
  // click in the game. z-index 12 clears the sheets (10) and the toast (11):
  // the pack is opened inside a sheet, and its cards have to fly over it.
  root.style.cssText =
    "position:fixed;inset:0;pointer-events:none;z-index:12;overflow:hidden;contain:strict"
  canvas = document.createElement("canvas")
  canvas.className = "fx-canvas"
  canvas.style.cssText = "position:absolute;left:0;top:0"
  dom = document.createElement("div")
  dom.className = "fx-dom"
  dom.style.cssText = "position:absolute;inset:0"
  // Canvas under the DOM layer: a number should never be painted over by the
  // sparks it throws off.
  root.append(canvas, dom)
  document.body.append(root)
  return { root, dom, canvas }
}

/** The particle canvas, created with the layer if need be. */
export const fxCanvas = (): HTMLCanvasElement => ensure().canvas

/** The DOM half of the layer. */
export const fxDom = (): HTMLDivElement => ensure().dom

export type Point = { x: number; y: number }

/** The center of an element, in layer coordinates. */
export function centerOf(node: Element): Point {
  const rect = node.getBoundingClientRect()
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }
}

/**
 * A node placed in the layer, centered on a point, removed when `lasting`
 * resolves. Positioned with `left`/`top` and centered with a translate that
 * the caller's own transforms must compose with rather than replace, which is
 * why `translate` (the individual property) is used for the centering and
 * `transform` is left free for the tweens.
 */
export function place(node: HTMLElement, at: Point, lasting: Promise<unknown>): HTMLElement {
  node.style.position = "absolute"
  node.style.left = `${at.x}px`
  node.style.top = `${at.y}px`
  node.style.translate = "-50% -50%"
  fxDom().append(node)
  const gone = () => node.remove()
  lasting.then(gone, gone)
  return node
}

/**
 * A stand-in for an element that is about to move somewhere a render will not
 * let it go: a copy, fixed over the original's box, that can be flown while the
 * real one is rebuilt underneath. The copy is not interactive and carries no
 * ids, so nothing that queries the screen can mistake it for the real thing.
 */
export function stand(node: Element): { ghost: HTMLElement; rect: DOMRect } {
  const rect = node.getBoundingClientRect()
  const ghost = node.cloneNode(true) as HTMLElement
  ghost.removeAttribute("id")
  for (const inner of ghost.querySelectorAll("[id]")) inner.removeAttribute("id")
  ghost.style.position = "absolute"
  ghost.style.left = `${rect.left}px`
  ghost.style.top = `${rect.top}px`
  ghost.style.width = `${rect.width}px`
  ghost.style.height = `${rect.height}px`
  ghost.style.margin = "0"
  ghost.style.pointerEvents = "none"
  fxDom().append(ghost)
  return { ghost, rect }
}

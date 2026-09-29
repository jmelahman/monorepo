import { ms } from "./motion"

/**
 * The marquee: a ring of bulbs around the window, the casino's sign for
 * "something is happening".
 *
 * Three states and a one-shot, and they are an API rather than an event because
 * the callers are far apart: the scoring show chases the bulbs as a hand climbs
 * and the screen transitions flash them, and neither should know how a bulb is
 * drawn. `marquee("chase")` runs the lights round the ring until something calls
 * `marquee("idle")`; `"flash"` is every bulb at full for a beat and then back to
 * whichever of the two was running. Idle is the resting state: dim bulbs, lit at
 * a low glow that breathes.
 *
 * How it is drawn, and why: the ring is one fixed element with four
 * children, and no bulb is a node. Bulbs are gradients tiled along the four
 * edges (see `styles/table/marquee.css`): the element itself is the dark ring,
 * `.a .b .c` are three copies of the lit pattern each lighting every third bulb,
 * and `.all` is every bulb at a glow. The chase steps the opacity of the three
 * in sequence. Opacity on a static layer is composited without a repaint, so a
 * chase costs the GPU three blends and the main thread nothing. Sixty child nodes
 * with a transform each was the version tried first and its cost was in style
 * recalculation, not paint. It sits in the document beside `#fx`, outside `#app`,
 * so a render, which rebuilds everything inside `#app`, never restarts it.
 *
 * Reduced motion: the stylesheet answers `chase` with a steady lit ring, since
 * "the hand is hot" is information and only the running of the lights is motion,
 * and `flash` with nothing. No timer here needs to know: the class comes and
 * goes on the same schedule either way and the CSS decides what it means.
 * Nothing runs at all on the phone or the light theme: the element is created
 * only by `startMarquee`, which `background.ts` calls only for a table with its
 * lights on.
 */

export type MarqueeMode = "chase" | "flash" | "idle"

let el: HTMLElement | null = null
/** What the ring returns to after a flash. */
let resting: "chase" | "idle" = "idle"
let flashTimer = 0

function show(mode: "chase" | "idle" | "flash"): void {
  if (el) el.dataset.mode = mode
}

export function marquee(mode: MarqueeMode): void {
  if (mode === "flash") {
    if (!el) return
    window.clearTimeout(flashTimer)
    // Off and on again, a frame apart, so a flash during a flash restarts the
    // one-shot rather than being swallowed by an animation already running.
    el.dataset.mode = resting
    void el.offsetWidth
    show("flash")
    flashTimer = window.setTimeout(() => show(resting), ms(700))
    return
  }
  window.clearTimeout(flashTimer)
  resting = mode
  show(mode)
}

export function startMarquee(): void {
  if (el) return
  el = document.createElement("div")
  el.id = "marquee"
  el.setAttribute("aria-hidden", "true")
  el.dataset.mode = resting
  for (const cls of ["a", "b", "c", "all"]) {
    const layer = document.createElement("i")
    layer.className = cls
    el.append(layer)
  }
  const anchor = document.getElementById("felt-gl") ?? document.getElementById("app")
  if (anchor) anchor.after(el)
  else document.body.prepend(el)
}

export function stopMarquee(): void {
  window.clearTimeout(flashTimer)
  el?.remove()
  el = null
}

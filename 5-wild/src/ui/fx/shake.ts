import { juiced, rate } from "./motion"

/**
 * Screen shake, as trauma rather than as a keyframe.
 *
 * The phone's shake is a CSS animation with its amplitude in a custom property
 * (`--shake`, set by the scoring scene), which is right for one shake per guess
 * and wrong for the table, where a chain of relics can land four hits inside a
 * second. Four keyframe shakes either restart each other, snapping back to rest
 * between hits, or queue, shaking for two seconds after the chain ended.
 *
 * Trauma is the standard answer (Squirrel Eiserloh, "Math for Game
 * Programmers: Juicing Your Cameras", GDC 2016). Each hit *adds* trauma, capped
 * at 1, and trauma decays linearly; what is drawn is trauma squared, so small
 * hits barely register and big ones compound. Offsets come from smooth noise
 * rather than `Math.random()` per frame, which reads as vibration rather than
 * as a camera being knocked.
 *
 * It is applied as inline `translate` and `rotate` (the individual properties)
 * on the live screen, `#app`'s first child, looked up every frame: a render may
 * replace the screen mid-shake, and the shake belongs to the table, not to the
 * node it started on. Never on `#app` itself, which would make it the
 * containing block for every `position: fixed` sheet inside it for as long as
 * the shake lasts.
 */

/** Pixels and degrees at trauma 1. */
const MAX_OFFSET = 14
const MAX_TURN = 1.4
/** Trauma lost per second at ×1. A full-strength hit settles in ~0.8s. */
const DECAY = 1.25

let trauma = 0
let host: HTMLElement | null = null
let shaken: HTMLElement | null = null
let frame = 0
let last = 0
let clock = 0

/** Where the live screen is found. Set once by `App`. */
export function bindShake(root: HTMLElement): void {
  host = root
}

/** Add a hit, 0 to 1. Returns without effect off the table or under reduced motion. */
export function shake(amount: number): void {
  if (!juiced() || !host) return
  trauma = Math.min(1, trauma + Math.max(0, amount))
  if (!frame) {
    last = performance.now()
    frame = requestAnimationFrame(tick)
  }
}

/** Smooth noise in -1..1, cheap: a sum of incommensurate sines. */
const noise = (t: number, seed: number): number =>
  (Math.sin(t * 13.1 + seed) + Math.sin(t * 21.7 + seed * 2.3) * 0.5) / 1.5

function tick(now: number): void {
  const dt = (Math.min(50, now - last) / 1000) * rate()
  last = now
  clock += dt
  trauma = juiced() ? Math.max(0, trauma - DECAY * dt) : 0

  const screen = host?.firstElementChild instanceof HTMLElement ? host.firstElementChild : null
  if (shaken && shaken !== screen) rest(shaken)
  shaken = screen

  if (!screen || trauma <= 0) {
    if (screen) rest(screen)
    shaken = null
    frame = 0
    return
  }
  const power = trauma * trauma
  const x = MAX_OFFSET * power * noise(clock, 1)
  const y = MAX_OFFSET * power * noise(clock, 7)
  const turn = MAX_TURN * power * noise(clock, 13)
  screen.style.translate = `${x.toFixed(2)}px ${y.toFixed(2)}px`
  screen.style.rotate = `${turn.toFixed(3)}deg`
  frame = requestAnimationFrame(tick)
}

function rest(node: HTMLElement): void {
  node.style.removeProperty("translate")
  node.style.removeProperty("rotate")
}

import { formatNumber as num } from "../format"
import { ms, reduced } from "./motion"
import { skipping } from "./timeline"

/**
 * Numbers that move.
 *
 * `roll` is the count-up the scoring sequence has always used for the total,
 * lifted out of `App` so every scene can climb a number the same way: gold
 * counting into the purse, a reward line tallying, a stamp's factor landing.
 */

export type Roll = {
  /** Authored milliseconds for the whole climb; paced here. */
  span?: number
  /** Sees every intermediate value, so a bar can move with the digits. */
  also?: (value: number) => void
  format?: (value: number) => string
}

/** How long the score counts up to its new value, as authored. */
export const COUNT_UP = 340

/**
 * Climb `node`'s text from `from` to `to`, snapping if the player skipped, has
 * reduced motion on, or there is nowhere to climb.
 *
 * Ease-out cubic: most of the distance is covered early, so the number reads as
 * arriving rather than crawling. The span is read once rather than per frame: a
 * climb whose span moved under it would ease out of the wrong number.
 */
export function roll(
  node: Element | null | undefined,
  from: number,
  to: number,
  options: Roll = {},
): void {
  if (!node) return
  const format = options.format ?? num
  const also = options.also
  if (skipping() || reduced() || from === to) {
    node.textContent = format(to)
    also?.(to)
    return
  }
  const started = performance.now()
  const span = ms(options.span ?? COUNT_UP)
  const tick = (now: number) => {
    const progress = Math.min(1, (now - started) / span)
    const eased = 1 - (1 - progress) ** 3
    const value = Math.round(from + (to - from) * eased)
    node.textContent = format(value)
    also?.(value)
    if (progress < 1 && !skipping()) requestAnimationFrame(tick)
    else {
      node.textContent = format(to)
      also?.(to)
    }
  }
  requestAnimationFrame(tick)
}

/**
 * How hard a moment should hit, 0 to 1, from what it was worth against what
 * the round asked for.
 *
 * On a log curve because the ratios span three orders of magnitude: an opening
 * gray guess is worth a few hundredths of the target and a late-game chain can
 * be worth twenty targets at once. Linear would leave everything below the
 * target indistinguishable and everything above it pinned. `log2(1 + ratio)`
 * over `log2(1 + 8)` puts a guess worth the whole target at 0.32, one worth
 * three targets at 0.63, and saturates at eight.
 */
export function heat(value: number, target: number): number {
  const ratio = Math.max(0, value) / Math.max(1, target)
  return Math.min(1, Math.log2(1 + ratio) / Math.log2(9))
}

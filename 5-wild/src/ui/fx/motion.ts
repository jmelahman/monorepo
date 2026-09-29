import { atSpeed, type Speed } from "../speed"
import { isTable } from "../table"

/**
 * The one motion policy.
 *
 * Before this file there were two places that decided whether something moved,
 * and they disagreed about what they covered. The stylesheet's reduced-motion
 * block stops every CSS animation and transition in the document, and five call
 * sites in `app.ts` asked `matchMedia` for themselves before starting a timer or
 * a count. That was enough while every motion in the game was a class and a
 * keyframe. It stops being enough the moment anything moves by script: an
 * `Element.animate()` call or a canvas loop is invisible to the stylesheet, so
 * each one would have to remember to ask, and the first one that forgot would be
 * a phone with "Remove animations" on playing the whole table at full tilt.
 *
 * So everything that moves by script comes through here, and the questions it
 * has to answer are answered once:
 *
 * - `reduced()`: the player asked for stillness. Motion is *stopped*, never
 *   compressed; see CLAUDE.md for the three bugs that taught that.
 * - `ms()`: a duration as authored, at the speed the player chose, by the same
 *   arithmetic the stylesheet's `calc(Xms * var(--pace))` does.
 * - `juiced()`: the table's extra effects may run. They are the desktop's alone
 *   (see `../table`), and a phone must never reach one.
 * - `tween()`: a WAAPI animation that obeys all three and can be finished early
 *   when the player taps through a sequence.
 */

let speed: Speed = 1

/** Kept in step with the setting by `App`, which owns it. */
export function setMotionSpeed(next: Speed): void {
  speed = next
}

export const reduced = (): boolean =>
  typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches

/** A duration as authored, at the speed the player asked for. */
export const ms = (authored: number): number => atSpeed(authored, speed)

/** How many times faster than authored, for loops that integrate over time. */
export const rate = (): number => speed

/** The table's effects may run: a desktop table, and a player who wants motion. */
export const juiced = (): boolean => isTable() && !reduced()

/** Every tween still running, so a skip can finish all of them at once. */
const running = new Set<Animation>()

export type TweenOptions = {
  /** Authored milliseconds; paced here. */
  duration: number
  delay?: number
  easing?: string
  iterations?: number
}

/**
 * A one-shot animation that leaves nothing behind.
 *
 * Every tween here returns its element to where it started, or animates *from*
 * somewhere *to* the element's own resting style, and holds no fill once it is
 * done. That is a rule rather than a limitation, and it is what makes skipping
 * and reduced motion safe without either needing to know what the animation
 * was: a tween that never ran and a tween that was finished early both leave
 * the element as its stylesheet draws it. A `forwards` fill would make the end
 * state a property of the animation instead, and `animation: none` has already
 * deleted two things in this codebase by removing exactly that kind of end.
 *
 * Resolves when the animation ends, is finished early, or was never started.
 */
export function tween(
  node: Element | null | undefined,
  keyframes: Keyframe[] | PropertyIndexedKeyframes,
  options: TweenOptions,
): Promise<void> {
  if (!node || reduced() || typeof node.animate !== "function") return Promise.resolve()
  const animation = node.animate(keyframes, {
    duration: ms(options.duration),
    delay: ms(options.delay ?? 0),
    easing: options.easing ?? "cubic-bezier(0.23, 1, 0.32, 1)",
    iterations: options.iterations ?? 1,
    // Backwards so a delayed tween holds its first frame through the delay
    // rather than showing the resting state and then jumping; never forwards.
    fill: "backwards",
  })
  running.add(animation)
  const done = () => {
    running.delete(animation)
  }
  return animation.finished.then(done, done)
}

/**
 * Finish everything in flight, for a player who tapped to skip. `finish()`
 * rather than `cancel()`, so anything awaiting a tween resumes as though it had
 * run to the end, which is the promise a skip makes.
 */
export function settle(): void {
  for (const animation of running) {
    try {
      animation.finish()
    } catch {
      // An infinite animation cannot be finished; nothing here makes one, and
      // a stray one is better left running than allowed to abort the skip.
    }
  }
  running.clear()
}

/**
 * A one-shot CSS class, restarted if it is already running, taken back off
 * after `authored` milliseconds at the player's speed.
 *
 * Moved here from `App.replay` so the scenes can use it without an app to call
 * back into. The forced reflow is what restarts the keyframes when two of these
 * land in a row.
 */
export function replay(node: Element | null | undefined, name: string, authored: number): void {
  if (!(node instanceof HTMLElement)) return
  node.classList.remove(name)
  void node.offsetWidth
  node.classList.add(name)
  setTimeout(() => node.classList.remove(name), ms(authored))
}

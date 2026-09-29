import { wait } from "../dom"
import { ms, settle } from "./motion"

/**
 * The clock a sequence keeps.
 *
 * A sequence is anything the player watches rather than drives: a guess
 * scoring, a pack opening, a round being won. Only one plays at a time, since
 * `App` holds input while one does, so the skip is module state rather than a
 * field threaded through every scene: a tap anywhere sets it, every `step`
 * after that resolves at once, every tween in flight is finished, and the
 * sequence runs to its end in the same frame. That is the promise a tap makes
 * and it is kept by the clock rather than by each scene remembering to ask.
 */

let skipped = false

/** A new sequence starts unskipped. */
export function begin(): void {
  skipped = false
}

/** The player asked for the end. */
export function skip(): void {
  if (skipped) return
  skipped = true
  settle()
}

export const skipping = (): boolean => skipped

/**
 * Wait an authored duration at the player's speed, or not at all once skipped.
 *
 * A step already waiting when the tap lands still runs out its remainder, which
 * is how the phone's sequence has always behaved and is kept so the phone does
 * not change; everything after it is instant.
 */
export function step(authored: number): Promise<void> {
  return skipped ? Promise.resolve() : wait(ms(authored))
}

/**
 * A run of steps that speeds up as it goes, like Balatro's triggers: each gap
 * is `ratio` times the last, down to `floor`, both in authored milliseconds.
 *
 * 0.9 and 55 are the plan's numbers: a ten-link chain at a 150ms authored gap
 * takes about 1.0s rather than 1.5, and the thirtieth link and every one after
 * lands at 55ms, which is still three frames and still legible as a separate
 * hit. The table's alone; the phone keeps its even `step`s.
 */
export class Chain {
  private n = 0
  constructor(
    private readonly ratio = 0.9,
    private readonly floor = 55,
  ) {}

  /** How many links so far, for anything that climbs with the chain. */
  get length(): number {
    return this.n
  }

  next(authored: number): Promise<void> {
    const gap = Math.max(this.floor, authored * this.ratio ** this.n)
    this.n++
    return step(gap)
  }
}

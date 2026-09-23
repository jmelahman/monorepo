/** Run pacing and the score curve. Balatro's shape, our numbers. */

export const STAGES = 8
export const ROUNDS_PER_STAGE = 3
export const BASE_GUESSES = 6

export const RELIC_SLOTS = 5
export const CONSUMABLE_SLOTS = 2

/**
 * The three round names now live in `src/ui/lang`, indexed by position in this
 * same order, and the reasoning that chose them went with them: Elite and Boss
 * are the same kind of word, since both name an encounter, and Easy/Hard was
 * dropped because difficulty is an axis the ascension ladder already owns.
 *
 * What stays here is the count, because the count is pacing rather than prose.
 * A language that wants four names for three rounds has a bug, and this is what
 * says so.
 */
export const ROUND_PAYOUT = [3, 4, 5] as const

/** Gold per unused guess, and the interest cap. */
export const GOLD_PER_UNUSED_GUESS = 1
export const INTEREST_PER = 5
export const INTEREST_CAP = 5

export const STARTING_GOLD = 4

/**
 * Hand-authored through stage 3, the part a player actually meets while
 * learning, then geometric. The multiplier is the single knob that decides
 * whether the late game is about scaling or about dying, so it lives alone.
 */
const AUTHORED: ReadonlyArray<readonly [number, number, number]> = [
  [300, 450, 600],
  [800, 1200, 1600],
  [2000, 3000, 4000],
]
const STAGE_GROWTH = 2.2

export function roundTargets(stage: number): readonly [number, number, number] {
  const authored = AUTHORED[stage - 1]
  if (authored) return authored

  const last = AUTHORED[AUTHORED.length - 1]
  if (!last) throw new Error("no authored stages")

  // Snapped to the nearest 100 at each step so targets stay readable at a
  // glance; compounding that is fine, it only ever flattens the curve slightly.
  // ("Snapped" rather than "rounded" because a round is now a thing in this game
  // and the sentence would have been about two of them.)
  let [normal, elite, boss] = last
  for (let a = AUTHORED.length; a < stage; a++) {
    const grow = (v: number) => Math.round((v * STAGE_GROWTH) / 100) * 100
    ;[normal, elite, boss] = [grow(normal), grow(elite), grow(boss)]
  }
  return [normal, elite, boss]
}

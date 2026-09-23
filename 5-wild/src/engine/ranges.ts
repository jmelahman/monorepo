import type { RunState } from "./state"

/**
 * Alphabet ranges: the leveled half of the letter line.
 *
 * An etching sells a *themed* group, vowels or staples or the heavy four, and a
 * range sells a *contiguous slice* of the alphabet, leveled the way a word
 * category is. The two crosscut rather than nest: A–E holds two vowels and three
 * consonants, and Etch Vowels reaches into all four ranges. So neither line can
 * dominate the other, and a letter can be improved twice for different reasons,
 * which is the point. Etchings say what kind of letter you are paying for,
 * ranges say where in the alphabet you are investing.
 *
 * **Every letter belongs to exactly one range.** That is the property worth
 * protecting: it is what puts a real opportunity cost on a letter, since typing
 * E advances A–E and nothing else. The etching groups cannot offer that, since every
 * word has a vowel, so Etch Vowels is closer to a rebate than to a build.
 *
 * ### Why contiguous slices rather than better-balanced buckets
 *
 * Splitting the alphabet by measured letter frequency gives four groups within
 * 4% of each other, which is flatter than what is below. It was rejected anyway:
 * the letter sets come out as unmemorable jumbles, and a player has to know
 * which group a letter is in *while typing it*. An alphabet range is the only
 * grouping that can be worked out at a glance from the keyboard.
 *
 * ### Why these cut points
 *
 * Measured over the 2300-word answer list, in letters hit per word:
 *
 * ```
 *   A–E  1.45   F–M  1.20   N–R  1.09   S–Z  1.27     spread 1.33×
 * ```
 *
 * That is the flattest of all 2300 ways to cut the alphabet into four
 * contiguous ranges. The obvious split into equal *sixths*, A–F, G–N, O–T, U–Z,
 * ranks 259th and comes out at 3.53×, because S is the second most common
 * letter in the answer list and anchors the tail of the alphabet; a range
 * starting at U scrapes together 0.47 letters a word while its siblings sit near
 * 1.5. Every one of the eight flattest cuts ends in S–Z for that reason.
 */
export type Range = {
  id: string
  name: string
  /** A contiguous slice of the alphabet. The four together partition it. */
  letters: string
}

/**
 * What one level above the first adds to every letter in the range.
 *
 * One number for all four cards, which is what the flat cut above buys: word
 * categories need five separately graded values because their shares run from
 * 0.8% to 33%, and a card that has to explain its own exchange rate is a worse
 * card. If the cut points are ever moved somewhere less even, this has to become
 * a per-range field: the shared constant *is* the claim that they are balanced.
 *
 * Priced against the etching line rather than in the abstract. Etch Vowels lands
 * +2 chips on 1.71 letters a word for $5, so about 0.68 chips a guess per gold;
 * a range level lands +4 on roughly 1.25 letters a word for $7, so about 0.71.
 * Near parity, with the edge to the range, because the range is the line that is
 * meant to still be worth buying on stage 7.
 */
export const CHIPS_PER_LEVEL = 4

export const RANGES: readonly Range[] = [
  { id: "range_ae", name: "A–E", letters: "abcde" },
  { id: "range_fm", name: "F–M", letters: "fghijklm" },
  { id: "range_nr", name: "N–R", letters: "nopqr" },
  { id: "range_sz", name: "S–Z", letters: "stuvwxyz" },
]

export const RANGE_BY_ID: Map<string, Range> = new Map(RANGES.map((range) => [range.id, range]))

/**
 * Letter to range, built once. A plain map rather than a scan because this runs
 * per tile of every guess, and because building it here is what makes the
 * partition checkable: a letter that landed in two ranges would overwrite, and
 * a letter in none would be missing, and the test asserts against `ALPHABET`.
 */
const RANGE_OF: Map<string, Range> = new Map(
  RANGES.flatMap((range) => [...range.letters].map((letter) => [letter, range] as const)),
)

/** The range a letter falls in. Total over the alphabet by construction. */
export const rangeOf = (letter: string): Range | undefined => RANGE_OF.get(letter)

/**
 * A range's level, where absent means one, on the same shape as a category level
 * and absent for the same reason: a run that never buys one keeps `ranges` out
 * of its save, and a save written before ranges existed loads unchanged.
 */
export const rangeLevelOf = (state: RunState, id: string): number => state.ranges?.[id] ?? 1

/**
 * What a range adds to each of its letters right now.
 *
 * Level one is worth nothing, deliberately, exactly as a word category's is: a
 * fresh run scores what it always did, so the whole system is opt-in and its
 * effect on the golden vectors is legible: only runs that buy a level move.
 */
export function rangeBonus(state: RunState, range: Range): { level: number; chips: number } {
  const level = rangeLevelOf(state, range.id)
  return { level, chips: CHIPS_PER_LEVEL * (level - 1) }
}

/** The chips a letter gets from its own range, for `baseChips` to add on. */
export function rangeChips(state: RunState, letter: string): number {
  const range = rangeOf(letter)
  return range ? rangeBonus(state, range).chips : 0
}

/** Ranges with at least one letter still typeable, for the shop to draw from. */
export const liveRanges = (state: RunState): readonly Range[] =>
  RANGES.filter((range) => [...range.letters].some((letter) => !state.letters[letter]?.destroyed))

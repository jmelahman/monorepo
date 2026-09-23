import { ALPHABET, isVowel } from "../content/letters"

/**
 * Permanent chip upgrades sold by the group.
 *
 * Etching used to be sold one letter at a time for one chip, against a modifier
 * line that sold twenty chips for the same four gold, which made it a slot the
 * shop wasted rather than a decision. The fix is not a bigger number, it is a
 * different axis: **a modifier is depth, an etching is breadth.** One letter
 * doing something dramatic, against many letters each doing something small.
 *
 * They stack, and that is the point. Nothing here is a one-time purchase; a run
 * that keeps buying Etch Vowels is a run that has decided what it is.
 *
 * The groups deliberately overlap only in one direction, since Consonants is
 * the broad, boring floor that touches everything the other three do not, so no
 * card is a strictly better version of another at a similar price.
 */
export type Etching = {
  id: string
  cost: number
  /** Added to every live letter in the group, on every purchase. */
  chips: number
  letters: string
}

const CONSONANTS = [...ALPHABET].filter((letter) => !isVowel(letter)).join("")

export const ETCHINGS: readonly Etching[] = [
  {
    id: "etch_vowels",
    cost: 5,
    chips: 2,
    letters: "aeiou",
  },
  {
    // The other half of AROSE and CRANE. Deduction wants to open with cheap
    // letters, and this is the only card that pays a player for doing so.
    id: "etch_staples",
    cost: 5,
    chips: 2,
    letters: "lnstr",
  },
  {
    // Nearly worthless in a word you are trying to solve, and enormous in one
    // you are typing purely to farm. A build card rather than a value card.
    id: "etch_heavy",
    cost: 5,
    chips: 3,
    letters: "jqxz",
  },
  {
    // Broad and boring on purpose: the floor the other three are measured
    // against, and the one that is never quite wrong.
    id: "etch_consonants",
    cost: 6,
    chips: 1,
    letters: CONSONANTS,
  },
]

export const ETCHING_BY_ID: Map<string, Etching> = new Map(
  ETCHINGS.map((etching) => [etching.id, etching]),
)

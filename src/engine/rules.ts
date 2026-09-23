import type { Refusal, RoundState } from "./state"

/**
 * The primitives that guess rules are built out of.
 *
 * Bosses and ascensions both restrict what may be typed, and they overlap by
 * design: ascension 5 *is* The Tyrant. Sharing the implementation is what keeps
 * the two from drifting into two slightly different definitions of the same
 * sentence, which the player would experience as the rule changing meaning
 * depending on which system happened to impose it.
 *
 * Every one of these reads `tile.color`, what actually happened, rather than
 * `tile.shown`, what the board displayed. That is load-bearing rather than
 * incidental: a rule derived from real feedback is one the answer itself always
 * satisfies, so the answer is always a legal guess and no round can be argued
 * into being unwinnable. Built on `shown` instead, The Mirror, which moves
 * feedback to positions it did not come from, could demand a letter that is in
 * no word at all, and the player would simply be unable to submit anything.
 *
 * The cost is that a lying boss makes the refusal message say more than the
 * board does. That is the right way round: a rule that cannot be satisfied is
 * broken, a rule that gives something away is merely generous.
 *
 * `color` alone was not quite the whole rule, which cost a run to learn. The
 * Magician writes a yellow onto a gray, and it writes it into `color` because a
 * yellow that did not score would be no card at all — so `color` means "what
 * this tile was worth", and only the tiles that earned their color also mean
 * "what the answer contains". A promoted H went into `found()`, ascension 10
 * demanded every found letter back, and the answer CIVIC was refused for want of
 * an H. So both readers below skip `promoted`: the invariant they are protecting
 * is that the answer is always legal, and the answer satisfies real feedback
 * only.
 *
 * Both validators return a `Refusal` rather than a sentence, which is what lets
 * the shared implementation stay shared: the boss and the ascension that impose
 * the same rule now hand the player the same *code*, and the catalog says it
 * once.
 */

/** Positions the player has already locked in green. */
export function knownGreens(round: RoundState): Map<number, string> {
  const found = new Map<number, string>()
  for (const guess of round.guesses) {
    guess.tiles.forEach((tile, i) => {
      // Nothing promotes to green today, so this arm never fires; it is here
      // because "granted colors are not evidence" is a fact about both readers,
      // and a card that one day hands out a green should not have to rediscover
      // that only one of them was told.
      if (tile.promoted) return
      if (tile.color === "green") found.set(i, tile.letter)
    })
  }
  return found
}

/**
 * Letters proven to be in the word.
 *
 * `only` narrows it to one color, because the ascension ladder demands yellows
 * two steps before it demands greens and the two have to be askable apart.
 * Insertion order is guess order, then tile order, which is what makes the
 * refusal a player sees for a given board the same one every time.
 */
export function found(round: RoundState, only?: "green" | "yellow"): Set<string> {
  const letters = new Set<string>()
  for (const guess of round.guesses) {
    for (const tile of guess.tiles) {
      if (tile.promoted) continue
      if (tile.color === "gray") continue
      if (!only || tile.color === only) letters.add(tile.letter)
    }
  }
  return letters
}

/** Every proven letter has to appear somewhere in the guess. */
export function useFound(word: string, letters: Iterable<string>): Refusal | null {
  for (const letter of letters) {
    if (!word.includes(letter)) return { code: "must_use", letter }
  }
  return null
}

/** Every green has to stay exactly where it was found. The Tyrant, and ascension 5. */
export function keepGreens(word: string, round: RoundState): Refusal | null {
  for (const [i, letter] of knownGreens(round)) {
    // One-based on the way out: the engine indexes tiles from zero and the
    // player counts them from one, and this is the only place the two meet.
    if (word[i] !== letter) return { code: "must_keep", letter, position: i + 1 }
  }
  return null
}

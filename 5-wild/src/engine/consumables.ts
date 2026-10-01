import { ALPHABET } from "../content/letters"
import type { Rng } from "./rng"
import { shuffled } from "./rng"
import type { GameEvent, Refusal, RunState } from "./state"

/**
 * One-shot cards. With no deck to manipulate, these act on the two things this
 * game does have: the puzzle and the keyboard. Each one is an answer to a
 * specific problem, being stuck or being fogged or being one guess short, which
 * is what stops them from being a flat resource.
 */
export type Consumable = {
  id: string
  cost: number
  /** Mutates the run. Returns why it could not be used, or null when it was. */
  apply: (state: RunState, rng: Rng, events: GameEvent[]) => Refusal | null
}

const CONSUMABLE_COST = 3

/**
 * How many letters The Hermit rules out. It was one, and one letter is a
 * twentieth of what an opener tells you for free: forking every round of 160
 * solver runs (2,067 rounds) and handing the card over after the first guess,
 * one letter was worth ×1.017 on the round's score and 0.07 guesses saved,
 * against ×1.065 and 0.34 for five, and ×1.043 for three. Five is also what a
 * guess spent entirely on gray letters would have told the player, which is
 * the card's honest price: a guess's worth of absence without the guess.
 */
export const HERMIT_LETTERS = 5

export const CONSUMABLES: readonly Consumable[] = [
  {
    id: "oracle",
    cost: CONSUMABLE_COST,
    apply: (state, rng, events) => {
      const round = state.round
      // Only a letter the last guess left unsolved is worth naming: a position
      // already green on the board is one the player has, so revealing it spends
      // the card on nothing. `shown`, not `color`, because it is the board the
      // player is reading, and a boss's lie is part of it.
      const last = round.guesses[round.guesses.length - 1]
      const hidden = round.revealed
        .map((value, index) =>
          value === null && last?.tiles[index]?.shown !== "green" ? index : -1,
        )
        .filter((index) => index >= 0)
      const position = shuffled(rng, hidden)[0]
      if (position === undefined) return { code: "word_already_revealed" }
      const letter = round.answer[position]
      if (letter === undefined) return { code: "nothing_to_reveal" }
      round.revealed[position] = letter
      // The position is one-based on the card and zero-based here; the +1 stays
      // on this side because it is the same number in every language, and a
      // catalog that had to remember to add one would eventually forget.
      events.push({
        type: "consumable",
        id: "oracle",
        note: { card: "oracle", letter, position: position + 1 },
      })
      return null
    },
  },
  {
    id: "hermit",
    cost: CONSUMABLE_COST,
    apply: (state, rng, events) => {
      const round = state.round
      const known = new Set(round.eliminated)
      for (const guess of round.guesses) for (const letter of guess.word) known.add(letter)

      const candidates = [...ALPHABET].filter(
        (letter) =>
          !round.answer.includes(letter) && !known.has(letter) && !state.letters[letter]?.destroyed,
      )
      const letters = shuffled(rng, candidates).slice(0, HERMIT_LETTERS)
      if (letters.length === 0) return { code: "nothing_to_rule_out" }
      round.eliminated.push(...letters)
      events.push({ type: "consumable", id: "hermit", note: { card: "hermit", letters } })
      return null
    },
  },
  {
    id: "magician",
    cost: CONSUMABLE_COST,
    // Applied after the boss rewrites feedback, so this is a real counter to
    // The Silence rather than something it quietly erases.
    //
    // Every gray of every guess for the rest of the round. It was the first
    // gray of the next guess, and a yellow is +1 mult, so the card was worth
    // ×1.004 on the round's score: forking each of 2,067 rounds from 160 solver
    // runs, with and without it, against ×1.065 for The Hermit and ×1.098 for
    // The Oracle on the same rounds. Every gray of one guess read ×1.017,
    // because grays are many only on the guesses that score little; every gray
    // all round reads ×1.048. Still the least of the four on a bare tray, and
    // deliberately: it is the one that tells the player nothing, and the yellow
    // relics are where it is meant to pay.
    apply: (state, _rng, events) => {
      if (state.round.promote) return { code: "already_prepared" }
      state.round.promote = true
      events.push({ type: "consumable", id: "magician", note: { card: "magician" } })
      return null
    },
  },
  {
    id: "fool",
    cost: CONSUMABLE_COST,
    apply: (state, _rng, events) => {
      const round = state.round
      const last = round.guesses[round.guesses.length - 1]
      if (!last) return { code: "no_guess_to_repeat" }
      round.score += last.score
      events.push({ type: "consumable", id: "fool", note: { card: "fool", score: last.score } })
      return null
    },
  },
]

export const CONSUMABLE_BY_ID = new Map(CONSUMABLES.map((card) => [card.id, card]))

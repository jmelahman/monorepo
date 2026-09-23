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

export const CONSUMABLES: readonly Consumable[] = [
  {
    id: "oracle",
    cost: CONSUMABLE_COST,
    apply: (state, rng, events) => {
      const round = state.round
      const hidden = round.revealed
        .map((value, index) => (value === null ? index : -1))
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
      const letter = shuffled(rng, candidates)[0]
      if (letter === undefined) return { code: "nothing_to_rule_out" }
      round.eliminated.push(letter)
      events.push({ type: "consumable", id: "hermit", note: { card: "hermit", letter } })
      return null
    },
  },
  {
    id: "magician",
    cost: CONSUMABLE_COST,
    // Applied after the boss rewrites feedback, so this is a real counter to
    // The Silence rather than something it quietly erases.
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

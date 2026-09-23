import type { RunState } from "../engine"
import { draftChips, solveBonusFor } from "../engine"
import { formatNumber as num } from "./format"
import { ui } from "./lang"

/**
 * The first round, taught while it is being played.
 *
 * The rules sheet already exists, and it used to open itself on first launch,
 * and that was not enough: it is read at the title screen with no board in front
 * of it, it says "green is +3 mult" to a player who has not yet seen a mult, and
 * by the time any of it is true they have closed it. Nothing in it is wrong; it
 * was simply being read at the wrong moment. This says a fifth as much at the
 * moment each piece first becomes true, with the live number it is about on
 * screen beside it, which is the half a sheet can never do. The sheet stayed,
 * as a button rather than a greeting.
 *
 * So the beats are not a script with a cursor. Each one is a *state* the first
 * round passes through, and the card shows whichever beat the run is standing in
 * right now:
 *
 *   nothing typed        chips are the letters you choose
 *   mid-word             and rare ones pay more; here is what you have
 *   word complete        mult is the color you have not seen yet
 *   one guess played     chips × mult, banked, and the pile keeps it
 *   two guesses played   what solving would multiply that pile by
 *
 * Written that way for one reason: there is no cursor to lose. A tutorial with a
 * step index has to decide what happens when the player backspaces, closes the
 * app mid-word, resumes a save, or spends a guess on a word the list refuses,
 * and every one of those is a chance for the card to be a beat ahead of the
 * board. Here they are not questions. Backspacing a full word steps the card
 * back to the chips beat because that is where the run now is; a resumed save
 * shows the beat matching the round it resumes into, with no tutorial state in
 * the save at all. Nothing here is stored except *whether the tutorial is owed*,
 * which is one flag in `localStorage` and is the only thing that outlives it.
 *
 * The consequence worth knowing: a beat cannot be shown twice without being
 * *true* twice, and it cannot be skipped by anything except the run leaving the
 * state it describes. A player who solves on their first guess never sees the
 * last two beats. That is correct: the round is over, and the sheet is still in
 * the menu for whoever wants the rest.
 *
 * Whether the cards run at all is asked once, on the round's intro card, before
 * any of this is reachable; see `coachAsks` below. The cards themselves carry no
 * button. They used to carry a "got it" that retired the tutorial mid-sentence,
 * and it was in the wrong place twice over: it sat on the board, where the whole
 * point is that nothing competes with the numbers being explained, and it asked
 * the question five times when the honest moment to ask it is before the first
 * card, when a player still has the attention to answer.
 */
export type CoachStep = {
  /** Stable name, so a test can pin a beat without quoting its prose. */
  id: "chips" | "rare" | "mult" | "banked" | "solve"
  /** What the card says, built against the run so it can quote live figures. */
  text: string
  /**
   * The thing on screen the card is about. Lit while the card is up, which is
   * what ties a sentence about "the ?" to the actual `?` under the board. A
   * selector rather than a node because the screen is rebuilt on every dispatch
   * and any node this held would be a node the next render threw away.
   */
  anchor: string
}

type Beat = {
  id: CoachStep["id"]
  /** True while the run is standing in this beat. Checked in order. */
  when: (state: RunState) => boolean
  say: (state: RunState) => string
  anchor: string
}

/**
 * The beats, in the order they are checked, which is also the order round one
 * walks through them, and the reason this is a list rather than five ifs.
 *
 * Each `when` reads the guess count first and the draft second, so the beats are
 * disjoint by construction: no two can be true at once, and the run cannot fall
 * between them while a round is live. That is what makes "first match wins" a
 * rule rather than a coincidence of ordering.
 */
const BEATS: readonly Beat[] = [
  {
    id: "chips",
    when: (state) => state.round.guesses.length === 0 && state.round.draft.length === 0,
    say: () => ui().coach.chips,
    anchor: ".readout",
  },
  {
    id: "rare",
    when: (state) =>
      state.round.guesses.length === 0 && state.round.draft.length < state.round.answer.length,
    // The live figure rather than a worked example, because the player is
    // holding the word that produced it: AROSE says 5 here and JAZZY says 33,
    // and the lesson is that they chose which.
    say: (state) => ui().coach.rare(num(draftChips(state, state.round.draft))),
    anchor: ".readout .chips",
  },
  {
    id: "mult",
    when: (state) => state.round.guesses.length === 0,
    say: () => ui().coach.mult,
    anchor: ".readout .mult",
  },
  {
    id: "banked",
    // Quotes the guess that just landed. `chips × mult = score` is the whole
    // pipeline in one line, and it is the only line in the game that can be
    // checked against the three numbers the player watched move.
    when: (state) => state.round.guesses.length === 1,
    say: (state) => {
      const guess = state.round.guesses[0]
      // `when` is "exactly one guess", so there is always a guess to quote; the
      // guard is what an indexed read costs, not a state the card can be in.
      if (!guess) return ""
      return ui().coach.banked(
        num(guess.chips),
        num(guess.mult),
        num(guess.score),
        num(state.round.target),
      )
    },
    anchor: ".hud-score",
  },
  {
    id: "solve",
    when: (state) => state.round.guesses.length === 2,
    // Both figures come from the engine rather than from `1 + guessesLeft` here,
    // for the reason `solveHint` does: a relic or a boss can bend this number,
    // and a tutorial that taught the arithmetic the game is not using would be
    // worse than one that taught nothing. Ascension 0 stage 1 has neither, and
    // this is still not the place to encode that.
    say: (state) => {
      const left = state.round.maxGuesses - state.round.guesses.length - 1
      const now = solveBonusFor(state, left)
      const next = solveBonusFor(state, left - 1)
      return ui().coach.solve(num(now), num(next))
    },
    anchor: ".solve-hint",
  },
]

/**
 * Which beat the run is standing in, or null for "say nothing".
 *
 * Gated to the first round of the first stage, and not by a counter: the point
 * of the tutorial is the moment a thing is first true, and there is no first
 * time left by round two. Everything past it, bosses and the shop and
 * ascensions, belongs to the sheet and the codex, which are written for a player who has
 * already held a guess in their hands.
 *
 * Whether the tutorial is still owed at all is the caller's question, because
 * the answer outlives the run and this file only knows about one.
 */
export function coachStep(state: RunState): CoachStep | null {
  if (state.phase !== "round" || state.stage !== 1 || state.roundIndex !== 0) return null
  // A finished round is a round with an answer on it; the last thing a player
  // needs then is a card explaining what to type.
  if (state.round.done) return null

  const beat = BEATS.find((candidate) => candidate.when(state))
  return beat ? { id: beat.id, text: beat.say(state), anchor: beat.anchor } : null
}

/**
 * Whether this is the round the tutorial would open on, and so the round whose
 * intro card has to ask whether it should.
 *
 * The same gate `coachStep` opens on, plus "nothing has been played yet", which
 * is what makes it a question rather than an interruption. It is deliberately
 * not the negation of `coachSpent`: that one answers "can the flag be retired",
 * which is true on every screen in the game bar one, and the offer has to be
 * made on exactly the one.
 *
 * Whether the offer is *owed* is still the caller's question, for the reason it
 * always was: the answer outlives the run and this file only knows about one.
 */
export function coachAsks(state: RunState): boolean {
  return (
    state.phase === "round" &&
    state.stage === 1 &&
    state.roundIndex === 0 &&
    !state.round.done &&
    state.round.guesses.length === 0
  )
}

/**
 * Whether the tutorial can never come back, so the flag that retires it can be
 * written.
 *
 * A different question from `coachStep(state) === null`, even though on a run
 * state the two currently agree: the beats cover the first three guesses with no
 * gaps between them, so a tutorial round always has something to say. Where they
 * come apart is *screens*. The card is suppressed by an open sheet, by the round
 * intro, and by the scoring animation, none of which are the tutorial finishing,
 * and a caller watching only for "no card on screen" would retire it on the
 * first of them. This is the half that only the run can answer, kept separate so
 * the caller can ask it without the other one in the way.
 */
export function coachSpent(state: RunState): boolean {
  return (
    state.phase !== "round" ||
    state.stage > 1 ||
    state.roundIndex > 0 ||
    state.round.done ||
    state.round.guesses.length > 2
  )
}

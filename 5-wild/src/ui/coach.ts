import type { RunState } from "../engine"
import { categoryOf, DISTINCT, difficultyOf, draftChips, solveBonusFor } from "../engine"
import { formatNumber as num } from "./format"
import { categoryCard, ui } from "./lang"
import { solveFloor, wordInPlay } from "./views"

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
 *     and typing           nothing, so the next card reads as new
 *     and a word complete  the shape that word scores as, and that it levels
 *   two guesses played   what solving would multiply that pile by
 *   three guesses played the switch that quiets the board, for later
 *
 * and then the first shop, which is the same idea on a screen with no clock:
 *
 *   no relic held yet    what the gold is for, and each kind of card it buys
 *   a relic held         where relics live, how many fit, how to make room
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
 * later beats. That is correct: the round is over, and the sheet is still in
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
  id: "chips" | "rare" | "mult" | "banked" | "solve" | "shape" | "decor" | "shelf" | "relics"
  /** What the card says, built against the run so it can quote live figures. */
  text: string
  /**
   * The thing on screen the card is about. Lit while the card is up, which is
   * what ties a sentence about "the ?" to the actual `?` in the header. A
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
 * disjoint by construction: no two can be true at once. That is what makes
 * "first match wins" a rule rather than a coincidence of ordering. The run can
 * fall between them in exactly one place, a second word part-typed, and that
 * gap is the point; see the banked beat.
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
    //
    // Only until the next word is started. It used to stay up through the
    // typing and hand the card to the shape beat on the last letter, the way
    // rare hands it to mult, and the two are one card to look at: the same
    // box, a sentence swapped under the same line. Rare to mult is one lesson
    // in two halves and wants that; banked to shape is two subjects, and a
    // board with no card on it for a few letters is what says the next one is
    // new. The first letter is also the moment the readout it reads back
    // switches to the word being built, so it would be quoting a line no
    // longer on screen.
    when: (state) => state.round.guesses.length === 1 && state.round.draft.length === 0,
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
    // The whole header rather than the score it is added to. The card reads
    // back the readout and names the score and the target, which between them
    // are most of the header, so the ring goes round all of it. It used to be
    // on the score alone, and the card hung from the score's foot, which is
    // the readout's line: it covered the one place the mult the previous card
    // said ENTER would reveal was still showing. From the header's foot a
    // two-line card lies over nothing but the band above the board.
    anchor: ".hud",
  },
  {
    id: "shape",
    // The chip has named a shape since the first guess landed, and at level 1
    // every shape pays nothing, so on the first board it is a label with no
    // number moving beside it and reads as decoration. What it cannot say on
    // its own is that it is for sale.
    //
    // On the second word once it is whole, after the few letters of silence
    // the banked beat leaves. It had the fourth board to itself,
    // quoting the last guess, and the decor card had the fifth, so a round
    // solved in three or four guesses, which is most rounds, ended before
    // either. A whole word is the moment the chip changes its answer from the
    // word played to the word about to be, so the card is read while the shape
    // it names is still a choice, and the second word is the first that choice
    // can be made having seen the chip at all.
    //
    // It quotes the word the chip is naming; `wordInPlay` is the chip's own
    // answer, so the card and the chip cannot name different words.
    when: (state) =>
      state.round.guesses.length === 1 && state.round.draft.length === state.round.answer.length,
    say: (state) => {
      const word = wordInPlay(state)
      return ui().coach.shape(word, categoryCard((word ? categoryOf(word) : DISTINCT).id).name)
    },
    anchor: ".category",
  },
  {
    id: "solve",
    when: (state) => state.round.guesses.length === 2,
    // Both figures come from the engine rather than from `1 + guessesLeft` here,
    // for the reason `meter` does: a relic or a boss can bend this number,
    // and a tutorial that taught the arithmetic the game is not using would be
    // worse than one that taught nothing. Ascension 0 stage 1 has neither, and
    // this is still not the place to encode that.
    say: (state) => {
      const left = state.round.maxGuesses - state.round.guesses.length - 1
      const now = solveBonusFor(state, left)
      // The card states the rule in words, "the guesses you have left", and
      // quotes the figure from here, so the number stays honest when the rule
      // is bent even where the sentence is only the plain case.
      // The sum's answer is the meter's own, so the card and the faint bar can
      // never quote two different figures for the same thing.
      const { score, target } = state.round
      const floor = solveFloor(state)?.floor ?? Math.round(score * now)
      return ui().coach.solve(num(score), num(target), num(now), num(floor))
    },
    anchor: ".hud .meter",
  },
  {
    id: "decor",
    // Not scoring, and the only beat that is not: it is about the round after
    // this one. Nothing on the first board is busy enough to need the switch,
    // which is exactly why nobody finds it until the keyboard is covered in
    // marks and they have stopped looking for buttons. So it is named here,
    // last, as a thing for later: a round still going after three guesses is
    // the round least likely to be read, and this is the card that can most
    // afford that.
    when: (state) => state.round.guesses.length === 3,
    say: () => ui().coach.decor,
    anchor: ".decor-toggle",
  },
]

/**
 * The first shop's beats, checked the same way and disjoint the same way: on
 * one question, whether a relic is held yet.
 *
 * Relics rather than purchases, because a purchase is not in the state. A
 * bought slot goes null, and a reroll deals a full shelf over it, so "has
 * anything been bought" is a question the run forgets on every reroll, and a
 * card keyed on it would step back to the shelf beat for a player who had
 * already bought. A held relic is still held after a reroll. It is also the
 * purchase worth a second card: a consumable or a letter says on its own
 * card what it does, and a relic goes somewhere the shelf did not show.
 */
const SHOP_BEATS: readonly Beat[] = [
  {
    id: "shelf",
    when: (state) => state.relics.length === 0,
    // What the shelf sells, by kind, rather than what gold is worth kept:
    // interest used to be here and was the one sentence about a card that
    // isn't on the shelf. It is on the rules sheet.
    say: () => ui().coach.shelf,
    anchor: ".shop-items",
  },
  {
    id: "relics",
    when: () => true,
    // The slot count from the run rather than from the table, for the reason
    // the solve beat asks the engine: ascension 6 takes one away.
    say: (state) => ui().coach.relics(num(difficultyOf(state).relicSlots)),
    anchor: ".shop-screen .relics",
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
  if (state.stage !== 1 || state.roundIndex !== 0) return null
  // The first shop is still round one's: `next_round` is what moves the index.
  // A pack laid open or a letter waiting for its modifier is a sheet over the
  // shop, and a card aimed at the shelf under it would be pointing at nothing
  // the player can touch.
  if (state.phase === "shop") {
    if (state.pack || state.placing) return null
    return pick(SHOP_BEATS, state)
  }
  if (state.phase !== "round") return null
  // A finished round is a round with an answer on it; the last thing a player
  // needs then is a card explaining what to type.
  if (state.round.done) return null

  return pick(BEATS, state)
}

function pick(beats: readonly Beat[], state: RunState): CoachStep | null {
  const beat = beats.find((candidate) => candidate.when(state))
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
 * A different question from `coachStep(state) === null`, and they come apart
 * twice. On a run state: a fifth guess in round one has nothing said over it,
 * and neither does the cleared round's reward screen, but the first shop is
 * still to come, so neither is the tutorial finished. The line is leaving that
 * shop, which is `next_round` moving the index, or the run ending before it
 * gets there. And on *screens*. The card is suppressed by an open sheet, by the round
 * intro, and by the scoring animation, none of which are the tutorial finishing,
 * and a caller watching only for "no card on screen" would retire it on the
 * first of them. This is the half that only the run can answer, kept separate so
 * the caller can ask it without the other one in the way.
 */
export function coachSpent(state: RunState): boolean {
  return (
    state.stage > 1 ||
    state.roundIndex > 0 ||
    state.phase === "game_over" ||
    state.phase === "victory"
  )
}

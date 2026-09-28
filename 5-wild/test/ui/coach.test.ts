import { describe, expect, it } from "vitest"
import type { Action, RunState, WordSource } from "../../src/engine"
import { difficultyOf, draftChips, reduce, solveBonusFor, startRun } from "../../src/engine"
import { coachAsks, coachSpent, coachStep } from "../../src/ui/coach"
import { realWords } from "../helpers/words"

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, realWords).state, state)

const type = (word: string): Action[] =>
  [...word].map((letter) => ({ type: "type_letter", letter }))

const guess = (word: string): Action[] => [...type(word), { type: "submit" }]

/** A word the allowed list takes and no first-round answer is, so it never solves. */
const MISS = "arose"

const fresh = (): RunState => startRun(7, realWords).state

/** Misses, one at a time: four of them is the whole tutorial walked at its natural pace. */
const played = (state: RunState, count: number): RunState =>
  Array.from({ length: count }).reduce<RunState>((current) => apply(current, guess(MISS)), state)

describe("the first-round coach", () => {
  it("opens on chips, pointing at the readout", () => {
    const step = coachStep(fresh())
    expect(step?.id).toBe("chips")
    expect(step?.anchor).toBe(".readout")
  })

  it("switches to rare letters mid-word, quoting what is actually on the board", () => {
    const state = apply(fresh(), type("qu"))
    const step = coachStep(state)
    expect(step?.id).toBe("rare")
    // The figure in the card is the one under the board, not a worked example.
    expect(step?.text).toContain(`${draftChips(state, "qu")} chips`)
    expect(step?.anchor).toBe(".readout .chips")
  })

  it("switches to mult once the word is whole", () => {
    const step = coachStep(apply(fresh(), type(MISS)))
    expect(step?.id).toBe("mult")
    expect(step?.anchor).toBe(".readout .mult")
  })

  it("steps back when the word is taken apart again", () => {
    // The property the whole design rests on: there is no cursor to be out of
    // step with the board, so backspacing is not a case anyone had to handle.
    const full = apply(fresh(), type(MISS))
    expect(coachStep(full)?.id).toBe("mult")
    const short = apply(full, [{ type: "backspace" }])
    expect(coachStep(short)?.id).toBe("rare")
    const empty = apply(
      short,
      Array.from({ length: 4 }, () => ({ type: "backspace" }) as Action),
    )
    expect(coachStep(empty)?.id).toBe("chips")
  })

  it("reads the banked guess back, arithmetic and all", () => {
    const state = played(fresh(), 1)
    const record = state.round.guesses[0]
    const step = coachStep(state)
    expect(step?.id).toBe("banked")
    expect(step?.text).toContain(`${record?.chips} × ${record?.mult} = ${record?.score}`)
    expect(step?.text).toContain(`${state.round.target}`)
    expect(step?.anchor).toBe(".hud")
  })

  it("quotes the solve bonus the engine would actually apply", () => {
    const state = played(fresh(), 2)
    const left = state.round.maxGuesses - state.round.guesses.length - 1
    const step = coachStep(state)
    expect(step?.id).toBe("solve")
    // Both figures from the engine rather than from `1 + guessesLeft` here, so a
    // relic or boss that bends the bonus cannot make the tutorial lie.
    expect(step?.text).toContain(`${solveBonusFor(state, left)}×`)

    expect(step?.anchor).toBe(".hud .meter")
  })

  it("names the shape of the second guess once it is whole, and steps back from it", () => {
    const state = apply(played(fresh(), 1), type("jazzy"))
    const step = coachStep(state)
    expect(step?.id).toBe("shape")
    expect(step?.anchor).toBe(".category")
    expect(step?.text).toContain("JAZZY scores as Cluster")
    expect(coachStep(apply(state, [{ type: "backspace" }]))).toBeNull()
  })

  it("puts the banked card away at the first letter of the next word", () => {
    // So the shape card arrives on an empty board rather than as a sentence
    // swapped inside the card already up.
    const state = played(fresh(), 1)
    expect(coachStep(state)?.id).toBe("banked")
    expect(coachStep(apply(state, type("j")))).toBeNull()
    expect(coachStep(apply(state, type("jazz")))).toBeNull()
    // Taken apart again, it is back, because the board is back where it was.
    const cleared = apply(state, [
      ...type("ja"),
      { type: "backspace" },
      { type: "backspace" },
    ] as Action[])
    expect(coachStep(cleared)?.id).toBe("banked")
  })

  it("names the decoration switch on the fourth guess, as a thing for later", () => {
    const step = coachStep(played(fresh(), 3))
    expect(step?.id).toBe("decor")
    expect(step?.anchor).toBe(".decor-toggle")
  })

  it("has nothing left to say after four guesses, and is not finished", () => {
    // The board is spent; the tutorial is not, because the first shop still
    // has its own cards to show.
    const state = played(fresh(), 4)
    expect(state.round.done).toBe(false)
    expect(coachStep(state)).toBeNull()
    expect(coachSpent(state)).toBe(false)
  })

  it("stays live for the whole of the first round", () => {
    let state = fresh()
    for (let count = 0; count < 4; count++) {
      expect(coachSpent(state)).toBe(false)
      expect(coachStep(state)).not.toBeNull()
      state = played(state, 1)
    }
  })

  it("goes quiet the moment the round is decided, with the shop still owed", () => {
    // Solving ends the round on the guess that lands it, whichever beat was up.
    const state = apply(fresh(), guess(fresh().round.answer))
    expect(state.round.solved).toBe(true)
    expect(coachStep(state)).toBeNull()
    expect(coachSpent(state)).toBe(false)
  })

  it("is spent the moment the run ends in round one", () => {
    const state = played(fresh(), fresh().round.maxGuesses)
    expect(state.phase).toBe("game_over")
    expect(coachSpent(state)).toBe(true)
  })

  it("never speaks outside the first round of the first stage", () => {
    const state = apply(apply(fresh(), guess(fresh().round.answer)), [
      { type: "collect" },
      { type: "next_round" },
    ])
    expect(state.stage).toBe(1)
    expect(state.roundIndex).toBe(1)
    expect(state.round.guesses.length).toBe(0)
    // A board that looks exactly like the one the tutorial opened on, and it is
    // silent: the first time is the only thing being taught, and it is spent.
    expect(coachStep(state)).toBeNull()
    expect(coachSpent(state)).toBe(true)
  })

  it("says nothing away from the board", () => {
    const state = apply(fresh(), guess(fresh().round.answer))
    expect(state.phase).toBe("reward")
    expect(coachStep(state)).toBeNull()
  })

  it("carries no tutorial state in the run it is teaching", () => {
    // The card is derived, never stored: a run round-tripped through the save
    // resumes at the beat it left off on with nothing written down to restore.
    const state = apply(fresh(), type("ja"))
    const resumed = JSON.parse(JSON.stringify(state)) as RunState
    expect(coachStep(resumed)).toEqual(coachStep(state))
  })
})

describe("the first shop", () => {
  const shop = (): RunState => {
    const state = apply(fresh(), [...guess(fresh().round.answer), { type: "collect" }])
    expect(state.phase).toBe("shop")
    return state
  }

  it("opens on the shelf, naming the kinds of card it deals", () => {
    const step = coachStep(shop())
    expect(step?.id).toBe("shelf")
    expect(step?.anchor).toBe(".shop-items")
    for (const kind of ["Relics", "Consumables", "Packs"]) expect(step?.text).toContain(kind)
  })

  it("stays on the shelf through a reroll", () => {
    const state = apply(shop(), [{ type: "reroll" }])
    expect(state.shop?.rerolls).toBe(1)
    expect(coachStep(state)?.id).toBe("shelf")
  })

  it("moves to the tray once a relic is held, quoting the run's own slot count", () => {
    const state: RunState = { ...shop(), relics: [{ id: "green_thumb" }] }
    const step = coachStep(state)
    expect(step?.id).toBe("relics")
    expect(step?.text).toContain(`${difficultyOf(state).relicSlots} slots`)
  })

  it("keeps quiet while a pack is open over the shelf", () => {
    const state: RunState = { ...shop(), pack: { id: "relic_pack", options: [], picks: 1 } }
    expect(coachStep(state)).toBeNull()
  })

  it("is spent on leaving, and says nothing in the next shop", () => {
    const next = apply(shop(), [{ type: "next_round" }])
    expect(coachSpent(next)).toBe(true)
    expect(coachStep({ ...next, phase: "shop" })).toBeNull()
  })
})

/**
 * Every beat's prose, checked once. The wording is the deliverable here, and a card
 * that fires at the right moment and says nothing useful is the failure mode
 * this whole file otherwise cannot see, so this pins the claims that would be
 * wrong if the content moved underneath them.
 */
describe("what the coach actually claims", () => {
  const words: WordSource = realWords

  it("names the color multipliers the engine uses", () => {
    const step = coachStep(apply(startRun(3, words).state, type(MISS)))
    expect(step?.text).toContain("yellow tile increases it by 1")
    expect(step?.text).toContain("each green by 3")
  })

  it("keeps every card to a length a phone can hold", () => {
    // Four lines at the ~60 characters a line holds in the card's text column
    // at 390×844. The solve card, the longest, once ran to 270 characters and
    // five lines walking the meter part by part, and covered the whole board's
    // top row doing it. It is about 200 now, worked sum included, and was
    // screenshotted at four lines, the last of them one word, with the card
    // ending above the board. It lies over the chips × mult row, which the
    // solve beat is not about; a fifth line would reach the tiles.
    let state = startRun(3, words).state
    const seen: string[] = []
    for (let count = 0; count < 4; count++) {
      seen.push(coachStep(state)?.text ?? "")
      state = apply(state, guess(MISS))
    }
    seen.push(coachStep(apply(played(startRun(3, words).state, 1), type(MISS)))?.text ?? "")
    seen.push(coachStep(apply(startRun(3, words).state, type("qu")))?.text ?? "")
    seen.push(coachStep(apply(startRun(3, words).state, type(MISS)))?.text ?? "")
    for (const text of seen) {
      expect(text.length).toBeGreaterThan(0)
      expect(text.length).toBeLessThanOrEqual(210)
    }
  })
})

/**
 * The question the intro card asks before any of the above is reachable.
 *
 * It is the one decision the tutorial offers, and it is offered once, so what
 * matters is that the window it is offered in is exactly one screen wide. A
 * round that is nearly the first round must not reopen it, and the round that is
 * must not still be asking after it has been played in.
 */
describe("the offer to skip the coaching", () => {
  it("is made on the round the cards would run on", () => {
    expect(coachAsks(fresh())).toBe(true)
  })

  it("is withdrawn the moment the round has been played in", () => {
    // A guess is an answer to the question by other means. The intro card is
    // long gone by here, and the offer must not come back with it if the player
    // walks to the title screen and resumes.
    expect(coachAsks(played(fresh(), 1))).toBe(false)
  })

  it("is not reopened by a later round that looks the same", () => {
    const state = apply(apply(fresh(), guess(fresh().round.answer)), [
      { type: "collect" },
      { type: "next_round" },
    ])
    expect(state.roundIndex).toBe(1)
    expect(state.round.guesses.length).toBe(0)
    expect(coachAsks(state)).toBe(false)
  })

  it("is not made away from the board", () => {
    const state = apply(fresh(), guess(fresh().round.answer))
    expect(state.phase).toBe("reward")
    expect(coachAsks(state)).toBe(false)
  })

  // The two are not each other's negation, and this is the seam. On the opening
  // board the offer stands and the tutorial is not spent; a caller that read
  // `!coachSpent` as "ask now" would ask on every screen in the game.
  it("is narrower than the question of whether the tutorial is finished", () => {
    const mid = apply(fresh(), type("ja"))
    expect(coachAsks(mid)).toBe(true)
    expect(coachSpent(mid)).toBe(false)
    const later = played(fresh(), 1)
    expect(coachAsks(later)).toBe(false)
    expect(coachSpent(later)).toBe(false)
  })
})

import { describe, expect, it } from "vitest"
import type { Action, Refusal, RunState, WordSource } from "../../src/engine"
import {
  ASCENSIONS,
  AUTHORED_ASCENSIONS,
  ascensionAt,
  clampAscension,
  difficultyAt,
  getBoss,
  keyboardColors,
  MAX_ASCENSION,
  MULT_FOR_COLOR,
  mustSolve,
  RELIC_SLOTS,
  RELICS,
  ROUND_PAYOUT,
  reduce,
  roundTargets,
  rulesFor,
  startRun,
} from "../../src/engine"
// The combined rule check is an internal seam: the reducer calls it, and this
// test needs to ask it things without playing a whole round to get there.
import { validateGuess } from "../../src/engine/ascensions"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "dairy", "ghost", "arose", "guild", "rabid", "acrid"]),
}

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, words).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** A run at a given level. Seed 1 draws BRAID, which every case here is about. */
const at = (ascension: number): RunState => startRun(1, words, ascension).state

/**
 * Types a word, submits it, and reports why it was refused, through the
 * reducer rather than by calling the rule, so what is under test is the rule as
 * the player meets it.
 *
 * The refusal comes back as the engine wrote it, code and operands, rather than
 * as the sentence the catalog would make of it. That is the sharper assertion of
 * the two: "must use A" pinned the letter only by accident of how English spells
 * it, and this pins it outright.
 */
function why(state: RunState, word: string): Refusal | undefined {
  const typed = [...word].reduce(
    (current, letter) => reduce(current, { type: "type_letter", letter }, words).state,
    state,
  )
  const { events } = reduce(typed, { type: "submit" }, words)
  return events.find((event) => event.type === "rejected")?.refusal
}

describe("the ladder", () => {
  it("is ten written steps, each arriving at its own level", () => {
    expect(ASCENSIONS.map((rule) => rule.level)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
    expect(AUTHORED_ASCENSIONS).toBe(10)
  })

  it("plays every rule up to the level, and none above it", () => {
    expect(rulesFor(at(0))).toHaveLength(0)
    expect(rulesFor(at(1)).map((rule) => rule.level)).toEqual([1])
    expect(rulesFor(at(4)).map((rule) => rule.level)).toEqual([1, 2, 3, 4])
    expect(rulesFor(at(AUTHORED_ASCENSIONS))).toHaveLength(AUTHORED_ASCENSIONS)
    // And the endless half adds no rules at all. It is a number, and `standing`
    // renders the number rather than twenty repetitions of one entry.
    expect(rulesFor(at(30))).toHaveLength(AUTHORED_ASCENSIONS)
  })

  it("keeps a level anyone could actually be at", () => {
    expect(clampAscension(-3)).toBe(0)
    expect(clampAscension(2.7)).toBe(2)
    expect(clampAscension(9999)).toBe(MAX_ASCENSION)
    expect(clampAscension(Number.NaN)).toBe(0)
    // And the run itself refuses a level off the dial rather than storing it.
    expect(startRun(1, words, 9999).state.ascension).toBe(MAX_ASCENSION)
  })

  /*
   * The half of the ladder nobody wrote. It exists so the dial keeps being a
   * thing to optimize against after the tenth rung has been beaten: the run's
   * own scoreboard, which "how many stages" and "final score" both fail at
   * because a won run can be farmed for either.
   */
  it("keeps going past the written rungs, one notch of target at a time", () => {
    const step = difficultyAt(11).targets / difficultyAt(10).targets
    // Gentler than Steeper's own 15%, because this one compounds ninety times:
    // at 15% the measured climb was spent within five rungs, and a scoreboard
    // with five marks on it is not a scoreboard.
    expect(step).toBeCloseTo(1.08, 10)
    // Compounding, not accumulating: rung 20 is the notch ten times over.
    expect(difficultyAt(20).targets).toBeCloseTo(difficultyAt(10).targets * step ** 10, 10)
    // And nothing but the targets moves up there.
    const thirtieth = difficultyAt(30)
    expect(thirtieth).toEqual({ ...difficultyAt(10), targets: thirtieth.targets })
    expect(thirtieth.targets).toBeGreaterThan(difficultyAt(10).targets)
  })

  it("has a rung above the written ones, so the dial always has something to say", () => {
    // The written rungs carry no numbers of their own; the synthesized one
    // carries both of them, which is what tells the two apart here now that
    // neither carries a name. `endless` is the flag and the operands at once.
    expect(ascensionAt(10)?.endless).toBeUndefined()
    const synthesized = ascensionAt(14)
    // The running total, because "another 8%" stops being useful by the third
    // time the dial says it, and the step, which is the same at every rung.
    expect(synthesized?.endless).toEqual({ percent: 8, total: difficultyAt(14).targets })
    expect(ascensionAt(0)).toBeUndefined()
    expect(ascensionAt(MAX_ASCENSION + 1)).toBeUndefined()
  })

  it("costs an ordinary run nothing at all, not even a field", () => {
    // The save written by a run at ascension 0 is the file it was before any of
    // this existed, which is what makes the whole feature free for most players.
    expect(at(0).ascension).toBeUndefined()
    expect(rulesFor(at(0))).toHaveLength(0)
    expect(mustSolve(at(0))).toBe(false)
  })
})

describe("the rules themselves", () => {
  it("1: makes you keep the letters you have found", () => {
    // AROSE against BRAID: R and A come back yellow.
    const played = apply(at(1), type("arose"))
    expect(why(played, "guild")).toEqual({ code: "must_use", letter: "a" })
    expect(why(played, "dairy")).toBeUndefined()
    // Off the ladder, the same board takes the same throwaway word.
    expect(why(apply(at(0), type("arose")), "guild")).toBeUndefined()
  })

  it("2: refuses a word already used this round", () => {
    const played = apply(at(2), type("crane"))
    expect(why(played, "crane")).toEqual({ code: "already_guessed_round" })
    expect(why(played, "ghost")).toBeUndefined()
  })

  it("3: raises every target, and rounds the result to something readable", () => {
    const [normal] = roundTargets(1)
    expect(at(2).round.target).toBe(normal)
    expect(at(3).round.target).toBe(Math.round((normal * 1.15) / 10) * 10)
    // Ten, not a hundred: 15% of the first target is 45, and rounding that to
    // the nearest hundred would make the first Steeper either free or double.
    expect(at(3).round.target % 10).toBe(0)
  })

  it("4: makes you keep the letters you have placed, wherever you like", () => {
    // ACRID against BRAID: A and R come back yellow, I and D land green.
    const played = apply(at(4), type("acrid"))
    // AROSE carries both yellows and neither green, so level 1 is satisfied and
    // this is the rule left to break.
    expect(why(played, "arose")).toEqual({ code: "must_use", letter: "i" })
    // DAIRY has every placed letter in it and neither of them where it was
    // placed, which is exactly what this level allows and the next will not.
    expect(why(played, "dairy")).toBeUndefined()
  })

  it("5: makes them stay where you put them, exactly as The Tyrant does", () => {
    const played = apply(at(5), type("acrid"))
    expect(why(played, "dairy")).toEqual({ code: "must_keep", letter: "i", position: 4 })
    expect(why(played, "braid")).toBeUndefined()
  })

  it("6: holds four relics instead of five, and says so when the fifth is offered", () => {
    expect(difficultyAt(5).relicSlots).toBe(RELIC_SLOTS)
    expect(difficultyAt(6).relicSlots).toBe(RELIC_SLOTS - 1)

    // The shelf is untouched: it still lays out its five cards, relics included.
    // What the rung takes is the room to keep them, so the offer still arrives
    // and now has to displace something: the decision a shelf-cut never asks for.
    const shelf = apply(apply(at(6), type("braid")), [{ type: "collect" }])
    expect(shelf.shop?.items).toHaveLength(5)

    // Four bought relics is a full tray here and a tray with a seat left one rung
    // down, which is the whole of the rule as the player meets it.
    const offered: RunState = {
      ...shelf,
      gold: 99,
      relics: RELICS.slice(0, 4).map((relic) => ({ id: relic.id })),
      shop: {
        rerolls: 0,
        items: [{ kind: "relic", id: RELICS[4]?.id ?? "", cost: 4 }],
      },
    }
    const refused = (state: RunState) =>
      reduce(state, { type: "buy", index: 0 }, words).events.some(
        (event) => event.type === "rejected",
      )
    expect(refused(offered)).toBe(true)
    expect(refused({ ...offered, ascension: 5 })).toBe(false)
  })

  it("7: takes a dollar off every round, and leaves the unused guesses alone", () => {
    const lean = apply(at(7), type("braid")).reward
    const plain = apply(at(6), type("braid")).reward
    expect(lean?.base).toBe((plain?.base ?? 0) - 1)
    expect(lean?.base).toBe(ROUND_PAYOUT[0] - 1)
    // The whole reason the cut lands on the base: the unused-guess dollars are
    // the one reward that pays for solving early rather than farming, so the
    // rung that squeezes the economy has to leave them untouched, and by
    // shrinking everything around them, make them matter more.
    expect(lean?.unusedGuesses).toBe(plain?.unusedGuesses)
  })

  it("8: pays nothing for a round cleared without the word", () => {
    expect(difficultyAt(7).unpaidIfUnsolved).toBe(false)
    expect(difficultyAt(8).unpaidIfUnsolved).toBe(true)
    // No rung takes a guess any more, at the top of the ladder or anywhere else.
    // The Clock's four still has to be the tighter of the two, which is the part
    // of the old "Five" worth keeping a test on. A run-level allowance that
    // made a boss round *easier* would be a rung nobody could reason about.
    expect(at(8).round.maxGuesses).toBe(difficultyAt(0).guesses)
    const clock = getBoss("clock")?.maxGuesses ?? 0
    expect(Math.min(clock, difficultyAt(8).guesses)).toBe(clock)
  })

  it("9: refuses a word used anywhere in the run", () => {
    const played = apply(at(9), type("crane"))
    expect(played.history).toEqual(["crane"])
    // Carried across a round, which is the whole difference from rule 2: the
    // round's own guesses are gone, the run's memory is not.
    const nextRound: RunState = { ...played, round: { ...at(9).round } }
    expect(why(nextRound, "crane")).toEqual({ code: "already_used_run" })
  })

  it("10: is not a guess rule but a verdict on the round", () => {
    expect(mustSolve(at(10))).toBe(true)
    expect(mustSolve(at(9))).toBe(false)
    expect(ASCENSIONS[9]?.validate).toBeUndefined()
  })
})

describe("finishing the word", () => {
  const farmed = (ascension: number): RunState => {
    const base = startRun(1, words, ascension).state
    // At target, out of guesses, and the word never found: the farm-and-fail-out
    // line, which is a win everywhere except the top of the ladder.
    return {
      ...base,
      round: { ...base.round, target: 1, score: 500, done: false, guesses: [], maxGuesses: 1 },
    }
  }

  it("pays out a farmed round at every level below the last", () => {
    expect(apply(farmed(AUTHORED_ASCENSIONS - 1), type("ghost")).phase).toBe("reward")
  })

  it("takes the run at the last, however big the pile is", () => {
    const dead = apply(farmed(AUTHORED_ASCENSIONS), type("ghost"))
    expect(dead.phase).toBe("game_over")
    expect(dead.round.score).toBeGreaterThan(dead.round.target)
  })

  it("still pays out when the word was actually solved", () => {
    expect(apply(farmed(AUTHORED_ASCENSIONS), type("braid")).phase).toBe("reward")
  })
})

describe("the order of refusals", () => {
  const underGlutton = (ascension: number): RunState => {
    const base = startRun(1, words, ascension).state
    return { ...base, round: { ...base.round, bossId: "glutton" } }
  }

  it("answers with the run's rule before the round's", () => {
    // GHOST breaks both at once: one vowel, and neither of the letters AROSE
    // just found. The player has to hear the same thing every time, and the
    // rule that holds every round of the run is the one to hear about.
    const played = apply(underGlutton(1), type("arose"))
    expect(why(played, "ghost")).toEqual({ code: "must_use", letter: "a" })
  })

  it("still answers with the round's rule when that is the only one broken", () => {
    expect(why(underGlutton(1), "ghost")).toEqual({ code: "needs_two_vowels" })
  })

  it("reads what happened rather than what was shown", () => {
    // The Mirror moves feedback to positions it did not come from. A rule built
    // on the board could demand a letter that is in no word at all; built on the
    // real colors, it can only ever demand letters the answer really has.
    const base = startRun(1, words, 1).state
    const under: RunState = { ...base, round: { ...base.round, bossId: "mirror" } }
    const played = apply(under, type("arose"))
    const shown = played.round.guesses[0]?.tiles.map((tile) => tile.shown)
    const real = played.round.guesses[0]?.tiles.map((tile) => tile.color)
    expect(shown).not.toEqual(real)
    // BRAID is the answer, so it satisfies a rule drawn from its own feedback,
    // whatever the board claimed.
    expect(why(played, "braid")).toBeUndefined()
  })
})

describe("the answer is always reachable", () => {
  const many: WordSource = {
    answers: ["braid", "crane", "ghost", "dairy", "arose", "guild"],
    allowed: new Set(["braid", "crane", "ghost", "dairy", "arose", "guild"]),
  }

  it("never deals a word the run is forbidden to type", () => {
    // Ascension 9 forbids repeating a word, so a round whose answer has already
    // been guessed is a round that cannot be solved, and at ascension 10 that is
    // a round that cannot be won at all. The pool has to know that.
    const base = startRun(4, many, AUTHORED_ASCENSIONS).state
    const used = many.answers.filter((word) => word !== "guild")
    const state: RunState = { ...base, history: used, stage: 1, roundIndex: 1 }
    // Dealt through the real path, with the shop left behind and the next round
    // begun, which is the only route that redraws an answer.
    const played = reduce({ ...state, phase: "shop", shop: null }, { type: "next_round" }, many)
    expect(used).not.toContain(played.state.round.answer)
    expect(played.state.round.answer).toBe("guild")
  })

  it("hands every rule a board its own answer satisfies", () => {
    // The property the whole design rests on: rules read real feedback, so the
    // answer always passes them. Played out over a full round at the top of the
    // ladder rather than argued about.
    let state = startRun(9, many, AUTHORED_ASCENSIONS).state
    const answer = state.round.answer
    for (const guess of ["crane", "ghost", "arose"]) {
      if (guess === answer) continue
      expect(validateGuess(answer, state), `after ${guess}`).toBeNull()
      const next = apply(state, type(guess))
      if (next.phase !== "round") break
      state = next
    }
    expect(validateGuess(answer, state)).toBeNull()
  })

  it("does not let The Magician's yellow become a letter the answer has to contain", () => {
    // The bug this exists to keep out, found in a real run: a stage 8 board at
    // ascension 10 promoted the H of CHOMP against an answer of CIVIC, the H
    // entered `found()` as a proven letter, and the run's own answer became an
    // illegal guess with three guesses still in hand. The card is worth mult; it
    // is not evidence, and the only reader that may treat it as either is the
    // one that prices it.
    const state = at(10)
    state.consumables.push({ id: "magician" })
    const cast = reduce(state, { type: "use_consumable", index: 0 }, words).state
    // Every letter of GHOST is absent from BRAID, so the promotion lands on the
    // G and invents a yellow for a letter in no word here at all.
    const played = apply(cast, type("ghost"))
    const tile = played.round.guesses[0]?.tiles[0]
    expect(tile).toMatchObject({ letter: "g", color: "yellow", shown: "yellow", promoted: true })
    expect(why(played, "braid")).toBeUndefined()

    // And the flag is what is doing the work, rather than the level happening
    // not to ask: the same board with the mark rubbed off refuses the answer.
    const unmarked = JSON.parse(JSON.stringify(played)) as RunState
    delete unmarked.round.guesses[0]?.tiles[0]?.promoted
    expect(why(unmarked, "braid")).toEqual({ code: "must_use", letter: "g" })
  })

  it("still pays The Magician's tile as a yellow", () => {
    // The other half of the same fix, and the reason `color` rather than `shown`
    // carries the promotion: a yellow that scored like a gray would be no card.
    const armed = at(0)
    armed.consumables.push({ id: "magician" })
    const cast = reduce(armed, { type: "use_consumable", index: 0 }, words).state
    const promoted = apply(cast, type("ghost")).round.guesses[0]
    const plain = apply(at(0), type("ghost")).round.guesses[0]
    expect(promoted?.mult).toBe((plain?.mult ?? 0) + MULT_FOR_COLOR.yellow)
  })

  it("reads a promoted letter back to the keyboard as the gray it was", () => {
    // The board shows the color, because the card really did paint it. The
    // keyboard is a claim about the answer, so it says what the tile fell as.
    const armed = at(0)
    armed.consumables.push({ id: "magician" })
    const cast = reduce(armed, { type: "use_consumable", index: 0 }, words).state
    const played = apply(cast, type("ghost"))
    expect(keyboardColors(played.round.guesses).get("g")).toBe("gray")
  })
})

describe("what a run carries", () => {
  it("writes down every word it has said, whatever the level", () => {
    const played = apply(at(0), [...type("crane"), ...type("ghost")])
    expect(played.history).toEqual(["crane", "ghost"])
  })

  it("stays plain JSON with the ladder on it", () => {
    const played = apply(at(AUTHORED_ASCENSIONS), type("crane"))
    expect(JSON.parse(JSON.stringify(played))).toEqual(played)
  })

  it("starts a run at the level the action asked for", () => {
    const { state } = reduce(at(0), { type: "start_run", seed: 3, ascension: 4 }, words)
    expect(state.ascension).toBe(4)
    // And omits it entirely when nobody asked for one.
    expect(reduce(at(0), { type: "start_run", seed: 3 }, words).state.ascension).toBeUndefined()
  })
})

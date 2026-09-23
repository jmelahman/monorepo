import { beforeEach, describe, expect, it } from "vitest"
import type { ModId, RunState, WordSource } from "../../src/engine"
import { draftChips, reduce, startRun } from "../../src/engine"

/**
 * A word source narrow enough that the answer is known in advance. The engine
 * takes its word lists as an argument precisely so tests can do this.
 */
const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "quazy", "dairy", "jazzy", "arose", "aloes", "zonks"]),
}

const play = (state: RunState, word: string): RunState => {
  let current = state
  for (const letter of word) current = reduce(current, { type: "type_letter", letter }, words).state
  return reduce(current, { type: "submit" }, words).state
}

describe("guess scoring", () => {
  let start: RunState

  beforeEach(() => {
    start = startRun(1, words).state
  })

  it("draws the answer from the supplied list", () => {
    expect(start.round.answer).toBe("braid")
  })

  /**
   * The worked example from the design doc, played end to end.
   *
   *   Q U A Z Y   ..G..   chips 26  mult  4  ->  104
   *   C R A N E   .GG..   chips  7  mult  7  ->   49
   *   B R A I D   GGGGG   chips  8  mult 16  ->  128
   *                                     banked  281, solve x4 -> 1124
   *
   * The solve bonus multiplies the round, not the guess that landed it, so the
   * two halves of the game compose instead of competing: QUAZY's 104 is worth
   * 416 by the end. What the player is really choosing is when to stop growing
   * the pile, because every guess spent growing it costs a point of multiplier.
   */
  it("reproduces the design doc's worked example", () => {
    let state = play(start, "quazy")
    expect(state.round.guesses[0]).toMatchObject({ chips: 26, mult: 4, score: 104 })

    state = play(state, "crane")
    expect(state.round.guesses[1]).toMatchObject({ chips: 7, mult: 7, score: 49 })

    // The guess records its own arithmetic; the bonus is the round's, not its.
    state = play(state, "braid")
    expect(state.round.guesses[2]).toMatchObject({ chips: 8, mult: 16, solveBonus: 4, score: 128 })

    expect(state.round.score).toBe((104 + 49 + 128) * 4)
    expect(state.round.solved).toBe(true)
  })

  /*
   * The rule that pays for the whole design: solving late on a fat pile beats
   * solving early on an empty one, but only up to the point where the shrinking
   * multiplier eats the gain. Both lines below solve; one banks first.
   */
  it("multiplies everything banked this round, not just the solving guess", () => {
    const early = play(start, "braid")
    const late = play(play(start, "quazy"), "braid")

    expect(early.round.score).toBe(128 * 6)
    expect(late.round.score).toBe((104 + 128) * 5)
    expect(late.round.score).toBeGreaterThan(early.round.score)
  })

  it("builds mult from the feedback: 1 + 3 per green + 1 per yellow", () => {
    const state = play(start, "dairy")
    // Four yellows, no greens.
    expect(state.round.guesses[0]?.mult).toBe(5)
  })

  it("values rare letters far above common ones", () => {
    // The tension in one assertion: QUAZY banks five times the chips of AROSE
    // while telling you almost nothing, and AROSE is the better probe.
    expect(play(start, "quazy").round.guesses[0]?.chips).toBe(26)
    expect(play(start, "arose").round.guesses[0]?.chips).toBe(5)
  })

  it("ends the round the moment the word is solved, forfeiting the rest", () => {
    const state = play(start, "braid")
    expect(state.round.done).toBe(true)
    expect(state.round.guesses).toHaveLength(1)
    // Solved on guess 1 of 6: five guesses remain, so the bonus is x6.
    expect(state.round.guesses[0]?.solveBonus).toBe(6)
  })

  it("shrinks the solve bonus the longer you take", () => {
    const bonuses = ["quazy", "crane", "dairy", "arose", "aloes"].map((_, index) => {
      let state = start
      for (let i = 0; i < index; i++) {
        state = play(state, ["quazy", "crane", "dairy", "arose", "aloes"][i] as string)
      }
      return play(state, "braid").round.guesses[index]?.solveBonus
    })
    expect(bonuses).toEqual([6, 5, 4, 3, 2])
  })

  /*
   * The live readout's promise, pinned against the thing it is a readout of.
   *
   * `draftChips` exists so the board can count a word while it is being typed,
   * and its only obligation is that the guess never comes in *under* the figure
   * the player was shown. Testing it against the real pipeline rather than
   * against arithmetic is the point: an effect added later that subtracts chips
   * would break the promise silently everywhere else, and here.
   */
  describe("the drafting readout", () => {
    const typed = (state: RunState, word: string): RunState =>
      [...word].reduce(
        (current, letter) => reduce(current, { type: "type_letter", letter }, words).state,
        state,
      )

    it("counts the letters as they are typed", () => {
      // Q, then QU, then the whole of QUAZY: 10, 11, 26.
      expect(draftChips(start, "")).toBe(0)
      expect(draftChips(start, "q")).toBe(10)
      expect(draftChips(start, "qu")).toBe(11)
      expect(draftChips(start, "quazy")).toBe(26)
    })

    it("never promises more than the guess pays", () => {
      for (const word of ["quazy", "crane", "arose", "jazzy"]) {
        const drafting = typed(start, word)
        const shown = draftChips(drafting, word)
        const paid = reduce(drafting, { type: "submit" }, words).state.round.guesses[0]?.chips ?? 0
        expect(paid).toBeGreaterThanOrEqual(shown)
      }
    })

    it("prices the letters the way the boss in play will", () => {
      // The Drought pays nothing for vowels, and says so before the guess is
      // spent rather than after, which is the whole reason the boss hook is
      // consulted here instead of the raw letter values being summed.
      const drought = { ...start, round: { ...start.round, bossId: "drought" } }
      expect(draftChips(drought, "quazy")).toBe(24)
      expect(
        reduce(typed(drought, "quazy"), { type: "submit" }, words).state.round.guesses[0],
      ).toMatchObject({ chips: 24 })
    })
  })

  it("rejects a word that is not in the allowed list", () => {
    let state = start
    for (const letter of "zzzzz") {
      state = reduce(state, { type: "type_letter", letter }, words).state
    }
    const { state: after, events } = reduce(state, { type: "submit" }, words)
    expect(events).toEqual([{ type: "rejected", refusal: { code: "not_in_word_list" } }])
    expect(after.round.guesses).toHaveLength(0)
    expect(after.round.draft).toBe("zzzzz")
  })

  /*
   * The row's own record of how it was scored, column by column, which is what
   * the tip on a played tile reads back an hour later.
   *
   * Written down rather than re-derived, and the tests below are the argument
   * for that: two of the numbers here cannot be recovered from a finished round
   * at all. See `TileScore`.
   */
  describe("what a row writes down about itself", () => {
    const underBoss = (bossId: string): RunState => ({
      ...start,
      round: { ...start.round, bossId },
    })

    const withMod = (state: RunState, letter: string, mod: ModId): RunState => {
      const entry = state.letters[letter]
      if (!entry) throw new Error(`no such letter: ${letter}`)
      return { ...state, letters: { ...state.letters, [letter]: { ...entry, mod } } }
    }

    it("itemises the chips the guess banked", () => {
      // QUAZY's 26, letter by letter.
      expect(play(start, "quazy").round.guesses[0]?.paid?.map((tile) => tile.base)).toEqual([
        10, 1, 1, 10, 4,
      ])
    })

    it("records what the boss charged rather than what the letters are worth", () => {
      const drought = play(underBoss("drought"), "quazy")
      expect(drought.round.guesses[0]?.paid?.map((tile) => tile.base)).toEqual([10, 0, 0, 10, 4])
    })

    /*
     * The columns against the row they add up to, which is what the tip sets
     * one beside the other. Chips are exactly attributable, since every one of
     * the 26 came from a letter, and the mult column carries the colors: one
     * green in QUAZY, so 3 of the row's 4, the missing 1 being the mult every
     * row starts with and no letter earns.
     */
    it("measures what each column moved the row's two numbers by", () => {
      const guess = play(start, "quazy").round.guesses[0]
      expect(guess?.paid?.map((tile) => tile.chips)).toEqual([10, 1, 1, 10, 4])
      expect(guess?.paid?.map((tile) => tile.mult)).toEqual([0, 0, 3, 0, 0])
      expect(guess?.paid?.reduce((total, tile) => total + tile.chips, 0)).toBe(guess?.chips)
    })

    /*
     * Chips a column paid but the letter did not: the relic's 8 lands on the
     * column that earned it, so the tip can say "9 of 34" on the green and "10
     * of 34" on the Q. `base` stays what the tile floated as it turned over.
     */
    it("counts a relic's chips into the column that earned them", () => {
      const guess = play({ ...start, relics: [{ id: "green_thumb" }] }, "quazy").round.guesses[0]
      expect(guess?.paid?.[2]).toMatchObject({ base: 1, chips: 9, mult: 3 })
      expect(guess?.paid?.reduce((total, tile) => total + tile.chips, 0)).toBe(guess?.chips)
    })

    /*
     * A ×mult card has no worth of its own. It is worth whatever was standing
     * in front of it, so the column records what the row actually gained where
     * it fired. Steel on CRANE's first tile doubles a mult of 1, and a 1 is what
     * goes down; the same card on the last tile would record 8.
     */
    it("prices a multiplying card at what it was worth where it fired", () => {
      const first = play(withMod(start, "c", "steel"), "crane").round.guesses[0]
      expect(first?.paid?.[0]).toMatchObject({ base: 3, chips: 3, mult: 1 })
      expect(first?.mult).toBe(8)

      const last = play(withMod(start, "e", "steel"), "crane").round.guesses[0]
      expect(last?.paid?.[4]).toMatchObject({ base: 1, chips: 1, mult: 7 })
      expect(last?.mult).toBe(14)
    })

    /*
     * The whole reason this is a record and not a calculation. The Miser prices
     * a letter by whether the round has already spent it, so asking again after
     * the fact answers about a different round: by the second guess every
     * letter of the first has been spent, and a re-derived first row would
     * report the 0 chips it would score *now* instead of the 26 it scored.
     */
    it("survives a boss whose prices depend on the round so far", () => {
      const miser = play(play(underBoss("miser"), "quazy"), "arose")
      expect(miser.round.guesses[0]?.chips).toBe(26)
      expect(miser.round.guesses[0]?.paid?.map((tile) => tile.base)).toEqual([10, 1, 1, 10, 4])
      // And what re-deriving that row would have said: A is spent, so the same
      // letter in the next guess is charged nothing.
      expect(miser.round.guesses[1]?.paid?.[0]?.base).toBe(0)
    })

    it("records what the modifier paid, and only where it fired", () => {
      const steel = play(withMod(start, "c", "steel"), "crane")
      expect(steel.round.guesses[0]?.paid?.[0]).toEqual({
        base: 3,
        chips: 3,
        mult: 1,
        mod: { kind: "times", factor: 2 },
      })
      expect(steel.round.guesses[0]?.paid?.[1]).toEqual({ base: 1, chips: 1, mult: 3 })
    })

    /*
     * The other unrecoverable one, and the subtler of the two: a card that was
     * asked and declined leaves nothing behind. Anchor is the deterministic
     * version of Lucky's problem, wanting a green where this tile is gray, and
     * the silence it leaves has to be told apart from The Vandal's, which is why
     * the two are asserted side by side.
     */
    it("stays quiet for a modifier that fired and had nothing to say", () => {
      const anchor = play(withMod(start, "c", "anchor"), "crane")
      expect(anchor.round.guesses[0]?.paid?.[0]).toEqual({ base: 3, chips: 3, mult: 0 })
    })

    it("stays quiet for a modifier the boss silenced", () => {
      const vandal = play(withMod(underBoss("vandal"), "c", "steel"), "crane")
      expect(vandal.round.guesses[0]?.paid?.[0]).toEqual({ base: 3, chips: 3, mult: 0 })
      expect(vandal.round.guesses[0]?.mult).toBe(7)
    })

    it("names the relics that paid on a tile, and only the tiles they paid on", () => {
      // Green Thumb wants a green, and QUAZY against BRAID has exactly one.
      const state = play({ ...start, relics: [{ id: "green_thumb" }] }, "quazy")
      expect(state.round.guesses[0]?.paid?.[2]).toEqual({
        base: 1,
        chips: 9,
        mult: 3,
        relics: [{ id: "green_thumb", label: { kind: "chips", amount: 8 } }],
      })
      expect(state.round.guesses[0]?.paid?.[0]).toEqual({ base: 10, chips: 10, mult: 0 })
    })

    /*
     * Two slots on one tile, which is the case the letter's own modifier never
     * has: the first Z of JAZZY is a rare letter *and* a repeat, so it pays Q's
     * Bargain and Doppelgänger both, and the two have to survive each other.
     * Slot order, so the list reads along the tray.
     */
    it("keeps every relic that fired on the same tile, in slot order", () => {
      const state = play(
        { ...start, relics: [{ id: "qs_bargain" }, { id: "doppelganger" }] },
        "jazzy",
      )
      expect(state.round.guesses[0]?.paid?.[2]).toEqual({
        base: 10,
        chips: 40,
        mult: 0,
        relics: [
          { id: "qs_bargain", label: { kind: "chips", amount: 20 } },
          { id: "doppelganger", label: { kind: "chips", amount: 10 } },
        ],
      })
    })

    /*
     * A relic that prices the finished row belongs to no letter in it. Hanging
     * Anagrammer's ×2 on all five tiles would read as five doublings, and
     * hanging it on one would be a lie about which letter earned it.
     */
    it("leaves the guess-wide relics out of the tiles entirely", () => {
      const state = play({ ...start, relics: [{ id: "anagrammer" }] }, "quazy")
      expect(state.round.guesses[0]?.mult).toBe(8)
      expect(state.round.guesses[0]?.paid?.some((tile) => tile.relics)).toBe(false)
    })
  })
})

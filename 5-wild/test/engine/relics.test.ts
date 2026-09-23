import { describe, expect, it } from "vitest"
import type { Action, GuessRecord, RunState, WordSource } from "../../src/engine"
import { INTEREST_CAP, RELICS, reduce, startRun } from "../../src/engine"
import { realWords } from "../helpers/words"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "quazy", "dairy", "ghost", "sassy", "arose"]),
}

const apply = (state: RunState, actions: Action[], source = words): RunState =>
  actions.reduce((current, action) => reduce(current, action, source).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** Plays one guess with a relic equipped and hands back the scored record. */
function withRelic(id: string, ...guesses: string[]): { last: GuessRecord; state: RunState } {
  const base = startRun(1, words).state
  let state: RunState = { ...base, relics: [{ id }] }
  for (const word of guesses) state = apply(state, type(word))
  const last = state.round.guesses[state.round.guesses.length - 1]
  if (!last) throw new Error("no guess was scored")
  return { last, state }
}

describe("relics", () => {
  // Baselines, so every expectation below reads as a delta rather than a magic
  // number: CRANE is 7 chips x 7 mult, QUAZY is 26 x 4.
  it("scores nothing extra with no relics", () => {
    const base = startRun(1, words).state
    const state = apply(base, type("crane"))
    expect(state.round.guesses[0]).toMatchObject({ chips: 7, mult: 7, score: 49 })
  })

  it("Green Thumb pays 8 chips per green", () => {
    // CRANE lands two greens: 7 + 16 chips.
    expect(withRelic("green_thumb", "crane").last).toMatchObject({ chips: 23, mult: 7 })
  })

  it("Vowel Hoarder pays 4 mult per vowel", () => {
    // A and E: 7 + 8 mult.
    expect(withRelic("vowel_hoarder", "crane").last).toMatchObject({ chips: 7, mult: 15 })
  })

  it("Greedy Grammarian pays 15 chips per gray", () => {
    // QUAZY leaves four grays: 26 + 60 chips.
    expect(withRelic("greedy_grammarian", "quazy").last).toMatchObject({ chips: 86, mult: 4 })
  })

  it("Masochist pays 8 mult per gray", () => {
    expect(withRelic("masochist", "quazy").last).toMatchObject({ chips: 26, mult: 36 })
  })

  it("Q's Bargain triples the rare letters", () => {
    // Q and Z are 10 each, so each adds another 20.
    expect(withRelic("qs_bargain", "quazy").last).toMatchObject({ chips: 66, mult: 4 })
  })

  it("Doppelgänger scores repeated letters twice", () => {
    // SASSY: three Ss at 1 chip each pay again.
    expect(withRelic("doppelganger", "sassy").last.chips).toBe(11)
  })

  it("Alphabetist doubles an in-order word", () => {
    // GHOST is non-decreasing and misses entirely: 9 chips, mult 1 doubled.
    expect(withRelic("alphabetist", "ghost").last).toMatchObject({ chips: 9, mult: 2 })
  })

  it("Consonant Cluster wants three consonants in a row", () => {
    // SASSY ends S-S-Y and lands a single yellow: mult 2, then x1.5.
    expect(withRelic("consonant_cluster", "sassy").last.mult).toBe(3)
    expect(withRelic("consonant_cluster", "crane").last.mult).toBe(7) // no run, untouched
  })

  it("Slow Burn pays for stalling", () => {
    expect(withRelic("slow_burn", "crane").last.mult).toBe(7)
    expect(withRelic("slow_burn", "crane", "crane").last.mult).toBe(12)
  })

  it("Speedrunner triples a fast solve", () => {
    const { last } = withRelic("speedrunner", "braid")
    expect(last).toMatchObject({ chips: 8, mult: 48, solveBonus: 6 })
  })

  it("Speedrunner does nothing on a slow one", () => {
    const { last } = withRelic("speedrunner", "crane", "crane", "crane", "braid")
    expect(last.mult).toBe(16)
  })

  it("Cold Open pays the opening probe and nothing after it", () => {
    expect(withRelic("cold_open", "crane").last.chips).toBe(37)
    expect(withRelic("cold_open", "crane", "crane").last.chips).toBe(7)
  })

  it("Bloodhound pays chips per yellow", () => {
    // DAIRY lands four yellows: 9 + 24 chips.
    expect(withRelic("bloodhound", "dairy").last).toMatchObject({ chips: 33, mult: 5 })
  })

  it("Anagrammer doubles a word with five distinct letters", () => {
    expect(withRelic("anagrammer", "crane").last.mult).toBe(14)
    // SASSY repeats S three times, so it earns nothing: its lone yellow A is
    // the whole of that 2, exactly as it would be with no relic at all.
    expect(withRelic("anagrammer", "sassy").last.mult).toBe(2)
    expect(apply(startRun(1, words).state, type("sassy")).round.guesses[0]?.mult).toBe(2)
  })

  it("Sunk Cost pays for the guesses you are about to give up", () => {
    // Guess one of six leaves five behind: +50 mult on top of CRANE's 7.
    expect(withRelic("sunk_cost", "crane").last.mult).toBe(57)
    expect(withRelic("sunk_cost", "crane", "crane").last.mult).toBe(47)
  })

  it("The Vault pays chips for stalling, the way Slow Burn pays mult", () => {
    expect(withRelic("vault", "crane").last.chips).toBe(7)
    expect(withRelic("vault", "crane", "crane").last.chips).toBe(32)
  })

  it("The Long Game buys back a point of solve multiplier", () => {
    // Solving on guess one is x6; with this it is x7, and the pile it
    // multiplies is the whole round rather than this guess.
    const { last, state } = withRelic("long_game", "braid")
    expect(last.solveBonus).toBe(7)
    expect(state.round.score).toBe(last.score * 7)
  })

  it("Scavenger pays gold per yellow", () => {
    const { state } = withRelic("scavenger", "dairy")
    expect(state.gold).toBe(startRun(1, words).state.gold + 4)
  })

  it("Pyromaniac breaks a letter out of the alphabet, and the answer respects it", () => {
    // Needs a real round transition, since breaking happens before the draw.
    let state = startRun(1, realWords).state
    state = apply(state, type(state.round.answer), realWords)
    state = apply(state, [{ type: "collect" }], realWords)
    state = { ...state, relics: [{ id: "pyromaniac" }] }
    state = apply(state, [{ type: "next_round" }], realWords)

    const broken = Object.entries(state.letters)
      .filter(([, letter]) => letter.destroyed)
      .map(([name]) => name)

    expect(broken).toHaveLength(1)
    expect(state.round.answer).not.toContain(broken[0])
  })

  it("refuses to type a broken letter", () => {
    const base = startRun(1, words).state
    const state: RunState = {
      ...base,
      letters: { ...base.letters, q: { etch: 0, destroyed: true, mod: null } },
    }
    const { events } = reduce(state, { type: "type_letter", letter: "q" }, words)
    expect(events).toEqual([{ type: "rejected", refusal: { code: "letter_broken", letter: "q" } }])
  })

  it("gives every relic a distinct id and a price", () => {
    expect(new Set(RELICS.map((relic) => relic.id)).size).toBe(RELICS.length)
    for (const relic of RELICS) expect(relic.cost).toBeGreaterThan(0)
  })
})

/*
 * The five that close the gaps the build rubric found. Two are terminals for
 * archetypes that previously had no way to cash in, money and sacrifice, and
 * three grow, one per lifecycle hook, which is what P1's `data` was built for.
 */
describe("the relics that close a build", () => {
  const held = (id: string, gold: number, word: string): GuessRecord => {
    const base = startRun(1, words).state
    const state: RunState = { ...base, gold, relics: [{ id }] }
    const last = apply(state, type(word)).round.guesses[0]
    if (!last) throw new Error("no guess was scored")
    return last
  }

  it("The Mint turns a held pile into mult, five dollars at a time", () => {
    // CRANE is 7 x 7. At $23 that is four whole $5 steps, so +12 mult.
    expect(held("mint", 23, "crane")).toMatchObject({ chips: 7, mult: 19 })
    expect(held("mint", 4, "crane")).toMatchObject({ mult: 7 })
  })

  it("The Mint takes the interest away, which is what pays for it", () => {
    const base = startRun(1, words).state
    // Parked at the end of a cleared round with enough gold to cap interest.
    const cleared = (relics: RunState["relics"]): number => {
      const state: RunState = { ...base, gold: 40, relics, round: { ...base.round, target: 1 } }
      return apply(state, type("braid")).reward?.interest ?? -1
    }
    expect(cleared([])).toBe(INTEREST_CAP)
    expect(cleared([{ id: "mint" }])).toBe(0)
  })

  it("Scorched Earth pays for every letter the run has broken", () => {
    const base = startRun(1, words).state
    const letters = { ...base.letters }
    for (const letter of "jkvwxz") letters[letter] = { etch: 0, destroyed: true, mod: null }
    const state: RunState = { ...base, letters, relics: [{ id: "scorched_earth" }] }
    // Six broken: 7 + 72 mult, and none of those letters is in CRANE.
    expect(apply(state, type("crane")).round.guesses[0]).toMatchObject({ chips: 7, mult: 79 })
  })

  it("Scorched Earth is worth nothing to a run that has broken nothing", () => {
    expect(withRelic("scorched_earth", "crane").last).toMatchObject({ mult: 7 })
  })

  it("Snowball pays what it banked, then counts the guess that grew it", () => {
    // CRANE lands two greens against BRAID. The first guess pays nothing and
    // banks +2; the second pays that 2 and banks another.
    const { state } = withRelic("snowball", "crane")
    expect(state.round.guesses[0]).toMatchObject({ mult: 7 })
    expect(state.relics[0]?.data).toEqual({ mult: 2 })

    const second = apply(state, type("crane")).round.guesses[1]
    expect(second).toMatchObject({ mult: 9 })
  })

  it("Snowball survives the save round trip with its growth intact", () => {
    const { state } = withRelic("snowball", "crane")
    const revived = JSON.parse(JSON.stringify(state)) as RunState
    expect(apply(revived, type("crane")).round.guesses[1]?.mult).toBe(9)
  })

  it("Hot Streak grows on a fast clear and not on a slow one", () => {
    const base = startRun(1, words).state
    // Target of 1, so any guess clears it; the guess count is what differs.
    const run = (probes: string[]): RunState => {
      let state: RunState = {
        ...base,
        relics: [{ id: "hot_streak" }],
        round: { ...base.round, target: 1 },
      }
      for (const word of probes) state = apply(state, type(word))
      return apply(state, type("braid"))
    }
    expect(run([]).relics[0]?.data).toEqual({ chips: 30 })
    // Four guesses to solve is past the three-guess line: nothing banked.
    expect(run(["crane", "quazy", "dairy"]).relics[0]?.data).toBeUndefined()
  })

  it("The Hoarder grows only when both card slots are full", () => {
    const base = startRun(1, words).state
    const shopped = (consumables: RunState["consumables"]): RunState => {
      const state: RunState = {
        ...base,
        consumables,
        relics: [{ id: "hoarder" }],
        phase: "reward",
        reward: { base: 0, unusedGuesses: 0, interest: 0, total: 0 },
      }
      return reduce(state, { type: "collect" }, words).state
    }
    expect(shopped([]).relics[0]?.data).toBeUndefined()
    expect(shopped([{ id: "oracle" }]).relics[0]?.data).toBeUndefined()
    expect(shopped([{ id: "oracle" }, { id: "hermit" }]).relics[0]?.data).toEqual({ chips: 40 })
  })

  it("announces every growth, so the card visibly gets bigger", () => {
    const base = startRun(1, words).state
    const state: RunState = {
      ...base,
      relics: [{ id: "snowball" }],
      round: { ...base.round, draft: "crane" },
    }
    const { events } = reduce(state, { type: "submit" }, words)
    expect(events).toContainEqual({
      type: "relic_grew",
      slot: 0,
      id: "snowball",
      amount: 2,
      unit: "mult",
    })
  })

  it("wears what it has grown to, so the board never has to be guessed at", () => {
    for (const id of ["snowball", "hot_streak", "hoarder"]) {
      const relic = RELICS.find((entry) => entry.id === id)
      expect(relic?.growth, `${id} has no growth`).toBeDefined()
      expect(relic?.growth?.({ id }).amount).toBe(0)
      expect(relic?.growth?.({ id, data: { mult: 5, chips: 5 } }).amount).toBe(5)
    }
  })
})

/*
 * The word-shape and position cards. Each expectation names the shape it is
 * checking rather than just the number, because the numbers here were chosen
 * from how often the word list actually produces that shape. A test that only
 * pinned the arithmetic would let the shape drift silently.
 */
describe("the relics that read the word's shape", () => {
  it("Head Start pays for a vowel in the first column and nothing for one later", () => {
    // AROSE opens on a vowel; GHOST does not. Both keep their own mult.
    expect(withRelic("head_start", "arose").last).toMatchObject({ mult: 5 + 15 })
    expect(withRelic("head_start", "ghost").last).toMatchObject({ mult: 1 })
    // CRANE holds three vowels and starts on a consonant, which is the case the
    // card is priced against: the column is the condition, not the vowel count.
    expect(withRelic("head_start", "crane").last).toMatchObject({ mult: 7 })
  })

  it("Keystone doubles on a green middle and leaves any other green alone", () => {
    // QUAZY against BRAID lands its A in the middle column: 4 mult becomes 12.
    expect(withRelic("keystone", "quazy").last).toMatchObject({ chips: 26, mult: 12 })
    // GHOST lands nothing at all, so there is nothing to double.
    expect(withRelic("keystone", "ghost").last).toMatchObject({ mult: 1 })
  })

  it("The Chorus wants three vowels and counts them in the word, not on the board", () => {
    // AROSE is A-O-E: 5 mult becomes 15, and only one of those vowels is even
    // in the answer: the shape is the condition, the feedback is not.
    expect(withRelic("chorus", "arose").last).toMatchObject({ mult: 15 })
    // CRANE has A and E only.
    expect(withRelic("chorus", "crane").last).toMatchObject({ mult: 7 })
  })

  it("Lexicographer counts the alphabet already spent, and never the guess itself", () => {
    // The opening guess has nothing behind it, so it pays nothing, on the same
    // rule Slow Burn and The Vault follow. GHOST is 9 chips on its own.
    expect(withRelic("lexicographer", "ghost").last).toMatchObject({ chips: 9 })
    // GHOST spent five distinct letters, so QUAZY's 26 chips become 41.
    expect(withRelic("lexicographer", "ghost", "quazy").last).toMatchObject({ chips: 41 })
    // SASSY adds only A and Y. Its three S's were spent by GHOST and its own
    // repeats count once. Seven letters, not ten, which is the whole reason the
    // card pays for covering ground rather than for typing.
    expect(withRelic("lexicographer", "ghost", "sassy", "quazy").last).toMatchObject({ chips: 47 })
  })

  it("Loaded Dice rolls inside its range, and rolls the same way twice", () => {
    const rolled = (word: string, seed: number): number => {
      const base = startRun(seed, words).state
      const state: RunState = { ...base, relics: [{ id: "loaded_dice" }] }
      const plain = apply(base, type(word)).round.guesses[0]
      const diced = apply(state, type(word)).round.guesses[0]
      return (diced?.mult ?? 0) - (plain?.mult ?? 0)
    }
    for (const seed of [1, 2, 3, 4, 5]) {
      const roll = rolled("crane", seed)
      expect(roll).toBeGreaterThanOrEqual(0)
      expect(roll).toBeLessThanOrEqual(20)
      expect(Number.isInteger(roll)).toBe(true)
    }
    // Replay is the whole contract: a golden vector cannot record a coin flip
    // that lands differently the second time.
    expect(rolled("crane", 7)).toBe(rolled("crane", 7))
    // And it is genuinely a roll: five identical values would mean it was not.
    expect(new Set([1, 2, 3, 4, 5].map((seed) => rolled("crane", seed))).size).toBeGreaterThan(1)
  })
})

describe("etchings", () => {
  it("raise a letter's chip value for the rest of the run", () => {
    const base = startRun(1, words).state
    // A is worth 1; etched twice it is worth 3, and CRANE holds one.
    const state: RunState = {
      ...base,
      letters: { ...base.letters, a: { etch: 2, destroyed: false, mod: null } },
    }
    expect(apply(state, type("crane")).round.guesses[0]?.chips).toBe(9)
  })
})

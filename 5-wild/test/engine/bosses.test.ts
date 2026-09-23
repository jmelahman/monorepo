import { describe, expect, it } from "vitest"
import type { Action, RunState, WordSource } from "../../src/engine"
import {
  BOSS_TIERS,
  BOSSES,
  bossesIn,
  draftChips,
  getBoss,
  LETTER_CHIPS,
  reduce,
  STAGES,
  solveBonusFor,
  startRun,
  tierForStage,
} from "../../src/engine"
// Not part of the engine's public surface. The draw is an internal detail that
// only the stage loop and this test have any business calling.
import { bossForStage } from "../../src/engine/bosses"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "quazy", "dairy", "ghost", "arose", "guild", "jazzy"]),
}

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, words).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** Drops a specific boss onto the opening round, bypassing the seeded draw. */
function underBoss(bossId: string): RunState {
  const base = startRun(1, words).state
  return {
    ...base,
    round: {
      ...base.round,
      bossId,
      maxGuesses: getBoss(bossId)?.maxGuesses ?? base.round.maxGuesses,
    },
  }
}

describe("boss rounds", () => {
  it("The Silence turns yellows gray, for the eye and for the math", () => {
    // DAIRY is normally four yellows: 9 chips x 5 mult.
    const state = apply(underBoss("silence"), type("dairy"))
    expect(state.round.guesses[0]).toMatchObject({ chips: 9, mult: 1 })
    expect(state.round.guesses[0]?.tiles.every((tile) => tile.shown === "gray")).toBe(true)
  })

  /*
   * The counterweight, and the reason the boss is playable: the row says
   * nothing, so the count has to. It is the only thing standing between the
   * player and a gray that means two different things at once.
   */
  it("The Silence counts what it hid", () => {
    // The count and not the sentence: the boss stopped authoring prose when the
    // catalog took it over, and what is asserted here is the rule's output.
    // DAIRY against BRAID: four of its letters are in the word, misplaced.
    expect(apply(underBoss("silence"), type("dairy")).round.guesses[0]?.note).toEqual({
      code: "misplaced",
      count: 4,
    })
    // GHOST shares nothing with BRAID, and being told so is worth more than any
    // of the five grays that carry the same claim ambiguously.
    expect(apply(underBoss("silence"), type("ghost")).round.guesses[0]?.note).toEqual({
      code: "misplaced",
      count: 0,
    })
  })

  it("counts misplaced letters only, never the ones it left green", () => {
    // CRANE against BRAID: R and A land green, only the leading C is a miss,
    // so the count must not quietly include the two the board already shows.
    const guess = apply(underBoss("silence"), type("crane")).round.guesses[0]
    expect(guess?.tiles.filter((tile) => tile.shown === "green")).toHaveLength(2)
    expect(guess?.note).toEqual({ code: "misplaced", count: 0 })
  })

  it("leaves no note on a guess no boss had anything to say about", () => {
    // Absent rather than empty, so a save and a vector written before notes
    // existed read back byte for byte.
    expect(apply(underBoss("fog"), type("dairy")).round.guesses[0]).not.toHaveProperty("note")
    const plain = apply(startRun(1, words).state, type("dairy"))
    expect(plain.round.guesses[0]).not.toHaveProperty("note")
  })

  it("The Fog hides yellows without disarming them", () => {
    const state = apply(underBoss("fog"), type("dairy"))
    const guess = state.round.guesses[0]
    expect(guess).toMatchObject({ chips: 9, mult: 5 })
    // The mult is real; the player just cannot see where it came from.
    expect(guess?.tiles[0]).toMatchObject({ color: "yellow", shown: "gray" })
  })

  it("The Tyrant demands you keep the greens you have found", () => {
    // CRANE fixes R and A in positions 2 and 3.
    const state = apply(underBoss("tyrant"), type("crane"))
    const { events } = reduce(
      apply(state, [...[..."quazy"].map((letter): Action => ({ type: "type_letter", letter }))]),
      { type: "submit" },
      words,
    )
    expect(events).toEqual([
      { type: "rejected", refusal: { code: "must_keep", letter: "r", position: 2 } },
    ])
  })

  it("The Miser pays nothing for a letter you have already spent", () => {
    const state = apply(underBoss("miser"), [...type("crane"), ...type("crane")])
    expect(state.round.guesses[0]?.chips).toBe(7)
    expect(state.round.guesses[1]).toMatchObject({ chips: 0, score: 0 })
  })

  it("The Clock allows only four guesses", () => {
    let state = underBoss("clock")
    expect(state.round.maxGuesses).toBe(4)
    for (let i = 0; i < 4; i++) state = apply(state, type("arose"))
    expect(state.round.done).toBe(true)
    expect(state.round.guesses).toHaveLength(4)
  })

  it("The Glutton demands two vowels", () => {
    const state = underBoss("glutton")
    const typed = apply(
      state,
      [..."ghost"].map((letter): Action => ({ type: "type_letter", letter })),
    )
    expect(reduce(typed, { type: "submit" }, words).events).toEqual([
      { type: "rejected", refusal: { code: "needs_two_vowels" } },
    ])
    // GUILD has U and I, so it passes.
    expect(apply(state, type("guild")).round.guesses).toHaveLength(1)
  })

  it("The Auditor caps the cash-out, so the build has to carry the round", () => {
    // Solving on guess one is normally worth x6. Under The Auditor it is worth
    // x2, and the pile it multiplies has to have been earned rather than timed.
    const normal = apply(startRun(1, words).state, type("braid"))
    const audited = apply(underBoss("auditor"), type("braid"))
    expect(normal.round.score).toBe(audited.round.score * 3)
    expect(solveBonusFor(underBoss("auditor"), 5)).toBe(2)
  })

  /*
   * The doubled letter is what makes JAZZY worth 34 chips and almost no
   * information, so this is the boss that takes the chip build's best line away
   * without touching deduction. That its answers stay typeable is covered by
   * the full-run test, which walks every boss across 120 seeds.
   */
  it("The Purist forbids repeated letters", () => {
    const state = underBoss("purist")
    const typed = apply(state, type("jazzy").slice(0, -1))
    expect(reduce(typed, { type: "submit" }, words).events).toEqual([
      { type: "rejected", refusal: { code: "no_repeated_letters" } },
    ])
    expect(apply(state, type("braid")).round.guesses).toHaveLength(1)
  })

  it("gives every boss a distinct id and some teeth", () => {
    expect(new Set(BOSSES.map((boss) => boss.id)).size).toBe(BOSSES.length)
    for (const boss of BOSSES) {
      const hasRule = Boolean(
        boss.maxGuesses ??
          boss.transform ??
          boss.validate ??
          boss.tileChips ??
          boss.solveBonus ??
          boss.noModifiers ??
          boss.noTimesMult,
      )
      expect(hasRule, `${boss.id} does nothing`).toBe(true)
    }
  })

  it("The Drought refuses to pay for vowels", () => {
    // AROSE is A-R-O-S-E: three vowels worth nothing, two consonants that still
    // score. The Glutton, in the same band, demands the letters this one voids.
    const state = apply(underBoss("drought"), type("arose"))
    const consonants = (LETTER_CHIPS.r ?? 0) + (LETTER_CHIPS.s ?? 0)
    expect(state.round.guesses[0]?.chips).toBe(consonants)
  })

  it("The Mirror reverses what you see and nothing else", () => {
    const state = apply(underBoss("mirror"), type("dairy"))
    const plain = apply(startRun(1, words).state, type("dairy"))
    const guess = state.round.guesses[0]
    // Identical arithmetic, reversed reading.
    expect(guess).toMatchObject({ chips: plain.round.guesses[0]?.chips, mult: 5 })
    expect(guess?.tiles.map((tile) => tile.shown)).toEqual(
      plain.round.guesses[0]?.tiles.map((tile) => tile.shown).reverse(),
    )
    expect(guess?.tiles.map((tile) => tile.color)).toEqual(
      plain.round.guesses[0]?.tiles.map((tile) => tile.color),
    )
  })

  it("The Famine allows only three guesses", () => {
    let state = underBoss("famine")
    expect(state.round.maxGuesses).toBe(3)
    for (let i = 0; i < 3; i++) state = apply(state, type("arose"))
    expect(state.round.done).toBe(true)
  })

  it("The Rust pays a letter what it started as, whatever was etched on it", () => {
    const base = underBoss("rust")
    const etched: RunState = {
      ...base,
      letters: { ...base.letters, a: { etch: 50, destroyed: false, mod: null } },
    }
    expect(apply(etched, type("braid")).round.guesses[0]?.chips).toBe(
      apply(base, type("braid")).round.guesses[0]?.chips,
    )
  })

  it("The Margin pays nothing for the outer columns, in the row and in the readout", () => {
    // BRAID under BRAID: B and D score nothing, R-A-I still do. The readout has
    // to agree, because it is what the player types against.
    const state = underBoss("margin")
    const middle = (LETTER_CHIPS.r ?? 0) + (LETTER_CHIPS.a ?? 0) + (LETTER_CHIPS.i ?? 0)
    expect(apply(state, type("braid")).round.guesses[0]?.chips).toBe(middle)
    expect(draftChips(state, "braid")).toBe(middle)
    // Half-typed, the last column does not exist yet, so only the first is
    // voided, and the promise stays a floor rather than becoming a guess.
    expect(draftChips(state, "bra")).toBe((LETTER_CHIPS.r ?? 0) + (LETTER_CHIPS.a ?? 0))
  })

  it("The Vandal silences the modifier layer without erasing the badge", () => {
    const base = underBoss("vandal")
    const etched: RunState = {
      ...base,
      letters: { ...base.letters, a: { etch: 0, destroyed: false, mod: "chip" } },
    }
    // +20 chips on the A, and none of it lands. The letter keeps its modifier;
    // the round suppresses it, it does not scrub the run.
    expect(apply(etched, type("braid")).round.guesses[0]?.chips).toBe(
      apply(base, type("braid")).round.guesses[0]?.chips,
    )
    expect(apply(etched, type("braid")).letters.a?.mod).toBe("chip")
    // And nothing narrates: a mod event here would light up a tile that did
    // nothing.
    expect(
      reduce(apply(etched, type("braid").slice(0, -1)), { type: "submit" }, words).events.some(
        (event) => event.type === "mod",
      ),
    ).toBe(false)
  })

  it("The Plateau lets mult be added and never multiplied", () => {
    const base = underBoss("plateau")
    const steel: RunState = {
      ...base,
      letters: { ...base.letters, a: { etch: 0, destroyed: false, mod: "steel" } },
    }
    // Steel is ×2, Anagrammer is ×2, and BRAID trips both. Under this boss
    // the mult is exactly what the tiles paid, and the modifier still fires;
    // only the multiplying part of it is swallowed.
    const plateau = apply({ ...steel, relics: [{ id: "anagrammer" }] }, type("braid"))
    expect(plateau.round.guesses[0]?.mult).toBe(apply(base, type("braid")).round.guesses[0]?.mult)
    // Five greens: 1 + 5×3 = 16.
    expect(plateau.round.guesses[0]?.mult).toBe(16)

    // The same board without the boss is the control, and it is 52 rather than
    // 64 because the steel fires *during* the row: 10 at the A, ×2 to 20, then
    // the last two greens, then Anagrammer's ×2 on 26. That interleave is the
    // pipeline working as documented, and it is exactly what The Plateau spares
    // the player from having to think about: under the boss there is nothing to
    // order, because nothing multiplies.
    const loose = apply(
      { ...steel, round: { ...steel.round, bossId: null }, relics: [{ id: "anagrammer" }] },
      type("braid"),
    )
    expect(loose.round.guesses[0]?.mult).toBe(52)
  })

  /*
   * `positional` is a claim the UI acts on, since the key's tip stops quoting a
   * chip value and quotes the rule instead, so a boss that reads the column and
   * forgot to say so would leave every key announcing what the first column
   * pays. Detected rather than trusted: score the same tile twice in different
   * columns and see whether the answer moves.
   */
  it("makes every boss that reads the column say so", () => {
    const round = startRun(1, words).state.round
    for (const boss of BOSSES) {
      if (!boss.tileChips) continue
      const at = (index: number) =>
        boss.tileChips?.(5, { letter: "a", color: "gray", shown: "gray" }, round, index)
      const moves = at(0) !== at(2) || at(2) !== at(4)
      expect(Boolean(boss.positional), `${boss.id} positional flag`).toBe(moves)
    }
  })

  it("gives every boss a band, and every band enough bosses to fill its stages", () => {
    for (const tier of ["early", "mid", "late"] as const) {
      const band = bossesIn(tier)
      const stages = Array.from({ length: STAGES }, (_, i) => i + 1).filter(
        (stage) => tierForStage(stage) === tier,
      )
      // A band with fewer bosses than stages would have to repeat one, which is
      // the property the old flat draw guaranteed and this one has to earn.
      expect(band.length, `${tier} band is short`).toBeGreaterThanOrEqual(stages.length)
    }
    expect(bossesIn("early").length + bossesIn("mid").length + bossesIn("late").length).toBe(
      BOSSES.length,
    )
  })

  /*
   * What replaced "every boss exactly once". A run no longer meets all of them
   * (that is the point of banding) but it must still never meet one twice,
   * and must never meet a late boss early. Both are checked across many seeds
   * because a single seed could pass by luck.
   */
  it("never repeats a boss, and never shows one out of its band", () => {
    const base = startRun(1, words).state
    for (let seed = 1; seed <= 120; seed++) {
      const seen = new Set<string>()
      for (let stage = 1; stage <= STAGES; stage++) {
        const id = bossForStage({ ...base, seed, stage })
        expect(seen.has(id), `seed ${seed} repeated ${id}`).toBe(false)
        seen.add(id)
        expect(getBoss(id)?.tier).toBe(tierForStage(stage))
      }
    }
  })

  it("accounts for every boss in the three bands", () => {
    // `BOSS_TIERS` is what the codex walks to list them, so a band missing from
    // it is a boss the player has no way to read about, and one `bossForStage`
    // would still deal them.
    const banded = BOSS_TIERS.flatMap((tier) => [...bossesIn(tier)])
    expect(banded).toHaveLength(BOSSES.length)
    expect(new Set(banded.map((boss) => boss.id)).size).toBe(BOSSES.length)
  })

  it("draws a different set from run to run, which banding is what buys", () => {
    const base = startRun(1, words).state
    const runs = new Set<string>()
    for (let seed = 1; seed <= 60; seed++) {
      const ids = Array.from({ length: STAGES }, (_, i) =>
        bossForStage({ ...base, seed, stage: i + 1 }),
      )
      runs.add(ids.join(","))
    }
    // The old draw produced one sequence per permutation of all eight; this one
    // also varies *which* bosses appear at all. Anything close to 1 would mean
    // the band streams had collapsed onto the same order.
    expect(runs.size).toBeGreaterThan(30)
  })
})

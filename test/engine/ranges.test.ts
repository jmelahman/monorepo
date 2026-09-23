import { describe, expect, it } from "vitest"
import type { RunState, WordSource } from "../../src/engine"
import {
  ALPHABET,
  baseChips,
  CHIPS_PER_LEVEL,
  LETTER_CHIPS,
  liveRanges,
  RANGE_BY_ID,
  RANGES,
  rangeBonus,
  rangeChips,
  rangeLevelOf,
  rangeOf,
  startRun,
} from "../../src/engine"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane"]),
}

const freshState = (): RunState => startRun(1, words).state

describe("the alphabet ranges", () => {
  it("partitions the alphabet, every letter in exactly one range", () => {
    const seen = new Map<string, string>()
    for (const range of RANGES) {
      for (const letter of range.letters) {
        expect(seen.has(letter), `${letter} is in two ranges`).toBe(false)
        seen.set(letter, range.id)
      }
    }
    expect([...seen.keys()].sort().join("")).toBe([...ALPHABET].sort().join(""))
  })

  it("covers every letter with a contiguous slice", () => {
    // Contiguity is the whole reason these are alphabet ranges rather than
    // better-balanced arbitrary buckets: a player has to be able to work out
    // which one a letter is in while typing it.
    const joined = RANGES.map((range) => range.letters).join("")
    expect(joined).toBe([...ALPHABET].sort().join(""))
  })

  it("names each range after its own endpoints", () => {
    for (const range of RANGES) {
      const first = range.letters[0]?.toUpperCase()
      const last = range.letters[range.letters.length - 1]?.toUpperCase()
      expect(range.name).toBe(`${first}–${last}`)
    }
  })

  it("finds the range a letter falls in", () => {
    expect(rangeOf("a")?.id).toBe("range_ae")
    expect(rangeOf("e")?.id).toBe("range_ae")
    expect(rangeOf("f")?.id).toBe("range_fm")
    expect(rangeOf("m")?.id).toBe("range_fm")
    expect(rangeOf("n")?.id).toBe("range_nr")
    expect(rangeOf("r")?.id).toBe("range_nr")
    expect(rangeOf("s")?.id).toBe("range_sz")
    expect(rangeOf("z")?.id).toBe("range_sz")
  })
})

describe("leveling a range", () => {
  it("starts every range at level one", () => {
    const state = freshState()
    for (const range of RANGES) expect(rangeLevelOf(state, range.id)).toBe(1)
  })

  it("pays nothing at level one, so a fresh run scores what it always did", () => {
    const state = freshState()
    for (const range of RANGES) expect(rangeBonus(state, range).chips).toBe(0)
    for (const letter of ALPHABET) {
      expect(baseChips(state, letter)).toBe(LETTER_CHIPS[letter] ?? 0)
    }
  })

  it("pays one step per level above the first", () => {
    const range = RANGE_BY_ID.get("range_ae")
    if (!range) throw new Error("range_ae missing")
    const state = freshState()

    state.ranges = { range_ae: 2 }
    expect(rangeBonus(state, range).chips).toBe(CHIPS_PER_LEVEL)

    state.ranges = { range_ae: 4 }
    expect(rangeBonus(state, range).chips).toBe(CHIPS_PER_LEVEL * 3)
  })

  it("raises only the letters inside its own slice", () => {
    const state = freshState()
    state.ranges = { range_ae: 3 }
    const bonus = CHIPS_PER_LEVEL * 2

    expect(rangeChips(state, "c")).toBe(bonus)
    expect(rangeChips(state, "e")).toBe(bonus)
    expect(rangeChips(state, "f")).toBe(0)
    expect(rangeChips(state, "z")).toBe(0)
  })

  it("stacks with an etching on the same letter", () => {
    // The two lines crosscut on purpose: A–E holds vowels and consonants, and
    // Etch Vowels reaches into all four ranges, so a letter can collect from
    // both and neither can dominate the other.
    const state = freshState()
    state.ranges = { range_ae: 2 }
    const entry = state.letters.e
    if (!entry) throw new Error("no letter state for e")
    entry.etch = 6

    expect(baseChips(state, "e")).toBe((LETTER_CHIPS.e ?? 0) + 6 + CHIPS_PER_LEVEL)
  })

  it("keeps paying a letter that a sibling range never touches", () => {
    const state = freshState()
    state.ranges = { range_sz: 5 }
    expect(rangeChips(state, "s")).toBe(CHIPS_PER_LEVEL * 4)
    expect(rangeChips(state, "a")).toBe(0)
  })
})

describe("what the shop may draw from", () => {
  it("offers every range while the alphabet is whole", () => {
    expect(liveRanges(freshState()).map((range) => range.id)).toEqual(RANGES.map((r) => r.id))
  })

  it("drops a range once every letter in it is broken", () => {
    const state = freshState()
    for (const letter of "nopqr") {
      const entry = state.letters[letter]
      if (entry) entry.destroyed = true
    }
    expect(liveRanges(state).map((range) => range.id)).not.toContain("range_nr")
    expect(liveRanges(state)).toHaveLength(RANGES.length - 1)
  })

  it("keeps a range whose letters are only partly broken", () => {
    const state = freshState()
    for (const letter of "nopq") {
      const entry = state.letters[letter]
      if (entry) entry.destroyed = true
    }
    expect(liveRanges(state).map((range) => range.id)).toContain("range_nr")
  })
})

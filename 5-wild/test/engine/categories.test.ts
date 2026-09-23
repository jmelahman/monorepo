import { describe, expect, it } from "vitest"
import type { Action, RunState, WordSource } from "../../src/engine"
import {
  CATEGORIES,
  CATEGORY_BY_ID,
  categoryOf,
  isCategory,
  levelBonus,
  levelOf,
  reduce,
  startRun,
} from "../../src/engine"
import { realWords } from "../helpers/words"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "abbot", "audio", "shrub", "level", "quazy"]),
}

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, words).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** A run at a chosen level for one category, with nothing else in the way. */
const leveled = (id: string, level: number): RunState => ({
  ...startRun(1, words).state,
  levels: { [id]: level },
})

describe("word categories", () => {
  it("sorts each word into exactly one, rarest first", () => {
    expect(categoryOf("abbot").id).toBe("alphabetical")
    expect(categoryOf("audio").id).toBe("vowel_heavy")
    expect(categoryOf("shrub").id).toBe("cluster")
    // LEVEL repeats, and is the awkward case worth pinning: SASSY also repeats
    // but ends S-S-Y, and Y is a consonant here, so it scores as a Cluster.
    expect(categoryOf("level").id).toBe("twinned")
    expect(categoryOf("sassy").id).toBe("cluster")
    expect(categoryOf("crane").id).toBe("distinct")
  })

  it("prefers the rarer shape when a word is several at once", () => {
    // ABBOT repeats a letter and is in alphabetical order. Both predicates say
    // yes; the ranking is what decides, and it decides for the harder one.
    expect(isCategory("twinned", "abbot")).toBe(true)
    expect(isCategory("alphabetical", "abbot")).toBe(true)
    expect(categoryOf("abbot").id).toBe("alphabetical")
  })

  it("has an answer for every real word", () => {
    for (const word of realWords.answers) expect(categoryOf(word)).toBeDefined()
  })

  it("splits the whole answer list between twinned and distinct", () => {
    // The two are complements, which is what makes `categoryOf` total rather
    // than merely lucky: every word falls into one of them if nothing rarer
    // claims it first.
    for (const word of realWords.answers) {
      expect(isCategory("twinned", word)).toBe(!isCategory("distinct", word))
    }
  })

  it("grades the step by how hard the shape is to hit", () => {
    const chips = CATEGORIES.map((category) => category.chips)
    const mult = CATEGORIES.map((category) => category.mult)
    expect(chips).toEqual([...chips].sort((a, b) => b - a))
    expect(mult).toEqual([...mult].sort((a, b) => b - a))
  })
})

describe("category levels", () => {
  it("starts every category at one, with nothing stored", () => {
    const state = startRun(1, words).state
    expect(state.levels).toBeUndefined()
    for (const category of CATEGORIES) expect(levelOf(state, category.id)).toBe(1)
  })

  it("pays nothing at level one", () => {
    const state = startRun(1, words).state
    for (const category of CATEGORIES) {
      expect(levelBonus(state, category)).toEqual({ level: 1, chips: 0, mult: 0 })
    }
  })

  it("pays one step per level above the first", () => {
    const distinct = CATEGORY_BY_ID.get("distinct")
    if (!distinct) throw new Error("no distinct category")
    const bonus = levelBonus(leveled("distinct", 4), distinct)
    expect(bonus).toEqual({ level: 4, chips: distinct.chips * 3, mult: distinct.mult * 3 })
  })

  it("lands on the base, so the guess scores more chips and more mult", () => {
    // CRANE is Distinct: 7 chips x 7 mult unleveled.
    const plain = apply(startRun(1, words).state, type("crane"))
    expect(plain.round.guesses[0]).toMatchObject({ chips: 7, mult: 7 })

    const distinct = CATEGORY_BY_ID.get("distinct")
    if (!distinct) throw new Error("no distinct category")
    const raised = apply(leveled("distinct", 3), type("crane"))
    expect(raised.round.guesses[0]).toMatchObject({
      chips: 7 + distinct.chips * 2,
      mult: 7 + distinct.mult * 2,
    })
  })

  it("only pays the category the word actually scored as", () => {
    // SHRUB is a Cluster, so a leveled Distinct does nothing for it.
    const raised = apply(leveled("distinct", 5), type("shrub"))
    const plain = apply(startRun(1, words).state, type("shrub"))
    expect(raised.round.guesses[0]?.score).toBe(plain.round.guesses[0]?.score)
  })

  it("announces itself only once it is worth something", () => {
    const quiet = reduce(apply(startRun(1, words).state, type("crane")), { type: "submit" }, words)
    expect(quiet.events.some((event) => event.type === "category")).toBe(false)

    let state = leveled("distinct", 2)
    for (const letter of "crane")
      state = reduce(state, { type: "type_letter", letter }, words).state
    const { events } = reduce(state, { type: "submit" }, words)
    expect(events).toContainEqual(
      expect.objectContaining({ type: "category", id: "distinct", level: 2 }),
    )
  })

  it("scores before the relics, so a ×mult relic multiplies the level", () => {
    // Anagrammer is ×2 mult on a distinct word. Level 2 Distinct adds its mult
    // to the base first, so the doubling lands on the raised figure.
    const distinct = CATEGORY_BY_ID.get("distinct")
    if (!distinct) throw new Error("no distinct category")
    const state: RunState = { ...leveled("distinct", 2), relics: [{ id: "anagrammer" }] }
    const played = apply(state, type("crane"))
    expect(played.round.guesses[0]?.mult).toBe((7 + distinct.mult) * 2)
  })
})

describe("buying a level", () => {
  const inShop = (level: number | null): RunState => {
    const base = startRun(1, words).state
    return {
      ...base,
      phase: "shop",
      gold: 50,
      ...(level === null ? {} : { levels: { distinct: level } }),
      shop: { items: [{ kind: "level", id: "distinct", cost: 6 }], rerolls: 0 },
    }
  }

  it("writes the first level on purchase and not before", () => {
    const before = inShop(null)
    expect(before.levels).toBeUndefined()
    const after = reduce(before, { type: "buy", index: 0 }, words).state
    expect(after.levels).toEqual({ distinct: 2 })
    expect(after.gold).toBe(50 - 6)
  })

  it("stacks on a level already held", () => {
    const after = reduce(inShop(3), { type: "buy", index: 0 }, words).state
    expect(after.levels).toEqual({ distinct: 4 })
  })

  it("survives the save round trip", () => {
    const saved = reduce(inShop(null), { type: "buy", index: 0 }, words).state
    const revived = JSON.parse(JSON.stringify(saved)) as RunState
    expect(levelOf(revived, "distinct")).toBe(2)
  })

  it("refuses a category it does not recognize, without taking the gold", () => {
    const state: RunState = {
      ...inShop(null),
      shop: { items: [{ kind: "level", id: "palindrome", cost: 6 }], rerolls: 0 },
    }
    const { state: after, events } = reduce(state, { type: "buy", index: 0 }, words)
    expect(after.gold).toBe(50)
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "unknown_category" } })
  })
})

describe("the relics that read a category", () => {
  /*
   * These three were written around inline predicates and now share the
   * category table's. They ask `isCategory` rather than `categoryOf`, which is
   * the part worth pinning: Anagrammer pays for a word with no repeats even
   * when a rarer category claimed it for scoring.
   */
  it("still pays Anagrammer for a distinct word that scored as something rarer", () => {
    const state: RunState = { ...startRun(1, words).state, relics: [{ id: "anagrammer" }] }
    // AUDIO has no repeats and scores as Vowel Heavy.
    expect(categoryOf("audio").id).toBe("vowel_heavy")
    const played = apply(state, type("audio"))
    const plain = apply(startRun(1, words).state, type("audio"))
    expect(played.round.guesses[0]?.mult).toBe((plain.round.guesses[0]?.mult ?? 0) * 2)
  })

  it("still pays Alphabetist for a word that also repeats a letter", () => {
    const state: RunState = { ...startRun(1, words).state, relics: [{ id: "alphabetist" }] }
    const played = apply(state, type("abbot"))
    const plain = apply(startRun(1, words).state, type("abbot"))
    expect(played.round.guesses[0]?.mult).toBe((plain.round.guesses[0]?.mult ?? 0) * 2)
  })

  it("still pays Consonant Cluster on a three-consonant run", () => {
    const state: RunState = { ...startRun(1, words).state, relics: [{ id: "consonant_cluster" }] }
    const played = apply(state, type("shrub"))
    const plain = apply(startRun(1, words).state, type("shrub"))
    expect(played.round.guesses[0]?.mult).toBe((plain.round.guesses[0]?.mult ?? 0) * 1.5)
  })
})

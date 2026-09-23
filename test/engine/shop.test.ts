import { describe, expect, it } from "vitest"
import type { Action, ModId, Rarity, RunState } from "../../src/engine"
import {
  ALPHABET,
  derive,
  ETCHING_BY_ID,
  ETCHINGS,
  MODIFIER_BY_ID,
  RELIC_BY_ID,
  RELICS,
  reduce,
  STAGES,
  startRun,
} from "../../src/engine"
import { rollShop } from "../../src/engine/shop"
import { realWords } from "../helpers/words"

const shopAt = (seed: number, stage = 1) =>
  rollShop(startRun(seed, realWords).state, derive(seed, "shop", stage, 0, 0), 0)

/** A run parked in the shop with plenty of gold, so buying is never rejected. */
function inShop(seed: number): RunState {
  const base = startRun(seed, realWords).state
  return { ...base, phase: "shop", gold: 999, shop: shopAt(seed) }
}

const buy = (state: RunState, index: number): RunState =>
  reduce(state, { type: "buy", index } satisfies Action, realWords).state

/** Puts a specific etching in slot 0, whatever the roll happened to deal. */
function offering(state: RunState, id: string): RunState {
  const etching = ETCHING_BY_ID.get(id)
  if (!etching) throw new Error(`no etching ${id}`)
  return {
    ...state,
    shop: { items: [{ kind: "etch", id, cost: etching.cost }], rerolls: 0 },
  }
}

describe("the shop layout", () => {
  it("always deals two relics and never a third", () => {
    for (let seed = 1; seed <= 50; seed++) {
      const kinds = shopAt(seed).items.map((item) => item?.kind)
      expect(kinds.slice(0, 2)).toEqual(["relic", "relic"])
      expect(kinds.filter((kind) => kind === "relic")).toHaveLength(2)
    }
  })

  it("never deals the same relic twice", () => {
    for (let seed = 1; seed <= 50; seed++) {
      const [first, second] = shopAt(seed).items
      expect(first?.kind === "relic" && second?.kind === "relic" && first.id === second.id).toBe(
        false,
      )
    }
  })

  it("keeps the upgrade and letter slots to their own stock", () => {
    for (let seed = 1; seed <= 50; seed++) {
      const items = shopAt(seed).items
      expect(["etch", "range", "level", "consumable"]).toContain(items[2]?.kind)
      expect(["mod", "consumable"]).toContain(items[3]?.kind)
    }
  })

  it("fills the relic slots with ordinary stock once every relic is owned", () => {
    const base = startRun(3, realWords).state
    const state: RunState = { ...base, relics: RELICS.map((relic) => ({ id: relic.id })) }
    const items = rollShop(state, derive(3, "shop", 1, 0, 0), 0).items
    expect(items).toHaveLength(5)
    expect(items.map((item) => item?.kind)).not.toContain("relic")
    for (const item of items) expect(item).not.toBeNull()
  })

  it("sells modifiers with no letter on them, at the choice price", () => {
    // Which letter a modifier sits on is most of what it is worth, so the shop
    // sells the card and lets the player aim it. The pack is the half that still
    // deals pairings; see packs.test.ts.
    let offered = 0
    for (let seed = 1; seed <= 200; seed++) {
      const item = shopAt(seed).items[3]
      if (item?.kind !== "mod") continue
      offered++
      expect(item.letter).toBeUndefined()
      expect(item.cost).toBe(MODIFIER_BY_ID.get(item.id)?.choiceCost)
    }
    expect(offered).toBeGreaterThan(0)
  })

  it("stops offering a modifier with nowhere left to put it", () => {
    // Echo is the only restricted one today, and the restriction is the point:
    // no five-letter answer repeats a J, Q or X, so Echo on one of those is a
    // card that cannot ever fire. It goes on AELOST only, so break all six and
    // the shop has to stop stocking it rather than sell a choice with no options.
    const base = startRun(5, realWords).state
    const letters = { ...base.letters }
    for (const letter of "aelost") letters[letter] = { etch: 0, destroyed: true, mod: null }
    const state: RunState = { ...base, letters }
    for (let stage = 1; stage <= 60; stage++) {
      const item = rollShop(state, derive(5, "shop", stage, 0, 0), 0).items[3]
      if (item?.kind === "mod") expect(item.id).not.toBe("echo")
    }
  })

  it("stops offering an etching whose group is entirely broken", () => {
    const base = startRun(5, realWords).state
    const letters = { ...base.letters }
    for (const letter of "jqxz") letters[letter] = { etch: 0, destroyed: true, mod: null }
    const state: RunState = { ...base, letters }
    // Every roll of the upgrade slot, across many streams: Heavy is unsellable
    // because there is nothing left in it to etch.
    for (let stage = 1; stage <= 40; stage++) {
      const item = rollShop(state, derive(5, "shop", stage, 0, 0), 0).items[2]
      if (item?.kind === "etch") expect(item.id).not.toBe("etch_heavy")
    }
  })
})

describe("group etchings", () => {
  it("raises every letter in the group at once", () => {
    const state = buy(offering(inShop(1), "etch_vowels"), 0)
    for (const letter of "aeiou") expect(state.letters[letter]?.etch).toBe(2)
    for (const letter of "lnstr") expect(state.letters[letter]?.etch).toBe(0)
  })

  it("stacks, because that is the whole idea", () => {
    let state = inShop(1)
    for (let bought = 0; bought < 3; bought++) state = buy(offering(state, "etch_vowels"), 0)
    expect(state.letters.a?.etch).toBe(6)
  })

  it("skips a letter that has already broken", () => {
    const base = inShop(1)
    const state = buy(
      offering(
        { ...base, letters: { ...base.letters, e: { etch: 0, destroyed: true, mod: null } } },
        "etch_vowels",
      ),
      0,
    )
    expect(state.letters.e?.etch).toBe(0)
    expect(state.letters.a?.etch).toBe(2)
  })

  it("covers every consonant and no vowel", () => {
    const state = buy(offering(inShop(1), "etch_consonants"), 0)
    const etched = [...ALPHABET].filter((letter) => (state.letters[letter]?.etch ?? 0) > 0)
    expect(etched.join("")).toBe("bcdfghjklmnpqrstvwxyz")
  })

  it("charges for the group and leaves the slot sold", () => {
    const state = buy(offering({ ...inShop(1), gold: 10 }, "etch_heavy"), 0)
    expect(state.gold).toBe(10 - 5)
    expect(state.shop?.items[0]).toBeNull()
  })

  it("refuses a group it does not recognize, without taking the gold", () => {
    const state = inShop(1)
    const stale: RunState = {
      ...state,
      gold: 20,
      shop: { items: [{ kind: "etch", id: "etch_letter_a", cost: 4 }], rerolls: 0 },
    }
    const { state: after, events } = reduce(stale, { type: "buy", index: 0 }, realWords)
    expect(after.gold).toBe(20)
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "unknown_etching" } })
  })

  it("prices breadth against depth rather than under it", () => {
    // The bug this replaced: $4 bought one chip on one letter, while $4 bought
    // twenty on one letter from the modifier line. Every group now buys at least
    // eight chips' worth of alphabet, so none of them is a wasted slot.
    for (const etching of ETCHINGS) {
      expect(etching.chips * etching.letters.length).toBeGreaterThanOrEqual(8)
      expect(etching.cost).toBeGreaterThan(4)
    }
  })
})

/** Puts a specific unattached modifier in slot 0, as the shop now sells them. */
function selling(state: RunState, id: ModId): RunState {
  const modifier = MODIFIER_BY_ID.get(id)
  if (!modifier) throw new Error(`no modifier ${id}`)
  return {
    ...state,
    shop: { items: [{ kind: "mod", id, cost: modifier.choiceCost }], rerolls: 0 },
  }
}

const act = (state: RunState, action: Action) => reduce(state, action, realWords)

describe("placing a bought modifier", () => {
  it("takes the gold and then holds the shop until a letter is chosen", () => {
    const state = buy(selling({ ...inShop(1), gold: 20 }, "steel"), 0)
    expect(state.placing).toBe("steel")
    expect(state.gold).toBe(20 - 12)
    // Nothing else in the shop may move while a card is in hand: the same rule
    // an open pack lives under, and for the same reason.
    for (const action of [
      { type: "buy", index: 0 },
      { type: "reroll" },
      { type: "next_round" },
      { type: "sell_relic", index: 0 },
    ] satisfies Action[]) {
      expect(act(state, action).events).toContainEqual({
        type: "rejected",
        refusal: { code: "place_mod_first" },
      })
    }
  })

  it("lands on the letter the player points at", () => {
    const held = buy(selling(inShop(1), "steel"), 0)
    const { state, events } = act(held, { type: "place_mod", letter: "e" })
    expect(state.letters.e?.mod).toBe("steel")
    expect(state.placing).toBeNull()
    expect(events).toContainEqual({ type: "mod_placed", id: "steel", letter: "e" })
  })

  it("trades away whatever the letter was already carrying", () => {
    const first = act(buy(selling(inShop(1), "chip"), 0), { type: "place_mod", letter: "e" }).state
    const second = act(buy(selling({ ...first, shop: inShop(1).shop }, "steel"), 0), {
      type: "place_mod",
      letter: "e",
    }).state
    expect(second.letters.e?.mod).toBe("steel")
  })

  it("refuses a letter the modifier is barred from", () => {
    // Echo goes on AELOST only. The picker grays the rest out, but the rule has
    // to live in the engine: the picker is the one input that comes from outside.
    const held = buy(selling(inShop(1), "echo"), 0)
    const { state, events } = act(held, { type: "place_mod", letter: "j" })
    expect(state.letters.j?.mod).toBeNull()
    // Names the card by id and leaves the spelling to the catalog, which is the
    // only refusal in the game that has to name anything at all.
    expect(events).toContainEqual({
      type: "rejected",
      refusal: { code: "mod_not_allowed", id: "echo", letter: "j" },
    })
  })

  it("refuses a letter that has broken", () => {
    const base = inShop(1)
    const held = buy(
      selling(
        { ...base, letters: { ...base.letters, e: { etch: 0, destroyed: true, mod: null } } },
        "steel",
      ),
      0,
    )
    expect(act(held, { type: "place_mod", letter: "e" }).state.letters.e?.mod).toBeNull()
  })

  it("will not sell one with nowhere left to put it", () => {
    const base = inShop(1)
    const letters = { ...base.letters }
    for (const letter of "aelost") letters[letter] = { etch: 0, destroyed: true, mod: null }
    const { state, events } = act(selling({ ...base, gold: 20, letters }, "echo"), {
      type: "buy",
      index: 0,
    })
    // Checked before the gold moves, or the player is left holding a card with
    // nowhere to put it and a shop that will not let them leave.
    expect(state.gold).toBe(20)
    expect(state.placing).toBeUndefined()
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "no_letter_for_mod" } })
  })
})

/** The shelf as it is rolled at a given stage, which is what the odds read. */
const shelfAt = (seed: number, stage: number) => {
  const base = startRun(seed, realWords).state
  return rollShop({ ...base, stage }, derive(seed, "shop", stage, 0, 0), 0).items
}

function relicRarities(stage: number, seeds = 200): Rarity[] {
  const out: Rarity[] = []
  for (let seed = 1; seed <= seeds; seed++) {
    for (const item of shelfAt(seed, stage)) {
      if (item?.kind === "relic") out.push(RELIC_BY_ID.get(item.id)?.rarity ?? "common")
    }
  }
  return out
}

const shareOf = (list: readonly Rarity[], ...of: Rarity[]) =>
  list.filter((rarity) => of.includes(rarity)).length / list.length

describe("what rarity the relic slots deal", () => {
  it("starts at the shelf the catalog was already dealing", () => {
    // The first stage is deliberately the neutral point: the same mix a uniform
    // draw over the whole catalog gives. Tilting it toward cheap cards was
    // tried three ways and every one of them cost the bots a tenth of an stage
    // and a quarter of their wins, because five relic slots fill by stage 2 and
    // a cheaper shelf is a permanently weaker tray.
    const early = relicRarities(1)
    const catalog = RELICS.map((relic) => relic.rarity)
    for (const rarity of ["common", "uncommon", "rare", "legendary"] satisfies Rarity[]) {
      expect(shareOf(early, rarity)).toBeCloseTo(shareOf(catalog, rarity), 1)
    }
  })

  it("opens up the expensive ones as the run gets rich", () => {
    // Gold compounds through interest while relic prices never move, so a shelf
    // still reading mostly common at stage 7 is one the run has outgrown.
    const early = relicRarities(1)
    const late = relicRarities(STAGES)
    expect(shareOf(late, "rare", "legendary")).toBeGreaterThan(
      shareOf(early, "rare", "legendary") * 1.5,
    )
    // And commons have to actually recede, or the tilt is just extra rares on
    // top of a shelf that still reads the same.
    expect(shareOf(late, "common")).toBeLessThan(shareOf(early, "common") / 1.6)
  })

  it("holds the odds steady past the last authored stage", () => {
    // `roundTargets` keeps climbing out there but the odds have nowhere left to
    // go, so an endless run should not drift toward an all-legendary shelf.
    expect(shareOf(relicRarities(40), "legendary")).toBeLessThan(0.3)
  })

  it("keeps dealing relics when a rarity has been bought out", () => {
    // Owning every cheap relic must not make the slot fail four times in five.
    // The odds are a shape for the shelf, not a promise to leave it empty.
    const base = startRun(3, realWords).state
    const cheap = RELICS.filter((relic) => relic.rarity !== "rare")
    const state: RunState = { ...base, stage: 1, relics: cheap.map((relic) => ({ id: relic.id })) }
    const items = rollShop(state, derive(3, "shop", 1, 0, 0), 0).items
    expect(items.slice(0, 2).map((item) => item?.kind)).toEqual(["relic", "relic"])
  })

  it("deals the whole catalog at some stage or other", () => {
    // Every rarity has to be reachable at both ends of the ramp, or a column
    // that reads as a weight is really a card the shop never sells.
    for (const stage of [1, STAGES]) {
      const dealt = new Set(relicRarities(stage))
      expect([...dealt].sort()).toEqual(["common", "legendary", "rare", "uncommon"])
    }
  })
})

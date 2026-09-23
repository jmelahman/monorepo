import { describe, expect, it } from "vitest"
import type { Action, RunState } from "../../src/engine"
import { derive, PACK_BY_ID, PACKS, RELIC_SLOTS, RELICS, reduce, startRun } from "../../src/engine"
import { rollShop } from "../../src/engine/shop"
import { realWords } from "../helpers/words"

/**
 * Packs. What is under test is mostly the *hold*: a pack is paid for before it
 * is chosen from, so the shop must not be allowed to move on, reroll or restock
 * underneath one, and skipping has to be something you did on purpose.
 */

const shopAt = (seed: number, stage = 1) =>
  rollShop(startRun(seed, realWords).state, derive(seed, "shop", stage, 0, 0), 0)

/** A run parked in the shop with plenty of gold, so buying is never rejected. */
function inShop(seed: number): RunState {
  const base = startRun(seed, realWords).state
  return { ...base, phase: "shop", gold: 999, shop: shopAt(seed) }
}

const act = (state: RunState, action: Action) => reduce(state, action, realWords)
const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => act(current, action).state, state)

/** Puts a named pack in slot 0, whatever the roll happened to deal. */
function offering(state: RunState, id: string, rerolls = 0): RunState {
  const pack = PACK_BY_ID.get(id)
  if (!pack) throw new Error(`no pack ${id}`)
  return { ...state, shop: { items: [{ kind: "pack", id, cost: pack.cost }], rerolls } }
}

const opened = (seed: number, id: string): RunState =>
  apply(offering(inShop(seed), id), [{ type: "buy", index: 0 }])

describe("the pack slot", () => {
  it("is on every shelf, and is the only thing in its slot", () => {
    for (let seed = 1; seed <= 50; seed++) {
      const items = shopAt(seed).items
      expect(items[4]?.kind).toBe("pack")
      expect(items.filter((item) => item?.kind === "pack")).toHaveLength(1)
    }
  })

  it("stops offering the relic pack once every relic is owned", () => {
    const base = startRun(3, realWords).state
    const state: RunState = { ...base, relics: RELICS.map((relic) => ({ id: relic.id })) }
    for (let stage = 1; stage <= 40; stage++) {
      const item = rollShop(state, derive(3, "shop", stage, 0, 0), 0).items[4]
      if (item?.kind === "pack") expect(item.id).not.toBe("relic")
    }
  })

  it("stops offering the relic pack once the slots are full", () => {
    const base = startRun(3, realWords).state
    const state: RunState = {
      ...base,
      relics: RELICS.slice(0, RELIC_SLOTS).map(({ id }) => ({ id })),
    }
    for (let stage = 1; stage <= 40; stage++) {
      const item = rollShop(state, derive(3, "shop", stage, 0, 0), 0).items[4]
      if (item?.kind === "pack") expect(item.id).not.toBe("relic")
    }
  })

  it("refuses to open a relic pack off a stale shelf, without taking the gold", () => {
    // The shelf rolled the pack while a slot was free; the relic in the slot
    // next to it filled that slot before the pack was opened. Paying $9 for
    // three cards none of which can land is worse than not being sold it.
    const base = inShop(1)
    const state: RunState = {
      ...offering(base, "relic"),
      relics: RELICS.slice(0, RELIC_SLOTS).map(({ id }) => ({ id })),
    }
    const { state: after, events } = act(state, { type: "buy", index: 0 })
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "pack_empty" } })
    expect(after.gold).toBe(state.gold)
    expect(after.pack).toBeFalsy()
  })

  it("gives every pack a distinct id and a price", () => {
    expect(new Set(PACKS.map((pack) => pack.id)).size).toBe(PACKS.length)
    for (const pack of PACKS) {
      expect(pack.cost).toBeGreaterThan(0)
      expect(pack.picks).toBeGreaterThan(0)
      expect(pack.options).toBeGreaterThanOrEqual(pack.picks)
    }
  })
})

describe("opening a pack", () => {
  it("charges for it and lays its cards out, without applying any of them", () => {
    const before = inShop(1)
    const state = apply(offering(before, "alphabet"), [{ type: "buy", index: 0 }])
    // Read off the card rather than written out, so a repricing moves this with
    // the catalog. What is under test is that the gold goes at all, not what
    // the alphabet pack happens to cost this week.
    expect(state.gold).toBe(before.gold - (PACK_BY_ID.get("alphabet")?.cost ?? 0))
    expect(state.pack?.id).toBe("alphabet")
    expect(state.pack?.options).toHaveLength(3)
    expect(state.pack?.picks).toBe(1)
    // The slot is spent, and nothing has landed on the alphabet yet.
    expect(state.shop?.items[0]).toBeNull()
    expect(state.letters).toEqual(before.letters)
  })

  it("lays out no card twice", () => {
    for (const id of ["alphabet", "relic", "category"]) {
      for (let seed = 1; seed <= 30; seed++) {
        const options = opened(seed, id).pack?.options ?? []
        const keys = options.map((item) =>
          item?.kind === "mod" ? `${item.id}:${item.letter}` : item?.id,
        )
        expect(new Set(keys).size).toBe(keys.length)
      }
    }
  })

  it("fills each pack from its own stock", () => {
    for (let seed = 1; seed <= 20; seed++) {
      for (const item of opened(seed, "alphabet").pack?.options ?? [])
        expect(item?.kind).toBe("mod")
      for (const item of opened(seed, "relic").pack?.options ?? []) expect(item?.kind).toBe("relic")
      for (const item of opened(seed, "category").pack?.options ?? [])
        expect(item?.kind).toBe("level")
    }
  })

  it("deals different cards after a reroll", () => {
    // Otherwise rerolling the shelf to hunt for a better pack would hand back
    // the same three cards, and the reroll would be gold for nothing.
    const base = inShop(4)
    const first = apply(offering(base, "alphabet"), [{ type: "buy", index: 0 }])
    const rerolled = apply(offering(base, "alphabet", 1), [{ type: "buy", index: 0 }])
    const key = (state: RunState) => JSON.stringify(state.pack?.options)
    expect(key(rerolled)).not.toBe(key(first))
  })
})

describe("choosing from a pack", () => {
  it("applies the card without charging for it", () => {
    const state = opened(1, "alphabet")
    const chosen = state.pack?.options[0]
    // A pack deals pairings, letter and all. The shop is the half that sells the
    // modifier loose and lets the player aim it.
    if (chosen?.kind !== "mod" || !chosen.letter) throw new Error("expected a pairing")
    const after = act(state, { type: "pick_pack", index: 0 })

    expect(after.state.letters[chosen.letter]?.mod).toBe(chosen.id)
    expect(after.state.gold).toBe(state.gold)
  })

  it("closes once the last pick is spent", () => {
    const after = apply(opened(1, "category"), [{ type: "pick_pack", index: 0 }])
    expect(after.pack).toBeNull()
    expect(Object.values(after.levels ?? {})).toEqual([2])
  })

  it("refuses the same card twice", () => {
    const state = opened(1, "alphabet")
    const { events } = act(apply(state, [{ type: "pick_pack", index: 0 }]), {
      type: "pick_pack",
      index: 0,
    })
    // The pack has already closed by then, so the second tap finds nothing open.
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "no_pack_open" } })
  })

  it("keeps the pack open when a card cannot be taken", () => {
    // Every relic slot full, so no card in a relic pack can land. The pack stays
    // up rather than swallowing the pick, because the gold is already spent and
    // the player still has a decision to make.
    const base = opened(2, "relic")
    const full: RunState = {
      ...base,
      relics: RELICS.slice(0, 5).map((relic) => ({ id: relic.id })),
    }
    const { state, events } = act(full, { type: "pick_pack", index: 0 })
    expect(events).toContainEqual({ type: "rejected", refusal: { code: "no_relic_slots" } })
    expect(state.pack?.options[0]).not.toBeNull()
  })

  it("can be walked away from, and says so", () => {
    const state = opened(1, "alphabet")
    const { state: after, events } = act(state, { type: "skip_pack" })
    expect(after.pack).toBeNull()
    expect(events).toContainEqual({ type: "pack_picked", id: "alphabet", taken: null })
    expect(after.letters).toEqual(state.letters)
  })
})

describe("a pack holds the shop", () => {
  const held = () => opened(1, "alphabet")

  it("refuses another purchase until it is resolved", () => {
    const state = {
      ...held(),
      shop: { items: [{ kind: "relic" as const, id: "gambler", cost: 4 }], rerolls: 0 },
    }
    expect(act(state, { type: "buy", index: 0 }).events).toContainEqual({
      type: "rejected",
      refusal: { code: "finish_pack_first" },
    })
  })

  it("refuses a reroll, a sale and the next round until it is resolved", () => {
    const state = held()
    for (const action of [
      { type: "reroll" },
      { type: "next_round" },
      { type: "sell_relic", index: 0 },
    ] satisfies Action[]) {
      expect(act(state, action).events).toContainEqual({
        type: "rejected",
        refusal: { code: "finish_pack_first" },
      })
    }
  })

  it("lets go the moment it is skipped", () => {
    const after = apply(held(), [{ type: "skip_pack" }])
    expect(act(after, { type: "reroll" }).events).not.toContainEqual({
      type: "rejected",
      refusal: { code: "finish_pack_first" },
    })
  })
})

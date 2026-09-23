import { afterAll, describe, expect, it } from "vitest"
import type { Action, Relic, RunState, WordSource } from "../../src/engine"
import { RELIC_BY_ID, reduce, startRun } from "../../src/engine"

/**
 * The scaling machinery, tested with fixtures rather than with shipped content.
 *
 * P1 adds the primitive and no relics that use it, on purpose: anything added to
 * `RELICS` would shift what the shop rolls and move every golden vector, which
 * would cost this phase the one guarantee that proves it is a pure refactor.
 *
 * So the fixtures go into `RELIC_BY_ID` only, the map scoring and the reducer
 * look relics up in, and never into `RELICS`, which is what the shop reads.
 */

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "quazy", "dairy"]),
}

const apply = (state: RunState, actions: Action[], source = words): RunState =>
  actions.reduce((current, action) => reduce(current, action, source).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

const FIXTURES: Relic[] = [
  {
    // Counts its own firings and pays what it has counted.
    id: "test_grower",
    rarity: "common",
    cost: 4,
    onGuess: (ctx) => {
      const grown = ctx.getData("n") + 1
      ctx.setData("n", grown)
      ctx.addMult(grown)
    },
    growth: (instance) => ({ amount: instance.data?.n ?? 0, unit: "mult" }),
  },
  {
    // Writes then reads inside one guess, to prove reads see uncommitted writes.
    id: "test_readback",
    rarity: "common",
    cost: 4,
    onGuess: (ctx) => {
      ctx.setData("x", 41)
      ctx.addMult(ctx.getData("x") + 1)
    },
  },
  {
    // Records both streams so they can be compared for collision.
    id: "test_roller",
    rarity: "common",
    cost: 4,
    onTile: (ctx, _tile, index) => {
      if (index === 0) ctx.setData("tileRoll", Math.floor(ctx.roll() * 1e9))
    },
    onGuess: (ctx) => ctx.setData("guessRoll", Math.floor(ctx.roll() * 1e9)),
  },
  {
    id: "test_ender",
    rarity: "common",
    cost: 4,
    onRoundEnd: (ctx, round) => {
      const grown = (ctx.instance.data?.ends ?? 0) + 1
      ctx.instance.data = { ...ctx.instance.data, ends: grown, solved: round.solved ? 1 : 0 }
      ctx.events.push({
        type: "relic_grew",
        slot: ctx.slot,
        id: "test_ender",
        amount: grown,
        unit: "mult",
      })
    },
  },
]

for (const fixture of FIXTURES) RELIC_BY_ID.set(fixture.id, fixture)
afterAll(() => {
  for (const fixture of FIXTURES) RELIC_BY_ID.delete(fixture.id)
})

/** A run holding one fixture relic, with the given words played. */
function withFixture(id: string, ...guesses: string[]): RunState {
  const base = startRun(1, words).state
  let state: RunState = { ...base, relics: [{ id }] }
  for (const word of guesses) state = apply(state, type(word))
  return state
}

describe("relic instance state", () => {
  it("carries a counter across guesses", () => {
    // CRANE alone is 7 chips x 7 mult, so the relic's contribution reads as a
    // delta: +1 on the first firing, +2 on the second, +3 on the third.
    const state = withFixture("test_grower", "crane", "crane", "crane")
    expect(state.round.guesses.map((guess) => guess.mult)).toEqual([8, 9, 10])
    expect(state.relics[0]?.data).toEqual({ n: 3 })
  })

  it("reads back a write made earlier in the same guess", () => {
    const state = withFixture("test_readback", "crane")
    expect(state.round.guesses[0]?.mult).toBe(7 + 42)
  })

  it("leaves a relic that never writes without a data field", () => {
    // The save is the reason this matters: an empty object on every relic would
    // be pure weight, and `data` is optional precisely so it stays absent.
    const state = withFixture("green_thumb", "crane", "crane")
    expect(state.relics[0]).toEqual({ id: "green_thumb" })
    expect(state.relics[0]?.data).toBeUndefined()
  })

  it("survives the save round trip", () => {
    const state = withFixture("test_grower", "crane", "crane")
    const revived = JSON.parse(JSON.stringify(state)) as RunState
    expect(revived.relics[0]?.data).toEqual({ n: 2 })

    // And keeps counting from where the save left off rather than from zero.
    const resumed = apply(revived, type("crane"))
    expect(resumed.round.guesses[2]?.mult).toBe(10)
  })

  it("does not let one relic read another's counter", () => {
    const base = startRun(1, words).state
    const state: RunState = { ...base, relics: [{ id: "test_grower" }, { id: "test_grower" }] }
    const played = apply(state, type("crane"))
    // Both fire, each counting only itself: 7 + 1 + 1.
    expect(played.round.guesses[0]?.mult).toBe(9)
    expect(played.relics.map((instance) => instance.data)).toEqual([{ n: 1 }, { n: 1 }])
  })

  it("reports what it has grown to", () => {
    const state = withFixture("test_grower", "crane", "crane")
    const relic = RELIC_BY_ID.get("test_grower")
    // The pair, not the sentence. Spelling it is the catalog's job, and asking
    // this test about the wording would make it a test of English.
    expect(relic?.growth?.(state.relics[0] ?? { id: "test_grower" })).toEqual({
      amount: 2,
      unit: "mult",
    })
  })
})

describe("the relic roll", () => {
  it("replays identically from the same seed", () => {
    const first = withFixture("test_roller", "crane")
    const second = withFixture("test_roller", "crane")
    expect(first.relics[0]?.data).toEqual(second.relics[0]?.data)
  })

  it("gives the tile and guess hooks separate streams", () => {
    // Same slot, same guess. The two coordinates differ only in length, so this
    // is the case a naive derive() key would collide on.
    const data = withFixture("test_roller", "crane").relics[0]?.data
    expect(data?.tileRoll).toBeDefined()
    expect(data?.guessRoll).toBeDefined()
    expect(data?.tileRoll).not.toBe(data?.guessRoll)
  })

  it("differs between runs with different seeds", () => {
    const one = { ...startRun(1, words).state, relics: [{ id: "test_roller" }] }
    const two = { ...startRun(9, words).state, relics: [{ id: "test_roller" }] }
    expect(apply(one, type("crane")).relics[0]?.data?.guessRoll).not.toBe(
      apply(two, type("crane")).relics[0]?.data?.guessRoll,
    )
  })
})

describe("the run-level hooks", () => {
  it("fires onRoundEnd with the finished round", () => {
    // BRAID is the answer, so this round ends solved.
    const state = withFixture("test_ender", "braid")
    expect(state.relics[0]?.data).toEqual({ ends: 1, solved: 1 })
  })

  it("fires onRoundEnd on a loss too", () => {
    // Six wrong guesses: the round ends done and unsolved.
    const state = withFixture("test_ender", "crane", "crane", "crane", "crane", "crane", "crane")
    expect(state.round.done).toBe(true)
    expect(state.relics[0]?.data).toEqual({ ends: 1, solved: 0 })
  })

  it("emits relic_grew pointing at the slot that grew", () => {
    const base = startRun(1, words).state
    // Slot 0 is a relic that never grows, so the slot in the event has to have
    // been read from the growing card rather than assumed to be the first one.
    let state: RunState = { ...base, relics: [{ id: "green_thumb" }, { id: "test_ender" }] }
    for (const letter of "braid") {
      state = reduce(state, { type: "type_letter", letter }, words).state
    }
    const { events } = reduce(state, { type: "submit" }, words)
    expect(events).toContainEqual({
      type: "relic_grew",
      slot: 1,
      id: "test_ender",
      amount: 1,
      unit: "mult",
    })
  })

  it("fires onShopEnter before the shop is rolled", () => {
    let shopAtFire: unknown = "never fired"
    RELIC_BY_ID.set("test_shopper", {
      id: "test_shopper",
      rarity: "common",
      cost: 4,
      onShopEnter: (ctx) => {
        shopAtFire = ctx.state.shop
      },
    })

    let state = withFixture("test_shopper", "braid")
    state = apply(state, [{ type: "collect" }])

    // Null at fire time and populated after: the hook genuinely precedes the roll,
    // which is what lets a relic bend the stock it is about to be offered.
    expect(shopAtFire).toBeNull()
    expect(state.shop).not.toBeNull()
    RELIC_BY_ID.delete("test_shopper")
  })
})

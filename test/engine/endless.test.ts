import { describe, expect, it } from "vitest"
import type { Action, RunState, WordSource } from "../../src/engine"
import { derive, getBoss, reduce, roundTargets, STAGES, startRun } from "../../src/engine"
// Not public surface: the shop roll and the boss draw are internals, and this
// test needs them to say *which* shop and *which* boss, not merely that there is
// one.
import { bossForStage } from "../../src/engine/bosses"
import { rollShop } from "../../src/engine/shop"

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "ghost", "audio"]),
}

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, words).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** A run standing on the reward for a given round, with the win still to come. */
function atReward(stage: number, roundIndex: 0 | 1 | 2, won = false): RunState {
  const base = startRun(7, words).state
  return {
    ...base,
    stage,
    roundIndex,
    // Spread rather than `won: undefined`, since the field is absent on a run that
    // has not won, and `exactOptionalPropertyTypes` holds us to the difference.
    ...(won ? { won: true } : {}),
    phase: "reward",
    reward: { base: 5, unusedGuesses: 0, interest: 0, total: 5 },
  }
}

describe("winning the run", () => {
  it("offers the win at the end of the last stage rather than taking it", () => {
    const { state, events } = reduce(atReward(STAGES, 2), { type: "collect" }, words)
    expect(state.phase).toBe("victory")
    expect(state.won).toBe(true)
    expect(events.some((event) => event.type === "run_won")).toBe(true)
    // The reward is still banked. Winning is not a reason to be short-changed.
    expect(state.gold).toBe(startRun(7, words).state.gold + 5)
  })

  it("does not offer it early, or on any other round of the last stage", () => {
    for (const roundIndex of [0, 1] as const) {
      expect(reduce(atReward(STAGES, roundIndex), { type: "collect" }, words).state.phase).toBe(
        "shop",
      )
    }
    expect(reduce(atReward(STAGES - 1, 2), { type: "collect" }, words).state.phase).toBe("shop")
  })

  it("refuses to continue a run that has not won", () => {
    for (const state of [startRun(7, words).state, atReward(STAGES, 2)]) {
      const { events } = reduce(state, { type: "continue_run" }, words)
      expect(events).toContainEqual({ type: "rejected", refusal: { code: "run_not_won" } })
    }
  })
})

describe("playing on past the win", () => {
  const won = reduce(atReward(STAGES, 2), { type: "collect" }, words).state

  it("hands back the shop the victory screen was shown instead of", () => {
    const state = apply(won, [{ type: "continue_run" }])
    expect(state.phase).toBe("shop")
    // Not merely *a* shop: the one those coordinates were always going to roll.
    // The win is a screen held in front of the ordinary loop, not a detour
    // around it, so continuing cannot deal a different stock than banking and
    // playing on would have.
    expect(state.shop).toEqual(rollShop(won, derive(won.seed, "shop", STAGES, 2, 0), 0))
  })

  it("carries the run into stages nobody authored", () => {
    const state = apply(won, [{ type: "continue_run" }, { type: "next_round" }])
    expect(state.stage).toBe(STAGES + 1)
    expect(state.roundIndex).toBe(0)
    expect(state.phase).toBe("round")
    expect(state.round.target).toBe(roundTargets(STAGES + 1)[0])
    expect(state.round.target).toBeGreaterThan(roundTargets(STAGES)[0])
  })

  it("keeps dealing bosses out there, from the band the stages belong to", () => {
    // `bossForStage` wraps within a band rather than running out, so stage 40 is
    // as ordinary a boss round as stage 8; it just repeats one seen before.
    for (let stage = STAGES + 1; stage <= STAGES + 12; stage++) {
      const boss = getBoss(bossForStage({ ...won, stage }))
      expect(boss, `stage ${stage}`).toBeDefined()
      expect(boss?.tier).toBe("late")
    }
  })

  it("offers the win exactly once, however far the run goes", () => {
    // Every third round from here is the last round of an stage at or past
    // `STAGES`, and the gate that offered the win reads exactly that. Without
    // `won` holding it shut, the ending would arrive again every stage.
    for (const stage of [STAGES, STAGES + 1, STAGES + 7]) {
      const { state, events } = reduce(atReward(stage, 2, true), { type: "collect" }, words)
      expect(state.phase, `stage ${stage}`).toBe("shop")
      expect(events.some((event) => event.type === "run_won")).toBe(false)
    }
  })

  it("keeps the win when the endless stages finally kill it", () => {
    const base = startRun(7, words).state
    const doomed: RunState = {
      ...base,
      won: true,
      stage: STAGES + 4,
      phase: "round",
      round: { ...base.round, answer: "braid", target: 999_999, maxGuesses: 1 },
    }
    const dead = apply(doomed, type("crane"))
    expect(dead.phase).toBe("game_over")
    // The run beat the game. What happened after it does not take that back,
    // and the end screen says both things because of this flag.
    expect(dead.won).toBe(true)
  })

  it("stays plain JSON with the flag on it", () => {
    const state = apply(won, [{ type: "continue_run" }, { type: "next_round" }])
    expect(JSON.parse(JSON.stringify(state))).toEqual(state)
  })
})

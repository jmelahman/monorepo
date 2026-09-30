import { describe, expect, it } from "vitest"
import { commandsFor } from "../../src/bench/commands"
import type { Episode } from "../../src/bench/session"
import { episodeOf, Session } from "../../src/bench/session"
import { verify } from "../../src/bench/verify"
import { blindPlayer } from "../helpers/blind"
import { realWords } from "../helpers/words"

function played(seed: number): Episode {
  const session = new Session(seed, realWords)
  const bot = blindPlayer("solver")
  while (!session.done) {
    const actions = bot.next(session.state, realWords)
    if (!actions) break
    for (const command of commandsFor(actions)) {
      session.apply(command)
      if (session.done) break
    }
  }
  session.abandon()
  return episodeOf(session, {
    commit: "test",
    promptHash: "test",
    label: "test",
    suite: "test",
    format: "text",
    wallMs: 0,
  })
}

describe("verifying an episode", () => {
  const died = played(1)
  const won = played(3)

  it("accepts the run as it was played", () => {
    expect(died.result.end).toBe("died")
    expect(won.result.end).toBe("victory")
    expect(verify(died, realWords)).toEqual({ ok: true })
    expect(verify(won, realWords)).toEqual({ ok: true })
  })

  it("takes refusals as recorded, since no step carries them", () => {
    expect(verify({ ...died, result: { ...died.result, refusals: 9 } }, realWords).ok).toBe(true)
  })

  it("catches a result that was edited", () => {
    const verdict = verify({ ...died, result: { ...died.result, roundsCleared: 20 } }, realWords)
    expect(verdict).toEqual({ ok: false, why: expect.stringContaining("roundsCleared") })
  })

  it("catches steps the result was not played with", () => {
    const cut = { ...won, steps: won.steps.slice(0, -1) }
    // The game had not ended, so a victory is a claim the steps cannot back.
    expect(verify(cut, realWords).ok).toBe(false)
    // Recorded as a stall, the same steps are a harness walking away, which is
    // honest, but the counts still have to be the ones the steps produce.
    const stalled = { ...cut, result: { ...won.result, end: "stalled" as const } }
    expect(verify(stalled, realWords)).toEqual({
      ok: false,
      why: expect.stringMatching(/on replay/),
    })
  })

  it("catches a step this engine refuses, and a step after the end", () => {
    const refused = { ...died, steps: [{ type: "buy" as const, index: 0 }, ...died.steps] }
    expect(verify(refused, realWords)).toEqual({
      ok: false,
      why: expect.stringContaining("step 1"),
    })
    const after = { ...died, steps: [...died.steps, { type: "collect" as const }] }
    expect(verify(after, realWords)).toEqual({
      ok: false,
      why: expect.stringContaining("after the run ended"),
    })
  })

  it("catches a run moved to another seed", () => {
    expect(verify({ ...won, seed: 4 }, realWords).ok).toBe(false)
  })
})

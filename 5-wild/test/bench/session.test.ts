import { describe, expect, it } from "vitest"
import { commandsFor } from "../../src/bench/commands"
import { DEFAULT_LIMITS, replayEpisode, Session } from "../../src/bench/session"
import type { RunState } from "../../src/engine"
import { reduce, STAGES, startRun } from "../../src/engine"
import { blindPlayer, firstWinningSeed } from "../helpers/blind"
import { realWords } from "../helpers/words"

const SEEDS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]

/** The sim's loop, straight into `reduce`, with nothing of the benchmark in the way. */
function direct(seed: number): RunState {
  const bot = blindPlayer("solver")
  let state = startRun(seed, realWords, 0).state
  while (state.phase !== "game_over" && state.phase !== "victory") {
    const actions = bot.next(state, realWords)
    if (!actions) break
    for (const action of actions) state = reduce(state, action, realWords).state
  }
  return state
}

function throughSession(seed: number): Session {
  const session = new Session(seed, realWords)
  const bot = blindPlayer("solver")
  while (!session.done) {
    const actions = bot.next(session.state, realWords)
    if (!actions) break
    for (const command of commandsFor(actions)) {
      expect(session.apply(command).ok).toBe(true)
      if (session.done) break
    }
  }
  return session
}

describe("a benchmark session", () => {
  // The wrapper's whole claim: it changes how moves are spoken, never what
  // they do. The same bot through both doors has to land in the same state.
  it("plays exactly the run the engine plays", () => {
    for (const seed of SEEDS) {
      expect(throughSession(seed).state, `seed ${seed}`).toEqual(direct(seed))
    }
  })

  it("replays from its episode to the same state", () => {
    for (const seed of SEEDS.slice(0, 4)) {
      const session = throughSession(seed)
      const replayed = replayEpisode({ seed, ascension: 0, steps: session.steps }, realWords)
      expect(replayed).toEqual({ ok: true, state: session.state })
    }
  })

  // `sim.test.ts` counts a round when the phase turns to reward, which the
  // last round of a won run skips on its way to victory. This does not.
  //
  // The long timeout is the seed search in `firstWinningSeed`: about thirty
  // blind runs before the win, which alone sits near vitest's default five
  // seconds on a cold start.
  it("counts a won run's final round", { timeout: 30_000 }, () => {
    const won = throughSession(firstWinningSeed(realWords))
    expect(won.state.phase).toBe("victory")
    const result = won.result()
    expect(result?.roundsCleared).toBe(STAGES * 3)
    expect(result?.won).toBe(true)
    expect(result?.end).toBe("victory")
  })

  it("applies a command whole or not at all", () => {
    const session = new Session(1, realWords)
    const before = session.state
    const outcome = session.act("guess zzzzz")
    expect(outcome.ok).toBe(false)
    // Not a half-typed draft: the very same object.
    expect(session.state).toBe(before)
    expect(session.refusals).toBe(1)
    expect(session.lastRefusal).toContain("zzzzz")
    expect(session.act("guess crane").ok).toBe(true)
    expect(session.lastRefusal).toBeNull()
  })

  it("keeps `end` for the victory screen", () => {
    const session = new Session(1, realWords)
    expect(session.act("end").ok).toBe(false)
    expect(session.done).toBe(false)
  })

  it("calls a run stalled after enough refusals in a row", () => {
    const session = new Session(1, realWords, 0, { ...DEFAULT_LIMITS, maxRefusals: 3 })
    session.act("dance")
    session.act("dance")
    expect(session.done).toBe(false)
    session.act("dance")
    expect(session.done).toBe(true)
    expect(session.result().end).toBe("stalled")
  })

  /** What a host restart does to a session: a new one, the steps replayed, the tally put back. */
  const resumed = (from: Session) => {
    const next = new Session(from.seed, realWords, from.ascension, from.limits)
    for (const step of from.steps) next.apply(step)
    next.restore(JSON.parse(JSON.stringify(from.checkpoint())))
    return next
  }

  it("keeps the streak across a restart, so a restart cannot reset the stall", () => {
    const session = new Session(1, realWords, 0, { ...DEFAULT_LIMITS, maxRefusals: 3 })
    session.act("dance")
    session.act("dance")
    const next = resumed(session)
    expect(next.checkpoint()).toEqual(session.checkpoint())
    expect(next.lastRefusal).toBe(session.lastRefusal)
    next.act("dance")
    expect(next.done).toBe(true)
    expect(next.result().end).toBe("stalled")
  })

  it("carries a streak an accepted command already cleared as none", () => {
    const session = new Session(1, realWords, 0, { ...DEFAULT_LIMITS, maxRefusals: 3 })
    session.act("dance")
    session.act("dance")
    expect(session.act("guess crane").ok).toBe(true)
    const next = resumed(session)
    expect(next.checkpoint()).toEqual({ refusals: 2, streak: 0, lastRefusal: null })
    next.act("dance")
    next.act("dance")
    expect(next.done).toBe(false)
  })

  it("refuses a tally that no session could have written", () => {
    const session = new Session(1, realWords)
    for (const tally of [
      { refusals: -1, streak: 0, lastRefusal: null },
      { refusals: 1.5, streak: 0, lastRefusal: null },
      { refusals: 1, streak: 2, lastRefusal: '"x": no' },
      { refusals: 1, streak: 1, lastRefusal: null },
      { refusals: 1, streak: 0, lastRefusal: '"x": no' },
    ]) {
      expect(() => session.restore(tally)).toThrow()
    }
    expect(session.checkpoint()).toEqual({ refusals: 0, streak: 0, lastRefusal: null })
  })
})

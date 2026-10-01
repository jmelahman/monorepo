import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import type { Action, RunState } from "../../src/engine"
import { reduce, startRun } from "../../src/engine"
import type { Payload, RunLog, Send } from "../../src/ui/telemetry"
import {
  beginLog,
  expand,
  file,
  flush,
  loadConsent,
  loadLog,
  OUTBOX_CAP,
  payload,
  readOutbox,
  saveLog,
  setConsent,
  stepFor,
} from "../../src/ui/telemetry"
import type { Vector } from "../golden/vectors"
import { replay } from "../golden/vectors"
import { realWords } from "../helpers/words"

const VECTORS: Vector[] = JSON.parse(
  readFileSync(
    join(dirname(fileURLToPath(import.meta.url)), "..", "golden", "vectors.json"),
    "utf8",
  ),
)

/** The logging half of `App.dispatch`, without the app: accepted actions only. */
function logRun(seed: number, actions: readonly Action[], ascension = 0): RunLog {
  let state = startRun(seed, realWords, ascension).state
  const log = beginLog(state, "en", 1)
  for (const action of actions) {
    const { state: next, events } = reduce(state, action, realWords)
    if (events.some((event) => event.type === "rejected")) continue
    const step = stepFor(action, state)
    if (step) log.steps.push(step)
    state = next
  }
  return log
}

/*
 * The claim the whole feature rests on: what the log keeps is enough to replay
 * the run exactly. Every golden vector is a real run through every action the
 * bots know, so each one goes in as raw actions, comes out as a log, and has to
 * replay to the vector's recorded ending, through JSON on the way, since that is
 * what the wire does to it.
 */
describe("a logged run replays to the same run", () => {
  for (const vector of VECTORS) {
    it(vector.name, () => {
      const log = logRun(vector.seed, vector.actions, vector.ascension)
      const wire: RunLog = JSON.parse(JSON.stringify(log))
      const rerun = replay(wire.seed, expand(wire.steps), realWords, wire.ascension)
      expect(rerun.refused).toEqual([])
      expect(rerun.expected).toEqual(vector.expected)
    })
  }

  it("is much smaller than the actions it stands for", () => {
    // What the folding of typing into guesses buys. Measured, not assumed.
    for (const vector of VECTORS) {
      const log = logRun(vector.seed, vector.actions, vector.ascension)
      expect(JSON.stringify(log).length).toBeLessThan(JSON.stringify(vector.actions).length / 2)
    }
  })
})

describe("folding the typing", () => {
  it("keeps the word that was submitted, not the keys that spelled it", () => {
    const state = startRun(1, realWords).state
    const before: RunState = { ...state, round: { ...state.round, draft: "crane" } }
    expect(stepFor({ type: "submit" }, before)).toEqual({ type: "guess", word: "crane" })
    expect(stepFor({ type: "type_letter", letter: "c" }, state)).toBeNull()
    expect(stepFor({ type: "backspace" }, state)).toBeNull()
    expect(stepFor({ type: "buy", index: 2 }, state)).toEqual({ type: "buy", index: 2 })
  })

  it("unfolds a guess into its letters and a submit", () => {
    expect(expand([{ type: "guess", word: "ab" }, { type: "reroll" }])).toEqual([
      { type: "type_letter", letter: "a" },
      { type: "type_letter", letter: "b" },
      { type: "submit" },
      { type: "reroll" },
    ])
  })
})

/* ------------------------------------------------------------- storage */

class FakeStorage {
  readonly items = new Map<string, string>()
  getItem(key: string): string | null {
    return this.items.get(key) ?? null
  }
  setItem(key: string, value: string): void {
    this.items.set(key, value)
  }
  removeItem(key: string): void {
    this.items.delete(key)
  }
}

let store: FakeStorage

beforeEach(() => {
  store = new FakeStorage()
  Object.defineProperty(globalThis, "localStorage", { value: store, configurable: true })
  vi.stubEnv("VITE_TELEMETRY_URL", "https://example.test/runs")
})

afterEach(() => {
  Reflect.deleteProperty(globalThis, "localStorage")
  vi.unstubAllEnvs()
})

/** A run one guess long, since a run with no steps is never queued at all. */
const run = (seed: number): Payload => {
  const log = beginLog(startRun(seed, realWords).state, "en", 1)
  log.steps.push({ type: "guess", word: "crane" })
  return payload(log, startRun(seed, realWords).state, "lost")
}

describe("the saved log", () => {
  it("belongs to the save it was written beside, or to nothing", () => {
    const state = startRun(5, realWords).state
    saveLog(beginLog(state, "en", 3))
    expect(loadLog(state)?.nth).toBe(3)
    // A log from another run is a replay of the wrong run, and is worse than none.
    expect(loadLog({ ...state, seed: 6 })).toBeNull()
    expect(loadLog(null)).toBeNull()
    saveLog(null)
    expect(loadLog(state)).toBeNull()
  })
})

describe("a chosen seed", () => {
  // Flagged so the analysis can set it aside, and absent rather than false on
  // every other run, so the payloads that were already being sent are unchanged.
  it("is flagged on the payload, and only then", () => {
    const state = startRun(5, realWords).state
    expect(payload(beginLog(state, "en", 1, true), state, "lost").seeded).toBe(true)
    expect("seeded" in payload(beginLog(state, "en", 1), state, "lost")).toBe(false)
  })

  it("stays flagged across a resume", () => {
    const state = startRun(5, realWords).state
    saveLog(beginLog(state, "en", 1, true))
    expect(loadLog(state)?.seeded).toBe(true)
  })
})

describe("consent", () => {
  it("sends nothing until the switch is turned on, and nothing from before it", () => {
    expect(loadConsent()).toBeNull()
    file(run(1), null)
    expect(store.items.size).toBe(0)
    // A yes starts from the next run: there is no question on an end screen,
    // so there is no run on screen that the yes could be about.
    setConsent("on")
    expect(readOutbox()).toEqual([])
    file(run(2), "on")
    expect(readOutbox().map((p) => p.seed)).toEqual([2])
  })

  it("drops the queue on a change of mind", () => {
    setConsent("on")
    file(run(1), "on")
    expect(readOutbox()).toHaveLength(1)
    setConsent("off")
    expect(readOutbox()).toEqual([])
    file(run(2), "off")
    expect(readOutbox()).toEqual([])
  })

  it("reads the retired `asked` as no answer", () => {
    store.items.set("5wild:telemetry", "asked")
    expect(loadConsent()).toBeNull()
  })

  it("sends no run that was never played", () => {
    setConsent("on")
    const unplayed = { ...run(1), end: "quit" as const, steps: [] }
    file(unplayed, "on")
    expect(readOutbox()).toEqual([])
    file(run(2), "on")
    expect(readOutbox().map((p) => p.seed)).toEqual([2])
  })

  it("does nothing at all in a build with nowhere to send", () => {
    vi.stubEnv("VITE_TELEMETRY_URL", "")
    file(run(1), "on")
    file(run(2), null)
    expect(store.items.size).toBe(0)
  })
})

describe("the outbox", () => {
  it("keeps the newest runs when it is full", () => {
    for (let seed = 1; seed <= OUTBOX_CAP + 5; seed++) file(run(seed), "on")
    const seeds = readOutbox().map((p) => p.seed)
    expect(seeds).toHaveLength(OUTBOX_CAP)
    expect(seeds[0]).toBe(6)
  })

  const replying =
    (...statuses: (number | "offline")[]): Send =>
    async () => {
      const status = statuses.shift() ?? 200
      if (status === "offline") throw new TypeError("Failed to fetch")
      return { ok: status < 300, status }
    }

  it("empties on success", async () => {
    file(run(1), "on")
    file(run(2), "on")
    await flush(replying(200, 200))
    expect(readOutbox()).toEqual([])
  })

  it("waits out a dead connection or a struggling server", async () => {
    file(run(1), "on")
    file(run(2), "on")
    await flush(replying(200, "offline"))
    expect(readOutbox().map((p) => p.seed)).toEqual([2])
    await flush(replying(503))
    expect(readOutbox().map((p) => p.seed)).toEqual([2])
    await flush(replying(429))
    expect(readOutbox().map((p) => p.seed)).toEqual([2])
  })

  it("drops only the run that was sent, whatever the queue became meanwhile", async () => {
    setConsent("on")
    file(run(1), "on")
    const sent: number[] = []
    let answer = (_: { ok: boolean; status: number }) => {}
    let pending = true
    const send: Send = (body) => {
      sent.push((JSON.parse(body) as Payload).seed)
      if (!pending) return Promise.resolve({ ok: true, status: 200 })
      pending = false
      return new Promise((resolve) => {
        answer = resolve
      })
    }
    const flushed = flush(send)
    try {
      expect(sent).toEqual([1])
      // A change of mind and back, and a run finished, all while the first is
      // in the air: the new run is now the head of the queue.
      setConsent("off")
      setConsent("on")
      file(run(2), "on")
    } finally {
      answer({ ok: true, status: 200 })
      await flushed
    }
    // Run 2 went out on its own turn rather than being dropped on run 1's 200,
    // and run 1 was not sent twice.
    expect(sent).toEqual([1, 2])
    expect(readOutbox()).toEqual([])
  })

  it("gives up on a run the server will never take", async () => {
    file(run(1), "on")
    file(run(2), "on")
    await flush(replying(400, 200))
    expect(readOutbox()).toEqual([])
  })
})

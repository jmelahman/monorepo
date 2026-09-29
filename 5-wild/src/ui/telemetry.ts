import type { Action, RunState } from "../engine"
import { CONTENT_VERSION } from "../engine"
import type { Lang } from "./lang"
import { seal, unseal } from "./seal"

/**
 * Opt-in run replays, for balance.
 *
 * What leaves the device is a *replay*, not a report: the seed, the ascension,
 * and the actions the engine accepted, which is exactly what a golden vector's
 * input is. The engine is deterministic, so every metric anyone could want
 * (where runs die, what relics win, which boss is overtuned) is a replay away
 * on the receiving end, including the ones nobody has thought to ask yet. A
 * report would have fixed the questions at the moment this file was written.
 * It also makes the upload self-checking: a forged or mangled log is one that
 * `reduce` refuses somewhere, and the analysis drops it by name.
 *
 * Everything here is the shell's. The engine does not learn that a run is being
 * watched, and `test/engine-purity.test.ts` would refuse it the `fetch` anyway.
 *
 * What is deliberately absent: an install id, a timestamp, the interface
 * language, every setting. `nth` is the one thing that tells a new player's run
 * from a veteran's, and it is a count, not a name. The server stores the day it
 * received a run and nothing about who sent it.
 */

/**
 * The deployed `telemetry/` worker. In the source rather than in an env file
 * because it is not a secret: it ships inside the bundle, where anyone can read
 * it, and what guards the worker is its origin check and rate limit, not an
 * unknown address. It used to live in a committed `.env.production`, and that
 * was the risk: a committed env file is where the next person puts the next
 * value, and the next value may be one. So every `.env*` is gitignored now, and
 * none is needed to build.
 *
 * Emptying this is the kill switch: nothing about the feature exists for the
 * player then, no prompt, no switch, no outbox.
 */
const DEPLOYED = "https://5wild-telemetry.lahmanja.workers.dev/runs"

/**
 * `VITE_TELEMETRY_URL` still overrides it, for pointing a build somewhere else
 * without an edit. `bun run dev` defaults to a local worker (`bun run dev` in
 * `telemetry/`), so play on a laptop never lands in the real data, and a dev
 * server with nothing on 8787 just leaves runs in the outbox.
 *
 * A function rather than a constant only so the tests can stub the variable;
 * vite still inlines the read at build time.
 */
export const endpoint = (): string =>
  import.meta.env.VITE_TELEMETRY_URL ??
  (import.meta.env.DEV ? "http://localhost:8787/runs" : DEPLOYED)

/** Whether this build has anywhere to send a run, and so whether to ask. */
export const enabled = (): boolean => endpoint() !== ""

/** Bumped when a field changes meaning, not when one is added. */
export const PAYLOAD_VERSION = 1

/**
 * The accepted action, minus the typing.
 *
 * A guess is five `type_letter`s, any number of `backspace`s and a `submit`,
 * and it is the draft at the submit that the engine scores, so one `guess`
 * carries all of it and a run shrinks to a few KB. That is only sound because
 * nothing between keystrokes reads the draft: a consumable used mid-word does
 * not look at it, and letters are destroyed inside `submit`, never between
 * taps. If either stops being true, the typing has to go back in the log.
 */
export type Step =
  | { type: "guess"; word: string }
  | Exclude<Action, { type: "type_letter" | "backspace" | "submit" | "start_run" }>

/** The run being played, as far as the replay needs it. Persisted beside the save. */
export type RunLog = {
  seed: number
  ascension: number
  /** The word list the run was dealt from, which is not the interface language. */
  words: Lang
  /** Which run this is for the player, from the record: 1 is their first. */
  nth: number
  steps: Step[]
}

/**
 * How the run stopped, which is a different question from whether it won: a
 * run can beat stage 8, play on, and lose at 11, and both halves are the data.
 * `abandoned` is a run replaced by a new one without being lost or quit, which
 * is also what an app killed mid-run becomes when the next run starts.
 */
export type RunEnd = "lost" | "quit" | "abandoned"

export type Payload = {
  v: number
  content: number
  build: string
  commit: string
  words: Lang
  seed: number
  ascension: number
  nth: number
  end: RunEnd
  won: boolean
  /** Where it stopped. Derivable by replay; carried so the table can be read by SQL. */
  stage: number
  round: number
  steps: Step[]
}

/**
 * `null` means the switch has never been touched, which sends nothing, the same
 * as `off`. It is kept apart only so the record of an answer stays an answer.
 *
 * The key once held a fourth value, `asked`, from when the end of a player's
 * first run carried the question under its buttons. It reads as `null` like any
 * other stranger, which is what it means now: nobody said yes.
 */
export type Consent = "on" | "off" | null

const LOG_KEY = "5wild:run:log"
const CONSENT_KEY = "5wild:telemetry"
const OUTBOX_KEY = "5wild:telemetry:outbox"

/**
 * Runs waiting for a connection. A phone that plays offline for a month should
 * not come back holding a month; twenty runs is plenty of signal and a bounded
 * amount of storage, and the oldest go first because they were played on the
 * build furthest from the one that will read them.
 */
export const OUTBOX_CAP = 20

/* ----------------------------------------------------------------- log */

export function beginLog(state: RunState, words: Lang, nth: number): RunLog {
  return { seed: state.seed, ascension: state.ascension ?? 0, words, nth, steps: [] }
}

/**
 * What an accepted action adds to the log, if anything. Callers pass only
 * actions the engine accepted: a refusal changed nothing and replays as
 * nothing, so logging it would only teach the analysis to expect refusals.
 */
export function stepFor(action: Action, before: RunState): Step | null {
  switch (action.type) {
    case "type_letter":
    case "backspace":
    case "start_run":
      return null
    case "submit":
      return { type: "guess", word: before.round.draft }
    default:
      return action
  }
}

/** The log back into what `reduce` takes. The inverse of `stepFor`, modulo backspaces. */
export function expand(steps: readonly Step[]): Action[] {
  const actions: Action[] = []
  for (const step of steps) {
    if (step.type === "guess") {
      for (const letter of step.word) actions.push({ type: "type_letter", letter })
      actions.push({ type: "submit" })
    } else actions.push(step)
  }
  return actions
}

export function payload(log: RunLog, state: RunState, end: RunEnd): Payload {
  return {
    v: PAYLOAD_VERSION,
    content: CONTENT_VERSION,
    build: __BUILD_VERSION__,
    commit: __BUILD_COMMIT__,
    words: log.words,
    seed: log.seed,
    ascension: log.ascension,
    nth: log.nth,
    end,
    won: Boolean(state.won),
    stage: state.stage,
    round: state.roundIndex,
    steps: log.steps,
  }
}

/**
 * The log for the saved run, or null if there is none that matches it.
 *
 * The seed check is what makes "complete" mean something. A save written by a
 * build before this one has no log, and a log is only worth sending if it runs
 * all the way back to `startRun`: half a run replays into a different run, which
 * is worse than no run. So a save without its log plays on unlogged and is
 * never sent.
 */
export function loadLog(state: RunState | null): RunLog | null {
  if (!state) return null
  try {
    const raw = localStorage.getItem(LOG_KEY)
    if (!raw) return null
    const log = JSON.parse(unseal(raw)) as RunLog
    return log && log.seed === state.seed && Array.isArray(log.steps) ? log : null
  } catch {
    return null
  }
}

export function saveLog(log: RunLog | null): void {
  try {
    if (log) localStorage.setItem(LOG_KEY, seal(JSON.stringify(log)))
    else localStorage.removeItem(LOG_KEY)
  } catch {
    // The run is still logged in memory; only a relaunch loses it.
  }
}

/* ------------------------------------------------------------- consent */

export function loadConsent(): Consent {
  try {
    const value = localStorage.getItem(CONSENT_KEY)
    return value === "on" || value === "off" ? value : null
  } catch {
    return null
  }
}

/**
 * Record the answer. Only the about and pause sheets ask, so there is never a
 * run on screen to go with a yes: sharing starts from the next finished run. A
 * switch to off empties the outbox, since a run queued under a yes that has
 * since been withdrawn is no longer one the player is offering.
 *
 * There used to be a question on the end screen, asked once after the first
 * run, and a `held` slot beside it for the run it was asked under. It is gone,
 * and the switch on those two sheets is now the only way in. Old installs keep an orphaned `5wild:telemetry:held`; nothing reads it.
 */
export function setConsent(consent: "on" | "off"): void {
  try {
    localStorage.setItem(CONSENT_KEY, consent)
    if (consent === "off") localStorage.removeItem(OUTBOX_KEY)
  } catch {
    // The answer lasts the session. The switch reads off again on the next launch.
  }
}

/* -------------------------------------------------------------- outbox */

/** Queue a finished run if sharing is on; otherwise it goes nowhere. */
export function file(run: Payload, consent: Consent): void {
  if (!enabled() || consent !== "on") return
  enqueue(run)
}

export function readOutbox(): Payload[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(OUTBOX_KEY) ?? "[]")
    return Array.isArray(parsed) ? (parsed as Payload[]) : []
  } catch {
    return []
  }
}

function writeOutbox(runs: readonly Payload[]): void {
  try {
    if (runs.length) localStorage.setItem(OUTBOX_KEY, JSON.stringify(runs))
    else localStorage.removeItem(OUTBOX_KEY)
  } catch {
    // A full store drops the queue for the session. The runs were a courtesy.
  }
}

export function enqueue(run: Payload): void {
  writeOutbox([...readOutbox(), run].slice(-OUTBOX_CAP))
}

export type Send = (body: string) => Promise<{ ok: boolean; status: number }>

/**
 * `text/plain` rather than JSON, which is what keeps this a simple request:
 * the browser sends it without an `OPTIONS` preflight first, so a run costs one
 * round trip instead of two, and the worker reads the body as JSON regardless.
 * `keepalive` lets the last send survive the app being swiped away.
 */
const post: Send = (body) =>
  fetch(endpoint(), {
    method: "POST",
    body,
    keepalive: true,
    headers: { "content-type": "text/plain" },
  })

let flushing = false

/**
 * Send what is queued, oldest first, stopping at the first failure.
 *
 * A network error or a 5xx or a 429 is the server's problem, or the tunnel's,
 * and the run waits for the next launch. Any other 4xx is the server saying it
 * will never take this run, and a run it will never take is dropped rather than
 * retried forever. One flush at a time: a finish during a flush would otherwise
 * send the head of the queue twice.
 */
export async function flush(send: Send = post): Promise<void> {
  if (!enabled() || flushing) return
  flushing = true
  try {
    for (;;) {
      const [head] = readOutbox()
      if (!head) return
      let status: number
      try {
        const response = await send(JSON.stringify(head))
        status = response.ok ? 200 : response.status
      } catch {
        return
      }
      if (status >= 500 || status === 429) return
      // Read again rather than reusing the list above: a switch to off during
      // the send has emptied it, and writing the stale tail back would undo that.
      writeOutbox(readOutbox().slice(1))
    }
  } finally {
    flushing = false
  }
}

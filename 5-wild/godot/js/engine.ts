/**
 * The engine as the desktop build sees it: `src/engine`, unchanged, behind a
 * global `fivewild` object that takes and returns JSON text.
 *
 * Bundled to one classic script (`scripts/bundle.sh`) and run in QuickJS by the
 * `JsContext` class in `native/`. Strings are the whole interface because
 * `RunState` is plain JSON by contract, so JSON text crosses the boundary
 * losslessly and the Godot side never has to know an engine type exists.
 *
 * The wrapper holds the run in progress, the way `app.ts` does for the web
 * build, and that is deliberate rather than a convenience. Godot's JSON writer
 * rounds floats unless asked not to, and a state that made the trip out and
 * back through GDScript on every action would pick up whatever it did to them.
 * So state goes out to be read and to be saved, and never comes back except
 * through `resume`, as the text it left as.
 *
 * The word lists are held here too. `allowed` is a Set of every guessable
 * word, and rebuilding it on every guess would be the slowest thing the game
 * does, so `setWords` builds it once per language.
 */

import type { Action, Reduced, RunState, WordSource } from "../../src/engine"
import { CONTENT_VERSION, reduce, startRun } from "../../src/engine"

let words: WordSource | undefined
let current: RunState | undefined

const lines = (text: string): string[] => text.split("\n").filter(Boolean)

function need<T>(value: T | undefined, what: string): T {
  if (value === undefined) throw new Error(`fivewild: ${what}`)
  return value
}

const settle = (result: Reduced): string => {
  current = result.state
  return JSON.stringify(result)
}

const api = {
  contentVersion: (): string => String(CONTENT_VERSION),

  /** Newline-separated lists, exactly as `public/words/<lang>/` ships them. */
  setWords(answers: string, allowed: string): string {
    words = { answers: lines(answers), allowed: new Set(lines(allowed)) }
    return String(words.answers.length)
  },

  /** A new run. Returns `{state, events}` as JSON, as `dispatch` does. */
  start: (seed: string, ascension: string): string =>
    settle(startRun(Number(seed), need(words, "setWords first"), Number(ascension))),

  /** Picks up a run from the text `state()` gave out. */
  resume(state: string): string {
    current = JSON.parse(state) as RunState
    return state
  },

  /** One action, as JSON; returns `{state, events}` as JSON. */
  dispatch: (action: string): string =>
    settle(
      reduce(
        need(current, "no run: start or resume first"),
        JSON.parse(action) as Action,
        need(words, "setWords first"),
      ),
    ),

  /** The run in progress, as JSON, for a save. */
  state: (): string => JSON.stringify(need(current, "no run")),
}

export type FiveWild = typeof api
;(globalThis as { fivewild?: FiveWild }).fivewild = api

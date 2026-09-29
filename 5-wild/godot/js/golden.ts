/**
 * The golden vectors, replayed inside QuickJS: the parity half of the
 * portability contract in `test/golden.test.ts`. Node recorded them; if this
 * reproduces every one to the last chip, the desktop build is running the same
 * game.
 *
 * `replay` is the recorder's own, imported rather than rewritten, so this
 * compares engines and nothing else. Bundled with `engine.ts`, whose
 * `fivewild` global the seam test drives through GDScript in the same context.
 */

import "./intl-shim"
import "./engine"
import type { WordSource } from "../../src/engine"
import { reduce, startRun } from "../../src/engine"
import type { Vector } from "../../test/golden/vectors"
import { replay } from "../../test/golden/vectors"

const lines = (text: string): string[] => text.split("\n").filter(Boolean)

const api = {
  /**
   * Replays one vector and returns what it produced, as JSON, for GDScript to
   * compare against the recording. The comparison happens over there so a
   * failure prints the two values side by side in gdUnit's report.
   */
  replay(vector: string, answers: string, allowed: string): string {
    const v = JSON.parse(vector) as Vector
    const words: WordSource = { answers: lines(answers), allowed: new Set(lines(allowed)) }
    return JSON.stringify(replay(v.seed, v.actions, words, v.ascension ?? 0))
  },

  /**
   * Where the vector's run ends, played entirely inside JS. The seam test
   * plays the same actions one `fivewild.dispatch` at a time from GDScript and
   * must arrive at exactly this text, which is what shows nothing is lost or
   * reshaped on the way through Godot.
   */
  finalState(vector: string, answers: string, allowed: string): string {
    const v = JSON.parse(vector) as Vector
    const words: WordSource = { answers: lines(answers), allowed: new Set(lines(allowed)) }
    let state = startRun(v.seed, words, v.ascension ?? 0).state
    for (const action of v.actions) state = reduce(state, action, words).state
    return JSON.stringify(state)
  },
}

;(globalThis as { fivewildGolden?: typeof api }).fivewildGolden = api

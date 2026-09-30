/**
 * Whether an episode is the run it says it is.
 *
 * An episode's `result` is a claim and its `steps` are the evidence: the seed
 * and the steps are the whole run, so the result can be computed again rather
 * than believed. That matters once results travel. A table assembled from
 * bundles that came out of five worktrees and two devcontainers is only as good
 * as the least careful of them, and a hand-edited file, a label reused across
 * two machines, or a worktree that was quietly on a different engine all show
 * up here as a replay that does not land where the file says it did.
 *
 * Two fields cannot be recomputed, and are checked for consistency instead.
 * `refusals` never reaches the step list, since a refused command changes
 * nothing, so it is taken as recorded. And `end` is only derivable when the game
 * ended the run itself: a run the harness stopped (a refusal streak, a walk-away)
 * stops on an ordinary state, and all a replay can say is that the game had not
 * ended it, which is exactly what `stalled` claims.
 */

import type { WordSource } from "../engine"
import type { Episode, Result } from "./session"
import { DEFAULT_LIMITS, Session } from "./session"

export type Verdict = { ok: true } | { ok: false; why: string }

export function verify(episode: Episode, words: WordSource): Verdict {
  const session = new Session(episode.seed, words, episode.ascension, {
    ...DEFAULT_LIMITS,
    endless: episode.endless,
  })
  for (const [index, step] of episode.steps.entries()) {
    // A step after the game ended the run is a step no host would have taken.
    if (session.done) return { ok: false, why: `step ${index + 1} comes after the run ended` }
    const outcome = session.apply(step)
    if (!outcome.ok) return { ok: false, why: `step ${index + 1} is refused: ${outcome.reason}` }
  }
  session.refusals = episode.result.refusals

  const replayed = session.result()
  const recorded = episode.result
  if (session.done) {
    if (recorded.end !== replayed.end) {
      return { ok: false, why: `ends ${replayed.end} on replay, recorded ${recorded.end}` }
    }
  } else if (recorded.end !== "stalled") {
    // `retired` is the one ending the model chooses, and it adds no step; its
    // replay stops on the victory screen of an endless run and nowhere else.
    const retired =
      recorded.end === "retired" && episode.endless && session.state.phase === "victory"
    if (!retired) return { ok: false, why: `recorded ${recorded.end}, but the game had not ended` }
  }
  const fields: Array<keyof Result> = [
    "won",
    "stage",
    "round",
    "roundsCleared",
    "roundsSolved",
    "guesses",
    "score",
    "steps",
  ]
  for (const field of fields) {
    if (recorded[field] !== replayed[field]) {
      return {
        ok: false,
        why: `${field} is ${String(replayed[field])} on replay, recorded ${String(recorded[field])}`,
      }
    }
  }
  return { ok: true }
}

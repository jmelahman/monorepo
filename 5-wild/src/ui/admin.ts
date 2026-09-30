/**
 * A tester's back door: retry the round you just lost, or just played badly.
 *
 * A run is one seed and a chain of choices, and the round worth testing is
 * usually twelve rounds in, behind a shop, a boss and a relic that only shows
 * up on the right shelf. Losing it meant earning it again from stage one. So
 * every arrival at a round is snapshotted, and Alt+R puts the run back to that
 * snapshot: same word, same hand, same gold, as the intro card first showed it.
 *
 * On under `bun run dev` and nowhere else: a production build has no way in,
 * so there is no switch for a player to find.
 *
 * The snapshot lives in its own key rather than in `RunState`, for the reason
 * everything outliving a dispatch does: the engine must not learn that a run
 * can be rewound. It is sealed like the save, since it holds the answer.
 */

import type { RunState } from "../engine"
import type { Lang } from "./lang"
import { seal, unseal } from "./seal"

const CHECKPOINT_KEY = "5wild:admin:checkpoint"

interface Checkpoint {
  state: RunState
  /**
   * The list it was dealt from. A checkpoint restored against another
   * language's words is a round whose answer is not a legal guess, which is
   * the same stranding `5wild:run:lang` exists to prevent.
   */
  lang: Lang
}

/** Vite folds this to a constant, so a production bundle drops the rest. */
export const admin: boolean = import.meta.env.DEV

export function stashCheckpoint(state: RunState, lang: Lang): void {
  if (!admin) return
  try {
    const checkpoint: Checkpoint = { state, lang }
    localStorage.setItem(CHECKPOINT_KEY, seal(JSON.stringify(checkpoint)))
  } catch {
    // Nothing to retry to, which is where a tester without this started.
  }
}

export function loadCheckpoint(lang: Lang): RunState | null {
  if (!admin) return null
  try {
    const raw = localStorage.getItem(CHECKPOINT_KEY)
    if (!raw) return null
    const checkpoint = JSON.parse(unseal(raw)) as Checkpoint
    return checkpoint.lang === lang ? checkpoint.state : null
  } catch {
    return null
  }
}

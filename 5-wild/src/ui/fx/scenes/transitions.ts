import type { Action, GameEvent } from "../../../engine"
import { play, type Snapshot, snapshot } from "../flip"
import { juiced } from "../motion"
import { dealt } from "./board"
import type { SceneContext } from "./context"

/**
 * What happens between screens, and the big moments on them: round intro,
 * boss intro, round won, game over, run won, sheets opening. Owned by phase 6
 * of `plans/ui-overhaul.md`; the FLIP wiring and the render hooks are the
 * foundation's.
 *
 * `App.render` calls `leaving` before it throws the old screen away and
 * `arrived` after the new one is standing, which is the only window in which
 * both are measurable.
 */

export type Leaving = {
  /** The old screen's kind (see `screenKind` in `app.ts`), or null if none. */
  from: string | null
  rects: Snapshot
}

export function leaving(root: HTMLElement, from: string | null): Leaving {
  return { from, rects: snapshot(root) }
}

export function arrived(root: HTMLElement, to: string | null, was: Leaving): void {
  if (!juiced()) return
  play(root, was.rects)
  const screen = root.firstElementChild
  if (!(screen instanceof HTMLElement) || to === was.from) return
  const isRound = (kind: string | null) => kind?.split(" ").includes("round-screen") ?? false
  if (isRound(to) && !isRound(was.from)) dealt(screen)
}

/** Round won, run won, game over and the like, after their render. */
export function playMoments(
  _action: Action,
  _events: readonly GameEvent[],
  _ctx: SceneContext,
): void {}

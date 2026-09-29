import type { RunState } from "../../../engine"
import type { Sound } from "../../audio"

/**
 * What a scene is handed, and all it is handed.
 *
 * Scenes are the phase-owned halves of `App`: the scoring show, the shop's
 * card work, the board, the moments between screens. They were split out so
 * that several people could work on them at once without all editing the one
 * file that owns dispatch, and this is the seam: `App` builds one of these per
 * sequence and calls in, and no scene holds a reference to the app or can
 * reach its private state.
 *
 * `state` is the run *after* the action, because the engine has already
 * committed by the time anything plays. A scene that needs the run as it was
 * before reads it off the events, which carry their own running figures for
 * exactly this reason (see `guess_scored`).
 */
export type SceneContext = {
  /** `#app`. Survives renders; its children do not. */
  root: HTMLElement
  /** The screen on stage when the scene began. May be replaced by a render. */
  screen: HTMLElement
  state: RunState
  sound: Sound
}

/** The live screen, or null between a clear and its append. */
export const screenOf = (root: HTMLElement): HTMLElement | null =>
  root.firstElementChild instanceof HTMLElement ? root.firstElementChild : null

import type { Action, GameEvent } from "../../../engine"
import { juiced } from "../motion"
import type { SceneContext } from "./context"
import { playShopEvents } from "./shop"
import { playMoments } from "./transitions"

/**
 * Everything that is not a guess scoring, played after the render that shows
 * its result.
 *
 * Until the table, only a submit told a story: every other action rendered its
 * outcome and bumped the gold counter, so a relic bought, a pack torn open, a
 * round won or a card sold all simply *were*. The engine already says what
 * happened in the events it returns; this hands them to the scenes that own
 * each kind of moment.
 *
 * A router and nothing else, so that the shop's scene and the moments' scene
 * can be written side by side without either editing the other. Table only:
 * the phone keeps its render-and-bump.
 */
export function playEvents(action: Action, events: readonly GameEvent[], ctx: SceneContext): void {
  if (!juiced() || events.length === 0) return
  playShopEvents(action, events, ctx)
  playMoments(action, events, ctx)
}

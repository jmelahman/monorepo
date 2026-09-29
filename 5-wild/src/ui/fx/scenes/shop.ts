import type { Action, GameEvent } from "../../../engine"
import type { SceneContext } from "./context"

/**
 * The shop, relics and packs on the table: dealing, buying, selling, rerolling,
 * tearing a pack open. Owned by phase 4 of `plans/ui-overhaul.md`.
 *
 * Called after the render that shows an action's result, only when `juiced()`.
 */
export function playShopEvents(
  _action: Action,
  _events: readonly GameEvent[],
  _ctx: SceneContext,
): void {}

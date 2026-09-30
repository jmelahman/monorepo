/**
 * What the benchmark host tells a spectator, and nothing it has to decide.
 *
 * The host owns the run. The browser is handed the seed, the ascension and the
 * language, deals the same run from them, and is then handed each accepted
 * command as the `Step` the telemetry log already speaks. Nothing else crosses:
 * not the state, not the observation, and above all not the answer, which the
 * spectator's own engine derives from the seed like every other run it deals.
 * That keeps the feed small enough to replay to a tab that opens mid-run, and
 * it makes the spectator a second, independent check on the host: a step its
 * engine refuses means the two are not running the same game.
 *
 * Refusals are sent too, although they change nothing, because watching a model
 * fumble the grammar five times is half of what a spectator is for, and the
 * episode file does not keep them.
 *
 * Shared between `tools/bench/feed.ts`, which writes these, and
 * `src/ui/spectate.ts`, which reads them, so the two cannot disagree about a
 * field name without the build saying so.
 */

import type { Step } from "../ui/telemetry"
import type { Result } from "./session"

export type FeedEvent =
  | {
      type: "start"
      /** Counts runs since the host started, so a late `step` for a finished run is dropped. */
      run: number
      label: string
      seed: number
      ascension: number
      lang: string
      /** The host's `CONTENT_VERSION`. A spectator on another one will diverge, and says so first. */
      content: number
    }
  | { type: "step"; run: number; n: number; step: Step }
  | { type: "refused"; run: number; command: string; reason: string }
  | { type: "end"; run: number; result: Result }

/** Where the host listens unless told otherwise, and the path the spectator asks for. */
export const FEED_PORT = 7777
export const FEED_PATH = "/feed"

/**
 * Accepts what `JSON.parse` of one SSE message gave, or refuses it. The
 * spectator reads from any URL it is pointed at, so it checks shape before it
 * reduces anything, the same reason the telemetry worker does.
 */
export function readFeedEvent(raw: unknown): FeedEvent | null {
  if (typeof raw !== "object" || raw === null) return null
  const event = raw as Record<string, unknown>
  if (typeof event.run !== "number") return null
  switch (event.type) {
    case "start":
      return typeof event.seed === "number" &&
        typeof event.ascension === "number" &&
        typeof event.lang === "string" &&
        typeof event.label === "string" &&
        typeof event.content === "number"
        ? (event as FeedEvent)
        : null
    case "step":
      return typeof event.n === "number" &&
        typeof event.step === "object" &&
        event.step !== null &&
        typeof (event.step as { type?: unknown }).type === "string"
        ? (event as FeedEvent)
        : null
    case "refused":
      return typeof event.command === "string" && typeof event.reason === "string"
        ? (event as FeedEvent)
        : null
    case "end":
      return typeof event.result === "object" && event.result !== null ? (event as FeedEvent) : null
    default:
      return null
  }
}

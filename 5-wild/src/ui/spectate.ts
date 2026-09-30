/**
 * The game as a window onto someone else's run.
 *
 * Opened as `?watch=<host>`, where the host is the benchmark's feed
 * (`tools/bench/feed.ts`, port 7777 by default). The host deals a run from a
 * seed and forwards every move a model makes; this tab deals the same run
 * from the same seed and plays the moves through the ordinary `App`, so what
 * is on screen is the real game, animations, sound and all, and not a drawing
 * of it. No state crosses the wire and neither does the answer: this tab's
 * engine derives it, as it would for a player.
 *
 * That makes the spectator a check as well as a view. A move the host accepted
 * and this engine refuses means the two are not playing the same game, a
 * different content version or word list, and the banner says so and stops
 * rather than going on to draw a run that is not the one being scored.
 *
 * Moves are queued and played one at a time at animation pace, so a model that
 * answers in a second does not outrun its own scoring scene, and a replayed
 * episode, which arrives all at once, plays out as though live. Nothing here
 * touches storage; `App.watch` is where that promise is kept.
 */

import type { FeedEvent } from "../bench/feed"
import { FEED_PATH, readFeedEvent } from "../bench/feed"
import { CONTENT_VERSION } from "../engine"
import type { App } from "./app"
import { clear, h } from "./dom"
import type { Lang } from "./lang"
import { ui } from "./lang"

const LANGS: readonly Lang[] = ["en", "es", "fr", "de"]

export function spectate(app: App, host: string): void {
  app.watch()

  const banner = h("div", { class: "watch-banner", role: "status", "aria-live": "polite" })
  document.body.append(banner)

  let label = ""
  let seed: number | null = null
  let ascension = 0
  let move = 0
  let over = false
  let note: { text: string; bad: boolean } | null = null
  let lost = false

  const draw = () => {
    const copy = ui().watch
    const head =
      seed === null
        ? [copy.waiting]
        : [
            h("span", { class: "watch-label" }, label),
            " · ",
            copy.seed(seed, ascension),
            " · ",
            copy.move(move),
            " · ",
            over ? copy.over : h("span", { class: "watch-live" }, "●"),
          ]
    const shown = lost ? { text: copy.lost, bad: true } : note
    clear(banner).append(
      ...head,
      ...(shown ? [h("span", { class: `watch-note${shown.bad ? " bad" : ""}` }, shown.text)] : []),
    )
  }
  draw()

  // One run is on screen at a time. A reconnect gets the current run's history
  // again from the top, so everything already played is recognised and dropped
  // here, by run and by move number, rather than dealt twice.
  let current: string | null = null
  let played = 0
  let diverged = false
  const queue: FeedEvent[] = []
  let draining = false

  const drain = async () => {
    if (draining) return
    draining = true
    try {
      for (let event = queue.shift(); event; event = queue.shift()) await play(event)
    } finally {
      draining = false
    }
  }

  const play = async (event: FeedEvent): Promise<void> => {
    switch (event.type) {
      case "start": {
        label = event.label
        seed = event.seed
        ascension = event.ascension
        move = 0
        over = false
        diverged = false
        note =
          event.content === CONTENT_VERSION
            ? null
            : { text: ui().watch.version(event.content, CONTENT_VERSION), bad: true }
        draw()
        const lang = LANGS.find((known) => known === event.lang) ?? "en"
        await app.watchRun(event.seed, event.ascension, lang)
        return
      }
      case "step": {
        if (diverged) return
        move = event.n
        if (note && !note.bad) note = null
        draw()
        if (!(await app.watchStep(event.step))) {
          diverged = true
          note = { text: ui().watch.diverged(event.n), bad: true }
          draw()
        }
        return
      }
      case "refused":
        if (diverged) return
        note = { text: ui().watch.refused(event.command, event.reason), bad: false }
        draw()
        return
      case "end":
        over = true
        draw()
        return
    }
  }

  const source = new EventSource(new URL(FEED_PATH, host).href)
  source.onopen = () => {
    lost = false
    draw()
  }
  source.onerror = () => {
    // EventSource retries by itself; this only says so.
    lost = true
    draw()
  }
  source.onmessage = (message: MessageEvent<string>) => {
    let event: FeedEvent | null
    try {
      event = readFeedEvent(JSON.parse(message.data))
    } catch {
      return
    }
    if (!event) return
    if (event.type === "start") {
      const key = `${event.run}:${event.seed}:${event.label}`
      if (key === current) return
      current = key
      played = 0
    } else if (current === null || !current.startsWith(`${event.run}:`)) {
      return
    }
    if (event.type === "step") {
      if (event.n <= played) return
      played = event.n
    }
    queue.push(event)
    void drain()
  }
}

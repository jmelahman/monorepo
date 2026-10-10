/**
 * The spectator feed: server-sent events on a local port.
 *
 * SSE rather than a socket because the traffic only ever goes one way and
 * `EventSource` reconnects by itself, which is the whole of what a tab left
 * open across a host restart needs. `node:http` rather than `Bun.serve` so the
 * host runs under either, and because an MCP host that needed a particular
 * runtime would be one more thing for a harness to get right.
 *
 * The feed keeps the current run's events and replays them to anyone who
 * connects, so a tab opened at stage 5 deals the run from the seed and catches
 * up rather than waiting for the next one. Only the current run: a spectator
 * is watching a game, not auditing a suite, and the episodes on disk are the
 * audit.
 *
 * Run directly, it replays a finished episode instead:
 *
 *   bun tools/bench/feed.ts --replay bench/results/<file>.json
 *
 * which gives the spectator one input whether the run is live or recorded.
 */

import { readFileSync } from "node:fs"
import { createServer, type ServerResponse } from "node:http"
import type { FeedEvent } from "../../src/bench/feed"
import { FEED_PATH, FEED_PORT } from "../../src/bench/feed"
import type { Episode } from "../../src/bench/session"
import { CONTENT_VERSION } from "../../src/engine"

export type Feed = {
  publish(event: FeedEvent): void
  close(): void
  readonly url: string
}

const log = (...parts: unknown[]) => console.error("[feed]", ...parts)

/**
 * Never throws for a port in use. Two agent sessions each launching the MCP
 * host is ordinary, and the second losing its spectator is a far smaller
 * failure than the second losing its benchmark.
 */
export function startFeed(port = FEED_PORT): Feed {
  const clients = new Set<ServerResponse>()
  let history: FeedEvent[] = []

  const send = (client: ServerResponse, event: FeedEvent) =>
    client.write(`data: ${JSON.stringify(event)}\n\n`)

  const server = createServer((request, response) => {
    // The dev server is on another origin, and so is `vite preview`, and the
    // spectator can be pointed at this from either. Nothing here is private:
    // it is a run a spectator could deal for itself from the seed.
    response.setHeader("Access-Control-Allow-Origin", "*")
    const path = new URL(request.url ?? "/", "http://localhost").pathname
    if (path !== FEED_PATH) {
      response.writeHead(404).end()
      return
    }
    response.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    })
    response.write(": 5 wild bench feed\n\n")
    for (const event of history) send(response, event)
    clients.add(response)
    request.on("close", () => clients.delete(response))
  })

  server.on("error", (error: NodeJS.ErrnoException) => {
    if (error.code === "EADDRINUSE") log(`port ${port} is taken; running without a spectator feed`)
    else log(error.message)
  })
  server.listen(port, () => log(`spectator feed on http://localhost:${port}${FEED_PATH}`))

  // A comment line every so often, so a proxy or a sleeping laptop that drops
  // idle connections does not strand a spectator between two slow moves.
  const keepAlive = setInterval(() => {
    for (const client of clients) client.write(": keep-alive\n\n")
  }, 15_000)
  keepAlive.unref()

  return {
    url: `http://localhost:${port}`,
    publish(event) {
      if (event.type === "start") history = []
      history.push(event)
      for (const client of clients) send(client, event)
    },
    close() {
      clearInterval(keepAlive)
      for (const client of clients) client.end()
      server.close()
    },
  }
}

/* ------------------------------------------------------------------ replay */

/**
 * Returns the feed so a caller that is not a person at a terminal can close it:
 * `tools/bench/record.ts` replays one episode after another on the same port.
 */
export function replay(path: string, port: number): Feed {
  const episode = JSON.parse(readFileSync(path, "utf8")) as Episode
  if (episode.content !== CONTENT_VERSION) {
    log(
      `episode was recorded on content ${episode.content} and this is ${CONTENT_VERSION}; ` +
        "the spectator will most likely diverge",
    )
  }
  const feed = startFeed(port)
  // All at once: the spectator paces what it is given at animation speed, so
  // there is nothing to gain from trickling, and a tab opened late gets the
  // same history either way.
  feed.publish({
    type: "start",
    run: 1,
    label: `${episode.label} (replay)`,
    seed: episode.seed,
    ascension: episode.ascension,
    lang: episode.lang,
    content: episode.content,
  })
  for (const [n, step] of episode.steps.entries())
    feed.publish({ type: "step", run: 1, n: n + 1, step })
  feed.publish({ type: "end", run: 1, result: episode.result })
  log(`replaying ${episode.steps.length} steps of seed ${episode.seed}; Ctrl-C to stop`)
  return feed
}

if (import.meta.main) {
  const args = process.argv.slice(2)
  const at = args.indexOf("--replay")
  const file = at >= 0 ? args[at + 1] : undefined
  const portAt = args.indexOf("--port")
  const port = portAt >= 0 ? Number(args[portAt + 1]) : FEED_PORT
  if (!file) {
    console.error("usage: bun tools/bench/feed.ts --replay <episode.json> [--port 7777]")
    process.exit(2)
  }
  replay(file, port)
}

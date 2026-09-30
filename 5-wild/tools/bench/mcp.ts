/**
 * The benchmark as an MCP server over stdio.
 *
 *   bun tools/bench/mcp.ts   (registered as `5wild` in .mcp.json)
 *
 * Any agent harness that speaks MCP can then play: `rules` once, `new_run`,
 * and `act` until the run is over. The harness brings the model and this
 * brings everything else, which is the line the benchmark draws: the seeds, the
 * briefing, the observation, the caps and the scoring are fixed here, and the
 * episode records the prompt hash and commit that fixed them. What it cannot
 * fix is the harness, so a result is a model *in* a harness, and the label is
 * where that is written down.
 *
 * Hand-rolled rather than on `@modelcontextprotocol/sdk`: four tools over
 * newline-delimited JSON-RPC is under a hundred lines, and the SDK would be the
 * repo's first runtime dependency that the game itself never loads.
 *
 * stdout is the protocol and nothing else may write to it. Everything a person
 * should see goes to stderr, which every harness either shows or drops.
 *
 * Flags: `--suite <path>` (default `bench/suite.json`), `--port <n>` for the
 * spectator feed (default 7777), `--no-feed`, `--format text|json` (overrides
 * the suite's), `--label <name>` (the default when `new_run` is not given one).
 */

import { createInterface } from "node:readline"
import type { Format } from "../../src/bench/observe"
import { FORMATS, readFormat } from "../../src/bench/observe"
import { promptHash, rulesText } from "../../src/bench/rules"
import { summarize, table } from "../../src/bench/score"
import type { Limits } from "../../src/bench/session"
import { DEFAULT_LIMITS, episodeOf, Session } from "../../src/bench/session"
import { CONTENT_VERSION } from "../../src/engine"
import type { Feed } from "./feed"
import { startFeed } from "./feed"
import type { Suite } from "./host"
import {
  clearLive,
  commit,
  loadLive,
  loadSuite,
  nextSeed,
  played,
  RESULTS,
  readEpisodes,
  saveLive,
  slug,
  writeEpisode,
} from "./host"
import { wordsFor } from "./words"

const log = (...parts: unknown[]) => console.error("[5wild]", ...parts)

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}

const suite: Suite = loadSuite(flag("--suite"))
const defaultFormat: Format = readFormat(flag("--format")) ?? suite.format
const defaultLabel = flag("--label") ?? "unlabelled"
const feed: Feed | null = args.includes("--no-feed")
  ? null
  : startFeed(Number(flag("--port") ?? 7777))
const words = wordsFor("en")
const stamp = { commit: commit(), promptHash: promptHash() }
const limits: Limits = { ...DEFAULT_LIMITS, endless: suite.endless }

/* ------------------------------------------------------------------ the run */

type Live = {
  session: Session
  label: string
  suite: string | null
  format: Format
  started: number
  run: number
}
let live: Live | null = null
let runs = 0

/**
 * File the run and tell the spectator. Called once per run, when the game or
 * the caps end it, or when an ad-hoc run is dropped for another: a run
 * abandoned for a fresh one is still a result, scored as a stall. A suite run
 * is never dropped that way; see `LiveRun`.
 */
function close(current: Live): string {
  current.session.abandon()
  if (current.suite !== null) clearLive(current.label)
  const episode = episodeOf(current.session, {
    ...stamp,
    label: current.label,
    suite: current.suite,
    format: current.format,
    wallMs: Date.now() - current.started,
  })
  const path = writeEpisode(episode)
  feed?.publish({ type: "end", run: current.run, result: episode.result })
  log(
    `seed ${episode.seed}: ${episode.result.end}, ${episode.result.roundsCleared} rounds -> ${path}`,
  )
  return path
}

function describeEnd(current: Live): string {
  const result = current.session.result()
  const left = suite.seeds.filter((seed) => !played(current.label, seed, suite.ascension)).length
  return [
    "",
    `RUN ENDED (${result.end}): ${result.roundsCleared} rounds cleared, reached stage ${result.stage}, ` +
      `${result.won ? "won" : "not won"}, score ${result.score.toLocaleString("en-US")}.`,
    left
      ? `${left} run${left === 1 ? "" : "s"} left in the suite. Call new_run to deal the next.`
      : "The suite is complete.",
  ].join("\n")
}

/** Write a suite run's progress down, so a host that dies mid-run loses nothing. */
function keep(current: Live): void {
  if (current.suite === null || current.session.done) return
  saveLive(current.label, {
    seed: current.session.seed,
    ascension: current.session.ascension,
    format: current.format,
    steps: current.session.steps,
    refusals: current.session.refusals,
    started: current.started,
  })
}

function begin(next: Omit<Live, "run">, resumed: boolean): ToolResult {
  runs++
  live = { ...next, run: runs }
  const { session, label } = next
  feed?.publish({
    type: "start",
    run: runs,
    label,
    seed: session.seed,
    ascension: session.ascension,
    lang: "en",
    content: CONTENT_VERSION,
  })
  // A resumed run is replayed to the spectator whole, so a tab watching the
  // new connection deals the same run and catches up to where it stands.
  for (const [n, step] of session.steps.entries()) {
    feed?.publish({ type: "step", run: runs, n: n + 1, step })
  }
  log(`${label}: seed ${session.seed}, ascension ${session.ascension}${resumed ? ", resumed" : ""}`)
  const head = resumed
    ? `RESUMED seed ${session.seed}, ascension ${session.ascension}, after ${session.steps.length} commands`
    : `NEW RUN seed ${session.seed}, ascension ${session.ascension}`
  return text(`${head}\n\n${session.observe(next.format)}`)
}

/* ------------------------------------------------------------------ tools */

type ToolResult = { content: Array<{ type: "text"; text: string }>; isError?: boolean }
const text = (body: string, isError = false): ToolResult => ({
  content: [{ type: "text", text: body }],
  ...(isError ? { isError } : {}),
})

const formatProp = {
  type: "string",
  enum: [...FORMATS],
  description: "Observation format. Defaults to the suite's.",
}

const TOOLS = [
  {
    name: "rules",
    description:
      "The rules of 5 Wild and how to play it through this server. Read once before playing.",
    inputSchema: { type: "object", properties: {} },
  },
  {
    name: "new_run",
    description:
      "Deal the next run of the benchmark suite and return the first observation. " +
      "If this label has a suite run in progress, returns to it instead: a suite run is " +
      "played to its end, and cannot be swapped for another deal.",
    inputSchema: {
      type: "object",
      properties: {
        label: {
          type: "string",
          description:
            "Who is playing: model and harness, e.g. 'claude-opus-5-5 / claude-code'. Results are filed under it.",
        },
        seed: {
          type: "integer",
          description:
            "Play this seed instead of the suite's next. Such runs are filed but do not advance the suite.",
        },
        ascension: {
          type: "integer",
          minimum: 0,
          description: "Only with seed. Defaults to the suite's.",
        },
        format: formatProp,
      },
    },
  },
  {
    name: "observe",
    description: "The current observation, without acting.",
    inputSchema: { type: "object", properties: { format: formatProp } },
  },
  {
    name: "act",
    description:
      "Send one command (e.g. 'guess crane', 'buy 2', 'collect', 'next') and get the next observation back.",
    inputSchema: {
      type: "object",
      properties: { command: { type: "string" } },
      required: ["command"],
    },
  },
]

function callTool(name: string, input: Record<string, unknown>): ToolResult {
  switch (name) {
    case "rules":
      return text(rulesText())

    case "new_run": {
      const label =
        typeof input.label === "string" && input.label.trim() ? input.label.trim() : defaultLabel
      const format = readFormat(input.format) ?? defaultFormat
      const chosen = typeof input.seed === "number" ? Math.trunc(input.seed) : null
      // Back to a suite run in progress, whether this process was holding it or
      // an earlier one left it on disk.
      if (chosen === null) {
        if (live && !live.session.done && live.suite !== null && live.label === label) {
          return begin({ ...live }, true)
        }
        const stored = loadLive(label)
        if (stored && stored.ascension === suite.ascension && suite.seeds.includes(stored.seed)) {
          if (live && !live.session.done && live.suite === null) close(live)
          const session = new Session(stored.seed, words, stored.ascension, limits)
          for (const step of stored.steps) session.apply(step)
          session.refusals = stored.refusals
          return begin({ session, label, suite: suite.name, format, started: stored.started }, true)
        }
      }
      const seed = chosen ?? nextSeed(suite, label)
      if (seed === null) {
        const episodes = readEpisodes(`${RESULTS}/${slug(label)}`).filter(
          (episode) => episode.suite === suite.name && episode.ascension === suite.ascension,
        )
        return text(
          `The suite "${suite.name}" is complete for "${label}".\n\n` +
            table([{ label, summary: summarize(episodes.map((episode) => episode.result)) }]),
        )
      }
      const ascension =
        chosen !== null && typeof input.ascension === "number"
          ? Math.max(0, Math.trunc(input.ascension))
          : suite.ascension
      // An ad-hoc run dropped for another is filed as a stall. A suite run left
      // for an ad-hoc one is not touched: it is on disk, and waits.
      if (live && !live.session.done && live.suite === null) close(live)
      const next: Omit<Live, "run"> = {
        session: new Session(seed, words, ascension, limits),
        label,
        suite: chosen === null ? suite.name : null,
        format,
        started: Date.now(),
      }
      keep({ ...next, run: 0 })
      return begin(next, false)
    }

    case "observe": {
      if (!live) return text("No run yet. Call new_run.", true)
      return text(live.session.observe(readFormat(input.format) ?? live.format))
    }

    case "act": {
      if (!live) return text("No run yet. Call new_run.", true)
      if (typeof input.command !== "string") return text('"command" must be a string.', true)
      const current = live
      const session = current.session
      const wasDone = session.done
      const outcome = session.act(input.command)
      if (outcome.ok) {
        const step = session.steps.at(-1)
        // `end` is accepted without becoming a step: there is no action behind
        // it for the spectator to play, only the end event below.
        if (step && outcome.command !== "end") {
          feed?.publish({ type: "step", run: current.run, n: session.steps.length, step })
        }
      } else if (!wasDone) {
        feed?.publish({
          type: "refused",
          run: current.run,
          command: outcome.command,
          reason: outcome.reason,
        })
      }
      const head = outcome.ok
        ? `OK ${outcome.command}`
        : `REFUSED ${outcome.command}: ${outcome.reason}`
      keep(current)
      let tail = ""
      if (session.done && !wasDone) {
        close(current)
        tail = describeEnd(current)
      }
      return text(`${head}\n\n${session.observe(current.format)}${tail}`)
    }

    default:
      return text(`Unknown tool "${name}".`, true)
  }
}

/* ------------------------------------------------------------------ JSON-RPC */

type Request = { jsonrpc: "2.0"; id?: number | string | null; method: string; params?: unknown }

const SUPPORTED = ["2025-06-18", "2025-03-26", "2024-11-05"]

function respond(id: Request["id"], result: unknown): void {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`)
}

function fail(id: Request["id"], code: number, message: string): void {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, error: { code, message } })}\n`)
}

function handle(request: Request): void {
  const params = (request.params ?? {}) as Record<string, unknown>
  // A notification has no id and wants no answer, whatever it is.
  const notification = request.id === undefined
  switch (request.method) {
    case "initialize": {
      const asked = typeof params.protocolVersion === "string" ? params.protocolVersion : ""
      respond(request.id, {
        protocolVersion: SUPPORTED.includes(asked) ? asked : SUPPORTED[0],
        capabilities: { tools: {} },
        serverInfo: { name: "5wild-bench", version: String(CONTENT_VERSION) },
        instructions:
          "5 Wild, a roguelike word game, as a benchmark. Call `rules` once, then `new_run` " +
          "with a label naming yourself, then `act` one command at a time until the run ends.",
      })
      return
    }
    case "ping":
      respond(request.id, {})
      return
    case "tools/list":
      respond(request.id, { tools: TOOLS })
      return
    case "tools/call": {
      const name = typeof params.name === "string" ? params.name : ""
      const input = (params.arguments ?? {}) as Record<string, unknown>
      try {
        respond(request.id, callTool(name, input))
      } catch (error) {
        // A bug here, not a bad move: a bad move is a refusal, which is a
        // result. Said as a tool error so the harness can show it.
        log(error)
        respond(request.id, text(`Internal error: ${(error as Error).message}`, true))
      }
      return
    }
    default:
      if (!notification) fail(request.id, -32601, `Method not found: ${request.method}`)
  }
}

const input = createInterface({ input: process.stdin })
input.on("line", (line) => {
  if (!line.trim()) return
  let request: Request
  try {
    request = JSON.parse(line) as Request
  } catch {
    fail(null, -32700, "Parse error")
    return
  }
  handle(request)
})
input.on("close", () => {
  // The harness hung up. A suite run is already on disk and resumes on the
  // label's next `new_run`; an ad-hoc one has nowhere to resume from, so it is
  // filed as it stands.
  if (live && !live.session.done && live.suite === null) close(live)
  feed?.close()
  process.exit(0)
})

log(
  `suite "${suite.name}": ${suite.seeds.length} seeds at ascension ${suite.ascension}, ` +
    `${defaultFormat} observations, prompt ${stamp.promptHash}, commit ${stamp.commit}`,
)

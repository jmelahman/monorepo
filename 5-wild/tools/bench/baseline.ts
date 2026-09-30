/**
 * The blind bot, played through the benchmark's own door.
 *
 *   bun tools/bench/baseline.ts [--seeds 1..20] [--policy solver|farmer] [--watch]
 *
 * Two jobs. It is the reference line: a player with no judgement at all, only a
 * candidate filter and a fixed shopping list, so a model that reports below it
 * is losing to arithmetic. And it is the smoke test for everything but the
 * model: it goes through `Session` and `commandsFor` rather than calling
 * `reduce`, so a bug in how the host applies or scores a command shows up here
 * as a line that disagrees with `bun run sim` for the same seeds.
 *
 * `--watch` also serves the spectator feed and waits between moves, so the
 * browser at `?watch=http://localhost:7777` has something to show without an
 * agent session. Without it the whole suite runs in about a second.
 *
 * Episodes are filed under the label `baseline-<policy>` like any other.
 */

import { commandsFor } from "../../src/bench/commands"
import { promptHash } from "../../src/bench/rules"
import { summarize, table } from "../../src/bench/score"
import { DEFAULT_LIMITS, episodeOf, Session } from "../../src/bench/session"
import { CONTENT_VERSION } from "../../src/engine"
import type { Policy } from "../../test/helpers/blind"
import { blindPlayer } from "../../test/helpers/blind"
import { startFeed } from "./feed"
import { commit, loadSuite, writeEpisode } from "./host"
import { wordsFor } from "./words"

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}

const suite = loadSuite(flag("--suite"))
const policy: Policy = flag("--policy") === "farmer" ? "farmer" : "solver"
const watch = args.includes("--watch")
// A pause long enough for the spectator to show each move rather than queue a
// whole run behind the first animation. The spectator paces itself anyway; this
// only keeps the host from finishing minutes before the browser does.
const pause = Number(flag("--pause") ?? 1500)

function seedsFrom(raw: string | undefined): number[] {
  if (!raw) return suite.seeds
  const range = /^(\d+)\.\.(\d+)$/.exec(raw)
  if (range) {
    const [from, to] = [Number(range[1]), Number(range[2])]
    return Array.from({ length: to - from + 1 }, (_, i) => from + i)
  }
  return raw.split(",").map(Number)
}

const seeds = seedsFrom(flag("--seeds"))
const label = `baseline-${policy}`
// Filed as the suite's own run only when it plays the suite's seeds, so a
// `--seeds 1..3` smoke test does not leave the suite looking two-thirds done.
const suiteName = flag("--seeds") ? null : suite.name
const words = wordsFor("en")
const stamp = { commit: commit(), promptHash: promptHash() }
const feed = watch ? startFeed(Number(flag("--port") ?? 7777)) : null
const wait = (ms: number) => new Promise((done) => setTimeout(done, ms))

if (feed) {
  console.error(`open http://localhost:5173/?watch=${encodeURIComponent(feed.url)} and press enter`)
  await new Promise((done) => process.stdin.once("data", done))
}

const results = []
for (const [index, seed] of seeds.entries()) {
  const run = index + 1
  const started = Date.now()
  const session = new Session(seed, words, suite.ascension, {
    ...DEFAULT_LIMITS,
    endless: suite.endless,
  })
  const bot = blindPlayer(policy)
  feed?.publish({
    type: "start",
    run,
    label,
    seed,
    ascension: suite.ascension,
    lang: "en",
    content: CONTENT_VERSION,
  })
  while (!session.done) {
    const actions = bot.next(session.state, words)
    // The bot has nothing left to do, which the sim files as a death. Here it
    // is a stall, because that is what it is, and the report counts them.
    if (!actions) break
    for (const command of commandsFor(actions)) {
      const outcome = session.apply(command)
      if (!outcome.ok) {
        console.error(`seed ${seed}: the bot's "${outcome.command}" was refused: ${outcome.reason}`)
        break
      }
      const step = session.steps.at(-1)
      if (feed && step) {
        feed.publish({ type: "step", run, n: session.steps.length, step })
        await wait(step.type === "guess" ? pause * 2 : pause)
      }
      if (session.done) break
    }
  }
  session.abandon()
  const episode = episodeOf(session, {
    ...stamp,
    label,
    suite: suiteName,
    format: "text",
    wallMs: Date.now() - started,
  })
  writeEpisode(episode)
  feed?.publish({ type: "end", run, result: episode.result })
  results.push(episode.result)
  const r = episode.result
  console.error(`seed ${seed}: ${r.end}, ${r.roundsCleared} rounds, stage ${r.stage}`)
  if (feed) await wait(pause * 4)
}

console.log(table([{ label, summary: summarize(results) }]))
feed?.close()
process.exit(0)

/**
 * What real players did, replayed.
 *
 * `sim.test.ts` asks what the rules are like for a bot that cannot see the
 * answer; this asks what they were like for people, which is the question the
 * bots were standing in for. The runs arrive as replays (see
 * `src/ui/telemetry.ts`), so every number below is computed here, from the
 * engine in this checkout, rather than trusted from the client. That is also
 * the validation: a run the engine refuses at any step is counted as invalid
 * and dropped, whether it was forged, mangled, or played on rules this
 * checkout no longer has.
 *
 *   bunx wrangler d1 execute 5wild-telemetry --remote --json \
 *     --command "SELECT payload FROM runs" > .tmp/runs.json   (from telemetry/)
 *   RUNS=.tmp/runs.json bun run telemetry
 *
 * Only runs at the current `CONTENT_VERSION` are replayed. An older one is a
 * run on different numbers, and replaying it here would report the new rules'
 * verdict on the old player's choices. They are counted and set aside, and the
 * way to read them is to check out the commit they name.
 *
 * Without `RUNS` this is skipped, so `bun run test` never needs the network.
 */

import { readFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"
import type { RunState, WordSource } from "../src/engine"
import { CONTENT_VERSION, reduce, startRun } from "../src/engine"
import type { Payload } from "../src/ui/telemetry"
import { expand } from "../src/ui/telemetry"

const SOURCE = process.env.RUNS

const WORDS = resolve(dirname(fileURLToPath(import.meta.url)), "..", "public", "words")
const lists = new Map<string, WordSource>()
/**
 * Any of the four, not only the English one `realWords` pins: a run is dealt
 * from the list it was played in, and replaying a Spanish run against English
 * answers deals a different run that the log will not fit.
 */
function wordsFor(lang: string): WordSource {
  let words = lists.get(lang)
  if (!words) {
    const read = (name: string) =>
      readFileSync(join(WORDS, lang, `${name}.txt`), "utf8")
        .split("\n")
        .filter(Boolean)
    words = { answers: read("answers"), allowed: new Set(read("allowed")) }
    lists.set(lang, words)
  }
  return words
}

/**
 * Takes either what `wrangler d1 execute --json` prints, rows under `results`
 * with the payload as a string, or a plain array of payloads, which is what a
 * hand-assembled file or a test fixture will be.
 */
function load(path: string): Payload[] {
  const raw: unknown = JSON.parse(readFileSync(path, "utf8"))
  const rows: unknown[] = Array.isArray(raw)
    ? raw.flatMap((entry) =>
        entry && typeof entry === "object" && "results" in entry
          ? (entry as { results: unknown[] }).results
          : [entry],
      )
    : []
  return rows.map((row) =>
    row && typeof row === "object" && "payload" in row
      ? (JSON.parse(String((row as { payload: unknown }).payload)) as Payload)
      : (row as Payload),
  )
}

type Replayed = {
  run: Payload
  end: RunState
  /** Every relic the run held at any point, not only at the end: selling one is a verdict too. */
  relics: Set<string>
  /** Boss rounds played, and the one it died on if it did. */
  bosses: string[]
  killedBy: string | null
  /** Gold in hand on walking into each shop, by stage. */
  shopGold: Map<number, number>
  /** The first guess of every round: what players actually open with. */
  openers: string[]
}

function replayRun(run: Payload): Replayed | null {
  const words = wordsFor(run.words)
  let state = startRun(run.seed, words, run.ascension).state
  const relics = new Set<string>()
  const bosses: string[] = []
  const shopGold = new Map<number, number>()
  const openers: string[] = []
  let killedBy: string | null = null

  for (const step of run.steps) {
    if (step.type === "guess" && state.round.guesses.length === 0) openers.push(step.word)
    for (const action of expand([step])) {
      const before = state
      const { state: next, events } = reduce(state, action, words)
      if (events.some((event) => event.type === "rejected")) return null
      state = next
      for (const relic of state.relics) relics.add(relic.id)
      if (state.phase === "shop" && before.phase !== "shop") shopGold.set(state.stage, state.gold)
      if (state.round.bossId && state.round.done && !before.round.done) {
        bosses.push(state.round.bossId)
        if (state.phase === "game_over") killedBy = state.round.bossId
      }
    }
  }
  return { run, end: state, relics, bosses, killedBy, shopGold, openers }
}

const pct = (part: number, whole: number) => (whole ? `${((part / whole) * 100).toFixed(1)}%` : "-")

function median(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.floor(sorted.length / 2)] ?? 0
}

/** New players and veterans are different games; one win rate would average them into neither. */
const BUCKETS: [string, (nth: number) => boolean][] = [
  ["first run", (nth) => nth <= 1],
  ["runs 2-5", (nth) => nth >= 2 && nth <= 5],
  ["runs 6-20", (nth) => nth >= 6 && nth <= 20],
  ["runs 21+", (nth) => nth >= 21],
]

function report(all: Payload[], runs: Replayed[], invalid: number): string {
  const lines: string[] = []
  const stale = new Map<number, number>()
  for (const run of all) {
    if (run.content !== CONTENT_VERSION) stale.set(run.content, (stale.get(run.content) ?? 0) + 1)
  }
  const won = runs.filter((r) => r.end.won).length
  lines.push(
    `content ${CONTENT_VERSION}  ${runs.length} runs replayed` +
      (invalid ? `, ${invalid} refused by the engine and dropped` : ""),
  )
  for (const [version, count] of [...stale].sort((a, b) => a[0] - b[0])) {
    lines.push(`  set aside     ${count} at content ${version}`)
  }
  lines.push(
    `  won run       ${won} (${pct(won, runs.length)})`,
    `  median stage  ${median(runs.map((r) => r.end.stage))}`,
  )
  const ends = new Map<string, number>()
  for (const r of runs) ends.set(r.run.end, (ends.get(r.run.end) ?? 0) + 1)
  lines.push(`  ended         ${[...ends].map(([end, n]) => `${end} ${n}`).join(", ")}`)

  lines.push("  by experience")
  for (const [name, test] of BUCKETS) {
    const group = runs.filter((r) => test(r.run.nth))
    if (!group.length) continue
    const wins = group.filter((r) => r.end.won).length
    lines.push(
      `    ${name.padEnd(10)} ${String(group.length).padStart(5)} runs  won ${pct(wins, group.length).padStart(6)}  median stage ${median(group.map((r) => r.end.stage))}`,
    )
  }

  lines.push("  by ascension")
  const levels = [...new Set(runs.map((r) => r.run.ascension))].sort((a, b) => a - b)
  for (const level of levels) {
    const group = runs.filter((r) => r.run.ascension === level)
    const wins = group.filter((r) => r.end.won).length
    lines.push(
      `    A${String(level).padEnd(3)} ${String(group.length).padStart(5)} runs  won ${pct(wins, group.length).padStart(6)}`,
    )
  }

  // Lost runs only, and the same stage.round key as the sim's histogram, so the
  // two can be read side by side: where the bots die and where people do.
  const deaths = new Map<string, number>()
  for (const r of runs) {
    if (r.end.phase !== "game_over") continue
    const at = `${r.end.stage}.${r.end.roundIndex + 1}`
    deaths.set(at, (deaths.get(at) ?? 0) + 1)
  }
  const order = (at: string) => Number(at.split(".")[0]) * 10 + Number(at.split(".")[1])
  lines.push("  died on stage.round")
  for (const at of [...deaths.keys()].sort((a, b) => order(a) - order(b))) {
    const count = deaths.get(at) ?? 0
    lines.push(`    ${at.padEnd(5)} ${"#".repeat(Math.min(count, 60))} ${count}`)
  }

  // Win rate among runs that held it against the rate among runs that did not.
  // A relic that wins because good players buy it and a relic that wins because
  // it is strong look the same here; the experience split above is the check.
  lines.push("  relics (runs held, won when held, won when not)")
  const relicIds = [...new Set(runs.flatMap((r) => [...r.relics]))]
  const rows = relicIds.map((id) => {
    const held = runs.filter((r) => r.relics.has(id))
    const not = runs.length - held.length
    return {
      id,
      held: held.length,
      with: held.filter((r) => r.end.won).length / held.length,
      without: not ? (won - held.filter((r) => r.end.won).length) / not : 0,
    }
  })
  for (const row of rows.sort((a, b) => b.held - a.held)) {
    lines.push(
      `    ${row.id.padEnd(18)} ${String(row.held).padStart(5)}  ${(row.with * 100).toFixed(1).padStart(5)}%  ${(row.without * 100).toFixed(1).padStart(5)}%`,
    )
  }

  lines.push("  bosses (faced, killed the run)")
  const faced = new Map<string, number>()
  const killed = new Map<string, number>()
  for (const r of runs) {
    for (const boss of r.bosses) faced.set(boss, (faced.get(boss) ?? 0) + 1)
    if (r.killedBy) killed.set(r.killedBy, (killed.get(r.killedBy) ?? 0) + 1)
  }
  for (const [boss, count] of [...faced].sort((a, b) => b[1] - a[1])) {
    const deadly = killed.get(boss) ?? 0
    lines.push(
      `    ${boss.padEnd(18)} ${String(count).padStart(5)}  ${pct(deadly, count).padStart(6)}`,
    )
  }

  lines.push("  median gold on entering the shop, by stage")
  const stages = [...new Set(runs.flatMap((r) => [...r.shopGold.keys()]))].sort((a, b) => a - b)
  for (const stage of stages) {
    const golds = runs.flatMap((r) => (r.shopGold.has(stage) ? [r.shopGold.get(stage) ?? 0] : []))
    lines.push(`    ${String(stage).padStart(2)}  $${median(golds)}  (${golds.length} runs)`)
  }

  const openers = new Map<string, number>()
  for (const r of runs)
    for (const word of r.openers) openers.set(word, (openers.get(word) ?? 0) + 1)
  const total = [...openers.values()].reduce((a, b) => a + b, 0)
  lines.push("  top openers")
  for (const [word, count] of [...openers].sort((a, b) => b[1] - a[1]).slice(0, 10)) {
    lines.push(`    ${word.toUpperCase()}  ${pct(count, total)}`)
  }
  return lines.join("\n")
}

describe.skipIf(!SOURCE)("telemetry report", () => {
  it("replays every run it was given", () => {
    const all = load(SOURCE ?? "")
    const current = all.filter((run) => run.content === CONTENT_VERSION)
    const replayed = current.map(replayRun)
    const runs = replayed.filter((r): r is Replayed => r !== null)
    process.stdout.write(`\n${report(all, runs, replayed.length - runs.length)}\n`)
    expect(all.length).toBeGreaterThan(0)
  })
})

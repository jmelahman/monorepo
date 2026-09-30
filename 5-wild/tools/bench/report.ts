/**
 * Every label's results, side by side.
 *
 *   bun tools/bench/report.ts [path...] [--suite <path>] [--json <file>] [--csv <file>]
 *
 * The paths are anything `loadEpisodes` reads: bundles from `bench:export`, a
 * results directory, a folder of bundles gathered from several worktrees. With
 * none, this checkout's `bench/results`.
 *
 * Only episodes the suite dealt, at the suite's ascension, on the current
 * prompt hash. The last one matters most: a reworded rule or a rebalanced relic
 * is a different benchmark, and a table that mixed the two would compare models
 * across games. What was left out is counted underneath, so a label that
 * vanished from the table did not vanish silently.
 *
 * Every kept episode is replayed first (`verify`), because results gathered
 * from other machines are claims until they are, and because a worktree on a
 * different engine with the same prompt hash is exactly the drift the hash
 * cannot see. The same run arriving twice, from a results directory and from
 * the bundle made of it, is one run. The same label and seed arriving with two
 * different plays is a label reused on two machines, and neither is kept: there
 * is no telling which one the table should believe.
 *
 * `--json` writes the summaries with every run beneath them, and `--csv` one
 * row per run, which is the shape a plotting tool wants.
 */

import { writeFileSync } from "node:fs"
import { promptHash } from "../../src/bench/rules"
import type { Summary } from "../../src/bench/score"
import { summarize, table } from "../../src/bench/score"
import type { Episode } from "../../src/bench/session"
import { verify } from "../../src/bench/verify"
import { CONTENT_VERSION } from "../../src/engine"
import { loadEpisodes } from "./bundle"
import { loadSuite, RESULTS } from "./host"
import { wordsFor } from "./words"

const args = process.argv.slice(2)
const flag = (name: string) => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const suite = loadSuite(flag("--suite"))
const paths = args.filter((arg, i) => !arg.startsWith("--") && !args[i - 1]?.startsWith("--"))
const hash = promptHash()
const words = wordsFor("en")

const { episodes, unreadable } = loadEpisodes(paths.length ? paths : [RESULTS])

const skipped = { adhoc: 0, otherSuite: 0, otherPrompt: 0, duplicate: 0 }
const failed: string[] = []
const conflicts: string[] = []

// label -> seed -> the one play of it, or null once two plays disagree.
const byLabel = new Map<string, Map<number, Episode | null>>()
for (const episode of episodes) {
  const why =
    episode.suite === null
      ? "adhoc"
      : episode.suite !== suite.name || episode.ascension !== suite.ascension
        ? "otherSuite"
        : episode.promptHash !== hash
          ? "otherPrompt"
          : null
  if (why) {
    skipped[why]++
    continue
  }
  const seeds = byLabel.get(episode.label) ?? new Map<number, Episode | null>()
  byLabel.set(episode.label, seeds)
  if (!seeds.has(episode.seed)) {
    seeds.set(episode.seed, episode)
    continue
  }
  const held = seeds.get(episode.seed)
  if (held && JSON.stringify(held.steps) === JSON.stringify(episode.steps)) {
    // The same play claiming a different result: at most one of them is true,
    // and the one held is replayed below, so only the newcomer needs it here.
    // Dropped as a duplicate unexamined, an edited copy would pass unremarked.
    const verdict =
      JSON.stringify(held.result) === JSON.stringify(episode.result) ? null : verify(episode, words)
    if (verdict && !verdict.ok) failed.push(`${episode.label} seed ${episode.seed}: ${verdict.why}`)
    else skipped.duplicate++
  } else if (held) {
    conflicts.push(`${episode.label} seed ${episode.seed}`)
    seeds.set(episode.seed, null)
  }
}

type Row = { label: string; summary: Summary; commits: string[]; runs: Episode[] }
const rows: Row[] = []
const incomplete: string[] = []
for (const [label, seeds] of byLabel) {
  const runs: Episode[] = []
  for (const episode of seeds.values()) {
    if (!episode) continue
    const verdict = verify(episode, words)
    if (verdict.ok) runs.push(episode)
    else failed.push(`${label} seed ${episode.seed}: ${verdict.why}`)
  }
  if (!runs.length) continue
  runs.sort((a, b) => suite.seeds.indexOf(a.seed) - suite.seeds.indexOf(b.seed))
  if (runs.length < suite.seeds.length)
    incomplete.push(`${label} (${runs.length}/${suite.seeds.length})`)
  rows.push({
    label,
    summary: summarize(runs.map((episode) => episode.result)),
    commits: [...new Set(runs.map((episode) => episode.commit))].sort(),
    runs,
  })
}
rows.sort((a, b) => b.summary.meanCleared - a.summary.meanCleared)

if (!rows.length) {
  console.log(`No results for suite "${suite.name}" on prompt ${hash}.`)
} else {
  console.log(`suite "${suite.name}", ascension ${suite.ascension}, prompt ${hash}\n`)
  console.log(table(rows))
}
const notes = [
  skipped.adhoc && `${skipped.adhoc} ad-hoc runs`,
  skipped.otherSuite && `${skipped.otherSuite} from another suite or ascension`,
  skipped.otherPrompt && `${skipped.otherPrompt} on another prompt or content version`,
  skipped.duplicate && `${skipped.duplicate} duplicates`,
].filter(Boolean)
if (notes.length) console.log(`\nleft out: ${notes.join(", ")}`)
if (incomplete.length) console.log(`incomplete: ${incomplete.join(", ")}`)
const mixed = rows.filter((row) => row.commits.length > 1)
if (mixed.length)
  console.log(`played on several commits: ${mixed.map((row) => row.label).join(", ")}`)
for (const conflict of conflicts)
  console.log(`CONFLICT, two different plays of ${conflict}; both left out`)
for (const failure of failed) console.log(`DOES NOT REPLAY, left out: ${failure}`)
for (const path of unreadable) console.log(`unreadable: ${path}`)

const jsonPath = flag("--json")
if (jsonPath) {
  const report = {
    suite: { name: suite.name, seeds: suite.seeds, ascension: suite.ascension },
    content: CONTENT_VERSION,
    promptHash: hash,
    labels: rows.map((row) => ({
      label: row.label,
      commits: row.commits,
      complete: row.runs.length === suite.seeds.length,
      summary: row.summary,
      runs: row.runs.map((episode) => ({
        seed: episode.seed,
        ...episode.result,
        wallMs: episode.wallMs,
      })),
    })),
  }
  writeFileSync(jsonPath, `${JSON.stringify(report, null, 2)}\n`)
  console.log(`\nwrote ${jsonPath}`)
}

const csvPath = flag("--csv")
if (csvPath) {
  const columns = [
    "label",
    "seed",
    "ascension",
    "end",
    "won",
    "stage",
    "round",
    "roundsCleared",
    "roundsSolved",
    "guesses",
    "score",
    "steps",
    "refusals",
    "wallMs",
    "commit",
  ] as const
  const cell = (value: unknown) => {
    const text = String(value)
    return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
  }
  const lines = rows.flatMap((row) =>
    row.runs.map((episode) => {
      const record: Record<(typeof columns)[number], unknown> = {
        label: row.label,
        seed: episode.seed,
        ascension: episode.ascension,
        ...episode.result,
        wallMs: episode.wallMs,
        commit: episode.commit,
      }
      return columns.map((column) => cell(record[column])).join(",")
    }),
  )
  writeFileSync(csvPath, `${[columns.join(","), ...lines].join("\n")}\n`)
  console.log(`${jsonPath ? "" : "\n"}wrote ${csvPath}`)
}

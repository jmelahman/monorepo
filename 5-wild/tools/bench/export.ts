/**
 * One file per label, to carry out of wherever the label was played.
 *
 *   bun tools/bench/export.ts [label...] [--out bench/exports] [--suite <path>]
 *
 * With no labels, every label that has played this suite. Only the suite's own
 * runs, at its ascension, on this checkout's prompt hash: a bundle is one
 * label on one benchmark, and anything else it held would only be filtered out
 * again by the report at the other end. Every episode is replayed before it is
 * written, so a bundle never carries a run this engine would not reproduce;
 * one that fails is left out and named, rather than exported for the report to
 * find later on another machine.
 */

import { mkdirSync, readdirSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { promptHash } from "../../src/bench/rules"
import { summarize } from "../../src/bench/score"
import { verify } from "../../src/bench/verify"
import { CONTENT_VERSION } from "../../src/engine"
import type { Bundle } from "./bundle"
import { BUNDLE_KIND, BUNDLE_VERSION, loadEpisodes } from "./bundle"
import { loadSuite, RESULTS, ROOT, slug } from "./host"
import { wordsFor } from "./words"

const args = process.argv.slice(2)
const flag = (name: string) => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const suite = loadSuite(flag("--suite"))
const out = flag("--out") ?? join(ROOT, "bench", "exports")
const named = args.filter((arg, i) => !arg.startsWith("--") && !args[i - 1]?.startsWith("--"))
const hash = promptHash()
const words = wordsFor("en")

const dirs = named.length
  ? named.map(slug)
  : readdirSync(RESULTS, { withFileTypes: true })
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name)

mkdirSync(out, { recursive: true })
let written = 0
for (const dir of dirs) {
  // The label directory only, not `adhoc/` beneath it: those never count.
  const { episodes } = loadEpisodes(
    readdirSync(join(RESULTS, dir))
      .filter((name) => name.endsWith(".json") && name !== "in-progress.json")
      .map((name) => join(RESULTS, dir, name)),
  )
  const mine = episodes.filter(
    (episode) =>
      episode.suite === suite.name &&
      episode.ascension === suite.ascension &&
      episode.promptHash === hash,
  )
  const kept = mine.filter((episode) => {
    const verdict = verify(episode, words)
    if (!verdict.ok) console.error(`${dir}: seed ${episode.seed} left out, ${verdict.why}`)
    return verdict.ok
  })
  if (!kept.length) {
    console.error(`${dir}: nothing on suite "${suite.name}" at prompt ${hash}; no bundle`)
    continue
  }
  kept.sort((a, b) => suite.seeds.indexOf(a.seed) - suite.seeds.indexOf(b.seed))
  const label = kept[0]?.label ?? dir
  const bundle: Bundle = {
    kind: BUNDLE_KIND,
    v: BUNDLE_VERSION,
    label,
    suite: {
      name: suite.name,
      seeds: suite.seeds,
      ascension: suite.ascension,
      endless: suite.endless,
    },
    content: CONTENT_VERSION,
    promptHash: hash,
    commits: [...new Set(kept.map((episode) => episode.commit))].sort(),
    exported: new Date().toISOString(),
    summary: summarize(kept.map((episode) => episode.result)),
    episodes: kept,
  }
  const path = join(out, `${slug(label)}.json`)
  writeFileSync(path, `${JSON.stringify(bundle)}\n`)
  written++
  const missing = suite.seeds.length - kept.length
  console.log(
    `${label}: ${kept.length}/${suite.seeds.length} runs -> ${path}` +
      (missing ? `  (incomplete: ${missing} to play)` : ""),
  )
}
if (!written) process.exitCode = 1

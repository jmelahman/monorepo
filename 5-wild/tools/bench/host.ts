/**
 * What the MCP host and the baseline share: where episodes go, which seeds a
 * suite deals, and what a finished run is stamped with.
 *
 * Episodes are filed by label and by seed, one file each, and that layout is
 * the suite's cursor. The next seed a label plays is the first one it has no
 * file for, so a harness that crashes at seed 12 resumes at seed 12 on its next
 * `new_run`, and a label can never quietly play one seed twice and keep the
 * better result.
 */

import { execSync } from "node:child_process"
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import type { Format } from "../../src/bench/observe"
import type { Episode } from "../../src/bench/session"
import type { Step } from "../../src/ui/telemetry"

export const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..")
export const RESULTS = join(ROOT, "bench", "results")

export type Suite = {
  name: string
  seeds: number[]
  ascension: number
  /** Whether a won run plays on into the endless stages. Off by default: see `Limits`. */
  endless: boolean
  format: Format
}

export function loadSuite(path = join(ROOT, "bench", "suite.json")): Suite {
  const raw = JSON.parse(readFileSync(path, "utf8")) as Partial<Suite>
  if (!Array.isArray(raw.seeds) || raw.seeds.length === 0) {
    throw new Error(`${path}: a suite needs a non-empty "seeds" list`)
  }
  return {
    name: raw.name ?? "default",
    seeds: raw.seeds,
    ascension: raw.ascension ?? 0,
    endless: raw.endless ?? false,
    format: raw.format === "json" ? "json" : "text",
  }
}

/**
 * A label as a directory name. Labels are free text a harness types
 * ("claude-opus-5-5 via claude code, text"), so anything that would not
 * survive a path is folded to a dash.
 *
 * Dots survive for version numbers ("sonnet-4.5"), and so leading ones are
 * trimmed: a label of `..` was a slug of `..`, and filed its episodes in
 * `bench/` beside `results/` rather than in it, outside the ignore.
 */
export const slug = (label: string): string =>
  label
    .toLowerCase()
    .replace(/[^a-z0-9.]+/g, "-")
    .replace(/^[-.]+|-+$/g, "") || "unlabelled"

/**
 * A seed asked for by name goes under `adhoc/`, apart from the suite's files,
 * and never counts as the suite having dealt it. Otherwise a label could play a
 * suite seed again by name and overwrite the result it did not like.
 */
const fileFor = (label: string, seed: number, ascension: number, adhoc = false) =>
  join(RESULTS, slug(label), ...(adhoc ? ["adhoc"] : []), `seed-${seed}-a${ascension}.json`)

export const played = (label: string, seed: number, ascension: number): boolean =>
  existsSync(fileFor(label, seed, ascension))

export function nextSeed(suite: Suite, label: string): number | null {
  return suite.seeds.find((seed) => !played(label, seed, suite.ascension)) ?? null
}

export function writeEpisode(episode: Episode): string {
  const adhoc = episode.suite === null
  const path = adhoc
    ? fileFor(episode.label, episode.seed, episode.ascension, true).replace(
        /\.json$/,
        `-${Date.now()}.json`,
      )
    : fileFor(episode.label, episode.seed, episode.ascension)
  mkdirSync(dirname(path), { recursive: true })
  writeFileSync(path, `${JSON.stringify(episode, null, 2)}\n`)
  return path
}

export function readEpisodes(dir: string): Episode[] {
  if (!existsSync(dir)) return []
  return readdirSync(dir, { recursive: true, encoding: "utf8" })
    .filter((name) => name.endsWith(".json") && !name.endsWith(LIVE))
    .map((name) => JSON.parse(readFileSync(join(dir, name), "utf8")) as Episode)
}

/**
 * The commit a result was measured on, with `-dirty` when the tree was not
 * clean, since a benchmark run on uncommitted balance is a number nobody can
 * reproduce, and it should say so on its face.
 */
export function commit(): string {
  try {
    const head = execSync("git rev-parse --short HEAD", { cwd: ROOT, encoding: "utf8" }).trim()
    const dirty = execSync("git status --porcelain --untracked-files=no", {
      cwd: ROOT,
      encoding: "utf8",
    }).trim()
    return dirty ? `${head}-dirty` : head
  } catch {
    return "unknown"
  }
}

/* ------------------------------------------------------------ a live run */

/**
 * A suite run in progress, rewritten after every command.
 *
 * A harness that loses its connection mid-run (a context reset, a restarted
 * server, a laptop lid) has not finished the run, and filing it as a stall
 * would score the harness's plumbing as the model's play. Leaving it unfiled is
 * worse: the seed would be dealt again, and a label could escape any deal it
 * disliked by hanging up. So the run is kept here, and the label's next
 * `new_run` picks it up exactly where it stood, replayed from the seed, which
 * is the only way back into a run this repo has and the one every save and
 * replay already trusts.
 */
export type LiveRun = {
  seed: number
  ascension: number
  format: Format
  steps: Step[]
  refusals: number
  /**
   * The rest of `Session.checkpoint()`. Absent from a checkpoint written before
   * them, which resumes with no streak: the old file kept only the total, and
   * nothing in it says how many of those refusals came in a row.
   */
  streak?: number
  lastRefusal?: string | null
  started: number
}

const LIVE = "in-progress.json"
const liveFile = (label: string) => join(RESULTS, slug(label), LIVE)

export function saveLive(label: string, run: LiveRun): void {
  const path = liveFile(label)
  mkdirSync(dirname(path), { recursive: true })
  writeFileSync(path, `${JSON.stringify(run)}\n`)
}

export function loadLive(label: string): LiveRun | null {
  const path = liveFile(label)
  if (!existsSync(path)) return null
  try {
    return JSON.parse(readFileSync(path, "utf8")) as LiveRun
  } catch {
    return null
  }
}

export function clearLive(label: string): void {
  rmSync(liveFile(label), { force: true })
}

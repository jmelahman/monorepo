/**
 * Results that travel: one file per label, and the reader that takes them back.
 *
 * `bench/results` is the right shape for a host writing as it plays, one file
 * per run, and the wrong one to carry out of a worktree or a devcontainer: a
 * directory is not something a person attaches, and twenty files that must
 * travel together will one day travel as nineteen. A bundle is the same
 * episodes, whole and replayable, under a header saying which benchmark they
 * were played on.
 *
 * The reader does not care what it is handed. A bundle, a single episode, a
 * label's directory, a whole `bench/results`, or a folder of bundles collected
 * from several machines all come back as one list of episodes, so the report has
 * one input however the results were gathered.
 */

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs"
import { join } from "node:path"
import type { Summary } from "../../src/bench/score"
import type { Episode } from "../../src/bench/session"
import type { Suite } from "./host"

export const BUNDLE_KIND = "5wild-bench-bundle"
export const BUNDLE_VERSION = 1

export type Bundle = {
  kind: typeof BUNDLE_KIND
  v: number
  label: string
  suite: Pick<Suite, "name" | "seeds" | "ascension" | "endless">
  content: number
  promptHash: string
  /** Every commit the episodes were played on; more than one is worth a second look. */
  commits: string[]
  exported: string
  /** For a reader without the repo. The report recomputes it from `episodes`. */
  summary: Summary
  episodes: Episode[]
}

const isBundle = (value: unknown): value is Bundle =>
  typeof value === "object" &&
  value !== null &&
  (value as { kind?: unknown }).kind === BUNDLE_KIND &&
  Array.isArray((value as { episodes?: unknown }).episodes)

const isEpisode = (value: unknown): value is Episode =>
  typeof value === "object" &&
  value !== null &&
  Array.isArray((value as { steps?: unknown }).steps) &&
  typeof (value as { seed?: unknown }).seed === "number" &&
  typeof (value as { result?: unknown }).result === "object"

export type Loaded = { episodes: Episode[]; unreadable: string[] }

/** Every episode under any number of paths, bundles and loose files alike. */
export function loadEpisodes(paths: readonly string[]): Loaded {
  const episodes: Episode[] = []
  const unreadable: string[] = []
  const file = (path: string) => {
    let value: unknown
    try {
      value = JSON.parse(readFileSync(path, "utf8"))
    } catch {
      unreadable.push(path)
      return
    }
    if (isBundle(value)) episodes.push(...value.episodes)
    else if (isEpisode(value)) episodes.push(value)
    else unreadable.push(path)
  }
  const walk = (path: string) => {
    if (!existsSync(path)) {
      unreadable.push(path)
      return
    }
    if (!statSync(path).isDirectory()) {
      file(path)
      return
    }
    for (const name of readdirSync(path, { recursive: true, encoding: "utf8" }).sort()) {
      // A run still being played is not a result, and has no `result` to read.
      if (name.endsWith(".json") && !name.endsWith("in-progress.json")) file(join(path, name))
    }
  }
  for (const path of paths) walk(path)
  return { episodes, unreadable }
}

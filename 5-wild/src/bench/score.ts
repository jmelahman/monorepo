/**
 * Many runs, one line.
 *
 * Rounds cleared is the headline because it is the only number here that means
 * the same thing on every seed: one more round survived. Win rate saturates at
 * zero for weak models and at one for strong ones, and score grows
 * geometrically with the stage, so its mean is set by whichever run went
 * furthest. Both are reported, but neither is what a comparison should sort on.
 *
 * The interval is a bootstrap over seeds, with its own fixed stream, so the same
 * set of results always prints the same bounds. With twenty seeds it is wide,
 * and that width is the finding: two models whose intervals overlap have not
 * been separated by this suite.
 */

import type { Result } from "./session"

export type Summary = {
  runs: number
  won: number
  meanCleared: number
  /** 95% bootstrap interval on `meanCleared`. */
  low: number
  high: number
  medianStage: number
  medianScore: number
  solveRate: number
  refusalsPerRun: number
  stalled: number
}

const mean = (xs: readonly number[]) =>
  xs.length ? xs.reduce((total, x) => total + x, 0) / xs.length : 0

const median = (xs: readonly number[]) => {
  const sorted = [...xs].sort((a, b) => a - b)
  return sorted.length ? (sorted[Math.floor(sorted.length / 2)] ?? 0) : 0
}

/** A small seeded stream, so the interval does not move between two prints of one file. */
function lcg(seed: number): () => number {
  let state = seed >>> 0
  return () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0
    return state / 2 ** 32
  }
}

function bootstrap(xs: readonly number[], rounds = 2000): [number, number] {
  if (xs.length === 0) return [0, 0]
  const next = lcg(0x5eed)
  const means: number[] = []
  for (let r = 0; r < rounds; r++) {
    let total = 0
    for (let i = 0; i < xs.length; i++) total += xs[Math.floor(next() * xs.length)] ?? 0
    means.push(total / xs.length)
  }
  means.sort((a, b) => a - b)
  return [means[Math.floor(rounds * 0.025)] ?? 0, means[Math.floor(rounds * 0.975)] ?? 0]
}

export function summarize(results: readonly Result[]): Summary {
  // Sorted before resampling, so the interval depends on the set of results
  // and not on the order a directory listing happened to return them in.
  const cleared = results.map((result) => result.roundsCleared).sort((a, b) => a - b)
  const [low, high] = bootstrap(cleared)
  const totalCleared = cleared.reduce((total, x) => total + x, 0)
  return {
    runs: results.length,
    won: results.filter((result) => result.won).length,
    meanCleared: mean(cleared),
    low,
    high,
    medianStage: median(results.map((result) => result.stage)),
    medianScore: median(results.map((result) => result.score)),
    solveRate: totalCleared
      ? results.reduce((total, result) => total + result.roundsSolved, 0) / totalCleared
      : 0,
    refusalsPerRun: mean(results.map((result) => result.refusals)),
    stalled: results.filter((result) => result.end === "stalled").length,
  }
}

/** One row per label, sorted on the headline, with the interval beside it. */
export function table(rows: ReadonlyArray<{ label: string; summary: Summary }>): string {
  const header = [
    "label",
    "runs",
    "cleared (95% CI)",
    "won",
    "med stage",
    "med score",
    "solved",
    "refusals",
    "stalled",
  ]
  const body = [...rows]
    .sort((a, b) => b.summary.meanCleared - a.summary.meanCleared)
    .map(({ label, summary: s }) => [
      label,
      String(s.runs),
      `${s.meanCleared.toFixed(2)} (${s.low.toFixed(2)}–${s.high.toFixed(2)})`,
      `${s.won}/${s.runs}`,
      String(s.medianStage),
      Math.round(s.medianScore).toLocaleString("en-US"),
      `${(s.solveRate * 100).toFixed(0)}%`,
      s.refusalsPerRun.toFixed(1),
      String(s.stalled),
    ])
  const widths = header.map((cell, i) =>
    Math.max(cell.length, ...body.map((row) => (row[i] ?? "").length)),
  )
  const line = (row: readonly string[]) =>
    row
      .map((cell, i) => cell.padEnd(widths[i] ?? 0))
      .join("  ")
      .trimEnd()
  return [line(header), line(widths.map((w) => "-".repeat(w))), ...body.map(line)].join("\n")
}

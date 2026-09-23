/**
 * How far a player who does not know the answer actually gets.
 *
 * The golden vectors pin what the rules *compute*; this asks what they are
 * *like* to play, which is a different question and until now an unasked one.
 * Every scenario that writes a vector types `state.round.answer`, so no test in
 * this repo has ever exercised a run that had to find the word, and "is stage 6
 * too steep" is exactly a question about finding the word.
 *
 * Two modes, on the same shape as `golden.test.ts`:
 *
 *   npm test        a dozen seeds, asserted loosely: a smoke alarm for a
 *                   balance edit that made the game unwinnable or trivial
 *   npm run sim     a few hundred seeds, printed as a report
 *
 * The assertions are deliberately a wide band rather than a number. A tight one
 * would be a balance expectation pinned in a test nobody thinks of as a balance
 * test, which is the failure mode CLAUDE.md warns about. A shelf reshuffle
 * would break it on seeds nobody was thinking about, and the repair would be to
 * renumber it, which teaches nothing.
 */

import { describe, expect, it } from "vitest"
import type { RunState, WordSource } from "../src/engine"
import { reduce, STAGES, startRun } from "../src/engine"
import type { Policy } from "./helpers/blind"
import { blindPlayer, playerView } from "./helpers/blind"
import { realWords } from "./helpers/words"

/**
 * `SIM` switches the report on, `SIM_SEEDS` says how wide. Two variables rather
 * than one because `SIM=1` reads as "on" and would otherwise mean "one seed",
 * which is the one width whose answer is guaranteed to be noise.
 */
const SEEDS = process.env.SIM ? Number(process.env.SIM_SEEDS) || 300 : 12

/** A run cannot need more than this many actions unless something is looping. */
const CAP = 6000

type Outcome = {
  seed: number
  /** The stage the run died on, or `STAGES` + 1 if it never did. */
  stage: number
  round: number
  won: boolean
  guesses: number
  /**
   * Rounds banked, and how many of those were banked by finding the word.
   *
   * Two numbers rather than one because `reduce.ts:220` fails a round on
   * `score < target`. Finding the word is not what clears it at ascension
   * zero, reaching the number is. So a player can bank a round it never solved,
   * and the gap between these two columns is the income line actually being
   * played rather than described.
   */
  cleared: number
  solved: number
}

function play(seed: number, policy: Policy, words: WordSource): Outcome {
  const player = blindPlayer(policy)
  let state: RunState = startRun(seed, words, 0).state
  let guesses = 0
  let cleared = 0
  let solved = 0
  let stage = state.stage
  let round = state.roundIndex

  for (let step = 0; step < CAP; step++) {
    if (state.phase === "game_over" || state.phase === "victory") break
    const batch = player.next(state, words)
    if (!batch || batch.length === 0) break
    for (const action of batch) {
      const before = state.round.guesses.length
      const wasRewarding = state.phase === "reward"
      const next = reduce(state, action, words).state
      if (next.round.guesses.length > before) guesses++
      // A round is banked when the phase turns to reward, the same edge
      // `replay` watches for. Read before the action, because clearing a round
      // swaps the round out from under the question.
      if (!wasRewarding && next.phase === "reward") {
        cleared++
        if (next.round.solved) solved++
      }
      state = next
    }
    if (state.phase === "round") {
      stage = state.stage
      round = state.roundIndex
    }
  }

  return {
    seed,
    stage: state.phase === "victory" || state.won ? STAGES + 1 : stage,
    round,
    won: state.phase === "victory" || Boolean(state.won),
    guesses,
    cleared,
    solved,
  }
}

const median = (xs: readonly number[]) => {
  const sorted = [...xs].sort((a, b) => a - b)
  return sorted[Math.floor(sorted.length / 2)] ?? 0
}

function report(policy: Policy, runs: readonly Outcome[]): string {
  const won = runs.filter((run) => run.won).length
  const cleared = runs.reduce((total, run) => total + run.cleared, 0)
  const solved = runs.reduce((total, run) => total + run.solved, 0)
  const guesses = runs.reduce((total, run) => total + run.guesses, 0)
  const per = Math.max(1, cleared)
  const lines = [
    `${policy}  ${runs.length} runs`,
    `  won run       ${won} (${((won / runs.length) * 100).toFixed(1)}%)`,
    `  median stage  ${median(runs.map((run) => run.stage))}`,
    `  rounds banked ${cleared}`,
    `  by solving    ${solved} (${((solved / per) * 100).toFixed(1)}%)`,
    `  guesses/round ${(guesses / per).toFixed(2)}`,
  ]
  // Stage *and* round: "died on stage 1" and "died on the first round of the
  // game" are different findings, and the histogram that cannot tell them apart
  // is the one that sends you to look at the wrong curve.
  const deaths = new Map<string, number>()
  for (const run of runs) {
    if (run.won) continue
    const at = `${run.stage}.${run.round + 1}`
    deaths.set(at, (deaths.get(at) ?? 0) + 1)
  }
  const order = (at: string) => Number(at.split(".")[0]) * 10 + Number(at.split(".")[1])
  lines.push("  died on stage.round")
  for (const at of [...deaths.keys()].sort((a, b) => order(a) - order(b))) {
    const count = deaths.get(at) ?? 0
    lines.push(`    ${at}  ${"#".repeat(count)} ${count}`)
  }
  return lines.join("\n")
}

describe("blind simulation", () => {
  const seeds = Array.from({ length: SEEDS }, (_, i) => i * 7 + 1)
  const played = new Map<Policy, Outcome[]>()
  for (const policy of ["solver", "farmer"] as const) {
    played.set(
      policy,
      seeds.map((seed) => play(seed, policy, realWords)),
    )
  }

  if (process.env.SIM) {
    for (const [policy, runs] of played) process.stdout.write(`\n${report(policy, runs)}\n`)
  }

  it("hides the answer from the player", () => {
    const state = startRun(1, realWords, 0).state
    expect(state.round.answer).toHaveLength(5)
    expect("answer" in playerView(state).round).toBe(false)
  })

  for (const policy of ["solver", "farmer"] as const) {
    const runs = () => played.get(policy) ?? []

    /*
     * The band, not the number. Clearing more than one round per run is a floor
     * so far under either policy's real figure that only a broken rule reaches
     * it: a word class refused, feedback that stopped meaning what it says, a
     * target curve off by an order of magnitude. It is deliberately not a
     * measurement of the curve, because a test that measured the curve would
     * have to be renumbered every time the curve moved on purpose.
     */
    it(`${policy} banks rounds`, () => {
      const cleared = runs().reduce((sum, run) => sum + run.cleared, 0)
      expect(cleared / runs().length).toBeGreaterThan(1)
    })

    it(`${policy} neither walks it nor is walled out`, () => {
      const won = runs().filter((run) => run.won).length
      expect(won).toBeLessThan(runs().length)
      expect(median(runs().map((run) => run.stage))).toBeGreaterThan(1)
    })
  }
})

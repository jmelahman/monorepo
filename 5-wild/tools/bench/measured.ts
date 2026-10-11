/**
 * What a round cost and how long it took, for the labels somebody measured.
 *
 * Neither number is in an episode. An episode's `wallMs` runs from the deal to
 * the last move, and a run that sat out a plan's rate limit for an hour in the
 * middle has the hour in it: Opus at ascension 10 reads 247 seconds a round
 * that way, against 34 with the waits taken out. And no episode knows what it
 * cost, because the host never sees a token.
 *
 * So these came out of the harness logs in `bench/logs/`, by hand, once per
 * label: cost is what the harness reported for the five runs (list price for
 * the plans that bill flat), seconds is the gaps between log lines with any
 * gap over five minutes dropped as a wait, and both are divided by rounds
 * played. They are keyed `<results directory>@a<ascension>`.
 *
 * A label absent from here gets no cost bar, and its speed is the median over
 * its runs of `wallMs` per round, which the one interrupted run in five does
 * not move. `charts.ts` says which labels it fell back for.
 */
export type Measured = { cents?: number; seconds?: number }

export const MEASURED: Record<string, Measured> = {
  "claude-opus-5-5-claude-code@a10": { cents: 20.6, seconds: 34 },
  "claude-sonnet-5-5-claude-code@a10": { cents: 13.5, seconds: 17 },
  "openai-gpt-6-sol-openrouter@a10": { cents: 9.1, seconds: 20 },
  "openai-gpt-6.1-sol-openrouter@a10": { cents: 4.3, seconds: 19 },
  "z-ai-glm-5.3-flash-openrouter@a10": { cents: 4.6, seconds: 121 },
  "deepseek-deepseek-v4.1-flash-openrouter@a10": { cents: 3.1, seconds: 105 },
  "google-gemini-3.8-flash-openrouter@a10": { cents: 7.3, seconds: 55 },
  "google-gemini-3.8-flash-openrouter@a0": { cents: 8.4, seconds: 32 },
  "gpt-6-luna-opencode-go@a0": { cents: 0.6, seconds: 41 },
  "claude-haiku-5-5-opencode-go@a0": { cents: 0.4, seconds: 20 },
  // The two free previews: no `cents`, since a 0¢ bar would read as a price and
  // it is a promotion. Step 5 logged 30,452 active seconds over 67 rounds and
  // Nemotron 13,095 over 47.
  "step-5-preview-free-opencode-go@a0": { seconds: 455 },
  "nvidia-nemotron-3-ultra-550b-a55b-free-openrouter@a0": { seconds: 279 },
}

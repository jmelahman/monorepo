/**
 * One benchmarked run: the engine, a command at a time.
 *
 * Everything that makes two benchmark numbers comparable is decided here and
 * nowhere else, so the MCP host, the baseline and the tests are all driving the
 * same object: what a command costs, when a run is over, and what it scored.
 *
 * A command is applied whole or not at all. "guess crane" is six actions to
 * the engine, and a letter the keyboard refuses halfway through would otherwise
 * leave a half-typed draft that the next command has to discover. The state
 * before the command is kept and restored instead, so the only thing a refusal
 * leaves behind is the sentence saying why.
 */

import type { GameEvent, RunState, WordSource } from "../engine"
import { CONTENT_VERSION, reduce, startRun } from "../engine"
import { refusalText } from "../ui/lang"
import type { Step } from "../ui/telemetry"
import { expand } from "../ui/telemetry"
import type { Command } from "./commands"
import { formatCommand, parseCommand } from "./commands"
import type { Format } from "./observe"
import { observe } from "./observe"

/**
 * How a run can stop.
 *
 * `victory` is a run that cleared the last authored stage in a suite that stops
 * there; `retired` is the same run in an endless suite, banked by the model's
 * own `end`. `stalled` is the harness giving up rather than the game ending: a
 * model that cannot produce a legal move, or one that never stops producing
 * moves, and it is reported apart from `died` because the two are different
 * findings about a model, even though both score the stage they stopped on.
 */
export type End = "died" | "victory" | "retired" | "stalled"

export type Result = {
  end: End
  /** Cleared the last authored stage, whatever happened after. */
  won: boolean
  /** The stage the run stopped on. One past `STAGES` for a run that stopped at the win. */
  stage: number
  /** The round within it, counted from 1. */
  round: number
  /** Rounds banked, the headline number: it rises by one for every round survived. */
  roundsCleared: number
  /** Rounds banked by finding the word, which ascension 10 alone insists on. */
  roundsSolved: number
  guesses: number
  /** The sum of every banked round's total. Grows geometrically, so read its median. */
  score: number
  /** Commands accepted. */
  steps: number
  /** Commands refused, whether the parser or the engine said no. */
  refusals: number
}

export type Limits = {
  /**
   * Accepted commands before the run is called stalled. A full eight-stage run
   * takes a few hundred; this is ten times that, so only a loop reaches it.
   */
  maxSteps: number
  /**
   * Refusals in a row before the run is called stalled. Enough to let a model
   * read a refusal, misread it and try again several times, and few enough that
   * a model which cannot make progress does not spend an hour proving it.
   */
  maxRefusals: number
  /** Whether a won run may play on into the endless stages, or stops at the win. */
  endless: boolean
}

export const DEFAULT_LIMITS: Limits = { maxSteps: 3000, maxRefusals: 25, endless: false }

export type Outcome =
  | { ok: true; command: string; events: GameEvent[] }
  | { ok: false; command: string; reason: string }

export class Session {
  state: RunState
  readonly steps: Step[] = []
  refusals = 0
  /** Refusals since the last accepted command. */
  private streak = 0
  /** The last refusal, until the next command lands. Shown to the model once, then dropped. */
  lastRefusal: string | null = null
  private ended: End | null = null
  private cleared = 0
  private solved = 0
  private guesses = 0
  private banked = 0

  constructor(
    readonly seed: number,
    readonly words: WordSource,
    readonly ascension = 0,
    readonly limits: Limits = DEFAULT_LIMITS,
  ) {
    this.state = startRun(seed, words, ascension).state
  }

  get done(): boolean {
    return this.ended !== null
  }

  observe(format: Format): string {
    return observe(this.state, format, { refusal: this.lastRefusal, over: this.done })
  }

  /** Parse and apply one line. Never throws on anything a model can type. */
  act(raw: string): Outcome {
    if (this.ended) return this.refuse(raw.trim(), "The run is over.")
    const parsed = parseCommand(raw)
    if (!parsed.ok) return this.refuse(raw.trim(), parsed.error)
    return this.apply(parsed.command)
  }

  /** Apply an already-parsed command; the baseline speaks in these directly. */
  apply(command: Command): Outcome {
    const said = formatCommand(command)
    if (this.ended) return this.refuse(said, "The run is over.")

    if (command.type === "end") {
      if (this.state.phase !== "victory") {
        return this.refuse(said, "You can only end the run at the victory screen.")
      }
      this.accept()
      this.ended = "retired"
      return { ok: true, command: said, events: [] }
    }

    const before = this.state
    let state = before
    const events: GameEvent[] = []
    for (const action of expand([command])) {
      const result = reduce(state, action, this.words)
      const refusal = result.events.find((event) => event.type === "rejected")
      if (refusal) return this.refuse(said, refusalText(refusal.refusal))
      state = result.state
      events.push(...result.events)
    }

    this.state = state
    this.steps.push(command)
    this.tally(before, state)
    this.accept()

    if (state.phase === "game_over") this.ended = "died"
    else if (state.phase === "victory" && !this.limits.endless) this.ended = "victory"
    else if (this.steps.length >= this.limits.maxSteps) this.ended = "stalled"
    return { ok: true, command: said, events }
  }

  result(): Result {
    const state = this.state
    const won = Boolean(state.won) || state.phase === "victory"
    // The stage a run *reached*: a run that died on stage 4 reached 4, and one
    // that stopped at the win is counted one past the end, the same convention
    // `sim.test.ts` uses, so the two reports can sit beside each other.
    const atWin = state.phase === "victory"
    return {
      end: this.ended ?? "stalled",
      won,
      stage: atWin ? state.stage + 1 : state.stage,
      round: atWin ? 1 : state.roundIndex + 1,
      roundsCleared: this.cleared,
      roundsSolved: this.solved,
      guesses: this.guesses,
      score: this.banked,
      steps: this.steps.length,
      refusals: this.refusals,
    }
  }

  /** Stop the run where it stands, as a stall. For a harness that walked away. */
  abandon(): void {
    this.ended ??= "stalled"
  }

  private accept(): void {
    this.streak = 0
    this.lastRefusal = null
  }

  private refuse(command: string, reason: string): Outcome {
    this.refusals++
    this.streak++
    this.lastRefusal = `"${command}": ${reason}`
    if (this.streak >= this.limits.maxRefusals) this.ended ??= "stalled"
    return { ok: false, command, reason }
  }

  /**
   * The same edges `sim.test.ts` watches, plus the one it misses: the last
   * round of the last stage goes straight to `victory` rather than through
   * `reward`, and a win that did not count its own final round would report the
   * best runs one round short.
   */
  private tally(before: RunState, after: RunState): void {
    if (after.round.guesses.length > before.round.guesses.length) this.guesses++
    const banked =
      before.phase === "round" && (after.phase === "reward" || after.phase === "victory")
    if (banked) {
      this.cleared++
      if (after.round.solved) this.solved++
      this.banked += after.round.score
    }
  }
}

/* --------------------------------------------------------------- episodes */

export const EPISODE_VERSION = 1

/**
 * A finished run, on disk. `steps` is telemetry's `Step[]`, so the episode is
 * a replay: `replayEpisode` below, or the spectator, turns it back into the
 * whole run from the seed. Everything else is what makes two episodes
 * comparable, or says why they are not.
 */
export type Episode = {
  v: number
  content: number
  commit: string
  promptHash: string
  label: string
  /** The suite this run was dealt by, or null for a seed a harness asked for by name. */
  suite: string | null
  format: Format
  seed: number
  ascension: number
  endless: boolean
  lang: "en"
  steps: Step[]
  result: Result
  wallMs: number
}

export function episodeOf(
  session: Session,
  meta: {
    commit: string
    promptHash: string
    label: string
    suite: string | null
    format: Format
    wallMs: number
  },
): Episode {
  return {
    v: EPISODE_VERSION,
    content: CONTENT_VERSION,
    commit: meta.commit,
    promptHash: meta.promptHash,
    label: meta.label,
    suite: meta.suite,
    format: meta.format,
    seed: session.seed,
    ascension: session.ascension,
    endless: session.limits.endless,
    lang: "en",
    steps: [...session.steps],
    result: session.result(),
    wallMs: meta.wallMs,
  }
}

/**
 * Re-run an episode from its seed. Returns the end state, or the index of the
 * first step the engine refused, which means the episode was recorded against
 * other rules or other words than these.
 */
export function replayEpisode(
  episode: Pick<Episode, "seed" | "ascension" | "steps">,
  words: WordSource,
): { ok: true; state: RunState } | { ok: false; step: number } {
  let state = startRun(episode.seed, words, episode.ascension).state
  for (const [index, step] of episode.steps.entries()) {
    for (const action of expand([step])) {
      const result = reduce(state, action, words)
      if (result.events.some((event) => event.type === "rejected"))
        return { ok: false, step: index }
      state = result.state
    }
  }
  return { ok: true, state }
}

/**
 * The shape of a golden vector, and the machinery that produces one.
 *
 * A vector is `{ seed, contentVersion, actions[] }` in, `{ guesses[], gold[],
 * outcome }` out. It is the portability contract: any implementation of these
 * rules, a rewrite or a port to another language, is correct if and only if it
 * reproduces these numbers from these actions.
 */

import type {
  Action,
  GameEvent,
  GuessNote,
  Phase,
  Refusal,
  RewardBreakdown,
  RunState,
  WordSource,
} from "../../src/engine"
import { reduce, startRun } from "../../src/engine"
import { en } from "../../src/ui/lang/en"
import type { Scenario } from "./scenarios"
import { placeMod } from "./scenarios"

/** One scored guess, with the round it belonged to so a failure can be placed. */
export type GuessVector = {
  stage: number
  round: number
  /**
   * Who this guess was scored against, on the one round in three that has
   * someone. Recorded because the draw is a rule like any other and it was the
   * last one nothing wrote down: `bossForStage` is a shuffle of a band keyed by
   * seed, so a port that shuffled differently would meet a different boss, and
   * five of the fifteen would score identically anyway. The Fog and The Mirror
   * touch only `shown`; The Tyrant, The Glutton and The Purist only refuse words
   * that were never played. None of the three columns a guess records can tell
   * those five apart. This can.
   *
   * On the guess rather than on the round because that is where the diff wants
   * it: a boss that starts biting shows up as a changed score on a line that
   * says whose it was, instead of a changed score and a hunt for the round.
   * Omitted on the other two rounds in three, so the field costs nothing where
   * there is no boss to name.
   */
  boss?: string
  word: string
  chips: number
  mult: number
  solveBonus: number
  score: number
  /**
   * What the boss told the player instead of showing it. Recorded because it is
   * a rule's output that a player can act on, so a reimplementation that counted
   * it wrong would be wrong in a way no score here would catch. Omitted when
   * there is none, which is every guess under fourteen of the fifteen bosses.
   * The Silence is still the only one that says anything.
   *
   * The engine now hands over `{ code, count }` and the sentence is the English
   * catalog's; this renders it back to the string the file has always held,
   * because a vector that re-recorded here would be claiming the refactor moved
   * something, and it did not. What a port has to reproduce is the count. It is
   * pinned to `en` rather than read through the live catalog for the same reason
   * `test/helpers/words.ts` is pinned to the English lists: the baseline must not
   * move when a setting does.
   */
  note?: string
}

export type GoldVector = { delta: number; reason: string }

export type Vector = {
  name: string
  covers: string
  seed: number
  /**
   * The difficulty the run was recorded at, omitted for the ordinary game. Part
   * of the input rather than of the expectation: an ascension changes which
   * words the engine will accept and which answers it will deal, so a replay
   * that started at the wrong level would not be replaying the same run.
   */
  ascension?: number
  contentVersion: number
  actions: Action[]
  expected: {
    guesses: GuessVector[]
    gold: GoldVector[]
    /** Itemized per round cleared. The gold event only carries the total. */
    rewards: RewardBreakdown[]
    outcome: Phase
    stage: number
    round: number
    finalGold: number
    relics: string[]
    /** "A+2": letter and the chips its etching adds. */
    etched: string[]
    /** "e:steel": letter and the modifier stuck to it. */
    mods: string[]
    /** "distinct:3": word category and the level it was bought up to. */
    levels: string[]
    /** "range_ae:2": alphabet slice and the level it was bought up to. */
    ranges: string[]
    /** "snowball:mult=310": what each growing relic has banked. */
    grown: string[]
    destroyed: string[]
  }
}

/** Everything a vector asserts, read off a finished run. */
function summarize(
  state: RunState,
  guesses: GuessVector[],
  gold: GoldVector[],
  rewards: RewardBreakdown[],
): Vector["expected"] {
  const letters = Object.entries(state.letters)
  return {
    guesses,
    gold,
    rewards,
    outcome: state.phase,
    stage: state.stage,
    round: state.roundIndex,
    finalGold: state.gold,
    relics: state.relics.map((relic) => relic.id),
    etched: letters
      .filter(([, value]) => value.etch > 0)
      .map(([letter, value]) => `${letter}+${value.etch}`)
      .sort(),
    mods: letters
      .filter(([, value]) => value.mod !== null)
      .map(([letter, value]) => `${letter}:${value.mod}`)
      .sort(),
    // The third permanent upgrade line, recorded beside the other two: a
    // leveling bug that happens not to move a score is still a bug, and this
    // is what makes the diff say which levels a run actually bought.
    levels: Object.entries(state.levels ?? {})
      .map(([id, level]) => `${id}:${level}`)
      .sort(),
    // The other leveled line, recorded beside its twin. Kept separate from
    // `etched` even though both end up as chips on a letter, because the diff
    // has to be able to say *which* upgrade a run bought. The two crosscut, so
    // a total alone could not tell a leveled range from an etched group.
    ranges: Object.entries(state.ranges ?? {})
      .map(([id, level]) => `${id}:${level}`)
      .sort(),
    // The fourth line that outlives a round. Recorded for the same reason as
    // `levels`: a relic that banks the wrong amount is a bug even on the runs
    // where the wrong amount happens to score the same, and this is the only
    // place the number is ever written down.
    grown: state.relics
      .flatMap((relic) =>
        Object.entries(relic.data ?? {}).map(([key, value]) => `${relic.id}:${key}=${value}`),
      )
      .sort(),
    destroyed: letters
      .filter(([, value]) => value.destroyed)
      .map(([letter]) => letter)
      .sort(),
  }
}

/**
 * An action the engine turned down, and where in the list it was.
 *
 * Never expected to be non-empty: every action in a vector was accepted at the
 * moment it was recorded. It is kept out of `expected` for exactly that reason:
 * an always-empty array in the JSON would be re-recorded as an always-empty
 * array, so a rule that started refusing something would launder itself through
 * the next `npm run golden`. Asserted as a property instead, which re-recording
 * cannot quietly agree with.
 */
export type RefusedAction = { index: number; action: Action; refusal: Refusal }

export type Replayed = { expected: Vector["expected"]; refused: RefusedAction[] }

/**
 * Replays a recorded action list. This is what the test runs, and it never
 * consults a scenario. The JSON is the input, so the bots that wrote it can
 * change freely without moving the baseline.
 *
 * Note what a refusal does here: `reduce` returns the state untouched and emits
 * a `rejected` event, so a replay that hits one does not stop, it silently plays
 * a shorter run. That is the asymmetry these vectors live with. A rule that got
 * *stricter* is caught, since the guess never lands and the score list comes up
 * short, but a rule that got *looser* is invisible, because every action recorded here
 * was already legal and permission cannot be tested by exercising it. The
 * refusals are collected so the first half at least fails by name.
 */
export function replay(
  seed: number,
  actions: readonly Action[],
  words: WordSource,
  ascension = 0,
): Replayed {
  let state = startRun(seed, words, ascension).state
  const guesses: GuessVector[] = []
  const gold: GoldVector[] = []
  const rewards: RewardBreakdown[] = []
  const refused: RefusedAction[] = []

  actions.forEach((action, index) => {
    // The round is read *before* the action, because clearing one swaps the
    // round out and a guess scored on the way belongs to the round it was
    // played in, not the one that replaced it.
    const stage = state.stage
    const round = state.roundIndex
    const boss = state.round.bossId
    const before = state.round.guesses.length
    const wasRewarding = state.phase === "reward"

    const result = reduce(state, action, words)
    state = result.state
    collectGold(result.events, gold)
    for (const event of result.events) {
      if (event.type === "rejected") refused.push({ index, action, refusal: event.refusal })
    }
    if (!wasRewarding && state.phase === "reward" && state.reward) rewards.push(state.reward)

    const scored = state.round.guesses[before]
    if (state.round.guesses.length > before && scored) {
      guesses.push({
        stage,
        round,
        ...(boss ? { boss } : {}),
        word: scored.word,
        chips: scored.chips,
        mult: scored.mult,
        solveBonus: scored.solveBonus,
        score: scored.score,
        ...(scored.note ? { note: noteText(scored.note) } : {}),
      })
    }
  })

  return { expected: summarize(state, guesses, gold, rewards), refused }
}

/**
 * The `string` arm is a note out of a save written before the field was
 * structured, which a replay can never produce — it always starts from
 * `startRun`. It is narrowed rather than cast because the field's type carries
 * it, and a cast here would be a claim about saves that this file is in no
 * position to make.
 */
const noteText = (note: GuessNote | string): string =>
  typeof note === "string" ? note : en.event.guessNote(note)

function collectGold(events: readonly GameEvent[], into: GoldVector[]): void {
  for (const event of events) {
    if (event.type === "gold") into.push({ delta: event.delta, reason: event.reason })
  }
}

/**
 * Plays a scenario and records what it did. The action cap is a recorder
 * guard, not a rule: a bot that stops producing actions ends the vector, and
 * one that never stops would otherwise write a vector the size of the repo.
 */
export function record(scenario: Scenario, words: WordSource, contentVersion: number): Vector {
  const ascension = scenario.ascension ?? 0
  let state = startRun(scenario.seed, words, ascension).state
  const actions: Action[] = []

  for (let step = 0; step < 4000; step++) {
    // Only defeat ends a recording outright. Victory used to as well, which
    // made `continue_run` the one action in the game no vector could contain:
    // the recorder stopped at the exact screen the action is offered on. Every
    // bot that does not want to play on returns null there anyway, so letting
    // the scenario answer costs nothing and buys the other half of the ending.
    if (state.phase === "game_over") break
    // A modifier in hand is not a decision a bot may defer: the shop refuses
    // every other action until it is placed. Handled here rather than in each
    // scenario because it is forced, and because a bot that bought one by
    // accident would otherwise stall against a shop it cannot leave.
    const batch = state.placing ? placeMod(state) : scenario.next(state, words)
    if (!batch || batch.length === 0) break
    for (const action of batch) {
      actions.push(action)
      state = reduce(state, action, words).state
    }
  }

  return {
    name: scenario.name,
    covers: scenario.covers,
    seed: scenario.seed,
    // Written only when there is one, so every vector recorded before ascensions
    // existed re-records byte for byte.
    ...(ascension > 0 ? { ascension } : {}),
    contentVersion,
    actions,
    expected: replay(scenario.seed, actions, words, ascension).expected,
  }
}

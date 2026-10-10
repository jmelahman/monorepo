import { describe, expect, it } from "vitest"
import type { Action, RunState } from "../../src/engine"
import { getBoss, reduce, roundTargets, STAGES, startRun } from "../../src/engine"
import { placeMod } from "../golden/scenarios"
import { realWords } from "../helpers/words"

const words = realWords

const apply = (state: RunState, actions: Action[]): RunState =>
  actions.reduce((current, action) => reduce(current, action, words).state, state)

const type = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** Common, vowel-heavy, and distinct, so at least one survives any boss's rule. */
const OPENERS = ["arose", "crane", "audio", "slate", "adieu", "irate"]

/**
 * The round after the first opener it will take that is not the answer, or null.
 *
 * The six openers first, then any answer still typeable. The fallback is for
 * Pyromaniac, which burns a letter a round: by stage 4 a run holding it can
 * have broken a letter in every one of the six, and seeds 12 and 14 got there
 * once Carbon Copy and Second Wind traded rarities and reshuffled the shelf,
 * 14 and 20 rounds solved on the first guess. A bot that cannot type its
 * opener is not a curve gone easy, so it reaches for another word rather than
 * the test learning to excuse it.
 */
function open(state: RunState): RunState | null {
  const live = (word: string) => [...word].every((letter) => !state.letters[letter]?.destroyed)
  const spare = words.answers.filter((word) => !OPENERS.includes(word) && live(word))
  for (const word of [...OPENERS, ...spare.slice(0, 20)]) {
    if (word === state.round.answer) continue
    const next = apply(state, type(word))
    if (next.round.guesses.length === 1 && next.phase === "round") return next
  }
  return null
}

/**
 * A bot that plays the greedy-solve line: one opener, then the answer, for the
 * largest solve bonus a player could honestly have earned, then spend
 * everything on the first thing it can afford. It is not a good player, since
 * it never farms chips, but it is a complete one, and it proves the loop runs
 * start to finish without a screen attached.
 *
 * It used to play the answer on the first guess. That stopped measuring
 * anything once a first-guess solve began clearing its round whatever it
 * scored: a bot that reads the answer won every seed, which is a fact about
 * the bot and not the curve. The opener is what any player who cannot see the
 * word has to spend, so this is still the greedy line, one guess later.
 */
function playRun(seed: number): {
  state: RunState
  roundsCleared: number
  illegal: string[]
  misplacedBosses: string[]
  unopened: number
} {
  let state = startRun(seed, words).state
  let roundsCleared = 0
  const illegal: string[] = []
  const misplacedBosses: string[] = []
  let unopened = 0

  // 24 rounds is a full win; the cap is a runaway guard, not an expectation.
  for (let step = 0; step < 200; step++) {
    if (state.phase === "game_over" || state.phase === "victory") break

    if (state.phase === "round") {
      if ((state.round.bossId !== null) !== (state.roundIndex === 2)) {
        misplacedBosses.push(
          `stage ${state.stage} round ${state.roundIndex}: ${state.round.bossId ?? "none"}`,
        )
      }
      const refusal = getBoss(state.round.bossId)?.validate?.(state.round.answer, state.round)
      if (refusal) {
        illegal.push(`${state.round.bossId}/${state.round.answer}: ${refusal}`)
        break
      }
      if (state.round.guesses.length === 0) {
        // The first opener nothing refuses and that is not the answer. A boss or
        // a broken letter can refuse all of them, which is rare enough that
        // going straight to the answer is the honest fallback.
        const opened = open(state)
        if (opened) {
          state = opened
          continue
        }
        // Recorded rather than silent: the fallback is a first-guess solve, which
        // clears the round for nothing, and a bot leaning on it would read as a
        // curve gone easy.
        unopened++
      }
      state = apply(state, type(state.round.answer))
      continue
    }
    if (state.phase === "reward") {
      roundsCleared++
      state = apply(state, [{ type: "collect" }])
      continue
    }
    if (state.phase === "shop") {
      // Forced, not strategy: an unaimed modifier holds the shop until it has a
      // letter, `next_round` included, so a bot that never places one sits here
      // until the cap. This one did not place for as long as modifiers have been
      // sold unaimed and passed on luck, since none of the seeds below ever met
      // one as the first thing on a shelf it could afford. Gold coming down from
      // $9 to $6 put one in seed 1's first shop, with $7 in hand.
      if (state.placing) {
        const placed = placeMod(state)
        if (!placed) break
        state = apply(state, placed)
        continue
      }
      const index = state.shop?.items.findIndex((item) => item && item.cost <= state.gold) ?? -1
      if (index < 0) {
        state = apply(state, [{ type: "next_round" }])
        continue
      }
      // A buy can be refused, by full relic slots or full card slots, and a bot
      // that cannot tell that from a purchase will shop forever.
      const attempt = reduce(state, { type: "buy", index }, words)
      const refused = attempt.events.some((event) => event.type === "rejected")
      state = refused ? apply(state, [{ type: "next_round" }]) : attempt.state
    }
  }

  return { state, roundsCleared, illegal, misplacedBosses, unopened }
}

describe("a full run, headless", () => {
  it("always terminates in a decided state", () => {
    for (const seed of [1, 7, 42, 1234, 99999]) {
      const { state } = playRun(seed)
      expect(["game_over", "victory"]).toContain(state.phase)
    }
  })

  /*
   * The round must always be winnable. The Glutton demands two vowels of every
   * guess and a fifth of the answer list has one, so an unfiltered draw hands
   * the player a word they are forbidden to type: a round that cannot be
   * solved by any play. Pyromaniac makes this worse: breaking enough letters can
   * empty the pool, and the escape hatch that heals the alphabet must not also
   * drop the boss rule on its way out.
   */
  it("never draws an answer its own boss forbids", () => {
    const illegal = Array.from({ length: 120 }, (_, seed) => playRun(seed + 1).illegal).flat()
    expect(illegal).toEqual([])
  })

  /*
   * The stage's shape is Normal → Elite → Boss, and the boss rule is a large part
   * of a round's difficulty. A boss leaking onto round 0 or 1, or missing from
   * round 2, silently rewrites the difficulty curve without failing anything
   * else, so the placement is asserted rather than assumed.
   */
  it("puts a boss on the third round of an stage and nowhere else", () => {
    const misplaced = Array.from({ length: 40 }, (_, seed) => playRun(seed + 1).misplacedBosses)
    expect(misplaced.flat()).toEqual([])
  })

  it("never leaves the state un-serializable, whatever it accumulated", () => {
    const { state } = playRun(2024)
    expect(JSON.parse(JSON.stringify(state))).toEqual(state)
  })

  it("is reproducible from the seed alone", () => {
    expect(playRun(31337).state).toEqual(playRun(31337).state)
  })

  /*
   * Not a balance assertion but a tripwire, and one that reads the spread rather
   * than a handful of seeds.
   *
   * It used to name five seeds and demand every one of them fall short of the
   * full 24. That held by luck: the greedy line wins about one seed in ten, and
   * none of the five happened to be one. Moving Snowball between rarity pools
   * reshuffled every shelf and flipped seed 1 into the winning tenth, which
   * failed the test without anything about the difficulty having moved: the
   * measured win rate went 10.0% to 10.3% across 300 runs.
   *
   * So the bound is now on the distribution, which is what was always meant.
   * The floor catches a curve gone brutal; the ceiling catches scoring gone
   * trivial. A greedy bot that never farms chips should sometimes win and
   * mostly not, and if either half of that stops being true someone should
   * know.
   */
  it("gets a greedy solver a few rounds in, and only sometimes all the way", () => {
    const runs = Array.from({ length: 20 }, (_, index) => playRun(index + 1))
    // Every round opened honestly, so no clear here was bought by the fallback.
    expect(runs.map((run) => run.unopened)).toEqual(runs.map(() => 0))
    const cleared = runs.map((run) => run.roundsCleared)
    for (const count of cleared) expect(count).toBeGreaterThanOrEqual(2)
    const won = cleared.filter((count) => count >= 24).length
    expect(won).toBeLessThan(cleared.length / 2)
  })

  it("has a final target far beyond any single unbuilt guess", () => {
    // Sanity on the curve itself: stage 8 must require a real build, not one
    // lucky word. A bare 5-letter solve tops out in the low thousands.
    expect(roundTargets(STAGES)[2]).toBeGreaterThan(100_000)
  })
})

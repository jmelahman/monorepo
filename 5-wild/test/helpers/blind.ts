/**
 * A player that does not know the answer.
 *
 * Every bot in `test/golden/scenarios.ts` reads `state.round.answer` (line 79,
 * line 90, line 132) and is right to: a recorder wants a run that reaches a
 * particular rule, and the shortest path there is to type the word. That makes
 * them useless for the other question. "Is the stage 6 target fair" is a question
 * about somebody who has to *find* the word, and nothing that reads the answer
 * can answer it.
 *
 * So this one is handed a `PlayerView`, which is the run state with `answer`
 * destructured off it. Not a convention: the field is genuinely absent from the
 * object at runtime, so a policy that tried to cheat would read `undefined`
 * rather than the word, and `test/sim.test.ts` asserts the absence. The point of
 * a blind player is the claim that it played blind, and a claim enforced by
 * discipline is worth what discipline is worth.
 *
 * It is deliberately *not* pure the way a scenario is. A scenario has to be, or
 * the recorder could not reproduce its action list; this thing records nothing,
 * so it is free to carry the one piece of state that makes it affordable, the
 * shrinking candidate pool. Re-deriving that from scratch on every guess is what
 * took the first draft from seconds to minutes.
 *
 * The failure mode to watch for is the one that cost this file its credibility
 * once already: a policy that runs out of legal moves returns null, `next`
 * returns null, and the caller stops the run and files it as a death. Nothing
 * goes red, no test fails, and every number the harness reports comes out low.
 * Two of those have been found and both are repaired below, in `narrow` and at
 * the end of `guess`, each with what it was worth. Before adding a rule that can
 * refuse a guess, ask what this player does when it refuses them all, because
 * the answer is not visible from the report.
 */

import type { PlayerView } from "../../src/bench/observe"
import { playerView } from "../../src/bench/observe"
import { INTEREST_CAP, INTEREST_PER } from "../../src/content/rounds"
import type { Action, Color, RunState, ShopItem, WordSource } from "../../src/engine"
import {
  baseChips,
  computeFeedback,
  difficultyOf,
  LETTER_CHIPS,
  reduce,
  startRun,
} from "../../src/engine"
import { CATEGORY_BY_ID } from "../../src/engine/categories"
import { placeMod } from "../golden/scenarios"

/**
 * The run as a player sees it, now owned by `src/bench/observe.ts`: the
 * benchmark makes the same claim this player does, that it played without the
 * word, and one claim should rest on one piece of code.
 */
export type { PlayerView } from "../../src/bench/observe"
export { playerView } from "../../src/bench/observe"

/**
 * How a player spends the guess budget. The whole tension the game is built on
 * is which of these two a round wants, so the simulator's job is to price both
 * rather than to pick one.
 *
 * Everything *else* the two do, shopping, packs, where a modifier goes, is
 * shared code below, on purpose. If the shop policies differed, a gap between
 * the two lines would not be attributable to the line.
 */
export type Policy =
  /** Guess to find the word. Fewest guesses spent, largest solve bonus. */
  | "solver"
  /** Burn the early guesses on chips, then solve. More tiles scored, smaller bonus. */
  | "farmer"
  /** Guess like the solver, but commit to a build and shop and steer for it. */
  | "builder"

/**
 * How many guesses the farmer keeps in hand to actually find the word with.
 *
 * Four, which leaves it two to farm, and four because the income line gets
 * worse the harder it is played. Over 60 seeds at v21, median stage reached:
 * reserve 2 → 1, reserve 3 → 3, reserve 4 → 5, against the solver's 5.
 *
 * Median saturates there, so it is the wrong column to read at the top end: at
 * reserve 4 the farmer ties the solver on it. Over 300 seeds the run win rate
 * separates them and the ordering holds: 7.3% for the solver against 3.3% for
 * this farmer, which is as close as the income line ever gets.
 *
 * That ordering is not a quirk of this policy, it is the scoring rule showing
 * through. Mult comes from colors and colors come from being right, so a
 * guess thrown at chips alone scores its tiles at ×1 and buys nothing toward
 * the next one. Income is worth taking when a guess you wanted anyway happens to
 * be rich; it does not survive being made the plan. The constant is set to the
 * kindest version of the strategy on purpose, since a strawman farmer would make the
 * comparison say nothing.
 */
const FARMER_RESERVE = 4

/**
 * Openers are ranked once and reused. The first guess of every round faces the
 * same untouched candidate pool, so the ranking cannot change, only which of
 * the top few a boss will accept, and what the run's etchings have done to the
 * chip tie-break.
 */
const OPENERS = 40

/**
 * How deep into the allowed list the farmer will look for chips. Wide enough
 * that there is still something rich left after the untried-letter filter has
 * taken the z- and q-words away, which happens by about the third guess.
 */
const INCOME = 400

/**
 * How close to the most informative guess still counts as just as informative.
 * The knob that decides how much of a chip premium a player will pay for a word
 * that narrows the pool slightly less well.
 */
const BAND = 0.9

const twin = <T>(a: readonly T[], b: readonly T[]) =>
  a.length === b.length && a.every((x, i) => x === b[i])

/** Chips a word is worth from the letter table alone, before the run touched it. */
const tableChips = (word: string) =>
  [...word].reduce((total, letter) => total + (LETTER_CHIPS[letter] ?? 0), 0)

/**
 * How much a guess would tell us, scored as the pool-frequency of its distinct
 * letters. A real solver would measure the expected split of the pool, which is
 * quadratic in its size and would dominate the runtime of the whole simulator;
 * this agrees with it on the opening word and disagrees only in the endgame,
 * where the pool is small enough that it rarely costs a guess.
 */
function information(word: string, freq: Map<string, number>): number {
  let total = 0
  for (const letter of new Set(word)) total += freq.get(letter) ?? 0
  return total
}

function frequencies(pool: readonly string[]): Map<string, number> {
  const freq = new Map<string, number>()
  for (const word of pool) {
    for (const letter of new Set(word)) freq.set(letter, (freq.get(letter) ?? 0) + 1)
  }
  return freq
}

/** Every action the engine accepted, dry-run. The twin of the one in scenarios.ts. */
function accepted(state: RunState, words: WordSource, actions: readonly Action[]): boolean {
  let current = state
  for (const action of actions) {
    const result = reduce(current, action, words)
    if (result.events.some((event) => event.type === "rejected")) return false
    current = result.state
  }
  return true
}

const typeWord = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/**
 * The first candidate the engine will actually take. A boss refuses whole
 * classes of word and a destroyed letter cannot be typed at all, so a player
 * that assumed its guess landed would stall on a round it can never submit to.
 */
function playable(
  state: RunState,
  words: WordSource,
  candidates: readonly string[],
): Action[] | null {
  for (const word of candidates) {
    const actions = typeWord(word)
    if (accepted(state, words, actions)) return actions
  }
  return null
}

/** What a shop item is worth taking first, high to low. */
function appeal(item: ShopItem): number {
  if (item.kind === "relic") return 5
  if (item.kind === "mod") return 4
  if (item.kind === "pack") return 3
  if (item.kind === "etch") return 2
  return 1
}

/**
 * The builds a player can commit to, as the builder policy reads them: the
 * relics that belong to each, and the word-shape category that feeds it, if
 * one does.
 *
 * Bot-side on purpose. The engine has no notion of an archetype and should not
 * grow one to serve a harness; a card's build is a player's reading of it, and
 * this is one player's: the archetypes the relics were written for, with the
 * shapes split out, since a player committed to Cluster does not want a level
 * of Twinned.
 *
 * Habit, Collector, Patron, Indelible and the rest are in no build: they pay
 * whatever the tray is doing, so buying one says nothing about what to buy next.
 */
const BUILDS: readonly { id: string; relics: readonly string[]; category?: string }[] = [
  { id: "green", relics: ["green_thumb", "keystone", "first_impression", "no_maybes", "snowball"] },
  { id: "yellow", relics: ["bloodhound", "scavenger"] },
  { id: "gray", relics: ["masochist", "greedy_grammarian"] },
  { id: "vowel", relics: ["vowel_hoarder", "head_start", "chorus"], category: "vowel_heavy" },
  { id: "cluster", relics: ["consonant_cluster"], category: "cluster" },
  { id: "twinned", relics: ["twins", "doppelganger"], category: "twinned" },
  { id: "distinct", relics: ["anagrammer"], category: "distinct" },
  { id: "alphabetical", relics: ["alphabetist"], category: "alphabetical" },
  { id: "money", relics: ["stipend", "mint", "compound", "royalties"] },
  { id: "speed", relics: ["speedrunner", "sunk_cost", "hot_streak", "reserve"] },
  { id: "farming", relics: ["slow_burn", "vault", "long_game"] },
  { id: "sacrifice", relics: ["pyromaniac", "scorched_earth"] },
  { id: "levels", relics: ["thesaurus"] },
]

/**
 * When to play a consumable rather than keep it.
 *
 * The bot bought cards and never played one, and that was not neutral: it
 * filled both slots by accident and kept them full, which is The Hoarder's
 * whole condition met for free. A person holding cards they never play is a
 * person who bought Hoarder on purpose and is paying for it in information, so
 * the bot now plays its cards unless Hoarder is in the tray, and then it keeps
 * them, which is the trade the card offers.
 *
 * The rules are a person's, not an optimum. The Oracle and The Hermit narrow
 * the word, so they are spent once a guess has landed and the pool is still
 * wider than two; The Fool repeats the last guess's score, so it is spent as
 * soon as there is a guess to repeat; The Magician is spent before a guess.
 */
function spend(state: RunState, pool: number): number {
  if (state.relics.some((relic) => relic.id === "hoarder")) return -1
  const guessed = state.round.guesses.length > 0
  return state.consumables.findIndex(({ id }) => {
    if (id === "oracle" || id === "hermit") return guessed && pool > 2
    if (id === "fool") return guessed
    return id === "magician"
  })
}

export type BlindPlayer = {
  /** The next batch of actions, or null to stop. Fed the real state; sees a view. */
  next: (state: RunState, words: WordSource) => Action[] | null
  /** The build this player committed to, for the policy that commits. */
  build?: () => string | null
}

export function blindPlayer(policy: Policy): BlindPlayer {
  let pool: string[] = []
  let openers: string[] = []
  let income: string[] = []
  let key = ""
  let filtered = 0
  /** The builder's commitment, taken at the first relic that belongs to a build. */
  let build: (typeof BUILDS)[number] | null = null
  const shapes = () => (build?.category ? CATEGORY_BY_ID.get(build.category) : undefined)

  /**
   * Narrow the pool by the guesses that have landed since the last look.
   *
   * The filter is `computeFeedback(guess, candidate) === what we were shown`,
   * which is exact where a hand-rolled green/yellow/gray check gets duplicate
   * letters wrong, and it is the engine's own function, so the player is
   * reasoning with precisely the rule it is being scored under.
   *
   * Note `tile.shown` rather than `tile.color`. Under The Fog and The Mirror
   * those differ, and reading the one the screen paints is what makes this a
   * player rather than an oracle. It also means a lying boss can eliminate the
   * true answer, which is why the empty case below is a reset rather than a bug.
   */
  function narrow(view: PlayerView, words: WordSource): void {
    const round = view.round
    const here = `${view.stage}:${view.roundIndex}`
    if (here !== key) {
      key = here
      filtered = 0
      pool = [...words.answers]
    }

    for (const guess of round.guesses.slice(filtered)) {
      const shown: Color[] = guess.tiles.map((tile) => tile.shown)
      pool = pool.filter((word) => twin(computeFeedback(guess.word, word), shown))
    }
    filtered = round.guesses.length

    // The Oracle hands over a position outright; The Hermit rules letters out
    // without spending a guess. Both are on the screen, so both are ours.
    if (round.revealed.some(Boolean)) {
      pool = pool.filter((word) =>
        round.revealed.every((letter, i) => !letter || word[i] === letter),
      )
    }
    if (round.eliminated.length) {
      const dead = new Set(round.eliminated)
      pool = pool.filter((word) => ![...word].some((letter) => dead.has(letter)))
    }

    /*
     * And the alphabet itself. A letter broken out of the run cannot be typed
     * at all, so a candidate holding one is not a candidate, it is a word the
     * keyboard will refuse.
     *
     * This is deduction rather than housekeeping in the Pyromaniac case, which
     * is the case that matters: it breaks a letter at round start, *before* the
     * answer is drawn, so a broken letter genuinely cannot be in the word. A
     * Glass letter that shatters mid-round is the other way round, and the
     * filter is still right for it: the answer was drawn first and may well
     * hold the letter, but it can no longer be typed, so a pool that kept
     * chasing it would be chasing a word this player is not allowed to say.
     * Either way what is filtered out is unreachable.
     *
     * Leaving this out is what taught the harness to lie about the ordinary
     * game. The shop policy takes the dearest relic it can afford and
     * Pyromaniac is a legendary, so the bot buys the thing that dismantles its
     * keyboard and then goes on ranking words by information as if the alphabet
     * were whole. A few rounds later every word in the band is untypeable,
     * `playable` returns null all the way up to `next`, and the caller in
     * `sim.test.ts` breaks its loop and writes the run down as having died
     * there. A stall is indistinguishable from a loss in that record, so the
     * cost was not noise but bias.
     *
     * Over 250 seeds at ascension 0 it ended 33 of the solver's runs and 18 of
     * the farmer's, and the letters were the whole of it: a median of 2 broken
     * out of the alphabet where the solver stopped, 3 where the farmer did.
     * Fixing it moves the solver's mean final stage from 4.51 to 4.92 and the
     * farmer's from 3.82 to 4.04.
     */
    const broken = [...Object.entries(view.letters)]
      .filter(([, entry]) => entry?.destroyed)
      .map(([letter]) => letter)
    if (broken.length) {
      const gone = new Set(broken)
      pool = pool.filter((word) => ![...word].some((letter) => gone.has(letter)))
    }

    // A boss that repaints feedback can talk us out of the truth. Falling back
    // to everything not already guessed is what a player does when the board
    // stops making sense, and it keeps the run going rather than deadlocking.
    if (pool.length === 0) {
      const spent = new Set(round.guesses.map((guess) => guess.word))
      pool = words.answers.filter((word) => !spent.has(word))
    }
  }

  function guess(state: RunState, words: WordSource): Action[] | null {
    const view = playerView(state)
    narrow(view, words)

    const card = spend(state, pool.length)
    if (card >= 0) {
      const use: Action[] = [{ type: "use_consumable", index: card }]
      if (accepted(state, words, use)) return use
    }

    const chips = (word: string) =>
      [...word].reduce((total, letter) => total + baseChips(state, letter), 0)

    // Income first, while there is budget for it. The farmer is buying tiles,
    // not information, so it reaches outside the candidate pool for the richest
    // words in the whole allowed list: the same trade `chip-farmer` makes in
    // the vectors, made by somebody who does not know how the round ends.
    //
    // The `fresh` filter is not a refinement, it is the difference between a
    // policy and a bug. Ranking by chips alone names the same word every guess,
    // because nothing about the ranking changes when it is played; the first
    // draft spent four guesses on one word and went to the last two knowing
    // nothing. Requiring an untried letter costs a little chip value and is
    // what makes this an income *line* rather than a stuck one.
    const left = view.round.maxGuesses - view.round.guesses.length
    const farming = policy === "farmer" || (policy === "builder" && build?.id === "farming")
    if (farming && left > FARMER_RESERVE && pool.length > 1) {
      if (income.length === 0) {
        income = [...words.allowed].sort((a, b) => tableChips(b) - tableChips(a)).slice(0, INCOME)
      }
      const tried = new Set(view.round.guesses.flatMap((guess) => [...guess.word]))
      const fresh = income.filter((word) => [...word].some((letter) => !tried.has(letter)))
      const ranked = fresh.sort((a, b) => chips(b) - chips(a))
      const played = playable(state, words, ranked)
      if (played) return played
    }

    const first = view.round.guesses.length === 0
    if (first && openers.length === 0) {
      const freq = frequencies(pool)
      openers = [...pool]
        .sort((a, b) => information(b, freq) - information(a, freq))
        .slice(0, OPENERS)
    }

    // Rank by what the guess would tell us, then take the best-paying of the
    // ones that tell us roughly the same thing.
    //
    // The band is the whole point. Sorting on information and using chips as a
    // strict tie-break sounds equivalent and is not: information here is a sum
    // of pool frequencies running to the thousands, so exact ties essentially
    // never occur and the chip term never fires at all. That solver was blind to
    // chips in a game about chips, and it died on stage one of 39 runs in 300.
    // Treating anything within `BAND` of the best as equally informative is what
    // lets it prefer the richer of two words that would tell it the same thing.
    const freq = frequencies(pool)
    const from = first ? openers : pool
    const scored = from.map((word) => ({ word, info: information(word, freq) }))
    const best = scored.reduce((top, next) => Math.max(top, next.info), 0)
    // The builder plays the same band and takes a word of its shape out of it
    // first when there is one. Out of the band, never: a shape bought with a
    // guess's worth of information is a round lost to pay for a level.
    const shape = shapes()
    const feeds = (word: string) => (shape?.matches(word) ? 1 : 0)
    const ranked = scored
      .filter((entry) => entry.info >= best * BAND)
      .sort(
        (a, b) => feeds(b.word) - feeds(a.word) || chips(b.word) - chips(a.word) || b.info - a.info,
      )
      .map((entry) => entry.word)
    const played = playable(state, words, ranked.length ? ranked : pool)
    if (played) return played

    /*
     * Nothing the pool proposed was legal, so play the richest word that is.
     *
     * The backstop, and deliberately kept after the filters above rather than
     * instead of them. Those two handle the pool going wrong for reasons this
     * player can reason about: a boss lying until the truth is filtered away
     * (the reset above), and an alphabet with holes in it (the filter above).
     * This one handles the pool going wrong for a reason it cannot: a rule that
     * bars a whole class of word, where every candidate is a real candidate and
     * the engine refuses it at `submit` anyway.
     *
     * That is the ascension half of the same stall, and the broken-letter
     * filter above does not touch it. Over the same 250 seeds it ended 169 of
     * the solver's runs at ascension 5 and 120 at ascension 10 — and the
     * alphabet was intact in most of them, a median of zero letters broken,
     * which is what says these were refusals rather than holes in the keyboard.
     * The ladder is where a run meets a permanent word rule and a boss on top
     * of it, so the ladder is where the band runs out of legal words. Fixing it
     * moves the solver's mean final stage at ascension 5 from 2.92 to 4.86, and
     * that near-two-stage gap is what the harness was calling difficulty.
     *
     * Ranked by chips because information is the thing that just ran out. The
     * band above prefers the best-paying word among those that would tell it
     * roughly the same amount; here nothing informative is available at all, so
     * what the guess pays is the only term left. It scans the whole allowed
     * list rather than the answer list: a word that cannot be the answer is
     * still worth tiles, and on a round the pool cannot play it is the only
     * thing left worth having.
     *
     * Returning null here is still possible and still correct. It means no word
     * in the language can be submitted, which is a round the engine will not let
     * anyone play, not a policy that ran out of ideas.
     */
    const affordable = [...words.allowed].sort((a, b) => chips(b) - chips(a))
    return playable(state, words, affordable)
  }

  /**
   * What the builder would rather buy. Its build's relics first, then a level
   * of its shape (any shape, for the Thesaurus), then whatever the solver
   * would have bought. The other relics stay above everything but those two,
   * because a tray of five is the run's scaling and an empty slot is worth
   * more than a flat step.
   */
  function want(item: ShopItem): number {
    if (policy !== "builder" || !build) return appeal(item)
    if (item.kind === "relic" && build.relics.includes(item.id)) return 7
    if (item.kind === "level" && (build.id === "levels" || item.id === build.category)) return 6
    return appeal(item)
  }

  /**
   * What the builder keeps back from a purchase. Nothing, except in the money
   * build, which is the one whose cards pay for gold *held*: interest, Compound
   * and The Mint all read the pile at the end of a round, and a bot that spent
   * to zero at every shop, as the other two policies do, would price all three
   * at a player who never plays them. The floor is the pile that earns the full
   * interest, and the build's own cards are bought through it, since a money
   * player who passed on The Mint to keep $25 would be keeping it for nothing.
   *
   * Only once the tray is full. Saving from the first shop starved it: holding
   * The Mint from the start, the builder's runs ended a mean stage 3.4 when it
   * kept $25 back from the outset, 4.2 from a tray of four and 4.6 from a full
   * one, over 300 seeds, and never saving at all left the card a mean +2.
   */
  function saving(state: RunState, item: ShopItem): number {
    if (policy !== "builder" || build?.id !== "money") return 0
    if (state.relics.length < difficultyOf(state).relicSlots) return 0
    if (item.kind === "relic" && build.relics.includes(item.id)) return 0
    return INTEREST_PER * INTEREST_CAP
  }

  function commit(state: RunState): void {
    if (policy !== "builder" || build) return
    for (const held of state.relics) {
      build = BUILDS.find((b) => b.relics.includes(held.id)) ?? null
      if (build) return
    }
  }

  return {
    build: () => build?.id ?? null,
    next(state, words) {
      commit(state)
      // Forced, not strategy: the shop refuses everything else until a bought
      // modifier has been pointed at a letter. Same answer as the vectors give.
      if (state.placing) return placeMod(state)

      const pack = state.pack
      if (pack) {
        const options = pack.options.map((item, index) => ({ item, index }))
        const best = options
          .filter((slot): slot is { item: ShopItem; index: number } => slot.item !== null)
          .sort((a, b) => want(b.item) - want(a.item))[0]
        const index =
          best && want(best.item) > appeal(best.item) ? best.index : pack.options.findIndex(Boolean)
        return index >= 0 ? [{ type: "pick_pack", index }] : [{ type: "skip_pack" }]
      }

      if (state.phase === "round") return guess(state, words)
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "victory") return null
      if (state.phase === "shop") {
        // Dearest affordable thing first, relics before modifiers before packs.
        // Shared by both policies deliberately: if the shopping differed, a gap
        // between the two lines could not be blamed on the line.
        const items = state.shop?.items ?? []
        const wanted = items
          .map((item, index) => ({ item, index }))
          .filter(
            (slot): slot is { item: ShopItem; index: number } =>
              slot.item !== null && slot.item.cost <= state.gold - saving(state, slot.item),
          )
          .sort((a, b) => want(b.item) - want(a.item) || b.item.cost - a.item.cost)
        for (const slot of wanted) {
          if (accepted(state, words, [{ type: "buy", index: slot.index }])) {
            return [{ type: "buy", index: slot.index }]
          }
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  }
}

/**
 * The first seed the solver plays to victory, for a test that needs a won run.
 *
 * Searched rather than named, because the bot wins about one run in forty and
 * which runs those are is a fact about the word list and the balance tables,
 * not about the test. Seed 3 was the only win in the first eighty until the
 * names came out of the English answers, and the two tests that had written it
 * down failed on a change that made the game no harder: afterwards seeds 30 and
 * 72 won instead. Two hundred seeds leaves the chance of finding none, at that
 * rate, well under one percent. Cached, since both callers want the same one.
 */
const firstWins = new WeakMap<WordSource, number>()
export function firstWinningSeed(words: WordSource): number {
  const cached = firstWins.get(words)
  if (cached !== undefined) return cached
  for (let seed = 1; seed <= 200; seed++) {
    const bot = blindPlayer("solver")
    let state = startRun(seed, words, 0).state
    while (state.phase !== "game_over" && state.phase !== "victory") {
      const actions = bot.next(state, words)
      if (!actions) break
      for (const action of actions) state = reduce(state, action, words).state
    }
    if (state.phase === "victory") {
      firstWins.set(words, seed)
      return seed
    }
  }
  throw new Error("the solver won none of the first 200 seeds")
}

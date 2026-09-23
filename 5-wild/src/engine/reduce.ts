import { ALPHABET } from "../content/letters"
import {
  BASE_GUESSES,
  CONSUMABLE_SLOTS,
  GOLD_PER_UNUSED_GUESS,
  INTEREST_CAP,
  INTEREST_PER,
  ROUND_PAYOUT,
  roundTargets,
  STAGES,
  STARTING_GOLD,
} from "../content/rounds"
import {
  clampAscension,
  difficultyOf,
  guessRestricted,
  scaleTarget,
  validateGuess,
} from "./ascensions"
import { bossForStage, getBoss } from "./bosses"
import { CATEGORY_BY_ID, levelOf } from "./categories"
import { CONSUMABLE_BY_ID } from "./consumables"
import { ETCHING_BY_ID } from "./etchings"
import { MODIFIER_BY_ID } from "./modifiers"
import { PACK_BY_ID } from "./packs"
import { RANGE_BY_ID, rangeLevelOf } from "./ranges"
import type { Relic, RelicCtx } from "./relics"
import { RELIC_BY_ID } from "./relics"
import type { Rng } from "./rng"
import { derive, pick } from "./rng"
import { scoreGuess } from "./scoring"
import { packContents, placeableLetters, rerollCost, rollShop, sellValue } from "./shop"
import type {
  Action,
  GameEvent,
  LetterState,
  PickedItem,
  Reduced,
  Refusal,
  RoundIndex,
  RoundState,
  RunState,
  ShopItem,
  WordSource,
} from "./state"
import { computeFeedback, toTiles } from "./words"

/**
 * The whole game, as one function.
 *
 *   reduce(state, action, words) -> { state, events }
 *
 * No I/O, no clock, no ambient randomness. The state is never mutated in
 * place, so callers can hold onto the old one, which is what makes undo, replay
 * and the golden vectors possible.
 */

/**
 * RunState is plain JSON by construction, so this is both the cheapest correct
 * deep clone and a standing assertion that it stayed that way: the day someone
 * puts a Map or a Date in the state, saves break here first and loudly.
 */
const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value)) as T

/**
 * Every equipped relic paired with the context its run-level hooks get.
 *
 * One `Rng` shared across the slots rather than one each, so the draws stay in
 * slot order and a relic bought later cannot shift what an earlier one rolled.
 */
function relicHooks(
  state: RunState,
  rng: Rng,
  events: GameEvent[],
): Array<readonly [Relic, RelicCtx]> {
  const out: Array<readonly [Relic, RelicCtx]> = []
  state.relics.forEach((instance, slot) => {
    const relic = RELIC_BY_ID.get(instance.id)
    if (relic) out.push([relic, { state, instance, slot, rng, events }])
  })
  return out
}

function freshLetters(): Record<string, LetterState> {
  const letters: Record<string, LetterState> = {}
  for (const letter of ALPHABET) letters[letter] = { etch: 0, destroyed: false, mod: null }
  return letters
}

/**
 * Answers must avoid destroyed letters, or a broken keyboard could be handed a
 * word it cannot type. If breaking ever narrows the pool to nothing the alphabet
 * heals rather than dead-ends the run, which is unreachable in practice since
 * Pyromaniac refuses to break below fifteen live letters.
 *
 * The answer must also be a legal guess under every rule in force, the boss's
 * and the run's ascension both. The Glutton demands two vowels of every guess,
 * and roughly a fifth of the answer list has only one; ascension 9 forbids
 * repeating a word the run has already used, which would strand a round on an
 * answer nobody is allowed to type. Either way the round would be literally
 * unsolvable, so the filter is the same filter.
 *
 * It runs against the empty round installed a moment ago, which is exactly right
 * for the rules that read the round's history: on a board with no guesses on it,
 * "keep the greens you found" is a rule about nothing and every word passes.
 */
function answerPool(state: RunState, words: WordSource): readonly string[] {
  const dead = [...ALPHABET].filter((letter) => state.letters[letter]?.destroyed)
  const restricted = guessRestricted(state)
  if (dead.length === 0 && !restricted) return words.answers

  const legal = (word: string) =>
    ![...word].some((letter) => dead.includes(letter)) &&
    (!restricted || validateGuess(word, state) === null)

  const pool = words.answers.filter(legal)
  if (pool.length > 0) return pool

  // Nothing survives both filters, so the keyboard heals rather than dead-ends
  // the run. The guess rules do not heal with it: an answer that cannot legally
  // be guessed is an unwinnable round, which is worse than an easy one.
  for (const letter of ALPHABET) {
    const entry = state.letters[letter]
    if (entry) entry.destroyed = false
  }
  if (!restricted) return words.answers
  const healed = words.answers.filter((word) => validateGuess(word, state) === null)
  return healed.length > 0 ? healed : words.answers
}

function beginRound(state: RunState, words: WordSource, events: GameEvent[]): void {
  const bossId = state.roundIndex === 2 ? bossForStage(state) : null

  // Round-start hooks run before the answer is drawn, so a relic that shrinks
  // the alphabet actually shrinks the search space too.
  //
  // The salt keeps the old spelling: it is a coordinate, and renaming it would
  // deal every seed a different run. See `derive` in `rng.ts`.
  const rng = derive(state.seed, "blind_start", state.stage, state.roundIndex)
  for (const [relic, ctx] of relicHooks(state, rng, events)) relic.onRoundStart?.(ctx)

  const boss = getBoss(bossId)
  const difficulty = difficultyOf(state)

  // The empty round is installed before the answer is drawn, so guess rules that
  // read the round's history, since The Tyrant reads its greens and so does
  // ascension 5, judge candidate answers against this round rather than the one
  // just finished.
  state.round = {
    answer: "",
    target: scaleTarget(roundTargets(state.stage)[state.roundIndex], difficulty.targets),
    // The tighter of the two wins rather than the boss's number simply winning.
    // No rung cuts the run's allowance any more, and `Dead Weight` records why
    // the one that did was removed, so today this is the boss's number or the base
    // six and the min never fires. It stays because it is the composition rule
    // rather than a special case of it: a run-level cut must never *loosen* a
    // boss. The Clock deals four, and a run that asked for five may not be
    // handed a fifth.
    maxGuesses: Math.min(boss?.maxGuesses ?? difficulty.guesses, difficulty.guesses),
    bossId,
    draft: "",
    guesses: [],
    score: 0,
    solved: false,
    done: false,
    revealed: [],
    eliminated: [],
    promote: false,
  }

  const answer = pick(
    derive(state.seed, "word", state.stage, state.roundIndex),
    answerPool(state, words),
  )
  state.round.answer = answer
  state.round.revealed = Array.from(answer, () => null)

  state.phase = "round"
  state.reward = null
  state.shop = null
}

/**
 * Roll the shop and stand the player in it.
 *
 * Shared by the two ways in, banking a reward and choosing to play on past the
 * win, so the endless path cannot drift from the ordinary one. The hooks run
 * before the roll, so a relic that bends the shop bends the one it is about to
 * be shown rather than the next one.
 */
function enterShop(state: RunState, events: GameEvent[]): void {
  const rng = derive(state.seed, "shop_enter", state.stage, state.roundIndex)
  for (const [relic, ctx] of relicHooks(state, rng, events)) relic.onShopEnter?.(ctx)

  state.shop = rollShop(state, derive(state.seed, "shop", state.stage, state.roundIndex, 0), 0)
  state.phase = "shop"
  events.push({ type: "shop_entered" })
}

/**
 * The fail state: score below target when the round ends and, at ascension 10,
 * a word left unsolved however big the pile is. One rung lower the same round is
 * survived and paid nothing.
 */
function resolveRound(state: RunState, events: GameEvent[]): void {
  const round = state.round

  // Before the win/lose branch, so a relic that grows on a round ending counts
  // the round that ended, including the one that ended the run. Losing makes
  // that moot rather than wrong, and the alternative is a hook whose firing
  // depends on an outcome it might itself be about to read.
  //
  // Frozen salt, as at round start.
  const rng = derive(state.seed, "blind_end", state.stage, state.roundIndex)
  for (const [relic, ctx] of relicHooks(state, rng, events)) relic.onRoundEnd?.(ctx, round)

  // Farming five wrong guesses to the target and never finding the word is a
  // real line in the ordinary game, and the whole of what ascension 10 takes
  // away. It is checked here rather than as a guess rule because it is not about
  // any one guess: it is about what the round had to have been.
  const difficulty = difficultyOf(state)
  if (round.score < round.target || (!round.solved && difficulty.mustSolve)) {
    state.phase = "game_over"
    events.push({ type: "round_lost" })
    return
  }

  // Paying for unused guesses is the counterweight to chip-farming: the economy
  // rewards exactly the restraint the scoring punishes. Which is why ascension 7
  // cuts the base and leaves this alone; see `Lean Years`.
  const base = Math.max(0, ROUND_PAYOUT[state.roundIndex] - difficulty.payoutCut)
  const unusedGuesses = (round.maxGuesses - round.guesses.length) * GOLD_PER_UNUSED_GUESS
  // Relics get to bend this the way they bend the solve multiplier, in slot
  // order and after the cap, so a card that zeroes it really zeroes it.
  let interest = Math.min(INTEREST_CAP, Math.floor(state.gold / INTEREST_PER))
  for (const instance of state.relics) {
    const bend = RELIC_BY_ID.get(instance.id)?.interest
    if (bend) interest = bend(interest, state)
  }

  // Dead Weight: the round was cleared on chips alone, so it funds nothing:
  // base, unused-guess dollars and interest together. Interest included on
  // purpose. Withholding only the base would leave the farming line paying most
  // of what it used to, since a farmed round spends every guess and so was never
  // collecting the unused-guess dollars anyway; interest is the part a hoarding
  // run would still have banked, and it is the part that makes the rule land.
  //
  // Read after the loss branch rather than inside it, which is what makes this a
  // rung and not a duplicate: by ascension 10 the branch above has already taken
  // the run, so this can only ever fire on the single rung where an unsolved
  // round is still survived.
  const unpaid = difficulty.unpaidIfUnsolved && !round.solved

  state.reward = unpaid
    ? { base: 0, unusedGuesses: 0, interest: 0, total: 0 }
    : { base, unusedGuesses, interest, total: base + unusedGuesses + interest }
  state.phase = "reward"
  events.push({ type: "round_won" })
}

/**
 * Commit an item to the run. Returns a reason it could not be, or null when it
 * landed.
 *
 * Deliberately touches neither the gold nor the shelf it came off: what an item
 * *does* is the same whether it was paid for in the stock or chosen out of a
 * pack, and only the caller knows which of those happened.
 */
function applyItem(state: RunState, item: ShopItem): Refusal | null {
  switch (item.kind) {
    case "relic": {
      if (state.relics.length >= difficultyOf(state).relicSlots) return { code: "no_relic_slots" }
      state.relics.push({ id: item.id })
      return null
    }
    case "consumable": {
      if (state.consumables.length >= CONSUMABLE_SLOTS) return { code: "no_card_slots" }
      state.consumables.push({ id: item.id })
      return null
    }
    case "etch": {
      const etching = ETCHING_BY_ID.get(item.id)
      if (!etching) return { code: "unknown_etching" }
      // Broken letters are skipped rather than etched: they cannot be typed
      // again, so the chips would be unspendable and the keyboard would wear a
      // "+2" pip on a dead key.
      for (const letter of etching.letters) {
        const entry = state.letters[letter]
        if (entry && !entry.destroyed) entry.etch += etching.chips
      }
      return null
    }
    case "level": {
      const category = CATEGORY_BY_ID.get(item.id)
      if (!category) return { code: "unknown_category" }
      // Written on first purchase rather than initialized at run start, so a run
      // that never levels anything keeps `levels` out of its save.
      state.levels = { ...state.levels, [category.id]: levelOf(state, category.id) + 1 }
      return null
    }
    case "range": {
      const range = RANGE_BY_ID.get(item.id)
      if (!range) return { code: "unknown_range" }
      // Stored on the run rather than pushed out into `letters`, unlike an
      // etching: a range level has to keep applying to a letter that is broken
      // and later restored, and writing it per letter would lose that. It
      // also means the save carries four numbers instead of 26.
      state.ranges = { ...state.ranges, [range.id]: rangeLevelOf(state, range.id) + 1 }
      return null
    }
    case "mod": {
      // An unattached one never reaches here: buying it opens the picker
      // instead, and packs only ever deal pairings. Refused rather than
      // asserted, because a save from a build where that stops being true would
      // otherwise put a modifier on the letter `undefined`.
      if (item.letter === undefined) return { code: "mod_needs_letter" }
      const entry = state.letters[item.letter]
      if (!entry) return { code: "unknown_letter" }
      // A letter holds one modifier, so this replaces rather than stacks, and the
      // card says so before the gold is spent.
      entry.mod = item.id
      return null
    }
    case "pack":
      // Packs open rather than apply, which the caller has to handle because it
      // needs a stream to roll the contents from. Reaching here means one was
      // nested inside another, which nothing produces.
      return { code: "nested_pack" }
  }
}

/**
 * Enough of a shop item to name it on screen, for the one line that announces
 * a pick.
 *
 * This was a `switch` over seven tables that read `.name` off each and pasted
 * the letter onto the modifier case. All of that was the catalog's job wearing
 * the engine's clothes: the tables no longer carry names, and the letter needs
 * a space on one side in English and a preposition in French. What is left is
 * the two coordinates the lookup actually needs, which is what the `switch` was
 * really computing.
 */
const picked = (item: ShopItem): PickedItem =>
  item.kind === "mod" && item.letter
    ? { kind: item.kind, id: item.id, letter: item.letter }
    : { kind: item.kind, id: item.id }

const PLACEHOLDER_ROUND: RoundState = {
  answer: "",
  target: 0,
  maxGuesses: BASE_GUESSES,
  bossId: null,
  draft: "",
  guesses: [],
  score: 0,
  solved: false,
  done: false,
  revealed: [],
  eliminated: [],
  promote: false,
}

/**
 * A new run, at the difficulty it was asked for.
 *
 * The ascension arrives as an argument rather than being read from anywhere: the
 * engine has no idea what the player has unlocked, and should not. Which level
 * is on offer is a question about a profile, and profiles live where browsers do.
 */
export function startRun(seed: number, words: WordSource, ascension = 0): Reduced {
  const state: RunState = {
    seed,
    stage: 1,
    roundIndex: 0,
    phase: "round",
    gold: STARTING_GOLD,
    relics: [],
    consumables: [],
    letters: freshLetters(),
    round: clone(PLACEHOLDER_ROUND),
    shop: null,
    reward: null,
  }
  // Left off the state entirely at zero, so the ordinary run's save is the same
  // file it was before ascensions existed.
  const level = clampAscension(ascension)
  if (level > 0) state.ascension = level
  const events: GameEvent[] = []
  beginRound(state, words, events)
  return { state, events }
}

export function reduce(state: RunState, action: Action, words: WordSource): Reduced {
  const events: GameEvent[] = []
  const next = clone(state)

  /** Refusals leave the original state untouched; the UI just flashes a reason. */
  const reject = (refusal: Refusal): Reduced => ({ state, events: [{ type: "rejected", refusal }] })

  switch (action.type) {
    case "start_run":
      return startRun(action.seed, words, action.ascension ?? 0)

    case "type_letter": {
      const round = next.round
      if (next.phase !== "round" || round.done) return reject({ code: "not_your_turn" })
      const letter = action.letter.toLowerCase()
      if (letter.length !== 1 || !ALPHABET.includes(letter)) return reject({ code: "not_a_letter" })
      if (next.letters[letter]?.destroyed) return reject({ code: "letter_broken", letter })
      if (round.draft.length >= round.answer.length) return reject({ code: "no_room" })
      round.draft += letter
      return { state: next, events }
    }

    case "backspace": {
      const round = next.round
      if (next.phase !== "round" || round.done) return reject({ code: "not_your_turn" })
      round.draft = round.draft.slice(0, -1)
      return { state: next, events }
    }

    case "submit": {
      const round = next.round
      if (next.phase !== "round" || round.done) return reject({ code: "not_your_turn" })

      const word = round.draft
      if (word.length !== round.answer.length)
        return reject({ code: "wrong_length", length: round.answer.length })
      if (!words.allowed.has(word)) return reject({ code: "not_in_word_list" })

      // Every rule in force, boss and ascension alike, in one stable order.
      const refusal = validateGuess(word, next)
      if (refusal) return reject(refusal)

      const boss = getBoss(round.bossId)

      const tiles = toTiles(word, computeFeedback(word, round.answer))
      // Before the transform, which is the whole point: the boss that wants to
      // say something about the feedback is the same boss about to destroy it.
      //
      // It is deliberately *not* re-asked after The Magician promotes a tile
      // below. The note is a fact about the guess, how many of these letters
      // are in the word, and stays true whether or not one of them was
      // afterwards handed back its color. A player who reads "2 misplaced" and
      // can see one of them knows where the other is not, which is the counter
      // doing its job rather than a contradiction.
      const note = boss?.note?.(tiles) ?? null
      boss?.transform?.(tiles)

      // After the boss, so The Magician is a genuine counter to The Silence
      // rather than something it quietly erases.
      if (round.promote) {
        const gray = tiles.find((tile) => tile.color === "gray")
        if (gray) {
          gray.color = "yellow"
          gray.shown = "yellow"
          // Marked, because a color this card hands out is worth mult and
          // nothing else. Left unmarked it entered `found()` as a letter proven
          // to be in the word, and ascension 10 then demanded it back in every
          // later guess: promoting the H of CHOMP against an answer of CIVIC
          // made CIVIC itself an illegal guess and the round unwinnable, which
          // is the exact failure `rules.ts` reads `color` rather than `shown`
          // to avoid. See `Tile.promoted`.
          gray.promoted = true
        }
        round.promote = false
      }

      const solved = word === round.answer
      const guessIndex = round.guesses.length
      const guessesLeft = round.maxGuesses - guessIndex - 1
      const result = scoreGuess({
        state: next,
        tiles,
        word,
        guessIndex,
        guessesLeft,
        solved,
        events,
      })

      round.guesses.push({
        word,
        tiles,
        paid: result.paid,
        chips: result.chips,
        mult: result.mult,
        solveBonus: result.solveBonus,
        score: result.score,
        // Spread rather than assigned, so a guess under any other boss carries
        // no key at all, the same discipline `ascension` follows in a vector
        // and `data` follows on a relic.
        ...(note ? { note } : {}),
      })
      round.score += result.score
      round.draft = ""
      // The run's own record of what it has said. Kept whatever the ascension,
      // because the rule that reads it cannot be the thing that decides whether
      // it was written. A run that started recording halfway would enforce
      // "no word twice" against half a run.
      next.history = [...(next.history ?? []), word]

      // A glass letter that broke goes out of the alphabet here rather than
      // mid-pipeline: scoring prices the guess, this owns the run. It lands
      // before the total so the break reads as part of the guess that caused it.
      for (const letter of result.broken) {
        const entry = next.letters[letter]
        if (!entry || entry.destroyed) continue
        entry.destroyed = true
        events.push({ type: "letter_destroyed", letter })
      }

      // Same discipline as the breaks above: scoring decided what each relic
      // grew to, and this is where growing becomes part of the run.
      for (const { slot, data } of result.relicData) {
        const instance = next.relics[slot]
        if (!instance) continue
        instance.data = data
        // Growth earned while scoring gets the same announcement as growth
        // earned at a round's end. Only slots that actually wrote turn up here,
        // so this never fires for a card that merely read its own counter, and
        // the figure is the one the card wears, so floater and relic agree.
        const growth = RELIC_BY_ID.get(instance.id)?.growth?.(instance)
        if (growth) events.push({ type: "relic_grew", slot, id: instance.id, ...growth })
      }

      if (result.gold > 0) {
        next.gold += result.gold
        events.push({ type: "gold", delta: result.gold, reason: "scoring" })
      }
      events.push({ type: "guess_scored", score: result.score, total: round.score })

      // The solve bonus lands on the running total, after the guess is banked,
      // so it multiplies the farming as well as the finish. Emitted last
      // because that is the order it reads on screen: the guess scores, then
      // the whole pile multiplies.
      if (solved && result.solveBonus > 1) {
        round.score = Math.round(round.score * result.solveBonus)
        events.push({ type: "solve_bonus", factor: result.solveBonus, total: round.score })
      }

      if (solved) round.solved = true
      // Solving ends the round on the spot, forfeiting every unplayed guess.
      if (solved || round.guesses.length >= round.maxGuesses) {
        round.done = true
        resolveRound(next, events)
      }
      return { state: next, events }
    }

    case "use_consumable": {
      if (next.phase !== "round") return reject({ code: "only_during_round" })
      const instance = next.consumables[action.index]
      if (!instance) return reject({ code: "no_such_card" })
      const card = CONSUMABLE_BY_ID.get(instance.id)
      if (!card) return reject({ code: "unknown_card" })

      const rng = derive(
        next.seed,
        "consumable",
        next.stage,
        next.roundIndex,
        next.round.guesses.length,
        action.index,
      )
      const problem = card.apply(next, rng, events)
      if (problem) return reject(problem)

      next.consumables.splice(action.index, 1)
      return { state: next, events }
    }

    case "collect": {
      if (next.phase !== "reward" || !next.reward) return reject({ code: "nothing_to_collect" })
      next.gold += next.reward.total
      events.push({ type: "gold", delta: next.reward.total, reason: "round cleared" })

      // The win is offered once, and `won` is what makes it once: past this the
      // run keeps passing stage `STAGES`'s last round every three rounds, and a
      // victory screen every stage would turn the ending into a nag.
      if (!next.won && next.stage >= STAGES && next.roundIndex === 2) {
        next.won = true
        next.phase = "victory"
        events.push({ type: "run_won" })
        return { state: next, events }
      }

      enterShop(next, events)
      return { state: next, events }
    }

    case "continue_run": {
      if (next.phase !== "victory") return reject({ code: "run_not_won" })
      // Picks up exactly where `collect` stopped. The win was offered *instead*
      // of the shop, so continuing is that shop, rolled now rather than then,
      // which is why a player who banks the win never fires a shop hook for a
      // shop they will not see.
      enterShop(next, events)
      return { state: next, events }
    }

    case "buy": {
      if (next.phase !== "shop" || !next.shop) return reject({ code: "not_in_shop" })
      if (next.pack) return reject({ code: "finish_pack_first" })
      if (next.placing) return reject({ code: "place_mod_first" })
      const item = next.shop.items[action.index]
      if (!item) return reject({ code: "already_bought" })
      if (next.gold < item.cost) return reject({ code: "not_enough_gold" })

      // A pack is the one item that is not applied when it is bought. It opens,
      // and the gold buys the choice rather than any particular card in it,
      // which is why the contents are rolled here, at open time, and why the
      // reroll count is in the coordinate: rerolling the shelf to get a
      // different pack has to get different cards in it too.
      if (item.kind === "pack") {
        const pack = PACK_BY_ID.get(item.id)
        if (!pack) return reject({ code: "unknown_pack" })
        const options = packContents(
          next,
          pack,
          derive(next.seed, "pack", next.stage, next.roundIndex, next.shop.rerolls, action.index),
        )
        if (options.length === 0) return reject({ code: "pack_empty" })
        next.pack = { id: pack.id, options, picks: Math.min(pack.picks, options.length) }
        events.push({ type: "pack_opened", id: pack.id, options: options.length })
      } else if (item.kind === "mod" && item.letter === undefined) {
        // The other item that is paid for before it is decided. Held rather than
        // applied, on the same terms as a pack: the gold buys the card, and where
        // it goes is the next question. Checked before the gold moves, because a
        // modifier with nowhere left to sit would otherwise take the money and
        // leave the player holding a card they cannot put down.
        const modifier = MODIFIER_BY_ID.get(item.id)
        if (!modifier) return reject({ code: "unknown_modifier" })
        if (placeableLetters(next, modifier).length === 0) {
          return reject({ code: "no_letter_for_mod" })
        }
        next.placing = modifier.id
      } else {
        const reason = applyItem(next, item)
        if (reason) return reject(reason)
      }

      next.gold -= item.cost
      next.shop.items[action.index] = null
      events.push({ type: "gold", delta: -item.cost, reason: "purchase" })
      return { state: next, events }
    }

    case "place_mod": {
      const held = next.placing
      if (!held) return reject({ code: "nothing_to_place" })
      const modifier = MODIFIER_BY_ID.get(held)
      if (!modifier) return reject({ code: "unknown_modifier" })
      const letter = action.letter.toLowerCase()
      // The same question the shop asked before it stocked the card and before
      // it took the gold, asked once more against the alphabet as it is now.
      // Nothing can have changed it in between, since the shop is held while a
      // modifier is in hand, but the picker is the only one of the three whose
      // input comes from outside.
      if (!placeableLetters(next, modifier).includes(letter)) {
        // The one refusal that names a card, and it names it by id: the player
        // is holding exactly one modifier and has just tapped a letter, so the
        // sentence is worth nothing without saying which of the two is the
        // problem. Echo is why this exists at all — it is the only modifier
        // with a letter list.
        return reject({ code: "mod_not_allowed", id: modifier.id, letter })
      }
      const entry = next.letters[letter]
      if (!entry) return reject({ code: "unknown_letter" })
      // Replaces whatever was there, exactly as buying a pairing does. The
      // picker says which letters are already carrying something, so a trade is
      // a trade the player could see coming.
      entry.mod = modifier.id
      next.placing = null
      events.push({ type: "mod_placed", id: modifier.id, letter })
      return { state: next, events }
    }

    case "pick_pack": {
      if (!next.pack) return reject({ code: "no_pack_open" })
      const item = next.pack.options[action.index]
      if (!item) return reject({ code: "already_taken" })
      // Nothing is charged; the pack was. A pick can still be refused, though,
      // by whatever the item itself needs: a relic with no slot free is the
      // ordinary case, and the pack stays open so the choice can go elsewhere.
      const reason = applyItem(next, item)
      if (reason) return reject(reason)

      next.pack.options[action.index] = null
      next.pack.picks -= 1
      events.push({ type: "pack_picked", id: next.pack.id, taken: picked(item) })
      if (next.pack.picks <= 0) next.pack = null
      return { state: next, events }
    }

    case "skip_pack": {
      if (!next.pack) return reject({ code: "no_pack_open" })
      events.push({ type: "pack_picked", id: next.pack.id, taken: null })
      next.pack = null
      return { state: next, events }
    }

    case "sell_relic": {
      if (next.phase !== "shop") return reject({ code: "sell_only_in_shop" })
      if (next.pack) return reject({ code: "finish_pack_first" })
      if (next.placing) return reject({ code: "place_mod_first" })
      const instance = next.relics[action.index]
      if (!instance) return reject({ code: "no_such_relic" })
      const value = sellValue(RELIC_BY_ID.get(instance.id)?.cost ?? 4)
      next.relics.splice(action.index, 1)
      next.gold += value
      events.push({ type: "gold", delta: value, reason: "sold" })
      return { state: next, events }
    }

    case "reroll": {
      if (next.phase !== "shop" || !next.shop) return reject({ code: "not_in_shop" })
      if (next.pack) return reject({ code: "finish_pack_first" })
      if (next.placing) return reject({ code: "place_mod_first" })
      const cost = rerollCost(next.shop)
      if (next.gold < cost) return reject({ code: "not_enough_gold" })
      next.gold -= cost

      const rerolls = next.shop.rerolls + 1
      next.shop = rollShop(
        next,
        derive(next.seed, "shop", next.stage, next.roundIndex, rerolls),
        rerolls,
      )
      events.push({ type: "gold", delta: -cost, reason: "reroll" })
      return { state: next, events }
    }

    case "next_round": {
      if (next.phase !== "shop") return reject({ code: "not_in_shop" })
      if (next.pack) return reject({ code: "finish_pack_first" })
      if (next.placing) return reject({ code: "place_mod_first" })
      if (next.roundIndex === 2) {
        next.stage += 1
        next.roundIndex = 0
      } else {
        next.roundIndex = (next.roundIndex + 1) as RoundIndex
      }

      // No ceiling here any more. Passing stage `STAGES` used to be the end of the
      // run and is now the end of the *authored* run: `roundTargets` is
      // geometric past the last hand-set stage and `bossForStage` wraps within its
      // band, so stage 9 and stage 90 are both ordinary stages as far as this is
      // concerned. The win was already offered when the reward for the final
      // stage's last round was banked, which is the only place it belongs. This
      // gate could only ever have fired for a run that skipped that one.
      beginRound(next, words, events)
      return { state: next, events }
    }
  }
}

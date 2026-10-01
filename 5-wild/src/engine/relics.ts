import { ALPHABET, isVowel, MIN_LIVE_LETTERS, MULT_FOR_COLOR } from "../content/letters"
import { CONSUMABLE_SLOTS, INTEREST_CAP, INTEREST_PER } from "../content/rounds"
import { difficultyOf } from "./ascensions"
import { CATEGORIES, categoryOf, isCategory, levelOf } from "./categories"
import type { Rng } from "./rng"
import { derive, randomInt, shuffled } from "./rng"
import type { ScoreCtx } from "./scoring"
import type { GameEvent, Growth, Rarity, RelicInstance, RoundState, RunState, Tile } from "./state"

/**
 * What a run-level hook gets.
 *
 * These fire inside the reducer, which is what owns mutation, so unlike the
 * scoring hooks they write `state` and `instance` straight through rather than
 * handing back a patch. Growth is `ctx.instance.data`, and `slot` is here so a
 * card that grows can emit an event pointing at itself.
 */
export type RelicCtx = {
  state: RunState
  /** This copy's row in `state.relics`. Write `data` to grow. */
  instance: RelicInstance
  /** This copy's slot, for events that point at the card. */
  slot: number
  rng: Rng
  events: GameEvent[]
  /**
   * Take this copy out of the tray once the hooks being fired have all run.
   * Deferred rather than immediate because the caller is walking the tray, and
   * a card that spliced itself out mid-walk would hand its slot number to the
   * card behind it for the rest of the loop.
   */
  destroy(): void
}

export type Relic = {
  id: string
  rarity: Rarity
  cost: number
  /** Fires once per tile, left to right, after that tile's base chips land. */
  onTile?: (ctx: ScoreCtx, tile: Tile, index: number, base: number) => void
  /** Fires once after all tiles, in slot order. */
  onGuess?: (ctx: ScoreCtx) => void
  /**
   * Fires after each guess is banked, with its 0-based index in the round. The
   * home for anything that happens to the card between one guess and the next,
   * which scoring cannot do: scoring prices a guess, and a card leaving the
   * tray is a change to the run.
   */
  onGuessEnd?: (ctx: RelicCtx, guessIndex: number) => void
  /** Fires when a round begins, before the answer is chosen. */
  onRoundStart?: (ctx: RelicCtx) => void
  /**
   * Fires when a round ends, win or lose, with the finished round to read:
   * `solved`, the guess list, the final score. The home for growth that is
   * earned over a round rather than over a guess.
   */
  onRoundEnd?: (ctx: RelicCtx, round: RoundState) => void
  /**
   * Fires on entering the shop, before its stock is rolled, so a relic that
   * bends what the shop offers bends the shop it is about to be shown.
   */
  onShopEnter?: (ctx: RelicCtx) => void
  /**
   * What this copy has grown to, for the card to wear. Only scaling relics
   * define it, since a relic whose value never moves has nothing to report that
   * its catalog entry does not already say.
   *
   * It returned the finished string ("+12 mult") until the prose left the
   * engine. Returning the pair instead costs nothing here — every one of the
   * three implementations was a template over exactly these two values — and it
   * is what lets the same growth be announced by `relic_grew` and worn on the
   * card without the two formatting it independently.
   */
  growth?: (instance: RelicInstance) => Growth
  /**
   * Added to the round's solve multiplier. Separate from `onGuess` because the
   * board quotes this figure before the guess exists, so it has to be knowable
   * from the state alone.
   */
  solveBonus?: (state: RunState) => number
  /**
   * Rewrites the interest a cleared round pays, in slot order. Shaped like
   * `solveBonus`, a run-level number a card is allowed to bend, rather than a
   * flag, because the interesting version of this is a card that *takes the
   * interest away* in exchange for something, and a boolean could only ever say
   * one thing.
   */
  interest?: (base: number, state: RunState) => number
  /**
   * Gold this card adds to a cleared round, itemised on the reward screen as a
   * line of its own. A hook rather than an `addGold` in `onRoundEnd` because
   * the reward is a breakdown the player reads before banking it, and gold
   * that arrived from nowhere would be the one figure on it with no line.
   */
  payout?: (state: RunState, instance: RelicInstance) => number
  /**
   * Called when a round is about to be lost. Returning true keeps the run
   * alive: the round is paid nothing, and the card is expected to spend itself
   * with `ctx.destroy()`, since a save that could fire twice is a second life
   * and not a safety net. The first card in slot order that answers wins.
   */
  onRoundLost?: (ctx: RelicCtx, round: RoundState) => boolean
  /** How many rerolls of each shop visit this card pays for. */
  freeRerolls?: number
  /**
   * Borrow the scoring hooks of the card in the next slot to the right: its
   * `onTile`, `onGuess` and `solveBonus`, and nothing that happens between
   * guesses. See `scoringRelic`.
   */
  copiesRight?: boolean
}

/** What a growing relic has banked, and the key every one of them stores it under. */
const grown = (instance: RelicInstance, key: string): number => instance.data?.[key] ?? 0

/**
 * Bank a step of growth and announce it. Shared because the growing relics
 * nearly all do exactly this and the announcement is the part worth keeping
 * identical: the player learns "this card just got bigger" from one animation,
 * whatever earned it.
 */
function grow(ctx: RelicCtx, id: string, key: string, step: number, unit: Growth["unit"]): void {
  const total = grown(ctx.instance, key) + step
  ctx.instance.data = { ...ctx.instance.data, [key]: total }
  // The running total and its unit, not the badge that used to be built here:
  // "+120 chips" is a sentence with a word in it, and the word belongs to a
  // language. `unit` narrows to the ones the badge can actually say, so a new
  // one has to be taught to the catalog rather than smuggled past it.
  ctx.events.push({ type: "relic_grew", slot: ctx.slot, id, amount: total, unit })
}

/**
 * Spend a step of a card that starts full and runs down, and retire it when it
 * is empty. Stored as what has been *spent* rather than what is left, so an
 * instance that has never been written reads as full, the same absent-means-
 * default rule every growing card follows.
 *
 * Silent where `grow` announces. `relic_grew` is the animation for "this card
 * just got bigger", and playing it for a card that just got smaller would teach
 * the opposite of what happened. The card's own badge says the new number.
 */
function decay(ctx: RelicCtx, full: number, step: number): void {
  const spent = grown(ctx.instance, "spent") + step
  if (spent >= full) {
    ctx.destroy()
    return
  }
  ctx.instance.data = { ...ctx.instance.data, spent }
}

/**
 * A one-in-`odds` chance this copy does not survive the round just ended, or
 * the guess, when `at` names one.
 *
 * Its own stream rather than the hook's `rng`, keyed to where the card sat and
 * when, so the roll does not move when a card is bought that fires before it,
 * and a save resumed at the reward screen already knows the answer.
 */
function perish(ctx: RelicCtx, odds: number, ...at: number[]): void {
  const { seed, stage, roundIndex } = ctx.state
  if (randomInt(derive(seed, "perish", stage, roundIndex, ctx.slot, ...at), odds) === 0)
    ctx.destroy()
}

/** How many fresh letters a guess must prove absent to feed Masochist. */
export const MASOCHIST_MISSES = 4

/**
 * Letters this guess proved absent that no earlier guess this round had
 * tried. Counted by letter, and only where every copy in the row is gray, so
 * a doubled letter whose other copy landed is not a miss.
 */
function freshGrays(tiles: readonly Tile[], earlier: readonly { word: string }[]): number {
  const tried = new Set(earlier.flatMap((guess) => [...guess.word]))
  const missed = new Set<string>()
  for (const tile of tiles) {
    if (tried.has(tile.letter)) continue
    if (tiles.every((other) => other.letter !== tile.letter || other.color === "gray"))
      missed.add(tile.letter)
  }
  return missed.size
}

/** Slow Burn's mult per guess already made: 3, plus whatever it has grown. */
const slowBurnStep = (grownBy: number): number => 3 + grownBy

const RARITY_COST: Record<Rarity, number> = {
  common: 4,
  uncommon: 6,
  rare: 8,
  legendary: 10,
}

/**
 * Forty-seven relics, spread deliberately across archetypes so a build identity
 * shows up within the first shop. Note that scoring always reads `tile.color`,
 * never `tile.shown`: The Fog lies to the player, not to the math.
 *
 * The axis most of these sit on is the one the solve bonus creates: farming a
 * round grows the pile the bonus will multiply, and costs a point of that
 * multiplier per guess. Slow Burn and The Vault pay for staying; Sunk Cost and
 * Speedrunner pay for leaving. Owning a pair from opposite ends is a real
 * dilemma rather than a stack, which is the point.
 *
 * The last five arrived together, and each answers something the build rubric
 * found missing: a terminal for the money build, a payoff that makes breaking
 * the alphabet a plan rather than a tax, and three cards that *grow*, one on a
 * guess condition, one on a round condition and one on a shop condition, so that
 * scaling reads as a class of card and not as one oddity.
 *
 * The five after those are word-shape and position cards, and they are priced
 * off the word list rather than off intuition: Head Start wants a vowel in
 * column one (16.1% of allowed words), Keystone wants the middle tile green,
 * The Chorus wants three vowels (13.4%). The rarer the shape, the bigger the
 * payoff, which is why the ×3 sits at rare and the +15 at common. Loaded Dice
 * is the exception and pays for variance instead of for a shape.
 *
 * All five were then re-priced against the shipped set by simulation, with one
 * card equipped, no shopping, 250 seeds and mean round score against an empty
 * tray, and three of them moved: Lexicographer +4 to +3, Loaded Dice 0–30 to 0–20,
 * Keystone ×2 to ×3. Each card's comment carries the pair of numbers that
 * settled it. What the harness cannot see is a player *steering*, so for the
 * two cards that want a shape it reads as a floor rather than as a price.
 *
 * The nineteen after those came as one batch, to a complaint rather than to a
 * gap: late trays had converged on the same handful of ×mult cards, and the
 * shop's first shelves had too little worth $4 on them. So seven of them are
 * commons, and most of those either last (Collector, which counts the tray) or
 * end (Fresh Ink, First Draft, Candle, which spend themselves and hand the slot
 * back). Every shape now has a multiplier behind it, Twins being the one that
 * was missing, and five of the nineteen do nothing on the board at all: a free
 * reroll, two wages, doubled interest and a second chance, so that a late slot
 * can be spent on something other than another multiplier. Carbon Copy is the
 * one that makes the order of the tray a decision.
 *
 * Then the whole set was re-priced a second way, because a price taken on an
 * empty tray answers a question no shop asks. The complaint was a stage-two
 * boss cleared ten times over at ascension 10 by Loaded Dice, Lexicographer and
 * Keystone. The harness is a solver with a fixed tray and no shopping, over
 * stages one to three and 250 seeds. It prices each card by what it adds on top
 * of a two-card background drawn from the seed, which is the tray a card is
 * actually bought into. It reproduced the complaint: that trio clears the
 * stage-two boss by a median ×6.1 before any further shopping, against ×1.9 for
 * a random three. It also cleared the trio. On a tray they read ×2.27, ×1.61
 * and ×1.96, each inside its band. What stacks them is that they sit on three
 * different factors of the product, and three cards that are each fair can
 * still multiply to seven.
 *
 * The cards that were *not* fair on a tray were four, and they moved together:
 * Hot Streak +30 to +12, Sunk Cost +10 to +6, Reserve +8 to +4, Green Thumb +8 to
 * +4. Each card's comment has its pair. Over 300 full runs with shopping, the
 * mean final stage went 4.46 to 4.29 at ascension 0 and 2.79 to 2.74 at
 * ascension 10, and the median stage-two boss overshoot went ×5.3 to ×4.4 and
 * ×3.1 to ×2.6. That is a trim, not a new curve. Blank Page read ×3.21 in the
 * same harness and was left alone: three cards on a five-slot tray is the one
 * tray it is built for, and at ascension 9, with a slot gone, it reads ×2.12.
 *
 * Then flat cards became growers, for a reason no price on a tray could
 * show: a target that grows ×2.2 a stage is only ever met by something that
 * grows too, and with Snowball, The Hoarder, Hot Streak and the Pyromaniac
 * pair banned, the builder bot won no run in the yellow, gray or farming
 * builds and 3 of 747 in money, over 5,000 seeds. Bloodhound, The Mint and Slow
 * Burn were the weakest card in three of them (×1.08, ×1.02, ×1.16 on
 * `bun run relics`), so they were converted rather than joined by new cards,
 * and Masochist was the gray build's mult half. Each grows on its build's own
 * condition: yellows played, gold held, rounds taken slowly, probes that
 * missed. Habit, the one card that already grew on a shape, went +2 to +3.
 *
 * Each was swept on `bun run builds` to about 25 wins among the runs holding
 * it, with the old growers banned: half a percent of 5,000 seeds, a door
 * apiece, so that six builds with nothing growing behind them come to about
 * what the four old doors carry between them. With those banned the builder
 * went from 32 wins to 132. With nothing banned it went from 95 to 210
 * (4.2%), the solver from 50 to 153 and the farmer to 109, and the old doors
 * held rather than gave way: the solver's runs holding Snowball, The Hoarder
 * and Hot Streak won 24, 15 and 22 with the new growers struck from the shelf
 * and 39, 43 and 42 beside them, since a run now finds a second door to
 * pair with its first.
 */
export const RELICS: readonly Relic[] = [
  {
    id: "green_thumb",
    rarity: "common",
    cost: RARITY_COST.common,
    // Halved from +8, and the reason is the size of what it adds to rather than
    // the size of the number. A word's own chips are five to ten, so eight a
    // green was the difference between a 7-chip row and a 31-chip one: ×2.55
    // on top of a two-card tray over 250 seeds, where every other unflagged
    // common read ×1.2 to ×1.7. +4 reads ×1.85, still the best common, no
    // longer ahead of most uncommons.
    onTile: (ctx, tile) => {
      if (tile.color === "green") ctx.addChips(4)
    },
  },
  {
    id: "scavenger",
    rarity: "common",
    cost: RARITY_COST.common,
    onTile: (ctx, tile) => {
      if (tile.color === "yellow") ctx.addGold(1)
    },
  },
  {
    id: "vowel_hoarder",
    rarity: "common",
    cost: RARITY_COST.common,
    onTile: (ctx, tile) => {
      if (isVowel(tile.letter)) ctx.addMult(4)
    },
  },
  {
    id: "slow_burn",
    rarity: "common",
    cost: RARITY_COST.common,
    // Pays you to stall, which is the exact opposite of what the solve bonus
    // pays you to do. Owning both is a genuine dilemma rather than a stack.
    //
    // The farming build's door, and Hot Streak's mirror: that card grows on a
    // round cleared in three, this one on a round cleared in four or more, so
    // the two are the same bet placed at opposite ends of the guess budget. It
    // grows its *step* rather than banking a flat sum, which keeps the card
    // what it was, a payoff that rises through the round, and makes the
    // growth pay most on exactly the late guesses a farmer is staying for.
    //
    // Farming had no grower, and the builder bot committed to it won 0 of 408
    // runs with every existing grower banned (`bun run builds`, 5,000 seeds).
    // The step starts at 3 rather than the old flat 5, so the fresh card is
    // weaker than it was and the growth is what it is bought for; +3 a long
    // round. Swept against the door's wins on the same run: step 5 growing +1
    // on rounds of five or more won 5, +2 on four or more won 11, +3 won 40
    // from a step of 5 and 28 from a step of 3. The last is the one that lands
    // on the half a percent a door is asked for; see the header above `RELICS`.
    onGuess: (ctx) => {
      if (ctx.guessIndex > 0) ctx.addMult(slowBurnStep(ctx.getData("step")) * ctx.guessIndex)
    },
    onRoundEnd: (ctx, round) => {
      if (!round.solved || round.guesses.length < 4) return
      // Not `grow`, which announces what it banked: the card wears its step,
      // and the step is the bank plus the base.
      const step = grown(ctx.instance, "step") + 3
      ctx.instance.data = { ...ctx.instance.data, step }
      ctx.events.push({
        type: "relic_grew",
        slot: ctx.slot,
        id: "slow_burn",
        amount: slowBurnStep(step),
        unit: "mult",
      })
    },
    growth: (instance) => ({ amount: slowBurnStep(grown(instance, "step")), unit: "mult" }),
  },
  {
    id: "consonant_cluster",
    rarity: "common",
    cost: RARITY_COST.common,
    onGuess: (ctx) => {
      if (isCategory("cluster", ctx.word)) ctx.timesMult(1.5)
    },
  },
  {
    id: "cold_open",
    rarity: "common",
    cost: RARITY_COST.common,
    // The opening probe is the guess with the least information behind it and
    // the most riding on it, and until now it was the one nothing paid for.
    onGuess: (ctx) => {
      if (ctx.guessIndex === 0) ctx.addChips(30)
    },
  },
  {
    id: "bloodhound",
    rarity: "common",
    cost: RARITY_COST.common,
    // Yellow is the color that actually teaches you something, so this is the
    // rare relic that pays for playing well rather than for playing wide.
    //
    // It was +6 points a yellow, flat, and the weakest common that touches a
    // guess: ×1.08 on `bun run relics` (solver, 400 seeds), and the yellow
    // build around it won 0 with every existing grower banned. Now it banks +1
    // for each yellow played and pays the bank on every guess, Snowball's shape
    // on the other color: pays what it had, *then* counts, so the number on the
    // card is the number it just added. The condition is the color, not the
    // guess: every probe lands some yellow, but a solving guess lands none, so
    // the card grows on the guesses that were searching and a run that finds
    // the word on the opener feeds it nothing.
    //
    // Swept on `bun run builds` (builder, 5,000 seeds, every existing grower
    // banned): +1 a yellow won 8 runs holding it, +2 won 24, +3 won 74, the
    // last three times what a door is asked for. Held from the first shop it
    // ends a run a mean +48, against Snowball's +61 on the color that is on
    // every row.
    //
    // Back to +1 at v36. The new doors grew up around it and +2 had become the
    // best of them by a distance: on the same harness, runs holding it won 109
    // of 680 (16.0%) against Habit's 65 and Mint's 36, the nearest thing to
    // Snowball on the shelf at a common's price, when Snowball is the same
    // shape paying mult, at legendary. At +1 it won 40 (5.9%), level with Mint
    // (6.0%) and Habit (6.0%); the yellow build fell from 20 wins to 6 and the
    // whole sweep from 176 to 107. Unbanned the builder fell from 5.7% to 4.5%
    // and the solver from 3.6% to 2.8%.
    onGuess: (ctx) => {
      const banked = ctx.getData("chips")
      if (banked > 0) ctx.addChips(banked)
      const yellows = ctx.tiles.filter((tile) => tile.color === "yellow").length
      if (yellows > 0) ctx.setData("chips", banked + yellows)
    },
    growth: (instance) => ({ amount: grown(instance, "chips"), unit: "chips" }),
  },
  {
    id: "head_start",
    rarity: "common",
    cost: RARITY_COST.common,
    /*
     * The first positional card in the game, and the column is a measurement
     * rather than a preference. `TODO` asked for this on column two; the word
     * list says column two holds a vowel in 64% of allowed words and 57% of
     * answers, which is not a condition, it is a rounding error. Column one is
     * 16% and 11%.
     *
     * That is the rare tight condition that does not fight deduction, which is
     * why it can pay this much at common. The famous openers are vowel-initial,
     * AROSE, ADIEU and AUDIO, so the word this card wants on guess one is the word
     * a good player was going to type anyway. It only starts costing something
     * later, once the greens are dictating the shape.
     *
     * ×2.55 on the mean round score over 250 seeds, which is the middle of the
     * common band: Green Thumb ×3.88, Slow Burn ×3.02, Cold Open ×2.81, Vowel
     * Hoarder ×2.76, this, Consonant Cluster ×1.13.
     */
    onGuess: (ctx) => {
      if (isVowel(ctx.word[0] ?? "")) ctx.addMult(15)
    },
  },
  {
    id: "loaded_dice",
    rarity: "common",
    cost: RARITY_COST.common,
    /*
     * Mean +10, and the variance is the price. Every other flat-mult card in the
     * game can be planned around; this one cannot, so it is worth less than its
     * average to a player deciding whether a guess clears the target, which is
     * exactly the decision this game is made of.
     *
     * It was written at 0–30 and that was too much: ×4.01 over 250 seeds made it
     * the strongest common in the game, ahead of Green Thumb's ×3.88, for a card
     * that asks nothing of the player. At 0–20 it reads ×3.16 and sits where a
     * no-condition common belongs: better than Cold Open, worse than the cards
     * that want something in return.
     *
     * The roll comes from `ctx.roll()` and therefore from the seed, keyed to the
     * stage, the round, the guess and the slot, so it is the same dice however
     * the run reaches that guess. Rerolling by retyping is not available, and a
     * save resumed mid-round scores what it would have scored.
     */
    onGuess: (ctx) => ctx.addMult(Math.floor(ctx.roll() * 21)),
  },
  {
    id: "fresh_ink",
    rarity: "common",
    cost: RARITY_COST.common,
    // The strongest flat mult at common, and the price is that it may not be
    // there next round. A one-in-six chance a round is a mean life of six
    // rounds, two stages, which is long enough to carry an opening and short
    // enough that nobody builds a late game on it.
    onGuess: (ctx) => ctx.addMult(15),
    onRoundEnd: (ctx) => perish(ctx, 6),
  },
  {
    id: "first_draft",
    rarity: "common",
    cost: RARITY_COST.common,
    // Front-loaded on purpose: +20 in the first round it is held, four less
    // every round after, gone after the fifth. It is the card for the stretch
    // where the tray is empty and the targets are not, and it clears its own
    // slot by the time a real build wants it.
    onGuess: (ctx) => {
      const left = 20 - ctx.getData("spent")
      if (left > 0) ctx.addMult(left)
    },
    onRoundEnd: (ctx) => decay(ctx, 20, 4),
    growth: (instance) => ({ amount: 20 - grown(instance, "spent"), unit: "mult" }),
  },
  {
    id: "candle",
    rarity: "common",
    cost: RARITY_COST.common,
    // First Draft on the chip axis, burning down at the same pace: five rounds
    // from +30 to nothing. Chips rather than mult so that the two are a pair
    // and not a stack.
    //
    // Written at +100, and chips on an empty tray are the scarce half: ×9.53
    // over 120 seeds, above every rare in the game. +30 reads ×3.56, beside
    // Reserve's ×3.60 and under Green Thumb's ×3.95, both of which have since
    // been halved; on a two-card tray this reads ×1.56 against their ×1.9.
    onGuess: (ctx) => {
      const left = 30 - ctx.getData("spent")
      if (left > 0) ctx.addChips(left)
    },
    onRoundEnd: (ctx) => decay(ctx, 30, 6),
    growth: (instance) => ({ amount: 30 - grown(instance, "spent"), unit: "chips" }),
  },
  {
    id: "reserve",
    rarity: "common",
    cost: RARITY_COST.common,
    // Sunk Cost's chip half, at common: largest on the opening guess and zero
    // on the last, so it pays for the same early exit the solve bonus does.
    //
    // +4 a guess left rather than +8, cut beside Sunk Cost and for the same
    // reason as Green Thumb: forty chips on an opener whose letters are worth
    // five is not a common's worth. ×2.67 on top of a two-card tray over 250
    // seeds, then ×1.91, level with Green Thumb at the top of the common band.
    onGuess: (ctx) => {
      if (ctx.guessesLeft > 0) ctx.addChips(4 * ctx.guessesLeft)
    },
  },
  {
    id: "collector",
    rarity: "common",
    cost: RARITY_COST.common,
    // Counts itself, so it is never worth less than +4. The common that stays
    // alive into the late game, because the thing it counts only goes up: a
    // full tray makes it +20, which a stage-one card has no business
    // being worth, and is why it is the card to keep when the others are sold.
    // +4 a card rather than +3: ×1.34 over 120 seeds became ×1.45 on an empty
    // tray, and ×1.24 on a late one, where most commons have faded to ×1.1.
    onGuess: (ctx) => ctx.addMult(4 * ctx.state.relics.length),
  },
  {
    id: "second_look",
    rarity: "common",
    cost: RARITY_COST.common,
    // The one common that does nothing on the board. A reroll is worth more the
    // more a build knows what it is looking for, so this is cheap early and
    // dear late, the reverse of every scoring common.
    freeRerolls: 1,
  },
  {
    id: "stipend",
    rarity: "common",
    cost: RARITY_COST.common,
    // Pays for itself in two rounds and then funds the shop, which is the
    // entire case for it. Small on purpose: the unused-guess dollars are the
    // economy's lever, and a flat wage large enough to rival them would pay a
    // farming run the same as a fast one.
    payout: () => 2,
  },
  {
    id: "anagrammer",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Five distinct letters is exactly what a good probe looks like, and the
    // exact opposite of what Doppelgänger wants. They do not belong in the
    // same build, which is what makes each of them a choice.
    //
    // ×1.5 rather than ×2, because the condition is barely one: 89% of the
    // blind solver's guesses had five distinct letters, since that is what a
    // probe is. On `bun run relics` (solver, 250 seeds) ×2 read ×1.87 / ×1.85
    // / ×1.68 on stages 1–2 / 3–5 / 6+, ×1.75 overall, above every uncommon
    // but Keystone and within 0.25 of Indelible, a rare that dies. +10 mult was
    // the other candidate: ×1.40 early and ×1.12 late, an early card to sell,
    // which is a different card and not Twins' mirror. ×1.5 reads ×1.43 /
    // ×1.43 / ×1.34, ×1.38 overall, mid-shelf among the uncommons.
    onGuess: (ctx) => {
      if (isCategory("distinct", ctx.word)) ctx.timesMult(1.5)
    },
  },
  {
    id: "keystone",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    /*
     * The first ×mult keyed to a color. Every other color payoff in the game
     * is additive, whether Green Thumb, Masochist or the base mult per tile, which left
     * the color build with no ceiling and no reason to want a *particular*
     * green rather than more of them.
     *
     * The middle column because it is the one deduction reaches last: the edges
     * fall out of a probe, the center usually takes a commitment. So this pays
     * late in a round, which is when a farming build wants its multiplier, and
     * asks for a green the player would have had to work for anyway.
     *
     * Written as ×2 and measured at ×1.40 over 250 seeds, which was the weakest
     * uncommon in the game, since the condition simply does not come up by accident,
     * and a bot that never steers for it almost never has it. ×3 reads ×1.90,
     * beside Anagrammer's ×2.19. The harness is the floor rather than the price:
     * it measures a player who never plays for the middle column, and the card
     * exists for the one who does.
     *
     * Then trimmed to ×2.5, not for the floor but for the ceiling: on a late kit
     * it read ×2.00, the most of any uncommon, and it was one of the handful
     * of multipliers every late tray converged on. ×2.5 reads ×1.74 there and
     * ×2.04 on an empty tray, still the best uncommon for the player who steers.
     *
     * Back to ×2, because the premise was wrong. The condition does come up by
     * accident: the middle tile is green on 54% of the blind solver's guesses,
     * since every solving guess is five greens and the ones before it are mostly
     * close. The ×1.40 above was a per-guess harness that could not see the
     * solve bonus, which multiplies the whole round. On `bun run relics`
     * (solver, 250 seeds, round totals) ×2.5 read ×1.81, the top uncommon by
     * 0.16 and within 0.2 of Indelible, a rare. ×2 reads ×1.59 / ×1.59 / ×1.51
     * on stages 1–2 / 3–5 / 6+, ×1.54 overall: still above First Impression's
     * ×1.47, which is the order the columns deserve, and still the multiplier
     * a player who steers for the middle will get the most out of.
     */
    onGuess: (ctx) => {
      if (ctx.tiles[2]?.color === "green") ctx.timesMult(2)
    },
  },
  {
    id: "lexicographer",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    /*
     * The card that pays for probing. It counts letters *spent*, not letters in
     * the word being scored, so it reads the same information the player is
     * playing to gather: five fresh letters a guess is +15 chips a guess, and
     * by guess four a clean opener has it near +50. That puts it just under The
     * Vault (+75 by then) without being it: The Vault pays for staying, this
     * pays for staying *and* covering ground.
     *
     * It was written at +4 and that put it at ×5.95 over 250 seeds, above
     * Snowball, a rare, and second among uncommons only to Sunk Cost. +3 reads
     * ×4.73. Both figures are the card's best case: the harness probes with
     * eight fixed words chosen to cover the alphabet, which is the play this
     * card most wants and more discipline than a real run manages.
     *
     * `state.round.guesses` holds only submitted guesses, so this reads prior
     * ones and never itself, the same rule Slow Burn and The Vault follow.
     *
     * Ascension 1 works directly against it: Hunted forces found letters to be
     * reused, so every guess after the first covers less new alphabet. That is a
     * real anti-synergy rather than an accident, and it is the reason this sits
     * at uncommon instead of rare.
     */
    onGuess: (ctx) => {
      const seen = new Set<string>()
      for (const guess of ctx.state.round.guesses) for (const letter of guess.word) seen.add(letter)
      if (seen.size > 0) ctx.addChips(3 * seen.size)
    },
  },
  {
    id: "sunk_cost",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Slow Burn read backwards: this one is worth most on the guess where the
    // solve bonus is also worth most, so it sharpens the incentive to leave
    // early instead of blunting it.
    //
    // +6 a guess left rather than +10. At +10 it read ×3.26 on top of a
    // two-card tray over 250 seeds, level with The Vault and Snowball, two
    // rares, and a card that also points the same way as the solve bonus has
    // no need to be priced like one. +6 reads ×2.29, beside Lexicographer and
    // Greedy Grammarian at the top of the uncommon band.
    onGuess: (ctx) => {
      if (ctx.guessesLeft > 0) ctx.addMult(6 * ctx.guessesLeft)
    },
  },
  {
    id: "speedrunner",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    onGuess: (ctx) => {
      if (ctx.solved && ctx.guessIndex <= 2) ctx.timesMult(3)
    },
  },
  {
    id: "qs_bargain",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    onTile: (ctx, tile, _index, base) => {
      if ("jqxz".includes(tile.letter)) ctx.addChips(base * 2)
    },
  },
  {
    id: "greedy_grammarian",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Gray tiles are worthless for deduction and contribute no mult, which
    // makes them the perfect substrate for rewarding being spectacularly wrong.
    onTile: (ctx, tile) => {
      if (tile.color === "gray") ctx.addChips(15)
    },
  },
  {
    id: "doppelganger",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // It paid each repeated letter's points a second time and nothing else, and
    // that read ×1.04 on `bun run relics` (solver, 400 seeds), last in the
    // uncommon band but for the growers measured fresh, against Twins' ×1.33 on
    // the very same condition. Paying the color's mult again as well, so the
    // letter really does score twice, only reached ×1.06: a repeat is gray or
    // yellow often enough that the color is worth 0 or 1. A flat +5 on top
    // reads ×1.12, ahead of Q's Bargain (×1.07) and still well short of Twins,
    // which is right: Twins multiplies the whole row and this adds to two tiles
    // of it. A flat +10 alone read ×1.15 and would have dropped the color,
    // which is the half of the card that says what it is.
    onTile: (ctx, tile, _index, base) => {
      const copies = [...ctx.word].filter((letter) => letter === tile.letter).length
      if (copies < 2) return
      ctx.addChips(base)
      ctx.addMult(MULT_FOR_COLOR[tile.color] + 5)
    },
  },
  {
    id: "hot_streak",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // The growth counterpart to Speedrunner, on the chip axis: both pull toward
    // cashing out early, and both are dead weight in a Slow Burn build. A
    // farming run will never trip this, which is the whole point of it.
    //
    // +12 a quick round rather than +30. The condition turned out to be no
    // condition for a player who deduces: the solver clears most early rounds
    // in three, so the card grew nearly every round and read ×4.12 on top of a
    // two-card tray over 250 seeds, the highest of any card short of the
    // Pyromaniac and above every rare. +15 still read ×2.60; +12 reads ×2.30,
    // the top of the uncommon band, and the growth keeps it climbing past that
    // for the run that keeps finding the word fast.
    onGuess: (ctx) => {
      const banked = ctx.getData("chips")
      if (banked > 0) ctx.addChips(banked)
    },
    onRoundEnd: (ctx, round) => {
      if (round.solved && round.guesses.length <= 3) grow(ctx, "hot_streak", "chips", 12, "chips")
    },
    growth: (instance) => ({ amount: grown(instance, "chips"), unit: "chips" }),
  },
  {
    id: "hoarder",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Consumables exist to be spent, and this pays you not to spend them. That
    // is the tension it is for: every Oracle you sit on is information you chose
    // not to have, banked as chips instead.
    //
    // +10 a visit rather than +40. `bun run relics` cannot see it, since it
    // prices every card fresh and a fresh Hoarder is worth nothing (×1.02), so
    // this came from a throwaway copy that credited the growth, assuming both
    // slots held full from the purchase on, over 250 seeds. The condition is
    // $6 once and then free forever, and the bank lands on every guess before
    // mult rather than once a round, so at +40 it read ×9.34 bought at the
    // first shop and ×6.80 at the third, against Pyromaniac's ×1.91 at the top
    // of the stock table. From the third shop +20 read ×3.91, +15 ×3.19, and
    // +10 ×2.46: still above every rare, but that is a ceiling the figure
    // cannot price the two unspent consumables against. Rare at +15 was the
    // other option, and was passed over because it would still top the shelf.
    onGuess: (ctx) => {
      const banked = ctx.getData("chips")
      if (banked > 0) ctx.addChips(banked)
    },
    onShopEnter: (ctx) => {
      if (ctx.state.consumables.length >= CONSUMABLE_SLOTS) {
        grow(ctx, "hoarder", "chips", 10, "chips")
      }
    },
    growth: (instance) => ({ amount: grown(instance, "chips"), unit: "chips" }),
  },
  {
    id: "first_impression",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Keystone's column-one twin. The first tile is the one a probe most often
    // lands, because openers are chosen for their commonest starts, so this
    // fires by accident far more than the middle column does and pays ×2
    // rather than ×3 for it.
    //
    // Keystone now pays the same ×2, and the premise above did not survive
    // measuring: on `bun run relics` the first tile is green on 45% of the
    // blind solver's guesses and the middle on 54%, the reverse of the claim.
    // They read ×1.47 and ×1.54, close enough to call twins.
    onGuess: (ctx) => {
      if (ctx.tiles[0]?.color === "green") ctx.timesMult(2)
    },
  },
  {
    id: "twins",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Twinned was the one shape with a category and no multiplier behind it.
    // Anagrammer's mirror: the two ask opposite questions of the same word, so
    // a tray cannot want both. ×2.5 rather than Anagrammer's ×1.5 because the
    // shape is the rarer one: at ×2 it read ×1.14 over 120 seeds, the weakest
    // uncommon on the shelf, and ×2.5 reads ×1.21. Still a floor, since the
    // harness never steers into a repeated letter.
    onGuess: (ctx) => {
      if (isCategory("twinned", ctx.word)) ctx.timesMult(2.5)
    },
  },
  {
    id: "blank_page",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Counts empty slots, itself included, so alone it is ×(slots) and in a
    // full tray it is ×1. The only card that argues for *selling*, and the one
    // an ascension that cuts a slot quietly nerfs.
    onGuess: (ctx) => {
      const open = difficultyOf(ctx.state).relicSlots - ctx.state.relics.length
      if (open > 0) ctx.timesMult(1 + open)
    },
  },
  {
    id: "habit",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Pays what the shape had, *then* counts this guess, the order Snowball
    // follows, so the first word of a shape pays nothing. Counted from the day
    // it was bought rather than from the start of the run, because the run
    // keeps no such tally and one kept only for this card would be state that
    // every other card pays to carry. No badge: the count is five numbers, one
    // per shape, and a card cannot wear five. +2 a word rather than +1, which
    // read ×1.22 over 120 seeds and was not worth a slot; +2 reads ×1.44.
    //
    // +3 since, as the shape build's grower beside its levels. On
    // `bun run builds` (builder, 5,000 seeds, every other grower banned) the
    // run won 109 without Habit on the shelf and 124 with it at +2, 15 wins
    // for the card; +3 won about 40 and +4 57. It is already the one card
    // that counts every shape at once, so +4 made it the door every run
    // walked through, which is what the flat step on Distinct was cut for.
    onGuess: (ctx) => {
      const shape = categoryOf(ctx.word).id
      const played = ctx.getData(shape)
      if (played > 0) ctx.addMult(3 * played)
      ctx.setData(shape, played + 1)
    },
  },
  {
    id: "no_maybes",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // A yellow is a letter in the wrong place, so a guess without one is either
    // a clean miss or a clean hit. Rewards the late, committed guess and the
    // wild opener both, and punishes the half-informed middle.
    //
    // ×1.75 rather than ×2, because the clean hit is not a choice: every
    // solving guess is five greens, so this pays on the round's biggest guess
    // whatever else the player did, and fires on 64% of the blind solver's
    // guesses, more than Keystone's 54% or First Impression's 45%. At ×2 that
    // made it the top uncommon on `bun run relics` (solver, 250 seeds), ×1.68 /
    // ×1.72 / ×1.61 on stages 1–2 / 3–5 / 6+ and ×1.65 overall, a rare's worth.
    // Promoting it was the alternative, and was not taken because the three
    // color-conditioned ×mults are one family and belong at one rarity, ordered
    // so the easiest condition pays least. ×1.75 reads ×1.51 / ×1.54 / ×1.45,
    // ×1.48 overall: level with First Impression, under Keystone's ×1.54.
    onGuess: (ctx) => {
      if (!ctx.tiles.some((tile) => tile.color === "yellow")) ctx.timesMult(1.75)
    },
  },
  {
    id: "compound",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Doubles interest after the cap, so the cap doubles with it. Mint's
    // opposite number: that card takes the interest and pays score, this one
    // pays more interest and no score, and holding both is a card that does
    // nothing, which is the honest outcome.
    interest: (base) => base * 2,
  },
  {
    id: "royalties",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // A wage that rises with the run: $1 a round to start, and a dollar more
    // for every boss beaten while it is held. Bought in stage one, it is paying
    // $5 or more by the end, which is what makes it a long bet at uncommon
    // rather than a Stipend with a bigger number.
    payout: (_state, instance) => 1 + grown(instance, "gold"),
    onRoundEnd: (ctx, round) => {
      if (round.solved && round.bossId !== null) grow(ctx, "royalties", "gold", 1, "gold")
    },
    growth: (instance) => ({ amount: 1 + grown(instance, "gold"), unit: "gold" }),
  },
  {
    id: "masochist",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // The gray build's door. It paid +8 mult a gray, flat, and the build
    // around it won 0 of 374 runs with every existing grower banned (builder,
    // 5,000 seeds, `bun run builds`). It then banked +2 mult on every guess
    // that landed three grays, and that condition turned out to be the one
    // every round meets by accident: 80% of the solver's openers land three
    // grays and 45% of its second guesses do (300 seeds), so the card grew
    // 1.66 times a round without a guess being given up for it. Worse, nothing
    // refuses a repeated word, so a word known to be all gray could be typed
    // five times a round and banked five times, and a run that did that early
    // stopped caring what it played.
    //
    // So the gray it pays for is now a guess that was actually spent: one
    // after the first that proves four letters absent that nobody had tried
    // this round. The solver lands that on 12% of second guesses and almost
    // never later, 0.15 times a round, so nearly every bank is a burner played
    // on purpose, and each costs a step of the solve bonus. A repeated word
    // has no fresh letters, so the spam pays nothing. And it banks once a
    // round, because about twenty absent letters is still room for three
    // burners, and three of them would put the farm back.
    //
    // +10 because a bank is now rare and dear. Swept with the builder's gray
    // build burning one guess a round and every existing grower banned
    // (5,000 seeds): +6 won 14 runs holding the card, +8 won 21, +10 won 29,
    // against 28 for the old card on the same seeds; the committed gray build
    // wins 24 where it won 10, which is the point of asking for a guess.
    onGuess: (ctx) => {
      const banked = ctx.getData("mult")
      if (banked > 0) ctx.addMult(banked)
      if (ctx.guessIndex === 0) return
      // One plus the round's index in the run, so zero still means never.
      const round = ctx.state.stage * 3 + ctx.state.roundIndex + 1
      if (ctx.getData("round") === round) return
      if (freshGrays(ctx.tiles, ctx.state.round.guesses) < MASOCHIST_MISSES) return
      ctx.setData("mult", banked + 10)
      ctx.setData("round", round)
    },
    growth: (instance) => ({ amount: grown(instance, "mult"), unit: "mult" }),
  },
  {
    id: "chorus",
    rarity: "rare",
    cost: RARITY_COST.rare,
    /*
     * The biggest word-shape multiplier in the game, because it asks for the
     * rarest shape: three vowels appear in 13.4% of allowed words and 9.1% of
     * answers, against 63.9% for Anagrammer's five-distinct. A ×3 that fires one
     * guess in eight is the same expected value as a ×2 that fires half the
     * time, bought with far more planning.
     *
     * It is also the answer to the vowel build being one card deep. Vowel
     * Hoarder pays per vowel and this multiplies once you have enough of them,
     * so the two stack the way an engine should, and a leveled Vowel Heavy
     * category triples the same guess a third time. That stack is intended: it
     * is the payoff for committing to a shape the answer list rarely rewards.
     *
     * ×1.95 over 250 seeds, which reads low for a rare and is the measurement
     * working: the harness types eight fixed probes, one of which happens to
     * hold three vowels, so that number is what the card pays a player who never
     * steers. It is priced for the one who does.
     *
     * One of the vowels now has to land green, because the stack paid most on
     * the guess that knows least. A three-vowel word fires it whatever the
     * tiles say, and the only guess free to be one is the opener, so a
     * player-reported A9 win took 45–75k on guess one and 5–12k on every guess
     * after, with an ADIEU that could land all gray and still triple. Rounds
     * where the builder held this and Vowel Hoarder gave guess one 44% of the
     * pile against an even 27% (1,000 seeds), where every other tray gives it
     * less than even. A green asks the word to have been read, not just
     * spelled. Over 400 solver seeds it moves the opener's share of the
     * card's firings from 31% to 8% and its rate from 8.2% of guesses to
     * 4.6%, and the builder's vowel build wins 21 runs in 5,000 where it won
     * 26 (`bun run builds`).
     */
    onGuess: (ctx) => {
      if ([...ctx.word].filter(isVowel).length < 3) return
      if (ctx.tiles.some((tile) => isVowel(tile.letter) && tile.color === "green")) ctx.timesMult(3)
    },
  },
  {
    id: "alphabetist",
    rarity: "rare",
    cost: RARITY_COST.rare,
    onGuess: (ctx) => {
      if (isCategory("alphabetical", ctx.word)) ctx.timesMult(2)
    },
  },
  {
    id: "vault",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // Slow Burn's chip half, so a farming build can grow both halves of the
    // product instead of one. Together they are the strongest argument in the
    // game for spending the whole guess budget, and the solve multiplier is
    // the strongest argument against.
    onGuess: (ctx) => {
      if (ctx.guessIndex > 0) ctx.addChips(25 * ctx.guessIndex)
    },
  },
  {
    id: "mint",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // The money build's terminal: the thing that finally converts a pile of
    // gold into score instead of into more gold.
    //
    // Priced by taking the interest away rather than by picking a small number.
    // Interest already pays you to hoard; a card that *also* paid you to hoard
    // would not be a choice, it would be the answer. This way the two are
    // alternatives: compound the pile, or cash it in every guess.
    //
    // It paid +3 mult per $5 held, on the pile as it stood, and that was a
    // flat card with a moving number: ×1.02 on `bun run relics`, and the money
    // build won 3 of 747 with every existing grower banned. Now it *mints*: at
    // the end of each round the pile is struck into mult the card keeps, +1
    // per $2, and the card pays what it has banked on every guess. The pile is
    // still the player's to spend, so the choice is the same one, sharpened:
    // every dollar spent in the shop is mult the next round will not strike.
    //
    // The step is steep because the card is rare and the condition is dear.
    // On `bun run builds` (builder, 5,000 seeds, every existing grower banned,
    // the builder keeping $25 back once its tray is full) +1 per $5 won 6 runs
    // holding it, per $4 won 9, per $3 won 14, per $2 won 25. Uncommon at $4 a
    // step was tried for the reach instead and won 13 on 627 runs held: the
    // shelf was never what held it back, the pile was.
    onGuess: (ctx) => {
      const banked = ctx.getData("mult")
      if (banked > 0) ctx.addMult(banked)
    },
    //
    // Struck from the pile up to the one interest stops at, $25, so a round
    // mints +12 at most. Uncapped, the golden victor, playing past the win on
    // $366, ended with +3,552 mult on it; the bot never holds more than it
    // needs to, and a person would have found the pile's ceiling was the sky.
    onRoundEnd: (ctx) => {
      const steps = Math.floor(Math.min(ctx.state.gold, INTEREST_PER * INTEREST_CAP) / 2)
      if (steps > 0) grow(ctx, "mint", "mult", steps, "mult")
    },
    growth: (instance) => ({ amount: grown(instance, "mult"), unit: "mult" }),
    interest: () => 0,
  },
  {
    id: "scorched_earth",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // What makes Pyromaniac and Glass a plan rather than a tax. The alphabet
    // stops at MIN_LIVE_LETTERS, so eleven letters is the ceiling and +132 mult
    // is what a fully committed sacrifice run is buying, paid for with a
    // keyboard that can no longer type eleven letters, which is a real price.
    onGuess: (ctx) => {
      const broken = [...ALPHABET].filter((letter) => ctx.state.letters[letter]?.destroyed).length
      if (broken > 0) ctx.addMult(12 * broken)
    },
  },
  {
    id: "snowball",
    rarity: "legendary",
    cost: RARITY_COST.legendary,
    // Pays what it had, *then* counts this guess, so a tile never pays on the
    // guess that earned it. Growing after paying is what keeps the card legible:
    // the number on the card is the number it just added.
    //
    // One, from five, by way of two. The value was never the whole problem: the
    // ceiling at +2 landed near +200, which is where a growing card *should*
    // finish, and it still read as an auto-buy. The reason is that it asks for
    // nothing. Its two siblings both name a condition, since Hot Streak wants
    // the round cleared in three and The Hoarder wants both slots full at the
    // shop, and every word ever typed has green tiles in it. An unconditional card at
    // $6 that ends the run as the biggest number on the board is not a build,
    // it is a tax on not buying it.
    //
    // So the rarity is the real fix and the halving is the trim that follows
    // it. Across 300 recorded runs of the greedy bot the card went from a mean
    // +154 at the ending (median 170, peak 230) to a mean +61 (median 60, peak
    // 115), still the strongest rare on the mult axis and no longer three times
    // the field. The number that matters most is the one that did *not* move:
    // the win rate held at 10.0% against 10.3% and the mean final stage at 4.90
    // against 4.89. Sixty points came off the best card in the game and the
    // game did not get harder, which is what it looks like when a card was
    // crowding builds out rather than carrying them.
    //
    // Being rare also self-corrects on the axis that matters, since the shelf
    // tilts *toward* rare as the stages go by and a Snowball found at stage 7 has
    // almost nothing left to eat.
    onGuess: (ctx) => {
      const banked = ctx.getData("mult")
      if (banked > 0) ctx.addMult(banked)
      const greens = ctx.tiles.filter((tile) => tile.color === "green").length
      if (greens > 0) ctx.setData("mult", banked + greens)
    },
    growth: (instance) => ({ amount: grown(instance, "mult"), unit: "mult" }),
  },
  {
    id: "thesaurus",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // Pays for levels bought in *any* shape, and on every guess rather than
    // only the ones of that shape, so it is the payoff for spreading upgrades
    // instead of stacking one. A tenth per level keeps it modest until late:
    // ten levels bought is ×2.
    //
    // Left alone when levels began to compound (`Category.growth`) and the
    // shelf began to lean toward leveled shapes, which were the two reasons it
    // might have needed repricing: on `bun run relics` (solver, 1,000 seeds)
    // it read ×1.13 before and ×1.15 after, ×1.23 to ×1.28 on stages 6+, still
    // among the weakest rares. It counts levels, not what they pay.
    onGuess: (ctx) => {
      const bought = CATEGORIES.reduce((sum, c) => sum + levelOf(ctx.state, c.id) - 1, 0)
      if (bought > 0) ctx.timesMult(1 + 0.1 * bought)
    },
  },
  {
    id: "patron",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // A rare that makes the tier below it worth keeping, which is the point of
    // it: the late-game tray was converging on rares and legendaries, and this
    // is a reason to hold an uncommon past stage four.
    onGuess: (ctx) => {
      const held = ctx.state.relics.filter(
        (r) => RELIC_BY_ID.get(r.id)?.rarity === "uncommon",
      ).length
      if (held > 0) ctx.timesMult(1.25 ** held)
    },
  },
  {
    id: "indelible",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // The largest unconditional multiplier in the game, and not quite
    // permanent: one round in forty it is gone. That is a mean life of more
    // than a full run, so the chance is not the price. It is the reason the
    // player never quite stops watching it.
    //
    // Written at ×3 and measured at ×2.95 on a late kit (every shape at level
    // three, every letter etched, two relics already held), the most any card
    // in the game added there. ×2 reads ×1.97, beside Anagrammer's ×1.89.
    //
    // Then ×1.75, because ×2 was worth more than any legendary: ×1.99 overall
    // on `bun run relics` (solver, 250 seeds), over Pyromaniac's ×1.87 and
    // Patron's ×1.86, and it earns that on 99% of guesses with no condition to
    // meet. ×1.75 reads ×1.75 / ×1.79 / ×1.71 on stages 1–2 / 3–5 / 6+, ×1.74
    // overall: under both, and still the largest unconditional multiplier.
    //
    // Then the roll moved from the round to the guess, at the same one in
    // forty, because a round-end roll was a mean life of forty rounds, more
    // than the run's twenty-four, and so no price at all. The solver plays four
    // guesses a round (300 seeds), so rolled after every guess it lasts about
    // ten rounds on average, and it can go in the middle of one: the guesses after the roll lose it, which is the thing
    // the player was meant to be watching for. It also makes the card dearer
    // to a farmer than to a solver, the one way a flat ×mult can lean on the
    // solve bonus rather than past it.
    onGuess: (ctx) => ctx.timesMult(1.75),
    onGuessEnd: (ctx, guessIndex) => perish(ctx, 40, guessIndex),
  },
  {
    id: "second_wind",
    rarity: "rare",
    cost: RARITY_COST.rare,
    // Does nothing until the run is over, and then says it is not. A quarter of
    // the target is the line between a round that went wrong and a build that
    // was never going to make it, and only the first deserves a second chance.
    // It spends itself either way, and the round it saved pays nothing.
    onRoundLost: (ctx, round) => {
      if (round.score < 0.25 * round.target) return false
      ctx.destroy()
      return true
    },
  },
  {
    id: "long_game",
    rarity: "legendary",
    cost: RARITY_COST.legendary,
    // Buys back a guess's worth of multiplier, so every point of farming is
    // worth more and the cash-out can wait one turn longer. It multiplies the
    // whole pile, which is why a flat +1 belongs at this rarity.
    solveBonus: () => 1,
  },
  {
    id: "pyromaniac",
    rarity: "legendary",
    cost: RARITY_COST.legendary,
    onGuess: (ctx) => ctx.addMult(40),
    // Runs before the answer is drawn, so a broken letter genuinely cannot
    // appear in the word: the search space shrinks along with your keyboard.
    onRoundStart: ({ state, rng, events }) => {
      const alive = [...ALPHABET].filter((letter) => !state.letters[letter]?.destroyed)
      // Leave enough alphabet to still form words; refuse to break past that.
      // The same floor a Glass letter stops at: one rule, two ways in.
      if (alive.length < MIN_LIVE_LETTERS) return
      const letter = shuffled(rng, alive)[0]
      if (letter === undefined) return
      const entry = state.letters[letter]
      if (!entry) return
      entry.destroyed = true
      events.push({ type: "letter_destroyed", letter })
    },
  },
  {
    id: "carbon_copy",
    rarity: "legendary",
    cost: RARITY_COST.legendary,
    // Worth whatever sits to its right, which makes slot order a decision for
    // the first time. Legendary because the best card in a tray is the ceiling
    // on a copy of it, and a common copier would be a second copy of that
    // ceiling at a common's price. See `scoringRelic` for what it borrows.
    copiesRight: true,
  },
]

export const RELIC_BY_ID = new Map(RELICS.map((relic) => [relic.id, relic]))

/**
 * The card whose scoring hooks a slot fires, and the slot that card sits in.
 *
 * Itself, for every card but a copier, which walks right until it finds one
 * that is not also a copier. Two copiers side by side both copy the card after
 * them, and a copier in the last slot copies nothing. Rightward only, so the
 * walk cannot loop.
 *
 * Only the *scoring* hooks are borrowed. The round, shop and reward hooks would
 * each need their own answer to "whose state does this write", and a copied
 * Pyromaniac breaking two letters a round is a card nobody asked for.
 */
export function scoringRelic(
  tray: readonly RelicInstance[],
  slot: number,
): { relic: Relic; source: number } | null {
  for (let at = slot; at < tray.length; at++) {
    const relic = RELIC_BY_ID.get(tray[at]?.id ?? "")
    if (!relic) return null
    if (!relic.copiesRight) return { relic, source: at }
  }
  return null
}

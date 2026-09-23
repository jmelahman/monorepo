import { ALPHABET, isVowel, MIN_LIVE_LETTERS } from "../content/letters"
import { CONSUMABLE_SLOTS, INTEREST_PER } from "../content/rounds"
import { isCategory } from "./categories"
import type { Rng } from "./rng"
import { shuffled } from "./rng"
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
}

export type Relic = {
  id: string
  rarity: Rarity
  cost: number
  /** Fires once per tile, left to right, after that tile's base chips land. */
  onTile?: (ctx: ScoreCtx, tile: Tile, index: number, base: number) => void
  /** Fires once after all tiles, in slot order. */
  onGuess?: (ctx: ScoreCtx) => void
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
}

/** What a growing relic has banked, and the key every one of them stores it under. */
const grown = (instance: RelicInstance, key: string): number => instance.data?.[key] ?? 0

/**
 * Bank a step of growth and announce it. Shared because all three growing
 * relics do exactly this and the announcement is the part worth keeping
 * identical: the player learns "this card just got bigger" from one animation,
 * whatever earned it.
 */
function grow(ctx: RelicCtx, id: string, key: string, step: number, unit: "chips" | "mult"): void {
  const total = grown(ctx.instance, key) + step
  ctx.instance.data = { ...ctx.instance.data, [key]: total }
  // The running total and its unit, not the badge that used to be built here:
  // "+120 chips" is a sentence with a word in it, and the word belongs to a
  // language. `unit` narrows to the two the badge can actually say, so a third
  // one would have to be taught to the catalog rather than smuggled past it.
  ctx.events.push({ type: "relic_grew", slot: ctx.slot, id, amount: total, unit })
}

const RARITY_COST: Record<Rarity, number> = {
  common: 4,
  uncommon: 6,
  rare: 8,
  legendary: 10,
}

/**
 * Twenty-eight relics, spread deliberately across archetypes so a build identity
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
 */
export const RELICS: readonly Relic[] = [
  {
    id: "green_thumb",
    rarity: "common",
    cost: RARITY_COST.common,
    onTile: (ctx, tile) => {
      if (tile.color === "green") ctx.addChips(8)
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
    onGuess: (ctx) => {
      if (ctx.guessIndex > 0) ctx.addMult(5 * ctx.guessIndex)
    },
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
    onTile: (ctx, tile) => {
      if (tile.color === "yellow") ctx.addChips(6)
    },
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
    id: "anagrammer",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // Five distinct letters is exactly what a good probe looks like, and the
    // exact opposite of what Doppelgänger wants. They do not belong in the
    // same build, which is what makes each of them a choice.
    onGuess: (ctx) => {
      if (isCategory("distinct", ctx.word)) ctx.timesMult(2)
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
     */
    onGuess: (ctx) => {
      if (ctx.tiles[2]?.color === "green") ctx.timesMult(3)
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
    onGuess: (ctx) => {
      if (ctx.guessesLeft > 0) ctx.addMult(10 * ctx.guessesLeft)
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
    onTile: (ctx, tile, _index, base) => {
      const copies = [...ctx.word].filter((letter) => letter === tile.letter).length
      if (copies > 1) ctx.addChips(base)
    },
  },
  {
    id: "hot_streak",
    rarity: "uncommon",
    cost: RARITY_COST.uncommon,
    // The growth counterpart to Speedrunner, on the chip axis: both pull toward
    // cashing out early, and both are dead weight in a Slow Burn build. A
    // farming run will never trip this, which is the whole point of it.
    onGuess: (ctx) => {
      const banked = ctx.getData("chips")
      if (banked > 0) ctx.addChips(banked)
    },
    onRoundEnd: (ctx, round) => {
      if (round.solved && round.guesses.length <= 3) grow(ctx, "hot_streak", "chips", 30, "chips")
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
    onGuess: (ctx) => {
      const banked = ctx.getData("chips")
      if (banked > 0) ctx.addChips(banked)
    },
    onShopEnter: (ctx) => {
      if (ctx.state.consumables.length >= CONSUMABLE_SLOTS) {
        grow(ctx, "hoarder", "chips", 40, "chips")
      }
    },
    growth: (instance) => ({ amount: grown(instance, "chips"), unit: "chips" }),
  },
  {
    id: "masochist",
    rarity: "rare",
    cost: RARITY_COST.rare,
    onTile: (ctx, tile) => {
      if (tile.color === "gray") ctx.addMult(8)
    },
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
     */
    onGuess: (ctx) => {
      if ([...ctx.word].filter(isVowel).length >= 3) ctx.timesMult(3)
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
    onGuess: (ctx) => {
      const steps = Math.floor(ctx.state.gold / INTEREST_PER)
      if (steps > 0) ctx.addMult(3 * steps)
    },
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
    rarity: "rare",
    cost: RARITY_COST.rare,
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
]

export const RELIC_BY_ID = new Map(RELICS.map((relic) => [relic.id, relic]))

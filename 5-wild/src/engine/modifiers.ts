import type { ScoreCtx } from "./scoring"
import type { Rarity, RunState, Tile } from "./state"

/**
 * Letter modifiers: the enhancement layer, Balatro's shape again.
 *
 * There is no deck here to enhance a card in, so the thing that carries a
 * modifier is the letter itself: buy Steel E and every E you ever play is steel,
 * for the rest of the run. That makes a modifier a bet on your own vocabulary
 * rather than on a draw. Gold E pays out most guesses and Gold Z pays out when
 * you force it, which is the decision the shop is really selling.
 *
 * One modifier per letter. Buying a second replaces the first, exactly as an
 * enhancement does, so a letter is a slot rather than a stack. Etchings are the
 * separate, stacking upgrade and survive being modified.
 *
 * Effects get real code and a narrow context, for the same reason relics do.
 * They fire per scored tile, after the tile's own chips and color land and
 * before any relic sees it: the letter is what was played, the relics are what
 * watched.
 *
 * Per tile means per tile for the ×mult ones too: a steel letter multiplies the
 * mult as it stands where it sits, not the finished total, so the same letter is
 * worth more at the end of a word than at the front. That is Balatro's rule and
 * it is worth keeping, because it gives word *order* a scoring consequence, which is
 * something this game otherwise has no way to ask about.
 */

export type ModId =
  | "chip"
  | "mult"
  | "gold"
  | "steel"
  | "glass"
  | "wild"
  | "lucky"
  | "echo"
  | "anchor"

export type ModCtx = ScoreCtx & {
  /**
   * Retire a letter from the alphabet once the guess has finished scoring.
   * Refused if it would leave too little alphabet to spell with.
   *
   * The only thing a modifier gets that a relic does not. `roll` lives on
   * `ScoreCtx` now, since both need seeded chance and neither may reach for
   * anything else.
   */
  breakLetter(letter: string): void
}

export type Modifier = {
  id: ModId
  /**
   * What the key wears in the corner. One or two glyphs, since keys are small.
   *
   * The one piece of a modifier's description that stayed behind when the rest
   * went to `src/ui/lang`: `+20` and `×2` and `⚓` are the same in every
   * language, and a pip is sized to fit a key rather than to read as a
   * sentence. The prose that expands it is `modifier[id].text`.
   */
  pip: string
  rarity: Rarity
  /**
   * What it costs on a letter somebody else picked: the price a pack quotes,
   * and the one the shop used to charge back when it rolled the pairing too.
   */
  cost: number
  /**
   * What the shop charges to sell it unattached, for the player to point at a
   * letter of their own choosing.
   *
   * Dearer than `cost`, and it has to be: a rolled Chip is worth 0.96 chips a
   * gold averaged over the alphabet, and Chip on E is worth 2.65. Choice is most
   * of this card's value, so the shop that hands it over has to charge for it.
   * Not the full 2.8× though, since a rolled pairing you did not like was never
   * bought, so what the premium is really buying is the visits where the letter
   * slot used to be dead, and those were already worth nothing.
   *
   * Set per card rather than derived, because the spread is not uniform: Echo
   * only ever goes on six letters, so choosing among them is worth less than
   * choosing among 26, and Anchor's whole value is which letter it lands on.
   */
  choiceCost: number
  /**
   * The letters this may be sold on, when it cannot go on just any of them.
   * Absent means the whole alphabet, which is the ordinary case.
   *
   * This exists because a conditional modifier can be sold onto a letter that
   * can never satisfy the condition, and then it is not a weak card but a dead
   * one. Echo pays on a repeated letter, and no five-letter answer repeats a J,
   * Q or X at all, and the shop was charging $5 for a card guaranteed to do
   * nothing. A restriction is the honest fix, because the alternative is
   * pricing every conditional card for the worst letter it might land on, which
   * makes it worthless on the good ones too.
   *
   * Only for conditions that are *independent* of how often the letter comes
   * up. Anchor needs a green tile, and a rare letter is green rarely for the
   * same reason it scores rarely. That is the ordinary Gold Z bet the whole
   * layer is built on, and it does not need protecting from.
   */
  letters?: string
  /** Fires once per tile carrying it, before any relic sees that tile. */
  onTile: (ctx: ModCtx, tile: Tile) => void
}

/**
 * How often a glass letter breaks on a tile that breaking is allowed on.
 *
 * Rarer in practice than a quarter sounds, because the tile has to be gray
 * *and* the letter genuinely absent: measured over 19,315 recorded guesses, a
 * glass E broke on 3.31% of them, a glass S on 0.79%, a glass K on 5.85%. Left
 * where it is with the ×3: the per-guess rate is small, but it compounds over
 * a run of fifty, and losing a common letter outright is the price the number
 * is charging for.
 */
const GLASS_BREAK = 0.25

/** How often a lucky letter pays. */
const LUCKY_CHANCE = 0.25

/**
 * What a wild letter adds, by the color its tile landed.
 *
 * Read it as a ladder rather than three numbers: the ratio decides what the card
 * is *for* and the base decides what it is worth, and the two are independent to
 * a degree worth knowing about. Six ladders in the 3:2:1 shape, from 9/6/3 up to
 * 21/14/7, all measured the same 1.25x tilt toward the guesses that missed while
 * their rate ran from 6 to 15 points a gold. So price this by scaling all three
 * together and re-aim it by changing their spacing; touching one number does
 * both at once and is almost never what is wanted.
 */
const WILD_MULT = { gray: 24, yellow: 14, green: 4 } as const

export const MODIFIERS: readonly Modifier[] = [
  {
    id: "chip",
    pip: "+20",
    rarity: "common",
    cost: 4,
    choiceCost: 6,
    // Flat, and flat is the point: it is worth the same on Q as on E, so it is
    // the one modifier that makes a cheap probing letter worth typing.
    onTile: (ctx) => ctx.addChips(20),
  },
  {
    id: "mult",
    pip: "+8",
    rarity: "common",
    cost: 5,
    choiceCost: 8,
    // +4 made this the worst card on the shelf at either end of a run, which is
    // not what the other common is for. Over 8,333 recorded guesses Chip on E
    // paid 202 points a guess for $6 and Mult on E paid 55 for $8, or 34 a gold
    // against 7. The gap narrows with depth and never closes: chips outgrow mult
    // over a run, so a point of mult is worth about 14 chips at stage two and
    // about 76 by the end, and even at the far end Mult was 38 a gold against
    // Chip's 82.
    //
    // Doubled rather than discounted, because the price was not the problem.
    // $4 buys the same card at 12 a gold and it is still the thing you take
    // last. At +8 the two commons stop being ranked and start being a choice:
    // Chip is worth more in the first two stages, Mult is worth more once the
    // relics have grown the chips it multiplies, and which one is right depends
    // on where the run is. That is the decision the slot is for.
    onTile: (ctx) => ctx.addMult(8),
  },
  {
    id: "gold",
    pip: "$2",
    rarity: "uncommon",
    cost: 6,
    choiceCost: 9,
    // Income priced against Scavenger, which pays $1 a yellow from a relic slot.
    // This takes no slot and fires on any color, but only on one letter, so
    // what it is really worth is decided by the letter the shop offered.
    onTile: (ctx) => ctx.addGold(2),
  },
  {
    id: "wild",
    pip: "★",
    rarity: "uncommon",
    cost: 6,
    choiceCost: 9,
    // Color is this game's suit, and a wild card is the one the suit does not
    // decide. It used to read that as "always counts as green", which capped it
    // at +3 mult and only on a tile that had missed: 12 points a guess against
    // Mult's 55, last on the shelf by a factor of four, and dominated outright
    // by a common that cost a gold less. The ladder is what it pays instead.
    //
    // The card's job is to crutch a guess that went badly, so it has to pay most
    // where the tile did worst, and the whole difficulty is in how much most is.
    // Two shapes were built and measured over 6,714 recorded guesses before this
    // one. Paying only for the miss, the old card's shape with a bigger number,
    // keeps the identity and destroys the price: a solving guess is five greens
    // by definition, so there is nothing left to promote and the card pays
    // literally nothing on the guess that wins the round. At x6 that is a 93x
    // tilt onto the guesses that went badly and still only 71 points a guess, 8 a
    // gold, under a common costing a gold less. Buying the identity back needed a
    // multiple around x10, which is +30 mult on a gray for a card that goes dead
    // the moment you find the word. A flat floor of 12 fixes the price the other
    // way and overshoots in the other direction: 118 points a guess at 13 a gold,
    // but a 0.76x tilt, paying *more* on the solve than on the miss. That is a
    // well-priced card that is no longer this card.
    //
    // 24/14/4 is both at once: 119 points a guess and 13 a gold, the floor's
    // rate to within the noise, at a 2.14x tilt. It pays double on a probe and
    // still pays four on a green, which is the part that keeps it honest: a card
    // that pays nothing when you solve is a card telling you not to.
    //
    // How far the tilt could go was measured rather than guessed, because the
    // fear it answers is that a miss-paying card makes farming to the target beat
    // solving on sight. Over 900 seeds with wilds on E/T/A, farming *loses*
    // either way: -0.079 stages with no wild at all, -0.024 under the floor,
    // -0.022 under this ladder, and -0.021 under a deliberately absurd 48/28/8.
    // The premium never turns positive because the solve bonus multiplies the
    // whole round total (`1 + guessesLeft`), so one more probe only pays when it
    // beats the round-so-far divided by the guesses left, a bar that rises as
    // the round scores. The card narrows that gap and cannot close it, which is
    // the right amount of subsidy for the line it is meant to rescue.
    //
    // A promotion card looked like it would reward hiding a wild on a letter that
    // never lands green. It does not: Q, J, X and Z are the four worst letters to
    // wild under every variant tried, floor and ladder alike, because a letter
    // that never gets typed never fires. The best letters stay E, A, R, O, L, T.
    //
    // It also stays out of the color-reading relics' way, which some of the
    // alternatives did not: this adds mult and never rewrites `tile.color`, so
    // Masochist and Greedy Grammarian still see the gray they are paid for.
    onTile: (ctx, tile) => ctx.addMult(WILD_MULT[tile.color]),
  },
  {
    id: "lucky",
    pip: "?",
    rarity: "uncommon",
    cost: 6,
    choiceCost: 9,
    // Expects +5 mult a tile against Mult's flat +4 for a gold less, so the
    // premium is entirely for the variance, which is the trade Balatro's Lucky
    // card offers too. It is the only modifier whose value you cannot read off
    // the board before you submit, and the one guess in four that it lands on is
    // worth waiting for.
    onTile: (ctx) => {
      if (ctx.roll() < LUCKY_CHANCE) ctx.addMult(20)
    },
  },
  {
    id: "echo",
    pip: "↺",
    rarity: "uncommon",
    cost: 5,
    choiceCost: 7,
    // Fires on every copy, so a doubled letter collects +120 across the word.
    // Pays a player for the shape the Twinned category and Anagrammer already
    // reward, which is the point: a modifier that only pays inside a build can
    // afford a bigger number than a flat one.
    //
    // Sold only on the six letters an answer actually doubles in more than 2%
    // of words. Below that the card is decoration: an answer repeats a W in one
    // word out of two thousand and a J, Q or X in none at all. Restricted to
    // AELOST, +60 is worth 0.97 chips a gold against Chip's flat 0.96: the
    // same money, taken in a lump on the words that earn it.
    letters: "aelost",
    onTile: (ctx, tile) => {
      const copies = [...ctx.word].filter((letter) => letter === tile.letter).length
      if (copies >= 2) ctx.addChips(60)
    },
  },
  {
    id: "anchor",
    pip: "⚓",
    rarity: "rare",
    cost: 8,
    choiceCost: 12,
    // Wild's opposite number, deliberately: Wild pays most on the guess that
    // went worst, this pays only on the letter you have already nailed. Both
    // sides of the color line are now purchasable.
    //
    // Back-loaded by nature, since greens accumulate through a round, and it
    // rewards re-typing a letter you have locked, which The Tyrant compels and
    // The Miser forbids, so the same card swings hard either way.
    //
    // +250 was priced off "only 8.8% of tiles land green", and that number is
    // right about the wrong sample. Across 17,018 recorded guesses it holds for
    // guesses that did not solve, 8.4% of 56,440 tiles, but the guess that
    // *wins* a round is five greens by definition, and those are another 28,650
    // tiles. Counting the win, an E is green on 44% of its tiles, not 8.8%, and
    // the card was sized against a fifth of its own firing rate.
    //
    // What that bought: +250 on E paid 3,633 points a guess against a 5,903
    // baseline, 404 a gold, 3.3x Glass, the best rare, and its *mean over all
    // 26 letters* still beat every other card's best letter. Strike the solving
    // guess and it measures 767 against Glass's 802, which is what an uncommon
    // ought to look like; every bit of the excess was the win, and the win is
    // the highest-leverage guess there is because the solve bonus multiplies the
    // round after it lands.
    //
    // So +125 at the rare band: 1,810 a guess, 151 a gold against Glass's 123.
    // Still the best modifier in the game, which the letter spread is the reason
    // for: e 3,633 down to q 57 at the old number, a 64x spread that the cut
    // leaves untouched. Reading the letter is still the whole decision.
    onTile: (ctx, tile) => {
      if (tile.color === "green") ctx.addChips(125)
    },
  },
  {
    id: "steel",
    pip: "×2",
    rarity: "rare",
    cost: 8,
    choiceCost: 12,
    // ×1.5 was the same card as Mult, sold a tier up and for half again the
    // price. Installed on the same board across 19,315 recorded guesses, steel
    // paid 165 points a guess on E against +4 mult's 164, 52 against 52 on S,
    // and 77 against 84 on K: a rare a common matched everywhere and beat on
    // the middling letters. The reason is that the multiply lands on a mult the
    // relics have already grown (38 by the finish, averaged), so ×1.5 was
    // arriving exactly where a flat +4 does rather than anywhere above it.
    //
    // ×2 pays 346 on those same guesses: 2.1x the common card for 1.5x its
    // price, and still comfortably beneath Glass, which is where the safe half
    // of a risk pair belongs.
    //
    // Multiplicative and per tile, so a doubled steel letter is ×4. That is the
    // whole build: steel a letter you can repeat, then find words that do.
    onTile: (ctx) => ctx.timesMult(2),
  },
  {
    id: "glass",
    pip: "×3",
    rarity: "rare",
    cost: 9,
    choiceCost: 13,
    // The risky half of the rare pair, and it has to out-pay the safe half by
    // enough that the letter is worth gambling. Sweeping the factor over the
    // same 19,315 guesses read 165 / 346 / 543 / 757 / 1237 points on E for
    // ×1.5 / ×2 / ×2.5 / ×3 / ×4, superlinear because a repeated letter
    // multiplies once per copy. ×3 is 757, comfortably clear of Steel's 346;
    // ×4 was where the card stopped reading as a rare and started reading as
    // Anchor, which the run already has one of.
    //
    // Dearer than Steel now, which it was not before: it was ×2 at $7 against
    // Steel's ×1.5 at $8, cheaper *and* stronger on every letter measured,
    // which left the pair with no decision in it at all.
    onTile: (ctx, tile) => {
      ctx.timesMult(3)
      // Only a gray tile can break it, and only when the letter is genuinely
      // absent from the answer. Gray is not proof of absence, since a second E
      // is gray when the answer holds one, and breaking a letter the answer needs
      // would leave a round that cannot be solved by anyone.
      //
      // That the break itself proves absence is the compensation for the risk:
      // a broken letter is a deduction you did not have to spend a guess on.
      if (tile.color !== "gray") return
      if (ctx.state.round.answer.includes(tile.letter)) return
      if (ctx.roll() >= GLASS_BREAK) return
      ctx.breakLetter(tile.letter)
    },
  },
]

export const MODIFIER_BY_ID = new Map(MODIFIERS.map((modifier) => [modifier.id, modifier]))

/** The modifier a letter is carrying right now, if any. */
export function modifierOf(state: RunState, letter: string): Modifier | undefined {
  const id = state.letters[letter]?.mod
  return id ? MODIFIER_BY_ID.get(id) : undefined
}

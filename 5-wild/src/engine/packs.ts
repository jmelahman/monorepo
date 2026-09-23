/**
 * Packs: the shop slot that sells a *choice* rather than a card.
 *
 * Balatro's booster packs, and here they answer a specific problem this game
 * had. The shop picks the letter for you: it rolls Steel, then rolls E, and the
 * pairing is what it is. That makes every letter modifier a bet on a letter you
 * did not choose, which is why the conditional ones had to be priced for the
 * worst letter they might land on, and why Echo needed a pool of letters it was
 * allowed to be sold on at all, rather than the number it wanted.
 *
 * A pack lays several cards out and lets you keep one. That single change is
 * what lets a card be spiky: Anchor on S is the best modifier buy in the game
 * and Anchor on H is nearly nothing, and when you are choosing between them the
 * gap is the decision instead of the tax.
 *
 * Priced off the measured uplift. A modifier dealt by the shop is worth about
 * 3.4 chips a guess averaged over every pairing the roll table can produce;
 * best-of-three is worth 6.1, a 1.77x lift, which puts a three-card pack at
 * about $9 against a $5 card. Going wider runs out of road quickly, since best
 * of four is 2.00x and best of five 2.18x, so three is where most of the choice
 * is bought, and the prices below sit a little under fair on purpose. A pack
 * should be the exciting thing on the shelf, not the correct thing.
 *
 * That priced them against value, and value turned out to be the half that was
 * already right. What the price actually decides is *when* a pack can be bought
 * at all, and on that it was doing something silly: across 3,906 recorded shop
 * visits only 37% of stage 1 visits and 36% of stage 2 visits could afford the
 * $9 relic pack, while from stage 4 on every single visit could. Gold compounds
 * through interest and these prices never moved, so the number was a wall for
 * two stages and free forever after: an unlock wearing a price tag.
 *
 * So down $2 each. The same visits could afford $7 82% of the time at stage 1
 * and 66% at stage 2, which is a decision in both rather than a card to walk
 * past while the shelf's cheap half gets bought by default. It also lands them
 * further under fair on the measurement above, which is the direction that last
 * paragraph asked for and then did not go far enough in.
 */

export type PackId = "alphabet" | "relic" | "category"

export type Pack = {
  id: PackId
  cost: number
  /** How many cards the pack lays out. */
  options: number
  /** How many of them the player keeps. */
  picks: number
}

/** Every pack lays out three and keeps one; see the note above on why three. */
const OPTIONS = 3

export const PACKS: readonly Pack[] = [
  {
    id: "alphabet",
    // The measurement above lands this at $8.9, and the affordability one takes
    // it further down than rounding would. This is the pack that exists to fix
    // the letter-pairing problem, so of the three it is the one that most has
    // to be reachable early, while the alphabet is still plain.
    cost: 6,
    options: OPTIONS,
    picks: 1,
  },
  {
    id: "relic",
    // A shade above the alphabet pack though relics average $6, because relics
    // are the only line that multiplies and choosing among three is how a run
    // actually gets the one it wants. Held down by the slot cap: the fifth
    // relic is worth much less than the first, so this does not scale away.
    //
    // Under-priced against its contents at $7, and it was under-priced at $9
    // too: over 1,656 opened relic packs the best card in the pack was a
    // legendary 26.8% of the time, a rare 42.5%, an uncommon 28.0% and a common
    // only 2.7%, for a mean shelf price of $7.87, before counting the choice,
    // which is the thing actually being sold. The shelf price is the wrong
    // ceiling for a pack; what caps this one is the tray it feeds.
    cost: 7,
    options: OPTIONS,
    picks: 1,
  },
  {
    id: "category",
    // The dearest of the three, and the only one whose choice is strategic
    // rather than tactical. The shop deals a category and you take the level it
    // offers; this hands you the shape to build toward, and a level is the one
    // purchase in the run that compounds for the rest of it.
    //
    // Kept dearest, and kept level with what one bare level costs. A pack that
    // undercut the $8 card beside it would make the shelf's own level slot the
    // wrong buy every time it appeared.
    cost: 8,
    options: OPTIONS,
    picks: 1,
  },
]

export const PACK_BY_ID: Map<string, Pack> = new Map(PACKS.map((pack) => [pack.id, pack]))

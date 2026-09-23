import { ALPHABET } from "../content/letters"
import { STAGES } from "../content/rounds"
import { difficultyOf } from "./ascensions"
import { CATEGORIES } from "./categories"
import { CONSUMABLES } from "./consumables"
import { ETCHINGS } from "./etchings"
import type { ModId, Modifier } from "./modifiers"
import { MODIFIER_BY_ID } from "./modifiers"
import type { Pack } from "./packs"
import { PACKS } from "./packs"
import { liveRanges } from "./ranges"
import type { Relic } from "./relics"
import { RELICS } from "./relics"
import type { Rng } from "./rng"
import { pick } from "./rng"
import type { Rarity, RunState, ShopItem, ShopState } from "./state"

const BASE_REROLL = 3

export const rerollCost = (shop: ShopState): number => BASE_REROLL + shop.rerolls

/** Half price, rounded down, never nothing. */
export const sellValue = (cost: number): number => Math.max(1, Math.floor(cost / 2))

/**
 * What one word-category level costs, flat: the card says the price it charges.
 *
 * Priced against the modifier line rather than in the abstract. A $4 Chip
 * modifier is worth roughly five score a gold on the guesses it lands in; a
 * level at $8 is worth about seven. Levels stay the better long-run buy, since
 * they are the only thing in the run that compounds and the rubric asked for
 * one, but not by so much that the letter slot becomes something to skip.
 */
const LEVEL_COST = 8

/**
 * What one alphabet range level costs. Priced in `ranges.ts` against the etching
 * line it sits beside; the number lives here because the shop is what charges it.
 */
const RANGE_COST = 7

/**
 * The upgrade slot: the run's permanent scaling lines. An etching raises what a
 * *kind* of letter is worth, a range level raises what a *slice of the alphabet*
 * is worth, a category level raises what a *shape of word* is worth, and a card
 * turns up often enough that the slot is not the same shape every visit.
 *
 * Etchings give up a share to make room rather than the slot growing: four kinds
 * of decision in one slot is already the most it can carry legibly, and the two
 * letter lines answer the same question, so they can afford to split a seat.
 */
const UPGRADE_TABLE = [
  "etch",
  "etch",
  "range",
  "range",
  "level",
  "level",
  "level",
  "consumable",
  "consumable",
] as const

/** The letter slot: depth, sold as a modifier on one letter. Same alternate. */
const LETTER_TABLE = ["mod", "mod", "mod", "consumable"] as const

/**
 * Which modifier a modifier slot offers. Weighted rather than uniform: the
 * build-defining ones should feel like a find, not like the default stock.
 *
 * `Modifier.rarity` cannot do this job, which is worth stating plainly because
 * the relic line next door reads as though it could. Relics draw their tier
 * first out of `rarityBag` and then a card within it; modifiers draw straight
 * out of this bag, so for a modifier `rarity` is a label the shelf paints the
 * card with and nothing else. A card is exactly as rare as its entries here.
 *
 * Ordered by what the cards measure rather than by what they are labeled,
 * because the two had drifted apart. Anchor sat at one entry, the same rate as
 * the two ×mult rares, while wearing "uncommon" and paying 3.3x the best of
 * them; that half is fixed on the card. What is fixed here is the other half:
 * the strong three are 3 in 16 now rather than 3 in 11, and the cheap reliable
 * ones fill the gap.
 *
 * Wild kept its single entry through that pass, on the grounds that dealing a
 * dominated card more often only spends more slots on it. It wanted a number
 * rather than a weight. It has since been given the number, and the entry is
 * still right for the reason it was originally: one is what an uncommon that can
 * anchor a build should be worth waiting for.
 */
const MOD_TABLE: readonly ModId[] = [
  "chip",
  "chip",
  "chip",
  "mult",
  "mult",
  "mult",
  "gold",
  "gold",
  "lucky",
  "lucky",
  "echo",
  "echo",
  "wild",
  "anchor",
  "steel",
  "glass",
]

const rollConsumable = (rng: Rng): ShopItem => {
  const card = pick(rng, CONSUMABLES)
  return { kind: "consumable", id: card.id, cost: card.cost }
}

/**
 * The letters a modifier could still be put on: ones that can be typed, that do
 * not already carry it, and that the modifier is willing to sit on at all.
 *
 * Exported because it is the same question in three places, whether the shop
 * may stock the card, whether the purchase can be honored, and which keys the
 * picker lights up, and those three must not be allowed to disagree.
 */
export function placeableLetters(state: RunState, modifier: Modifier): string[] {
  return [...(modifier.letters ?? ALPHABET)].filter(
    (letter) => !state.letters[letter]?.destroyed && state.letters[letter]?.mod !== modifier.id,
  )
}

/**
 * A modifier with no letter on it, for the player to place. Null when there is
 * nowhere left to put it, which hands the slot back to the ordinary roll rather
 * than selling a card that would do nothing.
 *
 * Which letter it lands on is most of what a modifier is worth, and the shop
 * used to roll that too, so the letter slot was a coin flip between "Steel E,
 * buy it immediately" and "Steel Q, walk past". Selling the card and letting the
 * player aim it turns the slot into a decision about their own vocabulary, which
 * is the decision this layer was always supposed to be asking for. It costs more
 * because it is worth more; see `Modifier.choiceCost`.
 */
function rollMod(state: RunState, rng: Rng): ShopItem | null {
  const modifier = MODIFIER_BY_ID.get(pick(rng, MOD_TABLE))
  if (!modifier) return null
  if (placeableLetters(state, modifier).length === 0) return null
  return { kind: "mod", id: modifier.id, cost: modifier.choiceCost }
}

/**
 * A modifier already paired with a letter, for a pack to lay out.
 *
 * The pack keeps rolling the pairing on purpose, now that the shop has stopped.
 * Two routes to the same layer that differ in what they ask of the player. The
 * shop sells the card and makes you aim it, the pack deals three aimed cards and
 * makes you choose between them, and it is the only thing that still gives
 * `Modifier.letters` a job, since a pack is where a Steel Q would otherwise turn
 * up. It is also why the pack is the cheap way in: three shots at a good pairing
 * for one price, at the cost of not picking the letter yourself.
 */
function rollPairing(state: RunState, rng: Rng): ShopItem | null {
  const modifier = MODIFIER_BY_ID.get(pick(rng, MOD_TABLE))
  if (!modifier) return null
  const candidates = placeableLetters(state, modifier)
  if (candidates.length === 0) return null
  return { kind: "mod", letter: pick(rng, candidates), id: modifier.id, cost: modifier.cost }
}

/**
 * An etching whose group still has a letter alive in it. Groups stack forever,
 * so unlike every other slot there is nothing to dedupe against: buying the
 * same etching twice is the whole idea.
 */
function rollEtch(state: RunState, rng: Rng): ShopItem | null {
  const usable = ETCHINGS.filter((etching) =>
    [...etching.letters].some((letter) => !state.letters[letter]?.destroyed),
  )
  if (usable.length === 0) return null
  const etching = pick(rng, usable)
  return { kind: "etch", id: etching.id, cost: etching.cost }
}

/**
 * A range with a letter left alive in it. Uniform across the four, and unlike
 * the category roll that is not a compromise: the ranges are cut to be worth the
 * same, so there is no rare one to lean the odds toward.
 */
function rollRange(state: RunState, rng: Rng): ShopItem | null {
  const usable = liveRanges(state)
  if (usable.length === 0) return null
  return { kind: "range", id: pick(rng, usable).id, cost: RANGE_COST }
}

function rollUpgrade(state: RunState, rng: Rng): ShopItem {
  const kind = pick(rng, UPGRADE_TABLE)
  if (kind === "etch") {
    const item = rollEtch(state, rng)
    if (item) return item
  }
  if (kind === "range") {
    const item = rollRange(state, rng)
    if (item) return item
  }
  if (kind === "level") {
    // Uniform across the categories, because the player picks the shape they
    // build toward rather than being dealt one. A rare category is harder to
    // type on purpose, not harder to find on the shelf. Balatro's model, where
    // the offer leans toward hands you have actually played, would need play
    // counts in the run state; it is the upgrade if uniform reads as noise.
    const category = pick(rng, CATEGORIES)
    return { kind: "level", id: category.id, cost: LEVEL_COST }
  }
  return rollConsumable(rng)
}

function rollLetter(state: RunState, rng: Rng): ShopItem {
  if (pick(rng, LETTER_TABLE) === "mod") {
    const item = rollMod(state, rng)
    if (item) return item
  }
  return rollConsumable(rng)
}

/** Relics already owned are off the table, since duplicates do not stack. */
const unowned = (state: RunState): readonly Relic[] => {
  const owned = new Set(state.relics.map((instance) => instance.id))
  return RELICS.filter((relic) => !owned.has(relic.id))
}

/**
 * How often each rarity fills a relic slot, at the first stage and at the last
 * authored one. In between it moves linearly; past stage `STAGES` it stays put.
 *
 * The early column is deliberately the neutral shelf: it is what drawing a card
 * uniformly out of today's catalog already deals. The ramp only ever adds, and
 * that is not timidity, it is what the bots measured. Gold compounds here through
 * interest while relic prices never move, so a shelf still reading mostly common
 * at stage 7 is a shelf the run has outgrown; that half is free, worth 3.736 mean
 * final stage against a uniform shelf's 3.724 across 2,500 recorded runs.
 *
 * Tilting the *early* half toward cheap cards is what costs, and it was tried
 * three ways. Balatro's own 70/25/5 took the mean to 3.37 and cut the wins by
 * four fifths. A gentle 38/33/21/8 still cost 0.10 of an stage and a quarter of
 * the wins. Even leaving rare and legendary untouched and moving only uncommon
 * into common cost 0.14. Five relic slots that fill by stage 2 and are never
 * sold means a cheaper shelf is a permanently weaker tray, and that is a real
 * player's habit and not only a bot's.
 *
 * It also bought almost nothing. The complaint that started this was first
 * shelves with nothing affordable on them, and a first shelf has *never* dealt
 * no relic at all: 90% of them hold one at $6 or under, and the tilt moved the
 * $8-or-nothing case from 9.6% to 8.4% while turning $6 uncommons into $4
 * commons. The affordable-relic problem is a catalog problem, since seven of
 * the twenty-three cost $8 or $10, and it wants more cheap relics, not a shop that
 * deals the existing ones more often.
 */
const RARITY_ODDS: Record<Rarity, readonly [number, number]> = {
  common: [30, 15],
  uncommon: [39, 30],
  rare: [22, 35],
  legendary: [9, 20],
}

/**
 * The weighted table for one shop visit, built as a bag to be picked from, on the
 * same trick `MOD_TABLE` plays, minus writing a hundred entries out by hand.
 *
 * Rarities with nothing left unowned are left out entirely, which is what
 * renormalises the odds: a run that has bought every common should see the other
 * three in their own proportions, not see the slot fail seven times in ten.
 */
function rarityBag(stage: number, available: ReadonlySet<Rarity>): Rarity[] {
  const through = Math.min(1, Math.max(0, (stage - 1) / (STAGES - 1)))
  const bag: Rarity[] = []
  for (const [rarity, [early, late]] of Object.entries(RARITY_ODDS)) {
    if (!available.has(rarity as Rarity)) continue
    const weight = Math.round(early + (late - early) * through)
    for (let n = 0; n < weight; n++) bag.push(rarity as Rarity)
  }
  return bag
}

/**
 * One relic out of a pool, rarity first and then uniformly within it.
 *
 * Two draws rather than one, and that is the whole point: drawing a card
 * directly makes a rarity's odds depend on how many cards happen to sit at it,
 * so adding a ninth uncommon would quietly make every other uncommon rarer.
 * Drawing the tier first means the catalog can grow anywhere without moving
 * the shelf's shape.
 */
function rollRelic(state: RunState, pool: readonly Relic[], rng: Rng): Relic | null {
  if (pool.length === 0) return null
  const bag = rarityBag(state.stage, new Set(pool.map((relic) => relic.rarity)))
  // Every rarity carries weight at every stage, so this only fires if a column is
  // ever set to zero. Uniform is the honest fallback then: an empty slot would be
  // worse than bending the odds, and the odds were never a promise not to sell.
  if (bag.length === 0) return pick(rng, pool)
  const rarity = pick(rng, bag)
  // Non-empty by construction: the rarity came out of the pool's own set.
  return pick(
    rng,
    pool.filter((relic) => relic.rarity === rarity),
  )
}

/**
 * Whether a relic could actually be taken right now: one exists to offer, and
 * there is somewhere to put it.
 *
 * The slot half matters more for packs than for the relic slots. An unbuyable
 * relic on the shelf costs nothing: the buy is refused and the gold stays put.
 * An unbuyable relic *pack* is a trap, because the gold goes when the pack
 * opens and the refusal comes three cards later.
 */
const canTakeRelic = (state: RunState): boolean =>
  state.relics.length < difficultyOf(state).relicSlots && unowned(state).length > 0

/**
 * Which pack the pack slot offers. Uniform across the three, since each one
 * points at a different line of the run and none is the fallback for another.
 *
 * The relic pack drops out when there is no relic to be had, for the same
 * reason the relic slots do: there would be nothing to lay out in it.
 */
function rollPack(state: RunState, rng: Rng): ShopItem {
  const usable = PACKS.filter((pack) => pack.id !== "relic" || canTakeRelic(state))
  const pack = pick(rng, usable.length > 0 ? usable : PACKS)
  return { kind: "pack", id: pack.id, cost: pack.cost }
}

/**
 * What a pack lays out when it is opened, rolled at open time rather than at
 * stock time so an unopened pack keeps its secret.
 *
 * Distinct cards, not distinct rolls: a pack that laid out Steel E three times
 * would be selling a choice it was not actually offering. The retry budget is
 * what makes that cheap to guarantee without a shuffle. The pools are far
 * larger than three, so a collision is rare and giving up after a few tries
 * costs a short pack rather than a hang.
 *
 * Empty is a meaningful answer, and the caller refuses the sale on it. The
 * shelf goes stale: a relic pack rolled while a slot was free is still sitting
 * there after the relic in the next slot along fills it, and opening it then
 * would spend the gold on three cards none of which could land.
 */
export function packContents(state: RunState, pack: Pack, rng: Rng): ShopItem[] {
  if (pack.id === "relic" && !canTakeRelic(state)) return []
  const out: ShopItem[] = []
  const seen = new Set<string>()
  const key = (item: ShopItem) =>
    item.kind === "mod" ? `${item.id}:${item.letter ?? ""}` : item.id

  for (let tries = 0; tries < pack.options * 6 && out.length < pack.options; tries++) {
    let item: ShopItem | null = null
    if (pack.id === "alphabet") item = rollPairing(state, rng)
    else if (pack.id === "relic") {
      // Weighted the same way the shelf is, so a relic pack is three more looks
      // at the same distribution rather than a back door to the dear ones.
      const pool = RELICS.filter((relic) => !seen.has(relic.id)).filter(
        (relic) => !state.relics.some((held) => held.id === relic.id),
      )
      const relic = rollRelic(state, pool, rng)
      item = relic ? relicItem(relic) : null
    } else {
      const category = pick(rng, CATEGORIES)
      item = { kind: "level", id: category.id, cost: LEVEL_COST }
    }
    if (!item || seen.has(key(item))) continue
    seen.add(key(item))
    out.push(item)
  }
  return out
}

const relicItem = (relic: Relic): ShopItem => ({
  kind: "relic",
  id: relic.id,
  cost: relic.cost,
})

/**
 * A fixed layout rather than four weighted rolls:
 *
 * ```
 *   slot 0   relic
 *   slot 1   relic, and the cap, never a third
 *   slot 2   upgrade: an etching group, a slice level, a category level, or a card
 *   slot 3   letter: a modifier, or a card
 *   slot 4   a pack
 * ```
 *
 * The pack gets a slot of its own rather than a share of an existing one. It is
 * the dearest thing on the shelf and the only one that asks a question back, so
 * a visit where the roll happened not to offer one would be a visit missing its
 * most interesting decision.
 *
 * Every visit now offers the same five kinds of decision. The old version rolled
 * each slot from one weighted table and could legally deal four etchings, a
 * shop with no build decision in it at all, which is what the retry loop and
 * the dedupe key dance existed to paper over. A layout that cannot deal a
 * duplicate does not need either, so both are gone.
 *
 * The two relic slots keep their fallback for the late run where every relic is
 * already owned; they fall through to what the slot beside them would have sold.
 *
 * Slot 1 is also the only one the ascension ladder is allowed to take away, and
 * it is the only one it *could*: the other four are each the sole route to a
 * layer of the run, while this one is a second look at a shelf that already has
 * one. See `Thin Shelves`.
 */
export function rollShop(state: RunState, rng: Rng, rerolls: number): ShopState {
  const pool = unowned(state)
  const first = rollRelic(state, pool, rng)
  const rest = first ? pool.filter((relic) => relic.id !== first.id) : pool
  const second = rollRelic(state, rest, rng)
  return {
    items: [
      first ? relicItem(first) : rollUpgrade(state, rng),
      second ? relicItem(second) : rollLetter(state, rng),
      rollUpgrade(state, rng),
      rollLetter(state, rng),
      rollPack(state, rng),
    ],
    rerolls,
  }
}

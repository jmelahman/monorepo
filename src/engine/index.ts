/**
 * The engine's entire public surface.
 *
 * Nothing outside this directory should reach past this file. The UI is meant
 * to be replaceable, and this is the seam it gets replaced along.
 */

export { ALPHABET, LETTER_CHIPS, MULT_FOR_COLOR } from "../content/letters"
export {
  BASE_GUESSES,
  CONSUMABLE_SLOTS,
  GOLD_PER_UNUSED_GUESS,
  INTEREST_CAP,
  INTEREST_PER,
  RELIC_SLOTS,
  ROUND_PAYOUT,
  ROUNDS_PER_STAGE,
  roundTargets,
  STAGES,
} from "../content/rounds"
export { CONTENT_VERSION } from "../content/version"
export type { Ascension, Difficulty } from "./ascensions"
export {
  ASCENSIONS,
  AUTHORED_ASCENSIONS,
  ascensionAt,
  clampAscension,
  difficultyAt,
  difficultyOf,
  MAX_ASCENSION,
  mustSolve,
  rulesFor,
} from "./ascensions"
export type { Boss, BossTier } from "./bosses"
export { BOSS_TIERS, BOSSES, bossesIn, getBoss, TIER_STAGES, tierForStage } from "./bosses"
export type { Category } from "./categories"
export {
  CATEGORIES,
  CATEGORY_BY_ID,
  categoryOf,
  isCategory,
  levelBonus,
  levelOf,
} from "./categories"
export type { Consumable } from "./consumables"
export { CONSUMABLE_BY_ID, CONSUMABLES } from "./consumables"
export type { Etching } from "./etchings"
export { ETCHING_BY_ID, ETCHINGS } from "./etchings"
export type { ModId, Modifier } from "./modifiers"
export { MODIFIER_BY_ID, MODIFIERS, modifierOf } from "./modifiers"
export type { Pack, PackId } from "./packs"
export { PACK_BY_ID, PACKS } from "./packs"
export type { Range } from "./ranges"
export {
  CHIPS_PER_LEVEL,
  liveRanges,
  RANGE_BY_ID,
  RANGES,
  rangeBonus,
  rangeChips,
  rangeLevelOf,
  rangeOf,
} from "./ranges"
export { reduce, startRun } from "./reduce"
export type { Relic, RelicCtx } from "./relics"
export { RELIC_BY_ID, RELICS } from "./relics"
export { derive } from "./rng"
export { baseChips, draftChips, solveBonusFor } from "./scoring"
export { placeableLetters, rerollCost, sellValue } from "./shop"
export type * from "./state"
export { computeFeedback, keyboardColors } from "./words"

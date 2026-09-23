import { isVowel } from "../content/letters"
import type { RunState } from "./state"

/**
 * Word categories: this game's hand types.
 *
 * Balatro scores a hand as exactly one of Pair, Two Pair, Flush, and lets Planet
 * cards level those types so a build's chosen hand compounds across a run. This
 * is the same idea over words. A guess is a Cluster or a Twin or a Distinct, and
 * leveling that category raises what every future guess of that shape is worth.
 *
 * It exists because of what the build rubric found: **nothing in this game scales
 * across a run** while round targets multiply by 2.2× per stage. Relics pay a flat
 * amount forever, modifiers pay a flat amount forever, etchings pay a flat amount
 * forever. Levels are the first thing that grows, and because they land on the
 * base, before the ×mult relics fire, a leveled category is worth more to a
 * build the more that build already multiplies.
 *
 * Level 1 is worth nothing, deliberately. A fresh run scores exactly as it did
 * before any of this existed, which keeps the whole system opt-in and makes its
 * effect on the golden vectors legible: only runs that buy levels move.
 */
export type Category = {
  id: string
  /*
   * The rule in words used to live here, and the argument for it was the best
   * of any table's: a relic says what it does on its own card, but `matches` is
   * a function, and a player who has never seen the source has no way at all to
   * find out what Cluster means. A description that drifts from the predicate it
   * describes is worse than no description.
   *
   * It went to `src/ui/lang` anyway, because the drift it was guarding against
   * is a smaller problem than the one it created: a sentence that cannot be
   * translated is not a description in any language but one. The guard that
   * replaces it is `category[id]` being a required key of `Strings`, so a shape
   * added here without a sentence written for it fails to compile. Proximity
   * was doing the work of a type; now the type does it.
   */
  /** What one level above the first adds to the base, before relics see it. */
  chips: number
  mult: number
  /**
   * Whether a word is of this shape. Read directly by the relics that were
   * written around these same predicates, and used by `categoryOf` to pick which
   * one a word scores as: two jobs, one definition, so they cannot drift apart.
   */
  matches: (word: string) => boolean
}

/**
 * Rarest first, because `categoryOf` takes the first match, so a word that is
 * both alphabetical and distinct scores as the harder of the two. Shares over
 * the answer list run roughly 2% / 7% / 20% / 20% / 51%, and the per-level
 * values are graded against exactly that: the rarer the shape, the bigger the
 * step, so no category is obviously the one to level.
 */
export const CATEGORIES: readonly Category[] = [
  {
    id: "alphabetical",
    chips: 40,
    mult: 5,
    // "" sorts below every letter, so the missing predecessor at index 0 is
    // trivially satisfied, the same thing an index guard would have said.
    matches: (word) => [...word].every((letter, i, all) => letter >= (all[i - 1] ?? "")),
  },
  {
    id: "vowel_heavy",
    chips: 32,
    mult: 4,
    matches: (word) => [...word].filter(isVowel).length >= 3,
  },
  {
    id: "cluster",
    chips: 25,
    mult: 3,
    matches: (word) => {
      let run = 0
      for (const letter of word) {
        run = isVowel(letter) ? 0 : run + 1
        if (run >= 3) return true
      }
      return false
    },
  },
  {
    id: "twinned",
    chips: 20,
    mult: 3,
    matches: (word) => new Set(word).size < word.length,
  },
  {
    // The floor, and the one a good opening probe always lands in. Cheapest step
    // per level because it is the shape you get for free by playing well.
    id: "distinct",
    chips: 15,
    mult: 2,
    matches: (word) => new Set(word).size === word.length,
  },
]

export const CATEGORY_BY_ID: Map<string, Category> = new Map(
  CATEGORIES.map((category) => [category.id, category]),
)

const DISTINCT = CATEGORIES[CATEGORIES.length - 1] as Category

/**
 * The category a word scores as.
 *
 * Total by construction: the last two entries partition every word between them
 * (a word either repeats a letter or it does not), so the loop always finds
 * something and the fallback is unreachable rather than a guess.
 */
export function categoryOf(word: string): Category {
  for (const category of CATEGORIES) {
    if (category.matches(word)) return category
  }
  return DISTINCT
}

/**
 * Whether a word has a named shape, regardless of which shape it *scores* as.
 *
 * The distinction is the reason both functions exist. A distinct word that also
 * happens to be alphabetical scores as Alphabetical, but Anagrammer still pays
 * for it, because Anagrammer asks whether the letters repeat, not which category
 * won the tie. The relics ask this one; the scoring stage asks `categoryOf`.
 */
export const isCategory = (id: string, word: string): boolean =>
  CATEGORY_BY_ID.get(id)?.matches(word) ?? false

/**
 * A category's level, where absent means one.
 *
 * Levels are stored as the level itself rather than as a count of upgrades, so
 * the number in the save is the number on the card. Absent is the default for
 * the same reason `RelicInstance.data` is absent until written: a run that never
 * buys a level adds nothing to its save file.
 */
export const levelOf = (state: RunState, id: string): number => state.levels?.[id] ?? 1

/** What a category contributes at its current level. Nothing at level one. */
export function levelBonus(
  state: RunState,
  category: Category,
): { level: number; chips: number; mult: number } {
  const level = levelOf(state, category.id)
  const steps = level - 1
  return { level, chips: category.chips * steps, mult: category.mult * steps }
}

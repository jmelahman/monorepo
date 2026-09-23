import { isVowel, LETTER_CHIPS } from "../content/letters"
import { STAGES } from "../content/rounds"
import { derive, shuffled } from "./rng"
import { keepGreens } from "./rules"
import type { GuessNote, Refusal, RoundState, RunState, Tile } from "./state"

/**
 * Boss rounds. Each one attacks a specific pole of the deduction/greed tension
 * rather than just raising the target, so the counterplay differs every time:
 * some punish information-gathering, some punish farming.
 *
 * They are banded by stage, and the band is the load-bearing part. This used to
 * be eight bosses across eight stages drawn without replacement, so a run met
 * each exactly once, a nice property and one that a ninth boss would have
 * silently deleted, turning the sequence into a random subset. Worse, it would
 * have let The Auditor, which caps the solve multiplier at ×2, land on stage 1
 * where no build exists yet to survive it.
 *
 * So the draw is now without replacement *within a band*. A run meets three of
 * the five early bosses, three of the five mid and two of the five late, never
 * the same one twice, never a late boss early, and never the same set twice
 * either, which is what the old scheme gave up in exchange for completeness.
 *
 * Because the bands are drawn short, every boss added to a band is a boss some
 * runs will not meet, and that is the argument for adding them in threes: one
 * per band keeps the odds of meeting any particular one even, and keeps a band
 * from becoming the one where you already know what is coming.
 */
export type BossTier = "early" | "mid" | "late"

/**
 * Which stages each band owns. Late runs to `STAGES` rather than to 8 so that
 * lengthening the run cannot leave an stage without a band to draw from.
 */
export const TIER_STAGES: Record<BossTier, { first: number; last: number }> = {
  early: { first: 1, last: 3 },
  mid: { first: 4, last: 6 },
  late: { first: 7, last: STAGES },
}

export type Boss = {
  id: string
  /** Which third of the run this one is allowed to show up in. */
  tier: BossTier
  /** Overrides the usual six. */
  maxGuesses?: number
  /** Rewrites feedback before it is scored and shown. */
  transform?: (tiles: Tile[]) => void
  /**
   * A line about the guess, recorded on it and shown beside the row. Asked
   * *before* `transform`, so it reads the feedback as it actually fell, which
   * is the only reason it can exist at all for a boss whose whole trick is to
   * overwrite that.
   *
   * Takes the tiles read-only for the same reason: a hook that both reported
   * the truth and edited it would make the order of the two hooks load-bearing.
   *
   * A count rather than the line the player reads. It is *recorded on the
   * guess*, so it is in every save and in every golden vector, and the two are
   * handled differently: the save reads an older string back as itself, and the
   * vectors are compared against the catalog's rendering of the count, which is
   * the same sentence they were recorded with. See `GuessRecord.note` and
   * `replay` in the vectors harness.
   */
  note?: (tiles: readonly Tile[]) => GuessNote | null
  /** Why a guess is refused, or null to allow it. */
  validate?: (word: string, round: RoundState) => Refusal | null
  /** Bends a tile's base chip value. `index` is the column it was played in. */
  tileChips?: (base: number, tile: Tile, round: RoundState, index: number) => number
  /**
   * `tileChips` reads the column, so what a letter is worth is not a fact about
   * the letter. Declared rather than inferred because the UI is what needs to
   * know: anything that prices a single letter, the key's tip most obviously,
   * has to stop quoting a number and start quoting the rule.
   */
  positional?: true
  /**
   * Letter modifiers do not fire this round. A flag rather than a hook because
   * there is nothing to compute. The layer is simply switched off, and the one
   * place that decides whether to run a modifier is the one place that reads it.
   */
  noModifiers?: true
  /**
   * `timesMult` does nothing; mult may only be added. Same reasoning as
   * `noModifiers`, and it deliberately catches everything multiplicative at
   * once, across relics, modifiers and category levels, rather than naming a source.
   */
  noTimesMult?: true
  /** Rewrites the solve multiplier, relics included. Applied last, so a cap caps. */
  solveBonus?: (base: number, round: RoundState) => number
}

export const BOSSES: readonly Boss[] = [
  {
    id: "silence",
    tier: "mid",
    /**
     * This used to say nothing at all, and it was the worst-behaved card in the
     * game, a mid-tier boss doing something harsher than anything in the late
     * band.
     *
     * Two probes (AROSE, UNLIT) across 300 fresh rounds: yellow is the dominant
     * signal at 1.17 tiles a guess against 0.38 green, so silencing it costs 35%
     * of what those probes banked: the mult base is only 1 + 1.17 + 3×0.38, and
     * a flat −1.17 is a third of it. That part is a fair price and stays.
     *
     * The part that did not was the lie. On 95% of those rounds at least one
     * letter that *is* in the word read gray, so the ordinary Wordle inference
     * that gray means gone and should never be typed again produced a wrong
     * elimination. The
     * Fog and The Mirror lie too, but invertibly: under the Fog you know a gray
     * might be a yellow, under the Mirror you know the row is backwards and can
     * turn it round. There was no undoing this one, because a gray had become
     * two different facts wearing the same color.
     *
     * The count separates them again. You learn how many of your letters are in
     * the word and not which, which is Bulls and Cows rather than Wordle: a
     * harder deduction instead of a broken one. The scoring stays as it was, and
     * that is what keeps this from collapsing into The Fog with a badge.
     */
    // Zero is the loudest reading this ever gives, since every letter not already
    // green is absent. Which is why the count goes out as a count: English says
    // that one in words rather than showing a 0, and whether another language
    // does is not this file's to decide.
    note: (tiles) => ({
      code: "misplaced",
      count: tiles.filter((tile) => tile.color === "yellow").length,
    }),
    transform: (tiles) => {
      for (const tile of tiles) {
        if (tile.color === "yellow") {
          tile.color = "gray"
          tile.shown = "gray"
        }
      }
    },
  },
  {
    id: "fog",
    tier: "early",
    // Only `shown` changes: the mult is real, the player just cannot see where
    // it came from. Punishes deduction without touching the math.
    transform: (tiles) => {
      for (const tile of tiles) {
        if (tile.shown === "yellow") tile.shown = "gray"
      }
    },
  },
  {
    id: "tyrant",
    tier: "mid",
    // The same sentence ascension 5 imposes, and literally the same function, so
    // the two can never come to mean slightly different things.
    validate: keepGreens,
  },
  {
    id: "miser",
    tier: "late",
    // The sharpest of the set: it forbids the repeat-letter probing that good
    // deduction leans on, so a scoring build has to carry the round.
    tileChips: (base, tile, round) => {
      const spent = round.guesses.some((g) => g.word.includes(tile.letter))
      return spent ? 0 : base
    },
  },
  {
    id: "clock",
    tier: "mid",
    maxGuesses: 4,
  },
  {
    id: "glutton",
    tier: "early",
    validate: (word) => {
      const vowels = [...word].filter(isVowel).length
      return vowels >= 2 ? null : { code: "needs_two_vowels" }
    },
  },
  {
    id: "auditor",
    tier: "late",
    // Every other round can be won by banking a modest pile and cashing it in
    // at ×5 or ×6. This one takes the cash-out away and asks whether the build
    // can actually reach the target on its own, which is the question the solve
    // bonus otherwise lets you avoid answering all run.
    solveBonus: (base) => Math.min(2, base),
  },
  {
    id: "purist",
    tier: "early",
    // Aimed at the fat scoring words. JAZZY, FUZZY and MUMMY are all chips and no
    // information, and all built on a doubled letter. Deduction barely notices;
    // a chip build loses its best line. The answer pool is filtered by this
    // same rule, so the word is always reachable.
    validate: (word) =>
      new Set(word).size === word.length ? null : { code: "no_repeated_letters" },
  },
  {
    id: "drought",
    tier: "early",
    // The Glutton's opposite number, and deliberately in the same band: one
    // demands vowels, the other refuses to pay for them. A build tuned for
    // either is soft to the other, which is what a band of four is for.
    tileChips: (base, tile) => (isVowel(tile.letter) ? 0 : base),
  },
  {
    id: "mirror",
    tier: "mid",
    // The Fog's trick at a longer range: `shown` is reversed and `color` is not,
    // so every point of mult is real and every position you read off the board
    // is a lie. Costs a scoring build nothing and a deducing build everything.
    transform: (tiles) => {
      const shown = tiles.map((tile) => tile.shown)
      tiles.forEach((tile, i) => {
        const flipped = shown[tiles.length - 1 - i]
        if (flipped) tile.shown = flipped
      })
    },
  },
  {
    id: "famine",
    tier: "late",
    // The Clock, late and meant it. Three guesses is barely a deduction at all,
    // so this is the round that asks whether the build can simply out-score the
    // target, and it hands you a ×4 solve multiplier if you can do it at once.
    maxGuesses: 3,
  },
  {
    id: "rust",
    tier: "late",
    // Aimed squarely at the permanent upgrade line: a run that bought four
    // etchings meets a round where none of them exist. It reads `LETTER_CHIPS`
    // rather than subtracting them, so it stays correct if the upgrade rules
    // ever change: the claim is "what the letter started as", not "minus what
    // you added". Which is why it caught alphabet range levels for free, and
    // why the text says upgrades rather than naming either line.
    tileChips: (_base, tile) => LETTER_CHIPS[tile.letter] ?? 0,
  },
  {
    id: "margin",
    tier: "early",
    // The first boss that cares *where* a letter was played, which is a pole
    // nothing else in the set attacks. Every other chip boss asks what the
    // letter is or whether it has been spent. The columns are the expensive
    // ones: 2.34 and 2.15 mean chips against 1.48–1.69 for the middle three, so
    // this takes 4.49 of an average word's 9.30. Nearly half, and all of it
    // recoverable by a player who moves the heavy letters inward, which is the
    // counterplay: an early boss should teach a habit rather than tax one.
    //
    // ×0.85 on the mean round score over 250 seeds, which lands it exactly on
    // The Drought in the same band. Two early bosses that each cost a sixth of
    // the pile by refusing to pay for a different thing is the shape that band
    // is for.
    positional: true,
    tileChips: (base, _tile, round, index) =>
      index === 0 || index === round.answer.length - 1 ? 0 : base,
  },
  {
    id: "vandal",
    tier: "mid",
    // The etching line has The Rust; the modifier line had nothing, which left
    // the layer the run spends most of its shop money on unattackable. This is
    // the counterpart, and mid rather than late on purpose: by stage 4 a run has
    // two or three modifiers placed and the loss is felt, but it is not yet the
    // whole build, so the round reads as a setback instead of a wall.
    //
    // The bite scales with what the run actually bought, which is the property
    // worth having. Over 250 seeds it costs a board with one modifier ×0.60, two
    // ×0.52 and three ×0.45, and a run that placed nothing does not notice it at
    // all. No other boss prices itself off the player's own investment, and it
    // is why this one can hit as hard as a late boss without landing like one.
    noModifiers: true,
  },
  {
    id: "plateau",
    tier: "late",
    // The late band is where builds are finished, and a finished build wins by
    // multiplying, whether by Anagrammer, Speedrunner, The Chorus, a leveled category or a
    // ×mult etching. Nothing in the game attacked that side, so the answer to
    // every late boss was the same stack. This one asks the opposite question:
    // what does the build score when only the flat half fires? A run that bought
    // Masochist and Sunk Cost walks through it, which is the whole idea.
    //
    // ×0.51 over 250 seeds against a tray holding one ×mult relic, which puts it
    // level with The Auditor, the other late boss that takes a multiplier away,
    // and they take different ones, so a build cannot be safe from both.
    noTimesMult: true,
  },
]

const BY_ID = new Map(BOSSES.map((boss) => [boss.id, boss]))

export const getBoss = (id: string | null): Boss | undefined =>
  id === null ? undefined : BY_ID.get(id)

/** The band an stage draws from. Anything past the last band stays in it. */
export function tierForStage(stage: number): BossTier {
  if (stage <= TIER_STAGES.early.last) return "early"
  if (stage <= TIER_STAGES.mid.last) return "mid"
  return "late"
}

/** The bands in the order a run meets them, for anything that lists all three. */
export const BOSS_TIERS: readonly BossTier[] = ["early", "mid", "late"]

export const bossesIn = (tier: BossTier): readonly Boss[] =>
  BOSSES.filter((boss) => boss.tier === tier)

/**
 * Draw without replacement *within the stage's band*, so a run never meets the
 * same boss twice and never meets a late boss early.
 *
 * Each band gets its own RNG stream, keyed by the band name. Deriving a whole
 * band's order up front, rather than picking one boss per stage, keeps the
 * sequence stable if stages are ever skipped or replayed, and keying by name
 * rather than by index means adding a band cannot reshuffle the others.
 */
export function bossForStage(state: RunState): string {
  const tier = tierForStage(state.stage)
  const order = shuffled(derive(state.seed, "bosses", tier), bossesIn(tier))
  // The offset within the band, so stage 4 takes the mid band's first boss
  // rather than its fourth. Modulo covers a band with fewer bosses than stages.
  const boss = order[(state.stage - TIER_STAGES[tier].first) % order.length]
  if (!boss) throw new Error(`no bosses in tier ${tier}`)
  return boss.id
}

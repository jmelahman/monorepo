import { ALPHABET, LETTER_CHIPS, MIN_LIVE_LETTERS, MULT_FOR_COLOR } from "../content/letters"
import { getBoss } from "./bosses"
import { categoryOf, levelBonus } from "./categories"
import type { ModCtx } from "./modifiers"
import { modifierOf } from "./modifiers"
import { rangeChips } from "./ranges"
import { RELIC_BY_ID } from "./relics"
import type { Rng } from "./rng"
import { derive } from "./rng"
import type { GameEvent, Payout, RunState, Tile, TileScore } from "./state"

/**
 * The scoring pipeline.
 *
 *   per tile, left to right:  base chips + color mult, then the letter's own
 *                             modifier, then every relic's onTile hook in slot
 *                             order
 *   after the tiles:          the word's category, at whatever level it holds
 *   then:                     every relic's onGuess hook in slot order
 *   finally:                  the solve bonus
 *
 * The modifier goes before the relics because the letter is what was played and
 * the relics are what watched it, and because a ×mult modifier landing before
 * the relics' flat mult is the weaker, more governable half of that ordering.
 *
 * Relics get real code rather than a data-driven effect DSL, because a DSL
 * always hits a wall around the tenth relic. What they get instead is a narrow
 * mutable context and a fixed firing order, which keeps them deterministic and
 * individually testable.
 *
 * The left-to-right tile cadence is not incidental: it is the animation, so the
 * event log doubles as the storyboard.
 */

export type ScoreCtx = {
  readonly state: RunState
  readonly word: string
  readonly tiles: readonly Tile[]
  /** 0-based index of this guess within the round. */
  readonly guessIndex: number
  /** Guesses that would remain after this one. */
  readonly guessesLeft: number
  readonly solved: boolean
  chips: number
  mult: number
  gold: number
  addChips(amount: number): void
  addMult(amount: number): void
  timesMult(factor: number): void
  addGold(amount: number): void
  /** The chip value a letter is worth right now, etchings included. */
  baseChipsOf(letter: string): number
  /**
   * A seeded roll for whatever is currently firing, in [0, 1). Chance effects
   * have to replay identically from a save and from a golden vector, so this is
   * the only randomness an effect may consult. Each hook gets its own stream,
   * keyed by where it fired rather than by the order things ran in.
   */
  roll(): number
  /**
   * This relic's own persistent counter, 0 if it has never written one. Reads
   * what the relic has grown to *including* anything written earlier in this
   * same guess.
   */
  getData(key: string): number
  /**
   * Grow this relic. Collected rather than applied: scoring prices a guess, it
   * does not edit the run, the same rule that puts `broken` in the result
   * instead of mutating `state.letters` here. The caller commits it.
   */
  setData(key: string, value: number): void
}

export type ScoreResult = {
  chips: number
  mult: number
  solveBonus: number
  score: number
  gold: number
  /**
   * What each tile paid, column by column, so the row can still say it once the
   * animation has gone by. Collected here rather than read back off the events
   * because it is the same two numbers the events are built from, and a second
   * reader of that stream would be a second thing to keep in step with it.
   */
  paid: TileScore[]
  /**
   * Letters a breaking modifier retired. Applied by the caller rather than
   * here: scoring prices a guess, it does not edit the run.
   */
  broken: string[]
  /**
   * Relics that grew this guess, by slot. Same discipline as `broken`: only
   * the slots that actually wrote appear, so committing this never plants an
   * empty `data` on a relic that does not scale.
   */
  relicData: Array<{ slot: number; data: Record<string, number> }>
}

/**
 * What a letter is worth before anything watches it: what it started as, plus
 * every etching bought on a group containing it, plus its alphabet range's
 * level. The two upgrade lines crosscut deliberately, so they add rather than
 * compete, so an etched E in a leveled A–E collects both.
 */
export function baseChips(state: RunState, letter: string): number {
  return (
    (LETTER_CHIPS[letter] ?? 0) + (state.letters[letter]?.etch ?? 0) + rangeChips(state, letter)
  )
}

/**
 * What the letters typed so far are worth in chips, as the boss prices them.
 *
 * The board shows this while the word is still being typed, which is the only
 * moment it can change what the player types, so it lives here beside the rule
 * rather than being re-derived in the view, for the reason `solveBonusFor` does:
 * a readout that disagreed with the scoring would be worse than no readout.
 *
 * Chips only, and only the tiles' own. Two things are left out and they are left
 * out for opposite reasons.
 *
 * Mult is left out because it cannot be known. Color is the whole of it, and
 * color is precisely what the player is typing the word to find out, which is
 * why this returns chips rather than a score, and why the board shows a
 * placeholder there instead of a number.
 *
 * Modifiers, relic `onTile` hooks and the category bonus are left out because
 * knowing them costs more than it pays. Most read the tile's color, so a dry
 * run would have to invent one; the chance-based ones draw from a stream keyed
 * to the position they will really fire at, so a dry run would not merely guess
 * a coin flip, it would *report* it, handing the player an outcome before they
 * committed to the guess. The category bonus is knowable once the word is whole,
 * and is deliberately still absent: the line directly above the readout already
 * names the shape and prints its bonus, and two places showing the same number
 * is one place too many.
 *
 * So the figure is a floor, the same promise `solveHint` makes: nothing in the
 * pipeline subtracts chips, so the guess can only beat this. The boss part of it
 * is exact: the hook is asked at gray, and none of the four bosses that bend
 * chips reads the color: The Miser goes by whether the letter has been spent,
 * The Drought by whether it is a vowel, The Rust by what the letter started as,
 * The Margin by which column it landed in. One that paid less on gray would
 * loosen the floor without breaking it, which is the direction a promise about
 * an unearned score should fail in.
 *
 * The column is exact for the same structural reason: a draft is typed left to
 * right, so a letter's index in the draft is the index it will be scored at.
 * That holds for a partial word too. The fifth column simply has nothing in it
 * yet, so The Margin zeroes the first letter immediately and the last only once
 * it exists, which is the honest reading of a word that is not finished.
 */
export function draftChips(state: RunState, draft: string): number {
  const boss = getBoss(state.round.bossId)
  let chips = 0
  for (const [index, letter] of [...draft].entries()) {
    const base = baseChips(state, letter)
    chips += boss?.tileChips
      ? boss.tileChips(base, { letter, color: "gray", shown: "gray" }, state.round, index)
      : base
  }
  return chips
}

/**
 * What solving right now would multiply the round's pile by.
 *
 * Deliberately a pure function of the state rather than something assembled
 * mid-pipeline: the board shows this figure *before* the guess is submitted, and
 * a readout that disagreed with the rule would be worse than no readout at all.
 * So relics touch it through their own hook instead of through `ScoreCtx`. A
 * bonus that could only be known by scoring a guess could not be predicted.
 *
 * The boss goes last so a cap really caps.
 *
 * @param guessesLeft guesses that would remain *after* the solving guess.
 */
export function solveBonusFor(state: RunState, guessesLeft: number): number {
  let bonus = 1 + guessesLeft
  for (const instance of state.relics) {
    bonus += RELIC_BY_ID.get(instance.id)?.solveBonus?.(state) ?? 0
  }
  const boss = getBoss(state.round.bossId)
  return boss?.solveBonus ? boss.solveBonus(bonus, state.round) : bonus
}

export function scoreGuess(params: {
  state: RunState
  tiles: readonly Tile[]
  word: string
  guessIndex: number
  guessesLeft: number
  solved: boolean
  events: GameEvent[]
}): ScoreResult {
  const { state, tiles, word, guessIndex, guessesLeft, solved, events } = params
  const round = state.round
  const boss = getBoss(round.bossId)

  /**
   * Whoever is currently firing gets to narrate what it did. Set around each
   * hook call, so an effect only has to report `+4 mult` as the shape it is and
   * the event it lands in, whether a relic card lighting up or the tile whose
   * letter carried a modifier, is decided by the caller.
   */
  let firing: ((paid: Payout) => void) | null = null
  const fire = (paid: Payout) => firing?.(paid)

  /** The firing hook's own seeded stream, replaced around each hook call. */
  let roll: Rng = () => 0

  /** The Plateau, read once rather than per call. */
  const blockTimesMult = boss?.noTimesMult ?? false

  const broken: string[] = []

  /**
   * Which relic slot is firing, so `setData` knows whose counter it is writing.
   * Null while a modifier or the tile loop itself is running, since a letter has no
   * slot to grow in.
   */
  let slotFiring: number | null = null
  const grown = new Map<number, Record<string, number>>()

  /** Copy-on-write, so a slot only lands in the result once it actually grows. */
  const bucketFor = (slot: number): Record<string, number> => {
    const existing = grown.get(slot)
    if (existing) return existing
    const fresh = { ...(state.relics[slot]?.data ?? {}) }
    grown.set(slot, fresh)
    return fresh
  }

  const ctx: ModCtx = {
    state,
    word,
    tiles,
    guessIndex,
    guessesLeft,
    solved,
    chips: 0,
    mult: 1,
    gold: 0,
    baseChipsOf: (letter) => baseChips(state, letter),
    addChips(amount) {
      ctx.chips += amount
      fire({ kind: "chips", amount })
    },
    addMult(amount) {
      ctx.mult += amount
      fire({ kind: "mult", amount })
    },
    timesMult(factor) {
      // The Plateau. Swallowed here rather than at each caller so that every
      // multiplicative effect in the game, whether relic, modifier or category
      // level, is covered by construction, including ones written after the boss was.
      //
      // It still narrates, and narrates the truth: a card that lit up saying
      // "×3 mult" while the total did not move would read as a bug, and the
      // player needs to see *which* of their cards the round is eating.
      if (blockTimesMult) {
        fire({ kind: "blocked" })
        return
      }
      ctx.mult *= factor
      fire({ kind: "times", factor })
    },
    addGold(amount) {
      ctx.gold += amount
      fire({ kind: "gold", amount })
    },
    roll: () => roll(),
    getData(key) {
      if (slotFiring === null) return 0
      // Reads through to what this guess has already written, so a relic that
      // grows and then spends its own counter in the same guess sees the new
      // value rather than a stale one.
      const bucket = grown.get(slotFiring)
      if (bucket) return bucket[key] ?? 0
      return state.relics[slotFiring]?.data?.[key] ?? 0
    },
    setData(key, value) {
      if (slotFiring === null) return
      bucketFor(slotFiring)[key] = value
    },
    breakLetter(letter) {
      if (broken.includes(letter)) return
      // Counted against what the alphabet *will* be, so a guess that breaks two
      // letters cannot walk past the floor one letter at a time.
      const live = [...ALPHABET].filter(
        (name) => !state.letters[name]?.destroyed && !broken.includes(name),
      )
      if (live.length < MIN_LIVE_LETTERS) return
      broken.push(letter)
    },
  }

  const relics = state.relics.map((instance) => RELIC_BY_ID.get(instance.id))

  /** The row's own record of itself, filled in as the cadence walks across it. */
  const paid: TileScore[] = []

  tiles.forEach((tile, index) => {
    // Where the row stood before this column touched it. The two totals are read
    // again once the modifier and the relics have had it, and the difference is
    // what the column was worth; see `TileScore`.
    const opened = { chips: ctx.chips, mult: ctx.mult }

    const base = boss?.tileChips
      ? boss.tileChips(baseChips(state, tile.letter), tile, round, index)
      : baseChips(state, tile.letter)

    ctx.chips += base
    ctx.mult += MULT_FOR_COLOR[tile.color]
    events.push({ type: "tile", index, gained: base, chips: ctx.chips, mult: ctx.mult })
    // The color is deliberately not written down beside the chips: it is on the
    // tile already, and a boss that rewrites what a row *shows* leaves `color`
    // and `shown` disagreeing. Whoever explains this later has to choose between
    // them, and copying one of them here would quietly make that choice for them.
    const record: TileScore = { base, chips: 0, mult: 0 }
    paid.push(record)

    // The Vandal switches the whole layer off. Asked here rather than inside
    // each modifier so the tile emits no `mod` event at all: the letter still
    // wears its badge on the board, and the row it scores in stays silent about
    // it, which is the reading the boss's one line promises.
    const modifier = boss?.noModifiers ? undefined : modifierOf(state, tile.letter)
    if (modifier) {
      // One stream per tile of one guess of one round, so a chance effect is a
      // property of the position it happened at rather than of the order things
      // were drawn in, which is what lets a run be replayed from its seed.
      roll = derive(state.seed, "mod", state.stage, state.roundIndex, guessIndex, index)
      firing = (paid) => {
        events.push({
          type: "mod",
          index,
          letter: tile.letter,
          id: modifier.id,
          paid,
          chips: ctx.chips,
          mult: ctx.mult,
        })
        // What it paid rather than the numbers it moved, because this is what the
        // tile said out loud and the whole point of keeping it is that the row
        // goes on saying it. A modifier that scored twice on one tile would leave
        // only the last thing it said. None do, and the first one that does will
        // want a sentence written for it rather than two payouts concatenated by
        // accident.
        record.mod = paid
      }
      modifier.onTile(ctx, tile)
      firing = null
    }

    relics.forEach((relic, slot) => {
      if (!relic?.onTile) return
      // One stream per slot per tile, so a chance effect is a property of where
      // it fired rather than of the order the slots happened to run in, on the
      // same rule the modifier stream above follows.
      //
      // The salt still says "joker" on purpose: it is a coordinate, not a name,
      // and changing it reshuffles every seed. See `derive` in `rng.ts`.
      roll = derive(state.seed, "joker", state.stage, state.roundIndex, guessIndex, slot, index)
      slotFiring = slot
      firing = (paid) => {
        events.push({ type: "relic", slot, id: relic.id, paid, chips: ctx.chips, mult: ctx.mult })
        // Appended rather than assigned, which is where this parts company with
        // the modifier above. A letter carries one card; the tray carries five,
        // and a green Q pays Green Thumb and Q's Bargain both, and a row that kept
        // only the last of them would answer a narrower question than the one
        // being asked of it. Two firings from the same slot append twice for the
        // same reason: that is what the tile did.
        record.relics = [...(record.relics ?? []), { id: relic.id, label: paid }]
      }
      relic.onTile(ctx, tile, index, base)
      firing = null
      slotFiring = null
    })

    record.chips = ctx.chips - opened.chips
    record.mult = ctx.mult - opened.mult
  })

  // The word's category, after the tiles and before the relics. That position is
  // the whole point: a level raises the *base*, so every ×mult relic downstream
  // multiplies it. Leveling and multiplying compound instead of competing,
  // which is what makes a leveled category a build rather than a bonus.
  const category = categoryOf(word)
  const bonus = levelBonus(state, category)
  if (bonus.chips > 0 || bonus.mult > 0) {
    ctx.chips += bonus.chips
    ctx.mult += bonus.mult
    events.push({
      type: "category",
      id: category.id,
      level: bonus.level,
      chips: ctx.chips,
      mult: ctx.mult,
    })
  }

  relics.forEach((relic, slot) => {
    if (!relic?.onGuess) return
    // One coordinate shorter than the per-tile stream above, so the two cannot
    // collide even for the same slot and guess. Same frozen salt, same reason.
    roll = derive(state.seed, "joker", state.stage, state.roundIndex, guessIndex, slot)
    slotFiring = slot
    firing = (paid) =>
      events.push({ type: "relic", slot, id: relic.id, paid, chips: ctx.chips, mult: ctx.mult })
    relic.onGuess(ctx)
    firing = null
    slotFiring = null
  })

  // Solving pays tempo and ends the round on the spot, and its bonus multiplies
  // everything banked this round rather than the guess that happened to land it.
  // So the two lines finally compose: farm the board up, then cash the whole
  // pile in at once. Deciding *when* is the game.
  //
  // The multiply itself belongs to the caller, which owns the running total,
  // this only prices it.
  const solveBonus = solved ? solveBonusFor(state, guessesLeft) : 1

  return {
    chips: ctx.chips,
    mult: ctx.mult,
    solveBonus,
    score: Math.round(ctx.chips * ctx.mult),
    gold: ctx.gold,
    paid,
    broken,
    relicData: [...grown].map(([slot, data]) => ({ slot, data })),
  }
}

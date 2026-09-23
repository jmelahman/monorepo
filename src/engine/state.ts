/**
 * Every type the engine speaks. Deliberately all plain data: a RunState round
 * trips through JSON with nothing lost, which is what makes save/resume and the
 * golden vectors cheap. Note what is *absent*: no PRNG state. Streams are
 * re-derived from the seed plus a coordinate, so there is nothing to serialize.
 */

import type { ModId } from "./modifiers"

export type Color = "green" | "yellow" | "gray"

export type Tile = {
  letter: string
  /** Drives scoring. */
  color: Color
  /** Drives the screen. Differs from `color` only when a boss lies. */
  shown: Color
  /**
   * Set on the one tile The Magician colored, which is the only color in the
   * game that was granted rather than earned. Both other fields say yellow and
   * both are telling the truth about what they drive: it scores as a yellow and
   * it is drawn as one. What it is *not* is evidence, and everything that reads
   * a played row as evidence about the answer — `rules.ts`, the keyboard — has
   * to skip it, because the letter may well not be in the word at all.
   *
   * Absent rather than `false` when the tile earned its color, so a save written
   * before the flag existed loads as what it was; see `rules.ts` for why the
   * distinction is load-bearing rather than tidy.
   */
  promoted?: true
}

/**
 * What one tile paid, written down as it was scored.
 *
 * Recorded rather than re-derived, for the same reason `note` below is: by the
 * time anyone asks, the arithmetic no longer exists. `baseChips` still knows
 * what a letter is worth, but The Miser prices a letter by whether it has
 * already been spent, so re-running it against the round as it now stands
 * reports no chips for the very guess that first spent the letter, and the row
 * would explain itself with a number it never scored. The modifier is worse
 * still: Lucky is a seeded quarter chance, and the only honest answer to "did it
 * fire" is the one it gave at the time.
 */
export type TileScore = {
  /**
   * The tile's own chips, boss included, before its modifier touched them,
   * which is exactly the figure the tile floats as it turns over.
   */
  base: number
  /**
   * What the column moved the row's two numbers by: everything from `base`
   * through the modifier and the relics, measured as the running totals before
   * and after. Named to match `GuessRecord`'s own `chips` and `mult` because
   * that is what they are a piece of: the tip sets one against the other and
   * says "9 of 27".
   *
   * A difference rather than a sum, so a multiplicative card lands somewhere
   * honest: Steel on the fourth tile of a row already at ×7 records 7, because
   * that is what the row gained where it fired. The same Steel on the first tile
   * records 1. Both are true and neither is the card's "worth" in the abstract,
   * which is a number this game does not have, since every ×2 is worth whatever was
   * standing in front of it.
   */
  chips: number
  mult: number
  /**
   * What the letter's modifier did here. Absent when the letter carries none,
   * when a boss silenced the layer, and when a chance modifier rolled and lost:
   * three different silences, which the view tells apart by asking the letter
   * what it carries.
   *
   * The `string` arm is a save written before this was a `Payout`, where the
   * field held the finished sentence. Kept rather than bumping the save key,
   * because what is at stake is one line of one tip on rows already played: a
   * run refused would be a worse answer than a run whose open round explains its
   * older tiles in the language they were scored in. It empties itself: no code
   * writes a string here any more, so the arm is gone from every save by the end
   * of the round that upgraded.
   */
  mod?: Payout | string
  /**
   * The relics that paid on this tile, in the slot order they fired in, each
   * with the words it used. Only the `onTile` half of the tray: an `onGuess`
   * relic fires once for the whole row and belongs to no letter in particular,
   * so hanging it on all five would invent an attribution the pipeline does not
   * make.
   *
   * Absent rather than empty when nothing fired, which is the ordinary case.
   * Most tiles are gray and most relics want something of the tile. A relic that
   * was asked and declined leaves no trace on purpose: the modifier's silence is
   * worth reporting because the modifier is stuck to this letter, but a tray of
   * five would otherwise print five lines of "nothing" under every tile on the
   * board.
   *
   * `label` keeps its name now that it holds a `Payout` rather than the words it
   * was named for, on the same reasoning as `mod` above and for the same key: a
   * rename reads as absent, and absent here is a tip that has forgotten which
   * cards paid on the tile it is explaining.
   */
  relics?: Array<{ id: string; label: Payout | string }>
}

export type GuessRecord = {
  word: string
  tiles: Tile[]
  /**
   * Column by column, what each tile paid; parallel to `tiles`.
   *
   * Optional, so a save written before rows kept their arithmetic loads as what
   * it is, a row that cannot say how it was scored, which is what those rows
   * were. Written on every guess since, so the gap closes within a round.
   */
  paid?: TileScore[]
  chips: number
  mult: number
  /**
   * ×(1 + guesses left) when this guess solved the word, else 1. It is recorded
   * here but *not* folded into `score`: the bonus multiplies the round's total,
   * so it belongs to the round rather than to any one guess.
   */
  solveBonus: number
  /** chips × mult for this guess alone. */
  score: number
  /**
   * What the boss will say about this guess instead of showing it: The
   * Silence's count of misplaced letters, and nothing else so far.
   *
   * Written at submit rather than derived on demand, because the thing it
   * describes no longer exists: the boss's `transform` overwrites `tile.color`,
   * so by the time a view could ask, the truth it would have to count is gone.
   * Optional, so every save and every vector written before it existed reads
   * back unchanged.
   *
   * The `string` arm is a save from before this was a count, on the same footing
   * as `TileScore.mod`: the sentence outlives the round it was written in and
   * nothing writes another.
   */
  note?: GuessNote | string
}

export type Rarity = "common" | "uncommon" | "rare" | "legendary"

/**
 * Where a scaling relic keeps what it has grown.
 *
 * `data` is absent until the relic actually writes to it, which is what keeps
 * this compatible in both directions: a save written before scaling existed
 * loads unchanged, and a run full of non-scaling relics adds nothing to the
 * file. Plain numbers only, for the same reason the rest of RunState is plain.
 */
export type RelicInstance = { id: string; data?: Record<string, number> }
export type ConsumableInstance = { id: string }

export type LetterState = {
  /** Extra chips from etchings, added to the base value. */
  etch: number
  /** Removed from the alphabet: cannot be typed, cannot appear in an answer. */
  destroyed: boolean
  /**
   * The modifier stuck to this letter, if any. One at a time, since buying a
   * second replaces the first, and it outlives being etched or broken.
   */
  mod: ModId | null
}

/** 0 normal, 1 elite, 2 boss. */
export type RoundIndex = 0 | 1 | 2

export type RoundState = {
  answer: string
  target: number
  maxGuesses: number
  bossId: string | null
  /** Letters typed but not yet submitted. */
  draft: string
  guesses: GuessRecord[]
  score: number
  solved: boolean
  /** No more guesses will be accepted; the outcome is already decided. */
  done: boolean

  /** The Oracle: positions whose letter has been revealed early. */
  revealed: (string | null)[]
  /** The Hermit: letters proven absent without spending a guess. */
  eliminated: string[]
  /** The Magician: promote this guess's first gray to yellow. */
  promote: boolean
}

export type ShopItem =
  | { kind: "relic"; id: string; cost: number }
  | { kind: "consumable"; id: string; cost: number }
  /** A group etching. Keyed by the group, not by a letter, since it buys many. */
  | { kind: "etch"; id: string; cost: number }
  /** One level for a named word category. */
  | { kind: "level"; id: string; cost: number }
  /** One level for a slice of the alphabet. */
  | { kind: "range"; id: string; cost: number }
  /**
   * A letter modifier. The shop sells it unattached and the player points it at
   * a letter; a pack lays out a specific pairing, letter printed on the card.
   *
   * `letter` is what tells those apart, and it is optional rather than nullable
   * so a save written before the shop stopped rolling the letter still loads as
   * what it was, a card with a letter already on it, which is exactly how the
   * pack version still works.
   */
  | { kind: "mod"; id: ModId; cost: number; letter?: string }
  /**
   * A booster pack. The only item that is not applied when it is bought. It
   * opens instead, and what comes out of it is chosen rather than dealt.
   */
  | { kind: "pack"; id: string; cost: number }

export type ShopState = {
  /** Slots go null once bought, so the layout does not reflow under the thumb. */
  items: (ShopItem | null)[]
  rerolls: number
}

/**
 * A pack laid out on the table, mid-decision.
 *
 * The options are ordinary `ShopItem`s and they keep the price they would have
 * carried in the stock, which is not charged, since the pack was paid for already.
 * Carrying it anyway is what lets the screen say what a card is worth, and it
 * means one function applies an item to the run whether it was bought or won.
 */
export type OpenPack = {
  id: string
  /** Cards laid out; a slot goes null once its card has been taken. */
  options: (ShopItem | null)[]
  /** Picks still owed. The pack closes when this reaches zero. */
  picks: number
}

export type Phase =
  | "round"
  /** Round cleared; waiting for the player to bank the reward. */
  | "reward"
  | "shop"
  | "game_over"
  /**
   * The final stage's last round is banked. Not a terminus: the run is complete
   * and the player chooses whether it is over, so this phase is a held screen
   * rather than a stopped machine. `continue_run` releases it into the shop.
   */
  | "victory"

export type RunState = {
  seed: number
  stage: number
  roundIndex: RoundIndex
  phase: Phase
  gold: number
  relics: RelicInstance[]
  consumables: ConsumableInstance[]
  letters: Record<string, LetterState>
  /**
   * Word category levels, by category id, where absent means level one. Optional
   * and unwritten until a level is bought, on the same reasoning as
   * `RelicInstance.data`: a run that never levels anything costs its save
   * nothing, and a save from before leveling existed loads unchanged.
   */
  levels?: Record<string, number>
  /**
   * Alphabet range levels, by range id, absent meaning level one. Same shape and
   * same reasoning as `levels`. The letters themselves stay in `letters`, since
   * a range level is a property of the slice rather than of any letter in it.
   */
  ranges?: Record<string, number>
  /**
   * Whether this run has already cleared the final stage.
   *
   * Set once and never cleared, which is what makes it more than a phase: it is
   * why the win is only offered once however far past stage `STAGES` the run goes,
   * and it is how a run that wins and then dies at stage 14 is told apart from
   * one that simply died. Optional, so a save written before endless existed
   * loads as a run that has not won, which is what it is.
   */
  won?: boolean
  /**
   * The difficulty this run was started at, absent meaning zero, the ordinary
   * game. Chosen once and never changed: an ascension is the terms the whole run
   * is played under, and a run that could be turned down halfway is not one.
   */
  ascension?: number
  /**
   * Every word this run has submitted, in order, across all its rounds.
   *
   * Ascension 9 forbids repeating one, and there is nowhere else that fact could
   * live, since a round only knows its own guesses. Written on every submit whatever
   * the ascension, because a rule that only records when it is switched on is a
   * rule that cannot be switched on. Optional, so older saves load as a run that
   * has not guessed anything yet, which costs those runs nothing: the rule that
   * reads it is not in play on a run started before it existed.
   */
  history?: string[]
  round: RoundState
  shop: ShopState | null
  /**
   * The pack currently open, if one is. Absent rather than null when there is
   * none, on the same reasoning as `levels` and `ranges`: a save written before
   * packs existed loads unchanged, and a shop visit that buys none adds nothing
   * to the file.
   *
   * While this is set the shop is held: nothing else can be bought, rerolled or
   * left behind until the pack is resolved. A pack is a decision, and letting
   * one sit open while the stock changed underneath it would make the pack's
   * own cards stale.
   */
  pack?: OpenPack | null
  /**
   * A modifier bought and not yet pointed at a letter.
   *
   * The other half of the pack's shape: the gold is gone, the card is the
   * player's, and the run is held until they say where it goes. Only the id is
   * kept. Which letters are still legal for it is a question about the alphabet
   * right now, and the alphabet is already in the state.
   *
   * Optional rather than nullable for the same reason `pack` is: a save from
   * before the shop sold choice has nothing to say here, and nothing to say is
   * the right answer.
   */
  placing?: ModId | null
  /** Set when a round is cleared, so the reward screen can itemise it. */
  reward: RewardBreakdown | null
}

export type RewardBreakdown = {
  base: number
  unusedGuesses: number
  interest: number
  total: number
}

export type Action =
  | { type: "start_run"; seed: number; ascension?: number }
  | { type: "type_letter"; letter: string }
  | { type: "backspace" }
  | { type: "submit" }
  | { type: "use_consumable"; index: number }
  | { type: "collect" }
  | { type: "buy"; index: number }
  | { type: "sell_relic"; index: number }
  | { type: "reroll" }
  | { type: "next_round" }
  /** Play on past the win, into stages nobody authored. */
  | { type: "continue_run" }
  /** Point the modifier bought a moment ago at a letter. */
  | { type: "place_mod"; letter: string }
  /** Take one of the open pack's cards. */
  | { type: "pick_pack"; index: number }
  /** Walk away from the open pack, forfeiting whatever is left in it. */
  | { type: "skip_pack" }

/**
 * What a one-shot card did, for the toast to say.
 *
 * The card knows what happened and the catalog knows how to say it, so this
 * carries the former and nothing of the latter. It was a preformatted `label`
 * until the prose left the engine, and the shapes are exactly what those
 * sentences interpolated: The Oracle names a letter and a position, The Hermit
 * a letter, The Fool a score, The Magician nothing at all.
 */
export type ConsumableNote =
  | { card: "oracle"; letter: string; position: number }
  | { card: "hermit"; letter: string }
  | { card: "magician" }
  | { card: "fool"; score: number }

/**
 * Enough of a shop item to name it on screen: which table to look it up in,
 * which row, and the letter when the item is a modifier already pointed at one.
 *
 * Deliberately not a `ShopItem`, which carries prices and roll data the toast
 * has no use for, and deliberately not a string, which is what it used to be.
 */
export type PickedItem = {
  kind: "relic" | "consumable" | "etch" | "level" | "range" | "mod" | "pack"
  id: string
  letter?: string
}

/**
 * What a scaling relic has banked so far, in the only two currencies one can
 * bank. Read twice: on the card, as the line under its name, and in the toast
 * the moment it grows, which is why it is one type rather than two agreeing
 * ones. The union on `unit` is the whole guard — a relic that grew in gold
 * would not compile until someone taught the catalog to say so.
 */
export type Growth = { amount: number; unit: "chips" | "mult" }

/**
 * What one effect did to the two numbers, at the moment it did it.
 *
 * This is the narration layer of the scoring pipeline, and it used to be a
 * string: `addChips` said `+20` and `timesMult` said `×3 mult`, and the sentence
 * travelled out on the event and into the save. Five shapes is all there ever
 * were, so the union costs nothing and buys the thing a string could not — a
 * language that puts the unit first, or spells `×` as a word, gets to.
 *
 * `blocked` is The Plateau eating a multiply. It is a shape of its own rather
 * than `times` with a factor of 1, because the two mean opposite things: one is
 * a card that multiplied by one, which no card does, and the other is a card
 * that would have multiplied and was stopped.
 */
export type Payout =
  | { kind: "chips"; amount: number }
  | { kind: "mult"; amount: number }
  | { kind: "times"; factor: number }
  | { kind: "blocked" }
  | { kind: "gold"; amount: number }

/**
 * What a boss says about a guess in place of showing it. One boss does this and
 * one count is all it has to say, which is why the union has a single arm: The
 * Silence replaces the row's colors with a number, and zero is the loudest
 * reading it gives, since every letter not already green is absent.
 *
 * A code and a count rather than the finished line, because "none misplaced"
 * and "2 misplaced" are one sentence in English and need not be in another,
 * where zero can take a different word or a different agreement.
 */
export type GuessNote = { code: "misplaced"; count: number }

/**
 * Why the engine turned an action down.
 *
 * These were English sentences until the prose left the engine, and they are the
 * half of it that was hardest to argue about: a refusal is not decoration, it is
 * the rules speaking, and it is the only text in the game the player reads at the
 * exact moment they are confused. It still has to be said in their language.
 *
 * So a code and its operands, and the sentence is the catalog's. The codes are
 * grouped below the way the reducer produces them rather than by what they mean,
 * because the grouping a reader wants when one of these is wrong is "where does
 * this come from".
 *
 * A word on the `unknown_*` family. Every one of those means a lookup that
 * cannot fail did: a corrupt save, or a table and an id that disagree. They are
 * refusals rather than throws because a run that cannot spend $6 is better than
 * a run that stops, and they are spelled out one at a time rather than collapsed
 * into one diagnostic because which lookup failed is the whole of what they
 * have to say. A translator may reasonably leave them in English.
 */
export type Refusal =
  // Typing and submitting.
  | { code: "not_your_turn" }
  | { code: "not_a_letter" }
  | { code: "letter_broken"; letter: string }
  | { code: "no_room" }
  | { code: "wrong_length"; length: number }
  | { code: "not_in_word_list" }
  // The rules in force on the guess: bosses, ascensions, and the primitives the
  // two share. `position` is one-based, as the player counts tiles.
  | { code: "must_use"; letter: string }
  | { code: "must_keep"; letter: string; position: number }
  | { code: "needs_two_vowels" }
  | { code: "no_repeated_letters" }
  | { code: "already_guessed_round" }
  | { code: "already_used_run" }
  // One-shot cards, and the four ways one can have nothing left to do.
  | { code: "only_during_round" }
  | { code: "no_such_card" }
  | { code: "unknown_card" }
  | { code: "word_already_revealed" }
  | { code: "nothing_to_reveal" }
  | { code: "nothing_to_rule_out" }
  | { code: "already_prepared" }
  | { code: "no_guess_to_repeat" }
  // Between rounds.
  | { code: "nothing_to_collect" }
  | { code: "run_not_won" }
  // The shop, which refuses more than anything else because it is the one screen
  // that holds itself: a pack or a modifier in hand freezes every other button.
  | { code: "not_in_shop" }
  | { code: "sell_only_in_shop" }
  | { code: "finish_pack_first" }
  | { code: "place_mod_first" }
  | { code: "already_bought" }
  | { code: "not_enough_gold" }
  | { code: "no_such_relic" }
  | { code: "no_relic_slots" }
  | { code: "no_card_slots" }
  | { code: "pack_empty" }
  | { code: "no_pack_open" }
  | { code: "already_taken" }
  | { code: "nothing_to_place" }
  | { code: "no_letter_for_mod" }
  /**
   * The one refusal that has to name a card. It carries the id rather than a
   * name for the reason the whole file carries ids: the letter needs uppercasing
   * in one language and a preposition in another, and the modifier's name is the
   * catalog's to spell.
   */
  | { code: "mod_not_allowed"; id: ModId; letter: string }
  | { code: "mod_needs_letter" }
  | { code: "nested_pack" }
  | { code: "unknown_letter" }
  | { code: "unknown_etching" }
  | { code: "unknown_category" }
  | { code: "unknown_range" }
  | { code: "unknown_modifier" }
  | { code: "unknown_pack" }

/**
 * A flat, ordered log the UI replays as animation. Scoring events carry the
 * *running* chips and mult so the screen can render them without re-deriving
 * anything, so the UI stays a dumb projection of this stream.
 */
export type GameEvent =
  | { type: "rejected"; refusal: Refusal }
  | { type: "tile"; index: number; gained: number; chips: number; mult: number }
  | { type: "relic"; slot: number; id: string; paid: Payout; chips: number; mult: number }
  /**
   * A relic permanently growing. Distinct from `relic` because it happens
   * outside the scoring pipeline, where there is no running chips or mult for
   * it to quote, and because the screen should say "this is worth more now"
   * differently from how it says "this just paid".
   */
  | ({ type: "relic_grew"; slot: number; id: string } & Growth)
  /** A letter's own modifier firing, on the tile that carried it. */
  | {
      type: "mod"
      index: number
      letter: string
      id: ModId
      paid: Payout
      chips: number
      mult: number
    }
  /**
   * A leveled word category paying out, between the tiles and the relics. Only
   * emitted when it is actually worth something. At level one the category is
   * still named on the board, but it has nothing to announce.
   */
  | { type: "category"; id: string; level: number; chips: number; mult: number }
  /** `total` is the round's score *after* the multiply, not the guess's. */
  | { type: "solve_bonus"; factor: number; total: number }
  | { type: "guess_scored"; score: number; total: number }
  | { type: "letter_destroyed"; letter: string }
  | { type: "consumable"; id: string; note: ConsumableNote }
  /** A bought modifier landing on the letter the player chose for it. */
  | { type: "mod_placed"; id: ModId; letter: string }
  | { type: "round_won" }
  | { type: "round_lost" }
  | { type: "gold"; delta: number; reason: string }
  | { type: "shop_entered" }
  /** A pack laid out on the table, waiting to be chosen from. */
  | { type: "pack_opened"; id: string; options: number }
  /** A card taken out of a pack, or the pack walked away from when `taken` is null. */
  | { type: "pack_picked"; id: string; taken: PickedItem | null }
  | { type: "run_won" }

export type Reduced = { state: RunState; events: GameEvent[] }

/**
 * The word lists are ~100 KB of text, so they are fetched by the shell rather
 * than imported, which would drag I/O into a pure module. They arrive here as
 * an argument instead.
 */
export type WordSource = {
  answers: readonly string[]
  allowed: ReadonlySet<string>
}

/**
 * The players whose runs get recorded as golden vectors.
 *
 * These only ever run while recording. The committed vectors hold the concrete
 * actions each one produced, and the test replays *those*, so a scenario can
 * be rewritten, or deleted, without invalidating a vector it authored. What is
 * under test is the engine, not the bot.
 *
 * Each scenario must be a pure function of the state it is handed. A bot that
 * consulted a clock or a random number would record an action list the engine
 * could never reproduce.
 */

import type { Action, ModId, RunState, WordSource } from "../../src/engine"
import {
  AUTHORED_ASCENSIONS,
  baseChips,
  categoryOf,
  levelBonus,
  MODIFIER_BY_ID,
  placeableLetters,
  reduce,
  rerollCost,
  solveBonusFor,
} from "../../src/engine"

export type Scenario = {
  name: string
  /** What this run is meant to pin down, for whoever reads a failure. */
  covers: string
  seed: number
  /** The difficulty to start at. Absent is the ordinary game, as it is in a run. */
  ascension?: number
  /** The next batch of actions, or null to stop. */
  next: (state: RunState, words: WordSource) => Action[] | null
}

const typeWord = (word: string): Action[] => [
  ...[...word].map((letter): Action => ({ type: "type_letter", letter })),
  { type: "submit" },
]

/** Every action the engine accepted, dry-run: nothing here mutates the run. */
function accepted(state: RunState, words: WordSource, actions: Action[]): boolean {
  let current = state
  for (const action of actions) {
    const result = reduce(current, action, words)
    if (result.events.some((event) => event.type === "rejected")) return false
    current = result.state
  }
  return true
}

/**
 * The first candidate the engine will actually take. Boss rules refuse whole
 * classes of word (two vowels, no repeats) and a destroyed letter cannot be
 * typed at all, so a bot that assumes its guess lands would stall the recorder
 * on a round it can never submit to.
 */
function firstPlayable(
  state: RunState,
  words: WordSource,
  candidates: readonly string[],
): Action[] | null {
  for (const word of candidates) {
    const actions = typeWord(word)
    if (accepted(state, words, actions)) return actions
  }
  return null
}

/**
 * A change of mind before the real word. Backspace is the one action nothing
 * else here would ever produce, and typing the answer's own first letter is
 * always legal, since answers are drawn to avoid broken letters, so the draft is
 * guaranteed to accept it and the erase leaves the guess exactly as it was.
 */
function withCorrection(state: RunState, guess: Action[] | null): Action[] | null {
  const letter = state.round.answer[0]
  if (!guess || !letter) return guess
  return [{ type: "type_letter", letter }, { type: "backspace" }, ...guess]
}

/** Words that are not the answer, walked from a seed-independent offset. */
function decoys(state: RunState, words: WordSource, offset: number): string[] {
  const list = words.answers
  const out: string[] = []
  for (let i = 0; i < 40; i++) {
    const word = list[(offset + i * 97) % list.length]
    if (word && word !== state.round.answer) out.push(word)
  }
  return out
}

/** How many tiles of a word would carry a modifier, copies counted separately. */
function modTiles(state: RunState, word: string): number {
  return [...word].filter((letter) => state.letters[letter]?.mod).length
}

/**
 * English letter frequency, roughly. What a player aiming a modifier is actually
 * reaching for, the letter they will type most, in a fixed order, which is
 * what a recorded vector needs.
 */
const BY_USE = "etaoinsrhldcumfpgwybvkxjqz"

/**
 * The two rare modifiers, and what it costs to be able to buy one. The price is
 * the dearer of the pair rather than either in particular: a bot that stopped
 * rerolling with exactly Steel money in hand would walk away from a Glass.
 */
const RARE_MODS: readonly ModId[] = ["steel", "glass"]
const RARE_PRICE = Math.max(...RARE_MODS.map((id) => MODIFIER_BY_ID.get(id)?.choiceCost ?? 0))

/** What it costs to walk out of a shop holding a Wild. */
const WILD_PRICE = MODIFIER_BY_ID.get("wild")?.choiceCost ?? 0

/** What it costs to walk out of a shop holding an Anchor. */
const ANCHOR_PRICE = MODIFIER_BY_ID.get("anchor")?.choiceCost ?? 0

/**
 * How many of a word's tiles would land green *on a letter carrying `mod`*.
 *
 * `modTiles` counts a card's tiles whatever color they come up, which is the
 * right question for a card that pays on every tile and the wrong one for a card
 * that pays on one color. Anchor is the second kind, so a bot aiming it has to
 * be able to tell a tile that will fire from a tile that merely carries.
 */
function greenMods(state: RunState, word: string, mod: ModId): number {
  const answer = state.round.answer
  return [...word].filter(
    (letter, index) => letter === answer[index] && state.letters[letter]?.mod === mod,
  ).length
}

/**
 * Put the modifier in hand on the most-typed letter still open to it.
 *
 * Lives outside any one scenario because it is not really a strategy: the shop
 * refuses every other action until a bought modifier has been placed, so any bot
 * that buys one has to answer this before it can do anything else, and they
 * would all answer it the same way. The recorder applies it for all of them.
 */
export function placeMod(state: RunState): Action[] | null {
  const modifier = state.placing ? MODIFIER_BY_ID.get(state.placing) : undefined
  if (!modifier) return null
  const open = new Set(placeableLetters(state, modifier))
  const letter = [...BY_USE].find((candidate) => open.has(candidate))
  return letter ? [{ type: "place_mod", letter }] : null
}

/** What a word's category level is currently worth to it, chips and mult together. */
function levelValue(state: RunState, word: string): number {
  const bonus = levelBonus(state, categoryOf(word))
  return bonus.chips + bonus.mult
}

/** What a word's letters are worth right now, both letter upgrade lines included. */
function wordChips(state: RunState, word: string): number {
  return [...word].reduce((total, letter) => total + baseChips(state, letter), 0)
}

/** Bank the reward, then leave the shop without spending. */
function passThrough(state: RunState): Action[] | null {
  if (state.phase === "reward") return [{ type: "collect" }]
  if (state.phase === "shop") return [{ type: "next_round" }]
  return null
}

export const SCENARIOS: readonly Scenario[] = [
  {
    name: "greedy-solver",
    covers: "the solve bonus at its largest, and a shop purchase every time one is affordable",
    seed: 1,
    next: (state, words) => {
      if (state.phase === "round") return firstPlayable(state, words, [state.round.answer])
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const index = state.shop?.items.findIndex((item) => item && item.cost <= state.gold) ?? -1
        if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
          return [{ type: "buy", index }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  {
    name: "chip-farmer",
    covers:
      "grays scoring, the guess budget spent down to one, and the solve bonus at its smallest",
    seed: 7,
    next: (state, words) => {
      if (state.phase === "round") {
        const spent = state.round.guesses.length
        // Burn every guess but the last on decoys, then solve. This is the
        // income line: more tiles scored, a far smaller solve multiplier.
        const candidates =
          spent >= state.round.maxGuesses - 1
            ? [state.round.answer]
            : [...decoys(state, words, spent * 13), state.round.answer]
        return firstPlayable(state, words, candidates)
      }
      return passThrough(state)
    },
  },
  {
    name: "banker",
    covers: "a pile banked and then multiplied, the line the scoring rule exists for",
    seed: 2024,
    next: (state, words) => {
      if (state.phase === "round") {
        // Farm two guesses, then cash in while the multiplier is still large.
        // Solving instantly multiplies nothing and solving on the last guess
        // multiplies by one, so without this scenario the vectors would record
        // the same totals whether the bonus applied to the round or the guess.
        const spent = state.round.guesses.length
        const candidates =
          spent < 2
            ? [...decoys(state, words, spent * 29), state.round.answer]
            : [state.round.answer]
        return firstPlayable(state, words, candidates)
      }
      return passThrough(state)
    },
  },
  {
    name: "never-solves",
    covers: "running a round out of guesses, and the run ending in defeat",
    seed: 42,
    next: (state, words) => {
      if (state.phase === "round") return firstPlayable(state, words, decoys(state, words, 5))
      return passThrough(state)
    },
  },
  {
    name: "shopkeeper",
    covers: "reroll, sell, consumables used rather than hoarded, and a corrected draft",
    seed: 1234,
    next: (state, words) => {
      if (state.phase === "round") {
        // Spend a consumable the moment there is one: the Oracle and the
        // Hermit change what a later guess scores, which is exactly the kind of
        // cross-turn effect a per-guess score list is there to pin down.
        if (state.round.guesses.length === 0 && state.consumables.length > 0) {
          if (accepted(state, words, [{ type: "use_consumable", index: 0 }])) {
            return [{ type: "use_consumable", index: 0 }]
          }
        }
        return withCorrection(state, firstPlayable(state, words, [state.round.answer]))
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        if (state.shop?.rerolls === 0 && accepted(state, words, [{ type: "reroll" }])) {
          return [{ type: "reroll" }]
        }
        const index = state.shop?.items.findIndex((item) => item && item.cost <= state.gold) ?? -1
        if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
          return [{ type: "buy", index }]
        }
        // Selling back is the only way the relic slots ever empty, and it is
        // priced, and a vector that never sells cannot catch that price moving.
        if (
          state.relics.length >= 2 &&
          accepted(state, words, [{ type: "sell_relic", index: 0 }])
        ) {
          return [{ type: "sell_relic", index: 0 }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * The consumables, actually spent.
   *
   * `shopkeeper` uses index 0 before its first guess of a round, which reaches
   * exactly one of the four: The Magician. The other three were bought, carried
   * and thrown away unused across every vector in the file, and The Fool is the
   * reason why, because it rescores the previous guess and there is no previous
   * guess before the first one. A bot that only ever spends at the top of a
   * round cannot use it at all, so the card's whole arithmetic went unrecorded.
   *
   * This one spends whatever the engine will take, whenever it will take it,
   * which is what turns the order into coverage rather than a rule: Oracle and
   * Hermit are accepted before a guess and go first, and The Fool becomes legal
   * only once there is something behind it. The probe is there for the same
   * reason: the Fool doubling a real score is the case worth pinning, and a
   * bot that solved on sight would have handed it a solve to copy instead.
   *
   * Spending all four is not the hard part once the order is right: 160 of the
   * first 400 seeds manage it, and only four spend nothing. Seed 126 is simply
   * the shortest of the 160, which is the whole basis for the choice: at this
   * hit rate the seed is not buying an outcome, it is buying fewer lines of
   * JSON for the same coverage.
   */
  {
    name: "mystic",
    covers: "consumables spent rather than hoarded, including the one that needs a guess behind it",
    seed: 126,
    next: (state, words) => {
      if (state.phase === "round") {
        // Whichever card the engine will take right now, in slot order. Trying
        // them rather than knowing them is the point: the legality is the rule
        // under test, and a bot that hardcoded which card works when would stop
        // noticing when that changed.
        const index = state.consumables.findIndex((_, slot) =>
          accepted(state, words, [{ type: "use_consumable", index: slot }]),
        )
        if (index >= 0) return [{ type: "use_consumable", index }]

        const spent = state.round.guesses.length
        const candidates =
          spent === 0 ? [...decoys(state, words, 19), state.round.answer] : [state.round.answer]
        return firstPlayable(state, words, candidates)
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        // Cards first, then relics to stay alive long enough to draw more of
        // them. Consumables are the one item that can be bought while already
        // holding one, so the slot cap does the throttling here, not the bot.
        for (const kind of ["consumable", "relic"] as const) {
          const index = items.findIndex((item) => item?.kind === kind && item.cost <= state.gold)
          if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
            return [{ type: "buy", index }]
          }
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * Seed 125 rather than 9, and the criterion is written down here because this
   * scenario had none and quietly lost a card for it.
   *
   * The three below this one each name a modifier and go looking for it. This is
   * the one that covers the rest, and what "the rest" means in practice is the
   * three that any run will actually be offered: Chip, Mult and Gold, the two
   * commons and the payout. Seed 9 landed Chip and Gold; the answer-list scrub
   * moved its shelf and it landed Glass and Gold, and Chip — the commonest card
   * in the game — stopped firing in any vector in the file. Nothing went red,
   * because no test asks which modifiers a vector happens to hold.
   *
   * So: all three, and then the shortest run that has them. 8 of the first 1,200
   * seeds hold Chip, Mult and Gold together, which is rare enough that depth
   * cannot also be asked for — the eight run from stage 5 to stage 8. 125 is the
   * shallowest and shortest of them at 26 guesses, two more than seed 9 spent,
   * with all three landed on a, e and t, the letters a probe is most likely to
   * put under them.
   */
  {
    name: "letter-smith",
    covers: "letter modifiers bought, and then landed on tiles often enough to score",
    seed: 125,
    next: (state, words) => {
      if (state.phase === "round") {
        // One probe chosen for the modifiers it would fire, then the answer. A
        // modifier that is only ever bought is a shop test; what these vectors
        // are for is the mult it multiplies and the gold it pays.
        const probes =
          state.round.guesses.length === 0
            ? [...decoys(state, words, 7)].sort((a, b) => modTiles(state, b) - modTiles(state, a))
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        const wanted = items.findIndex((item) => item?.kind === "mod" && item.cost <= state.gold)
        if (wanted >= 0 && accepted(state, words, [{ type: "buy", index: wanted }])) {
          return [{ type: "buy", index: wanted }]
        }
        // Hunt for one while there is gold to spare. The cost climbs with every
        // reroll, so this drains rather than loops.
        if (state.gold >= 10 && accepted(state, words, [{ type: "reroll" }])) {
          return [{ type: "reroll" }]
        }
        const index = items.findIndex((item) => item && item.cost <= state.gold)
        if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
          return [{ type: "buy", index }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * Anchor, and the color it is fussy about.
   *
   * This one is here because of how it went missing. `letter-smith` held an
   * Anchor for as long as the modifier table had eleven entries; the reweighting
   * to sixteen dropped the card to one roll in sixteen behind a slot that is
   * three in four, and it fell out of every recorded run at once. Nothing failed
   * No test went red, the vectors re-recorded cleanly, and the diff read as a
   * shop change, which it was. The card's own arithmetic simply stopped being
   * exercised, and it stopped on the same pass that resized it. A card is worth
   * a scenario when the shelf can take it away from you quietly.
   *
   * What that scenario has to do is not obvious, because solving hides the bug.
   * The winning guess is five greens by definition, so a solve-on-sight bot
   * fires every Anchor the answer contains and records a fat number every time,
   * and would go on recording it if the card paid on any color at all. The gate
   * is the half worth pinning, so the probe is filtered rather than sorted: a
   * candidate is played only if it puts an anchored letter in its own position,
   * and the guess is skipped when none would. Sorting by `modTiles` the way the
   * rare hunt does was the first attempt and it is the wrong question: it ranks
   * tiles that carry the card above tiles that fire it, which for a color-gated
   * card are different sets.
   *
   * Five copies is what makes this useful rather than merely green. The card is
   * a flat +125 per firing tile, so a run carrying five of them is where an
   * error in that number is loudest instead of roundable. Five is also rare:
   * over the first 1,200 seeds only three place that many, and seed 249 is the
   * deepest of the three, holding all five across 34 guesses and living to
   * stage 6.
   *
   * That is the third seed this scenario has had, and the two it replaced went
   * the same way, which is the point worth carrying forward rather than the
   * numbers. It was 32, then 801, and each time a word-list change reshuffled
   * which words this bot is dealt — a bot filtered on "would this guess fire an
   * Anchor" is a bot whose whole run hangs on the words available to it. 801
   * survived the audit that took 32 and did not survive the scrub that took the
   * obscenities out of the answer lists: it now places two Anchors instead of
   * five and dies in stage 3. Neither time did anything go red. The vector
   * re-recorded cleanly and went on asserting a shorter run, which is exactly
   * the failure this scenario exists to catch, arriving by the other door.
   */
  {
    name: "anchor-smith",
    covers: "Anchor stacked across letters, fired on greens no solve handed it",
    seed: 249,
    next: (state, words) => {
      if (state.phase === "round") {
        // The probe is chosen to land the card green rather than merely to carry
        // it, and is skipped entirely when no candidate would. Anchor pays on one
        // color, so a guess that puts it on a gray costs a gold and records
        // nothing the solve was not going to record anyway.
        const armed = Object.values(state.letters).some((letter) => letter.mod === "anchor")
        const probes =
          armed && state.round.guesses.length === 0
            ? decoys(state, words, 29)
                .filter((word) => greenMods(state, word, "anchor") > 0)
                .sort((a, b) => greenMods(state, b, "anchor") - greenMods(state, a, "anchor"))
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        const anchor = items.findIndex(
          (item) => item?.kind === "mod" && item.id === "anchor" && item.cost <= state.gold,
        )
        if (anchor >= 0 && accepted(state, words, [{ type: "buy", index: anchor }])) {
          return [{ type: "buy", index: anchor }]
        }
        // Same stopping rule as the rare hunt, for the same reason: reroll only
        // while the change still covers the card being hunted.
        const reroll = rerollCost(state.shop ?? { items: [], rerolls: 0 })
        if (state.gold >= reroll + ANCHOR_PRICE && accepted(state, words, [{ type: "reroll" }])) {
          return [{ type: "reroll" }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * The rare modifier pair, which nothing else here ever holds.
   *
   * `letter-smith` buys the first modifier it can afford, and the first
   * affordable modifier is a common one. Steel and Glass are one entry each in
   * a sixteen-entry table behind a slot that is itself three rolls in four, so
   * a given visit offers a particular one about 5% of the time. Across every
   * other vector that came to zero: the two dearest cards in the letter line,
   * the two whose numbers get argued over most, and no recorded run had ever
   * put one on a letter.
   *
   * So this bot solves on sight, the fastest income line there is, with five
   * unused guesses and the round's base, and then spends the whole pile hunting. It
   * works: 303 of the first 600 seeds end holding one, and 17 of the 600 end
   * holding both. Counting the final board undercounts the pairing either way,
   * since Glass does what it says and breaks on a gray, so a run that played one
   * and lost it reads here as a run that never had it. The measurement that
   * matters is what the letters carried while the guesses were being scored.
   *
   * Seed 462 holds three rare cards at once, Steel on a and t against Glass on
   * e, so the vector records the two of them scoring side by side rather than in
   * different runs. Over the first 1,200 seeds 27 end holding both, and it is
   * the deepest of those at stage 3 across 14 guesses.
   *
   * It replaced 586, and 586 replaced 490, and 490 replaced 397, and all three
   * replacements are the same story told by a different upstream change: 397 was
   * chosen against the eleven-entry modifier table and degraded to a single Steel
   * and a stage-two death when the shelf was reweighted; 490 was chosen against
   * the old answer list and degraded to three Steel and no Glass when the
   * word-list audit swapped 52 answers out; 586 went the same way when the scrub
   * took the obscenities out, down to one Glass and no Steel — which is to say
   * down to no pair, the one thing this scenario exists to record. The paragraph
   * above ended by saying to assume the next edit to a shelf or a list breaks
   * this one too. It did, on the very next one. A seed picked for what it happens
   * to draw is not wrong afterwards, just weaker, and the vector goes on passing
   * while covering less than its comment claims. Re-measure rather than
   * re-record.
   *
   * These numbers have moved twice, and they moved for two different reasons.
   * At the eleven-entry table it was 7% a visit and 386 of 600, and the
   * reweighting that made the strong cards 3 in 16 is what took it to 5% and
   * 302: the shelf deciding how often the card is offered. Then the word-list
   * audit left 302 alone, at 303, and took the runs ending with both from 9 to
   * 17, because Glass breaks on a gray letter the answer genuinely lacks, and
   * which letters an answer lacks is a fact about the answer list. So these read
   * the current shelf *and* the current list, and neither is a fact about the bot.
   *
   * It dies shallow, and that is the trade being made on purpose. Depth is what
   * every other vector already has; what this one is for is the pair that only
   * turns up if you go looking and can still pay the reroll when it does.
   */
  {
    name: "rare-smith",
    covers: "the rare modifier pair, hunted down with rerolls and played through",
    seed: 462,
    next: (state, words) => {
      if (state.phase === "round") {
        // Solve on sight until there is a card to fire, then spend one guess a
        // round on the word that fires it most. Before the first purchase every
        // guess left unspent is a gold toward the hunt; after it, a modifier
        // nobody ever plays through is a shop test rather than a scoring one,
        // and the second is what this vector is for. The trade is one gold a
        // round, which is what an unused guess pays.
        const armed = Object.values(state.letters).some((letter) => letter.mod)
        const probes =
          armed && state.round.guesses.length === 0
            ? [...decoys(state, words, 23)].sort((a, b) => modTiles(state, b) - modTiles(state, a))
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        const rare = items.findIndex(
          (item) => item?.kind === "mod" && RARE_MODS.includes(item.id) && item.cost <= state.gold,
        )
        if (rare >= 0 && accepted(state, words, [{ type: "buy", index: rare }])) {
          return [{ type: "buy", index: rare }]
        }
        // Keep hunting only while the gold left over could still pay for what is
        // being hunted. Without the second term the bot rerolls itself broke and
        // walks past the card it was looking for on the visit it finally appears.
        const reroll = rerollCost(state.shop ?? { items: [], rerolls: 0 })
        if (state.gold >= reroll + RARE_PRICE && accepted(state, words, [{ type: "reroll" }])) {
          return [{ type: "reroll" }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * Wild, which nothing else here has ever held either.
   *
   * The same gap `rare-smith` was written to close, found the same way: Wild is
   * one entry in the sixteen-entry table, `letter-smith` buys the first card it
   * can afford and that is a common, and across every other vector no letter had
   * ever carried one. That mattered less when the card paid at most +3 mult on a
   * tile that had missed; it matters now that the floor makes it a card a build
   * can be pointed at, because the number is exactly the kind that gets argued
   * over and the vectors are what settle those arguments.
   *
   * Hunted rather than waited for, on `rare-smith`'s terms: solve on sight for
   * the unused-guess gold, spend it on rerolls, and stop rerolling while the
   * change still covers the card. One difference: this bot keeps probing after
   * it is armed, every round rather than the first. Wild pays per tile carrying
   * it and the letters worth putting it on are the ones a probe is full of, so a
   * vector that solved on sight would record the card sitting on a letter
   * instead of firing.
   *
   * 362 of the first 600 seeds end holding one, and 244 of those also run 12 to
   * 30 guesses and reach stage three, a far easier hunt than the rare pair's,
   * which is what one entry at $9 rather than one at $12 buys. Seed 130 is a
   * middling run of those rather than a lucky one: it is armed at the first shop
   * it visits, so all 27 of its guesses are scored with the card in play, and it
   * dies at stage five on a score in the middle of the pack. Buying early is the
   * half worth pinning: a vector that bought a Wild in stage four would record
   * the purchase and almost none of the arithmetic.
   *
   * It ends holding four of them, on A, E, O and T, which is not the bot losing
   * the plot: `placeableLetters` only bars the letter already carrying the same
   * card, so a bot that hunts one modifier stacks it across the alphabet by
   * frequency. That turns out to be the useful case to have recorded, since the
   * floor is per tile, so four of them is where a mistake in it shows up
   * loudest.
   */
  {
    name: "wild-smith",
    covers: "Wild bought over and over, aimed by letter frequency, and fired on every color",
    seed: 130,
    next: (state, words) => {
      if (state.phase === "round") {
        const armed = Object.values(state.letters).some((letter) => letter.mod === "wild")
        const probes =
          armed && state.round.guesses.length === 0
            ? [...decoys(state, words, 11)].sort((a, b) => modTiles(state, b) - modTiles(state, a))
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        const wild = items.findIndex(
          (item) => item?.kind === "mod" && item.id === "wild" && item.cost <= state.gold,
        )
        if (wild >= 0 && accepted(state, words, [{ type: "buy", index: wild }])) {
          return [{ type: "buy", index: wild }]
        }
        const reroll = rerollCost(state.shop ?? { items: [], rerolls: 0 })
        if (state.gold >= reroll + WILD_PRICE && accepted(state, words, [{ type: "reroll" }])) {
          return [{ type: "reroll" }]
        }
        // Unlike the rare hunt, this one spends what is left over on relics. A
        // card that pays per tile wants a run long enough to play tiles in, and
        // rerolling to broke every visit buys depth in nothing.
        const relic = items.findIndex((item) => item?.kind === "relic" && item.cost <= state.gold)
        if (relic >= 0 && accepted(state, words, [{ type: "buy", index: relic }])) {
          return [{ type: "buy", index: relic }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  {
    name: "leveler",
    covers: "category levels bought, stacked, and paid out on the guesses that match them",
    seed: 77,
    next: (state, words) => {
      if (state.phase === "round") {
        // One probe chosen for the level bonus it would collect, then the
        // answer. The sort is what makes this vector worth recording: it pins
        // that levels land on the base *and* that `categoryOf` picked the same
        // shape the shop charged for, because a mismatch would show up as a
        // probe that scored like an unleveled word.
        const probes =
          state.round.guesses.length === 0
            ? [...decoys(state, words, 3)].sort(
                (a, b) => levelValue(state, b) - levelValue(state, a),
              )
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        // Levels before anything else, whichever category the slot dealt. It
        // cannot choose, since the shop picks the category, so this ends the run
        // holding several at level two rather than one high, which is a fair
        // picture of what buying every level you are offered actually gets you.
        const wanted = items.findIndex((item) => item?.kind === "level" && item.cost <= state.gold)
        if (wanted >= 0 && accepted(state, words, [{ type: "buy", index: wanted }])) {
          return [{ type: "buy", index: wanted }]
        }
        // Relics otherwise, so the run has ×mult for the levels to pass through.
        const relic = items.findIndex((item) => item?.kind === "relic" && item.cost <= state.gold)
        if (relic >= 0 && accepted(state, words, [{ type: "buy", index: relic }])) {
          return [{ type: "buy", index: relic }]
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  {
    name: "etcher",
    covers: "alphabet range levels and etchings stacking on the same letters",
    seed: 21,
    next: (state, words) => {
      if (state.phase === "round") {
        // One probe picked for what its letters are worth right now, then the
        // answer. The sort is what makes this vector worth recording: it reads
        // `baseChips`, so if a range level ever stopped reaching a letter, or
        // stopped adding to the etching already on it, the probe would score
        // like an un-upgraded word and every number after it would move.
        const probes =
          state.round.guesses.length === 0
            ? [...decoys(state, words, 5)].sort((a, b) => wordChips(state, b) - wordChips(state, a))
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        const items = state.shop?.items ?? []
        // Two relics first, then both letter lines, ranges ahead of etchings.
        // The relic floor is not a flourish: chips are only ever half of a
        // score, and a bot that spent its whole run raising them stalled in stage
        // two with nothing to multiply them by. Once it has some mult, buying in
        // this order is what gets the two lines stacked on one letter: the
        // ranges partition the alphabet, so whichever etching lands afterwards
        // is guaranteed to overlap one that has already been leveled.
        const order =
          state.relics.length < 2
            ? (["relic", "range", "etch"] as const)
            : (["range", "etch", "relic"] as const)
        for (const kind of order) {
          const index = items.findIndex((item) => item?.kind === kind && item.cost <= state.gold)
          if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
            return [{ type: "buy", index }]
          }
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  {
    name: "pack-opener",
    covers: "packs bought, held open across the shop, and chosen from",
    seed: 5,
    next: (state, words) => {
      if (state.phase === "round") {
        // One probe, then the answer. Packs deal all three card lines, so the
        // sort keys off modifiers first and levels second rather than off any
        // single one. Whichever the packs happened to hand this run, the probe
        // is the word that collects the most of it.
        const probes =
          state.round.guesses.length === 0
            ? [...decoys(state, words, 11)].sort(
                (a, b) =>
                  modTiles(state, b) - modTiles(state, a) ||
                  levelValue(state, b) - levelValue(state, a),
              )
            : []
        return firstPlayable(state, words, [...probes, state.round.answer])
      }
      if (state.phase === "reward") return [{ type: "collect" }]
      if (state.phase === "shop") {
        // An open pack holds the shop, so it has to be resolved before anything
        // else is even legal. Taking the first card the engine accepts is the
        // whole point of recording this bot: a pack applies its card for free,
        // and a vector that only ever *bought* one could not tell whether the
        // card that came out of it ever landed.
        if (state.pack) {
          // Stage one is walked away from on purpose, for the same reason the
          // shopkeeper's backspace is there: the shelf no longer sells a pack
          // that cannot be opened, so nothing these bots do would otherwise
          // ever produce a skip, and a forfeit that quietly handed the gold
          // back would then be outside the contract entirely.
          if (state.stage === 1) return [{ type: "skip_pack" }]
          const index = state.pack.options.findIndex(
            (item, slot) => item && accepted(state, words, [{ type: "pick_pack", index: slot }]),
          )
          return index >= 0 ? [{ type: "pick_pack", index }] : [{ type: "skip_pack" }]
        }
        const items = state.shop?.items ?? []
        // Two relics before the packs, for the same reason the etcher wants
        // them: packs deal cards that add to a score, and a run with nothing to
        // multiply by stalls in stage two no matter how good the cards were.
        const order =
          state.relics.length < 2 ? (["relic", "pack"] as const) : (["pack", "relic"] as const)
        for (const kind of order) {
          const index = items.findIndex((item) => item?.kind === kind && item.cost <= state.gold)
          if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
            return [{ type: "buy", index }]
          }
        }
        return [{ type: "next_round" }]
      }
      return null
    },
  },
  /*
   * The run that wins, and the only one. Every other vector here ends in
   * `game_over`, which left the whole back half of the ending (the `victory`
   * phase, `continue_run`, and the stages past `STAGES` that have no authored
   * target) asserted by nothing at all. A rewrite could have dropped the win
   * condition on the floor and this file would have agreed with it.
   *
   * Nothing here records `outcome: "victory"` even so, because this bot walks
   * through the win rather than stopping on it. What pins the win instead is the
   * refusal test: `continue_run` is refused unless the phase is `victory`, so an
   * engine that stopped awarding the win would refuse the action, and the
   * assertion that no recorded action is ever refused would name it.
   *
   * Picking a seed to get an outcome is normally how a vector stops being about
   * the rules and starts being about the bot, so the choice is defended rather
   * than asserted. Winning is genuinely rare, and the spread is not a curve but
   * two piles: over the first 3,000 seeds, 2,356 die in stage one against 50
   * that reach stage nine. No bot wins on an arbitrary seed, so a seed had to be
   * chosen. What 1983 was chosen *for*:
   *
   *   - It supplies both bosses the other sixteen vectors miss between them,
   *     The Miser and The Rust. Measured, not assumed: the check is the union
   *     of every `boss` this file records against `BOSSES`.
   *   - It goes as deep as anything in 3,000 seeds, dying on stage 11's boss
   *     round, which four of the 3,000 reach.
   *   - It is not tuned to this bot. Five other scenarios' bots run on it reach
   *     stage 7 or 8. The run is winnable; the climber is not being carried.
   *
   * That second point is why one run can close the boss gap at all, and the
   * reason is structural rather than lucky. The late band is drawn without
   * replacement and indexed from stage 7, so a run reaching stage 11's boss has
   * met all five of them in order. Those bosses were uncovered *because* nothing
   * survived past stage 7. No shallow vector could have reached them, and no
   * number of shallow vectors would have helped.
   *
   * The seed was 5517, then 2111, and each was chosen the same way against the
   * answer list of its day. The audit that dropped SENOR and GONNA and added
   * TEETH and EMOJI changed which word every seed deals, and 5517 went from a
   * 75-guess win to dying in stage one, taking The Miser, The Rust and The
   * Plateau out of the file with it. The scrub that took the obscenities out of
   * the answer lists did the same thing to 2111, from a stage-11 win to six
   * guesses and out, and took The Miser and The Rust with it a second time.
   * Neither time did anything go red: the vector re-recorded cleanly and went on
   * asserting stage one. That is the standing hazard of a seed picked for what it
   * draws, and the check that catches it is coverage, not a test. Re-run it after
   * anything that moves the deal — twice now, that warning has been the thing
   * that found the damage.
   */
  {
    name: "victor",
    covers:
      "the run won and then played past, the computed targets beyond stage 8, and the whole " +
      "late boss band nothing else survives to meet",
    seed: 1983,
    next: climb,
  },
  /*
   * Seed 161 rather than 13, for the reason the two vectors above were reseeded:
   * the answer-list scrub took 13 from four relics and stage 5 down to one relic
   * and stage 1, which is a rung of the ladder covered by nothing. Of the first
   * 1,200 seeds 679 end holding four relics, so this is a common enough shape to
   * pick on its merits rather than a lucky one; 161 is the shortest run among
   * those that also reach stage 6, at 35 guesses against the old seed's 49. Depth
   * was available — seed 247 climbs to stage 12 — and declined on purpose. This
   * scenario pins the ladder's rules, not how far a bot can be carried up it, and
   * a vector twice the length would say the same thing twice.
   */
  {
    name: "ascendant",
    covers:
      "the whole written ascension ladder: guesses filtered by the run's rules, targets and " +
      "payouts bent by them, every round solved",
    seed: 161,
    ascension: AUTHORED_ASCENSIONS,
    next: climb,
  },
  /*
   * The half of the ladder that has no rules left to add. Its targets are the
   * only thing this pins that the rung below does not: the endless step is a
   * pure multiplication, and a port that compounded it wrongly, or rounded it
   * to the hundred `roundTargets` rounds to, would clear a different first
   * round and diverge on the very first score.
   *
   * It records a short run and that is the honest outcome, not a shortfall: at
   * ×1.56 targets with a thinned shelf and a round that has to be solved, the
   * climber's line does not last, and a vector that pretended otherwise would be
   * a vector of a bot rather than of the rules. Across 40 seeds it clears 0 to 14
   * rounds, mean 2.75, so a run of about four is this scenario at its typical.
   *
   * Seed 241, and it is the third, which is the point of this paragraph:
   * reseeding a vector is normally the wrong repair, and this scenario has now
   * needed it twice for the same reason both times. 21 became 20 when replacing
   * rung 9 handed this level back its sixth guess — nothing about what the
   * scenario covers, everything about where its decoy walk lands — and 21 had
   * diverged onto a line that dies on the first round, four rewards and three
   * relics down to one and one. The answer-list scrub did precisely that to 20:
   * one reward, one relic, stage 1. So the criterion is written down rather than
   * left to the next reader to reconstruct. Wanted is a run that reaches a
   * *shop*, which is what the rewards and the relics are evidence of, and does
   * not run so long that the vector is mostly the climber's line: 3 to 6 rewards.
   * 241 gives six rewards and four relics in 25 guesses. The seed moves to hold
   * the coverage the scenario was written to have, not to hold a number.
   */
  {
    name: "endless",
    covers: "a rung above the written ladder: targets compounded by the endless step",
    seed: 241,
    ascension: AUTHORED_ASCENSIONS + 4,
    next: climb,
  },
  /*
   * Halfway up, and the vector that should have existed before the rungs were
   * ever reordered.
   *
   * The two above it stand at `AUTHORED_ASCENSIONS` and four past it, where
   * every written rule is in force. `difficultyAt` folds the rungs with `*=`,
   * `+=`, `-=` and `||=`, all of them commutative, so the fold at the top of the
   * ladder is the same number whatever order the rungs are written in. When the
   * ladder was reshuffled into the order it now has, both vectors re-recorded
   * byte for byte and the whole suite went green over a change that moved five
   * rules. That is not the vectors being wrong; it is nine rungs having nothing
   * standing on them.
   *
   * Five, because the middle is where the most is *absent*, and absence is the
   * half neither vector above can assert. At this rung the run holds five relics
   * rather than four, because Crowded is a rung above; it is paid full round
   * money, because Lean Years is two above; and it opens all 23 of its rounds on
   * ABACK, because `decoys` indexes from the guess number rather than the round
   * number and No Echoes, which would refuse the repeat from the second round on,
   * is four rungs above. Move any one of those three down onto rung 5 and this
   * vector moves every score after the round it lands in. It also carries
   * Steeper's target multiplier across the entire written stage ladder, which is
   * the rule that arrived here in the reshuffle.
   *
   * Seed 38 for its length and its breadth: it dies on the last round of stage 8
   * without ever reaching the win, so it plays the whole authored target curve
   * at a bent multiplier and still leaves `victory` and `continue_run` to
   * `victor`, which is the only vector that should own them. 58 guesses, 23
   * rounds banked, a full tray of five relics, five etchings and a Hot Streak
   * banking 600 chips. Nothing above stage 8 and nothing tuned: at this rung a
   * run of about five stages is typical, and this one is chosen for what it
   * covers rather than for how far it got.
   */
  {
    name: "midway",
    covers:
      "the middle of the ladder: the rules in force at rung five and, just as much, the four " +
      "above it that are not",
    seed: 38,
    ascension: 5,
    next: climb,
  },
]

/**
 * The climber's line, shared by both ascension vectors.
 *
 * Named rather than inlined because the two differ only in where on the ladder
 * they stand: the same play against the written rules and against the endless
 * ones is what makes the pair of vectors a comparison rather than two runs.
 */
function climb(state: RunState, words: WordSource): Action[] | null {
  // Take the win and keep going. Only `victor` ever gets here, since the two
  // ascension runs die well short, but it belongs to the line rather than to
  // one scenario: a climber is the bot that would carry on, and this is the
  // only place `continue_run` is reachable at all.
  if (state.phase === "victory") return [{ type: "continue_run" }]
  if (state.phase === "round") {
    const round = state.round
    const left = round.maxGuesses - round.guesses.length - 1
    // Cash in the moment solving would clear the target, and on the last
    // guess whatever the pile is worth: at the top of the ladder a round
    // that is never solved is a round that is lost, target met or not.
    //
    // The probes are where the rules bite. `firstPlayable` walks past every
    // decoy the engine refuses, so what lands in this vector is the first
    // word the ladder actually allowed. A port that filtered guesses
    // differently would record a different word and every score after it.
    const cashOut = left <= 0 || round.score * solveBonusFor(state, left) >= round.target
    const candidates = cashOut
      ? [round.answer]
      : [...decoys(state, words, round.guesses.length * 17), round.answer]
    return firstPlayable(state, words, candidates)
  }
  if (state.phase === "reward") return [{ type: "collect" }]
  if (state.phase === "shop") {
    // A pack holds the shop until it is resolved, so it comes first however
    // it was bought.
    if (state.pack) {
      const picked = state.pack.options.findIndex(
        (item, slot) => item && accepted(state, words, [{ type: "pick_pack", index: slot }]),
      )
      return picked >= 0 ? [{ type: "pick_pack", index: picked }] : [{ type: "skip_pack" }]
    }
    const items = state.shop?.items ?? []
    // Relics before anything else, then whatever is affordable. A bot that
    // spent nothing would die in stage one and this vector would cover three
    // rounds of a ladder meant to be climbed for eight stages.
    for (const kind of ["relic", null] as const) {
      const index = items.findIndex(
        (item) => item && (kind === null || item.kind === kind) && item.cost <= state.gold,
      )
      if (index >= 0 && accepted(state, words, [{ type: "buy", index }])) {
        return [{ type: "buy", index }]
      }
    }
    return [{ type: "next_round" }]
  }
  return null
}

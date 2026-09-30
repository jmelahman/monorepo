/**
 * What a benchmarked model is shown, in the two forms it may ask for.
 *
 * Built from the same two sources the screen is: the run with its answer taken
 * off, and the English catalog. Nothing here writes prose about a card of its
 * own. A relic says what `relicCard` says and a shop item says what
 * `describeItem` puts on the shelf, so a model and a player are reading the same
 * sentence about the same card, and a rebalance that rewords one rewords both.
 *
 * The text form is the default because it is the one that tests play. A bare
 * `PlayerView` names relics by id and letters by a table of numbers, and a
 * model handed that is being tested on reading a save file. It is still offered,
 * since some harnesses would rather parse than read, and it carries the same
 * legal-action list so that neither form has to be reasoned from the other.
 */

import type { RoundState, RunState, ShopItem } from "../engine"
import {
  ALPHABET,
  ascensionAt,
  baseChips,
  bossForStage,
  CATEGORY_BY_ID,
  CONSUMABLE_SLOTS,
  difficultyOf,
  keyboardColors,
  levelOf,
  RANGES,
  RELIC_BY_ID,
  ROUNDS_PER_STAGE,
  rangeLevelOf,
  rerollCost,
  roundTarget,
  rulesFor,
  STAGES,
  solveBonusFor,
} from "../engine"
import {
  ascensionCard,
  bossCard,
  categoryCard,
  consumableCard,
  growthBadge,
  guessNote,
  modifierCard,
  packCard,
  relicCard,
  roundName,
  ui,
} from "../ui/lang"
import { describeItem } from "../ui/views"
import { legalCommands } from "./commands"

/**
 * The run as a player sees it. `round.answer` is gone; everything else stays,
 * including `bossId` and `round.target`. Both are on the screen, and a player
 * who could not see what they were up against would be blind in a way the game
 * never asks anyone to be.
 *
 * Here rather than in `test/helpers/blind.ts`, where it was written, because
 * the benchmark makes the same claim the blind bot does, that it played without
 * the word, and one claim should rest on one piece of code. The field is
 * genuinely absent at runtime rather than typed away, so a harness that tried to
 * read it would get `undefined`, and `JSON.stringify` has nothing to leak.
 */
export type PlayerView = Omit<RunState, "round"> & { round: Omit<RoundState, "answer"> }

export function playerView(state: RunState): PlayerView {
  const { answer: _hidden, ...round } = state.round
  return { ...state, round }
}

export type Format = "text" | "json"

export const FORMATS: readonly Format[] = ["text", "json"]

export const readFormat = (raw: unknown): Format | null =>
  typeof raw === "string" && (FORMATS as readonly string[]).includes(raw) ? (raw as Format) : null

/** What the session knows beyond the state: the last thing it turned down. */
export type Context = {
  refusal?: string | null
  /**
   * The harness has ended the run although the engine has not: `end`, a
   * stall, or a win in a suite that stops there. The victory screen still
   * offers `continue`, and a list that offered it to a run nobody may continue
   * would be the one lie this observation tells.
   */
  over?: boolean
}

export function observe(state: RunState, format: Format, context: Context = {}): string {
  if (format === "json") {
    return JSON.stringify({
      view: playerView(state),
      legal: context.over ? [] : legalCommands(state),
      refusal: context.refusal ?? null,
    })
  }
  return renderText(state, context)
}

/* ------------------------------------------------------------------ text */

const upper = (word: string) => word.toUpperCase()

/** "1,234", which is what a model reads most reliably; the screen's "1.23K" is for thumbs. */
const n = (value: number) => Math.round(value).toLocaleString("en-US")

function renderText(state: RunState, context: Context): string {
  const lines: string[] = []
  const push = (...more: string[]) => lines.push(...more)
  const round = state.round
  const ascension = state.ascension ?? 0

  push(
    `STAGE ${state.stage} of ${STAGES}${state.stage > STAGES ? " (endless)" : ""}, ` +
      // Between rounds the run still points at the round just played, and a
      // shop that says "Normal Round" beside a NEXT ROUND line naming the elite
      // reads as two answers to one question.
      `${state.phase === "reward" || state.phase === "shop" ? "just cleared " : ""}` +
      `${roundName(state.roundIndex)} (${state.roundIndex + 1} of ${ROUNDS_PER_STAGE}), ` +
      `ascension ${ascension}`,
    `PHASE ${state.phase}${state.pack ? " (pack open)" : ""}${state.placing ? " (placing a modifier)" : ""}`,
    `GOLD $${state.gold}`,
  )

  const rules = standingRules(state)
  if (rules.length) push("", "STANDING RULES (this ascension)", ...rules.map((rule) => `- ${rule}`))

  if (state.phase === "round" || state.phase === "game_over") push("", ...roundLines(state))
  if (state.phase === "reward" && state.reward) {
    const reward = state.reward
    push(
      "",
      `ROUND CLEARED with ${n(round.score)} against ${n(round.target)}${round.solved ? ", word found" : ""}.`,
      // Why a round short of its target is being called cleared, which otherwise
      // reads as a contradiction to an agent with nothing but this text to go on.
      ...(reward.firstGuess ? [ui().reward.firstGuess] : []),
      ...(reward.saved ? [ui().reward.savedBy(relicCard(reward.saved).name)] : []),
      `Reward: $${reward.base} base + $${reward.unusedGuesses} unused guesses + ` +
        `$${reward.interest} interest${reward.relics ? ` + $${reward.relics} relics` : ""} = $${reward.total}`,
    )
  }
  if (state.phase === "game_over") {
    push("", `RUN OVER: ${n(round.score)} fell short of ${n(round.target)}.`)
  }
  if (state.phase === "victory") {
    push("", `RUN WON: all ${STAGES} stages cleared.`)
  }
  if (state.phase === "shop") push("", ...shopLines(state))
  if (state.pack) push("", ...packLines(state))
  if (state.placing) push("", ...placingLines(state))

  push("", ...trayLines(state), "", ...letterLines(state))

  if (state.phase === "shop" || state.phase === "reward") {
    const next = nextRoundLine(state)
    if (next) push("", next)
  }

  if (context.refusal) push("", `LAST COMMAND REFUSED: ${context.refusal}`)
  const legal = context.over ? [] : legalCommands(state)
  push(
    "",
    "LEGAL COMMANDS",
    ...(legal.length ? legal.map((command) => `- ${command}`) : ["  (none: the run is over)"]),
  )
  return lines.join("\n")
}

function standingRules(state: RunState): string[] {
  const level = state.ascension ?? 0
  const rules = rulesFor(state).map((rule) => ascensionCard(rule).text)
  // The endless rung is synthesized rather than listed, so `rulesFor` never
  // returns it; it is asked for by level, the way the ascension picker does.
  const endless = ascensionAt(level)
  if (endless?.endless) rules.push(ascensionCard(endless).text)
  return rules
}

function roundLines(state: RunState): string[] {
  const round = state.round
  const lines: string[] = []
  const used = round.guesses.length
  const left = round.maxGuesses - used

  lines.push(
    `TARGET ${n(round.target)}   BANKED ${n(round.score)}   GUESSES LEFT ${left} of ${round.maxGuesses}`,
  )
  if (!round.done && left > 0) {
    // The readout's fainter bar, in words: where the pile lands if the next guess
    // is the word. Before the guess's own points, which nobody can know yet.
    const bonus = solveBonusFor(state, left - 1)
    lines.push(`Solving with your next guess multiplies the round's pile by ×${bonus}.`)
  }
  if (round.bossId) {
    const card = bossCard(round.bossId)
    lines.push(`BOSS ${card.name}: ${card.text}`)
  } else if (state.phase === "round") {
    // The intro card's stage track names the stage's boss from its first round,
    // and the shops between here and there are where a player prepares for it.
    const card = bossCard(bossForStage(state))
    lines.push(`STAGE BOSS (round 3) ${card.name}: ${card.text}`)
  }

  lines.push("BOARD")
  if (used === 0) lines.push("  (no guesses yet)")
  for (const [index, guess] of round.guesses.entries()) {
    const tiles = guess.tiles
      .map((tile) => `${upper(tile.letter)}=${tile.shown}${tile.promoted ? "(granted)" : ""}`)
      .join(" ")
    const note = guess.note ? `  [${guessNote(guess.note)}]` : ""
    lines.push(
      `  ${index + 1}. ${upper(guess.word)}  ${tiles}  → ${n(guess.chips)} points × ${n(guess.mult)} mult = ${n(guess.score)}${note}`,
    )
  }

  if (round.revealed.some(Boolean)) {
    lines.push(
      `Revealed: ${round.revealed.map((letter) => (letter ? upper(letter) : "_")).join(" ")}`,
    )
  }
  if (round.eliminated.length) {
    lines.push(`Ruled out: ${round.eliminated.map(upper).join(" ")}`)
  }
  if (round.promote)
    lines.push("The Magician is primed: your next guess's first gray becomes yellow.")

  // The keyboard's own reading, which already skips the Magician's granted tile.
  const keys = keyboardColors(round.guesses)
  const byColor = (color: string) =>
    [...keys.entries()]
      .filter(([, shown]) => shown === color)
      .map(([letter]) => upper(letter))
      .sort()
      .join(" ")
  if (keys.size) {
    lines.push(
      `KEYBOARD green: ${byColor("green") || "-"} | yellow: ${byColor("yellow") || "-"} | gray: ${byColor("gray") || "-"}`,
    )
  }
  return lines
}

function describe(item: ShopItem, state: RunState): string {
  const card = describeItem(item, state)
  const level = card.level > 0 ? ` (Lv ${card.level} → ${card.level + 1})` : ""
  const swap = card.swap ? ` WARNING: ${card.swap}` : ""
  const blocked = card.blocked ? " (no free slot)" : ""
  return `${card.title}${level} [${card.tag}]: ${card.text}${swap}${blocked}`
}

function shopLines(state: RunState): string[] {
  const shop = state.shop
  if (!shop) return []
  const lines = ["SHOP"]
  for (const [index, item] of shop.items.entries()) {
    lines.push(
      item ? `  ${index + 1}. $${item.cost}  ${describe(item, state)}` : `  ${index + 1}. (sold)`,
    )
  }
  lines.push(`Reroll costs $${rerollCost(state, shop)}.`)
  return lines
}

function packLines(state: RunState): string[] {
  const pack = state.pack
  if (!pack) return []
  const card = packCard(pack.id)
  const lines = [`PACK ${card.name}: pick ${pack.picks} more, free. Skipping forfeits the rest.`]
  for (const [index, item] of pack.options.entries()) {
    lines.push(item ? `  ${index + 1}. ${describe(item, state)}` : `  ${index + 1}. (taken)`)
  }
  return lines
}

function placingLines(state: RunState): string[] {
  const id = state.placing
  if (!id) return []
  const card = modifierCard(id)
  return [
    `PLACE ${card.name}: ${card.text}`,
    "Choose the letter it sticks to for the rest of the run. A letter already carrying a modifier loses it.",
  ]
}

function trayLines(state: RunState): string[] {
  const relics = state.relics.map((instance, index) => {
    const card = relicCard(instance.id)
    // What a scaling relic has grown to, and only that: the tray's badge, not
    // the instance's bookkeeping. A relic's `data` is whatever it needed to
    // count, keyed in engine words, and a player is never shown it, so a
    // model reading it would be reading past the screen.
    const growth = RELIC_BY_ID.get(instance.id)?.growth?.(instance)
    const grown = growth ? ` (now ${growthBadge(growth)})` : ""
    return `  ${index + 1}. ${card.name}: ${card.text}${grown}`
  })
  const cards = state.consumables.map((instance, index) => {
    const card = consumableCard(instance.id)
    return `  ${index + 1}. ${card.name}: ${card.text}`
  })
  return [
    // The run's own seat count, as the tray draws it: ascension 6 takes one away.
    `RELICS (${state.relics.length}/${difficultyOf(state).relicSlots}, fire left to right)`,
    ...(relics.length ? relics : ["  (none)"]),
    `CONSUMABLES (${state.consumables.length}/${CONSUMABLE_SLOTS})`,
    ...(cards.length ? cards : ["  (none)"]),
  ]
}

function letterLines(state: RunState): string[] {
  const lines: string[] = []
  const values = [...ALPHABET]
    .map((letter) => {
      const entry = state.letters[letter]
      if (entry?.destroyed) return `${upper(letter)}:broken`
      return `${upper(letter)}${baseChips(state, letter)}${entry?.mod ? "*" : ""}`
    })
    .join(" ")
  lines.push(`LETTER POINTS ${values}`)
  // The keyboard marks a modified key and its tip says what the mark does, so
  // this says both: a star on the letter above and the card's sentence here.
  const mods = [...ALPHABET].flatMap((letter) => {
    const id = state.letters[letter]?.mod
    if (!id || state.letters[letter]?.destroyed) return []
    const card = modifierCard(id)
    return [`  ${upper(letter)}*: ${card.name}, ${card.text}`]
  })
  if (mods.length) lines.push("LETTER MODIFIERS", ...mods)

  // Only the upgrades actually bought. Twenty-odd zero levels would be a wall
  // of text the model has to read past on every turn to learn nothing.
  const shapes = Object.keys(state.levels ?? {})
    .filter((id) => levelOf(state, id) > 1 && CATEGORY_BY_ID.has(id))
    .map((id) => `${categoryCard(id).name} Lv ${levelOf(state, id)}`)
  if (shapes.length) lines.push(`WORD SHAPE LEVELS ${shapes.join(", ")}`)
  const ranges = RANGES.filter((range) => rangeLevelOf(state, range.id) > 1).map(
    (range) => `${range.name} Lv ${rangeLevelOf(state, range.id)}`,
  )
  if (ranges.length) lines.push(`ALPHABET RANGE LEVELS ${ranges.join(", ")}`)
  return lines
}

/**
 * The next round's target, for a shop deciding how much to spend, and its
 * stage's boss. The shop screen shows neither, but the intro card does a tap
 * later, in the card and on the stage track above it, and the model has no
 * later tap to wait for. The boss is named whichever round comes next, not only
 * before the boss round itself: the shop after the first round is one of the
 * two a player gets to prepare in, and naming the boss only in the second left
 * the model one.
 */
function nextRoundLine(state: RunState): string | null {
  const last = state.roundIndex === ROUNDS_PER_STAGE - 1
  const stage = last ? state.stage + 1 : state.stage
  const index = last ? 0 : state.roundIndex + 1
  if (stage > STAGES && !state.won) return null
  const next = { ...state, stage }
  const target = roundTarget(next, index)
  const card = bossCard(bossForStage(next))
  const line = `NEXT ROUND stage ${stage}, ${roundName(index)}: target ${n(target)}`
  return index === ROUNDS_PER_STAGE - 1
    ? `${line}, boss ${card.name}: ${card.text}`
    : `${line}. STAGE BOSS (round 3) ${card.name}: ${card.text}`
}

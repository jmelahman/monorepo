/**
 * The benchmark's one input: a line of text, parsed into a telemetry `Step`.
 *
 * A `Step` rather than an `Action` because a model plays at the grain a player
 * thinks at, "guess CRANE", and not at the keyboard's. `Step` is already that
 * grain, `expand` already turns it into keystrokes, and the telemetry replay
 * already validates a list of them against the engine, so an episode is a replay
 * in a format this repo has a reader for.
 *
 * Positions are one-based, as the screen and the refusals count them. The JSON
 * observation is the one place an array index is zero-based, and it carries
 * the legal commands spelled out for exactly that reason, so nobody has to
 * convert.
 */

import type { Action, RunState } from "../engine"
import { ALPHABET, MODIFIER_BY_ID, placeableLetters, rerollCost } from "../engine"
import type { Step } from "../ui/telemetry"

/** Retiring at the victory screen: the one command with no action behind it. */
export type Command = Step | { type: "end" }

export type Parsed = { ok: true; command: Command } | { ok: false; error: string }

/**
 * Forgiving about everything that carries no meaning (case, extra spaces, a
 * trailing full stop, a leading slash) and strict about everything that does.
 * A model that writes "Guess: crane." has said something unambiguous, and
 * refusing it would be scoring its punctuation.
 */
export function parseCommand(raw: string): Parsed {
  const text = raw
    .trim()
    .replace(/^[/>`*-]+\s*/, "")
    .replace(/[.`*]+$/, "")
    .replace(/:/g, " ")
    .trim()
    .toLowerCase()
  const [verb = "", ...rest] = text.split(/\s+/)
  const arg = rest.join(" ")
  const index = (): number | null => {
    if (!/^\d+$/.test(arg)) return null
    const value = Number(arg)
    return value >= 1 ? value - 1 : null
  }
  const needIndex = (make: (i: number) => Command): Parsed => {
    const i = index()
    return i === null
      ? { ok: false, error: `"${verb}" takes a number counted from 1, e.g. "${verb} 1".` }
      : { ok: true, command: make(i) }
  }

  switch (verb) {
    case "guess": {
      const word = arg.replace(/\s+/g, "")
      if (!/^[a-z]+$/.test(word)) {
        return { ok: false, error: 'Write the guess as letters only, e.g. "guess crane".' }
      }
      return { ok: true, command: { type: "guess", word } }
    }
    case "use":
      return needIndex((i) => ({ type: "use_consumable", index: i }))
    case "buy":
      return needIndex((i) => ({ type: "buy", index: i }))
    case "sell":
      return needIndex((i) => ({ type: "sell_relic", index: i }))
    case "pick":
      return needIndex((i) => ({ type: "pick_pack", index: i }))
    case "place": {
      if (!/^[a-z]$/.test(arg)) return { ok: false, error: 'Name one letter, e.g. "place e".' }
      return { ok: true, command: { type: "place_mod", letter: arg } }
    }
    case "collect":
      return { ok: true, command: { type: "collect" } }
    case "reroll":
      return { ok: true, command: { type: "reroll" } }
    case "next":
      return { ok: true, command: { type: "next_round" } }
    case "continue":
      return { ok: true, command: { type: "continue_run" } }
    case "skip":
      return { ok: true, command: { type: "skip_pack" } }
    case "end":
      return { ok: true, command: { type: "end" } }
    default:
      return {
        ok: false,
        error: `Unknown command "${verb}". See LEGAL COMMANDS for what this screen accepts.`,
      }
  }
}

/** The inverse, for the log and the spectator: a step as the model would have typed it. */
export function formatCommand(command: Command): string {
  switch (command.type) {
    case "guess":
      return `guess ${command.word}`
    case "use_consumable":
      return `use ${command.index + 1}`
    case "buy":
      return `buy ${command.index + 1}`
    case "sell_relic":
      return `sell ${command.index + 1}`
    case "pick_pack":
      return `pick ${command.index + 1}`
    case "place_mod":
      return `place ${command.letter}`
    case "collect":
      return "collect"
    case "reroll":
      return "reroll"
    case "next_round":
      return "next"
    case "continue_run":
      return "continue"
    case "skip_pack":
      return "skip"
    case "end":
      return "end"
  }
}

/**
 * Keystrokes back into commands, for a player that speaks the engine's
 * language rather than a model's: the blind baseline hands over `Action[]`, and
 * it has to reach the `Session` through the same door a model does, or its line
 * in the report is measuring a different interface. The same fold `stepFor`
 * does for the telemetry log, carried across a whole list because nothing here
 * holds a live draft to read the word off.
 */
export function commandsFor(actions: readonly Action[]): Command[] {
  const commands: Command[] = []
  let draft = ""
  for (const action of actions) {
    switch (action.type) {
      case "type_letter":
        draft += action.letter
        break
      case "backspace":
        draft = draft.slice(0, -1)
        break
      case "submit":
        commands.push({ type: "guess", word: draft })
        draft = ""
        break
      case "start_run":
        break
      default:
        commands.push(action)
    }
  }
  return commands
}

/**
 * What this screen will plausibly take, as the commands themselves.
 *
 * A hint and not a second rulebook: it is derived from the phase and what is
 * visibly on offer, and the engine's refusal is still what decides. So a buy the
 * shop will turn down for a full tray is listed, because a player looking at
 * the shelf would try it too, and learning why it bounced is part of the game.
 * What is *not* listed is anything that is refused on every screen of this kind,
 * like an item nobody can afford: a list padded with those would be one the
 * model learns to read past.
 */
export function legalCommands(state: RunState): string[] {
  if (state.placing) {
    const modifier = MODIFIER_BY_ID.get(state.placing)
    const letters = modifier ? placeableLetters(state, modifier) : [...ALPHABET]
    return [`place <letter>  (one of: ${letters.join(" ")})`]
  }
  if (state.pack) {
    const picks = state.pack.options.flatMap((item, i) => (item ? [`pick ${i + 1}`] : []))
    return [...picks, "skip"]
  }

  const consumables = state.consumables.map((_, i) => `use ${i + 1}`)
  switch (state.phase) {
    case "round":
      return state.round.done ? [] : ["guess <five-letter word>", ...consumables]
    case "reward":
      return ["collect"]
    case "shop": {
      const shop = state.shop
      const buys = (shop?.items ?? []).flatMap((item, i) =>
        item && item.cost <= state.gold ? [`buy ${i + 1}`] : [],
      )
      const sells = state.relics.map((_, i) => `sell ${i + 1}`)
      const reroll = shop && rerollCost(state, shop) <= state.gold ? ["reroll"] : []
      return [...buys, ...sells, ...reroll, "next"]
    }
    case "victory":
      return ["continue", "end"]
    case "game_over":
      return []
  }
}

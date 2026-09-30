import { describe, expect, it } from "vitest"
import type { Command } from "../../src/bench/commands"
import { commandsFor, formatCommand, parseCommand } from "../../src/bench/commands"
import type { Action } from "../../src/engine"
import { expand } from "../../src/ui/telemetry"

const EVERY: Command[] = [
  { type: "guess", word: "crane" },
  { type: "use_consumable", index: 1 },
  { type: "buy", index: 0 },
  { type: "sell_relic", index: 4 },
  { type: "drop_consumable", index: 1 },
  { type: "pick_pack", index: 2 },
  { type: "place_mod", letter: "e" },
  { type: "collect" },
  { type: "reroll" },
  { type: "next_round" },
  { type: "continue_run" },
  { type: "skip_pack" },
  { type: "end" },
]

describe("the command grammar", () => {
  it("reads back everything it writes", () => {
    for (const command of EVERY) {
      expect(parseCommand(formatCommand(command))).toEqual({ ok: true, command })
    }
  })

  // What a model actually sends, from a transcript's worth of habits. Each one
  // is unambiguous, and refusing it would be scoring punctuation.
  it("forgives what carries no meaning", () => {
    expect(parseCommand("Guess: CRANE.")).toEqual({
      ok: true,
      command: { type: "guess", word: "crane" },
    })
    expect(parseCommand("  /buy   2 ")).toEqual({ ok: true, command: { type: "buy", index: 1 } })
    expect(parseCommand("`next`")).toEqual({ ok: true, command: { type: "next_round" } })
    expect(parseCommand("- place E")).toEqual({
      ok: true,
      command: { type: "place_mod", letter: "e" },
    })
  })

  it("refuses what does carry meaning, as a result rather than a throw", () => {
    for (const raw of [
      "",
      "buy",
      "buy 0",
      "buy two",
      "sell -1",
      "place ab",
      "guess cr4ne",
      "dance",
    ]) {
      const parsed = parseCommand(raw)
      expect(parsed.ok, raw).toBe(false)
      if (!parsed.ok) expect(parsed.error.length).toBeGreaterThan(0)
    }
  })

  it("folds keystrokes into the commands that expand back to them", () => {
    const actions: Action[] = [
      { type: "type_letter", letter: "c" },
      { type: "type_letter", letter: "r" },
      { type: "type_letter", letter: "x" },
      { type: "backspace" },
      { type: "type_letter", letter: "a" },
      { type: "type_letter", letter: "n" },
      { type: "type_letter", letter: "e" },
      { type: "submit" },
      { type: "collect" },
      { type: "buy", index: 3 },
    ]
    const commands = commandsFor(actions)
    expect(commands).toEqual([
      { type: "guess", word: "crane" },
      { type: "collect" },
      { type: "buy", index: 3 },
    ])
    // Modulo the backspace, which is the one thing a replay has no use for.
    expect(expand(commands as Parameters<typeof expand>[0])).toEqual(
      actions.filter(
        (action, i) => action.type !== "backspace" && actions[i + 1]?.type !== "backspace",
      ),
    )
  })
})

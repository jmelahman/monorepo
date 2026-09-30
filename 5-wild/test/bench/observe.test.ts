/**
 * The benchmark's one hard promise: the model plays blind. Checked on the
 * structure, not by searching the text for the word, because a guess that
 * finds the answer prints it legitimately, and because five-letter answers
 * include words like STAGE and PLACE that the text form uses as headings.
 */

import { describe, expect, it } from "vitest"
import { commandsFor, parseCommand } from "../../src/bench/commands"
import { observe } from "../../src/bench/observe"
import { Session } from "../../src/bench/session"
import { blindPlayer } from "../helpers/blind"
import { realWords } from "../helpers/words"

/** Every key anywhere in a parsed JSON value. */
function keys(value: unknown, into = new Set<string>()): Set<string> {
  if (Array.isArray(value)) for (const item of value) keys(item, into)
  else if (value && typeof value === "object") {
    for (const [key, inner] of Object.entries(value)) {
      into.add(key)
      keys(inner, into)
    }
  }
  return into
}

/** A whole blind run, observed at every decision, in both forms. */
function observed(seed: number): Array<{ phase: string; text: string; json: string }> {
  const session = new Session(seed, realWords)
  const bot = blindPlayer("solver")
  const seen: Array<{ phase: string; text: string; json: string }> = []
  while (!session.done) {
    seen.push({
      phase: session.state.phase,
      text: session.observe("text"),
      json: session.observe("json"),
    })
    const actions = bot.next(session.state, realWords)
    if (!actions) break
    for (const command of commandsFor(actions)) {
      if (!session.apply(command).ok || session.done) break
    }
  }
  return seen
}

describe("observations", () => {
  const runs = [3, 7, 10].map(observed)

  it("never carry the answer as a field", () => {
    for (const run of runs) {
      for (const { json } of run) expect(keys(JSON.parse(json)).has("answer")).toBe(false)
    }
  })

  it("list only commands the grammar can read", () => {
    for (const run of runs) {
      for (const { json } of run) {
        const { legal } = JSON.parse(json) as { legal: string[] }
        for (const line of legal) {
          // Placeholders are for a reader; fill them the way a reader would.
          const filled = line.replace("<five-letter word>", "crane").replace(/<letter>.*$/, "e")
          expect(parseCommand(filled).ok, line).toBe(true)
        }
      }
    }
  })

  it("reach every phase a run passes through", () => {
    const phases = new Set(runs.flat().map((seen) => seen.phase))
    for (const phase of ["round", "reward", "shop"]) expect(phases.has(phase)).toBe(true)
  })

  it("render the text form without leaving a placeholder or an undefined", () => {
    for (const run of runs) {
      for (const { text } of run) {
        expect(text).not.toMatch(/undefined|NaN|\[object Object\]/)
        expect(text).toContain("LEGAL COMMANDS")
      }
    }
  })

  it("offer nothing once the harness has ended the run", () => {
    const session = new Session(1, realWords)
    const json = observe(session.state, "json", { over: true })
    expect((JSON.parse(json) as { legal: string[] }).legal).toEqual([])
  })
})

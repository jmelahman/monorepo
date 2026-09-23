import { describe, expect, it } from "vitest"
import type { MetaState } from "../../src/ui/meta"
import { loadMeta } from "../../src/ui/meta"
import { streakLine } from "../../src/ui/views"

/**
 * The record screen is a view and views are not testable here, since there is no DOM
 * in node. What *is* testable is the sentence the view puts in it, which is why
 * it is a function rather than three ternaries inline: the streak is the one
 * figure on that screen that reads differently depending on where the player
 * stands relative to their own best, and getting that wrong is a bug nobody
 * would see until they were having their best evening at the game.
 */
const record = (over: Partial<MetaState>): MetaState => {
  // `loadMeta` with nothing under the key is the cheapest honest blank record,
  // and it stays right when a field is added.
  const items = new Map<string, string>()
  Object.defineProperty(globalThis, "localStorage", {
    value: { getItem: (key: string) => items.get(key) ?? null, setItem: () => {} },
    configurable: true,
  })
  const fresh = loadMeta()
  Reflect.deleteProperty(globalThis, "localStorage")
  return { ...fresh, ...over }
}

describe("the streak, in a sentence", () => {
  it("says nothing has been found before anything has", () => {
    expect(streakLine(record({}))).toBe("No answer found yet.")
    // Rounds played and every one of them missed is still no answer found. The
    // line is about solves, and there are none to report.
    expect(streakLine(record({ missed: 9 }))).toBe("No answer found yet.")
  })

  it("reports the gap when the player is off their best", () => {
    expect(streakLine(record({ streak: 3, bestStreak: 14 }))).toBe(
      "Longest streak: 14 in a row, 3 now.",
    )
  })

  it("drops the second number when there is no streak running", () => {
    // "14 in a row, 0 now" is a true sentence that reads as an accusation.
    expect(streakLine(record({ streak: 0, bestStreak: 14 }))).toBe("Longest streak: 14 in a row.")
  })

  it("says so rather than reporting a gap of nothing at the record", () => {
    // The moment worth getting right: level with the best and still playing.
    // "14 in a row, 14 now" is correct and reads as a bug.
    expect(streakLine(record({ streak: 14, bestStreak: 14 }))).toBe(
      "14 solved in a row, the longest yet, and still going.",
    )
  })
})

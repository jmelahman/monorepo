import { describe, expect, it } from "vitest"
import {
  BOSSES,
  CATEGORIES,
  CONSUMABLES,
  ETCHINGS,
  MODIFIERS,
  PACKS,
  RELICS,
  ROUNDS_PER_STAGE,
} from "../../src/engine"
import { CATALOGS, LANG_NAMES } from "../../src/ui/lang"
import { en } from "../../src/ui/lang/en"
import { LANGS } from "../../src/ui/lang/types"

/**
 * That every language says everything English says, and says it in the same
 * shape.
 *
 * `Strings` already catches most of this at build time, and where it does this
 * file is redundant on purpose — a compile error is a better version of the same
 * check and costs nothing to keep both. What it catches is the part the type
 * cannot see, which is larger than it looks:
 *
 * Six of the eight content tables are `Record<string, Card>`, because their ids
 * are strings the content files own rather than a union the type system knows.
 * `Record<string, Card>` is satisfied by `{}`. So a Spanish catalog missing
 * every relic in the game compiles, ships, and shows a shelf of slugs.
 *
 * And a *sentence* is a function, whose arity is part of its meaning: a
 * translator who writes `(chips) => ...` where English wrote
 * `(chips, mult) => ...` has quietly dropped a number out of a card, and the
 * screen shows a sentence that is fluent, plausible and wrong. Comparing arity
 * is the cheapest thing that notices.
 *
 * The comparison is against `en` in both directions, so a key invented in one
 * language fails here too. That is the direction people forget: an extra key is
 * dead prose that reads as translated work and is never shown to anyone.
 */

/**
 * A leaf's kind, at the resolution this file can compare.
 *
 * Functions carry their arity because that is the operand count. `length` stops
 * at the first defaulted or rest parameter, and nothing in `Strings` uses either,
 * so it is exact here and would only ever be lenient.
 */
function kind(value: unknown): string {
  if (typeof value === "function") return `(${value.length})`
  if (Array.isArray(value)) return `[${value.length}]`
  if (value === null) return "null"
  return typeof value
}

/**
 * Every path in a catalog, flattened to `a.b.c` with the kind that sits there.
 *
 * `ui.units` is skipped rather than compared, and it is the only exclusion. The
 * ladder is genuinely a different length per language — it is written out past
 * anything a run reaches, and how far past is nobody's business but the
 * language's — so its own check is below and asks the two things that matter.
 */
function shape(strings: unknown, path = ""): Record<string, string> {
  if (path === "ui.units") return {}
  const value = strings as Record<string, unknown>
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { [path]: kind(strings) }
  }
  const out: Record<string, string> = {}
  for (const [key, child] of Object.entries(value)) {
    Object.assign(out, shape(child, path ? `${path}.${key}` : key))
  }
  return out
}

/** Every path holding an empty string, which the shape above cannot tell from prose. */
function blanks(strings: unknown, path = ""): string[] {
  if (typeof strings === "string") return strings === "" ? [path] : []
  if (typeof strings !== "object" || strings === null) return []
  return Object.entries(strings).flatMap(([key, child]) =>
    blanks(child, path ? `${path}.${key}` : key),
  )
}

const REFERENCE = shape(en)

/**
 * The ids the catalogs are keyed by, off the engine's own tables rather than off
 * `en`, so a card added to the game fails in four languages at once instead of
 * passing in all four because none of them has it.
 */
const IDS: Record<string, readonly string[]> = {
  relic: RELICS.map((entry) => entry.id),
  boss: BOSSES.map((entry) => entry.id),
  modifier: MODIFIERS.map((entry) => entry.id),
  consumable: CONSUMABLES.map((entry) => entry.id),
  pack: PACKS.map((entry) => entry.id),
  category: CATEGORIES.map((entry) => entry.id),
  etching: ETCHINGS.map((entry) => entry.id),
}

describe.each(LANGS)("the %s catalog", (lang) => {
  const strings = CATALOGS[lang]()

  it("has exactly the keys English has, no more and no fewer", () => {
    // One assertion over the whole tree rather than a walk that stops at the
    // first miss: a half-finished catalog is missing a screen at a time, and the
    // useful failure is the list of what is missing, not its alphabetical first.
    expect(shape(strings)).toEqual(REFERENCE)
  })

  for (const [table, ids] of Object.entries(IDS)) {
    it(`covers every ${table} the game ships`, () => {
      const table_ = strings[table as keyof typeof strings] as Record<string, unknown>
      expect(Object.keys(table_).sort()).toEqual([...ids].sort())
    })
  }

  it("names itself the way the picker names it", () => {
    // `endonym` and `LANG_NAMES` are the same word written down twice — the
    // picker cannot read the catalogs, because three of them are the English one
    // until their prose lands, and a button reading "English" under a flag for
    // French is the one failure of this feature nobody would need to be told
    // about. So they are pinned to each other instead.
    expect(strings.endonym).toBe(LANG_NAMES[lang])
    expect(strings.tag).toBe(lang)
  })

  it("has a name for every round in a stage", () => {
    expect(strings.round).toHaveLength(ROUNDS_PER_STAGE)
  })

  it("writes its abbreviation ladder far enough out to outlast a run", () => {
    // Read positionally by `formatNumber`, so a language that stops early does
    // not degrade — it misprints every rung above where it stopped, calling a
    // milliard a million. English's length is the floor because English is the
    // one that has been played to the top.
    expect(strings.ui.units.length).toBeGreaterThanOrEqual(en.ui.units.length)
    expect(strings.ui.units.filter((unit) => unit === "")).toEqual([])
  })

  it("leaves no sentence blank", () => {
    // The shape check above is satisfied by `""`, which is exactly the shape a
    // half-finished catalog takes when a screen is stubbed out to come back to:
    // it has the key, it has the type, and it shows nothing. Every bare string
    // in `Strings` is words except `ui.common.none`, which is an em dash.
    expect(blanks(strings)).toEqual([])
  })
})

it("ships a catalog for every language the picker offers", () => {
  // The whole file is a `describe.each` over `LANGS`, which would pass just as
  // happily over three languages as over four. This is what makes the sweep
  // above mean anything.
  expect(Object.keys(CATALOGS).sort()).toEqual([...LANGS].sort())
})

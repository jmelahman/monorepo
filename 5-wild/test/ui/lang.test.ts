import { describe, expect, it } from "vitest"
import type { Ascension } from "../../src/engine"
import {
  ASCENSIONS,
  ascensionAt,
  BOSSES,
  CATEGORIES,
  CONSUMABLES,
  difficultyAt,
  ETCHINGS,
  MODIFIERS,
  PACKS,
  RELICS,
  ROUNDS_PER_STAGE,
} from "../../src/engine"
import { formatNumber, setUnits } from "../../src/ui/format"
import type { Lang } from "../../src/ui/lang"
import {
  ascensionCard,
  bossCard,
  categoryCard,
  consumableCard,
  etchingCard,
  keyRows,
  LAYOUTS,
  modifierCard,
  packCard,
  pickLang,
  readLang,
  relicCard,
  roundName,
  useLang,
} from "../../src/ui/lang"
import { en } from "../../src/ui/lang/en"

/**
 * That the catalog covers the content, which is the thing the content tables
 * used to guarantee by holding their own prose.
 *
 * The guarantee did not survive the move intact and this is the half of it that
 * had to be rebuilt here. `Strings` still catches a *missing table* and, for the
 * two keyed by a union, a missing row; what it cannot see is a row keyed by
 * `string`, where `Record<string, Card>` is happy with any set of keys at all.
 * So the ids come off the engine's own tables and every one is looked up.
 *
 * The uniqueness checks came out of `relics.test.ts`, `modifiers.test.ts` and
 * `packs.test.ts`, where they sat beside the price checks and asserted that two
 * cards never share a name. That is still worth asserting and it stopped being
 * the engine's business to assert: a duplicate name is a shop the player cannot
 * read, which is a fact about the catalog in one language.
 *
 * `catalogs.test.ts` looks like the same sweep and is not, so neither replaces
 * the other: it compares the four catalogs to each other as data, and this one
 * goes through `relicCard` and its siblings — the path the views actually take,
 * `missing()` fallback and all. A lookup that resolved to the wrong table would
 * pass there and fail here.
 */

/** Every table, as the two things this file asks of each: its ids and its cards. */
const TABLES = {
  relic: { ids: RELICS.map((entry) => entry.id), card: relicCard },
  boss: { ids: BOSSES.map((entry) => entry.id), card: bossCard },
  modifier: { ids: MODIFIERS.map((entry) => entry.id), card: modifierCard },
  consumable: { ids: CONSUMABLES.map((entry) => entry.id), card: consumableCard },
  pack: { ids: PACKS.map((entry) => entry.id), card: packCard },
  category: { ids: CATEGORIES.map((entry) => entry.id), card: categoryCard },
  etching: {
    ids: ETCHINGS.map((entry) => entry.id),
    // Etchings say their own chip value, so the card is built from the entry
    // rather than from the id alone. Reaching back through the table is what
    // makes the sentence a restatement of the balance number instead of a copy
    // of it: a nerf moves the prose with it and this test never notices.
    card: (id: string) => {
      const etching = ETCHINGS.find((entry) => entry.id === id)
      if (!etching) throw new Error(`no etching ${id}`)
      return etchingCard(etching)
    },
  },
} as const

describe("the English catalog", () => {
  for (const [table, { ids, card }] of Object.entries(TABLES)) {
    it(`names and describes every ${table}`, () => {
      for (const id of ids) {
        const entry = card(id)
        // The `missing` fallback returns the id as the name and nothing as the
        // text, so an empty description is exactly what a missed lookup looks
        // like. Both halves are asserted because a card with a name and no
        // sentence is the shape a half-finished translation takes.
        expect(entry.name, `${table} ${id} has no name`).not.toBe("")
        expect(entry.name, `${table} ${id} is unnamed and fell back to its id`).not.toBe(id)
        expect(entry.text.length, `${table} ${id} has no text`).toBeGreaterThan(0)
      }
    })

    it(`gives every ${table} a distinct name`, () => {
      const names = ids.map((id) => card(id).name)
      expect(new Set(names).size, names.join(" · ")).toBe(names.length)
    })
  }

  it("names and describes every written ascension", () => {
    for (const rule of ASCENSIONS) {
      const card = ascensionCard(rule)
      expect(card.name, `ascension ${rule.level} has no name`).not.toBe("")
      expect(card.text.length, `ascension ${rule.level} has no text`).toBeGreaterThan(0)
    }
    const names = ASCENSIONS.map((rule) => ascensionCard(rule).name)
    expect(new Set(names).size).toBe(names.length)
  })

  it("says the endless rung as the number the engine worked out", () => {
    const rung = ascensionAt(14) as Ascension
    const card = ascensionCard(rung)
    // Both operands, because the sentence is the only place a player ever sees
    // either: the step, which never changes, and the running total, which is
    // why "another 8%" alone stops being useful by the third rung.
    expect(card.text).toContain("8%")
    expect(card.text).toContain(`×${difficultyAt(14).targets.toFixed(2)}`)
  })

  it("has exactly one name per round in a stage", () => {
    expect(en.round).toHaveLength(ROUNDS_PER_STAGE)
    for (let index = 0; index < ROUNDS_PER_STAGE; index++) {
      expect(roundName(index).length, `round ${index} has no name`).toBeGreaterThan(0)
    }
  })
})

/**
 * Choosing a language, which is the half of this that runs before any prose is
 * read and is the half a browser cannot be relied on to get right.
 *
 * `loadLang` and `setLang` are not here: they are the same decisions wrapped in
 * `localStorage` and `document`, neither of which exists in this runner. What is
 * testable is what they wrap, which is why the parsing and the negotiation are
 * their own exported functions rather than lines inside them.
 */
describe("picking a language", () => {
  it("takes a stored setting over anything the browser says", () => {
    // The one of the two the player chose on purpose, so it wins even against a
    // browser insisting on something else, which is the ordinary case after the
    // first launch: the setting is only stored because it was set.
    expect(pickLang("de", ["es-ES", "es"])).toBe("de")
  })

  it("falls through a junk setting to the browser rather than to English", () => {
    // A blocked or scribbled-on store should not cost a Spanish reader the
    // negotiation they would have got with no store at all.
    expect(pickLang("klingon", ["es-ES"])).toBe("es")
    expect(pickLang(null, ["fr-CA"])).toBe("fr")
  })

  it("matches on the primary subtag, so a region is never a reason for English", () => {
    // This game has one Spanish. `es-419` is Latin American Spanish and there is
    // nothing here for it to be different from.
    expect(pickLang(null, ["es-419"])).toBe("es")
    expect(pickLang(null, ["DE-AT"])).toBe("de")
  })

  it("walks the preference list in order, skipping what it does not have", () => {
    // `navigator.languages` is ranked, and the ranking is the whole point of it:
    // a reader who lists Italian first and French second wants French, not the
    // language the game happens to have been written in.
    expect(pickLang(null, ["it", "fr", "en"])).toBe("fr")
    expect(pickLang(null, ["it", "pt"])).toBe("en")
    expect(pickLang(null, [])).toBe("en")
  })

  it("reads a language or nothing, never a guess", () => {
    expect(readLang("fr")).toBe("fr")
    expect(readLang("FR")).toBe(null)
    expect(readLang(null)).toBe(null)
    expect(readLang("")).toBe(null)
  })
})

describe("the keyboard each language types on", () => {
  it("gives every language all 26 letters exactly once", () => {
    // The check that matters, and the only one: a typo in a layout is a letter
    // that cannot be typed, on a board whose whole input is typing letters, and
    // it would show up as a word that mysteriously cannot be entered rather
    // than as anything that looks like a keyboard bug.
    for (const [lang, rows] of Object.entries(LAYOUTS)) {
      const letters = rows.join("")
      expect(letters.length, `${lang} has ${letters.length} keys`).toBe(26)
      expect(new Set(letters).size, `${lang} repeats a letter`).toBe(26)
      expect(letters.split("").sort().join(""), lang).toBe("abcdefghijklmnopqrstuvwxyz")
    }
  })

  it("lays them out the way each language's keyboards do", () => {
    expect(LAYOUTS.en[0]).toBe("qwertyuiop")
    // Spanish keyboards are QWERTY. The Ñ they also have is not here for the
    // same reason CAFÉ is spelled CAFE: accents fold to base letters.
    expect(LAYOUTS.es).toEqual(LAYOUTS.en)
    expect(LAYOUTS.fr[0]).toBe("azertyuiop")
    expect(LAYOUTS.de[0]).toBe("qwertzuiop")
  })

  it("follows the language in force", () => {
    useLang("fr")
    expect(keyRows()[0]).toBe("azertyuiop")
    useLang("en")
    expect(keyRows()[0]).toBe("qwertyuiop")
  })
})

describe("the abbreviation ladder", () => {
  it("comes from the catalog, so a language can rename its own rungs", () => {
    // Pushed into `format.ts` by `useLang` rather than read from it, because
    // every catalog imports `formatNumber` to build its own sentences and a read
    // back would be a cycle. This asserts the push actually happens.
    useLang("en")
    expect(formatNumber(2_200_000_000)).toBe("2.2B")
    setUnits(["k", "M", "Md"])
    expect(formatNumber(2_200_000_000)).toBe("2.2Md")
    // And that choosing a language puts its own ladder back, rather than leaving
    // whatever the last one set — which is the failure this guards, since the
    // ladder is the one piece of prose that lives outside the catalog once it is
    // in force.
    useLang("es")
    expect(formatNumber(2_200_000_000)).toBe("2.2MM")
    useLang("de")
    expect(formatNumber(2_200_000_000)).toBe("2.2Mrd")
    useLang("en")
  })

  it("says what each language means by this many zeroes", () => {
    // Three different answers for one number, which is the whole reason the
    // ladder is prose rather than notation. English is short scale: `10^9` is a
    // billion. Spanish, French and German are long scale, where `10^9` is a
    // thousand million — `mil millones`, `milliard`, `Milliarde` — and a
    // `billón` / `billion` / `Billion` is `10^12`, a thousand times larger than
    // the English word it is spelled like. A catalog that copied English's `B`
    // would be off by three orders of magnitude and look right.
    const rungs: Record<Lang, string> = { en: "B", es: "MM", fr: "Md", de: "Mrd" }
    for (const [lang, rung] of Object.entries(rungs)) {
      useLang(lang as Lang)
      expect(formatNumber(2_200_000_000), lang).toBe(`2.2${rung}`)
    }
    useLang("en")
  })
})

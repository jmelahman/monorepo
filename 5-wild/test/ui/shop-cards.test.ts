import { describe, expect, it } from "vitest"
import type { ModId, RunState, ShopItem, WordSource } from "../../src/engine"
import { CONSUMABLE_SLOTS, difficultyOf, startRun } from "../../src/engine"
import { describeItem } from "../../src/ui/views"

const words: WordSource = { answers: ["braid"], allowed: new Set(["braid", "crane"]) }

const fresh = (): RunState => startRun(1, words).state

/**
 * One item of every kind the shelf can stock, with real ids. Hand-written rather
 * than rolled out of `rollShop`, because the point of the list is that it is
 * exhaustive: a kind added to `ShopItem` and not added here fails to compile,
 * which is the only moment anyone is thinking about the card that draws it.
 */
const ONE_OF_EACH: Record<ShopItem["kind"], ShopItem> = {
  relic: { kind: "relic", id: "snowball", cost: 6 },
  consumable: { kind: "consumable", id: "oracle", cost: 4 },
  etch: { kind: "etch", id: "etch_vowels", cost: 5 },
  level: { kind: "level", id: "twinned", cost: 8 },
  range: { kind: "range", id: "range_ae", cost: 7 },
  mod: { kind: "mod", id: "steel", cost: 6 },
  pack: { kind: "pack", id: "relic", cost: 10 },
}

describe("what a shop card says it is", () => {
  it("tags every kind the shelf can stock, and explains every tag", () => {
    const state = fresh()
    for (const [kind, item] of Object.entries(ONE_OF_EACH)) {
      const { tag, tip } = describeItem(item, state)
      // A blank tag is the failure this exists to catch: the card still draws,
      // still buys and still reads almost right, so nothing else would notice.
      expect(tag, `${kind} tag`).not.toBe("")
      // Same failure one layer down. A missing tip is an inert ticket: the
      // hover panel simply never opens, which looks like nothing at all.
      expect(tip, `${kind} tip`).not.toBe("")
      // The panel is `pre-line`, so a tip wrapped in the source would arrive
      // with the source's own line breaks in it.
      expect(tip, `${kind} tip`).not.toContain("\n")
      // A tip is read in the second the pointer rests on the ticket, over the
      // top of the shelf it is explaining. The limit is what one plain sentence
      // fits in, and anything that wants more of the player's attention than
      // that belongs in the rules sheet or the codex, where they went to read.
      expect(tip.length, `${kind} tip length`).toBeLessThanOrEqual(80)
      // The panel's voice is plain speech, and a dash in it is the sound of
      // copy written to look considered rather than to be read quickly.
      expect(tip, `${kind} tip`).not.toContain("—")
    }
  })

  it("says the kind and leaves the rarity to the color", () => {
    const state = fresh()
    // Snowball is rare, Keystone uncommon, Head Start common, one tag between
    // them. Rarity is on the border, on the tag's own ink and on the tray the
    // relic is bound for; saying it a fourth time cost the name room on the
    // line and pushed the kind, which is what the tag is for, into second place.
    for (const id of ["snowball", "keystone", "head_start"]) {
      expect(describeItem({ kind: "relic", id, cost: 6 }, state).tag, id).toBe("Relic")
    }
    // The rarity itself still comes back, because the card is colored by it.
    expect(describeItem(ONE_OF_EACH.relic, state).rarity).toBe("rare")
  })

  /**
   * A range card is bought for the letters it covers, and "A–E" is the one form
   * of that fact a player has to expand themselves: mid-shop, under a price,
   * against their own vocabulary. F–M is the case that matters: nobody reciting
   * the endpoints thinks of I or L, which are the letters that decide whether
   * the level is worth $7.
   */
  it("spells a range out letter by letter rather than naming its endpoints", () => {
    const state = fresh()
    const slices: [string, string][] = [
      ["range_ae", "A B C D E"],
      ["range_fm", "F G H I J K L M"],
      ["range_nr", "N O P Q R"],
      ["range_sz", "S T U V W X Y Z"],
    ]
    for (const [id, letters] of slices) {
      const { text } = describeItem({ kind: "range", id, cost: 7 }, state)
      expect(text, id).toBe(`${letters} are worth +4 chips per level`)
    }
  })

  it("keeps a destroyed letter in the list, because the slice is fixed", () => {
    // Dropping C would read as a differently-cut range rather than as a fact
    // about this run, and the title above it still says A–E. Where a letter has
    // died is a mark on its key, and has been since the moment it broke.
    const state = fresh()
    const c = state.letters.c
    if (!c) throw new Error("no such letter: c")
    const broken: RunState = {
      ...state,
      letters: { ...state.letters, c: { ...c, destroyed: true } },
    }
    expect(describeItem(ONE_OF_EACH.range, broken).text).toBe(
      "A B C D E are worth +4 chips per level",
    )
  })

  it("turns the tag into a warning when there is nowhere to put the card", () => {
    const state = fresh()
    const full: RunState = {
      ...state,
      relics: Array.from({ length: difficultyOf(state).relicSlots }, () => ({ id: "snowball" })),
      consumables: Array.from({ length: CONSUMABLE_SLOTS }, () => ({ id: "oracle" })),
    }

    // The relic is the kind that does not, and the assertion is here rather than
    // absent because that is a decision and not an oversight. A full tray is a
    // fact about the run, so the tag fired on every relic on the shelf at once,
    // said what the tray below was already showing by having no empty seat, and
    // said it about a purchase that selling on this same screen makes room for.
    // The engine still refuses the buy; the toast is where the player hears it.
    const relic = describeItem(ONE_OF_EACH.relic, full)
    expect(relic.tag).toBe("Relic")
    expect(relic.blocked).toBe(false)

    // The consumable keeps it, because nothing on the shop screen empties a full
    // hand, since a card is used in a round, so the tap has no answer but this one.
    const card = describeItem(ONE_OF_EACH.consumable, full)
    expect(card.tag).toBe("Consumable · slots full")
    expect(card.blocked).toBe(true)

    // The kinds that need no seat are never blocked, however full the run is:
    // an etching lands on letters that are always there.
    expect(describeItem(ONE_OF_EACH.etch, full).blocked).toBe(false)
    expect(describeItem(ONE_OF_EACH.pack, full).blocked).toBe(false)
  })

  /**
   * An aimed modifier landing on a letter that already carries one destroys what
   * is there, for the rest of the run, the moment the card is tapped. It is the
   * only thing the shelf sells that takes something away, and the line saying so
   * is the only warning between a $12 Anchor and a thumb.
   *
   * Held by a test because it is invisible when it breaks: the card still draws,
   * still buys, still reads as an entirely sensible offer, and the only way to
   * find out the line went missing is to be the player who lost the Anchor.
   */
  describe("the line that says a card would replace something", () => {
    const withMod = (letter: string, mod: ModId): RunState => {
      const state = fresh()
      const entry = state.letters[letter]
      if (!entry) throw new Error(`no such letter: ${letter}`)
      return { ...state, letters: { ...state.letters, [letter]: { ...entry, mod } } }
    }

    it("names the modifier being displaced, with the pip off the key", () => {
      const state = withMod("e", "steel")
      const { swap } = describeItem({ kind: "mod", letter: "e", id: "glass", cost: 9 }, state)
      // The pip and not only the name. "Replaces Steel" asks the player to
      // remember what Steel did; the pip is the mark they have been reading off
      // that key since they bought it.
      expect(swap).toBe("Replaces Steel ×2")
    })

    it("stays out of the body copy, which is about what the card does", () => {
      const state = withMod("e", "steel")
      const { text } = describeItem({ kind: "mod", letter: "e", id: "glass", cost: 9 }, state)
      // Where it used to live, tacked onto the end of the longest sentence on the
      // card and in the same muted ink as the sales pitch.
      expect(text).not.toContain("Steel")
      expect(text).toBe("E scores ×3 mult, and can break when it lands gray")
    })

    it("says nothing when there is nothing to lose", () => {
      // A bare letter, which is most of the alphabet on most visits.
      expect(describeItem({ kind: "mod", letter: "e", id: "glass", cost: 9 }, fresh()).swap).toBe(
        "",
      )
      // The shop's unaimed card, which has not picked a letter yet. The picker
      // is where that question gets asked, and it asks it there.
      expect(describeItem(ONE_OF_EACH.mod, withMod("e", "steel")).swap).toBe("")
      // Nothing else on the shelf displaces anything, whatever the run holds.
      for (const [kind, item] of Object.entries(ONE_OF_EACH)) {
        if (kind === "mod") continue
        expect(describeItem(item, withMod("e", "steel")).swap, `${kind} swap`).toBe("")
      }
    })

    it("says nothing when the card is the modifier already on the letter", () => {
      // `placeableLetters` keeps this off the shelf and out of a pack in the
      // first place, so the card should not exist, but if one ever does, the
      // honest thing for it to say is nothing rather than "Replaces Steel ×2"
      // about the Steel it is not going to move.
      const state = withMod("e", "steel")
      expect(describeItem({ kind: "mod", letter: "e", id: "steel", cost: 8 }, state).swap).toBe("")
    })
  })
})

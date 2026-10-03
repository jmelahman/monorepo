/**
 * What a card on the shelf says, as strings and nothing else.
 *
 * It lived in `views.ts`, beside the card that draws it, and it had a second
 * reader from the day the benchmark landed: `src/bench/observe.ts` puts these
 * same sentences in front of a model, so that a model and a player are reading
 * one shelf. That reader runs under plain Bun, with no Vite in front of it, and
 * `views.ts` is the one module that cannot be loaded there: it imports the
 * pictures, and `emoji.ts` fills its table with `import.meta.glob`, which is a
 * transform Vite performs and not a function anything has at run time. The
 * tests never saw it, because vitest *is* Vite; `bun tools/bench/mcp.ts` died
 * on its first import.
 *
 * So the prose is here, importing the engine and the catalog and no drawing,
 * and it has to stay that way: nothing in this file may reach `dom.ts`, a
 * picture table or anything else that needs a bundler or a document.
 * `test/bench/loads.test.ts` holds the bench's whole import graph to that.
 */

import type { Range, RunState, ShopItem } from "../engine"
import {
  CATEGORY_BY_ID,
  CHIPS_PER_LEVEL,
  CONSUMABLE_SLOTS,
  ETCHING_BY_ID,
  levelOf,
  MODIFIER_BY_ID,
  PACK_BY_ID,
  RANGE_BY_ID,
  RELIC_BY_ID,
  rangeLevelOf,
} from "../engine"
import {
  categoryCard,
  consumableCard,
  etchingCard,
  modifierCard,
  packCard,
  relicCard,
  ui,
} from "./lang"

/**
 * What a range level buys, spelled out letter by letter.
 *
 * Shared between the shelf's card and the rules sheet's list because they are
 * the same sentence about the same four cards, and the sheet is most often
 * opened *from* the shop by a player deciding whether to buy the thing they are
 * looking at. Two copies of it drifted apart once already: the sheet enumerated
 * the letters from the day ranges landed and the card said "A–E letters", which
 * is the shorter of the two and the one being read under a price.
 */
export const rangeText = (range: Range): string =>
  ui().shop.rangeText([...range.letters].join(" ").toUpperCase(), CHIPS_PER_LEVEL)

/**
 * What a card says about itself. Split out from the card because a pack lays out
 * the same items the shop sells, and a Steel E ought to read identically whether
 * it is being bought or being chosen.
 *
 * The tag is the answer to a question the shelf was not answering. Seven kinds of
 * thing are sold here and every one of them was the same rectangle: a name, a
 * sentence, a price. So "The Mint" and "Etch Heavy" were distinguishable only by
 * reading both sentences and knowing the game well enough to classify them. The
 * tag says the kind in one word before the name is read.
 *
 * It also said when a card had nowhere to land. The relic's no longer does.
 * "Relic · tray full" is a fact about the run rather than about the card, so it
 * was never on one card at a time: a full tray tags every relic on the shelf at
 * once, and a shelf of warnings reads as a shelf that is broken rather than as
 * stock. It also spends the name's own line. Five cards across a phone leaves
 * about 64px for a name, and the ticket was taking three words of it to say
 * something the tray directly below is already saying by having no empty seat in
 * it, about a purchase the player can make room for by selling on this very
 * screen. The refusal is said at the till instead, where it is one line about
 * the one card that was actually tapped.
 *
 * The consumable keeps its warning, and the difference is whether the screen can
 * act on it. A card is used in a round, so a full hand is a wall for the whole
 * shop visit, since nothing on this screen empties it, and the tag is the only place
 * that can say so before the gold is committed to a tap that will bounce.
 *
 * Rarity stays a color and never becomes a word. Relics were the exception, with
 * "Uncommon Relic" on the grounds that rarity is an economy there. But a tag
 * that says two things says the second one second, and the kind is what the tag
 * is for. The card was already announcing its rarity three ways over: the
 * border, the tag's own ink, and the tray the relic is bound for. The word was
 * the fourth telling, and the only one charging the name room on the line.
 *
 * The tip is the sentence behind the tag. One word is enough to sort five cards
 * into kinds and not nearly enough to say what a kind *is*. "Alphabet" names
 * nothing to a player who has not already bought one, so the ticket answers the
 * question it provokes when it is hovered or held.
 *
 * One line, and deliberately shorter than the card it hangs off. It says how
 * that kind of thing works and stops: not what this card does, which the body
 * copy under it is already saying, and not the fine print, which is what the
 * rules sheet and the codex are for. A panel that has to be *read* is one the
 * player has stopped hovering by the end of. It covers the shelf it is
 * explaining while it does it, so anything that would not fit in a breath was
 * dropped rather than compressed. The test holds the length.
 *
 * `swap` is the one thing a card says that is about the run rather than about
 * itself: an aimed modifier landing on a letter that is already carrying one
 * destroys what is there, and that is the only line on the shelf describing
 * something the player *loses* by tapping. It gets a field and a line of its own
 * rather than a clause on the end of `text`, because it used to be a clause on
 * the end of `text`, "E scores ×3 mult, and can break when it lands gray,
 * replacing Steel", which buried the consequence at the end of the longest
 * sentence on the card, in the same ink as the sales copy, past the point a
 * shelf of five cards is read to.
 */
export function describeItem(
  item: ShopItem,
  state: RunState,
): {
  title: string
  text: string
  rarity: string
  tag: string
  tip: string
  blocked: boolean
  swap: string
  level: number
} {
  const copy = ui().shop
  let title = ""
  let text = ""
  let swap = ""
  let rarity = "common"
  let tag = ""
  /**
   * Written as one line each. The tip panel is `white-space: pre-line`, so a
   * catalog entry wrapped across source lines would arrive with the source's own
   * breaks in it; `en.ts` concatenates rather than using a template literal for
   * exactly that reason.
   */
  let tip = ""
  /** This one has nowhere to go: the card grows a warning line and its ticket goes red. */
  let blocked = false
  /**
   * The level a word shape or a range stands at now, zero for every other kind.
   * It was the title's tail, "Twinned → Lv 4", which told the player where the
   * card went and not where it came from; `ladderLine` says both.
   */
  let level = 0

  if (item.kind === "pack") {
    const pack = PACK_BY_ID.get(item.id)
    title = packCard(item.id).name
    text = packCard(item.id).text
    // Packs read as the rare thing on the shelf because they are the dearest and
    // the only one that asks a question back.
    rarity = "rare"
    tag = copy.tagPack
    tip = copy.tipPack(pack?.picks ?? 1)
  } else if (item.kind === "relic") {
    const relic = RELIC_BY_ID.get(item.id)
    title = relicCard(item.id).name
    text = relicCard(item.id).text
    rarity = relic?.rarity ?? "common"
    // Never blocked, whatever the tray holds. The refusal still happens. The
    // engine turns the purchase away with "no relic slots free" and the toast
    // says it. It just happens at the till rather than on the card.
    tag = copy.tagRelic
    tip = copy.tipRelic
  } else if (item.kind === "consumable") {
    title = consumableCard(item.id).name
    text = consumableCard(item.id).text
    blocked = state.consumables.length >= CONSUMABLE_SLOTS
    tag = copy.tagConsumable
    tip = copy.tipConsumable
  } else if (item.kind === "mod") {
    const mod = MODIFIER_BY_ID.get(item.id)
    const card = modifierCard(item.id)
    rarity = mod?.rarity ?? "common"
    // One sentence for both versions of this card. The difference between them,
    // which is who picks the letter, is already the difference between the two
    // titles, and the tip is about the kind rather than about the card.
    tip = copy.tipMod
    if (item.letter === undefined) {
      // The shop's version: a card with no letter on it yet. Named for what it
      // is being bought as, a choice, because the price is the price of the
      // choice, and the letter it ends up on is the next screen.
      title = copy.modAnyTitle(card.name)
      // Two whole sentences rather than one with a clause appended, because the
      // letters are a restriction on the choice and a language that puts them
      // before the verb has nowhere to append to.
      text = mod?.letters
        ? copy.modAnyTextOnly(card.text, [...mod.letters].join(" ").toUpperCase())
        : copy.modAnyText(card.text)
      tag = copy.tagLetter
    } else {
      const letter = item.letter.toUpperCase()
      title = copy.modTitle(card.name, letter)
      text = copy.modText(letter, card.text)
      // A letter holds one modifier, so this is sometimes a trade rather than an
      // addition, and that has to be legible before the gold is gone.
      const current = state.letters[item.letter]?.mod
      const held = current ? MODIFIER_BY_ID.get(current) : undefined
      if (held && held.id !== item.id) swap = copy.swap(modifierCard(held.id).name, held.pip)
      // The pack's aimed version. Still a letter card, and the tag says so for
      // the same reason it does in the shop: the name reads "Gold E", which is
      // the letter, not the kind of thing being handed over.
      tag = copy.tagLetter
    }
  } else if (item.kind === "range") {
    const range = RANGE_BY_ID.get(item.id)
    // The one name that stayed in the engine: A–E is the letters it holds, said
    // in the punctuation every language uses for a span, and `ranges.test.ts`
    // asserts it as a fact about the partition rather than as copy.
    title = range ? range.name : copy.fallbackRange
    level = range ? rangeLevelOf(state, item.id) : 0
    // Spelled out rather than named again. "A–E letters" is the title over
    // again in the body's ink, and it asks a player mid-shop to expand a dash
    // into five letters and check their own vocabulary against it, which is
    // the whole decision the card is selling. F–M is the one that settles it:
    // eight letters, and the ones that matter to a guess are the ones nobody
    // recites when they read the endpoints. So it reads exactly the way an
    // etching does, "A E I O U are worth +2 chips", and exactly the way the
    // same four ranges are already listed in the rules sheet.
    //
    // The whole slice, including any letter that has been destroyed. The range
    // is a fixed partition of the alphabet and the title names it as one, so a
    // list that quietly dropped C would read as a differently-cut range rather
    // than as a fact about this run; deadness is a mark on the key, which is
    // where the player has been reading it since the letter broke.
    text = range ? rangeText(range) : ""
    tag = copy.tagRange
    tip = copy.tipRange
  } else if (item.kind === "level") {
    const category = CATEGORY_BY_ID.get(item.id)
    const card = categoryCard(item.id)
    title = category ? card.name : copy.fallbackLevel
    level = category ? levelOf(state, item.id) : 0
    text = category ? copy.levelText(card.name, category.chips, category.mult, category.growth) : ""
    tag = copy.tagShape
    tip = copy.tipShape
  } else {
    // Falls back to something readable rather than to `undefined`, so a save
    // written before etchings were sold by the group degrades quietly.
    const etching = ETCHING_BY_ID.get(item.id)
    title = etching ? etchingCard(etching).name : copy.fallbackEtching
    text = etching ? etchingCard(etching).text : ""
    tag = copy.tagEtching
    tip = copy.tipEtching
  }

  return { title, text, rarity, tag, tip, blocked, swap, level }
}

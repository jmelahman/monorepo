import type {
  Ascension,
  ConsumableNote,
  Etching,
  Growth,
  GuessNote,
  ModId,
  Payout,
  Refusal,
} from "../../engine"
import { setUnits } from "../format"
import { de } from "./de"
import { en } from "./en"
import { es } from "./es"
import { fr } from "./fr"
import type { Card, Lang, Strings } from "./types"
import { LANGS } from "./types"

export type { Card, Lang, Rule, RuleOf, Section, SectionOf, Strings } from "./types"
export { LANGS } from "./types"

/**
 * The catalog in force, as a module-level value rather than an argument.
 *
 * `views.ts` is fifty-odd pure `state → HTMLElement` functions and threading a
 * `Strings` through every one of them would be the largest edit in this change
 * and the least interesting: nothing in there *decides* a language, they all
 * just read the one in force. So it lives here, next to the stylesheet in the
 * list of things the screen reads and does not own, and `setLang` is the only
 * writer.
 *
 * Safe because of the property the whole UI is built on: every dispatch rebuilds
 * the whole screen. There is no half-rendered tree holding strings from before
 * the switch, because there is never a half-rendered tree.
 */
let current: Strings = en

/**
 * Which language was *asked for*, which is not the same question as which
 * catalog answered.
 *
 * Anything that is a fact about the *language* rather than about its prose has
 * to read this, and the keyboard is the one that matters: English and Spanish
 * share QWERTY, so `current` cannot tell them apart even now that both catalogs
 * are written. It mattered more when they were not — for one phase all four
 * settings resolved to `en` and `current` could distinguish nothing at all,
 * which is how a French player got AZERTY months before there was a French
 * sentence to put above it.
 */
let currentLang: Lang = "en"

/**
 * Every catalog, behind a thunk each.
 *
 * The indirection is what made the setting shippable ahead of its translations:
 * for one phase all four entries returned `en`, so an unfilled language was an
 * English screen rather than a screen of `undefined`. Now that the other three
 * are written it is a plain lookup wearing the same shape, and the shape is
 * worth keeping — it is the seam a fifth language is added along, and it is what
 * `catalogs.test.ts` sweeps to check every language against English at once.
 */
export const CATALOGS: Record<Lang, () => Strings> = {
  en: () => en,
  es: () => es,
  fr: () => fr,
  de: () => de,
}

/**
 * Put a language in force, and nothing else.
 *
 * The half of `setLang` that touches no browser, so a test can switch catalogs
 * and the shell can put one up before it has decided anything is worth storing.
 */
export function useLang(lang: Lang): void {
  current = CATALOGS[lang]()
  currentLang = lang
  // The abbreviation ladder is prose the same way a card's name is: "1.35Md" is
  // French for what English writes as "1.35B". `format.ts` cannot read this
  // module, because the catalogs read *it* for their numbers and their plurals,
  // so the units are pushed the other way down the one edge that exists.
  setUnits(current.ui.units)
}

/** What was chosen, for the two things that are about the language, not its text. */
export const lang = (): Lang => currentLang

/**
 * The keyboard, which is a fact about the language and not a translation of
 * anything.
 *
 * Accents are folded to base letters everywhere in this game, so all four
 * alphabets are the same 26 keys and only their order differs: QWERTY for
 * English and Spanish, AZERTY for French, QWERTZ for German. A Spanish keyboard
 * has an Ñ on it and this one does not, for the same reason CAFÉ is spelled
 * CAFE here.
 *
 * The rows are uneven in French, ten and ten and six against English's ten,
 * nine and seven, which the board absorbs: the third row is a flex row with
 * ENTER and DEL on the ends of it.
 *
 * It follows the *setting* and not the run's word list, which is a real fork and
 * is only visible in the window where the two disagree: switch to English
 * mid-French-run and the board goes QWERTY while the answer is still French. The
 * setting is the right one of the two to follow, because a layout is a fact
 * about the hands typing on it rather than about the dictionary being typed at,
 * and a player who has just asked for English has said which keyboard they know.
 * The window closes at the next run either way.
 */
export const LAYOUTS: Record<Lang, readonly string[]> = {
  en: ["qwertyuiop", "asdfghjkl", "zxcvbnm"],
  es: ["qwertyuiop", "asdfghjkl", "zxcvbnm"],
  fr: ["azertyuiop", "qsdfghjklm", "wxcvbn"],
  de: ["qwertzuiop", "asdfghjkl", "yxcvbnm"],
}

export const keyRows = (): readonly string[] => LAYOUTS[currentLang]

const LANG_KEY = "5wild:lang"

/**
 * Whatever was stored or asked for, read as a language this game has.
 *
 * Null rather than a fallback, because the two callers want different things
 * from a miss: a stored value that does not parse should fall through to what
 * the browser prefers, and a browser preference that does not parse should fall
 * through to the next one in the list.
 */
export const readLang = (raw: string | null | undefined): Lang | null =>
  LANGS.find((entry) => entry === raw) ?? null

/**
 * The language to open in, from what was stored and what the reader prefers.
 *
 * A stored setting always wins: it is the one of the two the player chose on
 * purpose. Failing that, `navigator.languages` in order, matched on the primary
 * subtag alone, so `es-419` and `es-ES` both land on Spanish — this game has one
 * Spanish and a regional tag is not a reason to hand a Spanish reader English.
 * Failing that, English, which is the language the game was written in.
 *
 * Nothing consulted the browser at all before this, so a first launch on a
 * Spanish phone has always been an English screen. That is the thing worth
 * fixing here; every launch after the first reads the stored answer.
 */
export function pickLang(stored: string | null, preferred: readonly string[]): Lang {
  const chosen = readLang(stored)
  if (chosen) return chosen
  for (const tag of preferred) {
    const match = readLang(tag.toLowerCase().split("-")[0])
    if (match) return match
  }
  return "en"
}

export function loadLang(): Lang {
  try {
    return pickLang(localStorage.getItem(LANG_KEY), navigator.languages ?? [])
  } catch {
    return "en"
  }
}

/**
 * Choose a language: put it in force, tell the document, and remember it.
 *
 * `document.documentElement.lang` is the one of the three that is not for this
 * game's benefit. It is what a screen reader picks a voice from and what a
 * browser offers to translate against, and it is wrong on every screen until
 * something sets it, since `index.html` can only name the language the file was
 * written in.
 */
export function setLang(lang: Lang): void {
  useLang(lang)
  try {
    document.documentElement.lang = lang
    localStorage.setItem(LANG_KEY, lang)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}

/**
 * What each language calls itself.
 *
 * Endonyms, and the one list in this file that is not per-catalog: a picker that
 * named the languages in the language currently in force would offer "Spanish"
 * to a reader who is looking for the word "Español", which is the only word on
 * that button they are certain to recognize. Proper nouns, so nothing here is
 * translated and nothing here is a `Strings` key.
 */
export const LANG_NAMES: Record<Lang, string> = {
  en: "English",
  es: "Español",
  fr: "Français",
  de: "Deutsch",
}

/**
 * A flag per language, which is a lie everyone reads correctly.
 *
 * A flag is a country and a language is not: English is not the United States'
 * and Spanish is spoken by more people outside Spain than in it, so this is the
 * localization mistake every guide names. It is here anyway, and narrowly: the
 * endonym beside it carries the meaning, and the flag is what makes one button
 * in a stack of four findable at a glance by someone who is scanning rather than
 * reading. Drop the endonym and the objection becomes real.
 *
 * English flies the American flag rather than the British one, which does not
 * answer that objection so much as pick a side of it. Both are wrong in the same
 * way, so the tiebreak is which country's spelling `en.ts` actually writes, and
 * that was settled deliberately a phase ago: color, serialize, catalog. A 🇬🇧
 * over an American catalog is the one version of this that is wrong twice.
 *
 * They also do not render everywhere. Windows has no flag glyphs in its emoji
 * font at all and draws the two regional-indicator letters instead — "US", "ES"
 * — which is a legible degradation precisely because the word is doing the work.
 * Android, which is what the APK runs on, has Noto Color Emoji and draws them.
 */
export const LANG_FLAGS: Record<Lang, string> = {
  en: "🇺🇸",
  es: "🇪🇸",
  fr: "🇫🇷",
  de: "🇩🇪",
}

/** One button, one step per tap, wrapping at the end. As the speed dial. */
export const NEXT_LANG: Record<Lang, Lang> = { en: "es", es: "fr", fr: "de", de: "en" }

/** The whole catalog, for the screens that read more of it than one card. */
export const S = (): Strings => current

/**
 * The screens' own half of it, which is most of what a view reads.
 *
 * A function rather than a bound object for the same reason `current` is a
 * `let`: a module-level `const ui = en.ui` would be the catalog as it stood
 * when this file was first imported, and `setLang` would have nothing to move.
 */
export const ui = (): Strings["ui"] => current.ui

/**
 * Every table is looked up by a plain `string`, even the two whose catalog side
 * is keyed by a union. The completeness guarantee lives on `Strings`, where
 * `Record<ModId, Card>` makes a missing entry a compile error; asking for it
 * again at the call site would only mean the shop, whose `ShopItem.id` is a
 * `string` for every kind but one, had to cast on the way in.
 */
const lookup = (table: Record<string, Card>, id: string): Card => table[id] ?? missing(id)

export const relicCard = (id: string): Card => lookup(current.relic, id)
export const bossCard = (id: string): Card => lookup(current.boss, id)
export const modifierCard = (id: string): Card => lookup(current.modifier, id)
export const consumableCard = (id: string): Card => lookup(current.consumable, id)
export const packCard = (id: string): Card => lookup(current.pack, id)
export const categoryCard = (id: string): Card => lookup(current.category, id)

export function etchingCard(etching: Etching): Card {
  const entry = current.etching[etching.id]
  if (!entry) return missing(etching.id)
  return { name: entry.name, text: entry.text(etching.chips) }
}

/**
 * The authored rungs are a lookup; everything above them is the one synthesized
 * rung, whose two numbers the engine works out and this only writes down.
 */
export function ascensionCard(ascension: Ascension): Card {
  const endless = ascension.endless
  if (endless) {
    const entry = current.ascension.steeper
    return { name: entry.name, text: entry.text(endless.percent, endless.total.toFixed(2)) }
  }
  return current.ascension[ascension.level] ?? missing(`ascension ${ascension.level}`)
}

export const roundName = (index: number): string => current.round[index] ?? ""

/**
 * The one line the engine turns down an action with.
 *
 * The table is half strings and half functions of the operands the code carries,
 * and which of the two a given code is was decided by the engine when it wrote
 * the union; `Refusals` derives it rather than restating it. The cast is the
 * price of that: the entry's parameter type is the operands of *its* code, and
 * nothing at runtime can tell TypeScript that the entry just fetched and the
 * refusal just passed came from the same arm.
 */
export function refusalText(refusal: Refusal): string {
  const entry = current.refusal[refusal.code] as string | ((operands: Refusal) => string)
  return typeof entry === "function" ? entry(refusal) : entry
}

export const growthBadge = (growth: Growth): string => current.event.growth(growth)
export const consumableNote = (note: ConsumableNote): string => current.event.note(note)

/**
 * Both of these take the `string` arm too, which is a value out of a save
 * written before either was structured. Handled here rather than at the four
 * call sites, because the alternative is four ternaries that all mean "this run
 * started under the old build" and would each have to explain it.
 */
export const payoutBadge = (paid: Payout | string): string =>
  typeof paid === "string" ? paid : current.event.payout(paid)
export const guessNote = (note: GuessNote | string): string =>
  typeof note === "string" ? note : current.event.guessNote(note)
export const categoryLevel = (id: string, level: number): string =>
  current.event.categoryLevel(categoryCard(id).name, level)
export const modPlaced = (id: ModId, letter: string): string =>
  current.event.modPlaced(modifierCard(id).name, letter)

/**
 * What a lookup that should not have missed shows instead.
 *
 * The id, which is a slug nobody wants to read, and that is the point: it is
 * legible enough to debug from and ugly enough that it will not be mistaken for
 * copy if it ever reaches a screen. It replaces the `card?.name ?? item.id`
 * fallbacks the views used to spell out one at a time.
 */
const missing = (id: string): Card => ({ name: id, text: "" })

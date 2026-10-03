/**
 * The cards a third time: as emoji, for Classic.
 *
 * Classic drew a card as its line emblem in the kind's one ink, which is the
 * right voice for a button and the wrong one for a thing you own: five seats in
 * a tray were five drawings in at most four colours, told apart by silhouette
 * alone at 20px, and the Smoke Room's sprites had already shown what a relic
 * with its own colours is worth. The sprites themselves are no answer here,
 * since a 12x12 grid with a black outline is the Smoke Room's voice and Classic
 * is the phone's own look. An emoji is the phone's own picture.
 *
 * Bundled, not typed. An emoji as a character is whatever font the machine
 * has: Noto in the APK's WebView, Segoe on Windows, and on Linux under
 * WebKitGTK possibly nothing at all, which is a shelf of boxes. It is also
 * text, so it sits on a baseline and takes a size from a font rather than from
 * the box every layout file here already gives `.emblem`. So these are files,
 * one `<svg>` each in `./emoji/`: Microsoft's Fluent Emoji in its Flat style,
 * the one open set drawn without gradient or shadow, under the MIT licence
 * beside them. Seventy-odd of them are 74 KB before compression, which is the
 * cost, and it is paid once, in the bundle, with no request behind it.
 *
 * A file whose name starts `5w-` is ours. The set has no picture for a quarter
 * of the alphabet, a poker chip, a fingerprint, an arch, a snowball or a glass of water, and those are drawn
 * here in its terms: its 32-unit box, its flat fills, its own greens, blues and
 * greys.
 * Draw a new one only when no emoji will carry the pun; a near miss from the
 * set (a telescope where binoculars were wanted) reads better on a shelf than a
 * perfect subject in a second hand.
 *
 * A card is one subject in every look. Where the set's picture was not what
 * the emblem and the sprite drew (a dog for a paw print, a padlock for a safe
 * door, a clover for a horseshoe), those two were redrawn after it, each in its
 * own hand (and where they had it right, a snowball, the emoji was drawn after them), so a player who changes look does not have to learn the shelf again.
 *
 * The colours are the set's and are not tokens: an emoji is made to sit on a
 * white page and a black one alike, which is the one thing the page's own
 * palette is not.
 *
 * Keyed by the engine id, as `emblems.ts` and `sprites.ts` are, and
 * `test/ui/emoji.test.ts` holds the tables to the engine's and to the files.
 */

import type { ModId, PackId, ShopItem } from "../engine"

const SVG = "http://www.w3.org/2000/svg"

/**
 * Every file beside this module, by bare name, as the markup inside its
 * `<svg>`. Eager and raw: the pictures are in the bundle as strings, since a
 * shelf that fetched its pictures would deal five empty cards first.
 */
export const EMOJI_ART: Record<string, string> = Object.fromEntries(
  Object.entries(
    import.meta.glob<string>("./emoji/*.svg", { eager: true, query: "?raw", import: "default" }),
  ).map(([path, file]) => [
    path.replace(/^.*\/|\.svg$/g, ""),
    file.replace(/^\s*<svg[^>]*>|<\/svg>\s*$/g, ""),
  ]),
)

// A picture has to read on both Classic pages, and the set was drawn for a white
// one: four first picks did not survive the contact sheet. The pawn (#212121) and
// the paw prints (#321b41) were holes in the dark page, the two silhouettes barely
// more, and the white puff of dashing-away was a hole in the light one. A dog, a
// wave, a pair of masks and a leaf on the wind stand in. Check both tones before
// adding one; near-black and near-white are the two fills to distrust.
export const RELIC_EMOJI: Record<string, string> = {
  green_thumb: "potted-plant",
  scavenger: "magnifying-glass-tilted-left",
  vowel_hoarder: "jar",
  slow_burn: "firecracker",
  consonant_cluster: "grapes",
  cold_open: "snowflake",
  bloodhound: "dog-face",
  head_start: "triangular-flag",
  loaded_dice: "game-die",
  fresh_ink: "droplet",
  first_draft: "memo",
  candle: "candle",
  reserve: "battery",
  collector: "butterfly",
  // No binoculars in the set; a telescope is the same second look, and the
  // emblem and the sprite were redrawn as one to agree.
  second_look: "telescope",
  stipend: "dollar-banknote",
  anagrammer: "shuffle-tracks-button",
  // No arch in the set, and a brick said wall, not arch. Ours: five stones, the
  // middle one green and standing proud, which is the relic's whole condition.
  keystone: "5w-keystone",
  lexicographer: "feather",
  sunk_cost: "money-with-wings",
  speedrunner: "stopwatch",
  qs_bargain: "label",
  greedy_grammarian: "glasses",
  doppelganger: "performing-arts",
  hot_streak: "comet",
  hoarder: "money-bag",
  masochist: "cactus",
  chorus: "bell",
  alphabetist: "ladder",
  // No safe; the lock on its door.
  vault: "locked",
  mint: "coin",
  scorched_earth: "volcano",
  // Ours. The set's nearest is a snowman, which is what a snowball becomes and
  // not what this relic does: it rolls and it grows.
  snowball: "5w-snowball",
  thesaurus: "books",
  patron: "red-heart",
  indelible: "black-nib",
  second_wind: "leaf-fluttering-in-wind",
  long_game: "hourglass-not-done",
  // No match; what it is struck for.
  pyromaniac: "fire",
  carbon_copy: "bookmark-tabs",
  first_impression: "waving-hand",
  // Two of the one fruit on the one stem.
  twins: "cherries",
  blank_page: "page-facing-up",
  habit: "infinity",
  no_maybes: "bullseye",
  compound: "chart-increasing",
  royalties: "crown",
}

export const CONSUMABLE_EMOJI: Record<string, string> = {
  oracle: "crystal-ball",
  // His lantern, near enough: the set's only one is paper and red.
  hermit: "red-paper-lantern",
  magician: "magic-wand",
  // The bindle is nobody's emoji; the motley is.
  fool: "clown-face",
}

export const PACK_EMOJI: Record<PackId, string> = {
  alphabet: "input-latin-letters",
  relic: "gem-stone",
  // Categories, filed under tabs. The set has no ball, wedge and blocks.
  category: "card-index-dividers",
}

export const ETCHING_EMOJI: Record<string, string> = {
  etch_vowels: "mouth",
  etch_staples: "bread",
  // J Q X Z are the heavy ones, and the set has no weight but this.
  etch_heavy: "rock",
  etch_consonants: "scroll",
}

export const MOD_EMOJI: Record<ModId, string> = {
  // Ours: there is no poker chip, and a blue circle is a status light.
  chip: "5w-chip",
  // The times sign, since the set's plus is a button to add something.
  mult: "multiply",
  gold: "heavy-dollar-sign",
  // The one card that is not one subject in every look, on request: the line and
  // the pixels draw a bare star, which a joker's face is too fine for at their
  // sizes, and the emoji keeps the joker.
  wild: "joker",
  // The clover the emblem could not draw at 28px; an emoji can.
  lucky: "four-leaf-clover",
  echo: "speaker-high-volume",
  anchor: "anchor",
  // No girder; what holds one up.
  steel: "nut-and-bolt",
  // Ours. The set's glasses hold wine, milk, a cocktail or whisky on ice, and
  // this one holds nothing but water.
  glass: "5w-glass",
}

export const CATEGORY_EMOJI: Record<string, string> = {
  alphabetical: "right-arrow",
  vowel_heavy: "speech-balloon",
  cluster: "chains",
  // The Twins, by their sign.
  twinned: "gemini",
  // Ours: the set predates the fingerprint emoji.
  distinct: "5w-fingerprint",
}

// Ours, all four: the alphabet in quarters, the covered one standing tall, as
// the emblem and the sprite have it. No emoji is a quarter of an alphabet.
export const RANGE_EMOJI: Record<string, string> = {
  range_ae: "5w-range-ae",
  range_fm: "5w-range-fm",
  range_nr: "5w-range-nr",
  range_sz: "5w-range-sz",
}

/** The emoji by kind, which is every kind `emblems.ts` draws. */
export const EMOJI: Record<ShopItem["kind"], Record<string, string>> = {
  relic: RELIC_EMOJI,
  consumable: CONSUMABLE_EMOJI,
  pack: PACK_EMOJI,
  etch: ETCHING_EMOJI,
  mod: MOD_EMOJI,
  level: CATEGORY_EMOJI,
  range: RANGE_EMOJI,
}

/**
 * The emoji, added to the emblem it is an alternative to rather than set
 * beside it as the sprite is. The sprite needs a box of its own, because a
 * 12-unit grid wants a whole number of screen pixels to each of its own and the
 * emblem's sizes do not give it one. This scales to anything, so it goes inside
 * the emblem's `<svg>` as a `<g>` and inherits every size, window and seat the
 * layout files already give `.emblem`; the stylesheet shows the lines or the
 * group (`.emoji` in `base.css`, and Classic at the foot of
 * `skins/classic.css`). An id with no emoji is left as its lines, in every look.
 *
 * The scale is the two boxes: the set draws in 32 units and the emblems in 24.
 * `innerHTML` on an SVG element parses as SVG, and what it parses is a file in
 * this repository, never anything a player or a server wrote.
 */
export function emojied(svg: SVGSVGElement, kind: ShopItem["kind"], id: string): SVGSVGElement {
  const art = EMOJI_ART[EMOJI[kind][id] ?? ""]
  if (!art) return svg
  const group = document.createElementNS(SVG, "g")
  group.setAttribute("class", "emoji")
  group.setAttribute("transform", "scale(0.75)")
  group.innerHTML = art
  svg.classList.add("has-emoji")
  svg.append(group)
  return svg
}

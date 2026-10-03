/**
 * A drawing for every card the shelf sells: each relic, consumable, pack,
 * etching, letter modifier, word shape and alphabet range.
 *
 * The shelf drew one picture per *kind* (see `KIND_ICON` in `views.ts`), so
 * forty-odd relics all wore the same gem and the only thing telling Snowball
 * from Sunk Cost was a word. The kind drawing stays, at the card's foot, since
 * it answers a different question: what sort of thing this is, not which one.
 *
 * Kept apart from `icons.ts` because these are content and those are chrome.
 * The chrome's list is short and closed; this one grows every time a relic is
 * added, and folding it into `IconName` would have made the button icons a
 * union of fifty names. The drawing itself is built by the same `stroked`, on
 * the same terms: a 24-unit box, a 2-unit round stroke, `currentColor`, silent.
 *
 * Keyed by the engine table's `id`, which is also the key the language catalogs
 * use, so the engine never learns a picture exists. `test/ui/emblems.test.ts`
 * holds the table to the engine's, so a relic added there fails until it is
 * drawn.
 *
 * The rules each one was drawn to: the thing itself rather than a letter of its
 * name, since the name is printed right under it; a silhouette that still reads
 * at 28px, which rules out anything finer than about two units between strokes;
 * and no two that could be mistaken on a shelf of five. Where the name is not
 * a thing (Sunk Cost, No Maybes) the picture is a pun on it or on what the card
 * does, and says which in a comment.
 */

import type { ModId, PackId, ShopItem } from "../engine"
import { stroked } from "./icons"

type Paths = readonly string[]

/** A circle of radius `r` about `cx, cy`, as the two arcs a path needs for one. */
const ring = (cx: number, cy: number, r: number): string =>
  `M${cx} ${cy - r}a${r} ${r} 0 1 0 0 ${2 * r}a${r} ${r} 0 1 0 0-${2 * r}`

/** A dot: a zero-length stroke, which the round cap draws as a disc. */
const dot = (x: number, y: number): string => `M${x} ${y}h.01`

export const RELIC_EMBLEMS: Record<string, Paths> = {
  green_thumb: [
    "M12 21v-9",
    "M12 12C12 8 9 6 5 6c0 4 3 6 7 6z",
    "M12 15c0-4 3-6 7-6 0 4-3 6-7 6z",
    "M7 21h10",
  ],
  // Finding money in the yellows: a glass held over a coin.
  scavenger: [ring(10, 10, 6), "M14.5 14.5 20 20", ring(10, 10, 2)],
  vowel_hoarder: [
    "M9 7 7.5 3.5h9L15 7",
    "M9 7c-4 3-5 6-5 9a5 5 0 0 0 5 5h6a5 5 0 0 0 5-5c0-3-1-6-5-9z",
    "M9.5 18 12 11.5l2.5 6.5",
    "M10.3 16h3.4",
  ],
  // A lit fuse rather than a flame, which Pyromaniac and Candle both have.
  slow_burn: [
    "M2 21c4 0 5-5 9-5s4-4 4-7",
    ring(15, 7, 2),
    "M15 2v1",
    "M20 7h-1",
    "M18.5 3.5l-.7.7",
    "M11.5 3.5l.7.7",
  ],
  consonant_cluster: [
    ring(8.5, 11, 3),
    ring(15.5, 11, 3),
    ring(12, 17, 3),
    "M12 8V3",
    "M12 5c2-2 4-2 5-1",
  ],
  // The first guess of a round is the cold one. Every arm carries the barb: only
  // the upright pair did, which read as an arrow pointing both ways with a cross
  // through it rather than as a crystal.
  cold_open: [
    "M12 3v18",
    "M4.2 7.5l15.6 9",
    "M4.2 16.5l15.6-9",
    "M9.5 4.5 12 6 14.5 4.5",
    "M17.2 6.1 17.2 9 19.7 10.4",
    "M19.7 13.6 17.2 15 17.2 17.9",
    "M14.5 19.5 12 18 9.5 19.5",
    "M6.8 17.9 6.8 15 4.3 13.6",
    "M4.3 10.4 6.8 9 6.8 6.1",
  ],
  // The hound itself, ears down: it follows the yellows. A paw print was here
  // first, and is any animal's.
  bloodhound: [
    "M7 6h10a2 2 0 0 1 2 2v7a7 7 0 0 1-14 0V8a2 2 0 0 1 2-2z",
    "M7 6C4 6 3 8 3 11l2 2",
    "M17 6c3 0 4 2 4 5l-2 2",
    dot(9.5, 11),
    dot(14.5, 11),
    "M11 15h2",
    "M12 15v3",
  ],
  head_start: ["M5 21V3", "M5 4h13l-3 4 3 4H5"],
  loaded_dice: [
    "M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z",
    dot(8, 8),
    dot(12, 12),
    dot(16, 16),
  ],
  fresh_ink: [
    "M12 3c-3 4.5-6 7.5-6 11a6 6 0 0 0 12 0c0-3.5-3-6.5-6-11z",
    "M9.5 14.5a2.5 2.5 0 0 0 2 2.5",
  ],
  // A page written on, where Blank Page's has nothing on it.
  first_draft: ["M6 3h9l4 4v14H6z", "M15 3v4h4", "M9 11h7", "M9 14h7", "M9 17h4"],
  candle: [
    "M9 10h6v11H9z",
    "M12 3c-1.2 1.6-2 2.7-2 3.8a2 2 0 0 0 4 0c0-1.1-.8-2.2-2-3.8z",
    "M15 13h-1.5v2.5",
  ],
  // Guesses held back, as charge held back.
  reserve: ["M3 8h15v8H3z", "M21 11v2", "M6 11v2", "M9.5 11v2"],
  // Pinned and kept: what a collector does to a butterfly.
  collector: [
    "M12 7v12",
    "M12 10C9 4 3 4 3 8s4 5 9 3",
    "M12 10c3-6 9-6 9-2s-4 5-9 3",
    "M12 13c-3 0-6 2-5 5s4 1 5-3",
    "M12 13c3 0 6 2 5 5s-4 1-5-3",
  ],
  // A telescope on its legs: the second look, taken from further off.
  second_look: ["M3 13l12-6 2 4-12 6z", "M17 6l3-1.5 2 4-3 1.5", "M11 14l-3 7", "M11 14l4 7"],
  // A banknote: the pay, counted out.
  stipend: ["M2 6h20v12H2z", ring(12, 12, 3), dot(5.5, 12), dot(18.5, 12)],
  anagrammer: [
    "M3 7h3c5 0 7 10 12 10h3",
    "M3 17h3c5 0 7-10 12-10h3",
    "M18 4l3 3-3 3",
    "M18 14l3 3-3 3",
  ],
  keystone: ["M3 21v-9a9 9 0 0 1 18 0v9", "M8 21v-9a4 4 0 0 1 8 0v9", "M10 3.2l.8 4.8h2.4l.8-4.8"],
  lexicographer: ["M20 3C12 3 7 8 5 19", "M20 3c0 7-5 11-11 12", "M4 21l1.5-2", "M10 11h5"],
  // What a sunk cost has done: money on the wing.
  sunk_cost: [
    "M7 9h10v8H7z",
    ring(12, 13, 1.5),
    "M7 11C5 8 3 7 1.5 7c0 3 2 5 5.5 6",
    "M17 11c2-3 4-4 5.5-4 0 3-2 5-5.5 6",
  ],
  speedrunner: [ring(12, 13, 8), "M12 13l3-3", "M10 2h4", "M12 2v3", "M19 5l1.5 1.5"],
  qs_bargain: ["M3 3h8l10 10-8 8L3 11z", dot(7.5, 7.5)],
  greedy_grammarian: [
    ring(6.5, 14, 3.5),
    ring(17.5, 14, 3.5),
    "M10 13c1-1 3-1 4 0",
    "M3 14 2 9",
    "M21 14l1-5",
  ],
  // Two masks, one face behind the other, and not the same face.
  doppelganger: [
    "M3 4h9v6a4.5 4.5 0 0 1-9 0z",
    "M5.5 10c1 1 3 1 4 0",
    dot(6, 7),
    dot(9, 7),
    "M15 10h6v6a4.5 4.5 0 0 1-9 0v-.5",
    "M14.5 17.5c1-1 3-1 4 0",
    dot(15.5, 13.5),
    dot(18.5, 13.5),
  ],
  hot_streak: [ring(7, 17, 3), "M9.5 14.5 20 4", "M10 17.5 16 11.5", "M6.5 14 12 8.5"],
  // A sack tied shut: what is held is not spent.
  hoarder: [
    "M9 8 7.5 4c2 1 3-1 4.5 0s2.5 1 4.5 0L15 8",
    "M9 8h6",
    "M9 8c-4 3-5 6-5 9a4 4 0 0 0 4 4h8a4 4 0 0 0 4-4c0-3-1-6-5-9",
    "M14 13.2c-.4-.7-1.1-1.2-2-1.2-1.1 0-2 .6-2 1.5 0 1.9 4 1 4 3 0 .9-.9 1.5-2 1.5-.9 0-1.6-.5-2-1.2",
  ],
  // Prickly, and paid for every gray it lands on.
  masochist: [
    "M10 21V5a2 2 0 0 1 4 0v16",
    "M10 14H8a2 2 0 0 1-2-2V8",
    "M14 12h2a2 2 0 0 0 2-2V6",
    "M5 21h14",
  ],
  chorus: [
    "M6 17v-6a6 6 0 0 1 12 0v6l2 2H4z",
    "M10 21a2 2 0 0 0 4 0",
    "M3 8a9 9 0 0 1 2-4",
    "M21 8a9 9 0 0 0-2-4",
  ],
  // Letters in order, climbing: a ladder, a rung at a time.
  alphabetist: ["M8 3v18", "M16 3v18", "M8 7h8", "M8 12h8", "M8 17h8"],
  // Locked: the vault is what has been guessed so far, and it stays put.
  vault: ["M5 11h14v10H5z", "M8 11V7a4 4 0 0 1 8 0v4", "M12 15v2"],
  mint: [
    "M4 6a8 3 0 1 0 16 0a8 3 0 1 0-16 0",
    "M4 6v5a8 3 0 0 0 16 0V6",
    "M4 11v5a8 3 0 0 0 16 0v-5",
  ],
  // A volcano, lava over the rim: the ground left burnt.
  scorched_earth: [
    "M2 21h20",
    "M4 21l5-11h6l5 11",
    "M9 10l1.5 2.5 1.5-2 1.5 2L15 10",
    "M10 7c0-2 2-2 2-4",
    "M14 7c0-2 2-2 2-3",
  ],
  // A snowball rolling, speed lines behind it. It was a snowman for one
  // afternoon, after the set's nearest emoji; the emoji was redrawn instead.
  snowball: [ring(14, 12, 7), "M10.5 9a4.5 4.5 0 0 1 3.5-2", "M2 9h3", "M1.5 12.5H5", "M2 16h3"],
  thesaurus: ["M5 4a1 1 0 0 1 1-1h13v18H6a1 1 0 0 1-1-1z", "M9 3v18", "M13 3v5l2-1.5L17 8V3"],
  // One for each uncommon relic, and the heart is what a patron gives.
  patron: ["M12 20s-8-5-8-11a4.5 4.5 0 0 1 8-2.8A4.5 4.5 0 0 1 20 9c0 6-8 11-8 11z"],
  // The pen nib, whose ink is the one that does not fade.
  indelible: ["M12 3l6 7-6 11-6-11z", "M12 21v-8", ring(12, 11.5, 1.5)],
  // A leaf the wind has got hold of.
  second_wind: [
    "M21 3c-7 0-11 4-11 10 6 0 11-4 11-10z",
    "M10 13l6-6",
    "M2 12h5",
    "M3 16h9",
    "M2 20h6",
  ],
  long_game: [
    "M6 3h12",
    "M6 21h12",
    "M7 3v2c0 3 5 5 5 7s-5 4-5 7v2",
    "M17 3v2c0 3-5 5-5 7s5 4 5 7v2",
  ],
  // A fire, well alight: the match that was here is Slow Burn's cousin, not this.
  pyromaniac: [
    "M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.4-.5-2-1-3-1-2.1-.2-4 2-6 .5 2.5 2 4.9 4 6.5s3 3.5 3 5.5a7 7 0 0 1-14 0c0-1.2.4-2.3 1-3a2.5 2.5 0 0 0 2.5 2.5z",
  ],
  carbon_copy: ["M8 8h12v12H8z", "M16 8V4H4v12h4"],
  // A hand raised in greeting: the first impression is the first tile.
  first_impression: [
    "M7 14V7a1.5 1.5 0 0 1 3 0v4",
    "M10 11V5a1.5 1.5 0 0 1 3 0v6",
    "M13 11V6a1.5 1.5 0 0 1 3 0v5",
    "M16 11V9a1.5 1.5 0 0 1 3 0v6a6 6 0 0 1-6 6h-1c-2 0-3.5-1-4.5-2.5L3.5 14a1.5 1.5 0 0 1 2.5-1.5l1 1.5",
    "M20.5 3.5c1 .8 1.5 1.8 1.5 3",
  ],
  // A pair of cherries on one stem: the letter that shows up twice.
  twins: [ring(7, 17, 4), ring(17, 17, 4), "M7 13c1-5 4-8 9-10", "M17 13c0-4 0-7-1-10"],
  // A page with nothing on it yet, where First Draft's is written.
  blank_page: ["M6 3h9l4 4v14H6z", "M15 3v4h4"],
  habit: ["M12 12c-2-3-4-4-6-4a4 4 0 0 0 0 8c2 0 4-1 6-4s4-4 6-4a4 4 0 0 1 0 8c-2 0-4-1-6-4z"],
  // No yellows is no maybes: dead centre or nothing.
  no_maybes: [ring(12, 12, 9), ring(12, 12, 5), dot(12, 12)],
  compound: ["M3 20h18", "M4 16l5-5 4 3 7-8", "M15 6h5v5"],
  royalties: ["M3 8l4 4 5-7 5 7 4-4-2 11H5z"],
}

export const CONSUMABLE_EMBLEMS: Record<string, Paths> = {
  // Her crystal ball on its stand, something showing in it.
  oracle: [ring(12, 10, 7), "M7 21l1.5-4h7L17 21z", "M12 7v5", "M9.5 9.5h5"],
  // His lantern.
  hermit: [
    "M12 2v2.5",
    "M7 8l5-3.5L17 8z",
    "M8 8h8v10H8z",
    "M7 18h10v3H7z",
    "M12 10.5c-1 1.3-1.5 2-1.5 2.8a1.5 1.5 0 0 0 3 0c0-.8-.5-1.5-1.5-2.8z",
  ],
  magician: ["M4 20 15 9", "M17 3v4", "M15 5h4", "M20 10v2", "M19 11h2"],
  // The Fool as the pack draws him: a clown's face, hair out at both sides and
  // a point of a hat. Two tufts and a nose keep it off Royalties' crown.
  fool: [
    ring(12, 14, 7),
    ring(4.5, 9, 2.5),
    ring(19.5, 9, 2.5),
    "M10 7.3 12 2l2 5.3",
    dot(9.5, 12),
    dot(14.5, 12),
    ring(12, 14.5, 1),
    "M8.5 17c2 2.5 5 2.5 7 0",
  ],
}

export const PACK_EMBLEMS: Record<PackId, Paths> = {
  // A fan of tiles, the front one lettered: the only emblem that is allowed a
  // letter, since a letter is the thing this pack deals.
  alphabet: ["M9 3h11v14h-5", "M4 7h11v14H4z", "M7 18l2.5-7 2.5 7", "M7.8 16h3.4"],
  relic: ["M6 9h12l4 5-10 8L2 14z", "M2 14h20", "M12 2v3", "M5 4l1.5 2", "M19 4l-1.5 2"],
  // Categories, filed: a box of tabbed dividers.
  category: ["M3 21V9h6l1.5-3h5L17 9h4v12z", "M3 13h18", "M3 17h18"],
}

/**
 * The etchings by which letters they cut, not by the word "etch", which all
 * four share and the foot's ticket already draws.
 */
export const ETCHING_EMBLEMS: Record<string, Paths> = {
  // An open mouth, the shape a vowel is sung in.
  etch_vowels: ["M3 12c3-5 15-5 18 0-3 6-15 6-18 0z", "M7 12c3 2 7 2 10 0"],
  // L N S T R, the letters every word is made of: the staple in the other
  // sense, a loaf.
  etch_staples: [
    "M5 20v-8.5A4 4 0 0 1 7 4h10a4 4 0 0 1 2 7.5V20z",
    "M9 8l-1 2",
    "M13 8l-1 2",
    "M17 8l-1 2",
  ],
  // J Q X Z are the heavy ones: a rock.
  etch_heavy: ["M3 19l2-8 5-5 6 1 5 7-1 5z", "M10 6l2 6 9 2", "M12 12l-7-1"],
  // A scroll: the long roll of everything that is not a vowel.
  etch_consonants: [
    "M4 3a2 2 0 0 0-2 2v2h4V5a2 2 0 0 0-2-2z",
    "M4 3h13a2 2 0 0 1 2 2v12",
    "M6 7v12a2 2 0 0 0 2 2 2 2 0 0 0 2-2v-2h12v2a2 2 0 0 1-2 2H8",
    "M10 8h5",
    "M10 12h5",
  ],
}

/**
 * The letter modifiers by what the letter scores, since that is all a modifier
 * is. Chip and Gold are both round things, so they are told apart by what is
 * on the face: a poker chip's rim marks against a dollar. Keyed by `ModId`, so
 * a modifier added to the engine is a compile error here before it is a test
 * failure.
 */
export const MOD_EMBLEMS: Record<ModId, Paths> = {
  chip: [
    ring(12, 12, 9),
    ring(12, 12, 4.5),
    "M12 3v3",
    "M12 18v3",
    "M3 12h3",
    "M18 12h3",
    "M5.6 5.6l2.1 2.1",
    "M16.3 16.3l2.1 2.1",
    "M18.4 5.6l-2.1 2.1",
    "M7.7 16.3l-2.1 2.1",
  ],
  // The sign mult is written with everywhere else on the card.
  mult: ["M5 5l14 14", "M19 5 5 19"],
  // A dollar, with no coin round it: Mint and Stipend have the coin and the note.
  gold: [
    "M17 7c-.8-1.8-2.6-3-5-3-3 0-5 1.5-5 3.8 0 5 10 2.8 10 8 0 2.3-2 3.7-5 3.7-2.4 0-4.2-1.2-5-3",
    "M12 2v2",
    "M12 19.5V22",
  ],
  // The wild card, by the star that marks one. It was the star on a card, and
  // at 28px the card was most of the drawing and the star a blot in it. Classic
  // keeps the joker, which an emoji has the colours to draw.
  wild: ["M12 2l3.1 6.3 6.9 1-5 4.8 1.2 6.9-6.2-3.2L5.8 21 7 14.1 2 9.3l6.9-1z"],
  // A four-leaf clover. Four rings read as Consonant Cluster's grapes at 28px,
  // which is why this was a horseshoe; four hearts on a stem do not.
  lucky: [
    "M12 11C9 9 8 7 8 5.5a2 2 0 0 1 4-.5 2 2 0 0 1 4 .5c0 1.5-1 3.5-4 5.5z",
    "M12 11C10 8 8 7 6.5 7a2 2 0 0 0-.5 4 2 2 0 0 0 .5 4c1.5 0 3.5-1 5.5-4z",
    "M12 11c2-3 4-4 5.5-4a2 2 0 0 1 .5 4 2 2 0 0 1-.5 4c-1.5 0-3.5-1-5.5-4z",
    "M12 11c-3 2-4 4-4 5.5a2 2 0 0 0 4 .5 2 2 0 0 0 4-.5c0-1.5-1-3.5-4-5.5z",
    "M15.5 16.5c1 2 2.5 3.5 4.5 4.5",
  ],
  // Scores when the word repeats it: a sound and what comes back of it.
  echo: [
    dot(4, 12),
    "M8.5 8.5a5 5 0 0 1 0 7",
    "M12 5.5a9.5 9.5 0 0 1 0 13",
    "M15.5 2.5a13.5 13.5 0 0 1 0 19",
  ],
  // Pays when the letter lands green and stays put.
  anchor: [
    ring(12, 5, 2),
    "M12 7v14",
    "M8 10.5h8",
    "M4 14a8 8 0 0 0 16 0",
    "M2 15.5 4 14l1.5 2",
    "M22 15.5 20 14l-1.5 2",
  ],
  // A steel nut, seen down the thread.
  steel: ["M12 3l7.8 4.5v9L12 21l-7.8-4.5v-9z", ring(12, 12, 3.5)],
  // A glass of water, already cracked, since cracking is what this one does.
  // A tumbler and not a stemmed glass: the stem said wine, which nothing here is.
  glass: ["M6 3h12l-1.5 18h-9z", "M7 14.5h10", "M10.5 3.5l2 2.5-2 1.5 1.5 2.5"],
}

/**
 * The word shapes by the rule each one tests, drawn as the rule rather than as
 * a word that keeps it. Twins and Consonant Cluster are relics too, and their
 * pictures (two rings, a bunch of grapes) were drawn first, so the shapes of
 * the same idea are drawn from another angle: a pair of tiles, a chain.
 */
export const CATEGORY_EMBLEMS: Record<string, Paths> = {
  // Letters in A–Z order, never going backwards: a one-way sign.
  alphabetical: ["M2 8h20v8H2z", "M5 12h11", "M13.5 9.5 17 12l-3.5 2.5"],
  // A mouthful of vowels, said: the speech bubble, three sounds in it.
  vowel_heavy: ["M4 4h16v12h-9l-5 4v-4H4z", dot(8, 10), dot(12, 10), dot(16, 10)],
  // Three consonants in a row, linked.
  cluster: [
    "M5.5 8.5h2a3.5 3.5 0 0 1 0 7h-2a3.5 3.5 0 0 1 0-7z",
    "M11 8.5h2a3.5 3.5 0 0 1 0 7h-2a3.5 3.5 0 0 1 0-7z",
    "M16.5 8.5h2a3.5 3.5 0 0 1 0 7h-2a3.5 3.5 0 0 1 0-7z",
  ],
  // Some letter twice: the sign of the twins.
  twinned: ["M5 4c4 2 10 2 14 0", "M5 20c4-2 10-2 14 0", "M9 5.3v13.4", "M15 5.3v13.4"],
  // No letter repeats: the one thing nobody shares, a fingerprint.
  distinct: [
    "M5.5 18c-1-2-1.5-4-1.5-6a8 8 0 0 1 16 0c0 2-.3 3.5-.8 5",
    "M9 20c-1-2.5-1.5-5-1.5-8a4.5 4.5 0 0 1 9 0c0 2.5-.5 5-1.5 7.5",
    "M12 12c0 3.5-.5 6-1.5 9",
  ],
}

/**
 * The alphabet in quarters, and which quarter this range levels: four bars,
 * the one it covers standing. Positional rather than lettered, since the
 * title prints the letters it covers right beside the picture, and a picture
 * of "A–E" would be the title again at a size nobody can read.
 */
const quarter = (n: number): Paths =>
  [4, 9.33, 14.67, 20].map((x, i) => (i === n ? `M${x} 3v18` : `M${x} 14v7`))

export const RANGE_EMBLEMS: Record<string, Paths> = {
  range_ae: quarter(0),
  range_fm: quarter(1),
  range_nr: quarter(2),
  range_sz: quarter(3),
}

/**
 * The drawing a card wears, or null for an id the tables have no drawing for,
 * which the caller draws with its kind's picture instead.
 */
export function emblem(kind: ShopItem["kind"], id: string): SVGSVGElement | null {
  const table =
    kind === "relic"
      ? RELIC_EMBLEMS
      : kind === "consumable"
        ? CONSUMABLE_EMBLEMS
        : kind === "pack"
          ? (PACK_EMBLEMS as Record<string, Paths>)
          : kind === "etch"
            ? ETCHING_EMBLEMS
            : kind === "mod"
              ? (MOD_EMBLEMS as Record<string, Paths>)
              : kind === "level"
                ? CATEGORY_EMBLEMS
                : RANGE_EMBLEMS
  const paths = table?.[id]
  return paths ? stroked(paths, `icon emblem emblem-${id}`) : null
}

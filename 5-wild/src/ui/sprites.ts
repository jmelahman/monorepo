/**
 * Pixel sprites for every card the shelf sells: the same seventy-six things
 * `emblems.ts` draws, again, as 12x12 grids of colour.
 *
 * They exist beside the emblems rather than instead of them. The Smoke Room skin
 * is a near-black room with one mint lamp in it, and a 2-unit line drawn in
 * `currentColor` reads there as a diagram, not as an object on a table. A sprite
 * carries its own colour and its own light, so a relic looks like a thing you
 * could pick up. Tabletop draws the emblems, which are line art on purpose and
 * suit it (it sets them in an engraved bone tile), and Classic the emoji in
 * `emoji.ts`. The three are one subject in three hands: a card is the same
 * thing in every look, so a subject changed in one is redrawn in the others. The
 * view puts all of them in every card and the stylesheet shows one, so a window crossing
 * the table breakpoint changes the look with no render; see `cardArt` in
 * `views.ts`. The two tables are keyed alike so a card can be given either.
 *
 * The format is the one a pixel artist would use, so a sprite is edited as a
 * picture. Twelve rows of twelve characters: `.` is nothing, `o` is the outline
 * (#06070a, the page's own black, so a sprite sits in the room and not on it),
 * and every other character is a key into that sprite's own `palette`. Light
 * comes from the top left everywhere, so a highlight is always up and left of its
 * shade, and a shelf of five never lights from five directions. At most five
 * colours besides the outline per sprite: past that the eye stops reading a
 * shape and starts reading a palette.
 *
 * Every colour comes from the one base palette `C`, about twenty entries chosen
 * against the mint accent, so no sprite invents a hue and a warm relic beside a
 * cool one is the palette's doing rather than two artists'. The greens and reds
 * are the tile colours' cousins but not the tiles' own: a letter's colour means
 * something, and a leaf must not be mistaken for a hit.
 *
 * The drawing is two functions so the test needs no DOM. `spritePaths` is pure
 * and returns one `{ fill, d }` per colour, horizontal runs merged into a single
 * `h`; `spriteSvg` only wraps those in an element, as `stroked` does for the
 * emblems. `shape-rendering: crispEdges` is what keeps a 12-unit box from
 * blurring at 60px, where each pixel is five screen pixels and a smoothed edge
 * is a smudge.
 *
 * Keyed by the engine table's `id`; `test/ui/sprites.test.ts` holds each table
 * to the engine's, so a relic added there fails until it is drawn.
 */

import type { ShopItem } from "../engine"

const SVG = "http://www.w3.org/2000/svg"

/** The outline, and the only colour not spelled out in a palette. */
export const OUTLINE = "#06070a"

/** One base palette: every sprite's own palette picks from these and invents nothing. */
export const C = {
  leafHi: "#9be08a",
  leaf: "#5fbf5a",
  clay: "#b8583a",
  clayHi: "#d8805a",
  steelHi: "#eef2f7",
  steel: "#b8c0cc",
  iron: "#8a8f9a",
  slate: "#6e7686",
  cyan: "#7fd8e8",
  gold: "#ffd166",
  goldDeep: "#c98a2b",
  wood: "#b98a5a",
  woodHi: "#c9a27a",
  amber: "#ffb347",
  coral: "#ff5a64",
  red: "#d8434f",
  cream: "#fff4d0",
  violet: "#8a6fd1",
  violetHi: "#b7a2f0",
  mint: "#58c9b3",
  mintHi: "#9be8d6",
} as const

export type Sprite = {
  /** One character per colour used in `rows`, the value a hex from `C`. */
  readonly palette: Readonly<Record<string, string>>
  /** Twelve rows of twelve characters; `.` empty, `o` outline. */
  readonly rows: readonly string[]
}

export type SpritePath = { readonly fill: string; readonly d: string }

/**
 * The sprite as filled paths: the outline first so a colour can never be
 * painted over it, then each palette colour in the palette's order. Runs of the
 * same character along a row are one rectangle, `M{x} {y}h{n}v1h-{n}z`.
 */
export function spritePaths(sprite: Sprite): SpritePath[] {
  const fills: [string, string][] = [["o", OUTLINE], ...Object.entries(sprite.palette)]
  const out: SpritePath[] = []
  for (const [key, fill] of fills) {
    let d = ""
    sprite.rows.forEach((row, y) => {
      for (let x = 0; x < row.length; ) {
        if (row[x] !== key) {
          x++
          continue
        }
        let n = 1
        while (row[x + n] === key) n++
        d += `M${x} ${y}h${n}v1h-${n}z`
        x += n
      }
    })
    if (d) out.push({ fill, d })
  }
  return out
}

/** The sprite as an element, in the emblems' box-less style: silent, and sized by CSS. */
export function spriteSvg(sprite: Sprite, className: string): SVGSVGElement {
  const svg = document.createElementNS(SVG, "svg")
  svg.setAttribute("class", className)
  svg.setAttribute("viewBox", "0 0 12 12")
  svg.setAttribute("shape-rendering", "crispEdges")
  svg.setAttribute("aria-hidden", "true")
  for (const { fill, d } of spritePaths(sprite)) {
    const path = document.createElementNS(SVG, "path")
    path.setAttribute("d", d)
    path.setAttribute("fill", fill)
    svg.append(path)
  }
  return svg
}

/** The relics, keyed by the engine id `emblems.ts` uses. */
export const RELIC_SPRITES: Record<string, Sprite> = {
  // A sprout in a clay pot: the green thumb, and the green it pays for.
  green_thumb: {
    palette: { L: C.leafHi, G: C.leaf, P: C.clay, p: C.clayHi },
    rows: [
      "..oo....oo..",
      ".oLLo..oLLo.",
      ".oLGGooGGLo.",
      "..oGGGGGGo..",
      "...ooGGoo...",
      ".....oGo....",
      "..oooooooo..",
      "..oPPPPPPo..",
      "..oPpPPPPo..",
      "...oPPPPo...",
      "...oPPPPo...",
      "....oooo....",
    ],
  },
  // A glass held over a coin: money found in the yellows.
  scavenger: {
    palette: { S: C.steel, w: C.cyan, g: C.gold, G: C.goldDeep, W: C.wood },
    rows: [
      "...oooo.....",
      "..oSSSSo....",
      ".oSwwwwSo...",
      "oSwwwwwwSo..",
      "oSwwggwwSo..",
      "oSwwGGwwSo..",
      "oSwwwwwwSo..",
      ".oSwwwwSoo..",
      "..oSSSSoWWo.",
      "...oooooWWo.",
      "........oWWo",
      ".........oo.",
    ],
  },
  // A corked jar with an A in it: the vowels, kept.
  vowel_hoarder: {
    palette: { W: C.wood, B: C.cyan, b: C.steelHi },
    rows: [
      "...oooooo...",
      "..oWWWWWWo..",
      "..oWWWWWWo..",
      "..oBBBBBBo..",
      ".oBbBBBBBBo.",
      ".oBbBooBBBo.",
      ".oBBoBBoBBo.",
      ".oBBooooBBo.",
      ".oBBoBBoBBo.",
      ".oBBBBBBBBo.",
      "..oBBBBBBo..",
      "...oooooo...",
    ],
  },
  // A lit fuse, sparking at the top: a slow burn is not a flame.
  slow_burn: {
    palette: { R: C.wood, F: C.cream, Y: C.amber, y: C.coral },
    rows: [
      ".......y.y..",
      "........Y...",
      "......yYFYy.",
      "........Y...",
      ".......oy...",
      "......oRo...",
      ".....oRo....",
      "....oRo.....",
      "...oRo......",
      "..oRo.......",
      ".oRo........",
      ".oo.........",
    ],
  },
  // A bunch of grapes: consonants growing in a cluster.
  consonant_cluster: {
    palette: { K: C.wood, G: C.leaf, V: C.violet, v: C.violetHi },
    rows: [
      ".....o.oo...",
      "....oKoGGo..",
      "..oooKGGo...",
      ".ovVVovVVo..",
      ".oVVVoVVVo..",
      ".oVVVoVVVo..",
      "..ooooooo...",
      "...ovVVo....",
      "...oVVVo....",
      "...oVVVo....",
      "....ooo.....",
      "............",
    ],
  },
  // A snowflake: the first guess of a round is the cold one.
  cold_open: {
    palette: { I: C.cyan, H: C.steelHi },
    rows: [
      ".....o......",
      "..o.oIo.o...",
      ".oIooIooIo..",
      "..oIoIoIo...",
      ".oooIHIooo..",
      "oIIIIHIIIIo.",
      ".oooIHIooo..",
      "..oIoIoIo...",
      ".oIooIooIo..",
      "..o.oIo.o...",
      ".....o......",
      "............",
    ],
  },
  // The hound itself, ears down and tongue out: it follows the yellows.
  bloodhound: {
    palette: { W: C.wood, w: C.woodHi, D: C.clay, c: C.cream, R: C.coral },
    rows: [
      "..oooooooo..",
      ".owwwwwwwwo.",
      "oDowwwwwwoDo",
      "oDoWoWWoWoDo",
      "oDoWWWWWWoDo",
      "oDoWccccWoDo",
      ".ooWcoocWoo.",
      "..oWccccWo..",
      "..oWcRRcWo..",
      "...ooRRoo...",
      "....oooo....",
      "............",
    ],
  },
  // A pennant already planted at the line.
  head_start: {
    palette: { S: C.steel, R: C.coral, r: C.amber },
    rows: [
      ".oooooooooo.",
      "oSrrrRRRRRRo",
      "oSrRRRRRRRo.",
      "oSRRRRRRRo..",
      "oSRRRRRoo...",
      "oSRRRoo.....",
      "oSRoo.......",
      "oSo.........",
      "oSo.........",
      "oSo.........",
      "oSo.........",
      ".o..........",
    ],
  },
  // A die with its pips on the diagonal: the slope is the loading.
  loaded_dice: {
    palette: { r: C.amber, R: C.coral, D: C.red, c: C.cream },
    rows: [
      "..oooooooo..",
      ".orrrrrrrro.",
      "orrRRRRRRRRo",
      "orRRRRRRRRRo",
      "oRRccRRRRRRo",
      "oRRccRRRRRRo",
      "oRRRRccRRRRo",
      "oRRRRccRRRRo",
      "oRRRRRRccRRo",
      "oRRRRRRccRRo",
      ".oDDDDDDDDo.",
      "..oooooooo..",
    ],
  },
  // A drop of wet ink, glossy on its shoulder.
  fresh_ink: {
    palette: { V: C.violet, v: C.violetHi, S: C.slate },
    rows: [
      ".....oo.....",
      "....oVVo....",
      "...ovVVVo...",
      "...ovVVVo...",
      "..ovVVVVSo..",
      "..ovVVVVSo..",
      ".ovVVVVVVSo.",
      ".ovVVVVVVSo.",
      ".oVVVVVVVSo.",
      "..oVVVVVSo..",
      "...oVVVSo...",
      "....oooo....",
    ],
  },
  // A page written on, corner turned down, where Blank Page's has nothing on it.
  first_draft: {
    palette: { P: C.steelHi, C: C.steel, S: C.slate },
    rows: [
      "..ooooo.....",
      ".oPPPPPo....",
      ".oPPPPPCo...",
      ".oPPPPPCCo..",
      ".oPPPPPPPPo.",
      ".oPSSSSSPPo.",
      ".oPPPPPPPPo.",
      ".oPSSSSPPPo.",
      ".oPPPPPPPPo.",
      ".oPSSSSSSPo.",
      ".oPPPPPPPPo.",
      "..oooooooo..",
    ],
  },
  // A candle burning down, one drip already run.
  candle: {
    palette: { F: C.amber, Y: C.cream, K: C.slate, w: C.woodHi, H: C.steel },
    rows: [
      ".....oo.....",
      "....oFFo....",
      "...oFYYFo...",
      "...oFYYFo...",
      "....oFFo....",
      "...ooKKoo...",
      "..oYYYYYwo..",
      "..oYYYYYwYo.",
      "..oYYYYYwYo.",
      "..oYYYYYwo..",
      ".oHHHHHHHHo.",
      "..oooooooo..",
    ],
  },
  // A battery with three cells lit: guesses held back, as charge held back.
  reserve: {
    palette: { H: C.steel, S: C.slate, G: C.mint },
    rows: [
      "............",
      ".ooooooooo..",
      "oHHHHHHHHHo.",
      "oSSSSSSSSSo.",
      "oSGGSGGSGGo.",
      "oSGGSGGSGGHo",
      "oSGGSGGSGGHo",
      "oSSSSSSSSSo.",
      "oSSSSSSSSSo.",
      ".ooooooooo..",
      "............",
      "............",
    ],
  },
  // A pinned butterfly: what a collector does to one.
  collector: {
    palette: { A: C.amber, F: C.cream, P: C.violet, K: C.slate },
    rows: [
      "...o....o...",
      "..oKo..oKo..",
      "...oKooKo...",
      ".ooooKKoooo.",
      "oFFAAKKAAAAo",
      "oAAAAKKAAAAo",
      ".oAAAKKAAAo.",
      "..oPPKKPPo..",
      "..oPPKKPPo..",
      "...oPKKPo...",
      "....oKKo....",
      ".....oo.....",
    ],
  },
  // A telescope on its tripod: a second look, first one free.
  second_look: {
    palette: { S: C.iron, H: C.steelHi, w: C.cyan, W: C.wood },
    rows: [
      ".........oo.",
      ".......oowwo",
      ".....ooHHwwo",
      "...ooHHSSoo.",
      ".ooHHSSoo...",
      "oHHSSoo.....",
      "oSSooWo.....",
      ".oo.oWWo....",
      "...oWooWo...",
      "..oWo..oWo..",
      ".oWo....oWo.",
      ".oo......oo.",
    ],
  },
  // A banknote: the pay, counted out.
  stipend: {
    palette: { L: C.leafHi, G: C.leaf, c: C.cream },
    rows: [
      "............",
      "............",
      ".oooooooooo.",
      "oLLLLLLLLLLo",
      "oLGGGccGGGGo",
      "oLcGccccGcGo",
      "oLcGccccGcGo",
      "oLGGGccGGGGo",
      "oGGGGGGGGGGo",
      ".oooooooooo.",
      "............",
      "............",
    ],
  },
  // Two arrows crossing: the same letters, shuffled.
  anagrammer: {
    palette: { M: C.mint, C: C.cyan },
    rows: [
      ".oo....oooo.",
      "oCCo..oMMMMo",
      ".oCCo..ooMMo",
      "..oCCo.oMMMo",
      "...oCCoMMoMo",
      "....oCMMo.o.",
      "....oMMCo.o.",
      "...oMMoCCoCo",
      "..oMMo.oCCCo",
      ".oMMo..ooCCo",
      "oMMo..oCCCCo",
      ".oo....oooo.",
    ],
  },
  // An arch of thick stone with its keystone set green: the stone that holds the rest up.
  keystone: {
    palette: { H: C.steelHi, S: C.iron, I: C.slate, K: C.leaf },
    rows: [
      "...oooooo...",
      "..oHHKKSSo..",
      ".oHHSKKSIIo.",
      "oHHSSKKSSIIo",
      "oHHSooooSIIo",
      "oHSSo..oSIIo",
      "oHSo....oSIo",
      "oHSo....oSIo",
      "oHSo....oSIo",
      "oHSo....oSIo",
      "oHSo....oSIo",
      ".oo......oo.",
    ],
  },
  // A quill: the dictionary-maker's pen.
  lexicographer: {
    palette: { H: C.steelHi, C: C.steel, W: C.wood },
    rows: [
      ".......oooo.",
      "......oHHHHo",
      ".....oHHHWCo",
      "....oHHHWCCo",
      "...oHHHWCCo.",
      "..oHHHWCCo..",
      "..oHHWCCo...",
      "...oHWCo....",
      "..oWWoo.....",
      ".oWoo.......",
      "oWo.........",
      ".o..........",
    ],
  },
  // A banknote on the wing: what a sunk cost has done.
  sunk_cost: {
    palette: { H: C.steelHi, S: C.steel, L: C.leafHi, G: C.leaf, c: C.cream },
    rows: [
      "............",
      ".o........o.",
      "oHo......oHo",
      "oHHooooooHHo",
      "oHHoLLLLoHHo",
      "oHSoLccGoSHo",
      ".oSoLccGoSo.",
      "..ooLGGGoo..",
      "...oGGGGo...",
      "...oooooo...",
      "............",
      "............",
    ],
  },
  // A stopwatch with its hand at two o'clock.
  speedrunner: {
    palette: { H: C.steelHi, S: C.iron, F: C.cream, R: C.coral },
    rows: [
      "....oooo....",
      "...oSSSSo...",
      "....oSSo....",
      "...oHHHHo...",
      "..oHFFFFSo..",
      ".oHFFFFRFSo.",
      ".oHFFFRFFSo.",
      ".oHFFRFFFSo.",
      ".oHFFFFFFSo.",
      "..oSFFFFSo..",
      "...oSSSSo...",
      "....oooo....",
    ],
  },
  // A price tag with its string hole: the bargain, and the letters it is priced on.
  qs_bargain: {
    palette: { T: C.amber, H: C.cream, S: C.goldDeep },
    rows: [
      "............",
      "....ooooooo.",
      "...oHHHHHHHo",
      "..oHTTTTTTTo",
      ".oHTTTTTTTTo",
      "oHTToTTTTTTo",
      "oHTToTTTTTTo",
      ".oTTTTTTTTSo",
      "..oTTTTTTTSo",
      "...oSSSSSSSo",
      "....ooooooo.",
      "............",
    ],
  },
  // Gold-rimmed reading glasses: a grammarian who wants paying for it.
  greedy_grammarian: {
    palette: { G: C.gold, H: C.steelHi, w: C.cyan, D: C.goldDeep },
    rows: [
      "............",
      "............",
      "..ooo..ooo..",
      ".oGGGooGGGo.",
      "oGHwwGGwwwGo",
      "oGwwwGGwwwGo",
      "oGwwwGGwwwGo",
      ".oDDDooDDDo.",
      "..ooo..ooo..",
      "............",
      "............",
      "............",
    ],
  },
  // Two masks, the glad one in front and its other face behind.
  doppelganger: {
    palette: { g: C.gold, G: C.goldDeep, C: C.cyan, S: C.slate },
    rows: [
      ".oooooo.....",
      "oggggggo....",
      "ogoggogooooo",
      "oggggggoCCCo",
      "ogoggogoCoCo",
      "oggooggoCCCo",
      ".oGGGGoCCCCo",
      "..ooooCCooCo",
      "....oCCoCCoo",
      ".....oSSSSo.",
      "......oooo..",
      "............",
    ],
  },
  // A comet with its tail streaming behind: the run does not stop.
  hot_streak: {
    palette: { Y: C.cream, F: C.amber, R: C.coral },
    rows: [
      "......oooo..",
      "...o.oYYYYo.",
      "..oRoYYYYFFo",
      ".oRooYYYFFFo",
      "oRooRYYFFFFo",
      ".ooRoYFFFFFo",
      ".oRo.oFFFFo.",
      "oRo.oRoooo..",
      ".o.oRo......",
      "..oRo.......",
      ".oRo........",
      "..o.........",
    ],
  },
  // A sack tied at the neck, a dollar on it: what is held is not spent.
  hoarder: {
    palette: { W: C.wood, w: C.woodHi, g: C.gold },
    rows: [
      "...oo..oo...",
      "..owwoowwo..",
      "...owwwwo...",
      "....oooo....",
      "...owwwwo...",
      "..owwwWWWo..",
      ".owwwggWWWo.",
      ".owwwgWWWWo.",
      ".owwwWgWWWo.",
      ".owwwggWWWo.",
      "..oWWWWWWo..",
      "...oooooo...",
    ],
  },
  // A cactus: prickly, and paid for every gray it lands on.
  masochist: {
    palette: { L: C.leafHi, G: C.leaf, P: C.clay, p: C.clayHi },
    rows: [
      "....oooo....",
      "...oLGGGo...",
      "...oLGGGooo.",
      ".oooLGGGoLGo",
      "oLGoLGGGoLGo",
      "oLGoLGGGoLGo",
      "oLGGLGGGGGGo",
      ".oooLGGGooo.",
      "...oLGGGo...",
      "..opPPPPPo..",
      "...oPPPPo...",
      "....oooo....",
    ],
  },
  // A bell ringing, three voices at once.
  chorus: {
    palette: { B: C.steel, b: C.steelHi, C: C.slate, w: C.cyan },
    rows: [
      ".....oo.....",
      "....oBBo....",
      "w..oBbBBo..w",
      ".w.oBbBBo.w.",
      "..oBBbBBBo..",
      "..oBBBBBBo..",
      "..oBBBBBBo..",
      ".oBBBBBBBBo.",
      "oBBBBBBBBBBo",
      "oooooooooooo",
      "....oCCo....",
      ".....oo.....",
    ],
  },
  // A ladder, climbed one letter at a time.
  alphabetist: {
    palette: { W: C.wood, w: C.woodHi },
    rows: [
      ".oo......oo.",
      "owWo....owWo",
      "owWoooooowWo",
      "owWwwwwwwwWo",
      "owWoooooowWo",
      "owWo....owWo",
      "owWoooooowWo",
      "owWwwwwwwwWo",
      "owWoooooowWo",
      "owWo....owWo",
      "owWo....owWo",
      ".oo......oo.",
    ],
  },
  // A gold padlock, shut: the vault is what has been guessed so far.
  vault: {
    palette: { H: C.steelHi, S: C.steel, I: C.iron, g: C.gold, G: C.goldDeep },
    rows: [
      "....oooo....",
      "...oHSSHo...",
      "..oHSooSIo..",
      ".ooSooooIoo.",
      "oggggggggggo",
      "ogggggggggGo",
      "oggggoogggGo",
      "oggggoogggGo",
      "ogggggggggGo",
      "oGGGGGGGGGGo",
      ".oooooooooo.",
      "............",
    ],
  },
  // A stack of freshly struck coins.
  mint: {
    palette: { H: C.cream, g: C.gold, G: C.goldDeep },
    rows: [
      "............",
      "...oooooo...",
      "..oHHHHHHo..",
      ".oHggggggGo.",
      ".oGGGGGGGGo.",
      ".oHggggggGo.",
      ".oGGGGGGGGo.",
      ".oHggggggGo.",
      ".oGGGGGGGGo.",
      "..oooooooo..",
      "............",
      "............",
    ],
  },
  // A volcano running with lava: the ground left burnt.
  scorched_earth: {
    palette: { R: C.coral, A: C.amber, I: C.steel, S: C.iron },
    rows: [
      ".....oo.....",
      "....oRRo....",
      "...oRAARo...",
      "..oISRRSSo..",
      "..oISSRSSo..",
      ".oISSSRSSSo.",
      ".oISSSSRSSo.",
      "oISSSSSRSSSo",
      "oISSSSSSSSSo",
      "oISSSSSSSSSo",
      "oSSSSSSSSSSo",
      ".oooooooooo.",
    ],
  },
  // A snowball rolling, speed lines behind it, growing as it goes.
  snowball: {
    palette: { w: C.cyan, H: C.steelHi, C: C.steel },
    rows: [
      "............",
      "......ooo...",
      ".....oHHHo..",
      ".oo.oHHHHCo.",
      "owwoHHHHHCCo",
      ".oooHHHHCCCo",
      "owwoHHHHCCCo",
      ".oo.oHHCCCo.",
      ".....oCCCo..",
      "......ooo...",
      "............",
      "............",
    ],
  },
  // A fat book with a gilt title band: a thesaurus.
  thesaurus: {
    palette: { V: C.violet, v: C.violetHi, g: C.gold, P: C.cream, C: C.steel },
    rows: [
      "..ooooooooo.",
      ".oVVVVVVVPPo",
      "ovVVVVVVVCPo",
      "ovVggggggCPo",
      "ovVVVVVVVCPo",
      "ovVggggVVCPo",
      "ovVVVVVVVCPo",
      "ovVVVVVVVCPo",
      "ovVVVVVVVCPo",
      ".oVVVVVVVPPo",
      "..ooooooooo.",
      "............",
    ],
  },
  // A heart: what a patron gives.
  patron: {
    palette: { R: C.coral, c: C.cream, D: C.red },
    rows: [
      "............",
      "..ooo..ooo..",
      ".oRRRooRRRo.",
      "oRccRRRRRRDo",
      "oRcRRRRRRRDo",
      "oRRRRRRRRRDo",
      ".oRRRRRRRDo.",
      "..oRRRRRDo..",
      "...oRRRDo...",
      "....oRDo....",
      ".....oo.....",
      "............",
    ],
  },
  // A gold pen nib: the ink that does not fade.
  indelible: {
    palette: { H: C.cream, g: C.gold, G: C.goldDeep },
    rows: [
      "....oooo....",
      "...oHggGo...",
      "..oHgggGGo..",
      ".oHgggggGGo.",
      ".oHggooggGo.",
      "..oHgoogGo..",
      "..oHgoogGo..",
      "...ogoogo...",
      "...oGooGo...",
      "....oGGo....",
      ".....oo.....",
      "............",
    ],
  },
  // A leaf the wind has got hold of, the gusts under it.
  second_wind: {
    palette: { L: C.leafHi, G: C.leaf, H: C.steelHi },
    rows: [
      "......ooooo.",
      "....ooLLLLGo",
      "...oLLLLGGGo",
      "..oLLLGGLGo.",
      "..oLLGGLGGo.",
      "..oLGGGGGo..",
      "..oGGGGoo...",
      ".ooooooo....",
      "oHHHHHHo....",
      ".oooooooooo.",
      "...oHHHHHHHo",
      "....ooooooo.",
    ],
  },
  // An hourglass, most of its sand still to fall.
  long_game: {
    palette: { W: C.wood, H: C.steelHi, A: C.amber },
    rows: [
      ".oooooooooo.",
      "oWWWWWWWWWWo",
      ".oHAAAAAAHo.",
      "..oHAAAAHo..",
      "...oHAAHo...",
      "....oAAo....",
      "...oHooHo...",
      "..oHoAAoHo..",
      ".oHoAAAAoHo.",
      ".oHAAAAAAHo.",
      "oWWWWWWWWWWo",
      ".oooooooooo.",
    ],
  },
  // A fire, well alight.
  pyromaniac: {
    palette: { R: C.coral, A: C.amber, g: C.gold, c: C.cream },
    rows: [
      ".....oo.....",
      "....oRRo....",
      "....oRRRo...",
      "...oRRARRo..",
      "..oRRAARRo..",
      ".oRRAAAARRo.",
      ".oRAAggAARo.",
      "oRRAggggARRo",
      "oRAAgccgAARo",
      "oRRAgccgARRo",
      ".oRRAAAARRo.",
      "..oooooooo..",
    ],
  },
  // Two sheets, one printed through the other.
  carbon_copy: {
    palette: { V: C.violet, v: C.violetHi, P: C.steelHi, S: C.slate },
    rows: [
      "....ooooooo.",
      "...ovVVVVVVo",
      "...ovVVVVVVo",
      ".oooVVVVVVVo",
      "oPPPPPPPVVVo",
      "oPSSSSSPVVVo",
      "oPPPPPPPVVVo",
      "oPSSSPPPVVVo",
      "oPPPPPPPVVVo",
      "oPSSSSSPooo.",
      "oPPPPPPPo...",
      ".ooooooo....",
    ],
  },
  // A hand raised and waving: the first impression is the first tile.
  first_impression: {
    palette: { g: C.gold, G: C.goldDeep, C: C.cyan },
    rows: [
      "...o.o.o.o..",
      "..ogogogogoC",
      "..ogogogogoC",
      "..ogogogogo.",
      "oooggggggGo.",
      "oggggggggGo.",
      ".ogggggggGo.",
      "..oggggggGo.",
      "..ogggggGGo.",
      "...oGGGGGo..",
      "....ooooo...",
      "............",
    ],
  },
  // Two cherries on one stem: the letter that shows up twice.
  twins: {
    palette: { R: C.coral, r: C.red, c: C.cream, L: C.leaf },
    rows: [
      ".......ooo..",
      "......oLLLo.",
      ".....ooooo..",
      "....oo..o...",
      "...oo...o...",
      ".oooo..oooo.",
      ".oRRo..oRRo.",
      "oRcRRooRcRRo",
      "oRRRrooRRRro",
      "oRRrrooRrrro",
      ".oooo..oooo.",
      "............",
    ],
  },
  // A page with nothing on it yet, corner turned down.
  blank_page: {
    palette: { c: C.cream, H: C.steelHi, S: C.steel },
    rows: [
      "..oooooo....",
      "..occccoo...",
      "..occccoHo..",
      "..occccooo..",
      "..occcccSo..",
      "..occcccSo..",
      "..occcccSo..",
      "..occcccSo..",
      "..occcccSo..",
      "..occcccSo..",
      "..oSSSSSSo..",
      "..oooooooo..",
    ],
  },
  // A figure eight on its side, two-toned: the loop you keep walking.
  habit: {
    palette: { H: C.cyan, C: C.mint },
    rows: [
      "............",
      "............",
      "..ooo..ooo..",
      ".oHHHooCCCo.",
      "oHHoHHCCoCCo",
      "oHooCHHCooCo",
      "oHHoCCCCoCCo",
      ".oHHHooCCCo.",
      "..ooo..ooo..",
      "............",
      "............",
      "............",
    ],
  },
  // A bullseye: dead centre or nothing.
  no_maybes: {
    palette: { R: C.coral, c: C.cream, g: C.amber },
    rows: [
      "....oooo....",
      "..ooRRRRoo..",
      ".oRRccccRRo.",
      ".oRccRRccRo.",
      "oRccRRRRccRo",
      "oRcRRggRRcRo",
      "oRcRRggRRcRo",
      "oRccRRRRccRo",
      ".oRccRRccRo.",
      ".oRRccccRRo.",
      "..ooRRRRoo..",
      "....oooo....",
    ],
  },
  // Growth that compounds: each column stands on the last.
  compound: {
    palette: { L: C.leafHi, G: C.leaf },
    rows: [
      "..........o.",
      ".........oLo",
      "........oLGo",
      "........oGGo",
      ".......oLGGo",
      ".......oGGGo",
      "......oLGGGo",
      ".....oLGGGGo",
      "...ooLGGGGGo",
      ".ooLLGGGGGGo",
      "oLLGGGGGGGGo",
      ".oooooooooo.",
    ],
  },
  // A crown: paid on every clear.
  royalties: {
    palette: { H: C.cream, g: C.gold, G: C.goldDeep, R: C.coral, c: C.cyan },
    rows: [
      "............",
      ".o...oo...o.",
      "oHo.oHHo.oHo",
      "oggooggooggo",
      "ogggoggogggo",
      "oggggggggggo",
      "ogRRgccgRRgo",
      "ogRRgccgRRgo",
      "oGGGGGGGGGGo",
      ".oooooooooo.",
      "............",
      "............",
    ],
  },
}

/** The consumables, keyed by the engine id `emblems.ts` uses. */
export const CONSUMABLE_SPRITES: Record<string, Sprite> = {
  // Her crystal ball on its stand, a star turning in it.
  oracle: {
    palette: { V: C.violet, v: C.violetHi, g: C.gold, w: C.woodHi, W: C.wood },
    rows: [
      "...oooooo...",
      "..ovvvVVVo..",
      ".ovvVVVVVVo.",
      "ovvVVgVVVVVo",
      "ovVVgggVVVVo",
      "oVVVVgVVVVVo",
      "oVVVVVVVgVVo",
      ".oVVVVVVVVo.",
      "..oVVVVVVo..",
      "..oooooooo..",
      ".owwWWWWWWo.",
      ".oooooooooo.",
    ],
  },
  // His lantern, hung on the staff, lit.
  hermit: {
    palette: { W: C.wood, H: C.steel, A: C.amber, Y: C.cream },
    rows: [
      ".....oooooo.",
      "....oWWWWWWo",
      "...ooWooooWo",
      "..oHHHHo.oWo",
      ".oHAYYAHooWo",
      ".oHAYYAHooWo",
      ".oHAAAAHooWo",
      "..oHHHHo.oWo",
      "...oooo..oWo",
      ".........oWo",
      ".........oWo",
      "..........o.",
    ],
  },
  // A wand, with a spark at its tip.
  magician: {
    palette: { W: C.wood, Y: C.cream, A: C.amber, V: C.violet },
    rows: [
      "........o...",
      ".......oYo..",
      "......oYAYo.",
      ".......oYo..",
      "......oWoo..",
      ".....oWooVo.",
      "....oWooVYVo",
      "...oWo..ooVo",
      "..oWo.....o.",
      ".oWo........",
      "oWo.........",
      ".o..........",
    ],
  },
  // The Fool as the pack draws him: a clown, red nose and a wide red mouth.
  fool: {
    palette: { c: C.cream, A: C.amber, R: C.coral },
    rows: [
      "....oooo....",
      ".oooccccooo.",
      "oAAoccccoAAo",
      "oAoccccccoAo",
      "oAococcocoAo",
      ".occcRRccco.",
      ".occcRRccco.",
      ".ocRccccRco.",
      ".occRRRRcco.",
      "..occcccco..",
      "...oooooo...",
      "............",
    ],
  },
}

/** The packs, keyed by the engine id `emblems.ts` uses. */
export const PACK_SPRITES: Record<string, Sprite> = {
  // A tile lettered A, another behind it: the alphabet pack.
  alphabet: {
    palette: { H: C.cream, S: C.steel, K: C.violet },
    rows: [
      "...oooooooo.",
      ".ooSSSSSSSSo",
      "oHHHHHHHHSSo",
      "oHHKKKHHHSSo",
      "oHKHHHKHHSSo",
      "oHKHHHKHHSSo",
      "oHKKKKKHHSSo",
      "oHKHHHKHHSSo",
      "oHKHHHKHHSSo",
      "oHKHHHKHHSSo",
      "oHHHHHHHHoo.",
      ".oooooooo...",
    ],
  },
  // A cut gem with its sparkles: the pack the relics come out of.
  relic: {
    palette: { H: C.cyan, C: C.violet, D: C.violetHi, g: C.gold },
    rows: [
      "...oo..oo...",
      "..oggooggo..",
      "...ogoogo...",
      "..oHHHHHHo..",
      ".oHDDCCCCCo.",
      "oHDDCCCCCCCo",
      ".oHCCCCCCCo.",
      "..oHCCCCCo..",
      "...oHCCCo...",
      "....oHCo....",
      ".....oo.....",
      "............",
    ],
  },
  // A box of tabbed dividers: categories, filed.
  category: {
    palette: { R: C.coral, C: C.cyan, L: C.leaf, g: C.gold, A: C.amber },
    rows: [
      "............",
      ".ooo........",
      "oRRRoooo....",
      "oRRRoCCCoooo",
      "oRRRoCCCoLLo",
      "oooooooooooo",
      "oggggggggggo",
      "oggggggggggo",
      "oggggggggggo",
      "oAAAAAAAAAAo",
      "oooooooooooo",
      "............",
    ],
  },
}

/** The etchings, keyed by the engine id `emblems.ts` uses. */
export const ETCHING_SPRITES: Record<string, Sprite> = {
  // An open mouth: the shape a vowel is sung in.
  etch_vowels: {
    palette: { R: C.coral, D: C.red, c: C.cream },
    rows: [
      "............",
      "....oooo....",
      "..ooDDDDoo..",
      ".oDDRRRRDDo.",
      "oDRRRRRRRRDo",
      "oDRRcccRRRDo",
      "oDRRRRRRRRDo",
      ".oDDRRRRDDo.",
      "..ooDDDDoo..",
      "....oooo....",
      "............",
      "............",
    ],
  },
  // A loaf, scored: L N S T R, what every word is made of.
  etch_staples: {
    palette: { W: C.wood, w: C.woodHi, A: C.amber },
    rows: [
      "............",
      "............",
      "....oooo....",
      "...owwwwo...",
      "..owAAAAWo..",
      ".owAAAAAAWo.",
      "owAAWAAWAAWo",
      "owAAAAAAAAWo",
      ".oWWWWWWWWo.",
      "..oooooooo..",
      "............",
      "............",
    ],
  },
  // A rock: J Q X Z are the heavy ones.
  etch_heavy: {
    palette: { H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      "....oooo....",
      "..ooHHHSoo..",
      ".oHHHSSSSIo.",
      ".oHSSSSSIIo.",
      "oHHSSSISSIIo",
      "oHSSSIISSSIo",
      "oHSSSSSSSIIo",
      "oSSSSSSSIIIo",
      ".oSIIIIIIIo.",
      "..oooooooo..",
      "............",
    ],
  },
  // A scroll between its rollers: the long roll of everything not a vowel.
  etch_consonants: {
    palette: { w: C.woodHi, W: C.wood, c: C.cream, S: C.slate },
    rows: [
      ".oooooooooo.",
      "owwwwwwwwwWo",
      ".oooooooooo.",
      "..occcccco..",
      "..ocSSSSco..",
      "..occcccco..",
      "..ocSSSSco..",
      "..occcccco..",
      "..ocSSScco..",
      ".oooooooooo.",
      "owwwwwwwwwWo",
      ".oooooooooo.",
    ],
  },
}

/** The letter modifiers, keyed by the engine id `emblems.ts` uses. */
export const MOD_SPRITES: Record<string, Sprite> = {
  // A poker chip, its rim marked.
  chip: {
    palette: { R: C.coral, D: C.red, c: C.cream },
    rows: [
      "....oooo....",
      "...oRRRRo...",
      "..oRRccRRo..",
      ".oRRRRRRRRo.",
      "oRRDDDDDDRRo",
      "ocRDRRRRDRco",
      "ocRDRRRRDRco",
      "oRRDDDDDDRRo",
      ".oRRRRRRRRo.",
      "..oRRccRRo..",
      "...oRRRRo...",
      "....oooo....",
    ],
  },
  // The sign mult is written with.
  mult: {
    palette: { M: C.mint, H: C.mintHi },
    rows: [
      "............",
      ".oo......oo.",
      "oHHo....oHMo",
      "oHMMo..oMMMo",
      ".oMMMooMMMo.",
      "..oMMMMMMo..",
      "...oMMMMo...",
      "..oMMMMMMo..",
      ".oMMMooMMMo.",
      "oMMMo..oMMMo",
      "oMMo....oMMo",
      ".oo......oo.",
    ],
  },
  // A dollar sign in gold, no coin round it.
  gold: {
    palette: { g: C.gold, G: C.goldDeep },
    rows: [
      "....oooo....",
      "..ooogGooo..",
      ".oggggggggo.",
      ".oggooooooo.",
      ".oggoooooo..",
      ".oggggggggo.",
      "..ooooooGGo.",
      ".oooooooGGo.",
      ".oggggggGGo.",
      "..ooogGooo..",
      "....oooo....",
      "............",
    ],
  },
  // The wild card's star, off its card: the card was most of the twelve pixels.
  wild: {
    palette: { H: C.cream, A: C.gold, G: C.goldDeep },
    rows: [
      "............",
      ".....oo.....",
      "....oHAo....",
      "....oHAo....",
      "ooooAAAAoooo",
      "oAAAAAAAAAGo",
      ".oAAAAAAAGo.",
      "..oAAAAAGo..",
      "..oAAAAAGo..",
      ".oAAGooAAGo.",
      ".oAGo..oGGo.",
      ".ooo....ooo.",
    ],
  },
  // A four-leaf clover on its stem.
  lucky: {
    palette: { L: C.leafHi, G: C.leaf, W: C.wood },
    rows: [
      "..ooo.ooo...",
      ".oLLLoLLGo..",
      "oLLLLLLGGGo.",
      "oLLLLLGGGGo.",
      ".oLLLGGGGo..",
      "oLLLLGGGGGo.",
      "oLLGGGGGGGo.",
      ".oGGGoGGGo..",
      "..oooWoooo..",
      ".....oWWo...",
      "......oWWo..",
      ".......oo...",
    ],
  },
  // Rings spreading from a sound.
  echo: {
    palette: { C: C.cyan, H: C.cream },
    rows: [
      "............",
      ".....oo.....",
      "....oCCo....",
      "...oCooCo...",
      "..oCoHHoCo..",
      ".oCoHooHoCo.",
      ".oCoHooHoCo.",
      "..oCoHHoCo..",
      "...oCooCo...",
      "....oCCo....",
      ".....oo.....",
      "............",
    ],
  },
  // A real anchor: pays when the letter stays put.
  anchor: {
    palette: { S: C.steel, H: C.steelHi },
    rows: [
      "....oooo....",
      "...oHHHHo...",
      "...oHSSHo...",
      ".oooHHHHooo.",
      "oHHHHSSHHHHo",
      ".oooHSSHooo.",
      ".o.oHSSHo.o.",
      "oHooHSSHooHo",
      "oHSoHSSHoSHo",
      ".oHSSSSSSHo.",
      "..oHHHHHHo..",
      "...oooooo...",
    ],
  },
  // A steel nut, seen down the thread.
  steel: {
    palette: { H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      "...oooooo...",
      "..oHHHHHSo..",
      ".oHHSSSSSIo.",
      "oHSSooooSSIo",
      "oHSSooooSSIo",
      "oHSSooooSSIo",
      "oSSSooooSIIo",
      ".oSSSSSSIIo.",
      "..oSIIIIIo..",
      "...oooooo...",
      "............",
    ],
  },
  // A glass of water, already cracked.
  glass: {
    palette: { H: C.cream, S: C.steelHi, C: C.cyan, I: C.iron },
    rows: [
      "............",
      ".oooooooooo.",
      ".oHSSSISSSo.",
      ".oHSSSSISSo.",
      ".oHCCCICCCo.",
      ".oHCCCCCCCo.",
      "..oHCCCCCo..",
      "..oHCCCCCo..",
      "..oHCCCCCo..",
      "..oCCCCCCo..",
      "...oooooo...",
      "............",
    ],
  },
}

/** The word shapes, keyed by the engine id `emblems.ts` uses. */
export const CATEGORY_SPRITES: Record<string, Sprite> = {
  // A one-way sign.
  alphabetical: {
    palette: { H: C.steelHi, S: C.steel, M: C.mint },
    rows: [
      ".oooooooooo.",
      "oHHHHHHHHHHo",
      "oHSSSSSSSSSo",
      "oHSSSSSSSSSo",
      "oHSSMSSSSSSo",
      "oHSSSMMSSSSo",
      "oHSSSMMMMSSo",
      "oHSSSMMSSSSo",
      "oHSSMSSSSSSo",
      "oHSSSSSSSSSo",
      "oHHHHHHHHHHo",
      ".oooooooooo.",
    ],
  },
  // A speech bubble of three vowels.
  vowel_heavy: {
    palette: { H: C.cream, S: C.steel, M: C.mint },
    rows: [
      "............",
      ".oooooooooo.",
      "oHHHHHHHHHHo",
      "oHSSSSSSSSSo",
      "oHSMSMSMSSSo",
      "oHSSSSSSSSSo",
      "oHHHHHHHHSSo",
      ".ooooHHSSoo.",
      ".....oHSo...",
      "......oo....",
      "............",
      "............",
    ],
  },
  // Three consonant tiles, linked.
  cluster: {
    palette: { H: C.cream, S: C.steel, M: C.mint },
    rows: [
      "............",
      "............",
      "..oo.oo.oo..",
      ".oHSoHSoHSo.",
      ".oHSoHSoHSo.",
      ".oHSMHSMHSo.",
      ".oHSMHSMHSo.",
      ".oHSoHSoHSo.",
      ".oHSoHSoHSo.",
      "..oo.oo.oo..",
      "............",
      "............",
    ],
  },
  // The sign of the twins: some letter twice.
  twinned: {
    palette: { V: C.violet, v: C.violetHi },
    rows: [
      "oooooooooooo",
      "ovvvvvvvvvVo",
      "ooovVoovVooo",
      "..ovVoovVo..",
      "..ovVoovVo..",
      "..ovVoovVo..",
      "..ovVoovVo..",
      "..ovVoovVo..",
      "..ovVoovVo..",
      "ooovVoovVooo",
      "ovVVVVVVVVVo",
      "oooooooooooo",
    ],
  },
  // A fingerprint: no ridge repeats.
  distinct: {
    palette: { M: C.mint, H: C.mintHi, I: C.iron },
    rows: [
      "...oooooo...",
      "..oHHHHHHo..",
      ".oHMMMMMMIo.",
      "oHMMIIIIMMIo",
      "oHMIMMMMIMIo",
      "oHMIMIIMIMIo",
      "oHMIMIMMIMIo",
      "oHMIMIIMIMIo",
      "oHMMIMMIMMIo",
      ".oHMMIIMMIo.",
      "..oIIIIIIo..",
      "...oooooo...",
    ],
  },
}

/** The alphabet ranges, keyed by the engine id `emblems.ts` uses. */
export const RANGE_SPRITES: Record<string, Sprite> = {
  // The alphabet in quarters, A to E standing tall.
  range_ae: {
    palette: { M: C.mintHi, N: C.mint, H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      "MNN.........",
      "MNN.........",
      "MNN.........",
      "MNN.........",
      "MNN.........",
      "MNNHIIHSSHII",
      "MNNHIIHSSHII",
      "MNNHIIHSSHII",
      "MNNHIIHSSHII",
      "MNNHIIHSSHII",
      "MNNHIIHSSHII",
    ],
  },
  // The alphabet in quarters, F to M standing tall.
  range_fm: {
    palette: { M: C.mintHi, N: C.mint, H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      "...MNN......",
      "...MNN......",
      "...MNN......",
      "...MNN......",
      "...MNN......",
      "HSSMNNHSSHII",
      "HSSMNNHSSHII",
      "HSSMNNHSSHII",
      "HSSMNNHSSHII",
      "HSSMNNHSSHII",
      "HSSMNNHSSHII",
    ],
  },
  // The alphabet in quarters, N to R standing tall.
  range_nr: {
    palette: { M: C.mintHi, N: C.mint, H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      "......MNN...",
      "......MNN...",
      "......MNN...",
      "......MNN...",
      "......MNN...",
      "HSSHIIMNNHII",
      "HSSHIIMNNHII",
      "HSSHIIMNNHII",
      "HSSHIIMNNHII",
      "HSSHIIMNNHII",
      "HSSHIIMNNHII",
    ],
  },
  // The alphabet in quarters, S to Z standing tall.
  range_sz: {
    palette: { M: C.mintHi, N: C.mint, H: C.steelHi, S: C.steel, I: C.iron },
    rows: [
      "............",
      ".........MNN",
      ".........MNN",
      ".........MNN",
      ".........MNN",
      ".........MNN",
      "HSSHIIHSSMNN",
      "HSSHIIHSSMNN",
      "HSSHIIHSSMNN",
      "HSSHIIHSSMNN",
      "HSSHIIHSSMNN",
      "HSSHIIHSSMNN",
    ],
  },
}

/**
 * The sprite a card wears, or null for an id the tables have no drawing for.
 * The lookup `emblem` makes for the line drawings, kind for kind, so a view can
 * ask for both and let the stylesheet decide which one is on show.
 */
export function sprite(kind: ShopItem["kind"], id: string): SVGSVGElement | null {
  const table: Record<string, Sprite> =
    kind === "relic"
      ? RELIC_SPRITES
      : kind === "consumable"
        ? CONSUMABLE_SPRITES
        : kind === "pack"
          ? PACK_SPRITES
          : kind === "etch"
            ? ETCHING_SPRITES
            : kind === "mod"
              ? MOD_SPRITES
              : kind === "level"
                ? CATEGORY_SPRITES
                : RANGE_SPRITES
  const found = table[id]
  return found ? spriteSvg(found, `sprite sprite-${id}`) : null
}

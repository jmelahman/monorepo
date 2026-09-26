/**
 * The handful of line icons the title screen and the shop draw, as SVG built in
 * place.
 *
 * Its own module because `h()` cannot make them: it calls `createElement`, and
 * an `<svg>` made that way is an unknown HTML element that lays out as nothing.
 * SVG has to come from `createElementNS`, and teaching `h()` a namespace for
 * five glyphs would be the whole of `dom.ts` growing a second mode.
 *
 * Line icons rather than emoji, which is what these slots held before. An emoji
 * is drawn from the platform's colour font, so the same lock was a gold padlock
 * on Android, a grey one on Windows and a different size on each; and it cannot
 * take `currentColor`, which is what lets a lock that means "not earned yet" be
 * the carrot's gold everywhere it appears. The flag on the language button stays
 * an emoji on purpose: it is a picture of a country, not an icon, and no stroke
 * drawing says "Español" better.
 *
 * All share one 24-unit box, a 2-unit round stroke and `currentColor`, so a
 * caller sizes and colours them from CSS like a letter, and none of them is
 * announced: every one sits on a button whose label already says what it does.
 */

const SVG = "http://www.w3.org/2000/svg"

const PATHS = {
  lock: ["M5 11h14v10H5z", "M8 11V8a4 4 0 0 1 8 0v3"],
  help: [
    "M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18",
    "M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6V14",
    "M12 17.5h.01",
  ],
  book: ["M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2z", "M4 21V5"],
  sound: ["M11 5 6 9H3v6h3l5 4z", "M15.5 8.5a5 5 0 0 1 0 7", "M18.5 5.5a9 9 0 0 1 0 13"],
  muted: ["M11 5 6 9H3v6h3l5 4z", "M16 9l6 6", "M22 9l-6 6"],
  // The shelf's kinds, one each, so a list of five rows can be scanned for "is
  // there a relic" down the left edge without reading a word. Drawn as the thing
  // rather than as a letter of its name: a gem, a flask, a box, a tile. See
  // `KIND_ICON` in `views.ts` for which kind wears which.
  gem: ["M6 3h12l4 6-10 12L2 9z", "M2 9h20"],
  flask: ["M9 3h6", "M10 3v6l-5 9a2 2 0 0 0 1.7 3h10.6a2 2 0 0 0 1.7-3l-5-9V3", "M7.5 15h9"],
  box: ["M3 8l9-5 9 5v8l-9 5-9-5z", "M3 8l9 5 9-5", "M12 13v8"],
  alphabet: [
    "M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z",
    "M8.5 16.5l3.5-9 3.5 9",
    "M9.8 13.5h4.4",
  ],
  letter: [
    "M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z",
    "M12 7l3.5 5-3.5 5-3.5-5z",
  ],
  shape: ["M2 8h5v8H2z", "M9.5 8h5v8h-5z", "M17 8h5v8h-5z", "M2 20h20"],
  etching: ["M14 4l6 6-9 9H5v-6z", "M11 7l6 6"],
  check: ["M5 12l5 5 9-10"],
  reroll: [
    "M4 12a8 8 0 0 1 14-5.3L20 9",
    "M20 4v5h-5",
    "M20 12a8 8 0 0 1-14 5.3L4 15",
    "M4 20v-5h5",
  ],
} as const

export type IconName = keyof typeof PATHS

export function icon(name: IconName): SVGSVGElement {
  const svg = document.createElementNS(SVG, "svg")
  svg.setAttribute("class", `icon icon-${name}`)
  svg.setAttribute("viewBox", "0 0 24 24")
  svg.setAttribute("aria-hidden", "true")
  for (const d of PATHS[name]) {
    const path = document.createElementNS(SVG, "path")
    path.setAttribute("d", d)
    svg.append(path)
  }
  return svg
}

import { lang } from "./lang"
import type { Skin } from "./skin"

/**
 * The faces each look is set in, fetched before the look is put on the page.
 *
 * Every face is declared for every skin (in each skin's `fonts.css`), and a browser
 * fetches a declared face only when text on the page first asks for it. That is
 * what keeps a player who never leaves Classic from downloading any of them, and
 * it stays. What it cost was the switch: the dial flipped the class, the whole
 * screen painted in the system face, and then painted again when the file
 * arrived, which under Jersey 20's `size-adjust` is a reflow of every line and
 * not only a change of letterform. Measured in Chromium at 4× CPU and a 50 kB/s,
 * 400ms-RTT link from a cold cache: 88 frames of Tabletop in the system face
 * (about 1.5s) and 43 of Smoke.
 *
 * So the dial asks for the faces first and flips the class when they are in
 * (`skinReady`), and while the dial is on screen the next stop's faces are
 * already on their way (`warmSkin`). Asked for one step ahead rather than all
 * at once, because a player looking at the title screen has not chosen to see
 * Tabletop, and the one stop the next tap lands on is all a tap can need.
 *
 * Only what the look's text uses. Smoke's IBM Plex Mono is behind Jersey 20 in
 * the stack, and Jersey covers Latin and Latin-extended, so Plex is the fallback
 * for a glyph neither of those has and loading it here would fetch a file the
 * page never draws from. Tabletop leans on weight (see its `fonts.css`), so all
 * four of its weights are in; the two Classics are the system stack and need
 * nothing.
 */
export const FACES: Record<Skin, readonly string[]> = {
  smoke: ['400 1em "Jersey 20"'],
  tabletop: ["400 1em Jost", "500 1em Jost", "600 1em Jost", "700 1em Jost"],
  "classic-dark": [],
  "classic-light": [],
}

/**
 * How long the dial will hold a tap for its faces. A switch that waits on a slow
 * link forever is a dial that looks broken, which is worse than one late swap;
 * 300ms is under the point a tap reads as ignored, and the warm-ahead means the
 * wait is usually over before it starts.
 */
const CAP_MS = 300

/**
 * The text the faces are asked for with. `document.fonts.load` fetches only the
 * files whose `unicode-range` the sample touches, so English asks for the Latin
 * file alone, and the translations add the accented letters that pull in the
 * Latin-extended one their prose will need (`œ` and `ß` are outside Latin-1, so
 * they are what reaches it).
 */
const sample = (): string => (lang() === "en" ? "5 Wild" : "5 Wild œß")

function load(skin: Skin): Promise<unknown> {
  if (typeof document === "undefined" || !document.fonts) return Promise.resolve()
  const text = sample()
  // A face that fails to load is the stack's fallback and nothing worse, which is
  // the look the page would have shown anyway; it is never a reason to hold the
  // switch or to surface an error.
  return Promise.all(FACES[skin].map((face) => document.fonts.load(face, text).catch(() => [])))
}

/**
 * Start fetching a look's faces and forget about it. Cheap to call on every
 * render: a face already loaded resolves without asking the network again.
 */
export function warmSkin(skin: Skin): void {
  void load(skin)
}

/** Resolves when a look's faces are in, or when waiting longer would be worse. */
export const skinReady = (skin: Skin): Promise<void> =>
  Promise.race([
    load(skin).then(() => {}),
    new Promise<void>((resolve) => setTimeout(resolve, CAP_MS)),
  ])

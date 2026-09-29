/**
 * The table lights, as a player setting.
 *
 * The table has two kinds of life. One is what the game *does*: tiles landing,
 * the score rolling, the shake. It has no switch here, because stillness already
 * has an answer, `prefers-reduced-motion`, and every effect honours it. The other
 * is what the room does on its own: the smoke drifting behind the panels. Those are a *taste*.
 * Some players read them as a room and some as movement at the edge of the eye
 * while they are trying to tell two yellows apart, and a taste gets a switch.
 *
 * Off means the room holds still and dark-ish, not empty: the canvas is not
 * created at all, and the flat ground in `styles/table/smoke/backdrop.css` (a
 * vignette on near-black) is what is left, which is the same picture a machine without WebGL gets. It does not
 * touch the game's own effects, for the reason above: a "lights" switch that
 * also muted the scoring would be two settings sharing a button.
 *
 * On by default, so the key is absent until a player turns it off and an absent
 * key means the look the table was drawn with. It is applied as `.lights-off` on
 * the root (only the Smoke Room reads it: Tabletop's walnut is static), which is the stylesheet's whole interface to it, for the reason
 * `.plain` and `.quiet` are classes. It is the *off* state that is the class, so
 * a document that never ran this code, a phone's, is the document it always was.
 * `fx/background.ts` watches the same class, so no caller has to tell it.
 *
 * It replaces a CRT setting, `5wild:crt`, that never shipped: there is no
 * scanline overlay any more, so there is no key to migrate and nothing reads it.
 */

const KEY = "5wild:ambience"

export const readAmbience = (raw: string | null): boolean => raw !== "off"

export function loadAmbience(): boolean {
  try {
    return readAmbience(localStorage.getItem(KEY))
  } catch {
    return true
  }
}

/**
 * Apply it, and remember it if asked. The constructor applies without writing,
 * as a first launch has chosen nothing and the key should stay absent until
 * something is.
 */
export function setAmbience(on: boolean, remember = true): void {
  document.documentElement.classList.toggle("lights-off", !on)
  if (!remember) return
  try {
    localStorage.setItem(KEY, on ? "on" : "off")
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}

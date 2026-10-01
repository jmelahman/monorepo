import { Capacitor, registerPlugin } from "@capacitor/core"
import { StatusBar, Style } from "@capacitor/status-bar"
import {
  currentSkin,
  loadLegacyTone,
  loadSkin,
  resolveSkin,
  SKINS,
  type Skin,
  saveSkin,
  skinClass,
  TONE,
  type Tone,
} from "./skin"
import { syncTable, watchTable } from "./table"

/**
 * Putting the look on the page: which skin, and everything that is keyed on its
 * tone.
 *
 * `skin.ts` decides what the look is; this applies it, and is the one place that
 * touches the root, the browser's chrome and the Android shell for it. The
 * stylesheet sees two things and only two: the `skin-*` class, for a look's own
 * rules, and `data-theme`, already resolved to `light` or `dark`, for the
 * phone's palette and everything that predates skins.
 *
 * Resolving here rather than with a `prefers-color-scheme` block in the sheet
 * keeps each palette written out once instead of twice, once under the
 * attribute and again under the media query, which is a pair that drifts the
 * first time someone edits one and not the other. The cost is that the device
 * can change its mind under a running game, which a media query would have
 * followed for free, and the window can cross into or out of the table's
 * layout, which also changes how a legacy dark pick is mapped. `watch` follows
 * both, and only while nothing has been picked: a pick is the player's and the
 * window does not overrule it.
 *
 * `data-theme` carries the tone and not the skin so that nothing keyed on it had
 * to change: `ThemePlugin.java`'s launch window, `BACKDROP`, the status bar, the
 * light tile inks in `board.css` and `keyboard.css`. The one visible price is
 * on Android, where the launch window is drawn in the tone's colour before the
 * page exists: Smoke's ground is `#06070a` and Tabletop's walnut `#1a110b`,
 * against the launch `#0e0f13`. Smoke is a step darker and Tabletop a step
 * warmer, both dark to dark, so the one frame of Classic dark's window before
 * the page paints is a shift in tint and not a flash; it was judged from the
 * values and not measured on a device.
 */

export type Theme = Tone

/**
 * What the page behind the game is, per tone. `--bg` in the stylesheet, repeated
 * because the browser chrome and the Android status bar are painted from it
 * outside the stylesheet's reach.
 */
export const BACKDROP: Record<Theme, string> = { dark: "#0e0f13", light: "#f4f3ef" }

const LIGHT_QUERY = "(prefers-color-scheme: light)"

const prefersLight = (): boolean =>
  typeof matchMedia === "function" && matchMedia(LIGHT_QUERY).matches

/**
 * `ThemePlugin.java`, which hands the tone to Android so the next launch window
 * is drawn in it. The page can only theme itself once it exists, and the launch
 * is over by then.
 */
const NativeTheme = registerPlugin<{ set(options: { theme: Theme | "system" }): Promise<void> }>(
  "Theme",
)

let picked: Skin | null = null
let legacy: Tone | null = null
let watching = false

/** What the player has said about tone, with or without a skin: a pick or an old theme. */
const stated = (): Tone | null => (picked ? TONE[picked] : legacy)

/** The tone of the look on screen now. */
export const currentTheme = (): Theme => TONE[currentSkin()]

function apply(): void {
  // The layout first: the default look depends on it.
  syncTable()
  const skin = resolveSkin({
    picked,
    legacy,
    table: document.documentElement.classList.contains("table"),
    prefersLight: prefersLight(),
  })
  const root = document.documentElement
  // One class at a time: clear every look's, then set this one's.
  for (const other of SKINS) root.classList.remove(skinClass(other))
  root.classList.add(skinClass(skin))
  const tone = TONE[skin]
  root.dataset.theme = tone
  for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    // Both media-qualified tags, so whichever one the browser is reading agrees
    // with the setting rather than with the device.
    meta.content = BACKDROP[tone]
  }
  if (Capacitor.isNativePlatform()) {
    // Style names the *background* it suits: Light is dark icons on a light bar.
    const style = tone === "light" ? Style.Light : Style.Dark
    StatusBar.setStyle({ style }).catch(() => {})
    StatusBar.setBackgroundColor({ color: BACKDROP[tone] }).catch(() => {})
    // "system" only while the player has said nothing: the phone's default look
    // follows the device, and so must the window it starts in.
    NativeTheme.set({ theme: stated() ?? "system" }).catch(() => {})
  }
}

/**
 * Follow the device and the window until the player picks. Installed once, and
 * it reads `picked` rather than closing over a look, so the first pick stops
 * the following without anything being torn down. The layout listener is
 * unconditional, since a window crossing the room query changes `.table`
 * whatever the look, and `apply` is what re-lands it.
 */
function watch(): void {
  if (watching) return
  watching = true
  watchTable(apply)
  if (typeof matchMedia === "function") {
    matchMedia(LIGHT_QUERY).addEventListener("change", () => {
      if (picked === null) apply()
    })
  }
}

/**
 * Read what was stored and put it on the page, before the first render. Writes
 * nothing: a first launch has chosen nothing and the keys stay absent until
 * something is.
 */
export function initLook(): void {
  picked = loadSkin()
  legacy = loadLegacyTone()
  watch()
  apply()
}

/** Apply a pick and remember it. */
export function setSkin(skin: Skin): void {
  picked = skin
  apply()
  saveSkin(skin)
}

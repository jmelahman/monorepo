import { Capacitor, registerPlugin } from "@capacitor/core"
import { StatusBar, Style } from "@capacitor/status-bar"
import { syncTable } from "./table"

/**
 * Light or dark, and until the player says which, whichever the device is.
 *
 * Two looks and no "system" rung. Following the device is where every install
 * starts, and it lasts until the first tap; after that the game keeps what was
 * picked. A third setting for "follow the phone" earned its place in neither
 * control: the dial had to draw it as a half-filled circle nobody reads, and on
 * a phone already in the look being followed, one of its three taps changed
 * nothing on screen. The price is that a player who picks cannot go back to
 * following except by clearing the app's data, which for a colour scheme is a
 * fair trade for a switch that always does what its face shows.
 *
 * The stylesheet sees one thing: `data-theme` on the root, already resolved.
 * Resolving here rather than with a `prefers-color-scheme` block in the sheet
 * keeps each palette written out once instead of twice, once under the
 * attribute and again under the media query, which is a pair that drifts the
 * first time someone edits one and not the other.
 *
 * The cost is that the device can change its mind under a running game, which a
 * media query would have followed for free. `watch` is that follow.
 */

const THEME_KEY = "5wild:theme"

export type Theme = "light" | "dark"

export const THEMES: readonly Theme[] = ["light", "dark"]

/** The dial's one move. */
export const OTHER_THEME: Record<Theme, Theme> = { light: "dark", dark: "light" }

/**
 * What the page behind the game is, per look. `--bg` in the stylesheet, repeated
 * because the browser chrome and the Android status bar are painted from it
 * outside the stylesheet's reach.
 */
export const BACKDROP: Record<Theme, string> = { dark: "#0e0f13", light: "#f4f3ef" }

/**
 * A stored pick, or null for "never picked", which is what a first launch and
 * anything unrecognized both get.
 */
export const readTheme = (raw: string | null): Theme | null =>
  THEMES.find((theme) => theme === raw) ?? null

export const resolveTheme = (picked: Theme | null, prefersLight: boolean): Theme =>
  picked ?? (prefersLight ? "light" : "dark")

export function loadTheme(): Theme | null {
  try {
    return readTheme(localStorage.getItem(THEME_KEY))
  } catch {
    return null
  }
}

const LIGHT_QUERY = "(prefers-color-scheme: light)"

const prefersLight = (): boolean =>
  typeof matchMedia === "function" && matchMedia(LIGHT_QUERY).matches

/**
 * `ThemePlugin.java`, which hands the choice to Android so the next launch
 * window is drawn in it. The page can only theme itself once it exists, and
 * the launch is over by then.
 */
const NativeTheme = registerPlugin<{ set(options: { theme: Theme | "system" }): Promise<void> }>(
  "Theme",
)

let picked: Theme | null = null
let watching = false

/** The look on screen now, which is what the dial shows and what a tap leaves. */
export const currentTheme = (): Theme => resolveTheme(picked, prefersLight())

function apply(): void {
  const resolved = currentTheme()
  document.documentElement.dataset.theme = resolved
  // The table look is dark-only, so it is re-asked every time the theme lands.
  syncTable()
  for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    // Both media-qualified tags, so whichever one the browser is reading agrees
    // with the setting rather than with the device.
    meta.content = BACKDROP[resolved]
  }
  if (Capacitor.isNativePlatform()) {
    // Style names the *background* it suits: Light is dark icons on a light bar.
    const style = resolved === "light" ? Style.Light : Style.Dark
    StatusBar.setStyle({ style }).catch(() => {})
    StatusBar.setBackgroundColor({ color: BACKDROP[resolved] }).catch(() => {})
    NativeTheme.set({ theme: picked ?? "system" }).catch(() => {})
  }
}

/**
 * Follow the device until the player picks. Installed once, on the first
 * `setTheme`, and it reads `picked` rather than closing over a theme, so the
 * first pick stops the following without anything being torn down.
 */
function watch(): void {
  if (watching || typeof matchMedia !== "function") return
  watching = true
  matchMedia(LIGHT_QUERY).addEventListener("change", () => {
    if (picked === null) apply()
  })
}

/**
 * Apply it to the document, and remember it. Null is "not picked", which is
 * only ever what `loadTheme` hands back at startup, so it is applied and not
 * written: nothing was chosen, and the key stays absent until something is.
 */
export function setTheme(theme: Theme | null): void {
  picked = theme
  watch()
  apply()
  if (theme === null) return
  try {
    localStorage.setItem(THEME_KEY, theme)
  } catch {
    // The setting lasts the session instead of the install. Nothing else breaks.
  }
}
